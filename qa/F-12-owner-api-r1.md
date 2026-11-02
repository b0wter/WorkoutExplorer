# QA F-12 Owner API - round 1

- **Date:** 2026-10-23
- **Verdict:** not ready. One Major finding: an anonymous client with no token can make the server write ERROR lines and answer `500` at will, just by dropping the connection early (QA-12-r1-1). ADR-025 item 8 and O-13 forbid exactly this. Everything else holds: the not-found response is byte-identical in 102 refusal variants plus expired rows, AC 6 holds on the Go side, and concurrent deletes release storage exactly once.
- **Inputs:** `features/F-12-owner-api.md` (AC 1 to 7, updated 23 Oct 2026); `architect/F-12-owner-api.md` 1.0 (O-1 to O-13); ADR-025, ADR-019, ADR-020 rev. 1, ADR-021, ADR-022; the decision log entry "Owner API" of 23 Oct 2026 in `features/README.md`; TD Publish and view flow 4 and 5, Code structure (Lifecycle row), Security and privacy, Glossary. Code at commit `2ade9db` (clean working tree): `src/server/internal/{lifecycle,httpapi,store,privatedir}`, `src/server/cmd/workout-explorer`.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. `go vet` clean; `go test ./...`: all 12 packages `ok`; svelte-check 0 errors, 0 warnings; size: activity entry 45,328 / 60,000 gzip, upload entry 2,272 / 8,000 gzip, both PASS (F-12 changes no front-end code). The only warning is npm's `EBADENGINE` (Node 22 locally, Node 24 required), which F-12 did not cause |
| `go test -race -count=1 ./internal/lifecycle/ ./internal/httpapi/ ./internal/store/ ./internal/privatedir/ ./cmd/workout-explorer/` | all `ok` (lifecycle 3.1 s, httpapi 14.9 s, store 1.2 s, privatedir 1.0 s, cmd 6.0 s) |
| `go test -race -count=20 -run 'Owner\|Delete\|Update\|Open' ./internal/lifecycle/` | `ok` |
| Scratch copy of the module plus a QA test (`qa_race_test.go`, not committed): 30 rounds × 24 concurrent `DELETE`/`PATCH`/`GET original.fit` against one workout through `newApp` (real SQLite and `privatedir`), `-race` | `ok`: exactly one `204` per round, every other answer `200` or `404`, never `500`; `Storage.Used()` back at the start value every round; no row and no `private/` entry left |
| Live server (`workout-explorer serve` on a scratch data folder, synthetic `ride-30min.fit` only): raw-socket probe of the not-found response, path probe for AC 6, delete during a download, slow downloads, `PATCH` body edges, aborted anonymous requests | results in the findings and in Coverage |

## Findings

### Major

