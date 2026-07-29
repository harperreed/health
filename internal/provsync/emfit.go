// ABOUTME: Emfit QS provider: fetches latest sleep presence from the Emfit QS API.
// ABOUTME: Supports config-token or username/password login; writes metrics via UpsertMetric.
package provsync

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/health/internal/storage"
)

const (
	// EmfitAPIBaseURL is the production Emfit QS API base.
	// Endpoints: /api/v1/user/get, /api/v1/presence/{device_id}/latest.
	EmfitAPIBaseURL = "https://qs-api.emfit.com"
)

// EmfitClient fetches sleep metrics from the Emfit QS API.
// Construct with NewEmfitClient (config token) or NewEmfitClientWithLogin (username/password).
// Call Sync to pull the latest night's data.
type EmfitClient struct {
	apiBase  string
	token    string // pre-configured; empty means login required
	username string // used when token is empty
	password string // used when token is empty
	deviceID string
	http     *http.Client
}

// NewEmfitClient returns an EmfitClient that authenticates using the supplied
// Bearer token directly, skipping login. apiBaseURL and token are constructor
// parameters so tests can inject httptest servers and tokens; use EmfitAPIBaseURL
// and the operator-supplied token for production. deviceID is required and must
// match the Emfit device in the account.
func NewEmfitClient(apiBaseURL, token, deviceID string) *EmfitClient {
	return &EmfitClient{
		apiBase:  apiBaseURL,
		token:    token,
		deviceID: deviceID,
		http:     &http.Client{Timeout: httpTimeout},
	}
}

// NewEmfitClientWithLogin returns an EmfitClient that obtains a token by posting
// username and password to /api/v1/login at the start of each Sync call.
// Per api-research.md: the login response includes "token"; fall back to
// "remember_token" if "token" is absent.
func NewEmfitClientWithLogin(apiBaseURL, username, password, deviceID string) *EmfitClient {
	return &EmfitClient{
		apiBase:  apiBaseURL,
		username: username,
		password: password,
		deviceID: deviceID,
		http:     &http.Client{Timeout: httpTimeout},
	}
}

// Close releases resources (currently a no-op; present for interface parity).
func (c *EmfitClient) Close() {}

// Sync fetches the latest night's presence record from Emfit and upserts the four
// mapped metrics into repo. It is idempotent: re-syncing the same night writes no
// new rows (UpsertMetric deduplicates on source+type+timestamp).
// start and end are accepted for interface consistency but Emfit's /latest
// endpoint returns the most recent night regardless of date range.
func (c *EmfitClient) Sync(repo storage.Repository, start, end time.Time) error {
	if c.deviceID == "" {
		return fmt.Errorf("emfit: device_id is required in config")
	}

	tok, err := c.resolveToken()
	if err != nil {
		return err
	}

	return c.syncLatest(repo, tok)
}

// resolveToken returns the token to use for this Sync call.
// If a static token was supplied at construction, use it.
// Otherwise POST /api/v1/login to obtain one.
func (c *EmfitClient) resolveToken() (string, error) {
	if c.token != "" {
		return c.token, nil
	}
	return c.login()
}

// --- login ---

// emfitLoginResponse is the wire format of POST /api/v1/login.
// Per api-research.md: prefer "token"; fall back to "remember_token" if absent.
type emfitLoginResponse struct {
	Token         string `json:"token"`
	RememberToken string `json:"remember_token"`
}

// login performs form-encoded POST /api/v1/login with username+password and
// returns the bearer token to use for subsequent calls.
func (c *EmfitClient) login() (string, error) {
	params := url.Values{}
	params.Set("username", c.username)
	params.Set("password", c.password)

	resp, err := c.http.PostForm(c.apiBase+"/api/v1/login", params)
	if err != nil {
		return "", fmt.Errorf("emfit: login POST: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("emfit: login read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("emfit: login status %d: %s", resp.StatusCode, body)
	}

	var lr emfitLoginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return "", fmt.Errorf("emfit: login decode response: %w", err)
	}

	if lr.Token != "" {
		return lr.Token, nil
	}
	if lr.RememberToken != "" {
		return lr.RememberToken, nil
	}
	return "", fmt.Errorf("emfit: login response contained neither token nor remember_token")
}

