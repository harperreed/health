// ABOUTME: Tests for the Whoop provider: fixtures, pagination, token refresh, upsert idempotency.
// ABOUTME: All network calls go to injected httptest servers; uses a real SQLite repo.
package provsync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/health/internal/storage"
)

// --- helpers ---

func setupTestRepo(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(dir + "/health.db")
	if err != nil {
		t.Fatalf("open test repo: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func setupTestTokenStore(t *testing.T, tok Token) *TokenStore {
	t.Helper()
	dir := t.TempDir()
	store := NewTokenStore(dir)
	if err := store.Save("whoop", tok); err != nil {
		t.Fatalf("save token: %v", err)
	}
	return store
}

func freshToken() Token {
	return Token{
		AccessToken:  "acc-fresh",
		RefreshToken: "ref-fresh",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
		TokenType:    "Bearer",
	}
}

func expiredToken() Token {
	return Token{
		AccessToken:  "acc-expired",
		RefreshToken: "ref-expired",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
		TokenType:    "Bearer",
	}
}

// whoopServer builds a minimal Whoop-API httptest server that serves
// the three endpoints with realistic JSON. The first call to each
// endpoint returns page 1 with a next_token; the second call returns
// page 2 with no next_token (pagination exhaustion).
func whoopServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Recovery page 1
	recovPage1 := `{
		"records": [
			{
				"cycle_id": 100,
				"sleep_id": 200,
				"created_at": "2026-07-28T07:00:00.000Z",
				"updated_at": "2026-07-28T08:00:00.000Z",
				"score_state": "SCORED",
				"score": {
					"recovery_score": 78.0,
					"resting_heart_rate": 52,
					"hrv_rmssd_milli": 65.5,
					"spo2_percentage": 97.2
				}
			}
		],
		"next_token": "tok-rec-1"
	}`
	recovPage2 := `{
		"records": [
			{
				"cycle_id": 101,
				"sleep_id": 201,
				"created_at": "2026-07-27T07:00:00.000Z",
				"updated_at": "2026-07-27T08:00:00.000Z",
				"score_state": "SCORED",
				"score": {
					"recovery_score": 62.0,
					"resting_heart_rate": 55,
					"hrv_rmssd_milli": 48.0,
					"spo2_percentage": 98.0
				}
			}
		],
		"next_token": ""
	}`

	// Sleep page 1 — includes one nap (should be skipped)
	sleepPage1 := `{
		"records": [
			{
				"start": "2026-07-27T22:00:00.000Z",
				"end": "2026-07-28T06:30:00.000Z",
				"nap": false,
				"score_state": "SCORED",
				"score": {
					"stage_summary": {
						"total_in_bed_time_milli": 31200000,
						"total_awake_time_milli": 1800000
					},
					"respiratory_rate": 15.4
				}
			},
			{
				"start": "2026-07-28T14:00:00.000Z",
				"end": "2026-07-28T14:30:00.000Z",
				"nap": true,
				"score_state": "SCORED",
				"score": {
					"stage_summary": {
						"total_in_bed_time_milli": 1800000,
						"total_awake_time_milli": 0
					},
					"respiratory_rate": 14.0
				}
			}
		],
		"next_token": "tok-sleep-1"
	}`
	sleepPage2 := `{
		"records": [
			{
				"start": "2026-07-26T22:00:00.000Z",
				"end": "2026-07-27T06:00:00.000Z",
				"nap": false,
				"score_state": "SCORED",
				"score": {
					"stage_summary": {
						"total_in_bed_time_milli": 28800000,
						"total_awake_time_milli": 900000
					},
					"respiratory_rate": 14.8
				}
			}
		],
		"next_token": ""
	}`

	// Cycle page 1
	cyclePage1 := `{
		"records": [
			{
				"start": "2026-07-28T00:00:00.000Z",
				"end": "2026-07-29T00:00:00.000Z",
				"score_state": "SCORED",
				"score": {
					"strain": 12.5
				}
			}
		],
		"next_token": "tok-cycle-1"
	}`
	cyclePage2 := `{
		"records": [
			{
				"start": "2026-07-27T00:00:00.000Z",
				"end": "2026-07-28T00:00:00.000Z",
				"score_state": "SCORED",
				"score": {
					"strain": 9.0
				}
			}
		],
		"next_token": ""
	}`

	// Also include an unscored record to verify it gets skipped.
	recovUnscoredPage := `{
		"records": [
			{
				"cycle_id": 199,
				"created_at": "2026-07-26T07:00:00.000Z",
				"score_state": "PENDING_PAYMENT",
				"score": null
			}
		],
		"next_token": ""
	}`
	_ = recovUnscoredPage // referenced below

	// Per-endpoint page counters.
	var recovPage, sleepPage, cyclePage int32

	mux := http.NewServeMux()

	mux.HandleFunc("/developer/v2/recovery", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&recovPage, 1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			fmt.Fprint(w, recovPage1)
		case 2:
			fmt.Fprint(w, recovPage2)
		default:
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
		}
	})

	mux.HandleFunc("/developer/v2/activity/sleep", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&sleepPage, 1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			fmt.Fprint(w, sleepPage1)
		case 2:
			fmt.Fprint(w, sleepPage2)
		default:
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
		}
	})

	mux.HandleFunc("/developer/v2/cycle", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&cyclePage, 1)
		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			fmt.Fprint(w, cyclePage1)
		case 2:
			fmt.Fprint(w, cyclePage2)
		default:
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// --- tests ---

