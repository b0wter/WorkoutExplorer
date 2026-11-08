# QA F-35 Zip upload - round 1

- **Date:** 2026-11-09
- **Verdict:** ready with conditions. No Blocker or Major. The conditions are the user checks AC 45 (real Finder, Explorer, Linux `zip` and `zip -0` archives; iOS Safari picker) and AC 58 (wording on screen), which cannot be run here.
- **Inputs:** `features/F-35-zip-upload.md` (UP-11, UP-1, UP-2, UP-10, NFR Security and Compatibility); `architect/F-35-zip-upload.md` v0.1 (Z-1 to Z-12); ADR-040, ADR-024, ADR-002; decision log 7 Nov 2026 (zip upload, zip upload details) and 9 Nov 2026 (OQ-108 to OQ-111) in `features/README.md`. Commit f78b0de (HEAD); the working tree differs only in `features/README.md` (the Status column).

## Build and test results
- `make ci`: exit 0. svelte-check 0 errors and 0 warnings. Vitest: 59 files, 1,535 tests passed. Go tests passed. Caddy and Docker checks passed.
- Size: `entry upload.html: 3761 / 8000 bytes gzip PASS`. The design's baseline is 2,275, so F-35 adds 1,486, under its 2,500 cap. `edit.html` and `index.html` are 57,715 / 60,000. `upload entry: no storage API, no shared chunk PASS`.
- `git show --stat HEAD`: nothing changed under `src/server`, `deploy`, `package.json` or `package-lock.json`. The commit contains no binary archive and no FIT file. Test archives are built at run time from `src/server/testdata/corpus/synthetic`.
- Probes in the scratchpad (not committed):
  - Mutation run: an instrumented copy of `lib/zip`, given 3,000 random byte changes of the test archive. 677 inputs reached the catch-all, and every one was a `TypeError` from `DecompressionStream` (invalid deflate data). None was a `RangeError`, which would point to a bounds bug. All 391 `ok` results were byte-identical to the original. Truncation at every length: 0 reached the catch-all.
  - Node's `DecompressionStream('deflate-raw')` accepts bytes after the end of the deflate stream. Browsers reject them (Compression Streams spec). See Coverage.

## Findings

### Minor
- **ID:** QA-35-r1-1
- **Severity / kind:** Minor / defect (documentation)
- **Route:** architect
- **Where:** `architect/F-35-zip-upload.md`, Verification row "Many entries stay fast"; `spec/Workout Explorer — Technical Design.md:325`
- **Relates to:** Z-6, Z-4
- **Evidence:** The Verification table asks for "65,535 entries with one root FIT, accepted". Z-6 says "any all-ones count" is Zip64, and the code follows Z-6 (`index.ts:78`). The test uses 65,534 entries and explains why (`zip.test.ts`, "many entries").
- **Consequence:** The design contradicts itself. The engineer's choice is correct: Info-ZIP and libzip switch to Zip64 at 0xFFFF entries, and no FIT export comes anywhere near that number. The design was not updated to match. The technical design still marks `lib/zip/` as "planned".

- **ID:** QA-35-r1-2
- **Severity / kind:** Minor / risk (test)
- **Route:** frontend
- **Where:** `src/web/src/lib/zip/zip.test.ts`, "a few thousand random byte changes give only result values"
- **Relates to:** design Verification "Every structural failure is a refusal, never an exception"
- **Evidence:** Because `extractFit` catches every exception (`index.ts:49-53`), `expect(typeof r.ok).toBe('boolean')` cannot fail. The probe shows the code is sound today, but the test would also pass if a future bounds bug threw a `RangeError` or produced wrong bytes.
- **Consequence:** The test does not prove what it claims to prove. Fix: assert that every `ok` result equals the original bytes, which this test can fail on.

