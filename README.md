# health

A fast, privacy-focused CLI for tracking personal health metrics with cloud sync and AI assistant integration.

## Features

- **22 metric types** across biometrics, activity, nutrition, and mental health
- **Workout tracking** with custom sub-metrics (distance, pace, heart rate, etc.)
- **End-to-end encrypted sync** across devices via Charm Cloud
- **MCP server** for AI assistant integration (Claude Desktop, etc.)
- **Backdating support** for logging historical data
- **SQLite storage** for reliable concurrent access

## Installation

### Homebrew (macOS)

```bash
brew tap harperreed/homebrew-tap
brew install health
```

### Go Install

```bash
go install github.com/harperreed/health/cmd/health@latest
```

### Build from Source

```bash
git clone https://github.com/harperreed/health.git
cd health
go build -o health ./cmd/health
```

## Quick Start

```bash
# Log some metrics
health add weight 82.5
health add bp 120 80
health add mood 7 --notes "Good day"
health add steps 10432

# View recent entries
health list
health list --type weight

# Log a workout
health workout add run --duration 30
health workout metric <id> distance 5.0 km

# Set up cloud sync
health sync link
```

## Commands

### `health add` - Record Metrics

```bash
health add <type> <value> [flags]
health add bp <systolic> <diastolic>  # Blood pressure (special case)
```

**Flags:**
- `--at <timestamp>` - Backdate entry (e.g., `"2024-12-14 07:00"`, `"2024-12-14"`)
- `--notes <string>` - Add notes

**Examples:**
```bash
health add weight 82.5
health add hrv 48 --at "2024-12-14 07:00"
health add mood 7 --notes "Morning check-in"
health add sleep_hours 7.5
```

### `health list` - View Metrics

```bash
health list [flags]
```

**Flags:**
- `-t, --type <type>` - Filter by metric type
- `-n, --limit <int>` - Max results (default: 20)

**Examples:**
```bash
health list
health list --type weight -n 30
health ls -t mood
```

### `health delete` - Remove Metrics

```bash
health delete <id>
health rm <id-prefix>
```

### `health workout` - Manage Workouts

```bash
# Create workout
health workout add run --duration 45 --notes "Morning run"

# Add metrics to workout
health workout metric <id> distance 5.2 km
health workout metric <id> avg_hr 145 bpm

# View workouts
health workout list
health workout show <id>

# Delete workout
health workout delete <id>
```

### `health sync` - Cloud Synchronization

```bash
health sync link      # Connect to Charm Cloud
health sync status    # Check sync status
health sync unlink    # Disconnect
health sync wipe      # Reset local data from cloud
```

## Supported Metrics

### Biometrics
| Type | Unit | Description |
|------|------|-------------|
| `weight` | kg | Body weight |
| `body_fat` | % | Body fat percentage |
| `bp` | mmHg | Blood pressure (creates bp_sys + bp_dia) |
| `heart_rate` | bpm | Resting heart rate |
| `hrv` | ms | Heart rate variability |
| `temperature` | °C | Body temperature |

### Activity
| Type | Unit | Description |
|------|------|-------------|
| `steps` | steps | Daily step count |
| `sleep_hours` | hours | Sleep duration |
| `active_calories` | kcal | Calories burned |

### Nutrition
| Type | Unit | Description |
|------|------|-------------|
| `water` | ml | Water intake |
| `calories` | kcal | Caloric intake |
| `protein` | g | Protein intake |
| `carbs` | g | Carbohydrate intake |
| `fat` | g | Fat intake |

### Mental Health (1-10 scale)
| Type | Description |
|------|-------------|
| `mood` | Overall mood |
| `energy` | Energy level |
| `stress` | Stress level |
| `anxiety` | Anxiety level |
| `focus` | Focus/concentration |
| `meditation` | Minutes meditated |

## MCP Server Integration

The health CLI includes an MCP server for AI assistant integration.

### Setup for Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "health": {
      "command": "health",
      "args": ["mcp"]
    }
  }
}
```

### Available Tools

- `add_metric` - Record a health metric
- `list_metrics` - List recent metrics
- `delete_metric` - Delete a metric
- `add_workout` - Create workout session
- `add_workout_metric` - Add metric to workout
- `list_workouts` - List workouts
- `get_workout` - Get workout details
- `delete_workout` - Delete a workout
- `get_latest` - Get most recent value for metric types

### Available Resources

- `health://recent` - Last 10 metrics + 5 workouts
- `health://today` - Today's entries
- `health://summary` - Latest value per metric type

## Data Storage

- **Location:** `~/.local/share/charm/kv/health`
- **Backend:** SQLite via Charm KV
- **Sync:** End-to-end encrypted with SSH key

## Development

```bash
# Build
go build -o health ./cmd/health

# Test
go test ./...

# Install locally
go build -o health ./cmd/health && mv health ~/.local/bin/
```

## License

MIT