// TestWhoopSyncRecoveryMetrics verifies that recovery records produce
// the four expected metric types with correct values and timestamps.
func TestWhoopSyncRecoveryMetrics(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/developer/v2/recovery":
			fmt.Fprint(w, `{
				"records": [{
					"cycle_id": 1,
					"created_at": "2026-07-28T07:00:00.000Z",
					"score_state": "SCORED",
					"score": {
						"recovery_score": 78.0,
						"resting_heart_rate": 52,
						"hrv_rmssd_milli": 65.5,
						"spo2_percentage": 97.2
					}
				}],
				"next_token": ""
			}`)
		default:
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	repo := setupTestRepo(t)
	store := setupTestTokenStore(t, freshToken())
	client := NewWhoopClient(apiSrv.URL, "http://unused-token-endpoint", "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}

	// Expect recovery, hrv, heart_rate, spo2 from the one SCORED recovery record.
	byType := map[models.MetricType]*models.Metric{}
	for _, m := range all {
		byType[m.MetricType] = m
	}

	checkMetric := func(mt models.MetricType, wantVal float64, wantUnit string) {
		t.Helper()
		m, ok := byType[mt]
		if !ok {
			t.Errorf("missing metric type %q", mt)
			return
		}
		if m.Value != wantVal {
			t.Errorf("%s value = %v, want %v", mt, m.Value, wantVal)
		}
		if m.Unit != wantUnit {
			t.Errorf("%s unit = %q, want %q", mt, m.Unit, wantUnit)
		}
		if m.Source != models.SourceWhoop {
			t.Errorf("%s source = %q, want %q", mt, m.Source, models.SourceWhoop)
		}
	}

	checkMetric(models.MetricRecovery, 78.0, "%")
	checkMetric(models.MetricHRV, 65.5, "ms")
	checkMetric(models.MetricHeartRate, 52, "bpm")
	checkMetric(models.MetricSpO2, 97.2, "%")

	// Timestamps must equal created_at.
	wantTS, _ := time.Parse(time.RFC3339, "2026-07-28T07:00:00.000Z")
	for _, mt := range []models.MetricType{models.MetricRecovery, models.MetricHRV, models.MetricHeartRate, models.MetricSpO2} {
		m, ok := byType[mt]
		if !ok {
			continue
		}
		if !m.RecordedAt.Equal(wantTS) {
			t.Errorf("%s recorded_at = %v, want %v", mt, m.RecordedAt, wantTS)
		}
	}
}

// TestWhoopSyncSleepMetrics verifies sleep records → sleep_hours and respiratory_rate,
// and that nap==true is skipped.
func TestWhoopSyncSleepMetrics(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/developer/v2/activity/sleep":
			fmt.Fprint(w, `{
				"records": [
					{
						"start": "2026-07-27T22:00:00.000Z",
						"end": "2026-07-28T06:30:00.000Z",
						"nap": false,
						"score_state": "SCORED",
						"score": {
							"stage_summary": {
								"total_in_bed_time_milli": 31200000,
								"total_awake_time_milli": 1800000
							},
							"respiratory_rate": 15.4
						}
					},
					{
						"start": "2026-07-28T14:00:00.000Z",
						"end": "2026-07-28T14:30:00.000Z",
						"nap": true,
						"score_state": "SCORED",
						"score": {
							"stage_summary": {
								"total_in_bed_time_milli": 1800000,
								"total_awake_time_milli": 0
							},
							"respiratory_rate": 14.0
						}
					}
				],
				"next_token": ""
			}`)
		default:
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	repo := setupTestRepo(t)
	store := setupTestTokenStore(t, freshToken())
	client := NewWhoopClient(apiSrv.URL, "http://unused-token-endpoint", "cid", "csec", store)

	start := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}

	// Only the non-nap record contributes: sleep_hours + respiratory_rate = 2 metrics.
	if len(all) != 2 {
		t.Fatalf("metric count = %d, want 2 (nap must be skipped)", len(all))
	}

	byType := map[models.MetricType]*models.Metric{}
	for _, m := range all {
		byType[m.MetricType] = m
	}

	// sleep_hours = (31200000 − 1800000) / 3600000 = 29400000 / 3600000 ≈ 8.1667
	wantSleep := 29400000.0 / 3600000.0
	sh, ok := byType[models.MetricSleepHours]
	if !ok {
		t.Fatal("missing sleep_hours metric")
	}
	if sh.Value != wantSleep {
		t.Errorf("sleep_hours = %v, want %v", sh.Value, wantSleep)
	}
	if sh.Unit != "hours" {
		t.Errorf("sleep_hours unit = %q, want hours", sh.Unit)
	}

	// recorded_at = end of sleep window.
	wantTS, _ := time.Parse(time.RFC3339, "2026-07-28T06:30:00.000Z")
	if !sh.RecordedAt.Equal(wantTS) {
		t.Errorf("sleep_hours recorded_at = %v, want %v", sh.RecordedAt, wantTS)
	}

	rr, ok := byType[models.MetricRespiratoryRate]
	if !ok {
		t.Fatal("missing respiratory_rate metric")
	}
	if rr.Value != 15.4 {
		t.Errorf("respiratory_rate = %v, want 15.4", rr.Value)
	}
}

