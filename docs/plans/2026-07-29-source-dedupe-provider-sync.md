# Source Field, Idempotent Upsert, New Metric Types, Provider Sync — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-metric `source` field (whoop/withings/emfit/manual/free-form), idempotent upsert keyed on (source, metric_type, recorded_at), four new metric types (recovery, strain, respiratory_rate, spo2), and native provider sync (`health sync whoop|withings|emfit`).

**Architecture:** `models.Metric` gains a `Source string` field normalized through a single `models.NormalizeSource` choke point. Both storage backends (SQLite `DB`, `MarkdownStore`) persist it; existing rows/files read back as `manual`. The `Repository` interface gains a `source` filter on `ListMetrics` and a new `UpsertMetric` method. CLI and MCP surfaces expose both. Provider sync lives in `internal/provsync` with a file-locked token store so single-use refresh tokens never race.

**Tech Stack:** Go 1.24, cobra, modernc.org/sqlite (pure Go), mdstore, modelcontextprotocol/go-sdk, golang.org/x/oauth2 (promote from indirect), golang.org/x/sys (flock).

## Global Constraints

- Offline-first, SQLite/markdown local storage, **no telemetry**. Sync commands are the only network callers.
- Default source is exactly the string `manual`. Sources are free-form but normalized: trimmed, lowercased, empty → `manual`.
- Dedup key is `(source, metric_type, recorded_at)` compared as an **instant** (timezone-offset differences of the same moment must match).
- No global UNIQUE constraint on the dedup key — legacy data may contain duplicates; upsert is opt-in (`--dedupe` / `dedupe:true`).
- Existing rows migrate to source `manual` (SQLite: ALTER with DEFAULT; markdown: missing frontmatter key reads as manual).
- Every task: `go build ./...`, `go vet ./...`, `make test-race` green before commit. Lint: `golangci-lint run` if installed locally (CI runs v2.7.2).
- Conventional commits, imperative present tense.
- Match existing style: ABOUTME headers on new files, table-driven-ish plain tests with `t.Fatalf`/`t.Errorf` (no testify), errors wrapped with `fmt.Errorf("...: %w", err)`.
- NEVER invent external API details. Whoop/Withings/Emfit field mappings below come from the operator and are authoritative. Items marked **VERIFY** must be confirmed against official docs (WebFetch/WebSearch) before coding; if verification fails, stop and surface it.
- `models.Metric` has no JSON tags (MCP serializes Go field names) — do NOT add tags to it; the fleet reads the current shape.

---

### Task 1: Source field on the model

**Files:**
- Modify: `internal/models/metric.go`
- Test: `internal/models/metric_test.go`

**Interfaces:**
- Produces: `Metric.Source string`; constants `SourceManual/SourceWhoop/SourceWithings/SourceEmfit`; `NormalizeSource(s string) string`; `(m *Metric) WithSource(s string) *Metric`; `ValidMetricTypesList() string`. `NewMetric` now sets `Source: SourceManual`.

- [ ] **Step 1: Write failing tests** (append to `internal/models/metric_test.go`)

```go
func TestNewMetricDefaultsToManualSource(t *testing.T) {
	m := NewMetric(MetricWeight, 82.5)
	if m.Source != SourceManual {
		t.Errorf("Source = %q, want %q", m.Source, SourceManual)
	}
}

func TestWithSource(t *testing.T) {
	m := NewMetric(MetricHRV, 48).WithSource("whoop")
	if m.Source != "whoop" {
		t.Errorf("Source = %q, want whoop", m.Source)
	}
}

func TestNormalizeSource(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "manual"},
		{"  ", "manual"},
		{"Whoop", "whoop"},
		{" EMFIT ", "emfit"},
		{"withings", "withings"},
		{"my-custom-device", "my-custom-device"},
	}
	for _, c := range cases {
		if got := NormalizeSource(c.in); got != c.want {
			t.Errorf("NormalizeSource(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidMetricTypesList(t *testing.T) {
	list := ValidMetricTypesList()
	if !strings.Contains(list, "weight") || !strings.Contains(list, "meditation") {
		t.Errorf("ValidMetricTypesList missing types: %s", list)
	}
}
```
(add `"strings"` to test imports)

- [ ] **Step 2: Run to verify failure** — `go test ./internal/models/ -run 'Source|ValidMetricTypesList' -v` → FAIL (undefined symbols).

- [ ] **Step 3: Implement** in `internal/models/metric.go`:
  - Add `"strings"` import.
  - After the MetricType consts:

```go
// Known metric sources. Source is free-form; these are the built-in ones.
const (
	SourceManual   = "manual"
	SourceWhoop    = "whoop"
	SourceWithings = "withings"
	SourceEmfit    = "emfit"
)

// NormalizeSource canonicalizes a source string: trimmed, lowercased,
// empty defaults to manual.
func NormalizeSource(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return SourceManual
	}
	return s
}

// ValidMetricTypesList returns all metric type names joined for help/error text.
func ValidMetricTypesList() string {
	names := make([]string, len(AllMetricTypes))
	for i, mt := range AllMetricTypes {
		names[i] = string(mt)
	}
	return strings.Join(names, ", ")
}
```

  - Add `Source string` to `Metric` struct between `Notes` and `CreatedAt`.
  - In `NewMetric`, set `Source: SourceManual`.
  - Add builder:

```go
// WithSource sets the data source (whoop, withings, emfit, manual, or custom).
func (m *Metric) WithSource(source string) *Metric {
	m.Source = NormalizeSource(source)
	return m
}
```

- [ ] **Step 4: Verify** — `go test ./internal/models/ -v` → PASS; `go build ./...` → note other packages still compile (no signature changed yet).
- [ ] **Step 5: Commit** — `git add internal/models && git commit -m "feat(models): add source field with manual default"`

---

### Task 2: Persist source in both backends (with legacy migration)

