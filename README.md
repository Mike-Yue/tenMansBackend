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
connection (`db.Open`) and schema migrations (`db.Migrate`). `main.go` wires
each repository → service → handler and registers routes.

## Endpoints

| Method & path | Description |
|---|---|
| `GET /healthz` | Liveness check (no DB access) |
| `GET /api/users` | List all users |
| `GET /api/users/{id}` | One user by **Steam ID** |
| `GET /api/users/{id}/stats` | A user's aggregated all-time stats (kills, deaths, assists, mvps, winrate) |
| `GET /api/seasons` | List all seasons, newest first |
| `POST /api/seasons` | Create a season. Body: `{ name, startAt, endAt }` with dates as `YYYY-MM-DD` |
| `GET /api/matches?season={id}` | List matches; the `season` query param is optional |
| `POST /api/matches` | Create a match. Currently fabricates a random match (stand-in for a future demo-upload/parser pipeline) |
| `GET /api/matches/{matchId}` | One match with both teams and every player's scoreboard |

## Running locally

Requires **Go 1.26+**.

```bash
go run .
```

The server listens on `http://localhost:8080`.

Try it:

```bash
curl http://localhost:8080/api/users
curl -X POST http://localhost:8080/api/matches
```

### Configuration (environment variables)

| Variable | Default | Purpose |
|---|---|---|
| `DB_PATH` | `production.db` | Path to the SQLite file.
| `PORT` | `8080` | Port to listen on. |
| `CORS_ALLOWED_ORIGINS` | *(unset = allow all)* | Comma-separated list of allowed browser origins, e.g. `https://tenmansfrontend.onrender.com`. |

## Database & migrations

The schema lives in `db/migrations/` as goose-annotated `.sql` files (embedded into the
binary). On startup `db.Migrate` runs any not-yet-applied migrations against the database at
`DB_PATH`, tracking what's applied in a `goose_db_version` table — so it's a no-op once the DB
is up to date and safe to run on every boot. A fresh/empty database gets the full schema built
from migration `00001_baseline.sql`; an existing one only gets new migrations.

The `.db` file itself is **no longer committed** (it's gitignored) — it holds data only. Do not
overwrite production's file; to change the schema, add a new migration instead.

### Changing the schema

Add a new numbered file, e.g. `db/migrations/00002_add_region.sql`:

```sql
-- +goose Up
ALTER TABLE matches ADD COLUMN region TEXT;

-- +goose Down
ALTER TABLE matches DROP COLUMN region;
```

Commit and deploy; goose applies it to the live DB in place, preserving existing rows. Run the
server locally against your dev DB to apply the same migration there. For changes SQLite's
limited `ALTER TABLE` can't express (retyping a column, changing a CHECK constraint), use the
create-new-table → copy → drop → rename pattern inside a single migration.

## TODO

1. Steam ID Login to view the webpage
2. S3 storage + upload path for demos
3. Create match parser service (probably Andrew?)
4. ~~Implement proper DB migration and schema~~ ✅ (goose migrations in `db/migrations/`) + add development db
5. Glicko 2 elo system implementation