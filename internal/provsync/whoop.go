// ABOUTME: Whoop provider: fetches recovery, sleep, and cycle data from the Whoop v2 API.
// ABOUTME: Uses the TokenStore for serialized OAuth2 refresh; writes metrics via UpsertMetric.
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
	// WhoopAPIBaseURL is the production Whoop API host. Endpoint paths already
	// include the /developer/v2/... prefix, so the base must be the bare host.
	// Scopes required: read:recovery read:sleep read:cycles offline.
	WhoopAPIBaseURL = "https://api.prod.whoop.com"

	// WhoopTokenURL is the production OAuth2 token endpoint.
	WhoopTokenURL = "https://api.prod.whoop.com/oauth/oauth2/token" //nolint:gosec // URL, not a credential

	whoopProviderKey = "whoop"

	// httpTimeout is applied to every outbound HTTP call.
	httpTimeout = 30 * time.Second
)

// WhoopClient fetches health metrics from the Whoop v2 API.
// Construct with NewWhoopClient; call Sync to pull a date window.
type WhoopClient struct {
	apiBase      string
	tokenURL     string
	clientID     string
	clientSecret string
	store        *TokenStore
	httpClient   *http.Client
}

// NewWhoopClient returns a WhoopClient that talks to apiBaseURL and refreshes
// tokens against tokenURL. apiBaseURL and tokenURL are constructor parameters
// so tests can inject httptest servers; use WhoopAPIBaseURL and WhoopTokenURL
// for production. clientID and clientSecret are sent on every token refresh.
func NewWhoopClient(apiBaseURL, tokenURL, clientID, clientSecret string, store *TokenStore) *WhoopClient {
	return &WhoopClient{
		apiBase:      apiBaseURL,
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		store:        store,
		httpClient:   &http.Client{Timeout: httpTimeout},
	}
}

// Close releases resources (currently a no-op; present for future pooling).
func (c *WhoopClient) Close() {}

// Sync fetches recovery, sleep, and cycle records in [start, end) from Whoop
// and upserts them into repo. It is idempotent: re-syncing the same window
// will not create duplicate rows.
func (c *WhoopClient) Sync(repo storage.Repository, start, end time.Time) error {
	tok, err := c.store.EnsureFresh(whoopProviderKey, c.refreshToken)
	if err != nil {
		return fmt.Errorf("whoop: ensure token: %w", err)
	}

	if err := c.syncRecovery(repo, tok, start, end); err != nil {
		return err
	}
	if err := c.syncSleep(repo, tok, start, end); err != nil {
		return err
	}
	if err := c.syncCycle(repo, tok, start, end); err != nil {
		return err
	}
	return nil
}

// --- recovery ---

type whoopRecoveryEnvelope struct {
	Records   []whoopRecoveryRecord `json:"records"`
	NextToken string                `json:"next_token"`
}

type whoopRecoveryRecord struct {
	CreatedAt  string              `json:"created_at"`
	ScoreState string              `json:"score_state"`
	Score      *whoopRecoveryScore `json:"score"`
}

type whoopRecoveryScore struct {
	RecoveryScore    float64 `json:"recovery_score"`
	RestingHeartRate float64 `json:"resting_heart_rate"`
	HRVRmssdMilli    float64 `json:"hrv_rmssd_milli"`
	Spo2Percentage   float64 `json:"spo2_percentage"`
}

func (c *WhoopClient) syncRecovery(repo storage.Repository, tok Token, start, end time.Time) error {
	nextToken := ""
	for {
		var env whoopRecoveryEnvelope
		if err := c.fetchPage(tok, "/developer/v2/recovery", start, end, nextToken, &env); err != nil {
			return fmt.Errorf("whoop recovery: %w", err)
		}
		for _, rec := range env.Records {
			if rec.ScoreState != "SCORED" || rec.Score == nil {
				continue
			}
			ts, err := time.Parse(time.RFC3339, rec.CreatedAt)
			if err != nil {
				// Whoop timestamps include milliseconds; try the extended form.
				ts, err = time.Parse("2006-01-02T15:04:05.000Z", rec.CreatedAt)
				if err != nil {
					return fmt.Errorf("whoop recovery: parse timestamp %q: %w", rec.CreatedAt, err)
				}
			}
			metrics := []*models.Metric{
				models.NewMetric(models.MetricRecovery, rec.Score.RecoveryScore).
					WithSource(models.SourceWhoop).WithRecordedAt(ts),
				models.NewMetric(models.MetricHRV, rec.Score.HRVRmssdMilli).
					WithSource(models.SourceWhoop).WithRecordedAt(ts),
				models.NewMetric(models.MetricHeartRate, rec.Score.RestingHeartRate).
					WithSource(models.SourceWhoop).WithRecordedAt(ts),
				models.NewMetric(models.MetricSpO2, rec.Score.Spo2Percentage).
					WithSource(models.SourceWhoop).WithRecordedAt(ts),
			}
			for _, m := range metrics {
				if _, err := repo.UpsertMetric(m); err != nil {
					return fmt.Errorf("whoop recovery: upsert %s: %w", m.MetricType, err)
				}
			}
		}
		if env.NextToken == "" {
			break
		}
		nextToken = env.NextToken
	}
	return nil
}

// --- sleep ---

type whoopSleepEnvelope struct {
	Records   []whoopSleepRecord `json:"records"`
	NextToken string             `json:"next_token"`
}

type whoopSleepRecord struct {
	Start      string           `json:"start"`
	End        string           `json:"end"`
	Nap        bool             `json:"nap"`
	ScoreState string           `json:"score_state"`
	Score      *whoopSleepScore `json:"score"`
}

type whoopSleepScore struct {
	StageSummary    whoopStageSummary `json:"stage_summary"`
	RespiratoryRate float64           `json:"respiratory_rate"`
}