// --- presence sync ---

// emfitPresenceResponse is the wire format of GET /api/v1/presence/{id}/latest.
// minitrend_datestamps tolerates BOTH [{ts:N}, ...] objects and bare [N, ...] numbers.
type emfitPresenceResponse struct {
	SleepDuration       float64         `json:"sleep_duration"`
	HRVRmssdMorning     float64         `json:"hrv_rmssd_morning"`
	MeasuredHRAvg       float64         `json:"measured_hr_avg"`
	MeasuredRRAvg       float64         `json:"measured_rr_avg"`
	MinitrendDatestamps emfitDatestamps `json:"minitrend_datestamps"`
}

// emfitDatestamps handles the two wire shapes for minitrend_datestamps:
//   - Array of objects: [{"ts": 1700000000}, ...]
//   - Array of bare numbers: [1700000000, ...]
//
// UnmarshalJSON tries the object form first; falls back to bare numbers.
// The only value used is the last element.
type emfitDatestamps struct {
	stamps []int64 // unix seconds
}

func (d *emfitDatestamps) UnmarshalJSON(data []byte) error {
	// Try object shape: [{ts: N}, ...]
	var objShape []struct {
		TS int64 `json:"ts"`
	}
	if err := json.Unmarshal(data, &objShape); err == nil && len(objShape) > 0 {
		d.stamps = make([]int64, len(objShape))
		for i, o := range objShape {
			d.stamps[i] = o.TS
		}
		return nil
	}

	// Try bare number shape: [N, ...]
	var numShape []int64
	if err := json.Unmarshal(data, &numShape); err != nil {
		return fmt.Errorf("emfit: minitrend_datestamps: unrecognized shape: %w", err)
	}
	d.stamps = numShape
	return nil
}

// last returns the unix timestamp of the last datestamp element.
// Returns an error if the slice is empty.
func (d *emfitDatestamps) last() (int64, error) {
	if len(d.stamps) == 0 {
		return 0, fmt.Errorf("emfit: minitrend_datestamps is empty")
	}
	return d.stamps[len(d.stamps)-1], nil
}

func (c *EmfitClient) syncLatest(repo storage.Repository, tok string) error {
	path := fmt.Sprintf("/api/v1/presence/%s/latest", c.deviceID)
	reqURL := c.apiBase + path

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("emfit: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("emfit: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("emfit: read body %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("emfit: GET %s: status %d: %s", path, resp.StatusCode, body)
	}

	var presence emfitPresenceResponse
	if err := json.Unmarshal(body, &presence); err != nil {
		return fmt.Errorf("emfit: decode presence: %w", err)
	}

	tsUnix, err := presence.MinitrendDatestamps.last()
	if err != nil {
		return fmt.Errorf("emfit: %w", err)
	}
	ts := time.Unix(tsUnix, 0)

	// sleep_duration is in seconds; convert to hours.
	sleepHours := presence.SleepDuration / 3600.0

	metrics := []*models.Metric{
		models.NewMetric(models.MetricSleepHours, sleepHours).
			WithSource(models.SourceEmfit).WithRecordedAt(ts),
		models.NewMetric(models.MetricHRV, presence.HRVRmssdMorning).
			WithSource(models.SourceEmfit).WithRecordedAt(ts),
		models.NewMetric(models.MetricHeartRate, presence.MeasuredHRAvg).
			WithSource(models.SourceEmfit).WithRecordedAt(ts),
		models.NewMetric(models.MetricRespiratoryRate, presence.MeasuredRRAvg).
			WithSource(models.SourceEmfit).WithRecordedAt(ts),
	}

	for _, m := range metrics {
		if _, err := repo.UpsertMetric(m); err != nil {
			return fmt.Errorf("emfit: upsert %s: %w", m.MetricType, err)
		}
	}
	return nil
}
