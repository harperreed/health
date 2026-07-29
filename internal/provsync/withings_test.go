// ABOUTME: Tests for the Withings provider: fixtures, value×10^unit math, token refresh, upsert idempotency.
// ABOUTME: All network calls go to injected httptest servers; uses a real SQLite repo.
package provsync

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/harperreed/health/internal/models"
)

// setupWithingsTokenStore seeds a token for withings provider key.
func setupWithingsTokenStore(t *testing.T, tok Token) *TokenStore {
	t.Helper()
	dir := t.TempDir()
	store := NewTokenStore(dir)
	if err := store.Save(withingsProviderKey, tok); err != nil {
		t.Fatalf("save withings token: %v", err)
	}
	return store
}

// --- value math tests ---

// TestWithingsValueMath verifies real_value = value × 10^unit for signed exponents.
func TestWithingsValueMath(t *testing.T) {
	cases := []struct {
		value    int64
		unit     int
		wantReal float64
	}{
		{82500, -3, 82.5},  // the brief's canonical example
		{78500, -3, 78.5},  // api-research.md example
		{22500, -2, 225.0}, // unit=-2
		{850, 0, 850.0},    // unit=0 (no scaling)
		{2, 1, 20.0},       // unit=+1
		{175, -1, 17.5},    // fat_ratio style
	}
	for _, tc := range cases {
		got := withingsRealValue(tc.value, tc.unit)
		if math.Abs(got-tc.wantReal) > 1e-9 {
			t.Errorf("withingsRealValue(%d, %d) = %v, want %v", tc.value, tc.unit, got, tc.wantReal)
		}
	}
}

// --- measure sync tests ---

// TestWithingsSyncMeasureWeight verifies type 1 → weight metric with correct value×10^unit math.
func TestWithingsSyncMeasureWeight(t *testing.T) {
	// value=82500, unit=-3 → 82.5 kg; recorded_at = group date (unix).
	const groupDate = 1753574400 // 2025-07-27T00:00:00Z

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/measure":
			resp := withingsMeasureResp([]withingsTestMeasGrp{
				{date: groupDate, measures: []withingsTestMeas{{value: 82500, typ: 1, unit: -3}}},
			})
			fmt.Fprint(w, resp)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	// Only weight; sleep response returns empty.
	var wm *models.Metric
	for _, m := range all {
		if m.MetricType == models.MetricWeight {
			wm = m
		}
	}
	if wm == nil {
		t.Fatal("missing weight metric")
	}
	if wm.Value != 82.5 {
		t.Errorf("weight value = %v, want 82.5", wm.Value)
	}
	if wm.Unit != "kg" {
		t.Errorf("weight unit = %q, want kg", wm.Unit)
	}
	if wm.Source != models.SourceWithings {
		t.Errorf("weight source = %q, want %q", wm.Source, models.SourceWithings)
	}
	wantTS := time.Unix(groupDate, 0)
	if !wm.RecordedAt.Equal(wantTS) {
		t.Errorf("weight recorded_at = %v, want %v", wm.RecordedAt, wantTS)
	}
}

// TestWithingsSyncMeasureBodyFat verifies type 6 → body_fat metric.
func TestWithingsSyncMeasureBodyFat(t *testing.T) {
	const groupDate = 1753574400

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/measure":
			resp := withingsMeasureResp([]withingsTestMeasGrp{
				{date: groupDate, measures: []withingsTestMeas{{value: 175, typ: 6, unit: -1}}},
			})
			fmt.Fprint(w, resp)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	var bfm *models.Metric
	for _, m := range all {
		if m.MetricType == models.MetricBodyFat {
			bfm = m
		}
	}
	if bfm == nil {
		t.Fatal("missing body_fat metric")
	}
	// 175 × 10^(-1) = 17.5
	if bfm.Value != 17.5 {
		t.Errorf("body_fat value = %v, want 17.5", bfm.Value)
	}
	if bfm.Unit != "%" {
		t.Errorf("body_fat unit = %q, want %%", bfm.Unit)
	}
}