- **ID:** QA-12-r1-1
- **Severity / kind:** Major / defect
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/owner.go:160-163` (`lookup` passes the request context to `WorkoutByPublicID`, and `ownerFail` logs every error from it); the same pattern is in `owner.go:245-249` (`WorkoutExists` with the request context)
- **Relates to:** ADR-025 item 8 ("Refusals are not logged. Logging them would … add a line that an attacker can provoke") and item 6 (no rate limit, which is safe only because a refusal costs one lookup and one hash); F-12 design O-13 (no line for refusals); TD Conventions (Logging)
- **Evidence:** live server, scratch data folder. `probe_abort.py` opens a connection and sends `GET`, `PATCH` or `DELETE /api/workouts/zzzzzzzzzzzzzzzzzzzzzz` (an unknown ID, **no `Authorization` header**), then closes the socket at once. net/http starts its background read, sees EOF and cancels the request context. If that happens before `lookup` runs the query, `database/sql` returns `context canceled`, and `lookup` logs it as a server failure. Out of 500 requests per method, the log got: `GET` 270, `DELETE` 273, `PATCH` 246 lines of `{"level":"ERROR","msg":"owner request failed","stage":"lookup","error":"store: context canceled"}`. Each of those requests is also answered `500`, though nobody reads the answer. The upload handler already treats this case as "client gone" without a `500` (`upload.go:205`), so the owner path is the odd one out. Existing tests miss it: `TestOwnerLogsHaveNoSecrets` checks that no secret appears in the log, not that a refusal writes no line, and every test uses `context.Background()`.
- **Consequence:** with no token, no rate limit and nothing but TCP connects, anyone can make the server write ERROR lines without limit. That fills the journal and sets off false alarms in any ERROR-based monitoring (F-31). It breaks the "refusals are silent" property that ADR-025 gives as the reason not to rate-limit. Ops only, no privacy leak: the line carries no ID, token or request data.
- **What would close it:** a cancelled or expired request context, before the token check passes, gives a result that is not logged (for example, `ErrNotFound` or a distinct unlogged error), or `lookup` runs on `context.WithoutCancel(ctx)` (it is one indexed query). The same goes for the `WorkoutExists` re-check in `open`. Add a test where a lookup with a cancelled context writes no log line.

### Minor

- **ID:** QA-12-r1-2
- **Severity / kind:** Minor / risk
- **Route:** backend (architect if O-7 is to be reworded)
- **Where:** `src/server/internal/lifecycle/owner.go:260-266` (`DeleteByOwner` takes `s.mu` and then checks the token), compared with `src/server/internal/httpapi/owner.go:194-198` (`PATCH` checks the token before `Update` takes the lock)
- **Relates to:** O-7, ADR-025 item 6; the engineer's choice "a plain mutex that ignores cancellation"
- **Evidence:** reading the code. An anonymous `DELETE` takes the one service-wide write lock, waits for it without regard to cancellation, and only then gets `404`. The `PATCH` path avoids this with its pre-check. The probe for r1-1 showed that hundreds of anonymous `DELETE`s do queue on the lock.
- **Consequence:** harmless today, because the lock is held for microseconds. Once F-15's hook runs under the same lock (brotli, a page render), anonymous `DELETE` requests queue behind it with no rate limit and cannot be cancelled. Each keeps a goroutine, and the owner's own saves wait behind them. Fix: the same token pre-check as `PATCH` (the token is checked again under the lock anyway). I accept the plain mutex itself once only token holders can reach it.

- **ID:** QA-12-r1-3
- **Severity / kind:** Minor / risk
- **Route:** architect (hand-off to F-15)
- **Where:** `src/server/internal/lifecycle/owner.go:179-186` (`ownerFail` logs `err.Error()` verbatim), called for `hook` (`:219`) and `delete` (`:268`)
- **Relates to:** O-13 ("a line never contains the `public_id`"), O-11, F-15 hand-off (extend `Delete` with `public/{public_id}`)
- **Evidence:** reading the code. Today the error texts hold only the internal ID (`private/{id}` paths). F-15 will add `public/{public_id}` removals to `Delete` and to the hook. An `os.PathError` from those carries the path, so the `public_id` would reach an ERROR line through `ownerFail`.
- **Consequence:** a privacy rule broken by a later feature without any F-12 code changing. The fix belongs in the F-15 hand-off now: either F-15 errors must not contain public paths, or `ownerFail` logs the stage plus a reduced error for the `hook` and `delete` stages.

- **ID:** QA-12-r1-4
- **Severity / kind:** Minor / defect (documentation level)
- **Route:** architect
- **Where:** `src/server/internal/lifecycle/owner.go:136-139, 164-167`; ADR-025 item 4
- **Relates to:** ADR-025 item 4 ("a hash is always computed (of a fixed placeholder when the token is absent)")
- **Evidence:** the placeholder and dummy hashes are computed once when the package loads. For an absent or malformed token, `lookup` computes no SHA-256 at all. This branch depends only on the presented token, never on whether the row exists, so it reveals nothing about a workout. The code is fine; the ADR text no longer matches it.
- **Consequence:** a later reader checks the code against item 4 and finds a deviation that nobody recorded. Reword item 4 to "the comparison always runs, against a precomputed placeholder for an absent or malformed token and a dummy hash for a missing row".

- **ID:** QA-12-r1-5
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/internal/httpapi/owner.go:220-257` (`parsePatch` decodes into `map[string]json.RawMessage`)
- **Relates to:** F-12 design, Settings object ("A `PATCH` body with … a `null` … gets `400`"); the engineer's choice "the last duplicate JSON key wins"
- **Evidence:** live server. `{"title":null,"title":"dup-null-then-string"}` → `200` and the title is stored. `{"title":"a","title":null}` → `400`.
- **Consequence:** the `null` rule depends on key order. No harm, because the stored value is always valid, but the design's rule is not what the code does. Either refuse duplicate keys (a few lines with `json.Decoder.Token`), or write "last duplicate wins" into the design. My preference is to refuse: it costs little and keeps "one JSON object" exact.