**Files:**
- Modify: `internal/storage/schema.go`, `internal/storage/metrics.go`, `internal/storage/markdown.go`
- Test: `internal/storage/repository_test.go`, `internal/storage/markdown_test.go`

**Interfaces:**
- Consumes: `models.NormalizeSource`, `Metric.Source`.
- Produces: source column persisted/scanned in SQLite (`source TEXT NOT NULL DEFAULT 'manual'`); `source:` YAML key in markdown frontmatter; legacy DBs auto-migrated via `ensureMetricSourceColumn`. No Repository signature changes yet.

- [ ] **Step 1: Failing tests.** In `repository_test.go`:

```go
func TestMetricSourceRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := models.NewMetric(models.MetricHRV, 48).WithSource("whoop")
	if err := db.CreateMetric(m); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}
	got, err := db.GetMetric(m.ID.String())
	if err != nil {
		t.Fatalf("GetMetric failed: %v", err)
	}
	if got.Source != "whoop" {
		t.Errorf("Source = %q, want whoop", got.Source)
	}

	// Default path: no WithSource call
	m2 := models.NewMetric(models.MetricWeight, 82.5)
	if err := db.CreateMetric(m2); err != nil {
		t.Fatalf("CreateMetric failed: %v", err)
	}
	got2, err := db.GetMetric(m2.ID.String())
	if err != nil {
		t.Fatalf("GetMetric failed: %v", err)
	}
	if got2.Source != models.SourceManual {
		t.Errorf("default Source = %q, want manual", got2.Source)
	}
}

func TestLegacyDBMigratesSourceToManual(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")

	// Build a pre-source-column database by hand.
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	_, err = raw.Exec(`CREATE TABLE metrics (
		id TEXT PRIMARY KEY, metric_type TEXT NOT NULL, value REAL NOT NULL,
		unit TEXT NOT NULL, recorded_at DATETIME NOT NULL, notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	legacyID := uuid.New().String()
	_, err = raw.Exec(`INSERT INTO metrics (id, metric_type, value, unit, recorded_at, created_at) VALUES (?, 'weight', 80.0, 'kg', ?, ?)`,
		legacyID, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open migrated db: %v", err)
	}
	defer db.Close()

	got, err := db.GetMetric(legacyID)
	if err != nil {
		t.Fatalf("GetMetric legacy row: %v", err)
	}
	if got.Source != models.SourceManual {
		t.Errorf("legacy Source = %q, want manual", got.Source)
	}

	// New writes into the migrated DB carry their source.
	m := models.NewMetric(models.MetricHRV, 50).WithSource("emfit")
	if err := db.CreateMetric(m); err != nil {
		t.Fatalf("CreateMetric on migrated db: %v", err)
	}
	got2, err := db.GetMetric(m.ID.String())
	if err != nil {
		t.Fatalf("GetMetric: %v", err)
	}
	if got2.Source != "emfit" {
		t.Errorf("Source = %q, want emfit", got2.Source)
	}
}
```
(test imports gain `"database/sql"`; keep the existing `_ "modernc.org/sqlite"` availability — add the blank import to the test file if not present in the package's non-test files' import graph... it IS present via db.go, but the test file itself needs `database/sql` only.)

In `markdown_test.go` (mirror its existing setup helper, likely `NewMarkdownStore(t.TempDir())`):

```go
func TestMarkdownMetricSourceRoundTrip(t *testing.T) {
	s, err := NewMarkdownStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewMarkdownStore: %v", err)
	}
	m := models.NewMetric(models.MetricHRV, 48).WithSource("whoop")
	if err := s.CreateMetric(m); err != nil {
		t.Fatalf("CreateMetric: %v", err)
	}
	got, err := s.GetMetric(m.ID.String())
	if err != nil {
		t.Fatalf("GetMetric: %v", err)
	}
	if got.Source != "whoop" {
		t.Errorf("Source = %q, want whoop", got.Source)
	}
}

func TestMarkdownLegacyFileReadsManualSource(t *testing.T) {
	dir := t.TempDir()
	s, err := NewMarkdownStore(dir)
	if err != nil {
		t.Fatalf("NewMarkdownStore: %v", err)
	}
	// Hand-write a legacy metric file with no source key.
	id := uuid.New()
	path := filepath.Join(dir, "metrics", "2025", "01", "2025-01-15-weight-"+id.String()[:8]+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\nid: " + id.String() + "\nmetric_type: weight\nvalue: 80\nunit: kg\nrecorded_at: 2025-01-15T07:00:00Z\ncreated_at: 2025-01-15T07:00:00Z\n---\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}
	got, err := s.GetMetric(id.String())
	if err != nil {
		t.Fatalf("GetMetric: %v", err)
	}
	if got.Source != models.SourceManual {
		t.Errorf("legacy Source = %q, want manual", got.Source)
	}
}
```

- [ ] **Step 2: Verify failure** — `go test ./internal/storage/ -run 'Source' -v` → FAIL.
- [ ] **Step 3: Implement.**

`schema.go` — restructure `initSchema` so the source index is created AFTER the legacy-column migration:

```go
// initSchema creates or updates the database schema.
func (d *DB) initSchema() error {
	schema := ` ... existing schema text, with the metrics table gaining
		source TEXT NOT NULL DEFAULT 'manual',
	  between notes and created_at, and the three existing metrics indexes kept ...`
	if _, err := d.db.Exec(schema); err != nil {
		return err
	}
	if err := d.ensureMetricSourceColumn(); err != nil {
		return err
	}
	// Source indexes must come after the column migration so pre-source
	// databases have the column before indexing it.
	sourceIndexes := `
	CREATE INDEX IF NOT EXISTS idx_metrics_source ON metrics(source);
	CREATE INDEX IF NOT EXISTS idx_metrics_dedupe ON metrics(source, metric_type, recorded_at);
	`
	_, err := d.db.Exec(sourceIndexes)
	return err
}

