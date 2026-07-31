// ABOUTME: CLI commands for syncing health data from native providers (Whoop, Withings, Emfit).
// ABOUTME: Includes oauth auth subcommand for initial token acquisition.
package main

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/harperreed/health/internal/config"
	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/health/internal/provsync"
	"github.com/harperreed/health/internal/storage"
)

var syncDays int

var syncCmd = &cobra.Command{
	Use:   "sync <provider>",
	Short: "Sync health metrics from a native provider",
	Long: `Sync health data from Whoop, Withings, or Emfit into local storage.

PROVIDERS:
  whoop      Fetches recovery, sleep, and cycle data (OAuth2; run 'health sync auth whoop' first)
  withings   Fetches measurements and sleep summaries (OAuth2; run 'health sync auth withings' first)
  emfit      Fetches the latest sleep night from Emfit QS (config token or username/password)

EXAMPLES:
  health sync whoop             # Sync last 7 days from Whoop
  health sync withings --days 30
  health sync emfit

SETUP:
  See 'health sync auth --help' for the OAuth setup walkthrough for Whoop and Withings.
  Emfit credentials go directly in config.json (see README for details).

NOTE: Sync commands are the ONLY commands that touch the network.`,
	Args: cobra.ExactArgs(1),
	RunE: runSync,
}

var syncAuthCmd = &cobra.Command{
	Use:   "auth <provider>",
	Short: "Authorize a provider via OAuth (whoop or withings)",
	Long: `Run the OAuth2 authorization flow for whoop or withings.

Opens a localhost callback server, prints the authorize URL (for remote/tailscale use),
waits for the browser redirect, exchanges the code for a token, and saves it.

EXAMPLES:
  health sync auth whoop
  health sync auth withings

CONFIG:
  ClientID, ClientSecret, and optionally RedirectURI come from config.json:
    {
      "sync": {
        "whoop": { "client_id": "...", "client_secret": "..." },
        "withings": { "client_id": "...", "client_secret": "..." }
      }
    }

  The default redirect URI is http://localhost:42021/callback.
  Override per-provider via "redirect_uri" in the config.`,
	Args: cobra.ExactArgs(1),
	RunE: runSyncAuth,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncAuthCmd)
	syncCmd.Flags().IntVar(&syncDays, "days", 7, "number of days to sync (end = now, start = now − days)")
}

// --- sync run ---

func runSync(cmd *cobra.Command, args []string) error {
	provider := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	end := time.Now()
	start := end.AddDate(0, 0, -syncDays)

	counter := newCountingRepo(repo)

	switch provider {
	case "whoop":
		if err := runSyncWhoop(cfg, counter, start, end); err != nil {
			return err
		}
	case "withings":
		if err := runSyncWithings(cfg, counter, start, end); err != nil {
			return err
		}
	case "emfit":
		if err := runSyncEmfit(cfg, counter, start, end); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown provider %q: must be whoop, withings, or emfit", provider)
	}

	printSyncSummary(cmd, provider, counter)
	return nil
}

func runSyncWhoop(cfg *config.Config, repo storage.Repository, start, end time.Time) error {
	pcfg := cfg.Sync.Whoop
	if pcfg.ClientID == "" || pcfg.ClientSecret == "" {
		return fmt.Errorf("whoop sync: client_id and client_secret must be set in config.json under sync.whoop")
	}
	store := provsync.NewTokenStore(cfg.GetDataDir())
	client := provsync.NewWhoopClient(
		provsync.WhoopAPIBaseURL,
		provsync.WhoopTokenURL,
		pcfg.ClientID,
		pcfg.ClientSecret,
		store,
	)
	defer client.Close()
	return client.Sync(repo, start, end)
}

func runSyncWithings(cfg *config.Config, repo storage.Repository, start, end time.Time) error {
	pcfg := cfg.Sync.Withings
	if pcfg.ClientID == "" || pcfg.ClientSecret == "" {
		return fmt.Errorf("withings sync: client_id and client_secret must be set in config.json under sync.withings")
	}
	store := provsync.NewTokenStore(cfg.GetDataDir())
	client := provsync.NewWithingsClient(
		provsync.WithingsAPIBaseURL,
		provsync.WithingsTokenURL,
		pcfg.ClientID,
		pcfg.ClientSecret,
		store,
	)
	defer client.Close()
	return client.Sync(repo, start, end)
}

