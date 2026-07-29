// ABOUTME: Tests for the Emfit QS provider: fixtures, datestamp shapes, login vs token path, idempotency.
// ABOUTME: All network calls go to injected httptest servers; uses a real SQLite repo.
package provsync

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/harperreed/health/internal/models"
)

// --- fixture helpers ---

// emfitLatestResponse builds a minimal /api/v1/presence/{id}/latest JSON body.
// datestamps is the raw JSON array for minitrend_datestamps.
func emfitLatestResponse(datestamps string) string {
	return fmt.Sprintf(`{
		"id": 42,
		"sleep_duration": 25920,
		"hrv_rmssd_morning": 55.3,
		"measured_hr_avg": 58.0,
		"measured_rr_avg": 14.2,
		"minitrend_datestamps": %s
	}`, datestamps)
}

// objectDatestamps returns minitrend_datestamps as an array of {ts: N} objects.
// The last element's ts is the one that should be used as recorded_at.
func objectDatestamps(ts int64) string {
	return fmt.Sprintf(`[{"ts": 1700000000}, {"ts": %d}]`, ts)
}

// numberDatestamps returns minitrend_datestamps as an array of bare unix numbers.
func numberDatestamps(ts int64) string {
	return fmt.Sprintf(`[1700000000, %d]`, ts)
}

// emfitServer builds a test server serving the latest presence endpoint.
func emfitServer(t *testing.T, deviceID string, responseBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/get", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"user":{"id":1}}`)
	})
	mux.HandleFunc(fmt.Sprintf("/api/v1/presence/%s/latest", deviceID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, responseBody)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// loginServer builds a test server that handles login + data calls.
// loginBody is the JSON response to POST /api/v1/login.
func loginServer(t *testing.T, deviceID, loginBody, presenceBody string) (*httptest.Server, *int32) {
	t.Helper()
	var loginCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		atomic.AddInt32(&loginCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, loginBody)
	})
	mux.HandleFunc("/api/v1/user/get", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"user":{"id":1}}`)
	})
	mux.HandleFunc(fmt.Sprintf("/api/v1/presence/%s/latest", deviceID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, presenceBody)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &loginCalls
}

// --- tests ---

// TestEmfitSyncMetricsObjectDatestamps verifies the four metrics with correct
// values/units/source using object-shaped minitrend_datestamps [{ts: N}, ...].
func TestEmfitSyncMetricsObjectDatestamps(t *testing.T) {
	const deviceID = "dev-001"
	const wantTS = int64(1753617600) // some unix timestamp

	body := emfitLatestResponse(objectDatestamps(wantTS))
	srv := emfitServer(t, deviceID, body)

	repo := setupTestRepo(t)
	client := NewEmfitClient(srv.URL, "test-token", deviceID)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("metric count = %d, want 4", len(all))
	}

	byType := map[models.MetricType]*models.Metric{}
	for _, m := range all {
		byType[m.MetricType] = m
	}

	// sleep_duration 25920s / 3600 = 7.2h
	checkEmfitMetric(t, byType, models.MetricSleepHours, 7.2, "hours")
	checkEmfitMetric(t, byType, models.MetricHRV, 55.3, "ms")
	checkEmfitMetric(t, byType, models.MetricHeartRate, 58.0, "bpm")
	checkEmfitMetric(t, byType, models.MetricRespiratoryRate, 14.2, "brpm")

	// recorded_at must be the LAST element of minitrend_datestamps.
	wantRecordedAt := time.Unix(wantTS, 0)
	for mt, m := range byType {
		if !m.RecordedAt.Equal(wantRecordedAt) {
			t.Errorf("%s recorded_at = %v, want %v", mt, m.RecordedAt, wantRecordedAt)
		}
	}
}

// TestEmfitSyncMetricsNumberDatestamps verifies the four metrics with bare-number
// minitrend_datestamps (array of unix ints, not objects).
func TestEmfitSyncMetricsNumberDatestamps(t *testing.T) {
	const deviceID = "dev-002"
	const wantTS = int64(1753620000)

	body := emfitLatestResponse(numberDatestamps(wantTS))
	srv := emfitServer(t, deviceID, body)

	repo := setupTestRepo(t)
	client := NewEmfitClient(srv.URL, "test-token", deviceID)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("metric count = %d, want 4", len(all))
	}

	wantRecordedAt := time.Unix(wantTS, 0)
	for _, m := range all {
		if !m.RecordedAt.Equal(wantRecordedAt) {
			t.Errorf("%s recorded_at = %v, want %v", m.MetricType, m.RecordedAt, wantRecordedAt)
		}
	}
}

// TestEmfitSyncMissingDeviceIDErrors verifies that a missing device_id produces
// a clear error and makes NO HTTP calls.
func TestEmfitSyncMissingDeviceIDErrors(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	repo := setupTestRepo(t)
	// device_id is empty string — must error immediately, no HTTP calls.
	client := NewEmfitClient(srv.URL, "tok", "")

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	err := client.Sync(repo, start, end)
	if err == nil {
		t.Fatal("expected error for missing device_id, got nil")
	}
	if !strings.Contains(err.Error(), "device_id") {
		t.Errorf("error %q does not mention device_id", err.Error())
	}
	if calls := atomic.LoadInt32(&callCount); calls != 0 {
		t.Errorf("HTTP calls = %d, want 0 (no call when device_id missing)", calls)
	}
}

// TestEmfitSyncConfigTokenPath verifies that when a token is in the constructor,
// it is sent as Authorization: Bearer <token> without a login call.
func TestEmfitSyncConfigTokenPath(t *testing.T) {
	const deviceID = "dev-003"
	const configToken = "my-config-token"
	const wantTS = int64(1753700000) // distinct from other tests to satisfy unparam

	var gotAuth string
	body := emfitLatestResponse(objectDatestamps(wantTS))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		t.Error("login endpoint called — should not be called when token is configured")
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/api/v1/user/get", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"user":{"id":1}}`)
	})
	mux.HandleFunc(fmt.Sprintf("/api/v1/presence/%s/latest", deviceID), func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	repo := setupTestRepo(t)
	client := NewEmfitClient(srv.URL, configToken, deviceID)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	want := "Bearer " + configToken
	if gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
}