// ensureMetricSourceColumn adds the source column to databases created
// before the column existed. Existing rows read back as 'manual'.
func (d *DB) ensureMetricSourceColumn() error {
	rows, err := d.db.Query(`PRAGMA table_info(metrics)`)
	if err != nil {
		return fmt.Errorf("inspect metrics schema: %w", err)
	}
	defer rows.Close()

	hasSource := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan metrics schema: %w", err)
		}
		if name == "source" {
			hasSource = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if hasSource {
		return nil
	}
	if _, err := d.db.Exec(`ALTER TABLE metrics ADD COLUMN source TEXT NOT NULL DEFAULT 'manual'`); err != nil {
		return fmt.Errorf("add source column: %w", err)
	}
	return nil
}
```
(schema.go gains imports `"database/sql"`, `"fmt"`.)

`metrics.go` — thread source through every query. SELECT column order everywhere: `id, metric_type, value, unit, recorded_at, notes, source, created_at`.
- `CreateMetric`: normalize first — `m.Source = models.NormalizeSource(m.Source)` — then INSERT with the extra column/value.
- `scanMetric`/`scanMetrics`: scan `source` as `sql.NullString`; set `m.Source = models.NormalizeSource(source.String)` (handles NULL and empty defensively).
- `GetMetric`, `ListMetrics`, `GetLatestMetric` query strings updated to the new column list.

`markdown.go`:
- `metricFrontmatter` gains `Source string \`yaml:"source,omitempty"\`` after Unit.
- `metricToFrontmatter`: `Source: models.NormalizeSource(m.Source)` (always written).
- `metricFromFrontmatter`: `Source: models.NormalizeSource(fm.Source)`.
- `CreateMetric` (MarkdownStore): normalize `m.Source = models.NormalizeSource(m.Source)` before write.

- [ ] **Step 4: Verify** — `go test ./internal/storage/ -v` then `go build ./... && make test-race` → all PASS.
- [ ] **Step 5: Commit** — `git add internal/storage && git commit -m "feat(storage): persist metric source in sqlite and markdown backends"`

---

### Task 3: Source filter on ListMetrics (interface change, all call sites)

**Files:**
- Modify: `internal/storage/repository.go`, `internal/storage/metrics.go`, `internal/storage/markdown.go`, `internal/storage/export.go`, `internal/storage/migrate.go` (call site in `MigrateData`), `internal/mcp/tools.go`, `internal/mcp/resources.go`, `cmd/health/list.go` (compile fix only — flag lands in Task 5), any test call sites (`repository_test.go`, `markdown_test.go`, `export_test.go`, `migrate_test.go`, `internal/mcp/server_test.go`, `cmd/health/cmd_test.go`, `test/integration_test.go`).

**Interfaces:**
- Produces: `ListMetrics(metricType *models.MetricType, source *string, limit int) ([]*models.Metric, error)` on `Repository` and both impls. All existing callers pass `nil` for source.

- [ ] **Step 1: Failing tests.** In `repository_test.go`:

```go
func TestListMetricsFilterBySource(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	for _, src := range []string{"whoop", "emfit", "whoop"} {
		m := models.NewMetric(models.MetricSleepHours, 7.5).WithSource(src)
		if err := db.CreateMetric(m); err != nil {
			t.Fatalf("CreateMetric: %v", err)
		}
	}

	src := "whoop"
	got, err := db.ListMetrics(nil, &src, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("whoop count = %d, want 2", len(got))
	}

	// Combined type+source filter
	mt := models.MetricSleepHours
	got, err = db.ListMetrics(&mt, &src, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("combined filter count = %d, want 2", len(got))
	}

	// Filter normalizes case
	src2 := "Emfit"
	got, err = db.ListMetrics(nil, &src2, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("emfit count = %d, want 1", len(got))
	}
}
```

Mirror the same test against `MarkdownStore` in `markdown_test.go` (`TestMarkdownListMetricsFilterBySource`, same body with `s, _ := NewMarkdownStore(t.TempDir())`).

- [ ] **Step 2: Verify failure** — compile error (wrong arg count) counts as failure.
- [ ] **Step 3: Implement.**

`repository.go`: change the interface line to
```go
ListMetrics(metricType *models.MetricType, source *string, limit int) ([]*models.Metric, error)
```

`metrics.go` — rebuild `ListMetrics` with condition assembly instead of duplicated query strings:

```go
// ListMetrics retrieves metrics with optional filtering by type and source.
// Results are sorted by RecordedAt descending (most recent first).
func (d *DB) ListMetrics(metricType *models.MetricType, source *string, limit int) ([]*models.Metric, error) {
	query := `
		SELECT id, metric_type, value, unit, recorded_at, notes, source, created_at
		FROM metrics
	`
	var conds []string
	var args []interface{}
	if metricType != nil {
		conds = append(conds, "metric_type = ?")
		args = append(args, string(*metricType))
	}
	if source != nil {
		conds = append(conds, "source = ?")
		args = append(args, models.NormalizeSource(*source))
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY recorded_at DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}
	defer rows.Close()

	return d.scanMetrics(rows)
}
```

`markdown.go` `ListMetrics`: add `source *string` param; inside the walk callback, after the type check:
```go
if source != nil && models.NormalizeSource(m.Source) != models.NormalizeSource(*source) {
	return nil
}
```
Also update the internal callers: `GetLatestMetric` → `s.ListMetrics(&mt, nil, 1)`, `GetAllData` → `s.ListMetrics(nil, nil, 0)`.

Update every other call site to pass `nil`: `export.go` (`GetAllDataFromRepo`, `ExportMarkdownFromRepo`), `migrate.go` (`MigrateData`), `mcp/tools.go` (`handleListMetrics`, `handleGetLatest`), `mcp/resources.go` (recent/today/summary), `cmd/health/list.go`. Then sweep tests: `grep -rn "ListMetrics(" --include="*.go"` and fix each remaining call.

