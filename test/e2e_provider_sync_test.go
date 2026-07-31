// ABOUTME: End-to-end test for all three provider sync flows into one shared repository.
// ABOUTME: Exercises constructor-injection seam (in-process Sync) + real binary (CLI list --source).
package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/health/internal/provsync"
	"github.com/harperreed/health/internal/storage"
)

// ---- fixture timestamps (fixed so re-sync hits the same upsert keys) ----

// Whoop timestamps are RFC3339 strings embedded in JSON; the provider parses them.
const (
	whoopRecoveryTS   = "2026-07-28T07:00:00.000Z"
	whoopSleepEndTS   = "2026-07-28T06:30:00.000Z"
	whoopCycleStartTS = "2026-07-28T00:00:00.000Z"
)

// Withings timestamps are Unix epoch integers embedded in JSON.
const (
	withingsMeasDate = int64(1753574400) // 2025-07-27T00:00:00Z
	withingsSleepEnd = int64(1753617600) // 2025-07-27T12:00:00Z
)

// Emfit timestamp (last element of minitrend_datestamps) as Unix epoch.
const emfitTS = int64(1753700000)

// ---- Whoop fake server ----

// whoopFixtures returns the static JSON bodies for each Whoop endpoint.
// run=1 returns real data; run≥2 returns empty (pagination terminates).
// The function intentionally has no loop-counter state: each call to the
// mux handler decides what to return based on request parameters.
func newWhoopFakeServer(t *testing.T, recoveryScore float64) *httptest.Server {
	t.Helper()
	recoveryBody := fmt.Sprintf(`{
		"records": [{
			"cycle_id": 100,
			"created_at": %q,
			"score_state": "SCORED",
			"score": {
				"recovery_score": %.1f,
				"resting_heart_rate": 52,
				"hrv_rmssd_milli": 65.5,
				"spo2_percentage": 97.2
			}
		}],
		"next_token": ""
	}`, whoopRecoveryTS, recoveryScore)

	sleepBody := fmt.Sprintf(`{
		"records": [{
			"start": "2026-07-27T22:00:00.000Z",
			"end": %q,
			"nap": false,
			"score_state": "SCORED",
			"score": {
				"stage_summary": {
					"total_in_bed_time_milli": 31200000,
					"total_awake_time_milli": 1800000
				},
				"respiratory_rate": 15.4
			}
		}],
		"next_token": ""
	}`, whoopSleepEndTS)

	cycleBody := fmt.Sprintf(`{
		"records": [{
			"start": %q,
			"end": "2026-07-29T00:00:00.000Z",
			"score_state": "SCORED",
			"score": { "strain": 12.5 }
		}],
		"next_token": ""
	}`, whoopCycleStartTS)

	mux := http.NewServeMux()
	mux.HandleFunc("/developer/v2/recovery", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, recoveryBody)
	})
	mux.HandleFunc("/developer/v2/activity/sleep", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, sleepBody)
	})
	mux.HandleFunc("/developer/v2/cycle", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, cycleBody)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ---- Withings fake server ----

// newWithingsFakeServer returns a server that handles both the API and the token
// endpoint on the same mux (tests pass apiSrv.URL+"/v2/oauth2" as tokenURL,
// matching the real Withings token URL structure).
func newWithingsFakeServer(t *testing.T, weightValue float64) *httptest.Server {
	t.Helper()

	// Measures: type 1 = weight (82.5 kg → value=82500, unit=-3)
	// body_fat: type 6 (17.5% → value=175, unit=-1)
	measBody := withingsE2EMeasResp(withingsMeasDate, weightValue)

	sleepBody := withingsE2ESleepResp(withingsSleepEnd)

	mux := http.NewServeMux()

	// Data endpoints.
	mux.HandleFunc("/measure", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, measBody)
	})
	mux.HandleFunc("/v2/sleep", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, sleepBody)
	})

	// Token endpoint (Withings nonstandard {status,body} envelope).
	mux.HandleFunc("/v2/oauth2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{"status":0,"body":{"userid":"1","access_token":"withings-acc","refresh_token":"withings-ref","expires_in":10800,"token_type":"Bearer"}}`
		fmt.Fprint(w, resp)
	})

	// Fallback.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":0,"body":{}}`)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// withingsE2EMeasResp builds a getmeas JSON response with weight + body_fat.
