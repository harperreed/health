// ABOUTME: OAuth2 localhost-callback flow helper for Whoop and Withings authorization.
// ABOUTME: Starts a local HTTP server, prints the authorize URL, captures the code, exchanges it for a token.
package provsync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AuthorizeURLs holds the URL parameters needed to kick off an OAuth flow.
type AuthorizeURLs struct {
	// AuthURL is the provider's browser-facing authorization endpoint.
	AuthURL string
	// TokenURL is the endpoint used for the code→token exchange.
	TokenURL string
	// ClientID identifies the app.
	ClientID string
	// ClientSecret authenticates the app on token exchange.
	ClientSecret string
	// RedirectURI is the callback URL; the local server must listen on its port.
	RedirectURI string
	// Scopes is the space- or comma-separated scope string sent to the provider.
	Scopes string
}

// OAuthCallbackResult holds a successfully captured authorization code.
type OAuthCallbackResult struct {
	Code  string
	State string
}

// OAuthFlow manages the localhost-callback OAuth2 authorization flow.
// Construct with NewOAuthFlow; call Run to drive the entire exchange and persist the token.
type OAuthFlow struct {
	provider     string // e.g. "whoop" or "withings"
	urls         AuthorizeURLs
	store        *TokenStore
	exchangeFunc exchangeFn // injectable for tests
	listenerAddr string     // host:port override, empty → parse from RedirectURI
}

// exchangeFn is the function signature for a code→token exchange.
// Injected so tests can drive against httptest servers without real HTTP.
type exchangeFn func(tokenURL, clientID, clientSecret, redirectURI, code string) (Token, error)

// NewOAuthFlow returns an OAuthFlow for the named provider.
// provider is "whoop" or "withings". urls carries all the provider-specific parameters.
// store is used to persist the obtained token. exchangeFunc selects the exchange strategy:
// use ExchangeWhoop for Whoop (standard form POST), ExchangeWithings for Withings
// (nonstandard {status,body} envelope). listenerAddr overrides the listen address
// parsed from RedirectURI — useful in tests to bind :0 and then get the actual port.
func NewOAuthFlow(provider string, urls AuthorizeURLs, store *TokenStore, exchangeFunc exchangeFn, listenerAddr string) *OAuthFlow {
	return &OAuthFlow{
		provider:     provider,
		urls:         urls,
		store:        store,
		exchangeFunc: exchangeFunc,
		listenerAddr: listenerAddr,
	}
}

// Run executes the full authorization flow:
//  1. Generates a random state parameter.
//  2. Checks the callback port is free (errors clearly if taken).
//  3. Prints the authorize URL to stdout (mandatory for remote/tailscale use).
//  4. Starts a local HTTP server on RedirectURI's port to capture the callback.
//  5. Exchanges the captured code for a token via the provider-specific exchange.
//  6. Persists the token via the TokenStore.
//  7. Prints a success message and shuts the listener down cleanly.
func (f *OAuthFlow) Run(stdout io.Writer) error {
	state, err := randomState()
	if err != nil {
		return fmt.Errorf("oauthflow: generate state: %w", err)
	}

	listenAddr, err := f.resolveListenerAddr()
	if err != nil {
		return err
	}

	// Check port is free before printing the URL — avoids a confusing experience
	// where the URL is printed but the listener immediately fails.
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("oauthflow: callback port already in use (%s): %w", listenAddr, err)
	}
	// Keep ln open for the HTTP server — close only on error before serving.

	authorizeURL := f.buildAuthorizeURL(state)

	_, _ = fmt.Fprintf(stdout, "\nOpen this URL in your browser to authorize %s:\n\n  %s\n\n", f.provider, authorizeURL)
	_, _ = fmt.Fprintf(stdout, "Waiting for OAuth callback...\n")

	// Capture code + state from the callback; exchange; persist.
	codeCh := make(chan OAuthCallbackResult, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotState := q.Get("state")
		code := q.Get("code")
		errParam := q.Get("error")

		if errParam != "" {
			errDesc := q.Get("error_description")
			msg := fmt.Sprintf("oauthflow: provider returned error: %s: %s", errParam, errDesc)
			http.Error(w, "Authorization failed. You may close this tab.", http.StatusBadRequest)
			errCh <- fmt.Errorf("%s", msg)
			return
		}
		if gotState != state {
			http.Error(w, "State mismatch — possible CSRF. You may close this tab.", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauthflow: state mismatch (got %q, want %q)", gotState, state)
			return
		}
		if code == "" {
			http.Error(w, "No code in callback. You may close this tab.", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauthflow: callback contained no code")
			return
		}

		_, _ = fmt.Fprintf(w, "Authorization successful! You may close this tab.\n")
		codeCh <- OAuthCallbackResult{Code: code, State: gotState}
	})
	srv.Handler = mux

	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- fmt.Errorf("oauthflow: callback server: %w", serveErr)
		}
	}()

	// Wait for code or error from callback.
	var result OAuthCallbackResult
	select {
	case result = <-codeCh:
	case err = <-errCh:
		_ = srv.Close()
		return err
	case <-time.After(5 * time.Minute):
		_ = srv.Close()
		return fmt.Errorf("oauthflow: timed out waiting for callback after 5 minutes")
	}

	// Shut the server down cleanly — we have the code.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	// Exchange code for token.
	tok, err := f.exchangeFunc(f.urls.TokenURL, f.urls.ClientID, f.urls.ClientSecret, f.urls.RedirectURI, result.Code)
	if err != nil {
		return fmt.Errorf("oauthflow: %s token exchange: %w", f.provider, err)
	}

	// Persist token.
	if err := f.store.Save(f.provider, tok); err != nil {
		return fmt.Errorf("oauthflow: save token: %w", err)
	}

	_, _ = fmt.Fprintf(stdout, "\nAuthorization complete. Token for %s saved to token store.\n", f.provider)
	return nil
}