// TestWhoopSyncCycleMetrics verifies cycle records → strain metric.
func TestWhoopSyncCycleMetrics(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/developer/v2/cycle":
			fmt.Fprint(w, `{
				"records": [{
					"start": "2026-07-28T00:00:00.000Z",
					"end":   "2026-07-29T00:00:00.000Z",
					"score_state": "SCORED",
					"score": { "strain": 12.5 }
				}],
				"next_token": ""
			}`)
		default:
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	repo := setupTestRepo(t)
	store := setupTestTokenStore(t, freshToken())
	client := NewWhoopClient(apiSrv.URL, "http://unused-token-endpoint", "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("metric count = %d, want 1", len(all))
	}
	m := all[0]
	if m.MetricType != models.MetricStrain {
		t.Errorf("type = %q, want strain", m.MetricType)
	}
	if m.Value != 12.5 {
		t.Errorf("value = %v, want 12.5", m.Value)
	}
	// recorded_at = cycle start.
	wantTS, _ := time.Parse(time.RFC3339, "2026-07-28T00:00:00.000Z")
	if !m.RecordedAt.Equal(wantTS) {
		t.Errorf("recorded_at = %v, want %v", m.RecordedAt, wantTS)
	}
}

// TestWhoopSyncUnscored verifies that PENDING_PAYMENT / non-SCORED records are skipped.
func TestWhoopSyncUnscored(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"records": [{
				"cycle_id": 99,
				"created_at": "2026-07-28T07:00:00.000Z",
				"score_state": "PENDING_PAYMENT",
				"score": null
			}],
			"next_token": ""
		}`)
	}))
	t.Cleanup(apiSrv.Close)

	repo := setupTestRepo(t)
	store := setupTestTokenStore(t, freshToken())
	client := NewWhoopClient(apiSrv.URL, "http://unused-token-endpoint", "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("metric count = %d, want 0 (unscored must be skipped)", len(all))
	}
}

// TestWhoopSyncPagination verifies that the client follows next_token across pages.
func TestWhoopSyncPagination(t *testing.T) {
	// Recovery only; two pages.
	var callCount int32
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/developer/v2/recovery" {
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
			return
		}
		n := atomic.AddInt32(&callCount, 1)
		switch n {
		case 1:
			fmt.Fprint(w, `{
				"records": [{
					"cycle_id": 1,
					"created_at": "2026-07-28T07:00:00.000Z",
					"score_state": "SCORED",
					"score": {"recovery_score":80,"resting_heart_rate":50,"hrv_rmssd_milli":60,"spo2_percentage":98}
				}],
				"next_token": "page2"
			}`)
		case 2:
			// nextToken param must be present.
			nt := r.URL.Query().Get("nextToken")
			if nt != "page2" {
				t.Errorf("page 2 nextToken = %q, want page2", nt)
			}
			fmt.Fprint(w, `{
				"records": [{
					"cycle_id": 2,
					"created_at": "2026-07-27T07:00:00.000Z",
					"score_state": "SCORED",
					"score": {"recovery_score":60,"resting_heart_rate":58,"hrv_rmssd_milli":42,"spo2_percentage":97}
				}],
				"next_token": ""
			}`)
		default:
			t.Errorf("unexpected page %d request", n)
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	repo := setupTestRepo(t)
	store := setupTestTokenStore(t, freshToken())
	client := NewWhoopClient(apiSrv.URL, "http://unused-token-endpoint", "cid", "csec", store)

	start := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("recovery endpoint called %d times, want 2", callCount)
	}

	// 2 recovery records × 4 metrics each = 8.
	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 8 {
		t.Errorf("metric count = %d, want 8 (both pages, 4 metrics each)", len(all))
	}
}

// TestWhoopSyncExpiredToken verifies that an expired token triggers exactly
// one refresh against the token endpoint, and that the token is persisted.
func TestWhoopSyncExpiredToken(t *testing.T) {
	var refreshCount int32

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&refreshCount, 1)
		newExpiry := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
		// Whoop token endpoint returns standard OAuth2 JSON.
		resp := map[string]interface{}{
			"access_token":  "acc-new",
			"refresh_token": "ref-new",
			"expires_in":    7200,
			"token_type":    "Bearer",
			// expires_at is not part of the wire format; EnsureFresh computes it.
		}
		w.Header().Set("Content-Type", "application/json")
		_ = newExpiry // suppress unused warning
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode token response: %v", err)
		}
	}))
	t.Cleanup(tokenSrv.Close)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Bearer token is the refreshed one.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer acc-new" {
			t.Errorf("Authorization = %q, want Bearer acc-new", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"records":[],"next_token":""}`)
	}))
	t.Cleanup(apiSrv.Close)

	store := setupTestTokenStore(t, expiredToken())
	repo := setupTestRepo(t)
	client := NewWhoopClient(apiSrv.URL, tokenSrv.URL, "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	if atomic.LoadInt32(&refreshCount) != 1 {
		t.Errorf("refresh count = %d, want 1", refreshCount)
	}

	// Token file on disk must hold the rotated refresh token.
	tok, err := store.Load("whoop")
	if err != nil {
		t.Fatalf("load token after refresh: %v", err)
	}
	if tok.RefreshToken != "ref-new" {
		t.Errorf("persisted refresh_token = %q, want ref-new", tok.RefreshToken)
	}
	if tok.AccessToken != "acc-new" {
		t.Errorf("persisted access_token = %q, want acc-new", tok.AccessToken)
	}
}

