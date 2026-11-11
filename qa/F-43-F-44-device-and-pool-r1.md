# QA F-43 Device and sensor info + F-44 Pool length and lengths - round 1

- **Date:** 2026-11-11
- **Verdict:** ready. No Blocker or Major. The allow-list holds under every probe. The byte search is real but narrower than it looks: it cannot see a leak through the name chain (r1-1).
- **Inputs:** `features/F-43-device-info.md`, `features/F-44-pool-summary.md`, decision log of 11 Nov 2026 (OQ-146 to OQ-153); `architect/F-43-device-info.md`, `architect/F-44-pool-summary.md`, ADR-044, F-03 v1.6 §5.2, §5.7, §6, ADR-005 notes, ADR-011 item 4; commits 7245300 (backend) and 3544035 (frontend, HEAD), clean tree. Critical lane, effort high.

## Build and test results

- `make ci`: exit 0. `go vet` clean; `svelte-check` 0 errors and 0 warnings; `go test -count=1 ./...` all packages ok; vitest 67 files and 1,698 tests passed; bundle check `entry index.html` and `edit.html` 56,655 / 60,000 gzip PASS (+148 over 56,507, as booked in ADR-011); docker-check ok.
- `go test -race -count=1 -run 'Device|Pool|Sanitise|Readable|SensorKind|Metadata|RoundTrip'` on `internal/ingest`, `internal/workout`, `internal/datafile`, `internal/page` and `cmd/workout-explorer`: ok.
- `FuzzSanitiseName`, 60 s in a scratch copy: 91,376 execs, no failure. It ran alongside `make ci`, so the exec count is low.
- `Recorded with` occurs only in the `start.svelte` owner chunk, never in `main-*.js` (DI-9 holds).

## Findings

### Minor

- **ID:** QA-43-r1-1
- **Severity / kind:** Minor / risk (test strength, critical lane)
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/device_test.go:23`, `src/server/tools/corpusgen/f09.go:28`
- **Relates to:** F-43 AC 5, DI-1, UP-9, INF-1
- **Evidence:** I ran two mutants in a scratch copy:
  - Mutant A takes any device's `product_name` into the name chain. The byte search still passes, because `identifying-data.fit` has the file_id name "corpusgen", which wins the chain and hides the leak. Only the unit test `TestDeviceNameAndSensors` ("sensor product names never used") catches it.
  - Mutant B takes the creator's `descriptor` when the creator has no product name. The byte search passes, and so does the whole `internal/ingest` suite. The creator `device_info` in the fixture has no descriptor, and no unit test gives the creator a descriptor without a name.
- **Consequence:** the byte search guards the data file only against a leak through a non-name field. A regression in the name chain, the one place where text from `device_info` reaches the public file, would reach production unless a unit test happens to cover it. Fix: give `identifying-data.fit` no file_id product name and give its creator `device_info` a descriptor marker. Mutants A and B then fail the byte search.

- **ID:** QA-43-r1-2
- **Severity / kind:** Minor / risk (design drift, dead branch)
- **Route:** backend (one-line note in F-43 DI-2), or architect if a stricter rule is wanted
- **Where:** `src/server/internal/ingest/device.go:112`
- **Relates to:** DI-2 ("if it is valid UTF-8 … else not known")
- **Evidence:** the library's `utf8String` drops invalid bytes before `sanitiseName` sees the string. A patched file_id product name `\xff\xfeAB` becomes "Recorded with AB" (probe P5). So the `!utf8.ValidString` branch and its test case never run in production.
- **Consequence:** the design and the behaviour differ. This does not affect privacy or safety, since the output stays valid UTF-8 and plain text, but the unit test passes for a reason production never meets. Record in DI-2 that invalid bytes are dropped, or accept this finding as is.

- **ID:** QA-43-r1-3
- **Severity / kind:** Minor / risk (test gap)
- **Route:** backend, frontend
- **Where:** `src/server/internal/page/page_test.go:79` (hostile-text sample), `src/server/internal/datafile/datafile_test.go:569` (Go reject table)
- **Relates to:** ADR-026, F-03 §6 rule 4
- **Evidence:**
  - Escaping: no committed test renders a device name containing markup. My probe of the real template with `Device = "Recorded with <script>…‮&amp; \"x\""` gave fully escaped output (`&lt;script&gt;`, `&amp;amp;`, `&#34;`), and with `Device = ""` it printed no `<p class="device">`. Svelte prints `{device}` as text, and `hygiene.test.ts` bans `{@html}`.
  - Reader checks: the Go reject table has no `device` case. My probe showed that Go `Decode` rejects a missing `device`, a non-object `device`, a numeric `name` and a numeric sensor code. The TS reader has all four cases tested.
- **Consequence:** both behaviours are correct today but unpinned. Add a hostile `device.name` to the hostile-text sample transform (Go and TS) and `device` rows to the Go reject table.

- **ID:** QA-43-r1-4
- **Severity / kind:** Minor / preference (stale comments)
- **Route:** backend
- **Where:** `src/server/internal/ingest/convert.go:238-239` ("Used for the log line only"), `src/server/internal/ingest/listener.go:41` ("For the log line.")
- **Relates to:** DI-2
- **Evidence:** `garminProduct` now feeds the model fallback (`device.go` `model()`).
- **Consequence:** the comments are misleading. A reader may believe this value never reaches the public file.

