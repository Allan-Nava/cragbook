# Backlog — cragbook

Single source of truth for planned work. Items keep a stable `CB-n` id; ROADMAP.md is
generated from this file (`npx backlogsync roadmap`) and the GitHub issues and milestones
follow on push to main.

```
## v0.2.0 — Title <!-- ms: phase=next -->

- [ ] **CB-99 — Short name**: what it is, why, what it touches. <!-- cb: prio=high size=M labels=backend -->
```

## v0.1.0 — A diary on cragcast's routes <!-- ms: phase=now -->

Built on 2026-10-23: the crags and routes from cragcast's API v1, the diary on top, in DDD
layers wired by Google Wire.

- [x] **CB-1 — The domain**: `climbing` (crags, routes, the French `Grade` value object with its
  rank and level, `SortRoutes`, the catalogue's ports) and `diary` (`Ascent` built only through
  `NewAscent`, which refuses a day on a route not climbed, a day after today and notes over
  2,000 characters; the diary's order; the repository port). Standard library only. <!-- cb: prio=high size=M labels=domain ver=main -->
- [x] **CB-2 — The use cases**: `catalogue` (crags and their routes with their diary entries, a
  gone route only while it has one), `journal` (record, clear, list), `importing` (a few crags at
  a time, one run at a time, a crag that fails left as it was). <!-- cb: prio=high size=M labels=backend ver=main -->
- [x] **CB-3 — Storage, twice**: PostgreSQL (pgx, embedded migrations under an advisory lock,
  the diary's rules repeated as CHECKs) and memory, both passing the same contract tests. <!-- cb: prio=high size=M labels=backend ver=main -->
- [x] **CB-4 — The cragcast adapter**: the API v1 client with timeouts and the caller's
  context, translating crags, routes and sector names into the domain; a file for another crag
  refused. On 2026-10-23: 220 crags and 116 routes in 7 s. <!-- cb: prio=high size=S labels=backend ver=main -->
- [x] **CB-5 — The HTTP API**: crags, a crag with its routes, the diary's PUT, DELETE and list,
  `POST /api/import`; the domain's errors mapped to 400/404/409/502 in one place. <!-- cb: prio=high size=M labels=backend ver=main -->
- [x] **CB-6 — Google Wire**: the graph built at compile time, `Build` on PostgreSQL and
  `BuildInMemory` (`CRAGBOOK_STORE=memory`) sharing use cases and transport; wire pinned as a
  `go tool`, CI failing on a stale `wire_gen.go`. <!-- cb: prio=high size=S labels=backend ver=main -->
- [x] **CB-7 — The page**: crags with search, a crag's routes grouped and filtered by level,
  each marked done (with its day), project or to try with a note; the diary. React +
  TypeScript + Vite, the API behind a `CragbookApi` port. <!-- cb: prio=high size=M labels=frontend ver=main -->
- [x] **CB-8 — Docker and CI**: Compose for PostgreSQL 17, a scratch image (non-root, no shell)
  serving the page; CI with gofmt, vet, the Wire check, `go test -race` against PostgreSQL, the
  page's coverage floors and build, the image run. <!-- cb: prio=high size=M labels=infra ver=main -->
- [x] **CB-9 — The architecture test**: `internal/archtest` fails an import that points
  outwards, and any non-standard library in the domain. <!-- cb: prio=med size=S labels=backend ver=main -->

## v0.2.0 — Ready for others to run <!-- ms: phase=next -->

What someone needs to run cragbook without reading its code, and to trust it while it runs.

- [x] **CB-10 — Releases and an image**: tagged releases, the image on GHCR for amd64 and arm64
  (a Raspberry Pi runs it), the version in the binary and on `/healthz`. Done on 2026-10-23:
  `release.yml` on a `vX.Y.Z` tag of main pushes `ghcr.io/allan-nava/cragbook` as X.Y.Z, X.Y
  and latest (cross-compiled, no QEMU), then the Release with generated notes; `cragbook
  version` and `/healthz` report the version; CI builds arm64 on every PR. <!-- cb: prio=high size=S labels=infra ver=main -->
- [x] **CB-11 — The API described**: an OpenAPI 3.1 document of `/api/`, served at
  `/api/openapi.json`, and a test that every route the mux serves is in it and every example in
  it is what the handlers answer. Done on 2026-10-28: `openapi.json` embedded in `httpapi` and
  served with the build's version; the mux's routes a table the test compares both ways; each
  response example replayed through its `x-request` and checked against its schema, a field
  the schema does not name failing it. <!-- cb: prio=med size=M labels=backend,docs ver=main -->
- [x] **CB-12 — End to end**: Playwright against the binary in memory mode — mark a route,
  reload, find it in the diary — with axe on every view, on phone and desktop. Done on
  2026-10-28: `pnpm e2e` builds the page and the binary, which imports made-up crags from a fake
  cragcast (`web/e2e/cragcast`); a route marked, reloaded and found in the diary, one taken out,
  axe on eight views with no violation, on Desktop Chrome and a Pixel 7; a CI job, its report
  kept on failure. <!-- cb: prio=med size=M labels=frontend,infra ver=main -->
- [x] **CB-13 — Linters**: golangci-lint (errcheck, staticcheck, revive) in CI, fixing what it
  finds; the unchecked `Rollback` and `Write` errors are known cases. Done on 2026-10-28:
  golangci-lint v2.12.2, its standard linters and revive, in CI after vet. `Rollback` now joins
  a failed rollback to the error returned; the tests' setups fail on an error instead of
  ignoring it. revive's `exported` and `unused-parameter` are off, with the reason in
  `.golangci.yml`. <!-- cb: prio=med size=S labels=infra,backend ver=main -->
- [ ] **CB-14 — Seen while it runs**: request logging middleware (method, path, status,
  duration) and Prometheus metrics — requests by route and status, the last import's time,
  crags and failures — on `/metrics`. <!-- cb: prio=med size=M labels=backend -->
- [ ] **CB-15 — The import on its own**: `serve` refreshes daily at a configured hour, after
  cragcast's own monthly refresh has landed, through the same `importing.Service`. <!-- cb: prio=low size=S labels=backend -->
- [x] **CB-19 — Logo**: a mark of cragcast's family — its crag on near-black, the tick of a
  route climbed in place of the sun, an open book for the diary — in the README, as the page's
  favicon and header, and a 1280×640 social preview. Done on 2026-10-23, one copy in
  `assets/logo.svg`. <!-- cb: prio=low size=S labels=docs,frontend ver=main -->

## v0.3.0 — A better diary <!-- ms: phase=later -->

- [ ] **CB-16 — How it was climbed**: an ascent's style — a vista, flash, lavorata — as a value
  object in `diary`, only for a route climbed; shown and filtered in the diary. <!-- cb: prio=med size=M labels=domain,frontend -->
- [ ] **CB-17 — The grade pyramid**: routes climbed by grade, as a chart and a table, from the
  diary; the hardest climbed per level. <!-- cb: prio=low size=M labels=frontend,backend -->
- [ ] **CB-18 — The diary exported**: the diary as CSV and back, so it outlives a database and
  moves between installs; an import refuses rows the domain would. <!-- cb: prio=low size=S labels=backend -->
