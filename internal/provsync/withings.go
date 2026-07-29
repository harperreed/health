// ABOUTME: Withings provider: fetches measures (weight, body fat) and sleep summaries from the Withings API.
// ABOUTME: Hand-rolls OAuth2 token exchange (nonstandard {status,body} envelope); writes metrics via UpsertMetric.
package provsync

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/health/internal/storage"
)

const (
	// WithingsAPIBaseURL is the production Withings API host.
	// Scopes required: user.info,user.metrics,user.activity.
	WithingsAPIBaseURL = "https://wbsapi.withings.net"

	// WithingsTokenURL is the production OAuth2 token endpoint (nonstandard).
	// All calls POST action=requesttoken with the {status,body} JSON envelope.
	WithingsTokenURL = "https://wbsapi.withings.net/v2/oauth2" //nolint:gosec // URL, not a credential

	withingsProviderKey = "withings"
)

// WithingsClient fetches health metrics from the Withings API.
// Construct with NewWithingsClient; call Sync to pull a date window.
type WithingsClient struct {
	apiBase      string
	tokenURL     string
	clientID     string
	clientSecret string
	store        *TokenStore
	httpClient   *http.Client
}

// NewWithingsClient returns a WithingsClient that talks to apiBaseURL and refreshes
// tokens against tokenURL. apiBaseURL and tokenURL are constructor parameters so
// tests can inject httptest servers; use WithingsAPIBaseURL and WithingsTokenURL
// for production. clientID and clientSecret are sent on every token refresh.
func NewWithingsClient(apiBaseURL, tokenURL, clientID, clientSecret string, store *TokenStore) *WithingsClient {
	return &WithingsClient{
		apiBase:      apiBaseURL,
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		store:        store,
		httpClient:   &http.Client{Timeout: httpTimeout},
	}
}

// Close releases resources (currently a no-op; present for future pooling).
func (c *WithingsClient) Close() {}

// Sync fetches measures and sleep summaries in [start, end) from Withings
// and upserts them into repo. It is idempotent: re-syncing the same window
// will not create duplicate rows.
func (c *WithingsClient) Sync(repo storage.Repository, start, end time.Time) error {
	tok, err := c.store.EnsureFresh(withingsProviderKey, c.refreshToken)
	if err != nil {
		return fmt.Errorf("withings: ensure token: %w", err)
	}

	if err := c.syncMeasures(repo, tok, start, end); err != nil {
		return err
	}
	if err := c.syncSleep(repo, tok, start, end); err != nil {
		return err
	}
	return nil
}

// --- measures ---

// withingsMeasureEnvelope is the getmeas response envelope.
type withingsMeasureEnvelope struct {
	Status int                 `json:"status"`
	Body   withingsMeasureBody `json:"body"`
}

type withingsMeasureBody struct {
	Updatetime  int64                  `json:"updatetime"`
	Timezone    string                 `json:"timezone"`
	MeasureGrps []withingsMeasureGroup `json:"measuregrps"`
	More        int                    `json:"more"`
	Offset      int                    `json:"offset"`
}

type withingsMeasureGroup struct {
	GrpID    int               `json:"grpid"`
	Attrib   int               `json:"attrib"`
	Date     int64             `json:"date"`
	Created  int64             `json:"created"`
	Category int               `json:"category"`
	Measures []withingsMeasure `json:"measures"`
}

type withingsMeasure struct {
	Value int64 `json:"value"`
	Type  int   `json:"type"`
	Unit  int   `json:"unit"`
}

func (c *WithingsClient) syncMeasures(repo storage.Repository, tok Token, start, end time.Time) error {
	params := url.Values{}
	params.Set("action", "getmeas")
	params.Set("meastypes", "1,6")
	params.Set("category", "1")
	params.Set("startdate", fmt.Sprintf("%d", start.Unix()))
	params.Set("enddate", fmt.Sprintf("%d", end.Unix()))

	var env withingsMeasureEnvelope
	if err := c.postForm(tok, "/measure", params, &env); err != nil {
		return fmt.Errorf("withings measures: %w", err)
	}
	if env.Status != 0 {
		return fmt.Errorf("withings measures: API status %d", env.Status)
	}

	for _, grp := range env.Body.MeasureGrps {
		ts := time.Unix(grp.Date, 0)
		for _, m := range grp.Measures {
			var mt models.MetricType
			switch m.Type {
			case 1:
				mt = models.MetricWeight
			case 6:
				mt = models.MetricBodyFat
			default:
				continue // unmapped measure type; skip
			}
			val := withingsRealValue(m.Value, m.Unit)
			metric := models.NewMetric(mt, val).
				WithSource(models.SourceWithings).
				WithRecordedAt(ts)
			if _, err := repo.UpsertMetric(metric); err != nil {
				return fmt.Errorf("withings measures: upsert %s: %w", mt, err)
			}
		}
	}
	return nil
}

// --- sleep ---

// withingsSleepEnvelope is the getsummary response envelope.
type withingsSleepEnvelope struct {
	Status int               `json:"status"`
	Body   withingsSleepBody `json:"body"`
}

type withingsSleepBody struct {
	Series []withingsSleepSeries `json:"series"`
	More   int                   `json:"more"`
	Offset int                   `json:"offset"`
}

type withingsSleepSeries struct {
	Timezone  string            `json:"timezone"`
	Model     int               `json:"model"`
	Startdate int64             `json:"startdate"`
	Enddate   int64             `json:"enddate"`
	Date      string            `json:"date"`
	Created   int64             `json:"created"`
	Modified  int64             `json:"modified"`
	Data      withingsSleepData `json:"data"`
}

