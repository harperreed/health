// ABOUTME: Tests for CLI helper functions and command execution.
// ABOUTME: Tests parseTime, truncate, padRight, and command flags.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/harperreed/health/internal/models"
	"github.com/harperreed/health/internal/storage"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "date and time with space",
			input:   "2025-01-31 08:30",
			wantErr: false,
		},
		{
			name:    "date and time with T",
			input:   "2025-01-31T08:30",
			wantErr: false,
		},
		{
			name:    "date only",
			input:   "2025-01-31",
			wantErr: false,
		},
		{
			name:    "RFC3339",
			input:   "2025-01-31T08:30:00Z",
			wantErr: false,
		},
		{
			name:    "RFC3339 with offset",
			input:   "2025-01-31T08:30:00+05:00",
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "31-01-2025",
			wantErr: true,
		},
		{
			name:    "invalid random string",
			input:   "not a date",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTime(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseTime(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseTime(%q) unexpected error: %v", tt.input, err)
				return
			}

			if result.IsZero() {
				t.Errorf("parseTime(%q) returned zero time", tt.input)
			}
		})
	}
}

func TestParseTimeValues(t *testing.T) {
	// Test specific date value parsing
	result, err := parseTime("2025-06-15")
	if err != nil {
		t.Fatalf("parseTime failed: %v", err)
	}

	if result.Year() != 2025 || result.Month() != time.June || result.Day() != 15 {
		t.Errorf("parseTime returned wrong date: got %v", result)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string no truncation",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "needs truncation",
			input:  "hello world this is a long string",
			maxLen: 10,
			want:   "hello w...",
		},
		{
			name:   "truncate at boundary",
			input:  "abcdefghij",
			maxLen: 6,
			want:   "abc...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "very short maxLen",
			input:  "hello",
			maxLen: 3,
			want:   "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		length int
		want   string
	}{
		{
			name:   "needs padding",
			input:  "hi",
			length: 5,
			want:   "hi   ",
		},
		{
			name:   "exact length",
			input:  "hello",
			length: 5,
			want:   "hello",
		},
		{
			name:   "longer than length",
			input:  "hello world",
			length: 5,
			want:   "hello world",
		},
		{
			name:   "empty string",
			input:  "",
			length: 5,
			want:   "     ",
		},
		{
			name:   "zero length",
			input:  "hello",
			length: 0,
			want:   "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padRight(tt.input, tt.length)
			if got != tt.want {
				t.Errorf("padRight(%q, %d) = %q, want %q", tt.input, tt.length, got, tt.want)
			}
		})
	}
}

func TestRootCmdFlags(t *testing.T) {
	// Verify root command is properly initialized
	if rootCmd.Use != "health" {
		t.Errorf("rootCmd.Use = %q, want %q", rootCmd.Use, "health")
	}

	if rootCmd.Short == "" {
		t.Error("Expected rootCmd.Short to be non-empty")
	}
}

func TestAddCmdFlags(t *testing.T) {
	// Verify add command flags
	atFlag := addCmd.Flags().Lookup("at")
	if atFlag == nil {
		t.Error("Expected --at flag on add command")
	}

	notesFlag := addCmd.Flags().Lookup("notes")
	if notesFlag == nil {
		t.Error("Expected --notes flag on add command")
	}
}

func TestListCmdFlags(t *testing.T) {
	// Verify list command flags
	typeFlag := listCmd.Flags().Lookup("type")
	if typeFlag == nil {
		t.Error("Expected --type flag on list command")
	}

	limitFlag := listCmd.Flags().Lookup("limit")
	if limitFlag == nil {
		t.Fatal("Expected --limit flag on list command")
	}

	// Check default limit value
	if limitFlag.DefValue != "20" {
		t.Errorf("Expected default limit 20, got %s", limitFlag.DefValue)
	}
}