- [ ] **Step 4: Verify** — `go build ./... && make test-race` → PASS.
- [ ] **Step 5: Commit** — `git add -u && git add internal cmd && git commit -m "feat(storage): filter metrics by source in ListMetrics"`

---

### Task 4: UpsertMetric on both backends

**Files:**
- Modify: `internal/storage/repository.go`, `internal/storage/metrics.go`, `internal/storage/markdown.go`
- Test: `internal/storage/repository_test.go`, `internal/storage/markdown_test.go`

**Interfaces:**
- Produces: `UpsertMetric(m *models.Metric) (updated bool, err error)` on `Repository` and both impls. Semantics: match on (normalized source, metric_type, recorded_at as instant). On match: update value/unit/notes of the matched row, keep its id/created_at, reflect them back into `m`, return true. No match: behaves as CreateMetric, returns false. If legacy duplicates share the key, deterministically update the oldest (created_at ASC, id ASC) and leave the rest.

- [ ] **Step 1: Failing tests** in `repository_test.go`:

```go
func TestUpsertMetricInsertsWhenNew(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := models.NewMetric(models.MetricHRV, 48).WithSource("whoop")
	updated, err := db.UpsertMetric(m)
	if err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}
	if updated {
		t.Errorf("updated = true, want false for new row")
	}
	all, err := db.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("count = %d, want 1", len(all))
	}
}

func TestUpsertMetricReplacesSameKey(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	at := time.Date(2026, 7, 28, 7, 0, 0, 0, time.UTC)
	m1 := models.NewMetric(models.MetricHRV, 48).WithSource("whoop").WithRecordedAt(at)
	if _, err := db.UpsertMetric(m1); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}

	m2 := models.NewMetric(models.MetricHRV, 52).WithSource("whoop").WithRecordedAt(at)
	m2.WithNotes("resynced")
	updated, err := db.UpsertMetric(m2)
	if err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}
	if !updated {
		t.Errorf("updated = false, want true")
	}
	if m2.ID != m1.ID {
		t.Errorf("upsert should keep original ID: got %s, want %s", m2.ID, m1.ID)
	}

	all, err := db.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("count = %d, want 1 (no duplicate)", len(all))
	}
	if all[0].Value != 52 {
		t.Errorf("Value = %v, want 52", all[0].Value)
	}
	if all[0].Notes == nil || *all[0].Notes != "resynced" {
		t.Errorf("Notes not updated: %v", all[0].Notes)
	}
}

func TestUpsertMetricDistinguishesSources(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	at := time.Date(2026, 7, 28, 7, 0, 0, 0, time.UTC)
	m1 := models.NewMetric(models.MetricSleepHours, 7.2).WithSource("whoop").WithRecordedAt(at)
	m2 := models.NewMetric(models.MetricSleepHours, 7.8).WithSource("emfit").WithRecordedAt(at)
	if _, err := db.UpsertMetric(m1); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}
	updated, err := db.UpsertMetric(m2)
	if err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}
	if updated {
		t.Errorf("different source must not update")
	}
	all, err := db.ListMetrics(nil, nil, 0)
	if err != nil {
		t.Fatalf("ListMetrics: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("count = %d, want 2", len(all))
	}
}

func TestUpsertMetricMatchesInstantAcrossZones(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	utc := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	chicago := utc.In(time.FixedZone("CDT", -5*3600))

	m1 := models.NewMetric(models.MetricHeartRate, 55).WithSource("whoop").WithRecordedAt(utc)
	if _, err := db.UpsertMetric(m1); err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}
	m2 := models.NewMetric(models.MetricHeartRate, 56).WithSource("whoop").WithRecordedAt(chicago)
	updated, err := db.UpsertMetric(m2)
	if err != nil {
		t.Fatalf("UpsertMetric: %v", err)
	}
	if !updated {
		t.Errorf("same instant in different zone must match")
	}
}
```

Mirror all four in `markdown_test.go` against `NewMarkdownStore(t.TempDir())` (names prefixed `TestMarkdownUpsert...`).

- [ ] **Step 2: Verify failure** — `go test ./internal/storage/ -run Upsert -v` → FAIL (method undefined).
- [ ] **Step 3: Implement.**

`repository.go`: add under CreateMetric:
```go
// UpsertMetric inserts the metric, or if a metric with the same
// (source, metric_type, recorded_at) already exists, updates that row's
// value, unit, and notes in place. Returns true when an existing row was
// updated. The matched row's ID and CreatedAt are reflected back into m.
UpsertMetric(m *models.Metric) (bool, error)
```

`metrics.go`:
```go
// UpsertMetric inserts or replaces a metric keyed on (source, metric_type, recorded_at).
// recorded_at is compared as an instant via SQLite datetime(), so the same
// moment expressed in different timezone offsets still matches. If legacy
// duplicates share the key, the oldest row is updated deterministically.
func (d *DB) UpsertMetric(m *models.Metric) (bool, error) {
	m.Source = models.NormalizeSource(m.Source)

	var existingID, existingCreatedAt string
	err := d.db.QueryRow(`
		SELECT id, created_at FROM metrics
		WHERE source = ? AND metric_type = ? AND datetime(recorded_at) = datetime(?)
		ORDER BY created_at ASC, id ASC
		LIMIT 1
	`, m.Source, string(m.MetricType), m.RecordedAt.Format(time.RFC3339)).Scan(&existingID, &existingCreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, d.CreateMetric(m)
	}
	if err != nil {
		return false, fmt.Errorf("upsert metric lookup: %w", err)
	}

	if _, err := d.db.Exec(`UPDATE metrics SET value = ?, unit = ?, notes = ? WHERE id = ?`,
		m.Value, m.Unit, m.Notes, existingID); err != nil {
		return false, fmt.Errorf("upsert metric update: %w", err)
	}

	m.ID, _ = uuid.Parse(existingID)
	if t, perr := time.Parse(time.RFC3339, existingCreatedAt); perr == nil {
		m.CreatedAt = t
	}
	return true, nil
}
```