## Engineer's deviations (judged)
- **65,535 entries refused as Zip64:** accepted. This follows Z-6 and real tool behaviour. Only the design document needs fixing (r1-1).
- **Split check on each central entry's disk number (`index.ts:91`):** accepted. It falls under Z-6 ("a disk number other than 0"), costs nothing, and a single-disk archive always has 0 there.
- **Catch-all to "damaged":** accepted. It is how Z-1's "never throws" is implemented, and Z-8 says any stream error means damaged. The probe found that only stream errors reach it. The cost is that a future bug would show as "damaged" instead of failing loudly (r1-2 covers the test side).

## No archive bytes reach the server (checked paths)
- **Renamed zip** (`workout.fit`, no extension): detected by the first four bytes (`page.ts:76`). Only the extracted bytes are sent, as `new File(chunks, 'workout.fit')`. Tested in the DOM tests for three names.
- **Nested zip:** extracted bytes that start with any of the three signatures are refused (`index.ts:158-161`). Tested for deflated and stored entries, and in the DOM test.
- **Split first part (`PK\7\8`):** refused before parsing (`index.ts:61`). Tested in the library and DOM tests.
- **Later split parts, self-extracting archives and prefixed archives:** sent, and the server refuses them as not FIT. This matches the OQ-108 decision and ADR-040 item 2.
- **No `DecompressionStream`:** message shown, nothing sent, stored archives included (Z-9). Tested.
- **Read error, back-forward-cache restore, refusals:** nothing sent. The generation guard is checked after every `await` (`page.ts:76-92`). Tested.

## Previous findings
None (round 1).

## Questions
None. OQ-108 to OQ-111 are decided and built as decided.

## Coverage
- AC 45 (tools, iOS picker): the layouts are emulated in `zip.test.ts` (Linux extra field; Finder data descriptor with signature and `__MACOSX/`; Explorer data descriptor without signature; `zip -0`). Real archives: **user check U-1**.
- AC 46 (byte-identical, no zip bytes): FIT bytes are compared byte for byte for every synthetic corpus file, deflated and stored. The DOM test checks the sent part's name and bytes. A `.fit` file is sent as the picked `File` itself.
- AC 47, 48, 50, 51 (split first part), 52, 53 (limits: archive cap checked before `slice`; exactly 20,000,000 sent; 20,000,001 declared or produced refused; a 1 GiB bomb that declares 1,000 bytes stops at the limit plus one slice, with the stream cancelled), 54, 55, 56, 57: traced to code and passing tests.
- AC 49 (later split part and self-extracting archive refused by the server): follows from Z-3. The server's not-FIT path is tested since F-08. There is no end-to-end test (accepted).
- Rules checked in code and tests: CRC-32 and length against the central entry; `._` prefix; no `/` or `\`; `.fit` in any case, including `.fit` alone; encryption bits 0 and 6; methods 0 and 8 only; the declared size is checked before inflating.
- Not testable in Node: browser `DecompressionStream` behaviour. Node accepts bytes after the end of the deflate stream; browsers throw, which gives "damaged". Archives from standard tools have none, so the outcome is the same.
- Not run here: responsiveness on a real phone and the browser matrix.

### User checks
- **U-1 (AC 45):** On a desktop browser, drop each of these and confirm the edit page opens: a zip made by macOS Finder "Compress"; a zip made by Windows Explorer "Send to > Compressed (zipped) folder"; `zip ride.zip ride.fit` on Linux; and `zip -0 ride.zip ride.fit`. In iOS Safari, pick one of them with "Choose a file" (Files app).
- **U-2 (AC 46, 47):** In devtools Network, upload a zip renamed to `workout.fit`. The request's `file` part must be named `workout.fit` and be the size of the FIT file, not of the zip.
- **U-3 (AC 53, responsiveness):** On a phone, drop a zip holding a FIT file of about 20 MB. "Opening the archive…" appears, the page stays scrollable, and the upload starts. Optional: a bomb archive gives the extracted-size message within a few seconds.
- **U-4 (AC 58):** Read on screen: the intro, the drop zone text, "Opening the archive…", and the six archive messages (no FIT file, more than one, over 20 MB, could not be opened, browser cannot open, archive larger than 20 MB).