func TestDeleteCmdArgs(t *testing.T) {
	// Verify delete command requires exactly 1 argument
	if deleteCmd.Args == nil {
		t.Error("Expected deleteCmd to have Args validator")
	}
}

func TestWorkoutCmdSubcommands(t *testing.T) {
	// Verify workout command has subcommands
	subcommands := workoutCmd.Commands()
	expectedSubcmds := []string{"add", "delete", "list", "metric", "show"}

	cmdNames := make(map[string]bool)
	for _, cmd := range subcommands {
		cmdNames[cmd.Name()] = true
	}

	for _, expected := range expectedSubcmds {
		if !cmdNames[expected] {
			t.Errorf("Expected workout subcommand %q not found", expected)
		}
	}
}

func TestWorkoutAddCmdFlags(t *testing.T) {
	// Verify workout add command flags
	durationFlag := workoutAddCmd.Flags().Lookup("duration")
	if durationFlag == nil {
		t.Error("Expected --duration flag on workout add command")
	}

	notesFlag := workoutAddCmd.Flags().Lookup("notes")
	if notesFlag == nil {
		t.Error("Expected --notes flag on workout add command")
	}
}

func TestWorkoutListCmdFlags(t *testing.T) {
	// Verify workout list command flags
	typeFlag := workoutListCmd.Flags().Lookup("type")
	if typeFlag == nil {
		t.Error("Expected --type flag on workout list command")
	}

	limitFlag := workoutListCmd.Flags().Lookup("limit")
	if limitFlag == nil {
		t.Error("Expected --limit flag on workout list command")
	}
}

func TestExportCmdFlags(t *testing.T) {
	// Verify export command flags
	outputFlag := exportCmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("Expected --output flag on export command")
	}

	typeFlag := exportCmd.Flags().Lookup("type")
	if typeFlag == nil {
		t.Error("Expected --type flag on export command")
	}

	sinceFlag := exportCmd.Flags().Lookup("since")
	if sinceFlag == nil {
		t.Error("Expected --since flag on export command")
	}
}

func TestAddCmdAliases(t *testing.T) {
	// Verify aliases
	if len(addCmd.Aliases) == 0 {
		t.Error("Expected addCmd to have aliases")
	}

	found := false
	for _, alias := range addCmd.Aliases {
		if alias == "a" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'a' alias for addCmd")
	}
}

func TestListCmdAliases(t *testing.T) {
	// Verify list aliases
	expectedAliases := map[string]bool{"ls": false, "l": false}

	for _, alias := range listCmd.Aliases {
		if _, ok := expectedAliases[alias]; ok {
			expectedAliases[alias] = true
		}
	}

	for alias, found := range expectedAliases {
		if !found {
			t.Errorf("Expected alias %q for listCmd", alias)
		}
	}
}

func TestDeleteCmdAliases(t *testing.T) {
	// Verify delete aliases
	expectedAliases := map[string]bool{"del": false, "rm": false}

	for _, alias := range deleteCmd.Aliases {
		if _, ok := expectedAliases[alias]; ok {
			expectedAliases[alias] = true
		}
	}

	for alias, found := range expectedAliases {
		if !found {
			t.Errorf("Expected alias %q for deleteCmd", alias)
		}
	}
}

func TestWorkoutCmdAliases(t *testing.T) {
	// Verify workout command alias
	found := false
	for _, alias := range workoutCmd.Aliases {
		if alias == "w" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'w' alias for workoutCmd")
	}
}

func TestExportCmdValidArgs(t *testing.T) {
	// Verify valid arguments
	validArgs := exportCmd.ValidArgs
	expected := map[string]bool{"json": false, "yaml": false, "markdown": false}

	for _, arg := range validArgs {
		if _, ok := expected[arg]; ok {
			expected[arg] = true
		}
	}

	for arg, found := range expected {
		if !found {
			t.Errorf("Expected valid arg %q for exportCmd", arg)
		}
	}
}

