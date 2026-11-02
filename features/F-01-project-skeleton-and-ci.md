# F-01 Project skeleton and CI

- **Phase:** 0 Prototype
- **Owner:** both
- **Requirements:** NFR Performance (page budgets); Technical design: Tech stack, Front end (Budget), Hosting and operations (Local development)
- **Depends on:** none

## Goal
A repository both engineers can build on: a Go server, a Svelte front end and a CI pipeline that builds, tests and enforces the bundle budget from day one.

## Scope
- Go module using the standard library `net/http`, with a health endpoint (used later by monitoring, F-31).
- Svelte 5 + Vite + TypeScript app (no SvelteKit) with uPlot as the only chart dependency.
- Local development as in the technical design: `go run` for the server, `vite dev` with a proxy to `/api/*`.
- Make targets for build, test and a placeholder `make corpus` (filled in F-02).
- CI on GitHub Actions: builds the Go binary and the front-end bundle, runs Go and front-end tests, and runs a bundle size check against the 60 KB compressed budget (Svelte + uPlot + app code).
- Directory layout as decided in [ADR-001](../architect/ADR-001-repository-layout-and-build-entry-point.md), described in a short top-level README.

## Out of scope
- FIT parsing (F-02), data file (F-03, F-04), any UI beyond a placeholder page.
- Deploy to a server (F-31).

## Acceptance criteria
- [ ] `go run` plus `vite dev` serve a placeholder page that calls the health endpoint through the dev proxy and shows its result.
- [ ] CI runs on `v*` tag pushes and on manual runs (`workflow_dispatch`), not on every push, and fails when a Go or front-end test fails; local `make ci` is the gate before each commit (decided 2 Nov 2026).
- [ ] CI fails when the compressed app bundle exceeds 60 KB (verified by temporarily adding a large dependency) and passes again when it is removed.
- [ ] The bundle size is printed in the CI log on every run.