// TestWhoopSyncIdempotency verifies that re-syncing the same fixtures
// writes zero new rows (UpsertMetric deduplication).
func TestWhoopSyncIdempotency(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/developer/v2/recovery":
			fmt.Fprint(w, `{
				"records": [{
					"cycle_id": 1,
					"created_at": "2026-07-28T07:00:00.000Z",
					"score_state": "SCORED",
					"score": {"recovery_score":78,"resting_heart_rate":52,"hrv_rmssd_milli":65,"spo2_percentage":97}
				}],
				"next_token": ""
			}`)
		case "/developer/v2/activity/sleep":
			fmt.Fprint(w, `{
				"records": [{
					"start": "2026-07-27T22:00:00.000Z",
					"end": "2026-07-28T06:30:00.000Z",
					"nap": false,
					"score_state": "SCORED",
					"score": {
						"stage_summary": {"total_in_bed_time_milli":31200000,"total_awake_time_milli":1800000},
						"respiratory_rate": 15.4
					}
				}],
				"next_token": ""
			}`)
		case "/developer/v2/cycle":
			fmt.Fprint(w, `{
				"records": [{
					"start": "2026-07-28T00:00:00.000Z",
					"end":   "2026-07-29T00:00:00.000Z",
					"score_state": "SCORED",
					"score": {"strain":12.5}
				}],
				"next_token": ""
			}`)
		default:
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	repo := setupTestRepo(t)
	store := setupTestTokenStore(t, freshToken())
	client := NewWhoopClient(apiSrv.URL, "http://unused-token-endpoint", "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)

	// First sync.
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	first, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics after first sync: %v", err)
	}
	beforeCount := len(first)
	if beforeCount == 0 {
		t.Fatal("first sync wrote zero metrics — test setup is wrong")
	}

	// Second sync with same data.
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	second, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics after second sync: %v", err)
	}
	if len(second) != beforeCount {
		t.Errorf("after re-sync count = %d, want %d (no new rows)", len(second), beforeCount)
	}
}