`markdown.go`: extract a path-explicit writer from `writeMetricFile`:
```go
// writeMetricFileAt renders and writes a metric to an explicit path.
func (s *MarkdownStore) writeMetricFileAt(path string, m *models.Metric) error {
	fm := metricToFrontmatter(m)
	body := ""
	if m.Notes != nil && *m.Notes != "" {
		body = "\n" + *m.Notes + "\n"
	}
	content, err := mdstore.RenderFrontmatter(&fm, body)
	if err != nil {
		return fmt.Errorf("render metric file: %w", err)
	}
	return mdstore.AtomicWrite(path, []byte(content))
}
```
`writeMetricFile` becomes `return s.writeMetricFileAt(s.metricFilePath(m.RecordedAt, m.MetricType, m.ID), m)`.

```go
// UpsertMetric inserts or replaces a metric keyed on (source, metric_type, recorded_at).
// The existing file is rewritten in place so its path and identity are stable.
func (s *MarkdownStore) UpsertMetric(m *models.Metric) (bool, error) {
	m.Source = models.NormalizeSource(m.Source)

	type match struct {
		path   string
		metric *models.Metric
	}
	var matches []match
	err := s.walkMetricFiles(func(path string, existing *models.Metric) error {
		if existing.MetricType == m.MetricType &&
			models.NormalizeSource(existing.Source) == m.Source &&
			existing.RecordedAt.Equal(m.RecordedAt) {
			matches = append(matches, match{path: path, metric: existing})
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("upsert metric: %w", err)
	}
	if len(matches) == 0 {
		return false, s.CreateMetric(m)
	}

	// Deterministic pick when legacy duplicates share the key.
	sort.Slice(matches, func(i, j int) bool {
		a, b := matches[i].metric, matches[j].metric
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID.String() < b.ID.String()
	})
	target := matches[0]
	target.metric.Value = m.Value
	target.metric.Unit = m.Unit
	target.metric.Notes = m.Notes

	if err := s.writeMetricFileAt(target.path, target.metric); err != nil {
		return false, err
	}
	m.ID = target.metric.ID
	m.CreatedAt = target.metric.CreatedAt
	return true, nil
}
```
(`metrics.go` needs `"errors"` import if missing — it already has it.)

- [ ] **Step 4: Verify** — `go build ./... && make test-race` → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(storage): add UpsertMetric for idempotent writes"`

---

### Task 5: CLI — `add --source/--dedupe`, `list --source` + source column

**Files:**
- Modify: `cmd/health/add.go`, `cmd/health/list.go`
- Test: `cmd/health/cmd_test.go` (follow its existing harness pattern — read it first; it likely builds commands against a temp repo)

**Interfaces:**
- Consumes: `repo.UpsertMetric`, `models.NormalizeSource`, `models.ValidMetricTypesList`, `ListMetrics(mt, source, limit)`.
- Produces: flags `add --source <s>`, `add --dedupe`, `list --source <s>`; list line format `ID TIME TYPE SOURCE VALUE UNIT (NOTES)`.

- [ ] **Step 1: Failing tests** (adapt to cmd_test.go's existing pattern for invoking commands; if it drives `rootCmd` with a test repo, follow that; the assertions below are the contract):
  - `add hrv 48 --source whoop` → stored metric has Source "whoop"; output contains `[whoop]`.
  - `add hrv 48 --source whoop --at "2026-07-28 07:00" --dedupe` run twice → one stored row; second run's output starts `✓ Updated`.
  - `add bp 120 80 --source withings` → both bp_sys and bp_dia have Source "withings".
  - `add nosuchtype 1` → error message contains `spo2`? No — new types land in Task 7; assert it contains `weight, body_fat` (derived list).
  - `list --source whoop` after adding whoop + manual rows → only whoop rows in output; line contains `whoop` column.
- [ ] **Step 2: Verify failure.**
- [ ] **Step 3: Implement.**

`add.go`:
- New vars `addSource string`, `addDedupe bool`; register:
```go
addCmd.Flags().StringVar(&addSource, "source", "", "data source: whoop, withings, emfit, manual, or free-form (default manual)")
addCmd.Flags().BoolVar(&addDedupe, "dedupe", false, "replace an existing entry with the same source, type, and timestamp instead of adding a duplicate")
```
- Replace the hardcoded valid-types error with:
```go
return fmt.Errorf("unknown metric type: %s\nValid types: %s", metricType, models.ValidMetricTypesList())
```
- After the `--notes` handling:
```go
if addSource != "" {
	m.WithSource(addSource)
}

verb := "Added"
if addDedupe {
	updated, err := repo.UpsertMetric(m)
	if err != nil {
		return fmt.Errorf("failed to upsert metric: %w", err)
	}
	if updated {
		verb = "Updated"
	}
} else if err := repo.CreateMetric(m); err != nil {
	return fmt.Errorf("failed to create metric: %w", err)
}

color.Green("✓ %s %s", verb, metricType)
fmt.Printf("  %s %.2f %s [%s]\n",
	color.New(color.Faint).Sprint(m.ID.String()[:8]),
	m.Value, m.Unit, m.Source)
```
- `addBloodPressure`: apply `WithSource(addSource)` to both metrics when set; when `addDedupe`, use `repo.UpsertMetric` for both (ignore the updated flags individually; print `✓ Updated blood pressure` if either updated, else `✓ Added blood pressure`).
- Help text: add flags to EXAMPLES (`health add hrv 48 --source whoop --dedupe   # Idempotent sync write`) and a SOURCES section naming whoop/withings/emfit/manual/free-form + default.

`list.go`:
- New var `listSource string`; flag:
```go
listCmd.Flags().StringVarP(&listSource, "source", "s", "", "filter by data source (whoop, withings, emfit, manual, ...)")
```
- Build filter:
```go
var sourceFilter *string
if listSource != "" {
	s := models.NormalizeSource(listSource)
	sourceFilter = &s
}
metrics, err := repo.ListMetrics(metricType, sourceFilter, listLimit)
```
- Output line gains a source column after TYPE:
```go
fmt.Printf("%s %s %s %s %.2f %s%s\n",
	faint.Sprint(m.ID.String()[:8]),
	faint.Sprint(m.RecordedAt.Format("2006-01-02 15:04")),
	padRight(string(m.MetricType), 16),
	faint.Sprint(padRight(m.Source, 9)),
	m.Value,
	m.Unit,
	notes)
```
- Update the Long help's OUTPUT FORMAT line to `ID  TIMESTAMP  TYPE  SOURCE  VALUE  UNIT  (NOTES)` and add a `--source` example.

- [ ] **Step 4: Verify** — `go build ./... && make test-race`; also run a real smoke against a temp dir: `XDG_DATA_HOME=$(mktemp -d) XDG_CONFIG_HOME=$(mktemp -d) ./health add hrv 48 --source whoop --dedupe` twice, then `health list`.
- [ ] **Step 5: Commit** — `git commit -am "feat(cli): add --source and --dedupe flags, show source in list"`

---

### Task 6: MCP surface — source + dedupe in tools and resources

**Files:**
- Modify: `internal/mcp/tools.go`, `internal/mcp/resources.go`
- Test: `internal/mcp/server_test.go` (follow existing patterns there)

**Interfaces:**
- Consumes: `UpsertMetric`, `ListMetrics(mt, source, limit)`.
- Produces: `add_metric` input gains `source`, `dedupe`; output gains `source`, `updated`. `list_metrics` input gains `source`. `get_latest` results gain `"source"`. `health://summary` latest-metric entries gain `"source"`.

- [ ] **Step 1: Failing tests** in `server_test.go` (use its existing setup helper):
  - add_metric with `Source: "whoop"` → stored metric Source whoop; output.Source == "whoop".
  - add_metric same key twice with `Dedupe: true` → repo count 1; second output.Updated == true, message starts "Updated".
  - list_metrics with `Source: "whoop"` → only whoop metrics.
  - get_latest → entry map contains `"source"`.
- [ ] **Step 2: Verify failure.**
- [ ] **Step 3: Implement.**

`tools.go`:
```go
type addMetricInput struct {
	MetricType string  `json:"metric_type"`
	Value      float64 `json:"value"`
	RecordedAt string  `json:"recorded_at,omitempty"`
	Notes      string  `json:"notes,omitempty"`
	Source     string  `json:"source,omitempty"`
	Dedupe     bool    `json:"dedupe,omitempty"`
}

type metricOutput struct {
	ID         string  `json:"id"`
	MetricType string  `json:"metric_type"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Source     string  `json:"source"`
	Updated    bool    `json:"updated"`
	Message    string  `json:"message"`
}