// TestWithingsSyncMeasureBothTypes verifies a group with both type 1 and type 6.
func TestWithingsSyncMeasureBothTypes(t *testing.T) {
	const groupDate = 1753574400

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/measure":
			resp := withingsMeasureResp([]withingsTestMeasGrp{
				{date: groupDate, measures: []withingsTestMeas{
					{value: 82500, typ: 1, unit: -3},
					{value: 175, typ: 6, unit: -1},
				}},
			})
			fmt.Fprint(w, resp)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}

	byType := map[models.MetricType]*models.Metric{}
	for _, m := range all {
		byType[m.MetricType] = m
	}

	if wm, ok := byType[models.MetricWeight]; !ok {
		t.Error("missing weight metric")
	} else if wm.Value != 82.5 {
		t.Errorf("weight = %v, want 82.5", wm.Value)
	}

	if bfm, ok := byType[models.MetricBodyFat]; !ok {
		t.Error("missing body_fat metric")
	} else if bfm.Value != 17.5 {
		t.Errorf("body_fat = %v, want 17.5", bfm.Value)
	}
}

// TestWithingsSyncMeasureUnknownType verifies unknown measure types are silently skipped.
func TestWithingsSyncMeasureUnknownType(t *testing.T) {
	const groupDate = 1753574400

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/measure":
			// Type 9 = diastolic BP — not mapped, should be skipped.
			resp := withingsMeasureResp([]withingsTestMeasGrp{
				{date: groupDate, measures: []withingsTestMeas{{value: 120, typ: 9, unit: 0}}},
			})
			fmt.Fprint(w, resp)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("metric count = %d, want 0 (unknown type skipped)", len(all))
	}
}

// --- sleep sync tests ---

// TestWithingsSyncSleepHours verifies sleep summary → sleep_hours in hours (seconds/3600).
func TestWithingsSyncSleepHours(t *testing.T) {
	// light=7200s, deep=3600s, rem=1800s → total=12600s → 3.5h; enddate = recorded_at.
	const enddate = 1753617600 // some unix timestamp

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/sleep":
			resp := withingsSleepResp([]withingsTestSleepSeries{
				{enddate: enddate, light: 7200, deep: 3600, rem: 1800},
			})
			fmt.Fprint(w, resp)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{"measuregrps":[],"more":0,"offset":0}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}

	byType := map[models.MetricType]*models.Metric{}
	for _, m := range all {
		byType[m.MetricType] = m
	}

	sh, ok := byType[models.MetricSleepHours]
	if !ok {
		t.Fatal("missing sleep_hours metric")
	}
	want := (7200.0 + 3600.0 + 1800.0) / 3600.0 // 3.5
	if sh.Value != want {
		t.Errorf("sleep_hours = %v, want %v", sh.Value, want)
	}
	if sh.Unit != "hours" {
		t.Errorf("sleep_hours unit = %q, want hours", sh.Unit)
	}
	wantTS := time.Unix(enddate, 0)
	if !sh.RecordedAt.Equal(wantTS) {
		t.Errorf("sleep_hours recorded_at = %v, want %v", sh.RecordedAt, wantTS)
	}
	if sh.Source != models.SourceWithings {
		t.Errorf("sleep_hours source = %q, want %q", sh.Source, models.SourceWithings)
	}
}

// TestWithingsSyncSleepHRAndRR verifies hr_average → heart_rate and rr_average → respiratory_rate.
func TestWithingsSyncSleepHRAndRR(t *testing.T) {
	const enddate = 1753617600

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/sleep":
			// hr_average=58, rr_average=15
			resp := withingsSleepRespWithHRRR([]withingsTestSleepSeriesExt{
				{enddate: enddate, light: 14400, deep: 7200, rem: 3600, hrAvg: ptr(58), rrAvg: ptr(15)},
			})
			fmt.Fprint(w, resp)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{"measuregrps":[],"more":0,"offset":0}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	byType := map[models.MetricType]*models.Metric{}
	for _, m := range all {
		byType[m.MetricType] = m
	}

	if hr, ok := byType[models.MetricHeartRate]; !ok {
		t.Error("missing heart_rate metric")
	} else if hr.Value != 58 {
		t.Errorf("heart_rate = %v, want 58", hr.Value)
	}

	if rr, ok := byType[models.MetricRespiratoryRate]; !ok {
		t.Error("missing respiratory_rate metric")
	} else if rr.Value != 15 {
		t.Errorf("respiratory_rate = %v, want 15", rr.Value)
	}
}

