# QA F-36 Preview image with map context - round 2

- **Date:** 2026-11-09
- **Verdict:** ready. QA-36-r1-1 (Major) is fixed. A GET now takes a frame slot shared by all workouts before the original is read, or it gets `503`. The front end treats that `503` as a fallback. I found no new Blocker or Major. Two Minor findings remain open: one is documentation and one is a design note.
- **Inputs:** `qa/F-36-preview-image-with-map-context-r1.md`; the diff of 59b977d against f1a9c34: `lifecycle/previewmap.go`, `lifecycle/lifecycle.go`, `httpapi/owner.go`, three test files, the MC-4 row of `architect/F-36-preview-image-with-map-context.md`, the F-36 note in ADR-017, and `features/README.md`. I also read the front-end GET path, which did not change: `src/web/src/edit/basemap-job.ts` and `lib/owner/index.ts` `send`. The working tree was clean.

## Build and test results
- **First `make ci`:** exit 2, after 2 min 6 s. Go: every package passed. Vitest: 61 files, 1,578 tests passed. Size checks passed: `edit.html` 57,746 / 60,000, `upload.html` 3,761 / 8,000. The run failed in `caddy-check`: `TestCaddyForwardedFor/trusted_peer:_client-prepended_entries_dropped` reported "Go received [], want exactly ["9.9.9.9"]". F-36 does not touch this test. It last changed in 277043d. In isolation it passed 6 of 6 times.
- **Second `make ci`:** exit 0, after 3 min 18 s, with the same counts and sizes.
- **Race run:** `go test -race -count=3 -run 'PreviewMap|PreviewFrame|RenderOnMap|CheckMap|Frame|MapCard'` on lifecycle, preview, httpapi, publish and privatedir. All passed.
- **Mutants:** run on a scratch copy of HEAD, against the package's tests. All 6 killed:

| Mutant | Killed by |
| --- | --- |
| frame slot CAS removed | `TestPreviewFrameBusy` (it deadlocks on the gate, so the run hits the timeout) |
| slot never released | `TestPreviewFrameBusy` ("slot not freed") and `TestPreviewFrame` |
| slot released before the build | timeout, as above |
| re-lookup under the lock removed (r1's M5) | `TestPutPreviewMapRelookup` |
| attribution text not drawn (r1's M10) | `TestRenderOnMap` |
| GET `ErrMapBusy` → 503 mapping removed | `TestPreviewMapEndpoints` |

## Findings

### Minor
- **ID:** QA-36-r2-1
- **Severity / kind:** Minor / defect (documentation)
- **Route:** architect (small lane)
- **Where:** `deploy/RUNBOOK.md:468`
- **Relates to:** QA-36-r1-5, ADR-017 M(N), F-31 DP-8
- **Evidence:** ADR-017 now gives P as about 55 MB allocated, plus one frame GET (one original and one parse). The open RUNBOOK `memory.peak` row still says "one preview-map upload (F-36, up to 16 MB more)". The row names neither a 16-bit interlaced upload nor a concurrent frame GET.
- **Consequence:** The VPS measurement that sets the memory cap can run without the F-36 worst case, so the cap can come out too low.

- **ID:** QA-36-r2-2
- **Severity / kind:** Minor / risk (design not recorded)
- **Route:** architect (small lane)
- **Where:** `src/server/internal/lifecycle/previewmap.go:51-54`; the MC-4 row
- **Relates to:** MC-4, SH-12, QA-36-r1-4
- **Evidence:** The frame slot is global. Any token holder who loops GET keeps it busy. Each hold lasts one parse, and up to the 30 s read deadline while the parse slots are queued. Every other workout's publish job then gets `503` and has no retry within its 10 s budget, so those workouts keep the F-23 card. This is the same class as r1-4, but MC-4's "Accepted" sentence covers only the image slot (PUT). The fix chose availability of the map card over memory, which is the right trade, but the trade is not written down.
- **Consequence:** No data or privacy effect. The only cost is the loss of map cards for everyone while the slot is held. Add the frame slot to the accepted note in MC-4.

### Observation, not F-36
`TestCaddyForwardedFor` (F-31) failed once in `make ci`, then passed 6 of 6 times in isolation and in a full rerun. In the test, the upstream handler writes `got` with no synchronisation. Track it outside F-36 if it recurs.

## Previous findings
- **QA-36-r1-1 (Major): fixed.**
  - **Slot position:** `PreviewFrame` does the token `lookup` first, so a request without a valid token cannot take the slot. It then takes `frameSlot` with a CAS before `Frames.Frame`, which reads the original, so a waiting request holds no memory.
  - **Release:** a `defer` releases the slot on every path: success, the `gone` error, other errors, context cancellation inside the parse wait (the parse returns an error, and the defer runs), and a panic. The "slot never released" and "released early" mutants are killed.
  - **Handler:** it maps `ErrMapBusy` to `503 busy` with the map message (tested).
  - **Front end:** a 503 goes through `send` as `ok:false`, not as `notFound`. `basemapJobs` then returns at `!got.ok`, with no render, no PUT and nothing shown. That path is covered by the "a failed GET" case in `basemap-job.test.ts`.
  - **Memory:** M(N) now gains at most one original plus one parse from GETs.
  - **Accepted cost:** see r2-2.
- **QA-36-r1-2 (Minor): fixed.** `TestPutPreviewMapRelookup` covers both races.
  - **Method:** an `eofReader` runs a publish or a hiding change after the body is read, before the lock.
  - **Publish:** the in-place build is asserted.
  - **Hiding:** `ErrMapStale` and no stored map are asserted. The fake changes the key with `HideEnds`, so the stale record would give a matching key.
  - **Mutant:** M5 is killed.
- **QA-36-r1-3 (Minor): fixed.** The test counts ink pixels inside the measured attribution box. Below 100 it fails. M10 is killed.
- **QA-36-r1-4 (Minor): fixed (accepted).** MC-4 now records the shared image slot and its effect. The new frame slot has the same effect and is not recorded (r2-2).
- **QA-36-r1-5 (Minor): partially fixed.** The ADR-017 note is corrected. The RUNBOOK row is not (r2-1).

## Questions
None.

## Coverage
This round covered only the diff of 59b977d and the routed findings. The AC trace in r1 still holds: the diff changes no AC behaviour beyond adding a 503 that the job already treats as a fallback. Not run here: the user checks listed in r1 (real browser, messengers, city card size, CSP console, mobile, look review, VPS `memory.peak`).