// weightValue (kg) is encoded as int64(weightValue*1000), unit=-3.
func withingsE2EMeasResp(date int64, weightKg float64) string {
	// weight: value = weightKg*1000, unit = -3 → real = weightKg
	// body_fat: value=175, unit=-1 → real = 17.5%
	wv := int64(weightKg * 1000)
	type meas struct {
		Value int64 `json:"value"`
		Type  int   `json:"type"`
		Unit  int   `json:"unit"`
	}
	type grp struct {
		GrpID    int    `json:"grpid"`
		Date     int64  `json:"date"`
		Category int    `json:"category"`
		Measures []meas `json:"measures"`
	}
	type body struct {
		MeasureGrps []grp `json:"measuregrps"`
		More        int   `json:"more"`
		Offset      int   `json:"offset"`
	}
	type resp struct {
		Status int  `json:"status"`
		Body   body `json:"body"`
	}
	r := resp{
		Status: 0,
		Body: body{
			MeasureGrps: []grp{{
				GrpID:    1,
				Date:     date,
				Category: 1,
				Measures: []meas{
					{Value: wv, Type: 1, Unit: -3},
					{Value: 175, Type: 6, Unit: -1},
				},
			}},
		},
	}
	b, err := json.Marshal(r)
	if err != nil {
		panic("withingsE2EMeasResp: marshal failed: " + err.Error())
	}
	return string(b)
}

// withingsE2ESleepResp builds a getsummary JSON response: light+deep+rem → sleep_hours.
func withingsE2ESleepResp(enddate int64) string {
	// light=14400s, deep=7200s, rem=3600s → total=25200s → 7.0h
	return fmt.Sprintf(`{"status":0,"body":{"series":[{
		"timezone":"UTC","model":16,
		"startdate":%d,"enddate":%d,
		"date":"2025-07-27","created":0,"modified":0,
		"data":{"lightsleepduration":14400,"deepsleepduration":7200,"remsleepduration":3600}
	}],"more":0,"offset":0}}`, enddate-28800, enddate)
}

// ---- Emfit fake server ----