// TestWhoopClientDefaultURLs documents the production base URL constants.
// This is a compile-time sanity check — actual network calls are not made.
func TestWhoopClientDefaultURLs(t *testing.T) {
	if WhoopAPIBaseURL == "" {
		t.Error("WhoopAPIBaseURL must not be empty")
	}
	if WhoopTokenURL == "" {
		t.Error("WhoopTokenURL must not be empty")
	}
}

// TestWhoopSyncTokenSentInHeader verifies Authorization header format.
func TestWhoopSyncTokenSentInHeader(t *testing.T) {
	var gotAuth string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"records":[],"next_token":""}`)
	}))
	t.Cleanup(apiSrv.Close)

	store := setupTestTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWhoopClient(apiSrv.URL, "http://unused", "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	_ = client.Sync(repo, start, end)

	if gotAuth != "Bearer acc-fresh" {
		t.Errorf("Authorization = %q, want Bearer acc-fresh", gotAuth)
	}
}

// TestWhoopSyncHTTPError verifies that a server error propagates as a non-nil error.
func TestWhoopSyncHTTPError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal"}`)
	}))
	t.Cleanup(apiSrv.Close)

	store := setupTestTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWhoopClient(apiSrv.URL, "http://unused", "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	err := client.Sync(repo, start, end)
	if err == nil {
		t.Error("expected error on HTTP 500, got nil")
	}
}

// TestWhoopRefreshTokenRotation verifies the token rotation on refresh:
// the new refresh_token from the server replaces the old one on disk.
func TestWhoopRefreshTokenRotation(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Confirm it received the current refresh token.
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.FormValue("refresh_token") != "ref-expired" {
			t.Errorf("refresh_token sent = %q, want ref-expired", r.FormValue("refresh_token"))
		}
		resp := map[string]interface{}{
			"access_token":  "acc-rotated",
			"refresh_token": "ref-rotated",
			"expires_in":    7200,
			"token_type":    "Bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	t.Cleanup(tokenSrv.Close)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"records":[],"next_token":""}`)
	}))
	t.Cleanup(apiSrv.Close)

	store := setupTestTokenStore(t, expiredToken())
	repo := setupTestRepo(t)
	client := NewWhoopClient(apiSrv.URL, tokenSrv.URL, "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	saved, err := store.Load("whoop")
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if saved.RefreshToken != "ref-rotated" {
		t.Errorf("saved refresh_token = %q, want ref-rotated", saved.RefreshToken)
	}
}

// TestWhoopSyncSetsSourceWhoop verifies every emitted metric carries source=whoop.
func TestWhoopSyncSetsSourceWhoop(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/developer/v2/cycle":
			fmt.Fprint(w, `{
				"records": [{
					"start":"2026-07-28T00:00:00.000Z",
					"end":"2026-07-29T00:00:00.000Z",
					"score_state":"SCORED",
					"score":{"strain":10}
				}],
				"next_token":""
			}`)
		default:
			fmt.Fprint(w, `{"records":[],"next_token":""}`)
		}
	}))
	t.Cleanup(apiSrv.Close)

	store := setupTestTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWhoopClient(apiSrv.URL, "http://unused", "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	for _, m := range all {
		if m.Source != models.SourceWhoop {
			t.Errorf("source = %q, want %q", m.Source, models.SourceWhoop)
		}
	}
}

// TestWhoopSyncLargeMultiPageSet exercises the full whoopServer fixture
// (2 pages × 3 endpoints) and checks total metric count.
func TestWhoopSyncLargeMultiPageSet(t *testing.T) {
	// recovery: 2 records × 4 metrics = 8
	// sleep:    2 non-nap records × 2 metrics = 4 (nap is skipped)
	// cycle:    2 records × 1 metric = 2
	// total = 14

	srv := whoopServer(t)
	store := setupTestTokenStore(t, freshToken())
	repo := setupTestRepo(t)
	client := NewWhoopClient(srv.URL, "http://unused", "cid", "csec", store)

	start := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	all, err := repo.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 14 {
		// Print a breakdown to help debug if count is wrong.
		byType := map[models.MetricType]int{}
		for _, m := range all {
			byType[m.MetricType]++
		}
		t.Errorf("metric count = %d, want 14; breakdown: %v", len(all), byType)
	}
}

