# tenMansBackend

A small HTTP JSON API for a CS2 "ten mans" (5v5 inhouse) stats tracker. It serves
players, matches, and aggregated per-player stats out of a SQLite database.

## Tech

- **Go** standard library (`net/http`, Go 1.22+ `ServeMux` method/path routing) — no web framework
- **SQLite** via [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) — a pure-Go driver, so the build needs no CGO
- Opened in WAL mode with a busy timeout and foreign-key enforcement

## Architecture

Each domain is a package split into the same three layers, so HTTP, business
logic, and persistence stay independent and testable:

| File | Responsibility |
|------|----------------|
| `handler.go` | HTTP: parse the request, call the service, write the JSON/status. Owns the response DTOs. |
| `service.go` | Business logic. Depends on a repository **interface**, not the concrete DB type. |
| `data.go` | Persistence: the SQL. Implements the repository interface. |
| `model.go` | Domain structs. |

Domains: **`users`**, **`matches`**, **`stats`**. The **`db`** package handles the
connection (`db.Open`) and one-time seeding (`db.EnsureSeeded`). `main.go` wires
each repository → service → handler and registers routes.

## Endpoints

| Method & path | Description |
|---|---|
| `GET /healthz` | Liveness check (no DB access) |
| `GET /api/users` | List all users |
| `GET /api/users/{id}` | One user by **Steam ID** |
| `GET /api/users/{id}/stats` | A user's aggregated all-time stats (kills, deaths, assists, mvps, winrate) |
| `GET /api/matches?season={id}` | List matches; the `season` query param is optional |
| `POST /api/matches` | Create a match. Currently fabricates a random match (stand-in for a future demo-upload/parser pipeline) |
| `GET /api/matches/{matchId}` | One match with both teams and every player's scoreboard |

## Running locally

Requires **Go 1.26+**.

```bash
go run .
```

The server listens on `http://localhost:8080`. On first start it seeds the
database from the embedded `db/seed.db` (10 players + one season) if the target
file doesn't already exist, then serves from it.

Try it:

```bash
curl http://localhost:8080/api/users
curl -X POST http://localhost:8080/api/matches
```

### Configuration (environment variables)

| Variable | Default | Purpose |
|---|---|---|
| `DB_PATH` | `production.db` | Path to the SQLite file. In production this points at a persistent disk (e.g. `/data/production.db`). |
| `PORT` | `8080` | Port to listen on. |
| `CORS_ALLOWED_ORIGINS` | *(unset = allow all)* | Comma-separated list of allowed browser origins, e.g. `https://tenmansfrontend.onrender.com`. |

## Database & seeding

The schema and initial data live in `db/seed.db`, which is embedded into the
binary via `//go:embed`. On startup, `db.EnsureSeeded` copies it to `DB_PATH`
**only if no file exists there** — so a fresh disk gets seeded on first boot while
existing live data is left untouched on later restarts/deploys.

The local dev database (`production.db` and its `-wal`/`-shm` sidecars) is
gitignored; `db/seed.db` is committed.

## Deployment

Deployed to Render as a native **Go web service** on the Starter plan (needed for
a persistent disk), configured via `render.yaml`: build `go build ... -o app`,
start `./app`, a 1 GB disk mounted at `/data`, `DB_PATH=/data/production.db`, and
health check `/healthz`.