// TestWithingsSyncSleepNoHRNoRR verifies that missing hr_average/rr_average fields don't produce metrics.
func TestWithingsSyncSleepNoHRNoRR(t *testing.T) {
	const enddate = 1753617600

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/sleep":
			resp := withingsSleepResp([]withingsTestSleepSeries{
				{enddate: enddate, light: 14400, deep: 7200, rem: 3600},
			})
			fmt.Fprint(w, resp)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{"measuregrps":[],"more":0,"offset":0}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	// Only sleep_hours; no heart_rate or respiratory_rate.
	for _, m := range all {
		if m.MetricType == models.MetricHeartRate || m.MetricType == models.MetricRespiratoryRate {
			t.Errorf("unexpected metric %s — should only appear when hr_average/rr_average present", m.MetricType)
		}
	}
	byType := map[models.MetricType]bool{}
	for _, m := range all {
		byType[m.MetricType] = true
	}
	if !byType[models.MetricSleepHours] {
		t.Error("missing sleep_hours metric")
	}
}

// --- token tests ---

// TestWithingsTokenEnvelope verifies the nonstandard {status,body} token response is parsed.
func TestWithingsTokenEnvelope(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		// Verify withings-specific params.
		if r.FormValue("action") != "requesttoken" {
			t.Errorf("action = %q, want requesttoken", r.FormValue("action"))
		}
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", r.FormValue("grant_type"))
		}
		resp := `{"status":0,"body":{"userid":"123","access_token":"acc-withings","refresh_token":"ref-withings","expires_in":10800,"token_type":"Bearer"}}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	t.Cleanup(tokenSrv.Close)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the refreshed access token is used.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer acc-withings" {
			t.Errorf("Authorization = %q, want Bearer acc-withings", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/measure":
			fmt.Fprintf(w, `{"status":0,"body":{"measuregrps":[],"more":0,"offset":0}}`)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, expiredToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, tokenSrv.URL, "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	tok, err := store.Load(withingsProviderKey)
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if tok.AccessToken != "acc-withings" {
		t.Errorf("access_token = %q, want acc-withings", tok.AccessToken)
	}
	if tok.RefreshToken != "ref-withings" {
		t.Errorf("refresh_token = %q, want ref-withings", tok.RefreshToken)
	}
}

// TestWithingsTokenNonzeroStatusIsError verifies nonzero status in token response → error.
func TestWithingsTokenNonzeroStatusIsError(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Withings returns HTTP 200 even for errors; status field carries the error code.
		fmt.Fprint(w, `{"status":401,"body":{}}`)
	}))
	t.Cleanup(tokenSrv.Close)

	// Token is expired to force a refresh attempt.
	store := setupWithingsTokenStore(t, expiredToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient("http://unused-api", tokenSrv.URL, "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	err := client.Sync(repo, start, end)
	if err == nil {
		t.Fatal("expected error for nonzero token status, got nil")
	}
}

// TestWithingsDataNonzeroStatusIsError verifies nonzero status in data response → error.
func TestWithingsDataNonzeroStatusIsError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// All Withings responses are HTTP 200; errors via status field.
		fmt.Fprint(w, `{"status":2555,"body":{}}`)
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	err := client.Sync(repo, start, end)
	if err == nil {
		t.Fatal("expected error for nonzero data status, got nil")
	}
}

// TestWithingsRefreshOnce verifies the serial path: an expired token triggers
// exactly one refresh call and the rotated token is persisted to the store.
// Concurrency proof (two goroutines must not double-refresh) lives in
// tokens_test.go TestConcurrentRefreshExactlyOnce, which owns the flock logic.
func TestWithingsRefreshOnce(t *testing.T) {
	var refreshCount int32

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&refreshCount, 1)
		resp := `{"status":0,"body":{"userid":"u","access_token":"acc-new","refresh_token":"ref-new","expires_in":10800,"token_type":"Bearer"}}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	t.Cleanup(tokenSrv.Close)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/measure":
			fmt.Fprintf(w, `{"status":0,"body":{"measuregrps":[],"more":0,"offset":0}}`)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	// Shared store — two clients point at the same store.
	dir := t.TempDir()
	store := NewTokenStore(dir)
	if err := store.Save(withingsProviderKey, expiredToken()); err != nil {
		t.Fatalf("seed token: %v", err)
	}

	client := NewWithingsClient(apiSrv.URL, tokenSrv.URL, "cid", "csec", store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	repo := setupTestRepo(t)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if atomic.LoadInt32(&refreshCount) != 1 {
		t.Errorf("refresh count = %d, want 1", refreshCount)
	}

	tok, err := store.Load(withingsProviderKey)
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if tok.RefreshToken != "ref-new" {
		t.Errorf("persisted refresh_token = %q, want ref-new", tok.RefreshToken)
	}
}

