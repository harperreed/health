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
├── pyproject.toml
├── src/health/
│   ├── __init__.py
│   ├── __main__.py           # CLI entry point
│   ├── cli/
│   │   ├── __init__.py
│   │   ├── root.py           # Main CLI (Click)
│   │   ├── metrics.py        # add/list/query metrics commands
│   │   ├── workouts.py       # workout commands
│   │   └── mcp.py            # MCP server launch command
│   ├── db/
│   │   ├── __init__.py
│   │   ├── database.py       # Connection, init, XDG paths
│   │   ├── schema.py         # SQL schema
│   │   ├── metrics.py        # Metric CRUD
│   │   └── workouts.py       # Workout CRUD
│   ├── models/
│   │   ├── __init__.py
│   │   ├── metric.py         # Metric dataclass + types enum
│   │   └── workout.py        # Workout + WorkoutMetric dataclasses
│   └── mcp/
│       ├── __init__.py
│       ├── server.py         # MCP server setup
│       ├── tools.py          # Tool implementations
│       └── resources.py      # Resource implementations
└── tests/
    ├── test_db.py
    ├── test_models.py
    └── test_mcp.py
```

## Dependencies

- `click` - CLI framework
- `mcp` - Official MCP Python SDK
- `platformdirs` - XDG path handling (cross-platform)
- Standard library `sqlite3`

## Database

**Location:** `~/.local/share/health/health.db` (XDG_DATA_HOME)

**Pragmas:** WAL mode, foreign keys enabled, SYNCHRONOUS=NORMAL

## Data Model

### Metric Types

```python
class MetricType(str, Enum):
    # Biometrics
    WEIGHT = "weight"              # kg
    BODY_FAT = "body_fat"          # percentage
    BLOOD_PRESSURE_SYS = "bp_sys"  # mmHg
    BLOOD_PRESSURE_DIA = "bp_dia"  # mmHg
    HEART_RATE = "heart_rate"      # bpm
    HRV = "hrv"                    # ms (RMSSD)
    TEMPERATURE = "temperature"    # celsius

    # Activity
    STEPS = "steps"                # count
    SLEEP_HOURS = "sleep_hours"    # decimal hours
    ACTIVE_CALORIES = "active_calories"  # kcal

    # Nutrition
    WATER = "water"                # ml
    CALORIES = "calories"          # kcal
    PROTEIN = "protein"            # grams
    CARBS = "carbs"                # grams
    FAT = "fat"                    # grams

    # Mental Health
    MOOD = "mood"                  # 1-10 scale
    ENERGY = "energy"              # 1-10 scale
    STRESS = "stress"              # 1-10 scale
    ANXIETY = "anxiety"            # 1-10 scale
    FOCUS = "focus"                # 1-10 scale
    MEDITATION = "meditation"      # minutes
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