func newEmfitFakeServer(t *testing.T, deviceID string, hrvVal float64) *httptest.Server {
	t.Helper()

	// Build the latest presence body.
	// sleep_duration=25920s → 7.2h; hrv_rmssd_morning=hrvVal
	body := fmt.Sprintf(`{
		"id": 42,
		"sleep_duration": 25920,
		"hrv_rmssd_morning": %.1f,
		"measured_hr_avg": 58.0,
		"measured_rr_avg": 14.2,
		"minitrend_datestamps": [{"ts": 1700000000}, {"ts": %d}]
	}`, hrvVal, emfitTS)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user/get", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"user":{"id":1}}`)
	})
	mux.HandleFunc(fmt.Sprintf("/api/v1/presence/%s/latest", deviceID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ---- token seeding helpers ----

// seedToken writes a fresh (unexpired) OAuth token for the given provider key
// to the token store rooted at dataDir.
func seedToken(t *testing.T, dataDir, provider string) {
	t.Helper()
	store := provsync.NewTokenStore(dataDir)
	tok := provsync.Token{
		AccessToken:  provider + "-access",
		RefreshToken: provider + "-refresh",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
		TokenType:    "Bearer",
	}
	if err := store.Save(provider, tok); err != nil {
		t.Fatalf("seed token for %s: %v", provider, err)
	}
}

// ---- convergence helper ----

// openSQLiteRepo opens the same SQLite database that the binary will use
// when XDG_DATA_HOME is set to dataDir with a pre-written sqlite config.
func openSQLiteRepo(t *testing.T, dataDir string) storage.Repository {
	t.Helper()
	dbPath := filepath.Join(dataDir, "health", "health.db")
	repo, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

// writeConfigJSON writes a minimal config.json to the XDG_CONFIG_HOME location
// inside testConfigDir, forcing the binary to use the SQLite backend at
// dataDir/health/health.db (the same path openSQLiteRepo opens).
func writeConfigJSON(t *testing.T, testConfigDir, dataDir string) {
	t.Helper()
	configDir := filepath.Join(testConfigDir, "health")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	cfg := map[string]string{
		"backend":  "sqlite",
		"data_dir": filepath.Join(dataDir, "health"),
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), b, 0o600); err != nil {
		t.Fatalf("write config.json: %v", err)
	}
}

// ---- main test ----

// TestE2EProviderSync is the end-to-end test for the three provider sync flows.
//
// Two seams compose:
//  1. In-process: NewWhoopClient/NewWithingsClient/NewEmfitClient constructed
//     with httptest URLs, syncing into a real SQLite repo on a temp data dir.
//  2. CLI: the real binary, XDG_DATA_HOME pointed at the SAME temp dir,
//     running `health list --source <provider>` against the populated database.
func TestE2EProviderSync(t *testing.T) {
	// ---- environment setup ----
	// Two separate temp dirs: one for XDG_DATA_HOME, one for XDG_CONFIG_HOME.
	// The binary reads both; we control both.
	dataDir := t.TempDir()   // XDG_DATA_HOME: health.db lives at dataDir/health/health.db
	configDir := t.TempDir() // XDG_CONFIG_HOME: config.json lives here

	// Pre-write config.json so the binary uses sqlite, not the markdown auto-detect path.
	writeConfigJSON(t, configDir, dataDir)

	// Open the in-process repo against the same path the binary will use.
	repo := openSQLiteRepo(t, dataDir)

	// ---- device ID for Emfit ----
	const emfitDeviceID = "e2e-dev-001"

	// ---- fake API servers (pass 1 recovery score) ----
	whoopSrv := newWhoopFakeServer(t, 78.0)
	withingsSrv := newWithingsFakeServer(t, 82.5)
	emfitSrv := newEmfitFakeServer(t, emfitDeviceID, 55.3)

	// ---- seed token stores (token files at dataDir/health/tokens/<provider>.json) ----
	healthDataDir := filepath.Join(dataDir, "health")
	seedToken(t, healthDataDir, "whoop")
	seedToken(t, healthDataDir, "withings")

	// ---- build clients ----
	whoopStore := provsync.NewTokenStore(healthDataDir)
	withingsStore := provsync.NewTokenStore(healthDataDir)

	whoopClient := provsync.NewWhoopClient(
		whoopSrv.URL,
		"http://unused-whoop-token", // fresh token → no refresh needed
		"cid", "csec",
		whoopStore,
	)
	defer whoopClient.Close()

	withingsClient := provsync.NewWithingsClient(
		withingsSrv.URL,
		withingsSrv.URL+"/v2/oauth2",
		"cid", "csec",
		withingsStore,
	)
	defer withingsClient.Close()

	emfitClient := provsync.NewEmfitClient(emfitSrv.URL, "emfit-tok", emfitDeviceID)
	defer emfitClient.Close()

	// ---- sync window ----
	start := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)

	// ---- pass 1: sync all three ----
	if err := whoopClient.Sync(repo, start, end); err != nil {
		t.Fatalf("pass 1: whoop Sync: %v", err)
	}
	if err := withingsClient.Sync(repo, start, end); err != nil {
		t.Fatalf("pass 1: withings Sync: %v", err)
	}
	if err := emfitClient.Sync(repo, start, end); err != nil {
		t.Fatalf("pass 1: emfit Sync: %v", err)
	}

	// ---- assert pass 1 row counts per source ----
	// Whoop: recovery→4 metrics, sleep→2, cycle→1 = 7
	// Withings: weight+body_fat=2, sleep_hours=1 = 3
	// Emfit: sleep_hours+hrv+heart_rate+respiratory_rate = 4
	// Total = 14

	whoopSrc := models.SourceWhoop
	withingsSrc := models.SourceWithings
	emfitSrc := models.SourceEmfit

	pass1Whoop, err := repo.ListMetrics(nil, &whoopSrc, 0)
	if err != nil {
		t.Fatalf("pass 1 ListMetrics whoop: %v", err)
	}
	if len(pass1Whoop) != 7 {
		byType := metricCountByType(pass1Whoop)
		t.Errorf("pass 1 whoop count = %d, want 7; breakdown: %v", len(pass1Whoop), byType)
	}

	pass1Withings, err := repo.ListMetrics(nil, &withingsSrc, 0)
	if err != nil {
		t.Fatalf("pass 1 ListMetrics withings: %v", err)
	}
	if len(pass1Withings) != 3 {
		byType := metricCountByType(pass1Withings)
		t.Errorf("pass 1 withings count = %d, want 3; breakdown: %v", len(pass1Withings), byType)
	}

	pass1Emfit, err := repo.ListMetrics(nil, &emfitSrc, 0)
	if err != nil {
		t.Fatalf("pass 1 ListMetrics emfit: %v", err)
	}
	if len(pass1Emfit) != 4 {
		byType := metricCountByType(pass1Emfit)
		t.Errorf("pass 1 emfit count = %d, want 4; breakdown: %v", len(pass1Emfit), byType)
	}

	pass1Total := len(pass1Whoop) + len(pass1Withings) + len(pass1Emfit)

	// ---- pass 2: rebuild servers with bumped values so UpsertMetric's
	// update-in-place path is exercised with a detectable value change ----
	//
	// Whoop recovery_score: 78.0 → 80.0
	// Withings weight: 82.5 → 83.0
	// Emfit hrv: 55.3 → 60.0

	whoopSrv2 := newWhoopFakeServer(t, 80.0)
	withingsSrv2 := newWithingsFakeServer(t, 83.0)
	emfitSrv2 := newEmfitFakeServer(t, emfitDeviceID, 60.0)

	// Re-seed fresh tokens (the token store is shared; they're still valid, but
	// the servers changed, so we point new clients at them).
	whoopClient2 := provsync.NewWhoopClient(
		whoopSrv2.URL,
		"http://unused-whoop-token",
		"cid", "csec",
		whoopStore,
	)
	defer whoopClient2.Close()

	withingsClient2 := provsync.NewWithingsClient(
		withingsSrv2.URL,
		withingsSrv2.URL+"/v2/oauth2",
		"cid", "csec",
		withingsStore,
	)
	defer withingsClient2.Close()

	emfitClient2 := provsync.NewEmfitClient(emfitSrv2.URL, "emfit-tok", emfitDeviceID)
	defer emfitClient2.Close()

	if err := whoopClient2.Sync(repo, start, end); err != nil {
		t.Fatalf("pass 2: whoop Sync: %v", err)
	}
	if err := withingsClient2.Sync(repo, start, end); err != nil {
		t.Fatalf("pass 2: withings Sync: %v", err)
	}
	if err := emfitClient2.Sync(repo, start, end); err != nil {
		t.Fatalf("pass 2: emfit Sync: %v", err)
	}

	// ---- assert pass 2: zero new rows ----
	pass2Whoop, err := repo.ListMetrics(nil, &whoopSrc, 0)
	if err != nil {
		t.Fatalf("pass 2 ListMetrics whoop: %v", err)
	}
	pass2Withings, err := repo.ListMetrics(nil, &withingsSrc, 0)
	if err != nil {
		t.Fatalf("pass 2 ListMetrics withings: %v", err)
	}
	pass2Emfit, err := repo.ListMetrics(nil, &emfitSrc, 0)
	if err != nil {
		t.Fatalf("pass 2 ListMetrics emfit: %v", err)
	}

	pass2Total := len(pass2Whoop) + len(pass2Withings) + len(pass2Emfit)
	if pass2Total != pass1Total {
		t.Errorf("pass 2 total row count = %d, want %d (zero new rows)", pass2Total, pass1Total)
	}

	// ---- assert update-in-place: bumped values changed, row count stayed flat ----
	// Whoop recovery_score: 78.0 → 80.0 (recorded at whoopRecoveryTS)
	wantRecoveryTS, _ := time.Parse("2006-01-02T15:04:05.000Z", whoopRecoveryTS)
	if m := findMetric(pass2Whoop, models.MetricRecovery, wantRecoveryTS); m != nil {
		if m.Value != 80.0 {
			t.Errorf("whoop recovery updated value = %v, want 80.0 (in-place update failed)", m.Value)
		}
	} else {
		t.Error("whoop recovery metric not found after pass 2")
	}

	// Withings weight: 82.5 → 83.0 (recorded at unix(withingsMeasDate))
	wantWeightTS := time.Unix(withingsMeasDate, 0)
	if m := findMetric(pass2Withings, models.MetricWeight, wantWeightTS); m != nil {
		if m.Value != 83.0 {
			t.Errorf("withings weight updated value = %v, want 83.0 (in-place update failed)", m.Value)
		}
	} else {
		t.Error("withings weight metric not found after pass 2")
	}

	// Emfit HRV: 55.3 → 60.0 (recorded at unix(emfitTS))
	wantEmfitTS := time.Unix(emfitTS, 0)
	if m := findMetric(pass2Emfit, models.MetricHRV, wantEmfitTS); m != nil {
		if m.Value != 60.0 {
			t.Errorf("emfit hrv updated value = %v, want 60.0 (in-place update failed)", m.Value)
		}
	} else {
		t.Error("emfit hrv metric not found after pass 2")
	}

	// ---- CLI: build the real binary, list --source per provider ----
	// We must close the in-process repo before the binary opens the same DB.
	if err := repo.Close(); err != nil {
		t.Fatalf("close in-process repo before CLI: %v", err)
	}

	projectRoot, _ := filepath.Abs("..")
	healthBin := filepath.Join(projectRoot, "health-e2e-sync-test")

	buildCmd := exec.Command("go", "build", "-o", healthBin, "./cmd/health")
	buildCmd.Dir = projectRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary: %v\n%s", err, out)
	}
	defer os.Remove(healthBin)

	run := func(args ...string) (string, error) {
		cmd := exec.Command(healthBin, args...)
		cmd.Env = append(filterEnv(os.Environ(), "XDG_DATA_HOME", "XDG_CONFIG_HOME"),
			"XDG_DATA_HOME="+dataDir,
			"XDG_CONFIG_HOME="+configDir,
		)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// -- whoop filter --
	whoopOut, err := run("list", "--source", "whoop")
	if err != nil {
		t.Fatalf("list --source whoop: %v\n%s", err, whoopOut)
	}
	if !strings.Contains(whoopOut, "whoop") {
		t.Errorf("list --source whoop: expected whoop rows, got:\n%s", whoopOut)
	}
	if strings.Contains(whoopOut, "withings") {
		t.Errorf("list --source whoop: unexpected withings rows in output:\n%s", whoopOut)
	}
	if strings.Contains(whoopOut, "emfit") {
		t.Errorf("list --source whoop: unexpected emfit rows in output:\n%s", whoopOut)
	}

	// -- withings filter --
	withingsOut, err := run("list", "--source", "withings")
	if err != nil {
		t.Fatalf("list --source withings: %v\n%s", err, withingsOut)
	}
	if !strings.Contains(withingsOut, "withings") {
		t.Errorf("list --source withings: expected withings rows, got:\n%s", withingsOut)
	}
	if strings.Contains(withingsOut, "whoop") {
		t.Errorf("list --source withings: unexpected whoop rows in output:\n%s", withingsOut)
	}
	if strings.Contains(withingsOut, "emfit") {
		t.Errorf("list --source withings: unexpected emfit rows in output:\n%s", withingsOut)
	}

	// -- emfit filter --
	emfitOut, err := run("list", "--source", "emfit")
	if err != nil {
		t.Fatalf("list --source emfit: %v\n%s", err, emfitOut)
	}
	if !strings.Contains(emfitOut, "emfit") {
		t.Errorf("list --source emfit: expected emfit rows, got:\n%s", emfitOut)
	}
	if strings.Contains(emfitOut, "whoop") {
		t.Errorf("list --source emfit: unexpected whoop rows in output:\n%s", emfitOut)
	}
	if strings.Contains(emfitOut, "withings") {
		t.Errorf("list --source emfit: unexpected withings rows in output:\n%s", emfitOut)
	}
}

// ---- helpers ----

// metricCountByType returns a map of metric type → count for a slice of metrics.
func metricCountByType(metrics []*models.Metric) map[models.MetricType]int {
	m := make(map[models.MetricType]int)
	for _, met := range metrics {
		m[met.MetricType]++
	}
	return m
}

// findMetric returns the metric with the given type and recorded_at from the slice,
// or nil if not found.
func findMetric(metrics []*models.Metric, mt models.MetricType, at time.Time) *models.Metric {
	for _, m := range metrics {
		if m.MetricType == mt && m.RecordedAt.Equal(at) {
			return m
		}
	}
	return nil
}

// filterEnv removes keys from environ that match any of the given names,
// then returns the filtered slice. Used so we can inject fresh XDG vars.
func filterEnv(environ []string, removeKeys ...string) []string {
	remove := make(map[string]bool, len(removeKeys))
	for _, k := range removeKeys {
		remove[k] = true
	}
	out := make([]string, 0, len(environ))
	for _, e := range environ {
		key := e
		if i := strings.IndexByte(e, '='); i >= 0 {
			key = e[:i]
		}
		if !remove[key] {
			out = append(out, e)
		}
	}
	return out
}
