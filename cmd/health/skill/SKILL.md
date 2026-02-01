---
name: health
description: Health metrics and workout tracking - log weight, exercise, vitals, nutrition, and mood. Use when the user mentions health data or wants to track wellness metrics.
---

# health - Health Tracking

Track 22 metric types: biometrics, activity, nutrition, and mental health.

## When to use health

- User mentions weight, exercise, sleep, or health data
- User wants to log a workout or health metric
- User asks about their health trends
- User tracks mood, energy, or mental health

## Metric types

**Biometrics:** weight, body_fat, bp_sys, bp_dia, heart_rate, hrv, temperature
**Activity:** steps, sleep_hours, active_calories
**Nutrition:** water, calories, protein, carbs, fat
**Mental Health:** mood, energy, stress, anxiety, focus, meditation

## Available MCP tools

| Tool | Purpose |
|------|---------|
| `mcp__health__add_metric` | Log a health metric |
| `mcp__health__list_metrics` | Get metrics by type/date |
| `mcp__health__get_latest` | Get most recent value |
| `mcp__health__add_workout` | Log a workout session |
| `mcp__health__list_workouts` | Get workout history |
| `mcp__health__delete_metric` | Remove a metric |

## Common patterns

### Log weight
```
mcp__health__add_metric(metric_type="weight", value=82.5, unit="kg")
```

### Log a workout
```
mcp__health__add_workout(workout_type="run", duration_minutes=45, notes="Morning 5k")
```

### Check latest weight
```
mcp__health__get_latest(metric_type="weight")
```

### Log mood (1-10 scale)
```
mcp__health__add_metric(metric_type="mood", value=7, unit="score")
```

### Get weight history
```
mcp__health__list_metrics(metric_type="weight", since="2026-01-01")
```

## CLI commands (if MCP unavailable)

```bash
health add weight 82.5 kg
health add mood 7 --notes "Good day"
health workout run --duration 45 --notes "Morning jog"
health list weight --since 7d
health export markdown --type weight
```

## Data location

`~/.local/share/health/health.db` (SQLite, respects XDG_DATA_HOME)