func runSyncEmfit(cfg *config.Config, repo storage.Repository, start, end time.Time) error {
	ecfg := cfg.Sync.Emfit
	if ecfg.DeviceID == "" {
		return fmt.Errorf("emfit sync: device_id must be set in config.json under sync.emfit")
	}

	var client *provsync.EmfitClient
	switch {
	case ecfg.Token != "":
		client = provsync.NewEmfitClient(provsync.EmfitAPIBaseURL, ecfg.Token, ecfg.DeviceID)
	case ecfg.Username != "" && ecfg.Password != "":
		client = provsync.NewEmfitClientWithLogin(provsync.EmfitAPIBaseURL, ecfg.Username, ecfg.Password, ecfg.DeviceID)
	default:
		return fmt.Errorf("emfit sync: set either token or username+password in config.json under sync.emfit")
	}
	defer client.Close()
	return client.Sync(repo, start, end)
}

func printSyncSummary(cmd *cobra.Command, provider string, counter *countingRepo) {
	color.Green("Sync complete for %s:", provider)
	if len(counter.counts) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  (no metrics in the requested window)")
		return
	}
	for mt, c := range counter.counts {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-24s  added: %d  updated: %d\n", string(mt), c.added, c.updated)
	}
}

// --- auth flow ---

func runSyncAuth(cmd *cobra.Command, args []string) error {
	provider := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	store := provsync.NewTokenStore(cfg.GetDataDir())

	switch provider {
	case "whoop":
		return runAuthWhoop(cmd, cfg, store)
	case "withings":
		return runAuthWithings(cmd, cfg, store)
	default:
		return fmt.Errorf("auth is only supported for whoop and withings (emfit uses config credentials)")
	}
}

func runAuthWhoop(cmd *cobra.Command, cfg *config.Config, store *provsync.TokenStore) error {
	pcfg := cfg.Sync.Whoop
	if pcfg.ClientID == "" || pcfg.ClientSecret == "" {
		return fmt.Errorf("whoop auth: client_id and client_secret must be set in config.json under sync.whoop")
	}
	redirectURI := pcfg.RedirectURI
	if redirectURI == "" {
		redirectURI = provsync.DefaultRedirectURI
	}
	urls := provsync.AuthorizeURLs{
		AuthURL:      provsync.WhoopAuthorizeURL,
		TokenURL:     provsync.WhoopTokenURL,
		ClientID:     pcfg.ClientID,
		ClientSecret: pcfg.ClientSecret,
		RedirectURI:  redirectURI,
		Scopes:       provsync.WhoopScopes,
	}
	flow := provsync.NewOAuthFlow("whoop", urls, store, provsync.ExchangeWhoop, "")
	return flow.Run(cmd.OutOrStdout())
}

func runAuthWithings(cmd *cobra.Command, cfg *config.Config, store *provsync.TokenStore) error {
	pcfg := cfg.Sync.Withings
	if pcfg.ClientID == "" || pcfg.ClientSecret == "" {
		return fmt.Errorf("withings auth: client_id and client_secret must be set in config.json under sync.withings")
	}
	redirectURI := pcfg.RedirectURI
	if redirectURI == "" {
		redirectURI = provsync.DefaultRedirectURI
	}
	urls := provsync.AuthorizeURLs{
		AuthURL:      provsync.WithingsAuthorizeURL,
		TokenURL:     provsync.WithingsTokenURL,
		ClientID:     pcfg.ClientID,
		ClientSecret: pcfg.ClientSecret,
		RedirectURI:  redirectURI,
		Scopes:       provsync.WithingsScopes,
	}
	flow := provsync.NewOAuthFlow("withings", urls, store, provsync.ExchangeWithings, "")
	return flow.Run(cmd.OutOrStdout())
}

// --- counting repository decorator ---

// metricCount tracks added and updated counts for a single metric type.
type metricCount struct {
	added   int
	updated int
}

// countingRepo wraps a storage.Repository and tallies UpsertMetric calls
// by metric type. UpsertMetric returns true when an existing row was updated.
type countingRepo struct {
	storage.Repository
	counts map[models.MetricType]*metricCount
}

func newCountingRepo(base storage.Repository) *countingRepo {
	return &countingRepo{
		Repository: base,
		counts:     make(map[models.MetricType]*metricCount),
	}
}

// UpsertMetric delegates to the underlying repository and records the result.
func (r *countingRepo) UpsertMetric(m *models.Metric) (bool, error) {
	updated, err := r.Repository.UpsertMetric(m)
	if err != nil {
		return false, err
	}
	c := r.counts[m.MetricType]
	if c == nil {
		c = &metricCount{}
		r.counts[m.MetricType] = c
	}
	if updated {
		c.updated++
	} else {
		c.added++
	}
	return updated, nil
}
