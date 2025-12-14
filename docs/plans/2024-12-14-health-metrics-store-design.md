# Health Metrics Store Design

A SQLite-backed health metrics store with MCP server, following patterns from toki and chronicle.

## Overview

Manual entry system for tracking health data across four categories:
- **Biometrics** - weight, body composition, vitals, HRV
- **Activity** - steps, sleep, workouts with sub-metrics
- **Nutrition** - calories, macros, hydration
- **Mental Health** - mood, stress, energy, focus

No external integrations initially - pure manual data entry via CLI and MCP tools.

## Project Structure

```
health/
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yml
├── cmd/health/
│   ├── main.go               # Entry point
│   ├── root.go               # Root command, DB init
│   ├── add.go                # Add metric command
│   ├── list.go               # List metrics command
│   ├── workout.go            # Workout subcommands
│   ├── mcp.go                # MCP server command
│   └── version.go            # Version info
├── internal/
│   ├── db/
│   │   ├── db.go             # Connection, init, XDG paths
│   │   ├── schema.go         # SQL schema
│   │   ├── metrics.go        # Metric CRUD
│   │   ├── workouts.go       # Workout CRUD
│   │   └── *_test.go
│   ├── models/
│   │   ├── metric.go         # Metric struct + MetricType
│   │   ├── workout.go        # Workout + WorkoutMetric structs
│   │   └── *_test.go
│   ├── mcp/
│   │   ├── server.go         # MCP server setup
│   │   ├── tools.go          # Tool implementations
│   │   ├── resources.go      # Resource implementations
│   │   └── *_test.go
│   └── ui/
│       └── format.go         # Color output helpers
├── test/
│   └── integration_test.go
└── docs/
    └── plans/
```

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/modelcontextprotocol/go-sdk` - MCP Go SDK
- `modernc.org/sqlite` - Pure Go SQLite (no CGO)
- `github.com/google/uuid` - UUID generation
- `github.com/fatih/color` - Terminal colors

## Database

**Location:** `~/.local/share/health/health.db` (XDG_DATA_HOME)

**Pragmas:** WAL mode, foreign keys enabled, SYNCHRONOUS=NORMAL

## Data Model

### Metric Types

```go
type MetricType string

const (
    // Biometrics
    MetricWeight      MetricType = "weight"       // kg
    MetricBodyFat     MetricType = "body_fat"     // percentage
    MetricBPSys       MetricType = "bp_sys"       // mmHg
    MetricBPDia       MetricType = "bp_dia"       // mmHg
    MetricHeartRate   MetricType = "heart_rate"   // bpm
    MetricHRV         MetricType = "hrv"          // ms (RMSSD)
    MetricTemperature MetricType = "temperature"  // celsius

    // Activity
    MetricSteps          MetricType = "steps"           // count
    MetricSleepHours     MetricType = "sleep_hours"     // decimal hours
    MetricActiveCalories MetricType = "active_calories" // kcal

    // Nutrition
    MetricWater    MetricType = "water"    // ml
    MetricCalories MetricType = "calories" // kcal
    MetricProtein  MetricType = "protein"  // grams
    MetricCarbs    MetricType = "carbs"    // grams
    MetricFat      MetricType = "fat"      // grams

    // Mental Health
    MetricMood       MetricType = "mood"       // 1-10 scale
    MetricEnergy     MetricType = "energy"     // 1-10 scale
    MetricStress     MetricType = "stress"     // 1-10 scale
    MetricAnxiety    MetricType = "anxiety"    // 1-10 scale
    MetricFocus      MetricType = "focus"      // 1-10 scale
    MetricMeditation MetricType = "meditation" // minutes
)