- **ID:** QA-12-r1-6
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/owner_test.go:75`
- **Relates to:** AC 1, ADR-025 item 5 (status, body **and headers** identical; never `WWW-Authenticate`)
- **Evidence:** the end-to-end AC 1 test compares status, body and `Content-Type`, but not `Cache-Control` and not the absence of other headers. My raw-socket probe compared the full header set (minus `Date`) for 102 cases and found it identical, so behaviour is correct today. The gap is in the test.
- **Consequence:** a later edit that adds a header on one refusal path (for example, `WWW-Authenticate` on a missing token) would pass CI. Compare the whole header map, not one field.

## Engineer's listed choices

| Choice | Verdict |
| --- | --- |
| Double token check on `PATCH` | **Accept.** It keeps strangers out of the body rules and out of the O-7 lock, at the cost of one extra indexed lookup. Apply the same to `DELETE` (r1-2) |
| 30 s read and write deadlines, 10 min for downloads | **Accept.** 10 min covers 20 MB at about 33 kB/s. Probed: a 55 KB download throttled to 1.4 kB/s (39 s, past the 30 s read deadline) completes with `200` and every byte. Go 1.26 resets the write deadline after each request (`net/http/server.go:2095`), so a deadline does not carry over to the next keep-alive request. F-22/F-31 must set Caddy's proxy timeouts at or above 10 min (already in the hand-off) |
| 32 KiB `PATCH` cap answering `400`, not `413` | **Accept.** The design groups every malformed body under `400 invalid_settings`, and F-19 never sends such a body. `413` would be more exact but adds an error code that F-19 must handle |
| Control characters refused in the title, and in the description except line feed | **Accept for the title. Question for the description** (Q1): a tab pasted from notes or a spreadsheet is refused with an error, so F-19 must strip or block it. CR is not a problem, because `textarea.value` normalises to LF. Format characters such as U+202E (right-to-left override) are accepted (probed), which is within the rule as written |
| JSON escaping of `<` | **Accept.** It is still plain text after `JSON.parse` (AC 3), and it is an extra safeguard if a body is ever shown as HTML |
| Last duplicate JSON key wins | **Challenge** (r1-5) |
| A plain mutex that ignores cancellation | **Accept**, provided only token holders reach it (r1-2). Requests waiting for the lock lose nothing by not being cancellable, because writes run with `WithoutCancel` anyway |
| A hook failure answers `500` and the change is kept | **Accept.** This is O-11 as decided: a save is safe to retry, and F-15 must make regeneration idempotent and fail toward less public content (hand-off). F-19 should word the `500` so that "saved, but the page may not be updated yet" is not misread as "not saved", which is a frontend matter for F-19 |

## Previous findings

None for F-12 (first round). QA-10-r1-1 (two concurrent deletes released bytes twice) is fixed in `lifecycle.Delete` (only the call that removed the row releases), and it is confirmed under owner concurrency by the 30-round `-race` stress run above.

## Questions

- **Q1 (architect, product if needed):** Should the description allow tab (U+0009)? The answer that settles it is either "yes, allow tab", which changes one line in `ApplyPatch` plus a test, or "no", with F-19 told to replace tabs before sending.

## Coverage

| AC | Code | Test | Verified |
| --- | --- | --- | --- |
| 1 Not found without token, wrong token, token in query, any query with valid header | `httpapi/owner.go:89-114`, `lifecycle/owner.go:156-177` | `TestOwnerNotFound` (e2e, 9 cases × 5 endpoints), `TestOwnerNotFoundCases`, `TestOwnerOperationsAllCheckTheToken`, `TestOwnerQueryStringIsNotFound`, `TestBearer` | Live raw-socket probe: 102 variants (no header, wrong token, another workout's token, 23 characters, a symbol, `Basic`, two headers, two spaces, token in query, `?` and `?x` with a valid header, internal ID, unknown ID, short ID, percent-encoded ID, encoded slash) across `GET`, `GET draft.bin`, `GET original.fit`, `PATCH`, `DELETE`, `HEAD`. Status line, every header except `Date`, and body are identical; never `401`/`403`/`WWW-Authenticate`. An expired row is identical too. A trailing space after the token is accepted, which is correct: HTTP strips it as OWS (RFC 9110 §5.5) |
| 2 Lifetime 31 days or past refused, 30 accepted | `ApplyPatch` `owner.go:95-105` | `TestOwnerLifetime` (e2e 31/0/−3/30/1), `TestApplyPatch`, `TestApplyPatchLifetimeInThePast` (injected clock) | traced; non-integers `30.0`, `"30"`, `1e1` and an overflowing number give `400` (probed) |
| 3 `<script>` title stored and returned as plain text | `ApplyPatch`, `settingsOf` | `TestOwnerPlainText` (response, `GET`, row) | probed |
| 4 Original byte-identical to stored file; `workout-YYYY-MM-DD.fit` / `workout.fit` | `open`, `DownloadName`, `privatedir.Read`, `httpapi` file handler | `TestOwnerFiles`, `TestDownloadName`, `TestRead`, `TestOwnerResponses` | live: the download equals `private/{id}/original.fit` (`cmp`); `Content-Disposition: attachment; filename="workout-2026-06-23.fit"` (local date from the time zone offset) |
| 5 After delete: row and `private/{id}/` gone, every endpoint not found, second delete not found | `DeleteByOwner` → `Delete` | `TestOwnerDelete` (e2e, `dataHolders` list, storage total), `TestDeleteByOwner`, `TestOwnerDeleteReleasesStorage`, `TestOwnerWritesAreSerialised` | live: `DELETE` `204`, second `404`, `GET` `404`, folder gone; a download already running when the delete landed completed byte-identical. QA stress: 30 rounds × 24 concurrent writers and readers, released exactly once |
| 6 No Go URL serves `original.fit`/`draft.bin` without the valid token header | mux has only `/api/*` routes; `privatedir.Read` accepts only valid IDs and the two names | `TestNoPublicURLServesPrivateFiles` | live, without a token: `/private/{id}/…`, `/data/private/…`, `/original.fit`, `/w/{pub}/original.fit`, internal ID as handle, `?token=`, `%6Friginal.fit`, `original%2Efit`, `..%2f` traversal, `//`, `/API/`, `%00`, `;x`: every answer is `404`, or a `307` from the mux's path cleaning that points to the token-checked route. No `2xx` |
| 7 Title 100 / description 1,000 code points accepted, 101 / 1,001 refused | `ApplyPatch` (`utf8.RuneCountInString` after `TrimSpace`) | `TestOwnerLengths` (ASCII, `ä`, `🚴`; largest escaped body fits the cap), `TestApplyPatch` | traced; leading and trailing ideographic spaces (U+3000) are trimmed (probed) |

Also checked and found correct: the constant-time compare against a dummy hash when there is no row (ADR-025 item 4); expiry to the second; a draft refuses any `visibility`, and published workouts switch `unlisted` ↔ `private` with `draft` refused (O-6); the hook is called for published workouts only, also with no change; `UpdateWorkout` in one immediate transaction (`_txlock=immediate`, one connection); the file responses are streamed with `Content-Length`, `no-store` and `nosniff`; a file missing after the token check gives `404` when the row is gone and `500` otherwise; no public ID or token in the live log (grep of the full log against all four public IDs and tokens: 0 hits); `privatedir` stays the only code that builds private paths; TD Lifecycle row and Glossary updated as the hand-off asks.

Not run: `make corpus` (F-12 does not touch parsing); fuzzing of `parsePatch` (the input is bounded at 32 KiB and decoded by `encoding/json`; the probed edges were enough at this risk); anything behind Caddy (F-22, F-31). No real corpus file was used; every probe used the synthetic `ride-30min.fit`.