## Previous findings

None (round 1).

## Questions

1. **OQ-152 outcome, for the user and the architect.** The decision says the engineer checks local real files for `source_type`. The result is not recorded in any document or commit. I checked it locally, and no figures go into this report:
   - The Wahoo file: of its five non-creator devices, four carry only `product_name` (no `source_type` and no `device_type`). The fifth is ANT+ shifting, an unknown type.
   - The Garmin file: only `local` (built-in) devices.
   - The watch files: no non-creator devices.

   So no real file in the corpus lists any sensor. The "missing `source_type` counts as ANT+" fallback would not help, because `device_type` is missing too. The only remedy would be reading sensor product names, which the spec forbids.

   **The answer that settles it:** the user accepts that such files show no sensors, and the decision log says so. This is not a defect: the code follows DI-5.
2. **Creator name that sanitises to empty (probe P4).** A first creator `device_info` whose name is only control characters blocks a later valid creator name in the same sequence, and the line falls back to the profile name ("Garmin Edge 530"). DI-2 does not say which creator message wins. **Settles it:** the architect confirms that "first non-empty raw name" is intended, or the engineer stores the first name that sanitises to non-empty. Low value either way.

## Coverage

**Privacy probes (scratch copy, all passed):**

| Probe | What it tested | Result |
| --- | --- | --- |
| P1 | Creator `product_name` only in sequence 2 | Not used |
| P2 | Creator with descriptor, serial, versions and battery | Only the product name appears |
| P3 | `device_info` without `device_index`, carrying a name, a descriptor and a heart-rate type | Counted as a sensor; name and descriptor absent |
| P7 | Creator `product_name` equal to a sensor's descriptor | Shown as the device name. By design (OQ-146): the field is the product name, not the descriptor |

On the six local real files, no non-creator product name or descriptor occurs in the encoded data file. Strings are copied by the decoder (`utf8String`), so nothing aliases a reused buffer.

**Sanitiser:** valid UTF-8 out; Cc and Cf removed (bidi controls, ZWSP, BOM); white space including NEL, U+2028 and NBSP collapsed; the 40-character cut counts runes (`é` × 45 → 40) and does not end on a space; result idempotent (fuzz).

**F-43 acceptance criteria:**

| AC | Status | Traced to |
| --- | --- | --- |
| 1 | traced | Unit tests: cut to 40, Garmin profile name, `development` and makers not in the profile |
| 2 | traced | Sample "Sensors: heart-rate monitor" |
| 3 | traced | All four DI-7 forms pinned in the Go and TS samples; empty line gives no element (template and `ActivityPage.dom.test`) |
| 4 | traced | `indoor-no-gps` golden and unit test |
| 5 | traced | Byte search, with the limits of r1-1. It does not cover `.br`, which is `precompress.Brotli(bin)` of the same bytes |
| 6 | traced | Go and TS sample parity; edit-page placement test |
| 7 | not traced | Manual |
| 8 | traced | Golden data files pin the line: `chained-ride-run` gives "Garmin Edge 530" with the union of the sensors of both sequences; `triathlon` |
| 9 | traced | `about.test.ts` |
| 10 | traced | Metadata +40 to +92 bytes; corpus round trip green; LCP not measured |

**F-44 acceptance criteria:**

| AC | Status | Traced to |
| --- | --- | --- |
| 1, 2, 4, 6 | traced | `TestPoolSummary`, `TestPoolRead`, samples `pool-swim` and `pool-swim-no-lengths` |
| 3 | traced | `pool-swim-25yd` sample and `units.dom.test` (metric and imperial) |
| 5 | traced | Unit test and `triathlon` sample (open water keeps 25 m and gets no cells) |
| 7 | partly traced | Order pinned by samples (after Calories, before the first tinted cell). Untinted by construction: the keys are not in the CSS tint list. No automated tint test (manual) |
| 8 | traced | Sample parity |
| 9 | traced | `TestPoolTruncated` |
| 10 | traced | As F-43 AC 10 |

**Pool rule checked:** lap swimming only; half away from zero; m and yd never mixed; the yd value is computed from metres ÷ 0.9144; PS-4 key order. The PS-8 trim hand-off cannot be traced, because no trim code exists yet.

**Reader:** the version check is exact (3) in Go and TS; `device` is required in both; unknown sensor codes are skipped by both builders.

**Not run:** manual and visual checks (below), LCP, a full `-race` run of all packages.

**Manual checks for the user:**
1. Viewing page with JavaScript off, then the edit page, for `indoor-no-gps.fit`, `indoor-no-distance-no-gps.fit`, `ride-no-laps.fit` and `pool-swim-25yd.fit`. Check that the line and the cells match on both pages and that there is no gap without a line.
2. At 360 px, in both themes: the line wraps and the page does not scroll sideways.
3. Pool and Lengths cells untinted in both themes; the units switch leaves "25 yd" as is.
4. After deploy and `regenerate`, a real Wahoo upload shows the device line and no sensors (see question 1).