// resolveListenerAddr returns the host:port to listen on.
// If the flow was constructed with an explicit listenerAddr, use it.
// Otherwise parse the port from the RedirectURI.
func (f *OAuthFlow) resolveListenerAddr() (string, error) {
	if f.listenerAddr != "" {
		return f.listenerAddr, nil
	}
	u, err := url.Parse(f.urls.RedirectURI)
	if err != nil {
		return "", fmt.Errorf("oauthflow: parse redirect URI %q: %w", f.urls.RedirectURI, err)
	}
	port := u.Port()
	if port == "" {
		return "", fmt.Errorf("oauthflow: redirect URI %q has no port", f.urls.RedirectURI)
	}
	return ":" + port, nil
}

// buildAuthorizeURL constructs the browser-facing authorization URL.
func (f *OAuthFlow) buildAuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", f.urls.ClientID)
	q.Set("redirect_uri", f.urls.RedirectURI)
	q.Set("state", state)
	q.Set("scope", f.urls.Scopes)
	// url.Values.Encode encodes spaces as '+', which is only valid inside
	// application/x-www-form-urlencoded bodies. RFC 3986 requires '%20' in
	// URL query strings. Replace every bare '+' with '%20' — this is safe
	// because Encode already encodes any literal '+' in values as '%2B', so
	// every remaining '+' in the encoded string represents a space.
	encoded := strings.ReplaceAll(q.Encode(), "+", "%20")
	return f.urls.AuthURL + "?" + encoded
}

// randomState generates a 16-byte hex-encoded random state parameter.
func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// --- Exchange functions (injectable for tests) ---

// ExchangeWhoop performs a standard OAuth2 authorization_code exchange
// against the Whoop token endpoint (form POST, standard JSON response).
// Per api-research.md: scopes include "offline" for refresh token grant.
func ExchangeWhoop(tokenURL, clientID, clientSecret, redirectURI, code string) (Token, error) {
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("code", code)
	params.Set("client_id", clientID)
	params.Set("client_secret", clientSecret)
	params.Set("redirect_uri", redirectURI)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(tokenURL, params)
	if err != nil {
		return Token{}, fmt.Errorf("whoop exchange: POST token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("whoop exchange: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("whoop exchange: status %d: %s", resp.StatusCode, truncateBody(body))
	}

	var r tokenRefreshResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return Token{}, fmt.Errorf("whoop exchange: decode response: %w", err)
	}

	tokenType := r.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	expiresIn := r.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}

	return Token{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		TokenType:    tokenType,
	}, nil
}

// ExchangeWithings performs the nonstandard Withings authorization_code exchange.
// Per api-research.md: POST to /v2/oauth2 with action=requesttoken; response is
// {"status": 0, "body": {...}} — nonzero status is an application-level error.
func ExchangeWithings(tokenURL, clientID, clientSecret, redirectURI, code string) (Token, error) {
	params := url.Values{}
	params.Set("action", "requesttoken")
	params.Set("grant_type", "authorization_code")
	params.Set("client_id", clientID)
	params.Set("client_secret", clientSecret)
	params.Set("code", code)
	params.Set("redirect_uri", redirectURI)

	client := &http.Client{Timeout: 30 * time.Second}

	encoded := params.Encode()
	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(encoded))
	if err != nil {
		return Token{}, fmt.Errorf("withings exchange: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ContentLength = int64(len(encoded))

	resp, err := client.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("withings exchange: POST token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("withings exchange: read body: %w", err)
	}

	// Withings always returns HTTP 200; errors signal via status != 0.
	var env withingsTokenEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Token{}, fmt.Errorf("withings exchange: decode response: %w", err)
	}
	if env.Status != 0 {
		return Token{}, fmt.Errorf("withings exchange: API status %d", env.Status)
	}

	tokenType := env.Body.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	expiresIn := env.Body.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 10800
	}

	return Token{
		AccessToken:  env.Body.AccessToken,
		RefreshToken: env.Body.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		TokenType:    tokenType,
	}, nil
}

// --- Production auth URL constants (verified per api-research.md) ---

const (
	// WhoopAuthorizeURL is the browser-facing OAuth2 authorization endpoint.
	// Scopes required: read:recovery read:sleep read:cycles offline
	// (offline is REQUIRED to receive a refresh token).
	WhoopAuthorizeURL = "https://api.prod.whoop.com/oauth/oauth2/auth"

	// WhoopScopes are the OAuth2 scopes requested for Whoop.
	WhoopScopes = "read:recovery read:sleep read:cycles offline"

	// WithingsAuthorizeURL is the browser-facing OAuth2 authorization endpoint.
	WithingsAuthorizeURL = "https://account.withings.com/oauth2_user/authorize2"

	// WithingsScopes are the OAuth2 scopes requested for Withings (comma-separated).
	WithingsScopes = "user.info,user.metrics,user.activity"

	// DefaultRedirectURI is the default callback URI used when no RedirectURI is configured.
	DefaultRedirectURI = "http://localhost:42021/callback"
)
