// ABOUTME: Tests for the provsync token store with flock-based serialized refresh.
// ABOUTME: Covers Save/Load round-trip, 0600 perms, and concurrent-refresh safety.
package provsync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenSaveLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewTokenStore(dir)

	tok := Token{
		AccessToken:  "acc-abc",
		RefreshToken: "ref-xyz",
		ExpiresAt:    time.Now().Add(time.Hour).Truncate(time.Second),
		TokenType:    "bearer",
	}

	if err := store.Save("whoop", tok); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("whoop")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.AccessToken != tok.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, tok.AccessToken)
	}
	if loaded.RefreshToken != tok.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, tok.RefreshToken)
	}
	if !loaded.ExpiresAt.Equal(tok.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", loaded.ExpiresAt, tok.ExpiresAt)
	}
	if loaded.TokenType != tok.TokenType {
		t.Errorf("TokenType = %q, want %q", loaded.TokenType, tok.TokenType)
	}
}

func TestTokenFilePerms(t *testing.T) {
	dir := t.TempDir()
	store := NewTokenStore(dir)

	tok := Token{
		AccessToken:  "acc",
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(time.Hour),
		TokenType:    "bearer",
	}
	if err := store.Save("withings", tok); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(store.tokenPath("withings"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token file perms = %o, want 0600", perm)
	}
}

func TestTokenLoadMissing(t *testing.T) {
	dir := t.TempDir()
	store := NewTokenStore(dir)

	_, err := store.Load("nonexistent")
	if err == nil {
		t.Fatal("Load of missing token should return error")
	}
}

func TestTokenWithLock(t *testing.T) {
	dir := t.TempDir()
	store := NewTokenStore(dir)

	var callCount int
	err := store.WithLock("whoop", func() error {
		callCount++
		return nil
	})
	if err != nil {
		t.Fatalf("WithLock: %v", err)
	}
	if callCount != 1 {
		t.Errorf("fn called %d times, want 1", callCount)
	}
}

func TestEnsureFreshUsesExisting(t *testing.T) {
	dir := t.TempDir()
	store := NewTokenStore(dir)

	// Token valid for well over 60s.
	fresh := Token{
		AccessToken:  "fresh-acc",
		RefreshToken: "fresh-ref",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
		TokenType:    "bearer",
	}
	if err := store.Save("whoop", fresh); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var refreshCalled bool
	tok, err := store.EnsureFresh("whoop", func(old Token) (Token, error) {
		refreshCalled = true
		return Token{}, fmt.Errorf("should not be called")
	})
	if err != nil {
		t.Fatalf("EnsureFresh: %v", err)
	}
	if refreshCalled {
		t.Error("refresh func called for a still-valid token")
	}
	if tok.AccessToken != "fresh-acc" {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, "fresh-acc")
	}
}

// TestConcurrentRefreshExactlyOnce is the critical concurrency test.
// Two goroutines call EnsureFresh with an expired token simultaneously.
// Only ONE refresh HTTP call must happen, and the rotated token must be on disk.
func TestConcurrentRefreshExactlyOnce(t *testing.T) {
	dir := t.TempDir()

	// httptest server that counts refresh calls and returns a rotated token.
	var refreshCount atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCount.Add(1)
		// Simulate slight delay to make races more likely.
		time.Sleep(20 * time.Millisecond)
		resp := map[string]interface{}{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	store := NewTokenStore(dir)

	// Expired token on disk.
	expired := Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-10 * time.Minute), // expired
		TokenType:    "bearer",
	}
	if err := store.Save("whoop", expired); err != nil {
		t.Fatalf("Save expired token: %v", err)
	}

	refreshFn := func(_ Token) (Token, error) {
		resp, err := http.Get(srv.URL + "/token")
		if err != nil {
			return Token{}, fmt.Errorf("refresh request: %w", err)
		}
		defer resp.Body.Close()
		var body struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return Token{}, fmt.Errorf("decode refresh response: %w", err)
		}
		return Token{
			AccessToken:  body.AccessToken,
			RefreshToken: body.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(body.ExpiresIn) * time.Second),
			TokenType:    "bearer",
		}, nil
	}

	var wg sync.WaitGroup
	results := make([]Token, 2)
	errs := make([]error, 2)

	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			results[i], errs[i] = store.EnsureFresh("whoop", refreshFn)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d EnsureFresh error: %v", i, err)
		}
	}

	count := refreshCount.Load()
	if count != 1 {
		t.Errorf("refresh called %d times, want exactly 1", count)
	}

	// Both goroutines must have gotten the new access token.
	for i, tok := range results {
		if tok.AccessToken != "new-access" {
			t.Errorf("goroutine %d: AccessToken = %q, want %q", i, tok.AccessToken, "new-access")
		}
	}

	// Token on disk must hold the rotated refresh token.
	onDisk, err := store.Load("whoop")
	if err != nil {
		t.Fatalf("Load after concurrent refresh: %v", err)
	}
	if onDisk.RefreshToken != "new-refresh" {
		t.Errorf("disk RefreshToken = %q, want %q", onDisk.RefreshToken, "new-refresh")
	}
	if onDisk.AccessToken != "new-access" {
		t.Errorf("disk AccessToken = %q, want %q", onDisk.AccessToken, "new-access")
	}
}
