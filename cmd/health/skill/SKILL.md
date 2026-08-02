---
name: health
description: Health metrics and workout tracking - log weight, exercise, vitals, nutrition, and mood. Use when the user mentions health data or wants to track wellness metrics.
---

# health - Health Tracking

Track 25 metric types: biometrics, activity, nutrition, and mental health.

## When to use health

- User mentions weight, exercise, sleep, or health data
- User wants to log a workout or health metric
- User asks about their health trends
- User tracks mood, energy, or mental health

## Metric types

**Biometrics:** weight, body_fat, bp_sys, bp_dia, heart_rate, hrv, temperature, respiratory_rate, spo2
**Activity:** steps, sleep_hours, active_calories, recovery, strain
**Nutrition:** water, calories, protein, carbs, fat
**Mental Health:** mood, energy, stress, anxiety, focus, meditation

## Available MCP tools

| Tool | Purpose |
|------|---------|
| `mcp__health__add_metric` | Log a health metric (supports `source` and `dedupe`) |
| `mcp__health__list_metrics` | Get metrics by type/source |
| `mcp__health__get_latest` | Get most recent value per type |
| `mcp__health__add_workout` | Log a workout session |
| `mcp__health__add_workout_metric` | Add a metric to a workout |
| `mcp__health__list_workouts` | Get workout history |
| `mcp__health__get_workout` | Get a workout with its metrics |
| `mcp__health__delete_metric` | Remove a metric |
| `mcp__health__delete_workout` | Remove a workout |

## Common patterns

### Log weight (unit is derived from the type)
```
mcp__health__add_metric(metric_type="weight", value=82.5)
```

### Log a workout
```
mcp__health__add_workout(workout_type="run", duration_minutes=45, notes="Morning 5k")
```

### Check latest weight
```
mcp__health__get_latest(metric_types=["weight"])
```

### Log mood (1-10 scale)
```
mcp__health__add_metric(metric_type="mood", value=7)
```

### Get weight history
```
mcp__health__list_metrics(metric_type="weight", limit=30)
```

## CLI commands (if MCP unavailable)

```bash
health add weight 82.5
health add mood 7 --notes "Good day"
health workout add run --duration 45 --notes "Morning jog"
health list --type weight
health export markdown --type weight
```

## Data location

`~/.local/share/health/` (respects XDG_DATA_HOME) — markdown files by default; an existing SQLite store (health.db) is auto-detected and kept.