type whoopStageSummary struct {
	TotalInBedTimeMilli float64 `json:"total_in_bed_time_milli"`
	TotalAwakeTimeMilli float64 `json:"total_awake_time_milli"`
}

func (c *WhoopClient) syncSleep(repo storage.Repository, tok Token, start, end time.Time) error {
	nextToken := ""
	for {
		var env whoopSleepEnvelope
		if err := c.fetchPage(tok, "/developer/v2/activity/sleep", start, end, nextToken, &env); err != nil {
			return fmt.Errorf("whoop sleep: %w", err)
		}
		for _, rec := range env.Records {
			if rec.ScoreState != "SCORED" || rec.Score == nil {
				continue
			}
			if rec.Nap {
				continue // skip naps; nightly sleep only
			}
			ts, err := parseWhoopTime(rec.End)
			if err != nil {
				return fmt.Errorf("whoop sleep: parse end %q: %w", rec.End, err)
			}
			// sleep_hours = (total_in_bed_time_milli − total_awake_time_milli) / 3_600_000
			ss := rec.Score.StageSummary
			sleepHours := (ss.TotalInBedTimeMilli - ss.TotalAwakeTimeMilli) / 3_600_000.0

			metrics := []*models.Metric{
				models.NewMetric(models.MetricSleepHours, sleepHours).
					WithSource(models.SourceWhoop).WithRecordedAt(ts),
				models.NewMetric(models.MetricRespiratoryRate, rec.Score.RespiratoryRate).
					WithSource(models.SourceWhoop).WithRecordedAt(ts),
			}
			for _, m := range metrics {
				if _, err := repo.UpsertMetric(m); err != nil {
					return fmt.Errorf("whoop sleep: upsert %s: %w", m.MetricType, err)
				}
			}
		}
		if env.NextToken == "" {
			break
		}
		nextToken = env.NextToken
	}
	return nil
}

// --- cycle ---

type whoopCycleEnvelope struct {
	Records   []whoopCycleRecord `json:"records"`
	NextToken string             `json:"next_token"`
}

type whoopCycleRecord struct {
	Start      string           `json:"start"`
	End        string           `json:"end"`
	ScoreState string           `json:"score_state"`
	Score      *whoopCycleScore `json:"score"`
}

type whoopCycleScore struct {
	Strain float64 `json:"strain"`
}

func (c *WhoopClient) syncCycle(repo storage.Repository, tok Token, start, end time.Time) error {
	nextToken := ""
	for {
		var env whoopCycleEnvelope
		if err := c.fetchPage(tok, "/developer/v2/cycle", start, end, nextToken, &env); err != nil {
			return fmt.Errorf("whoop cycle: %w", err)
		}
		for _, rec := range env.Records {
			if rec.ScoreState != "SCORED" || rec.Score == nil {
				continue
			}
			ts, err := parseWhoopTime(rec.Start)
			if err != nil {
				return fmt.Errorf("whoop cycle: parse start %q: %w", rec.Start, err)
			}
			m := models.NewMetric(models.MetricStrain, rec.Score.Strain).
				WithSource(models.SourceWhoop).WithRecordedAt(ts)
			if _, err := repo.UpsertMetric(m); err != nil {
				return fmt.Errorf("whoop cycle: upsert strain: %w", err)
			}
		}
		if env.NextToken == "" {
			break
		}
		nextToken = env.NextToken
	}
	return nil
}

// --- HTTP helpers ---

// fetchPage performs a paginated GET against path and decodes the JSON body into dst.
// nextToken is appended as a query param when non-empty.
func (c *WhoopClient) fetchPage(tok Token, path string, start, end time.Time, nextToken string, dst interface{}) error {
	u, err := url.Parse(c.apiBase + path)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	q := u.Query()
	q.Set("start", start.UTC().Format(time.RFC3339))
	q.Set("end", end.UTC().Format(time.RFC3339))
	q.Set("limit", "25")
	if nextToken != "" {
		q.Set("nextToken", nextToken)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", tok.TokenType+" "+tok.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", u.Path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body %s: %w", u.Path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s %s: status %d: %s", http.MethodGet, u.Path, resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode %s: %w", u.Path, err)
	}
	return nil
}

// --- token refresh ---

// tokenRefreshResponse is the OAuth2 token endpoint wire format.
type tokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// refreshToken performs a standard OAuth2 refresh_token grant against the
// Whoop token endpoint and returns the new token. Refresh tokens are
// single-use and rotate on every call — the TokenStore's EnsureFresh
// persists the result before the caller uses it.
func (c *WhoopClient) refreshToken(old Token) (Token, error) {
	params := url.Values{}
	params.Set("grant_type", "refresh_token")
	params.Set("refresh_token", old.RefreshToken)
	params.Set("client_id", c.clientID)
	params.Set("client_secret", c.clientSecret)

	resp, err := c.httpClient.PostForm(c.tokenURL, params)
	if err != nil {
		return Token{}, fmt.Errorf("whoop refresh: POST token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("whoop refresh: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("whoop refresh: status %d: %s", resp.StatusCode, body)
	}

	var r tokenRefreshResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return Token{}, fmt.Errorf("whoop refresh: decode response: %w", err)
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

// --- utilities ---

// parseWhoopTime parses a Whoop timestamp, accepting RFC3339 and the
// millisecond variant Whoop returns ("2006-01-02T15:04:05.000Z").
func parseWhoopTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006-01-02T15:04:05.000Z", s)
	if err == nil {
		return t, nil
	}
	// Last attempt: ignore sub-second precision via a broader layout.
	t, err = time.Parse("2006-01-02T15:04:05.999999999Z", s)
	if err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized Whoop timestamp %q", s)
}