func TestImportCmdExists(t *testing.T) {
	// Verify import command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "import" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected import command to be registered")
	}
}

func TestInstallSkillCmdExists(t *testing.T) {
	// Verify install-skill command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "install-skill" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected install-skill command to be registered")
	}
}

// setupTestCLI sets up a test database for CLI testing.
// It sets XDG_DATA_HOME to redirect the database to a temp directory.
func setupTestCLI(t *testing.T) (*storage.DB, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "health-cli-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Save original XDG_DATA_HOME and set to our temp dir
	originalXDG := os.Getenv("XDG_DATA_HOME")
	os.Setenv("XDG_DATA_HOME", tmpDir)

	// Pre-open the database to create the schema
	dbPath := filepath.Join(tmpDir, "health", "health.db")
	testDB, err := storage.Open(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		os.Setenv("XDG_DATA_HOME", originalXDG)
		t.Fatalf("Failed to open database: %v", err)
	}

	cleanup := func() {
		if db != nil {
			db.Close()
			db = nil
		}
		testDB.Close()
		os.RemoveAll(tmpDir)
		os.Setenv("XDG_DATA_HOME", originalXDG)
	}

	return testDB, cleanup
}

func TestAddCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	// Test adding a weight metric
	rootCmd.SetArgs([]string{"add", "weight", "82.5"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("add command failed: %v", err)
	}

	// Verify metric was created
	metrics, err := testDB.ListMetrics(nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics failed: %v", err)
	}
	if len(metrics) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Value != 82.5 {
		t.Errorf("Expected value 82.5, got %f", metrics[0].Value)
	}
}

func TestAddCmdWithNotes(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetArgs([]string{"add", "mood", "8", "--notes", "great day!"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("add command with notes failed: %v", err)
	}

	metrics, err := testDB.ListMetrics(nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics failed: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Notes == nil || *metrics[0].Notes != "great day!" {
		t.Error("Notes not set correctly")
	}
}

func TestAddCmdWithTimestamp(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetArgs([]string{"add", "steps", "10000", "--at", "2025-01-31 08:00"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("add command with timestamp failed: %v", err)
	}
}

func TestAddCmdBloodPressure(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetArgs([]string{"add", "bp", "120", "80"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("add bp command failed: %v", err)
	}

	// Should create 2 metrics (bp_sys and bp_dia)
	metrics, err := testDB.ListMetrics(nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics failed: %v", err)
	}
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics for BP, got %d", len(metrics))
	}
}

func TestAddCmdInvalidType(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	// Capture stderr to suppress error output during test
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"add", "invalid_type", "100"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid metric type")
	}
}

func TestAddCmdBPMissingArg(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"add", "bp", "120"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for BP with missing diastolic")
	}
}

func TestAddCmdInvalidValue(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"add", "weight", "not_a_number"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid value")
	}
}

func TestAddCmdInvalidTimestamp(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"add", "weight", "82.5", "--at", "invalid-date"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid timestamp")
	}
}

func TestListCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	listType = ""
	listLimit = 20

	// Create some metrics first
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m2 := models.NewMetric(models.MetricMood, 7)
	testDB.CreateMetric(m1)
	testDB.CreateMetric(m2)

	rootCmd.SetArgs([]string{"list"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("list command failed: %v", err)
	}
}