// TestWhoopClientClose verifies no panic on Close (resource cleanup hook).
func TestWhoopClientClose(t *testing.T) {
	store := setupTestTokenStore(t, freshToken())
	client := NewWhoopClient("http://unused", "http://unused", "cid", "csec", store)
	client.Close()
}

// TestWhoopProductionURLPathComposition guards against the doubled-prefix bug:
// WhoopAPIBaseURL + "/developer/v2/recovery" must not produce a URL containing
// "/developer/developer/". The httptest server records every request path; we
// verify that the recovery endpoint is reached at exactly /developer/v2/recovery.
func TestWhoopProductionURLPathComposition(t *testing.T) {
	// Collect all paths the server receives.
	var receivedPaths []string
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPaths = append(receivedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"records":[],"next_token":""}`)
	}))
	t.Cleanup(apiSrv.Close)

	// Graft the path suffix of WhoopAPIBaseURL onto the test server so the test
	// exercises exactly the composition that production code performs. With the
	// correct constant ("https://api.prod.whoop.com", no path suffix) basePath is
	// empty and paths like "/developer/v2/recovery" are sent as-is. With the old
	// wrong constant ("https://api.prod.whoop.com/developer") basePath would be
	// "/developer", composing "/developer/developer/v2/recovery".
	u, err := url.Parse(WhoopAPIBaseURL)
	if err != nil {
		t.Fatalf("WhoopAPIBaseURL parse: %v", err)
	}
	testBase := apiSrv.URL + u.Path

	store := setupTestTokenStore(t, freshToken())
	client := NewWhoopClient(testBase, "http://unused-token-endpoint", "cid", "csec", store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	_ = client.Sync(setupTestRepo(t), start, end)

	// Find the path the recovery fetch used.
	recoveryPath := ""
	for _, p := range receivedPaths {
		if p == "/developer/v2/recovery" || p == "/developer/developer/v2/recovery" {
			recoveryPath = p
			break
		}
	}
	if recoveryPath == "" {
		t.Fatalf("recovery endpoint not called; all paths: %v", receivedPaths)
	}
	if recoveryPath != "/developer/v2/recovery" {
		t.Errorf("recovery request path = %q, want /developer/v2/recovery (WhoopAPIBaseURL path suffix %q caused doubled prefix)",
			recoveryPath, u.Path)
	}
}

// TestWhoopRefreshSendsClientCredentials verifies that the OAuth2 token refresh
// POST includes client_id and client_secret. The token endpoint returns 401 if
// either credential is missing, which causes EnsureFresh to fail and Sync to
// return a non-nil error — failing the test.
func TestWhoopRefreshSendsClientCredentials(t *testing.T) {
	const wantClientID = "test-client-id"
	const wantClientSecret = "test-client-secret"

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("token endpoint: parse form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.FormValue("client_id") != wantClientID || r.FormValue("client_secret") != wantClientSecret {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"error":"invalid_client","client_id":%q,"client_secret":%q}`,
				r.FormValue("client_id"), r.FormValue("client_secret"))
			return
		}
		resp := map[string]interface{}{
			"access_token":  "acc-cred-ok",
			"refresh_token": "ref-cred-ok",
			"expires_in":    7200,
			"token_type":    "Bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	t.Cleanup(tokenSrv.Close)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"records":[],"next_token":""}`)
	}))
	t.Cleanup(apiSrv.Close)

	store := setupTestTokenStore(t, expiredToken())
	repo := setupTestRepo(t)
	client := NewWhoopClient(apiSrv.URL, tokenSrv.URL, wantClientID, wantClientSecret, store)

	start := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := client.Sync(repo, start, end); err != nil {
		t.Fatalf("Sync failed (token endpoint rejected credentials): %v", err)
	}

	// Confirm the access token was rotated to the one the server returned.
	tok, err := store.Load("whoop")
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if tok.AccessToken != "acc-cred-ok" {
		t.Errorf("access_token = %q, want acc-cred-ok", tok.AccessToken)
	}
}
