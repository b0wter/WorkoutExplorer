# ADR-025 Owner API access: one token check, one not-found response, no rate limit

- **Status:** accepted (23 Oct 2026)
- **Date:** 23 Oct 2026
- **Features:** F-12 (introduces it), F-15 (publish uses the same check), F-19, F-20 (the edit and published pages show the generic page on not found), F-21, F-27
- **Note (30 Oct 2026):** wording only, private state removed (SH-2); no decision changed
- **Supersedes:** none. Narrows [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md) item 9 (query strings, below)
- **Requirements:** SH-2 (a draft looks like a page that does not exist), AC-1, AC-5, NFR Security (128-bit tokens, stored hashed; originals only for the edit-link holder); F-12 AC 1; TD Security and privacy (IDs and tokens, Private and draft workouts)

## Context

Every owner endpoint takes a `public_id` in the path and the edit token in a header. To a stranger, the answer must not show whether a draft exists (SH-2; the private state was removed on 30 Oct 2026), or whether a token was close. The token is 128 random bits and is stored only as its SHA-256 hash (ADR-020 items 3, 4). The answer has to be one response, produced by one code path, so that a later endpoint (F-15 publish) cannot drift from it.

## Decision

1. **Handle.** The workout is addressed by `public_id`, never by the internal ID. A path value that does not match `^[0-9A-Za-z]{22}$` is not found.
2. **Token source.** The token comes only from a single `Authorization` header with the scheme `Bearer` (compared without regard to case, RFC 9110 §11.1) and a value of exactly 22 base62 characters. Anything else counts as absent: no header, two headers, another scheme, or a malformed value.
3. **No query string.** A request to an owner endpoint with any query string is not found, **even with a valid header**. This is stricter than ADR-020 item 9. A client bug that puts the token into a URL then fails at once in development. It never works silently while the token ends up in proxy logs and browser history.
4. **One check, in `lifecycle`.** Every owner operation starts with the same lookup and check: find the row by `public_id`, then compare `HashToken(presented)` with the row's hash using `crypto/subtle.ConstantTimeCompare`. Each of these gives the same "not found" result, which `httpapi` turns into the one response:
   - no row;
   - a row whose `expires_at` is not after now (expired, but not yet removed by F-21's job);
   - an absent, malformed or wrong token.

   For a well-formed `public_id`, the code path is the same in every case: the lookup always runs, the comparison always runs, against a precomputed placeholder hash when the token is absent or malformed (no SHA-256 is computed then, and this depends only on the presented token), and against a fixed dummy hash when there is no row. A malformed `public_id` and a query string may return early, because they say nothing about any workout. Timing is a property of the code path. It is not measured and no delay is added: 128-bit values cannot be found by timing, and network jitter is far larger than the differences that remain.
5. **One response.** `404`, `Content-Type: application/json`, `Cache-Control: no-store`, body exactly `{"error":"not_found","message":"This workout has expired or is not available."}`. That is the sentence the user chose for the generic page (decision log, question 2). The bytes are identical in every case. Never `401` or `403`, and never a `WWW-Authenticate` header.
6. **No rate limit on owner endpoints.** A token cannot be guessed (2¹²⁸), and a refused request costs one indexed lookup and one hash. The expensive work behind these endpoints (file downloads, and F-15's regeneration later) needs a valid token, so it can only be spent on the caller's own workout. A limiter would add memory (ADR-021's L) and a state table for no security gain.
7. **No CORS and no CSRF token.** The credential is not ambient: a page can send it only by setting the header itself, which a cross-origin page cannot do without a preflight that we never answer. This is the reasoning of ADR-021 item 10.
8. **Refusals are not logged.** Logging them would record request metadata (I-7) or add a line that an attacker can provoke.

## Consequences

- An owner whose link is damaged sees exactly what a stranger sees: the generic page (F-19 AC 6). Client bugs (a missing header, a stray query) look like "not found" too. Tests catch them, not the response.
- Expiry takes effect for the owner API to the second, whatever the timing of F-21's job.
- A token holder can download their own files as often as they like. That costs bandwidth only.
- Every new owner endpoint (F-15 publish, phase 2 pins) must go through the item 4 check. `httpapi` has no other way to answer "not found" for a workout.

## Would choose differently if

- Abuse shows up in practice: many not-found answers from a few addresses, or heavy repeated downloads. Then add a per-address limit in Caddy first (ADR-021 "Would choose differently if"), and the `httpapi` limiter as the backstop.
- A browser has to fetch an owner file by following a link, without setting a header. Then use a short-lived signed URL, created after the item 4 check (ADR-020 "Would choose differently if"), never the token in a URL.