func TestListCmdWithTypeFilter(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	listType = ""
	listLimit = 20

	// Create metrics of different types
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m2 := models.NewMetric(models.MetricMood, 7)
	testDB.CreateMetric(m1)
	testDB.CreateMetric(m2)

	rootCmd.SetArgs([]string{"list", "--type", "weight"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("list command with type filter failed: %v", err)
	}
}

func TestListCmdWithLimit(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	listType = ""
	listLimit = 20

	// Create multiple metrics
	for i := 0; i < 10; i++ {
		m := models.NewMetric(models.MetricWeight, float64(80+i))
		testDB.CreateMetric(m)
	}

	rootCmd.SetArgs([]string{"list", "--limit", "5"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("list command with limit failed: %v", err)
	}
}

func TestListCmdEmptyDB(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	listType = ""
	listLimit = 20

	rootCmd.SetArgs([]string{"list"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("list command on empty DB failed: %v", err)
	}
}

func TestListCmdInvalidType(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	listType = ""
	listLimit = 20

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"list", "--type", "invalid_type"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid type filter")
	}
}

func TestDeleteCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a metric to delete
	m := models.NewMetric(models.MetricWeight, 82.5)
	testDB.CreateMetric(m)

	rootCmd.SetArgs([]string{"delete", m.ID.String()[:8]})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("delete command failed: %v", err)
	}

	// Verify metric was deleted
	_, err = testDB.GetMetric(m.ID.String())
	if err == nil {
		t.Error("Expected metric to be deleted")
	}
}

func TestDeleteCmdNotFound(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"delete", "nonexistent"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for non-existent metric")
	}
}

func TestWorkoutAddCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	workoutDuration = 0
	workoutNotes = ""

	rootCmd.SetArgs([]string{"workout", "add", "run"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout add command failed: %v", err)
	}

	// Verify workout was created
	workouts, err := testDB.ListWorkouts(nil, 0)
	if err != nil {
		t.Fatalf("ListWorkouts failed: %v", err)
	}
	if len(workouts) != 1 {
		t.Errorf("Expected 1 workout, got %d", len(workouts))
	}
}

func TestWorkoutAddCmdWithOptions(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	workoutDuration = 0
	workoutNotes = ""

	rootCmd.SetArgs([]string{"workout", "add", "lift", "--duration", "45", "--notes", "Leg day"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout add command with options failed: %v", err)
	}

	workouts, err := testDB.ListWorkouts(nil, 0)
	if err != nil {
		t.Fatalf("ListWorkouts failed: %v", err)
	}
	if len(workouts) != 1 {
		t.Fatalf("Expected 1 workout, got %d", len(workouts))
	}
	if workouts[0].DurationMinutes == nil || *workouts[0].DurationMinutes != 45 {
		t.Error("Duration not set correctly")
	}
}

func TestWorkoutListCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	workoutType = ""
	workoutLimit = 20

	// Create some workouts
	w := models.NewWorkout("run")
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "list"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout list command failed: %v", err)
	}
}

func TestWorkoutListCmdEmpty(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	workoutType = ""
	workoutLimit = 20

	rootCmd.SetArgs([]string{"workout", "list"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout list command on empty DB failed: %v", err)
	}
}

func TestWorkoutShowCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a workout with metrics
	w := models.NewWorkout("run")
	w.WithDuration(30)
	testDB.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	testDB.AddWorkoutMetric(wm)

	rootCmd.SetArgs([]string{"workout", "show", w.ID.String()[:8]})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout show command failed: %v", err)
	}
}

func TestWorkoutShowCmdNotFound(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"workout", "show", "nonexistent"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for non-existent workout")
	}
}

func TestWorkoutMetricCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a workout first
	w := models.NewWorkout("run")
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "metric", w.ID.String()[:8], "distance", "5.2", "km"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout metric command failed: %v", err)
	}

	// Verify metric was added
	metrics, err := testDB.ListWorkoutMetrics(w.ID)
	if err != nil {
		t.Fatalf("ListWorkoutMetrics failed: %v", err)
	}
	if len(metrics) != 1 {
		t.Errorf("Expected 1 workout metric, got %d", len(metrics))
	}
}

func TestWorkoutMetricCmdInvalidValue(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	w := models.NewWorkout("run")
	testDB.CreateWorkout(w)

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"workout", "metric", w.ID.String()[:8], "distance", "not_a_number", "km"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid value")
	}
}

func TestWorkoutMetricCmdWorkoutNotFound(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"workout", "metric", "nonexistent", "distance", "5.0", "km"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for non-existent workout")
	}
}

func TestWorkoutDeleteCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a workout to delete
	w := models.NewWorkout("run")
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "delete", w.ID.String()[:8]})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout delete command failed: %v", err)
	}

	// Verify workout was deleted
	_, err = testDB.GetWorkout(w.ID.String())
	if err == nil {
		t.Error("Expected workout to be deleted")
	}
}

func TestWorkoutDeleteCmdNotFound(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"workout", "delete", "nonexistent"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for non-existent workout")
	}
}

func TestExportJSONCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create some data
	m := models.NewMetric(models.MetricWeight, 82.5)
	testDB.CreateMetric(m)

	rootCmd.SetArgs([]string{"export", "json"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("export json command failed: %v", err)
	}
}

func TestExportYAMLCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create some data
	m := models.NewMetric(models.MetricWeight, 82.5)
	testDB.CreateMetric(m)

	rootCmd.SetArgs([]string{"export", "yaml"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("export yaml command failed: %v", err)
	}
}

func TestExportMarkdownCmdWithDB(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	exportType = ""
	exportSince = ""
	exportOutput = ""

	// Create some data
	m := models.NewMetric(models.MetricWeight, 82.5)
	testDB.CreateMetric(m)

	rootCmd.SetArgs([]string{"export", "markdown"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("export markdown command failed: %v", err)
	}
}

func TestExportInvalidFormat(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	exportType = ""
	exportSince = ""
	exportOutput = ""

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"export", "invalid"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid export format")
	}
}

func TestExportToFile(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	exportType = ""
	exportSince = ""
	exportOutput = ""

	// Create some data
	m := models.NewMetric(models.MetricWeight, 82.5)
	testDB.CreateMetric(m)

	tmpFile := filepath.Join(t.TempDir(), "export.json")

	rootCmd.SetArgs([]string{"export", "json", "--output", tmpFile})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("export to file command failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("Expected export file to be created")
	}
}

func TestMigrateCmdDryRun(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	migrateDryRun = false

	rootCmd.SetArgs([]string{"migrate", "--dry-run"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("migrate --dry-run command failed: %v", err)
	}
}

func TestMigrateCmdRegular(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	migrateDryRun = false

	rootCmd.SetArgs([]string{"migrate"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("migrate command failed: %v", err)
	}
}

func TestMcpCmdExists(t *testing.T) {
	// Verify mcp command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "mcp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected mcp command to be registered")
	}
}

func TestMigrateCmdExists(t *testing.T) {
	// Verify migrate command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "migrate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected migrate command to be registered")
	}
}

func TestMigrateCmdDryRunFlag(t *testing.T) {
	flag := migrateCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Error("Expected --dry-run flag on migrate command")
	}
}

func TestAddCmdBPWithTimestamp(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetArgs([]string{"add", "bp", "120", "80", "--at", "2025-01-31 08:00"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("add bp with timestamp failed: %v", err)
	}
}

func TestAddCmdBPWithNotes(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetArgs([]string{"add", "bp", "120", "80", "--notes", "morning reading"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("add bp with notes failed: %v", err)
	}
}

func TestAddCmdBPInvalidSystolic(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"add", "bp", "not_a_number", "80"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid systolic value")
	}
}

func TestAddCmdBPInvalidDiastolic(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"add", "bp", "120", "not_a_number"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid diastolic value")
	}
}

func TestAddCmdBPInvalidTimestamp(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	addAt = ""
	addNotes = ""

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"add", "bp", "120", "80", "--at", "invalid-date"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid timestamp")
	}
}

func TestExportMarkdownWithSince(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	exportType = ""
	exportSince = ""
	exportOutput = ""

	// Create some data
	m := models.NewMetric(models.MetricWeight, 82.5)
	testDB.CreateMetric(m)

	rootCmd.SetArgs([]string{"export", "markdown", "--since", "2025-01-01"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("export markdown with since failed: %v", err)
	}
}

func TestExportMarkdownWithInvalidSince(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	exportType = ""
	exportSince = ""
	exportOutput = ""

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"export", "markdown", "--since", "invalid-date"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid since date")
	}
}

