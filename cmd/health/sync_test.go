// ABOUTME: Tests for the health sync command and counting repository decorator.
// ABOUTME: Uses real repos and httptest servers; no mocks.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/health/internal/storage"
	"github.com/spf13/cobra"
)

// --- counting wrapper tests ---

func TestCountingRepoTalliesAdded(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "health.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	cr := newCountingRepo(db)

	m := models.NewMetric(models.MetricHRV, 48.0).WithSource("whoop")
	m.RecordedAt = time.Now().UTC().Truncate(time.Second)

	updated, err := cr.UpsertMetric(m)
	if err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}
	if updated {
		t.Error("first upsert should return updated=false (new row)")
	}

	c := cr.counts[models.MetricHRV]
	if c == nil {
		t.Fatal("no count for MetricHRV")
	}
	if c.added != 1 || c.updated != 0 {
		t.Errorf("counts = added:%d updated:%d, want added:1 updated:0", c.added, c.updated)
	}
}

func TestCountingRepoTalliesUpdated(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "health.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	cr := newCountingRepo(db)
	ts := time.Now().UTC().Truncate(time.Second)

	m := models.NewMetric(models.MetricWeight, 82.0).WithSource("withings")
	m.RecordedAt = ts

	if _, err := cr.UpsertMetric(m); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	m2 := models.NewMetric(models.MetricWeight, 83.0).WithSource("withings")
	m2.RecordedAt = ts

	updated, err := cr.UpsertMetric(m2)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if !updated {
		t.Error("second upsert should return updated=true (same source+type+ts)")
	}

	c := cr.counts[models.MetricWeight]
	if c == nil {
		t.Fatal("no count for MetricWeight")
	}
	if c.added != 1 || c.updated != 1 {
		t.Errorf("counts = added:%d updated:%d, want added:1 updated:1", c.added, c.updated)
	}
}

func TestCountingRepoMultipleTypes(t *testing.T) {
	dir := t.TempDir()
	db, err := storage.Open(filepath.Join(dir, "health.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	cr := newCountingRepo(db)
	ts := time.Now().UTC().Truncate(time.Second)

	types := []models.MetricType{models.MetricHRV, models.MetricSleepHours, models.MetricRecovery}
	for _, mt := range types {
		m := models.NewMetric(mt, 1.0).WithSource("whoop")
		m.RecordedAt = ts
		if _, err := cr.UpsertMetric(m); err != nil {
			t.Fatalf("UpsertMetric %s: %v", mt, err)
		}
	}

	if len(cr.counts) != 3 {
		t.Errorf("len(counts) = %d, want 3", len(cr.counts))
	}
	for _, mt := range types {
		c := cr.counts[mt]
		if c == nil || c.added != 1 {
			t.Errorf("%s: expected added=1, got %+v", mt, c)
		}
	}
}

// --- sync command registration ---

func TestSyncCmdExists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "sync" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sync command to be registered on rootCmd")
	}
}

func TestSyncAuthSubcmdExists(t *testing.T) {
	var foundSync *cobra.Command
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "sync" {
			foundSync = cmd
			break
		}
	}
	if foundSync == nil {
		t.Fatal("sync command not registered")
	}

	found := false
	for _, sub := range foundSync.Commands() {
		if sub.Name() == "auth" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth subcommand under sync")
	}
}

func TestSyncCmdDaysFlag(t *testing.T) {
	flag := syncCmd.Flags().Lookup("days")
	if flag == nil {
		t.Fatal("expected --days flag on sync command")
	}
	if flag.DefValue != "7" {
		t.Errorf("default --days = %q, want %q", flag.DefValue, "7")
	}
}

// --- sync command: unknown provider → error ---

func TestSyncUnknownProvider(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"sync", "garmin"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error should mention 'unknown provider', got: %v", err)
	}
}

// --- sync command: missing credentials → clear error naming the config section ---

func TestSyncWhoopMissingClientID(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// XDG_DATA_HOME is set by setupTestCLI; config.Load() will return a fresh config
	// with no sync.whoop credentials.

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"sync", "whoop"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing Whoop credentials")
	}
	if !strings.Contains(err.Error(), "sync.whoop") {
		t.Errorf("error should mention 'sync.whoop', got: %v", err)
	}
}

func TestSyncWithingsMissingClientID(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"sync", "withings"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing Withings credentials")
	}
	if !strings.Contains(err.Error(), "sync.withings") {
		t.Errorf("error should mention 'sync.withings', got: %v", err)
	}
}

func TestSyncEmfitMissingDeviceID(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	// Set emfit token but no device_id via env-redirected config.
	// Since no config file exists yet, defaults will be used → no device_id.
	rootCmd.SetArgs([]string{"sync", "emfit"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing Emfit device_id")
	}
	if !strings.Contains(err.Error(), "device_id") {
		t.Errorf("error should mention 'device_id', got: %v", err)
	}
}

func TestSyncEmfitMissingCreds(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Write a config with device_id but no token and no username/password.
	configDir := filepath.Join(os.Getenv("XDG_DATA_HOME"), "health")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Set XDG_CONFIG_HOME too so config.Load() writes there.
	origCfgHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", os.Getenv("XDG_DATA_HOME"))
	defer os.Setenv("XDG_CONFIG_HOME", origCfgHome)

	cfgPath := filepath.Join(os.Getenv("XDG_DATA_HOME"), "health", "config.json")
	cfgData := `{"backend":"sqlite","sync":{"emfit":{"device_id":"DEV123"}}}`
	if err := os.WriteFile(cfgPath, []byte(cfgData), 0o600); err != nil {
		t.Fatalf("WriteFile config: %v", err)
	}

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"sync", "emfit"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for emfit with device_id but no credentials")
	}
	if !strings.Contains(err.Error(), "token") && !strings.Contains(err.Error(), "username") {
		t.Errorf("error should mention credentials, got: %v", err)
	}
}

// --- auth command: missing credentials → clear error ---

func TestSyncAuthWhoopMissingClientID(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"sync", "auth", "whoop"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing Whoop auth credentials")
	}
	if !strings.Contains(err.Error(), "sync.whoop") {
		t.Errorf("error should mention 'sync.whoop', got: %v", err)
	}
}

func TestSyncAuthWithingsMissingClientID(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"sync", "auth", "withings"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing Withings auth credentials")
	}
	if !strings.Contains(err.Error(), "sync.withings") {
		t.Errorf("error should mention 'sync.withings', got: %v", err)
	}
}

func TestSyncAuthUnknownProvider(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"sync", "auth", "emfit"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for emfit auth (not an OAuth provider)")
	}
	if !strings.Contains(err.Error(), "emfit") {
		t.Errorf("error should mention 'emfit', got: %v", err)
	}
}

// cobra import for type assertion in test.
var _ = (*cobra.Command)(nil)
