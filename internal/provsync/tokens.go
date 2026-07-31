// ABOUTME: Token store for OAuth provider credentials with flock-based serialized refresh.
// ABOUTME: Guarantees single-use refresh tokens rotate exactly once under concurrent callers.
package provsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// Token holds OAuth access and refresh credentials for a provider.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// TokenStore manages per-provider token files under a data directory.
// Token files live at <dataDir>/tokens/<provider>.json, written 0600.
// Lock files live at <dataDir>/tokens/<provider>.lock.
type TokenStore struct {
	dir string // base data directory; tokens stored in dir/tokens/
}

// NewTokenStore returns a TokenStore rooted at dataDir.
func NewTokenStore(dataDir string) *TokenStore {
	return &TokenStore{dir: filepath.Join(dataDir, "tokens")}
}

// tokenPath returns the path for provider's token JSON file.
func (s *TokenStore) tokenPath(provider string) string {
	return filepath.Join(s.dir, provider+".json")
}

// lockPath returns the path for provider's advisory lock file.
func (s *TokenStore) lockPath(provider string) string {
	return filepath.Join(s.dir, provider+".lock")
}

// Load reads the token for provider from disk.
// Returns an error if the file does not exist or is malformed.
func (s *TokenStore) Load(provider string) (Token, error) {
	data, err := os.ReadFile(s.tokenPath(provider))
	if err != nil {
		return Token{}, fmt.Errorf("load token %s: %w", provider, err)
	}
	var tok Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return Token{}, fmt.Errorf("decode token %s: %w", provider, err)
	}
	return tok, nil
}

// Save writes tok for provider to disk atomically at 0600.
func (s *TokenStore) Save(provider string, tok Token) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create token dir: %w", err)
	}
	data, err := json.Marshal(tok) //nolint:gosec // token files hold credentials by design; written 0600
	if err != nil {
		return fmt.Errorf("encode token %s: %w", provider, err)
	}
	return atomicWrite600(s.tokenPath(provider), data)
}

// atomicWrite600 writes data to path via temp+rename with 0600 permissions.
func atomicWrite600(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-token-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// cleanup removes the temp file on error; its own errors are not actionable.
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}

// WithLock acquires an exclusive flock on provider's lock file,
// runs fn, then releases the lock. The lock is also released by the OS
// if the process exits unexpectedly.
func (s *TokenStore) WithLock(provider string, fn func() error) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create token dir: %w", err)
	}
	lf, err := os.OpenFile(s.lockPath(provider), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open lock file %s: %w", provider, err)
	}
	defer func() { _ = lf.Close() }()

	if err := unix.Flock(int(lf.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("acquire lock %s: %w", provider, err)
	}
	defer unix.Flock(int(lf.Fd()), unix.LOCK_UN) //nolint:errcheck

	return fn()
}

// EnsureFresh returns a valid (non-expired) token for provider.
//
// The refresh discipline for single-use rotating tokens (Whoop, Withings):
//  1. Load token; if ExpiresAt > now+60s, return it immediately.
//  2. Acquire exclusive flock for provider.
//  3. Re-read token from disk (another process may have refreshed while waiting).
//  4. Re-check expiry; if now valid, return it.
//  5. Call refresh(oldToken) to obtain a new token.
//  6. Persist the new token to disk BEFORE returning it.
//
// The refresh function is provider-agnostic: callers supply the HTTP exchange.
func (s *TokenStore) EnsureFresh(provider string, refresh func(old Token) (Token, error)) (Token, error) {
	// Step 1: optimistic load without lock.
	tok, err := s.Load(provider)
	if err == nil && tok.ExpiresAt.After(time.Now().Add(60*time.Second)) {
		return tok, nil
	}

	// Token is missing or stale. Acquire lock and double-check.
	var result Token
	lockErr := s.WithLock(provider, func() error {
		// Step 3: re-read under lock.
		reTok, reErr := s.Load(provider)
		if reErr == nil && reTok.ExpiresAt.After(time.Now().Add(60*time.Second)) {
			// Another process refreshed while we waited — use its token.
			result = reTok
			return nil
		}

		// Step 5: perform the refresh.
		old := reTok
		if reErr != nil {
			// If file is missing, carry forward whatever we loaded before.
			old = tok
		}
		newTok, refreshErr := refresh(old)
		if refreshErr != nil {
			return fmt.Errorf("refresh token %s: %w", provider, refreshErr)
		}

		// Step 6: persist BEFORE returning.
		if saveErr := s.Save(provider, newTok); saveErr != nil {
			return fmt.Errorf("save refreshed token %s: %w", provider, saveErr)
		}
		result = newTok
		return nil
	})
	if lockErr != nil {
		return Token{}, lockErr
	}
	return result, nil
}
