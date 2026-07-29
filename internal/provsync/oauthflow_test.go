// ABOUTME: Tests for the OAuth2 localhost-callback flow helper.
// ABOUTME: Uses httptest servers for all HTTP; no real provider calls.
package provsync

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// --- helpers ---

func newTestStore(t *testing.T) *TokenStore {
	t.Helper()
	dir, err := os.MkdirTemp("", "oauthflow-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return NewTokenStore(dir)
}

// freePort binds :0 and returns the OS-assigned port number, then closes the listener.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// extractStateFromOutput scans text for a line containing a URL with a "state=" param
// and returns the state value.
func extractStateFromOutput(text string) (string, error) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "http") {
			continue
		}
		u, err := url.Parse(line)
		if err != nil {
			continue
		}
		if s := u.Query().Get("state"); s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("no state found in output: %q", text)
}

// --- state generation ---

func TestRandomState(t *testing.T) {
	s1, err := randomState()
	if err != nil {
		t.Fatalf("randomState: %v", err)
	}
	if len(s1) != 32 {
		t.Errorf("state length = %d, want 32 hex chars", len(s1))
	}
	s2, _ := randomState()
	if s1 == s2 {
		t.Error("two consecutive states should not be equal")
	}
}

// --- authorize URL construction ---

func TestBuildAuthorizeURL(t *testing.T) {
	f := &OAuthFlow{
		provider: "whoop",
		urls: AuthorizeURLs{
			AuthURL:     "https://api.prod.whoop.com/oauth/oauth2/auth",
			ClientID:    "myclient",
			RedirectURI: "http://localhost:42021/callback",
			Scopes:      "read:recovery offline",
		},
	}
	u := f.buildAuthorizeURL("teststate123")
	if !strings.Contains(u, "client_id=myclient") {
		t.Errorf("URL missing client_id: %s", u)
	}
	if !strings.Contains(u, "state=teststate123") {
		t.Errorf("URL missing state: %s", u)
	}
	if !strings.Contains(u, "response_type=code") {
		t.Errorf("URL missing response_type: %s", u)
	}
}

// --- resolveListenerAddr ---

func TestResolveListenerAddrExplicit(t *testing.T) {
	f := &OAuthFlow{listenerAddr: ":9999"}
	addr, err := f.resolveListenerAddr()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != ":9999" {
		t.Errorf("got %q, want %q", addr, ":9999")
	}
}

func TestResolveListenerAddrFromRedirectURI(t *testing.T) {
	f := &OAuthFlow{
		urls: AuthorizeURLs{RedirectURI: "http://localhost:42021/callback"},
	}
	addr, err := f.resolveListenerAddr()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != ":42021" {
		t.Errorf("got %q, want :42021", addr)
	}
}

func TestResolveListenerAddrNoPort(t *testing.T) {
	f := &OAuthFlow{
		urls: AuthorizeURLs{RedirectURI: "http://localhost/callback"},
	}
	_, err := f.resolveListenerAddr()
	if err == nil {
		t.Error("expected error for URI with no port")
	}
}

// --- port already taken ---

func TestPortAlreadyTaken(t *testing.T) {
	port := freePort(t)
	listenAddr := fmt.Sprintf(":%d", port)

	// Hold the port.
	occupier, err := net.Listen("tcp", listenAddr)
	if err != nil {
		t.Fatalf("failed to occupy port: %v", err)
	}
	defer occupier.Close()

	store := newTestStore(t)
	urls := AuthorizeURLs{
		AuthURL:     "https://example.com/auth",
		ClientID:    "c",
		RedirectURI: fmt.Sprintf("http://localhost:%d/callback", port),
	}
	f := NewOAuthFlow("whoop", urls, store, nil, listenAddr)
	err = f.Run(io.Discard)
	if err == nil {
		t.Fatal("expected error when port is taken")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Errorf("error should mention 'already in use', got: %v", err)
	}
}

// --- callback: state mismatch → error, no exchange ---