// MetricUnits maps metric types to their units
var MetricUnits = map[MetricType]string{
    MetricWeight: "kg", MetricBodyFat: "%", MetricBPSys: "mmHg",
    MetricBPDia: "mmHg", MetricHeartRate: "bpm", MetricHRV: "ms",
    MetricTemperature: "°C", MetricSteps: "steps", MetricSleepHours: "hours",
    MetricActiveCalories: "kcal", MetricWater: "ml", MetricCalories: "kcal",
    MetricProtein: "g", MetricCarbs: "g", MetricFat: "g",
    MetricMood: "scale", MetricEnergy: "scale", MetricStress: "scale",
    MetricAnxiety: "scale", MetricFocus: "scale", MetricMeditation: "min",
}
```

### Schema

```sql
CREATE TABLE metrics (
    id TEXT PRIMARY KEY,          -- UUID
    metric_type TEXT NOT NULL,    -- from MetricType enum
    value REAL NOT NULL,
    unit TEXT NOT NULL,
    recorded_at DATETIME NOT NULL,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE workouts (
    id TEXT PRIMARY KEY,
    workout_type TEXT NOT NULL,   -- run, lift, cycle, swim, etc.
    started_at DATETIME NOT NULL,
    duration_minutes INTEGER,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE workout_metrics (
    id TEXT PRIMARY KEY,
    workout_id TEXT NOT NULL REFERENCES workouts(id) ON DELETE CASCADE,
    metric_name TEXT NOT NULL,    -- distance, pace, sets, reps, weight, etc.
    value REAL NOT NULL,
    unit TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_metrics_type_date ON metrics(metric_type, recorded_at);
CREATE INDEX idx_workouts_type_date ON workouts(workout_type, started_at);
CREATE INDEX idx_workout_metrics_workout ON workout_metrics(workout_id);
```

Blood pressure: two entries (bp_sys + bp_dia) with same `recorded_at` timestamp.

## MCP Tools

### CRUD Tools

| Tool | Description |
|------|-------------|
| `add_metric` | Record a single metric (type, value, optional timestamp, notes) |
| `list_metrics` | List recent metrics, optionally filter by type |
| `delete_metric` | Remove a metric by ID (prefix matching) |
| `add_workout` | Create workout session (type, duration, notes) |
| `add_workout_metric` | Add a metric to an existing workout |
| `list_workouts` | List recent workouts, filter by type |
| `get_workout` | Get full workout with all its metrics |
| `delete_workout` | Remove workout and its metrics (cascade) |

### Query Tools

| Tool | Description |
|------|-------------|
| `get_metric_range` | Get metrics of a type between two dates |
| `get_averages` | Average value for a metric type over a period |
| `compare_periods` | Compare this week vs last week, this month vs last, etc. |
| `get_latest` | Get most recent value for one or more metric types |

### Semantic Tools

| Tool | Description |
|------|-------------|
| `log_health_check` | Log multiple metrics at once: "weight 82kg, hrv 45, sleep 7.5, mood 7" |
| `how_am_i_doing` | Summarize recent trends across all categories |
| `whats_trending` | Identify metrics that changed significantly recently |

## MCP Resources

| URI | Description |
|-----|-------------|
| `health://recent` | Last 10 entries across all metrics |
| `health://today` | Today's logged metrics |
| `health://summary` | Dashboard: latest of each metric type + trends |

## CLI Commands

```bash
# Metrics
health add weight 82.5                    # Add metric (today, now)
health add hrv 48 --at "2024-12-14 07:00" # With specific timestamp
health add bp 120 80                      # Blood pressure (two values)
health add mood 7 --notes "Good day"      # Mental health with notes
health list                               # Recent metrics (all types)
health list --type weight --days 30       # Filter by type and range

# Workouts
health workout add run --duration 45 --notes "Easy morning jog"
health workout metric <workout-id> distance 5.2 km
health workout metric <workout-id> avg_hr 145 bpm
health workout list
health workout show <workout-id>          # Full workout with metrics

# Queries
health trend weight --days 30             # Show trend for metric
health latest                             # Most recent of each type
health compare week                       # This week vs last week

# MCP Server
health mcp                                # Start stdio MCP server
```

## Design Decisions

1. **Single metrics table** - Flexible, easy to add new metric types without schema changes
2. **Separate workouts table** - Workouts are complex entities with sub-metrics, warrant their own structure
3. **UUID primary keys** - Consistent with toki/chronicle, enables prefix matching for CLI
4. **Opinionated metric types** - Enum enforces valid types, provides units, enables validation
5. **Mental health as 1-10 scales** - Simple, comparable over time, notes field for context
6. **No external integrations** - Start simple, add Withings/Apple Health later if needed