type listMetricsInput struct {
	MetricType string `json:"metric_type,omitempty"`
	Source     string `json:"source,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}
```
`handleAddMetric`: after notes handling:
```go
if input.Source != "" {
	m.WithSource(input.Source)
}

var updated bool
if input.Dedupe {
	var err error
	updated, err = s.repo.UpsertMetric(m)
	if err != nil {
		return nil, metricOutput{}, fmt.Errorf("failed to upsert metric: %w", err)
	}
} else if err := s.repo.CreateMetric(m); err != nil {
	return nil, metricOutput{}, fmt.Errorf("failed to create metric: %w", err)
}

verb := "Added"
if updated {
	verb = "Updated"
}
return nil, metricOutput{
	ID:         m.ID.String()[:8],
	MetricType: input.MetricType,
	Value:      m.Value,
	Unit:       m.Unit,
	Source:     m.Source,
	Updated:    updated,
	Message:    fmt.Sprintf("%s %s: %.2f %s [%s] (ID: %s)", verb, input.MetricType, m.Value, m.Unit, m.Source, m.ID.String()[:8]),
}, nil
```
`handleListMetrics`: build `var source *string` from `input.Source` (non-empty → normalized pointer), pass to `ListMetrics(metricType, source, input.Limit)`.
`handleGetLatest`: pass `nil` source; add `"source": metrics[0].Source` to each result map.
Tool descriptions: `add_metric` → "Record a health metric (weight, hrv, mood, etc.). Optional source (whoop|withings|emfit|manual|custom) and dedupe (replace same source+type+timestamp)."; `list_metrics` → "...optionally filtered by type and/or source".

`resources.go` `handleSummaryResource`: add `"source": m.Source` to the latestMetrics entry map. (recent/today serialize `models.Metric` directly — Source appears automatically.)

- [ ] **Step 4: Verify** — `go build ./... && make test-race` → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(mcp): expose source and dedupe in tools and resources"`

---

### Task 7: New metric types — recovery, strain, respiratory_rate, spo2

**Files:**
- Modify: `internal/models/metric.go`, `cmd/health/add.go` (help), `cmd/health/list.go` (help), `cmd/health/root.go` (help), `internal/mcp/resources.go` (category lists)
- Test: `internal/models/metric_test.go`, `internal/mcp/server_test.go`

**Interfaces:**
- Produces: `MetricRecovery` ("recovery", unit "%"), `MetricStrain` ("strain", unit "score"), `MetricRespiratoryRate` ("respiratory_rate", unit "brpm"), `MetricSpO2` ("spo2", unit "%") — all in `AllMetricTypes`/`MetricUnits`. Summary categories: respiratory_rate+spo2 → biometrics; recovery+strain → activity.

- [ ] **Step 1: Failing tests** in `metric_test.go`:

```go
func TestNewMetricTypesAreValid(t *testing.T) {
	cases := []struct {
		name string
		unit string
	}{
		{"recovery", "%"},
		{"strain", "score"},
		{"respiratory_rate", "brpm"},
		{"spo2", "%"},
	}
	for _, c := range cases {
		if !IsValidMetricType(c.name) {
			t.Errorf("IsValidMetricType(%q) = false, want true", c.name)
		}
		m := NewMetric(MetricType(c.name), 50)
		if m.Unit != c.unit {
			t.Errorf("unit for %s = %q, want %q", c.name, m.Unit, c.unit)
		}
	}
}
```
And in `server_test.go`: a summary-resource test asserting a stored `spo2` metric appears under `metrics.biometrics.spo2` and `strain` under `metrics.activity.strain`.
- [ ] **Step 2: Verify failure.**
- [ ] **Step 3: Implement.**

`metric.go`:
- Biometrics block gains `MetricRespiratoryRate MetricType = "respiratory_rate"` and `MetricSpO2 MetricType = "spo2"`.
- Activity block gains `MetricRecovery MetricType = "recovery"` and `MetricStrain MetricType = "strain"`.
- `MetricUnits` gains: `MetricRespiratoryRate: "brpm"`, `MetricSpO2: "%"`, `MetricRecovery: "%"`, `MetricStrain: "score"`.
- `AllMetricTypes` gains all four (biometrics additions after MetricTemperature; activity additions after MetricActiveCalories).
- Update the ABOUTME header count ("Defines 25 metric types...").

`resources.go`: `biometricTypes` += `models.MetricRespiratoryRate, models.MetricSpO2`; `activityTypes` += `models.MetricRecovery, models.MetricStrain`.

Help text:
- `add.go` Long: Biometrics section gains `respiratory_rate  Breathing rate in brpm` and `spo2  Blood oxygen saturation %`; Activity gains `recovery  Recovery score (0-100%)` and `strain  Strain score (0-21)`. Add example `health add recovery 85 --source whoop`.
- `list.go` Long FILTERING list gains the four names.
- `root.go` Long WHAT IT TRACKS lines updated to include them.

- [ ] **Step 4: Verify** — `go build ./... && make test-race` → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(models): add recovery, strain, respiratory_rate, spo2 metric types"`

---

### Task 8: Export formats, integration test, README

**Files:**
- Modify: `internal/storage/export.go` (yamlMetric + markdown export table), `README.md`, `test/integration_test.go`
- Test: `internal/storage/export_test.go`, `test/integration_test.go`

**Interfaces:**
- Consumes: everything above.
- Produces: YAML export metrics gain `source:`; markdown export tables gain a Source column; JSON export gains `Source` automatically (no tags on models.Metric — verify only). Old JSON exports (no Source) import cleanly as manual.

- [ ] **Step 1: Failing tests:**
  - `export_test.go`: create metric with source whoop → `ExportYAMLFromRepo` output contains `source: whoop`; `ExportJSONFromRepo` output contains `"Source": "whoop"`; `ExportMarkdownFromRepo` row contains `whoop`.
  - Import round-trip: JSON export → import into fresh repo → source preserved; hand-built legacy JSON (metric object without Source) → import → Source manual.
  - `test/integration_test.go`: end-to-end scenario (follow the file's existing harness): add with `--source whoop --dedupe` twice → list shows one row with whoop.
- [ ] **Step 2: Verify failure.**
- [ ] **Step 3: Implement.**
  - `yamlMetric` gains `Source string \`yaml:"source"\``; populate in `ExportYAMLFromRepo`.
  - `ExportMarkdownFromRepo` table header `| Date | Value | Source | Notes |` and rows include `m.Source` (both the per-type and grouped branches).
  - `ImportDataToRepo` already routes through `CreateMetric`, which normalizes "" → manual (Task 2) — add the legacy-import test to prove it.
  - README: document `--source`/`--dedupe`, `list --source`, the four new types, and the MCP field additions.
- [ ] **Step 4: Verify** — `go build ./... && make test-race` → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(export): include source in export formats"`

---

## Phase B: Native provider sync (`health sync`)

**Package layout:** `internal/provsync/` (name avoids clash with stdlib `sync`): `provider.go` (shared types), `tokens.go` (token store + flock), `whoop.go`, `withings.go`, `emfit.go`, `oauthflow.go` (localhost callback helper). CLI: `cmd/health/sync.go`.

**Config additions (`internal/config/config.go`):**
```go
type SyncConfig struct {
	Whoop    OAuthProviderConfig `json:"whoop,omitempty"`
	Withings OAuthProviderConfig `json:"withings,omitempty"`
	Emfit    EmfitConfig         `json:"emfit,omitempty"`
}
type OAuthProviderConfig struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}
type EmfitConfig struct {
	Token    string `json:"token,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
}
```
`Config` gains `Sync SyncConfig \`json:"sync,omitempty"\``. Config file and token files written 0600.

**Token store contract (`tokens.go`):**
- Files at `<dataDir>/tokens/<provider>.json`, 0600, atomic write (temp+rename).
- `Token{AccessToken, RefreshToken string; ExpiresAt time.Time; TokenType string}` with JSON tags.
- `WithLock(provider string, fn func() error) error` — flock (`golang.org/x/sys/unix.Flock`, LOCK_EX) on `<provider>.lock`; lock released on unlock+close (and by the OS if the process dies).
- Refresh discipline (the single most important behavior — Whoop/Withings refresh tokens rotate and are single-use):
  1. Load token; if `ExpiresAt` > now+60s, use it.
  2. Otherwise `WithLock`: **re-read** the token file (another process may have refreshed while we waited), re-check expiry, and only then refresh.
  3. Persist the rotated refresh token to disk **before** using the new access token.
- Tests: expired token + httptest token endpoint counting calls → two goroutines requesting simultaneously produce exactly ONE refresh call; token file on disk holds the rotated refresh token.

**Provider gotchas (operator-supplied, authoritative):**
- **Whoop:** recovery + sleep live on `/developer/v2` (v1 404s); cycle works on v2. `v2/recovery` → `score.recovery_score` (recovery %), `score.hrv_rmssd_milli` (hrv ms), `score.resting_heart_rate` (heart_rate bpm), `score.spo2_percentage` (spo2 %). `v2/activity/sleep` → `score.stage_summary`: sleep_hours = (`total_in_bed_time_milli` − `total_awake_time_milli`) / 3,600,000; `score.respiratory_rate` → respiratory_rate. `v2/cycle` → `score.strain` → strain. **VERIFY** before coding: OAuth endpoints (believed `https://api.prod.whoop.com/oauth/oauth2/auth` + `/oauth/oauth2/token`), scopes (read:recovery, read:sleep, read:cycles, offline), pagination params (`limit`, `nextToken`), record timestamp fields — use sleep `end` for sleep metrics, cycle `start` for strain, recovery's linked timestamps per docs.
- **Withings:** token endpoint is nonstandard (`https://wbsapi.withings.net/v2/oauth2`, `action=requesttoken`, response wrapped in `{status, body:{...}}`) — hand-roll exchange/refresh, don't fight x/oauth2's TokenSource. `getmeas` (`https://wbsapi.withings.net/measure`, `action=getmeas`): type 1 = weight, type 6 = fat_ratio (body_fat); real value = `value × 10^unit`; group `date` (unix) → recorded_at. Sleep: `v2/sleep` `action=getsummary` → sleep_hours = (light+deep+rem durations, seconds)/3600, recorded at series `enddate`. Refresh tokens rotate + single-use → same lock discipline. **VERIFY**: authorize URL (`https://account.withings.com/oauth2_user/authorize2`), scope names (`user.metrics`, `user.activity`), exact summary field names (`lightsleepduration`, `deepsleepduration`, `remsleepduration`).
- **Emfit:** multiple devices per account — `device_id` required in config. `GET https://qs-api.emfit.com/api/v1/presence/{device_id}/latest`. Night's stable timestamp = `minitrend_datestamps[-1].ts` (no top-level timestamp; tolerate both object-with-ts and bare-number array shapes). `sleep_duration` seconds → sleep_hours; `hrv_rmssd_morning` → hrv; `measured_hr_avg` → heart_rate; `measured_rr_avg` → respiratory_rate. **VERIFY**: auth mechanism (token header form; login endpoint if any) — fallback: operator pastes a token into config.

**All providers:** every metric written via `repo.UpsertMetric(models.NewMetric(...).WithSource(models.SourceX).WithRecordedAt(ts))` — sync is idempotent by construction, no state file. Client HTTP timeout 30s. Base URLs are constructor parameters so tests inject httptest servers.

### Task 9: Config sync section + token store with serialized refresh
- [ ] TDD the config additions (round-trip Save/Load with sync section; 0600 perms on save when sync configured).
- [ ] TDD `tokens.go`: Save→Load round-trip; 0600 perms; concurrent-refresh test proving exactly one refresh HTTP call and rotated token persisted.
- [ ] `go get golang.org/x/sys` (promote to direct); build+test; commit `feat(sync): token store with serialized refresh`.

### Task 10: Whoop provider
- [ ] **VERIFY step:** fetch https://developer.whoop.com docs for v2 endpoints/pagination/auth; record findings in the commit message. STOP if they contradict the mapping above.
- [ ] TDD against httptest fixtures: recovery/sleep/cycle JSON → correct metrics (values, units, sources, timestamps); pagination followed; expired-token triggers locked refresh; re-sync produces zero new rows (upsert).
- [ ] Commit `feat(sync): whoop provider`.

### Task 11: Withings provider
- [ ] **VERIFY step:** fetch Withings developer docs for authorize/requesttoken and getmeas/getsummary field names.
- [ ] TDD: `value × 10^unit` math (e.g. value=82500, unit=-3 → 82.5 kg); type 1→weight, 6→body_fat; sleep summary seconds→hours; wrapped `{status,body}` token responses; nonzero `status` surfaces as error.
- [ ] Commit `feat(sync): withings provider`.

### Task 12: Emfit provider
- [ ] **VERIFY step:** confirm auth mechanism against available Emfit QS API docs; if undocumented, config token only.
- [ ] TDD: latest-presence fixture → sleep_hours/hrv/heart_rate/respiratory_rate at `minitrend_datestamps[-1].ts`; both datestamp shapes tolerated; missing device_id → clear error.
- [ ] Commit `feat(sync): emfit provider`.

### Task 13: `health sync` CLI + OAuth auth flow
- [ ] `health sync whoop|withings|emfit [--days N]` runs the provider, prints Summary{Added, Updated} per metric type; `health sync auth whoop|withings` runs localhost-callback OAuth (default redirect `http://localhost:42021/callback`, overridable via config; check port free first), prints the URL for remote use (tailscale reminder: print exact URL, don't auto-open only).
- [ ] Root help + README sync section (setup walkthrough per provider, credential storage locations, offline-first note).
- [ ] Commit `feat(cli): health sync command with oauth auth flow`.

### Task 14: Sync end-to-end test
- [ ] `test/` e2e: all three providers against httptest servers into one temp repo; run twice; assert second pass adds zero rows and updates in place; assert list --source filters per provider.
- [ ] Commit `test(sync): end-to-end provider sync coverage`.

---

## Self-Review Notes

- Spec coverage: source field (Tasks 1–6, 8), CLI flags (5), MCP exposure (6), default manual + migration (2), idempotent upsert (4, CLI 5, MCP 6), four new types w/ units+categories+tests (7), export/README (8), native sync incl. token rotation lock, Emfit device/timestamp, Withings 10^unit, Whoop v2 mapping (9–14). Offline-first/no-telemetry preserved (sync only on explicit command).
- Type consistency: `ListMetrics(metricType *models.MetricType, source *string, limit int)` and `UpsertMetric(m *models.Metric) (bool, error)` used identically across Tasks 3–8.
- Known judgment calls: sources normalized to lowercase; upsert updates oldest legacy duplicate; strain unit "score"; recovery/strain categorized under activity, respiratory_rate/spo2 under biometrics.