func TestCallbackStateMismatch(t *testing.T) {
	port := freePort(t)
	listenAddr := fmt.Sprintf(":%d", port)
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	exchangeCalled := false
	mockExchange := func(tokenURL, clientID, clientSecret, redirectURI, code string) (Token, error) {
		exchangeCalled = true
		return Token{}, nil
	}

	store := newTestStore(t)
	urls := AuthorizeURLs{
		AuthURL:     "https://example.com/auth",
		ClientID:    "c",
		RedirectURI: redirectURI,
	}
	f := NewOAuthFlow("whoop", urls, store, mockExchange, listenAddr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- f.Run(io.Discard)
	}()

	// Wait briefly then hit the callback with a wrong state.
	time.Sleep(80 * time.Millisecond)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/callback?code=THECODE&state=WRONGSTATE", port))
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	resp.Body.Close()

	select {
	case runErr := <-errCh:
		if runErr == nil {
			t.Fatal("expected error on state mismatch")
		}
		if !strings.Contains(runErr.Error(), "state mismatch") {
			t.Errorf("error should mention state mismatch, got: %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Run to return")
	}

	if exchangeCalled {
		t.Error("exchange should NOT be called on state mismatch")
	}
}

// --- callback: correct state → exchange called → token persisted ---

func TestCallbackSuccessWhoopExchange(t *testing.T) {
	// Set up a fake Whoop token server that validates the exchange params.
	exchangeHit := false
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form", 400)
			return
		}
		if r.FormValue("grant_type") != "authorization_code" {
			http.Error(w, "wrong grant_type", 400)
			return
		}
		exchangeHit = true
		json.NewEncoder(w).Encode(tokenRefreshResponse{
			AccessToken:  "access-abc",
			RefreshToken: "refresh-xyz",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		})
	}))
	defer tokenServer.Close()

	port := freePort(t)
	listenAddr := fmt.Sprintf(":%d", port)
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	store := newTestStore(t)
	urls := AuthorizeURLs{
		AuthURL:      "https://example.com/auth",
		TokenURL:     tokenServer.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  redirectURI,
		Scopes:       "read:recovery offline",
	}

	f := NewOAuthFlow("whoop", urls, store, ExchangeWhoop, listenAddr)

	// Capture the authorize URL to extract the real state.
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		errCh <- f.Run(pw)
		pw.Close()
	}()

	// Read enough output to capture the URL line.
	buf := make([]byte, 8192)
	n, _ := pr.Read(buf)
	pr.Close()
	output := string(buf[:n])

	gotState, err := extractStateFromOutput(output)
	if err != nil {
		t.Fatalf("extractStateFromOutput: %v (output: %q)", err, output)
	}

	// Trigger the callback with the correct state.
	cbURL := fmt.Sprintf("http://localhost:%d/callback?code=MYCODE&state=%s", port, gotState)
	resp, err := http.Get(cbURL)
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	resp.Body.Close()

	select {
	case runErr := <-errCh:
		if runErr != nil {
			t.Fatalf("Run returned error: %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Run to return")
	}

	if !exchangeHit {
		t.Error("exchange endpoint was never called")
	}

	// Verify token was persisted to the store.
	tok, err := store.Load("whoop")
	if err != nil {
		t.Fatalf("Load token: %v", err)
	}
	if tok.AccessToken != "access-abc" {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, "access-abc")
	}
	if tok.RefreshToken != "refresh-xyz" {
		t.Errorf("RefreshToken = %q, want %q", tok.RefreshToken, "refresh-xyz")
	}
}

// --- Withings exchange: nonzero status is an error ---

func TestExchangeWithingsNonzeroStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(withingsTokenEnvelope{
			Status: 401,
			Body:   withingsTokenPayload{},
		})
	}))
	defer srv.Close()

	_, err := ExchangeWithings(srv.URL, "c", "s", "http://localhost/cb", "code")
	if err == nil {
		t.Fatal("expected error for Withings status != 0")
	}
	if !strings.Contains(err.Error(), "API status 401") {
		t.Errorf("error should mention API status, got: %v", err)
	}
}

// --- Withings exchange: sends action=requesttoken ---

func TestExchangeWithingsSendsCorrectParams(t *testing.T) {
	var gotAction, gotGrantType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse", 400)
			return
		}
		gotAction = r.FormValue("action")
		gotGrantType = r.FormValue("grant_type")
		json.NewEncoder(w).Encode(withingsTokenEnvelope{
			Status: 0,
			Body: withingsTokenPayload{
				AccessToken:  "acc",
				RefreshToken: "ref",
				ExpiresIn:    10800,
				TokenType:    "Bearer",
			},
		})
	}))
	defer srv.Close()

	tok, err := ExchangeWithings(srv.URL, "c", "s", "http://localhost/cb", "mycode")
	if err != nil {
		t.Fatalf("ExchangeWithings: %v", err)
	}
	if gotAction != "requesttoken" {
		t.Errorf("action = %q, want %q", gotAction, "requesttoken")
	}
	if gotGrantType != "authorization_code" {
		t.Errorf("grant_type = %q, want %q", gotGrantType, "authorization_code")
	}
	if tok.AccessToken != "acc" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
}

// --- Whoop exchange: sends client creds + grant_type ---

func TestExchangeWhoopSendsCorrectParams(t *testing.T) {
	var gotGrantType, gotClientID, gotClientSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse", 400)
			return
		}
		gotGrantType = r.FormValue("grant_type")
		gotClientID = r.FormValue("client_id")
		gotClientSecret = r.FormValue("client_secret")
		json.NewEncoder(w).Encode(tokenRefreshResponse{
			AccessToken:  "wacc",
			RefreshToken: "wref",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		})
	}))
	defer srv.Close()

	tok, err := ExchangeWhoop(srv.URL, "myid", "mysecret", "http://localhost/cb", "mycode")
	if err != nil {
		t.Fatalf("ExchangeWhoop: %v", err)
	}
	if gotGrantType != "authorization_code" {
		t.Errorf("grant_type = %q, want %q", gotGrantType, "authorization_code")
	}
	if gotClientID != "myid" {
		t.Errorf("client_id = %q, want %q", gotClientID, "myid")
	}
	if gotClientSecret != "mysecret" {
		t.Errorf("client_secret = %q, want %q", gotClientSecret, "mysecret")
	}
	if tok.AccessToken != "wacc" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
}
