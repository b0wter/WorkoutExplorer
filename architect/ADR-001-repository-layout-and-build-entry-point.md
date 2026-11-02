# ADR-001 Repository layout and single build entry point

- **Status:** accepted
- **Date:** 10 Oct 2026
- **History:** [architect/history/ADR-001-repository-layout-and-build-entry-point.md](history/ADR-001-repository-layout-and-build-entry-point.md)
- **Features:** F-01; affects every later feature
- **Supersedes:** none
- **Requirements:** TD Tech stack, TD Hosting and operations (Deploys, Local development), TD Front end (Budget)

## Context

The repo holds a Go server and a Svelte front end, built by two engineers, checked by QA, and later deployed by a script (F-31). CLAUDE.md puts all code under `src/` and makes the architect responsible for its layout; the F-01 feature file defers the layout to this ADR. The repo will be hosted on GitHub, so CI runs on GitHub Actions (user decision, 10 Oct 2026). We want builds to be the same on a laptop and in CI, and changing CI host to be cheap.

## Decision

1. **Two top-level code roots under `src/`:** `src/server/` is a self-contained Go module (module path `workout-explorer`), `src/web/` is a self-contained npm package. Neither reaches into the other's folder. Shared contracts (data file, API, page view model) are documented in `architect/` and tested on each side, not shared as code.
2. **Go layout:** `cmd/workout-explorer/` for the one binary, everything else under `internal/` by bounded context (see TD Code structure, Bounded contexts and modules). No `pkg/`.
3. **Root `Makefile` is the only build entry point.** Every build, test, check and dev command is a make target. CI does nothing except install the pinned toolchains and run `make -k ci`.
4. **CI host is GitHub Actions; its config is a thin wrapper**, the single file `.github/workflows/ci.yml` (checkout, set up Go from `go.mod`, set up Node from `.nvmrc`, run `make -k ci`; exact spec in `F-01-project-skeleton-and-ci.md`, section CI). No build logic, matrices or scripts in it.
5. **All build output goes to the root `build/`** (git-ignored): `build/workout-explorer` and `build/web/`. Deploy (F-31) consumes only this folder.
6. **The architect decides the layout**; engineers describe it in the root `README.md`. The F-01 feature file now says the same (OQ-2, closed).

## Consequences

- Moving to another CI host means rewriting about 15 lines of wrapper; the build itself does not change.
- Engineers and QA run exactly what CI runs (`make ci`), so "works on my machine" differences show up locally.
- Two separate dependency trees (Go modules, npm). Accepted: they share nothing at runtime.
- Make must be available locally (standard on Linux and macOS; Windows users need WSL). Accepted for a two-person project.
- Placing the Makefile, README and CI wrapper at the root rather than under `src/` is a deliberate exception to "all code under `src/`": they orchestrate, they are not application code.

## Would choose differently if

- The front end ever needed to be built by Go (e.g. `go:embed` of the bundle). Today Caddy serves the bundle, so Go never needs it.
- More than two languages or many packages appeared; then a task runner with caching might pay off. Not expected.
