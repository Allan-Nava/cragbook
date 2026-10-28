<p align="center"><img src="assets/logo.svg" width="96" height="96" alt="cragbook logo"></p>

<h1 align="center">cragbook</h1>

<p align="center"><strong>A climbing diary on top of <a href="https://github.com/Allan-Nava/cragcast">cragcast</a>: the crags and routes of Lombardy from OpenStreetMap, and which of them you have climbed, are projecting or want to try.</strong></p>

<p align="center">
  <a href="https://github.com/Allan-Nava/cragbook/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/Allan-Nava/cragbook/actions/workflows/ci.yml/badge.svg"></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-10b981"></a>
  <img alt="Go" src="https://img.shields.io/github/go-mod/go-version/Allan-Nava/cragbook?color=10b981">
</p>

---

cragcast answers "can I climb there tomorrow?". cragbook keeps what happened next: for each
route, **climbed** (with the day), **project** or **to try**, and a note. The routes come from
cragcast's public API, which takes them from OpenStreetMap; cragbook imports them into
PostgreSQL and lets you read a crag's routes by level — 4 and under, 5, 6, 7, 8 and over — and
mark them as you go.

The import never touches the diary. A route that leaves OpenStreetMap is marked gone, not
deleted, so an ascent recorded on it stays readable.

## Run it

Go 1.26+, Node 22 with pnpm, and Docker for PostgreSQL:

```bash
docker compose up -d db                      # PostgreSQL 17 on localhost:5432
go run ./cmd/cragbook                        # the API on localhost:8080; imports on first start
cd web && pnpm install && pnpm dev           # the page on localhost:5173, /api proxied to :8080
```

Without a database, everything in memory (the diary is lost on exit):

```bash
CRAGBOOK_STORE=memory go run ./cmd/cragbook
```

All in one container, the page served by the Go binary:

```bash
docker compose --profile app up --build      # http://localhost:8080
```

`go run ./cmd/cragbook import` refreshes the crags and routes and exits; `cragbook version`
prints the version, which `/healthz` reports too.

From a release, no clone needed (linux/amd64 and linux/arm64, a Raspberry Pi included):

```bash
docker run --rm -p 8080:8080 -e CRAGBOOK_STORE=memory ghcr.io/allan-nava/cragbook:latest
```

| Variable | Default | |
|---|---|---|
| `CRAGBOOK_DATABASE_URL` | `postgres://cragbook:cragbook@localhost:5432/cragbook?sslmode=disable` | |
| `CRAGBOOK_ADDR` | `localhost:8080` | |
| `CRAGBOOK_STORE` | `postgres` | or `memory` |
| `CRAGBOOK_CRAGCAST_URL` | `https://allan-nava.github.io/cragcast/api/v1/` | cragcast's API v1 |
| `CRAGBOOK_WEB_DIR` | — | serve the built page (`web/dist`) from the binary |

## API

Described in OpenAPI 3.1 at `GET /api/openapi.json` (the source is
[`internal/transport/httpapi/openapi.json`](internal/transport/httpapi/openapi.json)); a test
holds it to the handlers — every endpoint, every example replayed, every field named.

| | |
|---|---|
| `GET /api/crags?q=&withRoutes=true` | crags by name or municipality, with their route and climbed counts |
| `GET /api/crags/{id}` | one crag, its routes easiest first, each with its `level` and diary entry |
| `PUT /api/routes/{id}/ascent` | `{"status": "done" \| "project" \| "todo", "climbedOn": "YYYY-MM-DD" \| null, "notes": ""}` |
| `DELETE /api/routes/{id}/ascent` | clear the entry |
| `GET /api/diary` | every entry, climbed first (latest day first), then projects, then to try |
| `POST /api/import` | refresh from cragcast; one at a time (409 meanwhile) |

Errors are `{"error": "..."}`: 400 for what the diary refuses (a day on a route not climbed, a
day after today, notes over 2,000 characters), 404 for an unknown crag or route, 502 when
cragcast cannot be read.

