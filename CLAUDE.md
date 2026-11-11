# Workout Explorer

A small, free viewer for FIT files: upload one, get a shareable page with linked map, charts and laps. For the operator and their friends; no accounts, no external services, store as little as possible.

Keep answers to the user concise unless the user asks for detail or an explanation. Present open decisions as a numbered list with a suggestion each; the user answers by number.

## Where things live

| Folder | Contents | Written by |
| --- | --- | --- |
| `spec/` | Product specification and technical design: the source of truth | user, senior-requirements-engineer |
| `features/` | One file per feature (`F-NN-*.md`) plus `README.md` index, coverage and decisions | senior-requirements-engineer (the Status and Waiting on columns of the index: main session) |
| `architect/` | Architecture documents and ADRs per feature, plus `INDEX.md` | senior-architect |
| `src/` | All code (Go server and Svelte front end); layout decided by the architect | senior-backend-engineer, senior-frontend-engineer |
| `qa/` | QA reports per feature and round, plus `INDEX.md` | senior-qa-engineer |
| `.claude/agents/` | Project agents | — |

## Delivery workflow

The main session orchestrates: it picks the lane, starts each agent with the right files, routes findings, reviews diffs, commits, and keeps the Status and Waiting on columns in `features/README.md` current. It does not write architecture, code or QA reports itself. Features go one at a time, in the build order of `features/README.md`.

### Pick a lane by risk

| Lane | Use for | Steps |
| --- | --- | --- |
| **Small** | Settings, limits, wording, comments, small bug fixes with an obvious fix, minor QA follow-ups | One engineer makes the change, adds a one-line note to the affected ADR or design doc if a recorded decision changes, and runs `make ci`. The main session reviews the diff and commits. No architect, no QA round. |
| **Normal** | A new feature | 1. `senior-architect` writes a short design (decisions and constraints, about one to two pages). 2. The engineer(s) build it. 3. One `senior-qa-engineer` pass against the acceptance criteria. |
| **Critical** | Privacy (UP-9, coordinates, tokens), data loss, security | The normal lane, plus whatever extra checking the risk needs (fuzzing, sweeps, mutants), and QA at effort high. |

When unsure, take the lower lane and move up if the work turns out riskier.

### Roles

1. **Architecture** - `senior-architect` (global agent) writes decisions and constraints to `architect/`: module boundaries, data formats, trade-offs, things a later reader might question. It does **not** write test names, helper signatures, value lists or step-by-step work orders; that is the engineer's job. Effort: high for a feature's first design, medium for revisions.
2. **Implementation** - the engineer named by the feature's **Owner** (`senior-backend-engineer`, `senior-frontend-engineer`, both for `both`, backend first when the frontend depends on it). The architecture docs set the decisions; on details the engineer decides, and lists every deviation and open choice in the report instead of stopping. The engineer stops only when a decision itself is wrong or missing and the choice matters (privacy, data format, public behaviour). For a backend-only feature with a clear spec and no real design question, one engineer at model Opus may replace the architect step.
3. **QA** - `senior-qa-engineer` checks the acceptance criteria, runs the build and tests, and writes a report to `qa/` with a row in `qa/INDEX.md`. Effort medium, high in the critical lane. A follow-up round reviews only the diff and the routed findings.
4. **Decision**
   - **Ready** means no Blocker or Major finding open. Minor findings stay listed in the QA report and the Waiting on column, and are fixed in the small lane, next time the feature is touched or in a batch.
   - Otherwise route each Blocker or Major finding to its **Route** and run a follow-up QA round.

Escalate to the user instead of looping when a finding needs a product decision (the spec is silent or contradictory), or when the same finding survives two fix rounds.

### Documents: one home per fact

- **Product spec:** what the app does (requirement IDs). **Technical design:** the system-wide technical baseline. **Feature file:** scope and acceptance criteria. **Architecture doc:** only the feature's decisions that the code and tests don't make obvious. **ADR:** only for a decision someone might later question.
- Don't copy a fact into a second document; link to it. Clean up existing duplication when a document is touched anyway.
- Product decisions go in the decision log of `features/README.md`. `architect/INDEX.md` lists documents and open technical questions; it does not repeat resolved product decisions.
- Live documents hold the current state only. Round lists, superseded decisions and change history go in `architect/history/<doc-name>.md`, which agents don't read unless they need the history.

## Naming

- Architecture: `architect/F-NN-<slug>.md` for a feature's design, `architect/ADR-NNN-<slug>.md` for decisions. The system-wide picture (invariants, code structure, conventions, glossary) lives in the technical design, `spec/Workout Explorer — Technical Design.md`.
- QA: `qa/F-NN-<slug>-rN.md`, where N is the review round (r1, r2, ...).
- Requirement IDs (UP-, AP-, SH-, PIN-, AC-, DS-, INF-) come from the product spec; cite them in every document.
