# CLAUDE.md

Guidance for Claude Code in this repository. README.md describes what cragbook is and how to run it.

## Rules

- **SOLID and DDD, layers pointing inwards.** domain ← application ← infrastructure and
  transport ← app. The domain imports the standard library only; rules live in it (a
  constructor that refuses invalid state), never in a handler. A port is an interface owned by
  its consumer; adapters implement it. `internal/archtest` enforces the imports — extend its
  table when a layer is added, never loosen it to make a change pass.
- **Wire builds the graph.** A new dependency is a provider in `internal/app/providers.go` and a
  line in `wire.go` (a `wire.Bind` sits in the set that provides its concrete type); then
  `go generate ./internal/app`. Never edit `wire_gen.go` by hand; CI regenerates it and fails on a
  difference.
- **Both storage adapters keep one contract.** A change to a port's behaviour goes into
  `internal/infrastructure/contract` first, and both `memory` and `postgres` pass it.
- **The import never touches the diary**, and a crag whose routes cannot be read is not saved
  (saving it without routes would mark them gone). Routes are never deleted, only marked gone.
- **The API is described in `internal/transport/httpapi/openapi.json`**, embedded and served at
  `/api/openapi.json`. An endpoint, a field or a status changed in the handlers is changed there
  in the same commit; each response example carries the `x-request` its test replays.
- **The database checks what the domain checks** (status, a day only when climbed, notes'
  length): a migration that adds a rule to the domain adds its CHECK too.
- **Data is cragcast's, from OpenStreetMap (ODbL).** Credit it where shown; never edit it here.
  Test data is made up, never real crags.
- **The web app**: type-check with `pnpm typecheck` (`tsc --noEmit`); components reach the API
  only through `useApi()`, so tests pass a fake. Coverage floors in `vite.config.ts` only go up.
  `pnpm e2e` (Playwright, `web/e2e`) runs the binary against a fake cragcast of made-up crags; a
  view added gets its axe check there, and axe's findings are fixed, never disabled.
- **The logo is `assets/logo.svg`, one copy**: the README, the page's favicon and header (the
  Vite plugin in `web/vite.config.ts`) and the image all read it. `social-preview.png` is rendered
  from `social-preview.html`; change both together. cragcast's family: its palette and crag.
- **Releases are SemVer tags on main** (`vX.Y.Z`); `release.yml` pushes the image before it
  publishes the Release. A milestone closed is a minor release. The version is linked in
  (`-X main.version`), never written in the code.
- **BACKLOG.md is the plan.** Work is a `CB-n` item there; ROADMAP.md is generated
  (`npx backlogsync roadmap`, CI checks it) and the issues follow on push to main — never edit
  either by hand. An item done is ticked, with what was done and when, in the same PR as the work;
  each part of an item is its own commit.
- Commits: `Allan Nava <allannava95@gmail.com>`, no Claude co-author or attribution lines.
