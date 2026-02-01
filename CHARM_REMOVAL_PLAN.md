# Health - Charm Removal Plan

## CRITICAL: Full KV→SQLite Migration

Health uses Charm KV with E2E encryption as primary storage. No existing SQLite backend.

## Charmbracelet Dependencies

**REMOVE:**
- `github.com/charmbracelet/charm` (2389-research fork)

## Key Prefixes in Charm KV

- `metric:` - Health metrics
- `workout:` - Workout sessions
- `workout_metric:` - Metrics within workouts

## Files Using Charm

| File | Purpose |
|------|---------|
| `internal/charm/client.go` | Core KV wrapper |
| `internal/charm/metrics.go` | Metric CRUD |
| `internal/charm/workouts.go` | Workout CRUD with cascade |
| `internal/charm/config.go` | Config (charm server, auto-sync) |
| `cmd/health/sync.go` | Sync commands |
| `cmd/health/root.go` | Client init |

## The 22 Metric Types

**Biometrics:** weight, body_fat, bp_sys, bp_dia, heart_rate, hrv, temperature

**Activity:** steps, sleep_hours, active_calories

**Nutrition:** water, calories, protein, carbs, fat

**Mental Health:** mood, energy, stress, anxiety, focus, meditation

## SQLite Schema

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS metrics (
    id TEXT PRIMARY KEY,
    metric_type TEXT NOT NULL,
    value REAL NOT NULL,
    unit TEXT NOT NULL,
    recorded_at DATETIME NOT NULL,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workouts (
    id TEXT PRIMARY KEY,
    workout_type TEXT NOT NULL,
    started_at DATETIME NOT NULL,
    duration_minutes INTEGER,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workout_metrics (
    id TEXT PRIMARY KEY,
    workout_id TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    value REAL NOT NULL,
    unit TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workout_id) REFERENCES workouts(id) ON DELETE CASCADE
);

CREATE INDEX idx_metrics_type ON metrics(metric_type);
CREATE INDEX idx_metrics_recorded ON metrics(recorded_at DESC);
CREATE INDEX idx_metrics_type_recorded ON metrics(metric_type, recorded_at DESC);  -- Composite for "get weight history"
CREATE INDEX idx_workouts_started ON workouts(started_at DESC);
CREATE INDEX idx_workout_metrics_workout ON workout_metrics(workout_id);
```

## Encryption Considerations

**What Charm provided:** E2E encryption via SSH key before cloud upload.

**Recommendation:** File permissions only (`0600`). Data is local-only, no cloud sync. Physical access = full system compromise anyway.

## Migration Command

```bash
health migrate           # Migrate from Charm KV to SQLite
health migrate --dry-run # Show what would be migrated
```

## Export Commands

```bash
health export markdown [--type TYPE] [--since DATE]
health export yaml [--output FILE]
health export json [--output FILE]  # Full backup
health import json backup.json      # Restore
```

**Markdown Format:**
```markdown
# Health Metrics Export

## Weight
| Date | Value | Notes |
|------|-------|-------|
| 2026-01-31 | 82.5 kg | |
```

**YAML Format:**
```yaml
version: "1.0"
exported_at: "2026-01-31T15:00:00Z"
tool: "health"

metrics:
  weight:
    - id: abc12345
      value: 82.5
      unit: kg
      recorded_at: 2026-01-31T08:00:00Z
workouts:
  - id: def67890
    type: run
    started_at: 2026-01-31T07:00:00Z
    duration_minutes: 45
    metrics:
      - name: distance
        value: 5.2
        unit: km
```

## Files to Modify

### DELETE:
- `internal/charm/client.go`
- `internal/charm/metrics.go`
- `internal/charm/workouts.go`
- `internal/charm/config.go`
- `internal/charm/wal_test.go`
- `internal/charm/metrics_test.go`

### CREATE:
- `internal/storage/db.go` - SQLite connection
- `internal/storage/schema.go` - Migrations
- `internal/storage/metrics.go` - Metric CRUD
- `internal/storage/workouts.go` - Workout CRUD
- `internal/storage/repository.go` - Interface
- `internal/storage/export.go` - Export functionality
- `internal/storage/migrate.go` - Charm migration
- `cmd/health/export.go` - Export commands

### MODIFY:
- `go.mod` - Remove charm, add sqlite
- `cmd/health/root.go` - Use storage client
- `cmd/health/add.go` - Use storage
- `cmd/health/list.go` - Use storage
- `cmd/health/delete.go` - Use storage
- `cmd/health/workout.go` - Use storage
- `cmd/health/sync.go` - Remove or replace with export
- `cmd/health/mcp.go` - Use storage
- `internal/mcp/server.go` - Use storage.Repository
- `internal/mcp/tools.go` - Update calls
- `internal/mcp/resources.go` - Update calls

## Data Path

| Current | New |
|---------|-----|
| `~/.local/share/charm/kv/health/` | `~/.local/share/health/health.db` |

## Implementation Order

1. Create `internal/storage/` package
2. Implement SQLite with XDG paths
3. Implement Repository interface matching charm.Client API
4. Create migrate command
5. Update CLI to use storage layer
6. Add export commands (markdown, yaml, json)
7. Remove charm package
8. Remove sync commands
9. Update go.mod
10. Update documentation