// withingsSleepData holds the data_fields from getsummary.
// hr_average and rr_average are pointers so we can detect their absence.
type withingsSleepData struct {
	LightSleepDuration int  `json:"lightsleepduration"`
	DeepSleepDuration  int  `json:"deepsleepduration"`
	RemSleepDuration   int  `json:"remsleepduration"`
	HRAverage          *int `json:"hr_average"`
	RRAverage          *int `json:"rr_average"`
}

func (c *WithingsClient) syncSleep(repo storage.Repository, tok Token, start, end time.Time) error {
	params := url.Values{}
	params.Set("action", "getsummary")
	params.Set("startdateymd", start.UTC().Format("2006-01-02"))
	params.Set("enddateymd", end.UTC().Format("2006-01-02"))
	params.Set("data_fields", "lightsleepduration,deepsleepduration,remsleepduration,hr_average,rr_average,sleep_score")

	var env withingsSleepEnvelope
	if err := c.postForm(tok, "/v2/sleep", params, &env); err != nil {
		return fmt.Errorf("withings sleep: %w", err)
	}
	if env.Status != 0 {
		return fmt.Errorf("withings sleep: API status %d", env.Status)
	}

	for _, s := range env.Body.Series {
		ts := time.Unix(s.Enddate, 0)

		// sleep_hours = (light + deep + rem) / 3600  (all values are seconds)
		totalSec := s.Data.LightSleepDuration + s.Data.DeepSleepDuration + s.Data.RemSleepDuration
		sleepHours := float64(totalSec) / 3600.0
		shMetric := models.NewMetric(models.MetricSleepHours, sleepHours).
			WithSource(models.SourceWithings).
			WithRecordedAt(ts)
		if _, err := repo.UpsertMetric(shMetric); err != nil {
			return fmt.Errorf("withings sleep: upsert sleep_hours: %w", err)
		}

		// hr_average → heart_rate (only when present in the series data)
		if s.Data.HRAverage != nil {
			hrMetric := models.NewMetric(models.MetricHeartRate, float64(*s.Data.HRAverage)).
				WithSource(models.SourceWithings).
				WithRecordedAt(ts)
			if _, err := repo.UpsertMetric(hrMetric); err != nil {
				return fmt.Errorf("withings sleep: upsert heart_rate: %w", err)
			}
		}

		// rr_average → respiratory_rate (only when present)
		if s.Data.RRAverage != nil {
			rrMetric := models.NewMetric(models.MetricRespiratoryRate, float64(*s.Data.RRAverage)).
				WithSource(models.SourceWithings).
				WithRecordedAt(ts)
			if _, err := repo.UpsertMetric(rrMetric); err != nil {
				return fmt.Errorf("withings sleep: upsert respiratory_rate: %w", err)
			}
		}
	}
	return nil
}

// --- HTTP helpers ---

// postForm POSTs params to c.apiBase+path, decodes the JSON body into dst, and
// returns an error if the HTTP transport itself fails. The caller checks the
// Withings-level status field (always HTTP 200; errors via status != 0).
func (c *WithingsClient) postForm(tok Token, path string, params url.Values, dst interface{}) error {
	reqURL := c.apiBase + path
	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", tok.TokenType+" "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Attach params as body.
	bodyReader, bodyWriter := io.Pipe()
	go func() {
		_, werr := fmt.Fprint(bodyWriter, params.Encode())
		bodyWriter.CloseWithError(werr)
	}()
	req.Body = bodyReader

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body %s: %w", path, err)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// --- token refresh ---

// withingsTokenEnvelope is the nonstandard Withings token response wrapper.
// All Withings API responses are HTTP 200; errors signal via status != 0.
type withingsTokenEnvelope struct {
	Status int                  `json:"status"`
	Body   withingsTokenPayload `json:"body"`
}

type withingsTokenPayload struct {
	UserID       string `json:"userid"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// refreshToken performs a Withings-flavored refresh_token grant and returns
// the new token. Withings tokens rotate on every refresh — the TokenStore's
// EnsureFresh persists the result before the caller uses it.
func (c *WithingsClient) refreshToken(old Token) (Token, error) {
	params := url.Values{}
	params.Set("action", "requesttoken")
	params.Set("grant_type", "refresh_token")
	params.Set("client_id", c.clientID)
	params.Set("client_secret", c.clientSecret)
	params.Set("refresh_token", old.RefreshToken)

	resp, err := c.httpClient.PostForm(c.tokenURL, params)
	if err != nil {
		return Token{}, fmt.Errorf("withings refresh: POST token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, fmt.Errorf("withings refresh: read body: %w", err)
	}

	var env withingsTokenEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Token{}, fmt.Errorf("withings refresh: decode response: %w", err)
	}
	// Nonzero status is an application-level error (always HTTP 200).
	if env.Status != 0 {
		return Token{}, fmt.Errorf("withings refresh: API status %d", env.Status)
	}

	tokenType := env.Body.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}
	expiresIn := env.Body.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 10800 // Withings access tokens are ~3h
	}

	return Token{
		AccessToken:  env.Body.AccessToken,
		RefreshToken: env.Body.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		TokenType:    tokenType,
	}, nil
}

// --- utilities ---

// withingsRealValue converts a Withings measure value and signed unit exponent
// to the real floating-point value: real = value × 10^unit.
// For example: value=82500, unit=-3 → 82.5.
func withingsRealValue(value int64, unit int) float64 {
	return float64(value) * math.Pow10(unit)
}