// TestWithingsSyncSetsSourceWithings verifies every metric carries source=withings.
func TestWithingsSyncSetsSourceWithings(t *testing.T) {
	const groupDate = 1753574400

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/measure":
			fmt.Fprint(w, withingsMeasureResp([]withingsTestMeasGrp{
				{date: groupDate, measures: []withingsTestMeas{{value: 82500, typ: 1, unit: -3}}},
			}))
		default:
			fmt.Fprintf(w, `{"status":0,"body":{}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

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
		if m.Source != models.SourceWithings {
			t.Errorf("source = %q, want %q", m.Source, models.SourceWithings)
		}
	}
}

// TestWithingsSyncIdempotency verifies that re-syncing the same window writes zero new rows.
func TestWithingsSyncIdempotency(t *testing.T) {
	const groupDate = 1753574400
	const enddate = 1753617600

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/measure":
			fmt.Fprint(w, withingsMeasureResp([]withingsTestMeasGrp{
				{date: groupDate, measures: []withingsTestMeas{
					{value: 82500, typ: 1, unit: -3},
					{value: 175, typ: 6, unit: -1},
				}},
			}))
		case "/v2/sleep":
			fmt.Fprint(w, withingsSleepResp([]withingsTestSleepSeries{
				{enddate: enddate, light: 14400, deep: 7200, rem: 3600},
			}))
		default:
			fmt.Fprintf(w, `{"status":0,"body":{}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, apiSrv.URL+"/v2/oauth2", "cid", "csec", store)

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

// TestWithingsClientDefaultURLs documents the production URL constants.
func TestWithingsClientDefaultURLs(t *testing.T) {
	if WithingsAPIBaseURL == "" {
		t.Error("WithingsAPIBaseURL must not be empty")
	}
	if WithingsTokenURL == "" {
		t.Error("WithingsTokenURL must not be empty")
	}
}

// TestWithingsRefreshSendsClientCredentials verifies client_id and client_secret are sent on refresh.
func TestWithingsRefreshSendsClientCredentials(t *testing.T) {
	const wantClientID = "test-withings-cid"
	const wantClientSecret = "test-withings-csec"

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("token endpoint: parse form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.FormValue("client_id") != wantClientID || r.FormValue("client_secret") != wantClientSecret {
			// Withings returns HTTP 200 with nonzero status for auth errors.
			fmt.Fprint(w, `{"status":401,"body":{}}`)
			return
		}
		resp := `{"status":0,"body":{"userid":"u","access_token":"acc-ok","refresh_token":"ref-ok","expires_in":10800,"token_type":"Bearer"}}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	}))
	t.Cleanup(tokenSrv.Close)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/measure":
			fmt.Fprintf(w, `{"status":0,"body":{"measuregrps":[],"more":0,"offset":0}}`)
		default:
			fmt.Fprintf(w, `{"status":0,"body":{}}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupWithingsTokenStore(t, expiredToken())
	repo := setupTestRepo(t)
	client := NewWithingsClient(apiSrv.URL, tokenSrv.URL, wantClientID, wantClientSecret, store)

	start := time.Date(2025, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 28, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync failed (token endpoint rejected credentials): %v", err)
	}

	tok, err := store.Load(withingsProviderKey)
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if tok.AccessToken != "acc-ok" {
		t.Errorf("access_token = %q, want acc-ok", tok.AccessToken)
	}
}

// TestWithingsClientClose verifies no panic on Close.
func TestWithingsClientClose(t *testing.T) {
	store := setupWithingsTokenStore(t, freshToken())
	client := NewWithingsClient("http://unused", "http://unused", "cid", "csec", store)
	client.Close()
}

// --- fixture helpers ---

type withingsTestMeas struct {
	value int64
	typ   int
	unit  int
}

type withingsTestMeasGrp struct {
	date     int64
	measures []withingsTestMeas
}

// withingsMeasureResp builds a JSON string for the getmeas endpoint.
func withingsMeasureResp(grps []withingsTestMeasGrp) string {
	type measJSON struct {
		Value int64 `json:"value"`
		Type  int   `json:"type"`
		Unit  int   `json:"unit"`
	}
	type grpJSON struct {
		GrpID    int        `json:"grpid"`
		Attrib   int        `json:"attrib"`
		Date     int64      `json:"date"`
		Created  int64      `json:"created"`
		Category int        `json:"category"`
		Measures []measJSON `json:"measures"`
	}
	type bodyJSON struct {
		Updatetime  int64     `json:"updatetime"`
		Timezone    string    `json:"timezone"`
		MeasureGrps []grpJSON `json:"measuregrps"`
		More        int       `json:"more"`
		Offset      int       `json:"offset"`
	}
	type respJSON struct {
		Status int      `json:"status"`
		Body   bodyJSON `json:"body"`
	}

	gs := make([]grpJSON, len(grps))
	for i, g := range grps {
		ms := make([]measJSON, len(g.measures))
		for j, m := range g.measures {
			ms[j] = measJSON{Value: m.value, Type: m.typ, Unit: m.unit}
		}
		gs[i] = grpJSON{GrpID: i + 1, Date: g.date, Category: 1, Measures: ms}
	}
	b, err := json.Marshal(respJSON{Status: 0, Body: bodyJSON{MeasureGrps: gs}})
	if err != nil {
		panic("withingsMeasureResp: marshal failed: " + err.Error())
	}
	return string(b)
}

type withingsTestSleepSeries struct {
	enddate int64
	light   int
	deep    int
	rem     int
}

type withingsTestSleepSeriesExt struct {
	enddate int64
	light   int
	deep    int
	rem     int
	hrAvg   *int
	rrAvg   *int
}

func ptr(v int) *int { return &v }

// withingsSleepResp builds a JSON string for the getsummary endpoint (no hr/rr).
func withingsSleepResp(series []withingsTestSleepSeries) string {
	ext := make([]withingsTestSleepSeriesExt, len(series))
	for i, s := range series {
		ext[i] = withingsTestSleepSeriesExt{enddate: s.enddate, light: s.light, deep: s.deep, rem: s.rem}
	}
	return withingsSleepRespWithHRRR(ext)
}

// withingsSleepRespWithHRRR builds a getsummary JSON string with optional hr_average/rr_average.
func withingsSleepRespWithHRRR(series []withingsTestSleepSeriesExt) string {
	// Build raw JSON manually to keep optional fields truly absent when nil.
	entries := make([]string, len(series))
	for i, s := range series {
		dataFields := fmt.Sprintf(`"lightsleepduration":%d,"deepsleepduration":%d,"remsleepduration":%d`, s.light, s.deep, s.rem)
		if s.hrAvg != nil {
			dataFields += fmt.Sprintf(`,"hr_average":%d`, *s.hrAvg)
		}
		if s.rrAvg != nil {
			dataFields += fmt.Sprintf(`,"rr_average":%d`, *s.rrAvg)
		}
		entries[i] = fmt.Sprintf(`{"timezone":"UTC","model":16,"startdate":%d,"enddate":%d,"date":"2025-07-27","created":0,"modified":0,"data":{%s}}`, s.enddate-28800, s.enddate, dataFields)
	}
	seriesJSON := "["
	for i, e := range entries {
		if i > 0 {
			seriesJSON += ","
		}
		seriesJSON += e
	}
	seriesJSON += "]"
	return fmt.Sprintf(`{"status":0,"body":{"series":%s,"more":0,"offset":0}}`, seriesJSON)
}
