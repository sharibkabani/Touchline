# Touchline

Touchline is a Bubble Tea terminal application for live FIFA World Cup scores, match details, and group standings.

## Architecture

Touchline keeps terminal UI, service orchestration, provider access, and domain types separate:

- `cmd/touchline` is the composition root. It wires config, logging, providers, caches, services, and the Bubble Tea program.
- `internal/types` contains provider-neutral football domain types.
- `internal/api` defines provider interfaces, the live ESPN provider, and a mock provider.
- `internal/services` owns cache-aware application use cases.
- `internal/tui` owns Bubble Tea state transitions, rendering, and Lip Gloss styles.
- `internal/cache` provides a small TTL cache used by services.
- `internal/config` loads `.env` and environment variables.
- `mock` contains local JSON responses for development without API keys.

The main architectural decision is that the TUI never calls a concrete API client. It talks to services, and services depend on interfaces. That keeps the code ready for multiple providers, competitions, SSH mode with Wish, WebSocket updates, player statistics, timelines, xG, and win probability without rewriting rendering logic.

## Live Data (ESPN)

By default Touchline pulls live FIFA World Cup data from ESPN's public soccer API. The provider is **date-aware**: pressing `left`/`right` changes the day and fetches that day's scoreboard from `.../soccer/fifa.world/scoreboard?dates=YYYYMMDD`. A single request per day returns the full picture, so match details (statistics plus a goal/card timeline with scorers and minutes) are bundled with the match list and cached together.

- Matches, scores, status, and per-match details/statistics come from ESPN.
- Group standings come from ESPN's standings endpoint.

Set `TOUCHLINE_PROVIDER=mock` to run fully offline against the bundled JSON.

## Controls

- `q`: quit
- `r`: refresh current view
- `left/right` or `h/l`: move the match list one day backward or forward
- `tab`: switch between the dashboard and group standings
- `up/down` or `k/j`: select a match (its details show in the right pane) or scroll

## Run

```sh
go mod tidy
go run ./cmd/touchline
```

To build a binary:

```sh
go build -o bin/touchline ./cmd/touchline
./bin/touchline
```

## Serve over SSH

Touchline can be served over SSH using [charmbracelet/wish](https://github.com/charmbracelet/wish). Each connection gets its own Bubble Tea program (independent view state), while the data services and caches are shared across all sessions so concurrent viewers reuse the same upstream data.

```sh
TOUCHLINE_SSH=true go run ./cmd/touchline
```

Then, from another terminal:

```sh
ssh -p 23234 localhost
```

A host key is generated automatically on first run at `TOUCHLINE_SSH_HOST_KEY_PATH` (default `.ssh/touchline_ed25519`). Connections require an interactive terminal (enforced by Wish's `activeterm` middleware); colors are forced on so SSH clients get the full themed UI.

## Configuration

Copy `.env.example` to `.env` and adjust values as needed.

```sh
cp .env.example .env
```

Supported variables:

- `TOUCHLINE_PROVIDER`: provider name. `espn` (default, live) or `mock` (offline).
- `TOUCHLINE_COMPETITION`: competition code. Currently `world-cup`.
- `TOUCHLINE_MOCK_DIR`: mock JSON directory.
- `TOUCHLINE_ESPN_BASE_URL`: optional override for the ESPN scoreboard base URL.
- `TOUCHLINE_REFRESH_INTERVAL`: auto-refresh interval, for example `30s`.
- `TOUCHLINE_CACHE_TTL`: in-memory cache TTL, for example `25s`.
- `TOUCHLINE_LOG_LEVEL`: `debug`, `info`, `warn`, or `error`.
- `TOUCHLINE_SSH`: set to `true` to serve the TUI over SSH instead of running locally.
- `TOUCHLINE_SSH_ADDR`: SSH listen address, for example `localhost:23234`.
- `TOUCHLINE_SSH_HOST_KEY_PATH`: host key path (auto-generated if missing).

## Mock Data

Mock responses live in:

- `mock/live_matches.json`
- `mock/match_details.json`
- `mock/standings.json`

These files let the full TUI run locally without network access or API keys.

## Adding Another Football API

The live ESPN provider (`internal/api/espn.go`) is a complete reference implementation. To add a different source:

1. Add a new provider type under `internal/api`.
2. Implement `FootballProvider`:

```go
type FootballProvider interface {
	GetScoreboard(ctx context.Context, competition types.CompetitionCode, date time.Time) (types.Scoreboard, error)
	GetStandings(ctx context.Context, competition types.CompetitionCode) ([]types.GroupStanding, error)
}
```

   `GetScoreboard` returns a `types.Scoreboard` containing the day's `Matches` plus a `Details` map keyed by match ID, so matches and their statistics/timeline are fetched together.

3. Use `net/http` inside the provider and map provider-specific payloads into `internal/types`.
4. Register the provider in `buildProvider` in `cmd/touchline/main.go`.
5. Add `.env` variables for the API base URL and API key.

Keep provider DTOs private to `internal/api` so provider changes do not leak into the TUI or services.