func TestExportMarkdownWithType(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	exportType = ""
	exportSince = ""
	exportOutput = ""

	// Create some data
	m := models.NewMetric(models.MetricWeight, 82.5)
	testDB.CreateMetric(m)

	rootCmd.SetArgs([]string{"export", "markdown", "--type", "weight"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("export markdown with type filter failed: %v", err)
	}
}

func TestImportCmdWithFile(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a valid import file
	tmpDir := t.TempDir()
	importFile := filepath.Join(tmpDir, "import.json")

	jsonData := `{
		"version": "1.0",
		"exported_at": "2025-01-31T12:00:00Z",
		"tool": "health",
		"metrics": [],
		"workouts": []
	}`
	err := os.WriteFile(importFile, []byte(jsonData), 0644)
	if err != nil {
		t.Fatalf("Failed to write import file: %v", err)
	}

	rootCmd.SetArgs([]string{"import", importFile})
	err = rootCmd.Execute()

	if err != nil {
		t.Errorf("import command failed: %v", err)
	}
}

func TestImportCmdFileNotFound(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"import", "/nonexistent/file.json"})
	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestImportCmdInvalidJSON(t *testing.T) {
	_, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create an invalid import file
	tmpDir := t.TempDir()
	importFile := filepath.Join(tmpDir, "invalid.json")

	err := os.WriteFile(importFile, []byte("not valid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write import file: %v", err)
	}

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})

	rootCmd.SetArgs([]string{"import", importFile})
	err = rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestListMetricsWithNotes(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	listType = ""
	listLimit = 20

	// Create metric with notes
	m := models.NewMetric(models.MetricWeight, 82.5)
	m.WithNotes("morning weight")
	testDB.CreateMetric(m)

	rootCmd.SetArgs([]string{"list"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("list command with notes failed: %v", err)
	}
}

func TestWorkoutShowWithNullableDuration(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a workout without duration
	w := models.NewWorkout("run")
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "show", w.ID.String()[:8]})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout show without duration failed: %v", err)
	}
}

func TestWorkoutShowWithNullableNotes(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a workout without notes
	w := models.NewWorkout("run")
	w.WithDuration(30)
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "show", w.ID.String()[:8]})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout show without notes failed: %v", err)
	}
}

func TestWorkoutShowWithNotes(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a workout with notes
	w := models.NewWorkout("run")
	w.WithNotes("Morning run")
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "show", w.ID.String()[:8]})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout show with notes failed: %v", err)
	}
}

func TestWorkoutMetricWithNullableUnit(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a workout with metric that has no unit
	w := models.NewWorkout("lift")
	testDB.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "sets", 4, "")
	testDB.AddWorkoutMetric(wm)

	rootCmd.SetArgs([]string{"workout", "show", w.ID.String()[:8]})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout show with no-unit metric failed: %v", err)
	}
}

func TestWorkoutMetricCmdWithUnit(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a workout first
	w := models.NewWorkout("run")
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "metric", w.ID.String()[:8], "distance", "5.2", "km"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout metric with unit failed: %v", err)
	}

	// Verify metric has unit
	metrics, err := testDB.ListWorkoutMetrics(w.ID)
	if err != nil {
		t.Fatalf("ListWorkoutMetrics failed: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Unit == nil || *metrics[0].Unit != "km" {
		t.Error("Unit not set correctly")
	}
}

func TestWorkoutMetricCmdWithoutUnit(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create a workout first
	w := models.NewWorkout("lift")
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "metric", w.ID.String()[:8], "sets", "4"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout metric without unit failed: %v", err)
	}
}