## How it is built

Domain-driven, in layers whose dependencies point inwards; `internal/archtest` fails the build
when an import goes the wrong way.

```mermaid
flowchart LR
  subgraph transport
    http["transport/httpapi<br/>handlers, DTOs"]
  end
  subgraph application
    cat["catalogue"]
    jou["journal"]
    imp["importing"]
  end
  subgraph domain
    cl["climbing<br/>Crag, Route, Grade"]
    di["diary<br/>Ascent, Status"]
  end
  subgraph infrastructure
    pg["postgres"]
    mem["memory"]
    cc["cragcast"]
  end
  http --> cat & jou & imp
  cat & jou & imp --> cl & di
  pg & mem & cc -. implement ports .-> cl & di & imp
  app["app (Google Wire)"] --> http & pg & cc
```

- **domain** — `climbing` (crags, routes, the French `Grade` value object, which ranks 6a < 6a+ <
  6b and gives the level) and `diary` (`Ascent`, built only through `NewAscent`, which keeps its
  rules). Repository interfaces live here; the domain imports the standard library only.
- **application** — one use case each: `catalogue` (read crags and routes with their entries),
  `journal` (record, clear, list the diary), `importing` (a few crags at a time; a crag that
  fails is left as it was, never saved without its routes).
- **infrastructure** — `postgres` (pgx, embedded migrations), `memory` (the same ports in
  memory), `cragcast` (the API client, and its translation into the domain). The memory and
  PostgreSQL adapters run the same contract tests in `infrastructure/contract`.
- **transport** — `httpapi`: decode, call a use case, map the domain's errors to statuses. It
  depends on small interfaces of its own, not on the services' types.
- **app** — [Google Wire](https://github.com/google/wire) builds the graph at compile time:
  `Build` on PostgreSQL, `BuildInMemory` on the memory adapter, the same use cases and transport
  in both. `go generate ./internal/app` rewrites `wire_gen.go`; CI fails when it is stale.

The page (`web/`, React + TypeScript + Vite) follows the same idea: `domain/` for the types and
labels, `api/` for the `CragbookApi` port and its HTTP adapter, components that get the API from
a context — the tests hand them an in-memory one.

## Tests

```bash
go test ./...                                 # domain, use cases, adapters, HTTP, architecture
CRAGBOOK_TEST_DATABASE_URL=postgres://cragbook:cragbook@localhost:5432/cragbook?sslmode=disable go test ./...
cd web && pnpm typecheck && pnpm coverage
cd web && pnpm exec playwright install chromium && pnpm e2e   # the page end to end
```

`pnpm e2e` builds the page and the binary and drives them with Playwright, on a phone and a
desktop: the binary in memory mode imports made-up crags from a fake cragcast (`web/e2e/cragcast`),
and axe checks every view.

CI runs the Go tests with `-race` against PostgreSQL 17, the page's tests with coverage floors,
the end-to-end tests, and builds the image and checks it runs as non-root with no shell.

## Releases

SemVer, one tag per release on a commit of main:

```bash
git tag v0.2.0 && git push origin v0.2.0
```

`release.yml` builds the image for amd64 and arm64 with the version inside, pushes it to GHCR as
`0.2.0`, `0.2` and `latest`, and only then publishes the GitHub Release, its notes generated from
the merged PRs. A tag that is not `vMAJOR.MINOR.PATCH` or not on main is refused.

## Roadmap

Planned work is in [BACKLOG.md](BACKLOG.md), with stable `CB-n` ids; [ROADMAP.md](ROADMAP.md)
is generated from it by [backlogsync](https://github.com/Allan-Nava/backlogsync), and the
GitHub issues and milestones follow it on every push to main.

## Data

Crags and routes © [OpenStreetMap contributors](https://www.openstreetmap.org/copyright),
ODbL 1.0, through cragcast's API. cragbook copies them and never edits them; a fix belongs on
OpenStreetMap, where cragcast's monthly refresh picks it up.

## Licence

[MIT](LICENSE).
