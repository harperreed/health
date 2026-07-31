# health

A fast, privacy-focused CLI for tracking personal health metrics with cloud sync and AI assistant integration.

## Features

- **25 metric types** across biometrics, activity, nutrition, and mental health
- **Source tagging** to track where data comes from (whoop, withings, emfit, manual, or custom)
- **Idempotent upsert** (`--dedupe`) for safe repeated imports from wearables
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
- `--source/-s <string>` - Tag the data source: `whoop`, `withings`, `emfit`, `manual`, or any free-form string (default: `manual`)
- `--dedupe` - Upsert: update an existing entry with the same source, type, and timestamp instead of creating a duplicate

**Examples:**
```bash
health add weight 82.5
health add hrv 48 --at "2024-12-14 07:00"
health add mood 7 --notes "Morning check-in"
health add sleep_hours 7.5
health add hrv 48 --source whoop --dedupe    # Idempotent sync write
health add recovery 85 --source whoop        # Recovery score from Whoop
health add strain 14.2 --source whoop        # Strain score from Whoop
health add spo2 98 --source whoop            # Blood oxygen from Whoop
health add respiratory_rate 16 --source whoop  # Breathing rate from Whoop
```

### `health list` - View Metrics

```bash
health list [flags]
```

**Flags:**
- `-t, --type <type>` - Filter by metric type
- `-n, --limit <int>` - Max results (default: 20)
- `-s, --source <string>` - Filter by data source (e.g., `whoop`, `manual`)

**Examples:**
```bash
health list
health list --type weight -n 30
health ls -t mood
health list --source whoop       # Only Whoop-sourced entries
health list -s manual -t hrv     # Manual HRV entries only
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

### `health sync` - Native Provider Sync

Pull health data from Whoop, Withings, and Emfit directly into local storage.

```bash
health sync whoop               # Sync last 7 days from Whoop
health sync withings --days 30  # Sync last 30 days from Withings
health sync emfit               # Sync latest Emfit sleep night
```

Sync commands are the **only** commands that touch the network; everything else is offline.

#### Whoop setup

1. Create an app at https://developer.whoop.com and get a client ID and secret.
2. Add to `~/.config/health/config.json`:
   ```json
   {
     "sync": {
       "whoop": {
         "client_id": "YOUR_CLIENT_ID",
         "client_secret": "YOUR_CLIENT_SECRET"
       }
     }
   }
   ```
3. Authorize (one-time):
   ```bash
   health sync auth whoop
   ```
   The command prints the authorize URL — open it in your browser. After approval, the token
   is saved automatically. Required scopes: `read:recovery read:sleep read:cycles offline`
   (`offline` is required for a refresh token).

4. Sync:
   ```bash
   health sync whoop
   health sync whoop --days 30
   ```

#### Withings setup

1. Create an app at https://developer.withings.com and get a client ID and secret.
2. Add to `~/.config/health/config.json`:
   ```json
   {
     "sync": {
       "withings": {
         "client_id": "YOUR_CLIENT_ID",
         "client_secret": "YOUR_CLIENT_SECRET"
       }
     }
   }
   ```
3. Authorize (one-time):
   ```bash
   health sync auth withings
   ```
4. Sync:
   ```bash
   health sync withings
   ```

   Scopes granted: `user.info,user.metrics,user.activity`.

#### Emfit QS setup

Emfit does not use OAuth. Supply either a pre-configured token or your username/password.

```json
{
  "sync": {
    "emfit": {
      "device_id": "YOUR_DEVICE_ID",
      "token": "YOUR_STATIC_TOKEN"
    }
  }
}
```

Or with login credentials (token obtained automatically each sync):

```json
{
  "sync": {
    "emfit": {
      "device_id": "YOUR_DEVICE_ID",
      "username": "you@example.com",
      "password": "yourpassword"
    }
  }
}
```

Then:

```bash
health sync emfit
```

#### Credential storage

- **Config file:** `~/.config/health/config.json` — holds client IDs, secrets, and Emfit credentials.
- **OAuth tokens:** `~/.local/share/health/tokens/<provider>.json` — written 0600, updated automatically on each token refresh.
- Whoop and Withings refresh tokens rotate on every use and are persisted immediately.
- The `--days` default is 7. Pass `--days N` to widen the window.

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
| `respiratory_rate` | brpm | Breathing rate |
| `spo2` | % | Blood oxygen saturation |

### Activity
| Type | Unit | Description |
|------|------|-------------|
| `steps` | steps | Daily step count |
| `sleep_hours` | hours | Sleep duration |
| `active_calories` | kcal | Calories burned |
| `recovery` | % | Recovery score (0–100) |
| `strain` | score | Strain score (0–21) |

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

- `add_metric` - Record a health metric. Accepts `source` (string, default `manual`) and `dedupe` (bool) to upsert instead of insert.
- `list_metrics` - List recent metrics. Accepts `source` to filter by data source. Output includes a `Source` field on every metric.
- `delete_metric` - Delete a metric
- `add_workout` - Create workout session
- `add_workout_metric` - Add metric to workout
- `list_workouts` - List workouts
- `get_workout` - Get workout details
- `delete_workout` - Delete a workout
- `get_latest` - Get most recent value for metric types. Output includes `Source` on each returned metric.

### Available Resources

- `health://recent` - Last 10 metrics + 5 workouts (each metric includes `Source`)
- `health://today` - Today's entries
- `health://summary` - Latest value per metric type (includes `Source` on each entry)

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