func TestWorkoutListWithDuration(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	workoutType = ""
	workoutLimit = 20

	// Create workout with duration
	w := models.NewWorkout("run")
	w.WithDuration(45)
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "list"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout list with duration failed: %v", err)
	}
}

func TestWorkoutListWithTypeFilter(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	workoutType = ""
	workoutLimit = 20

	// Create workouts of different types
	w1 := models.NewWorkout("run")
	w2 := models.NewWorkout("lift")
	testDB.CreateWorkout(w1)
	testDB.CreateWorkout(w2)

	rootCmd.SetArgs([]string{"workout", "list", "--type", "run"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout list with type filter failed: %v", err)
	}
}

func TestInstallSkillFunction(t *testing.T) {
	// Test with a temporary home directory to avoid modifying real files
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Enable skip confirmation
	skillSkipConfirm = true
	defer func() { skillSkipConfirm = false }()

	err := installSkill()
	if err != nil {
		t.Errorf("installSkill failed: %v", err)
	}

	// Verify skill file was created
	skillPath := filepath.Join(tmpDir, ".claude", "skills", "health", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Error("Expected skill file to be created")
	}
}

func TestInstallSkillOverwrite(t *testing.T) {
	// Test that existing skill file is overwritten
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create an existing skill file
	skillDir := filepath.Join(tmpDir, ".claude", "skills", "health")
	os.MkdirAll(skillDir, 0755)
	skillPath := filepath.Join(skillDir, "SKILL.md")
	os.WriteFile(skillPath, []byte("old content"), 0644)

	// Enable skip confirmation
	skillSkipConfirm = true
	defer func() { skillSkipConfirm = false }()

	err := installSkill()
	if err != nil {
		t.Errorf("installSkill overwrite failed: %v", err)
	}

	// Verify content was updated
	content, _ := os.ReadFile(skillPath)
	if string(content) == "old content" {
		t.Error("Expected skill file to be overwritten")
	}
}

func TestRootCmdLongDescription(t *testing.T) {
	if rootCmd.Long == "" {
		t.Error("Expected rootCmd.Long to be non-empty")
	}
}

func TestAddCmdLongDescription(t *testing.T) {
	if addCmd.Long == "" {
		t.Error("Expected addCmd.Long to be non-empty")
	}
}

func TestListCmdLongDescription(t *testing.T) {
	if listCmd.Long == "" {
		t.Error("Expected listCmd.Long to be non-empty")
	}
}

func TestDeleteCmdLongDescription(t *testing.T) {
	if deleteCmd.Long == "" {
		t.Error("Expected deleteCmd.Long to be non-empty")
	}
}

func TestWorkoutCmdLongDescription(t *testing.T) {
	if workoutCmd.Long == "" {
		t.Error("Expected workoutCmd.Long to be non-empty")
	}
}

func TestExportCmdLongDescription(t *testing.T) {
	if exportCmd.Long == "" {
		t.Error("Expected exportCmd.Long to be non-empty")
	}
}

func TestMcpCmdLongDescription(t *testing.T) {
	if mcpCmd.Long == "" {
		t.Error("Expected mcpCmd.Long to be non-empty")
	}
}

func TestMigrateCmdLongDescription(t *testing.T) {
	if migrateCmd.Long == "" {
		t.Error("Expected migrateCmd.Long to be non-empty")
	}
}

func TestInstallSkillCmdLongDescription(t *testing.T) {
	if installSkillCmd.Long == "" {
		t.Error("Expected installSkillCmd.Long to be non-empty")
	}
}

func TestAllMetricTypesInHelp(t *testing.T) {
	// Verify help text contains metric types
	helpText := addCmd.Long
	expectedTypes := []string{"weight", "body_fat", "bp", "hrv", "mood", "steps"}

	for _, mt := range expectedTypes {
		if !bytes.Contains([]byte(helpText), []byte(mt)) {
			t.Errorf("Help text should contain metric type %q", mt)
		}
	}
}

func TestWorkoutAddCmdLongDescription(t *testing.T) {
	if workoutAddCmd.Long == "" {
		t.Error("Expected workoutAddCmd.Long to be non-empty")
	}
}

func TestWorkoutMetricCmdLongDescription(t *testing.T) {
	if workoutMetricCmd.Long == "" {
		t.Error("Expected workoutMetricCmd.Long to be non-empty")
	}
}

func TestWorkoutDeleteCmdLongDescription(t *testing.T) {
	if workoutDeleteCmd.Long == "" {
		t.Error("Expected workoutDeleteCmd.Long to be non-empty")
	}
}

func TestImportCmdLongDescription(t *testing.T) {
	if importCmd.Long == "" {
		t.Error("Expected importCmd.Long to be non-empty")
	}
}

func TestExportWithAllTypes(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	exportType = ""
	exportSince = ""
	exportOutput = ""

	// Create metrics of different types
	m1 := models.NewMetric(models.MetricWeight, 82.5)
	m2 := models.NewMetric(models.MetricMood, 7)
	m3 := models.NewMetric(models.MetricSteps, 10000)
	testDB.CreateMetric(m1)
	testDB.CreateMetric(m2)
	testDB.CreateMetric(m3)

	// Create workout with metrics
	w := models.NewWorkout("run")
	w.WithDuration(30)
	w.WithNotes("Test run")
	testDB.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	testDB.AddWorkoutMetric(wm)

	rootCmd.SetArgs([]string{"export", "markdown"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("export markdown with all types failed: %v", err)
	}
}

func TestListWithAllNotesDisplayed(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	listType = ""
	listLimit = 20

	// Create metrics with long notes that get truncated
	m := models.NewMetric(models.MetricWeight, 82.5)
	m.WithNotes("This is a very long note that should be truncated in the display output because it exceeds the maximum length")
	testDB.CreateMetric(m)

	rootCmd.SetArgs([]string{"list"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("list with long notes failed: %v", err)
	}
}

func TestWorkoutListNoDuration(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	workoutType = ""
	workoutLimit = 20

	// Create workout without duration
	w := models.NewWorkout("run")
	testDB.CreateWorkout(w)

	rootCmd.SetArgs([]string{"workout", "list"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout list without duration failed: %v", err)
	}
}

func TestListWithEmptyNotes(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Reset global flags
	listType = ""
	listLimit = 20

	// Create metric with empty notes
	m := models.NewMetric(models.MetricWeight, 82.5)
	m.WithNotes("")
	testDB.CreateMetric(m)

	rootCmd.SetArgs([]string{"list"})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("list with empty notes failed: %v", err)
	}
}

func TestWorkoutShowWithMetricsUnit(t *testing.T) {
	testDB, cleanup := setupTestCLI(t)
	defer cleanup()

	// Create workout with metric that has unit
	w := models.NewWorkout("run")
	testDB.CreateWorkout(w)

	wm := models.NewWorkoutMetric(w.ID, "distance", 5.0, "km")
	testDB.AddWorkoutMetric(wm)

	rootCmd.SetArgs([]string{"workout", "show", w.ID.String()[:8]})
	err := rootCmd.Execute()

	if err != nil {
		t.Errorf("workout show with metric unit failed: %v", err)
	}
}

func TestAllAddMetricTypes(t *testing.T) {
	// Test a few different metric types
	metricTypes := []string{"hrv", "temperature", "sleep_hours", "water", "protein", "energy", "stress", "anxiety", "focus", "meditation"}

	for _, mt := range metricTypes {
		t.Run(mt, func(t *testing.T) {
			_, cleanup := setupTestCLI(t)
			defer cleanup()

			// Reset global flags
			addAt = ""
			addNotes = ""

			rootCmd.SetArgs([]string{"add", mt, "10"})
			err := rootCmd.Execute()

			if err != nil {
				t.Errorf("add %s failed: %v", mt, err)
			}
		})
	}
}