// TestEmfitSyncLoginPathTokenField verifies the login path: POST /api/v1/login
// with username/password, receive {token: "..."}, use as Bearer on data calls.
func TestEmfitSyncLoginPathTokenField(t *testing.T) {
	const deviceID = "dev-004"
	const username = "user@example.com"
	const password = "s3cr3t"
	const wantTS = int64(1753617600)

	body := emfitLatestResponse(objectDatestamps(wantTS))
	loginResp := `{"token": "login-token-123", "remember_token": "rem-tok"}`
	srv, loginCalls := loginServer(t, deviceID, loginResp, body)

	repo := setupTestRepo(t)
	// No pre-configured token → must login.
	client := NewEmfitClientWithLogin(srv.URL, username, password, deviceID)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if atomic.LoadInt32(loginCalls) != 1 {
		t.Errorf("login calls = %d, want 1", atomic.LoadInt32(loginCalls))
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("metric count = %d, want 4", len(all))
	}
}

// TestEmfitSyncLoginPathRememberTokenFallback verifies that when the login response
// has no "token" field but has "remember_token", the remember_token is used as Bearer.
func TestEmfitSyncLoginPathRememberTokenFallback(t *testing.T) {
	const deviceID = "dev-005"
	const wantTS = int64(1753617600)

	var gotAuth string
	body := emfitLatestResponse(objectDatestamps(wantTS))
	// No "token" field; only "remember_token".
	loginResp := `{"remember_token": "rem-tok-fallback"}`

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, loginResp)
	})
	mux.HandleFunc("/api/v1/user/get", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"user":{"id":1}}`)
	})
	mux.HandleFunc(fmt.Sprintf("/api/v1/presence/%s/latest", deviceID), func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	repo := setupTestRepo(t)
	client := NewEmfitClientWithLogin(srv.URL, "user", "pass", deviceID)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if gotAuth != "Bearer rem-tok-fallback" {
		t.Errorf("Authorization = %q, want Bearer rem-tok-fallback", gotAuth)
	}
}

// TestEmfitSyncIdempotency verifies re-syncing the same data produces no new rows.
func TestEmfitSyncIdempotency(t *testing.T) {
	const deviceID = "dev-006"
	const wantTS = int64(1753617600)

	body := emfitLatestResponse(objectDatestamps(wantTS))
	srv := emfitServer(t, deviceID, body)

	repo := setupTestRepo(t)
	client := NewEmfitClient(srv.URL, "tok", deviceID)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)

	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	first, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics after first sync: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("first sync wrote zero metrics — test setup is wrong")
	}

	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	second, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics after second sync: %v", err)
	}
	if len(second) != len(first) {
		t.Errorf("after re-sync count = %d, want %d (no new rows)", len(second), len(first))
	}
}

// TestEmfitSyncNon200Error verifies that a non-200 status from the API returns a wrapped error.
func TestEmfitSyncNon200Error(t *testing.T) {
	const deviceID = "dev-007"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal server error"}`)
	}))
	t.Cleanup(srv.Close)

	repo := setupTestRepo(t)
	client := NewEmfitClient(srv.URL, "tok", deviceID)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	err := client.Sync(repo, start, end)
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestEmfitSyncSetsSourceEmfit verifies every metric carries source=emfit.
func TestEmfitSyncSetsSourceEmfit(t *testing.T) {
	const deviceID = "dev-008"
	const wantTS = int64(1753617600)

	body := emfitLatestResponse(objectDatestamps(wantTS))
	srv := emfitServer(t, deviceID, body)

	repo := setupTestRepo(t)
	client := NewEmfitClient(srv.URL, "tok", deviceID)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) == 0 {
		t.Fatal("no metrics written")
	}
	for _, m := range all {
		if m.Source != models.SourceEmfit {
			t.Errorf("source = %q, want %q", m.Source, models.SourceEmfit)
		}
	}
}

// TestEmfitClientDefaultURLs documents the production base URL constant.
func TestEmfitClientDefaultURLs(t *testing.T) {
	if EmfitAPIBaseURL == "" {
		t.Error("EmfitAPIBaseURL must not be empty")
	}
}

// TestEmfitClientClose verifies no panic on Close.
func TestEmfitClientClose(t *testing.T) {
	client := NewEmfitClient("http://unused", "tok", "dev-000")
	client.Close()
}

// --- helpers ---

func checkEmfitMetric(t *testing.T, byType map[models.MetricType]*models.Metric, mt models.MetricType, wantVal float64, wantUnit string) {
	t.Helper()
	m, ok := byType[mt]
	if !ok {
		t.Errorf("missing metric type %q", mt)
		return
	}
	const epsilon = 1e-9
	diff := m.Value - wantVal
	if diff < -epsilon || diff > epsilon {
		t.Errorf("%s value = %v, want %v", mt, m.Value, wantVal)
	}
	if m.Unit != wantUnit {
		t.Errorf("%s unit = %q, want %q", mt, m.Unit, wantUnit)
	}
	if m.Source != models.SourceEmfit {
		t.Errorf("%s source = %q, want %q", mt, m.Source, models.SourceEmfit)
	}
}
