# ADR-020 Workout identifiers, edit token and link formats

- **Status:** accepted; revision 2 (first-open marker removed, published page added, item 7) accepted with the F-20 design v0.2; revision 3 (range link, item 11) with the F-40 design v0.1
- **Date:** 16 Oct 2026; revision 1 of 23 Oct 2026; revision 2 of 30 Oct 2026; revision 3 of 9 Nov 2026
- **History:** [architect/history/ADR-020-workout-identifiers-edit-token-and-links.md](history/ADR-020-workout-identifiers-edit-token-and-links.md)
- **Features:** F-09 (introduces it), F-11, F-12 (token check), F-15 (public folder name), F-19 and F-20 (edit page, published page), F-21, F-22 and F-31 (Caddy routes), F-27 (operator delete by `public_id`), F-40 (range link)
- **Supersedes:** none
- **Requirements:** SH-1 (unguessable URL), AC-1 (secret edit link, shown only on the published page), AC-6, NFR Security (128-bit public IDs and edit tokens, tokens stored hashed), TD Security and privacy (IDs and tokens: `crypto/rand`, SHA-256, token sent as a header, never in a URL that could leak through referrers, the edit link carries it in the fragment), TD Data model (`id`, `public_id`, `edit_token_hash`), F-09 AC 2 and AC 8

## Context

A workout needs three secrets or names: the internal key that names its private folder, the public ID in the viewing link, and the edit token, the only credential (AC-1). The TD fixes the size and source (128 bits, `crypto/rand`), base62 for `public_id`, SHA-256 for the token, and the fragment for the edit link. It leaves open the internal `id`, the exact encodings, the edit link's path and the header that carries the token. Links are handed to people, so once workouts exist their format cannot change without breaking links that are still alive (up to 30 days).

## Decision

1. **Internal ID (`id`):** 16 bytes from `crypto/rand`, written as 32 lowercase hex characters. It names `private/{id}/` and is the only workout identifier that may appear in a log line. It is never sent to a browser. Not a database row number: it is never reused, even if the database is recreated while folders remain, and it reveals no upload count.
2. **Public ID (`public_id`):** 16 bytes from `crypto/rand`, read as a big-endian unsigned 128-bit integer and written in base62 with the digit order of Go's `math/big` (`0-9`, then `a-z`, then `A-Z`, so `new(big.Int).SetBytes(b).Text(62)` is the reference), most significant digit first, **left-padded with `0` to exactly 22 characters** (62²¹ < 2¹²⁸ ≤ 62²²). Fixed width makes validation one pattern, `^[0-9A-Za-z]{22}$`. It is not logged.
3. **Edit token:** 16 more bytes from `crypto/rand`, encoded exactly like the public ID (22 base62 characters). It is returned once, in the upload response's edit link, and never again (AC-1).
4. **Stored hash:** `edit_token_hash` = SHA-256 of the token's 22 ASCII characters, a 32-byte `BLOB`. No salt and no slow hash: a 128-bit random token cannot be guessed, so a fast hash loses nothing. Checks (F-12) compare hashes with `crypto/subtle.ConstantTimeCompare`.
5. **Collisions:** `public_id` is `UNIQUE` and `id` the primary key (ADR-019). A collision (probability about 2⁻¹²⁸ per pair) fails that upload with a server error; there is no retry loop to test.
6. **Viewing link:** `/w/{public_id}` (TD Publish and view flow, View 1). Served only by Caddy from `public/` (I-1).
7. **Edit link:** `/edit/{public_id}#{token}`. The fragment the server returns is the bare token, nothing else. Browsers never send the fragment to a server or in a `Referer`, so the token does not reach Caddy's or Go's logs. `/edit/` is a separate path from `/w/`: the view path stays purely static, and Caddy serves one edit page (F-19) for every `/edit/*` with `no-store`. The base62 alphabet has no `&`, `=` or `#`, so a later fragment parameter can follow the token without ambiguity. The edit page takes the token as the fragment up to the first `&` and ignores anything after it. The upload page (F-11) opens the edit link exactly as returned; there is no first-open marker (rev. 2).

   **(rev. 2) Published page:** `/published/{public_id}#{token}`, the same token rules, served by Caddy as the edit page's file (F-20 C-1). The server never produces it: the browser builds it, and the edit link it shows there, only in `lib/owner` (F-19 E-2), from the path's `public_id` and the fragment's token.
8. **Links in API responses are root-relative paths** (`/w/…`, `/edit/…#…`). The browser makes them absolute with its own origin; the server needs no base-URL setting for F-09 (F-15 or F-23 add one when absolute URLs are needed for Open Graph).
9. **Token transport to the API (F-12):** the header `Authorization: Bearer <token>`. Caddy writes `Authorization` as `REDACTED` in access logs by default (since v2.5); a custom header such as `X-Edit-Token` would be logged in clear. A token in a query string or the path is refused as if it were absent (F-12 AC 1). F-31 must not enable Caddy's `log_credentials`.
10. **Where the code lives:** `internal/lifecycle` generates all three values (`NewIdentity(io.Reader)`), hashes tokens (`HashToken`) and formats both links (`ViewPath`, `EditLink`). Nothing else on the server builds these strings; in the browser only `lib/owner` builds a link with the token.
11. **(rev. 3) Range link (AP-19):** the viewing link may carry `#range={from}-{to}`, whole-second time offsets, built and read only in the browser on the viewing page ([F-40](F-40-shareable-range.md) SR-1 to SR-5). The server never sees, builds or logs it; the edit and published pages never read it, and their fragment stays the bare token. Copy link (SH-6) never includes it.

## Consequences

- One encoder (base62, fixed 22) for both public values; one hex form for the internal value. An operator can tell them apart at a glance in a log (32 hex vs 22 base62), and a log line with a 22-character base62 string is a bug.
- Base62 is case-sensitive. Two public IDs that differ only in case would clash as folder names on a case-insensitive file system (a developer's macOS). With 128-bit random values this is not a practical risk; the production file system is case-sensitive (F-31).
- **One-way door (soft): the link formats.** Changing `/w/{public_id}`, `/edit/{public_id}#{token}` or `/published/{public_id}#{token}` breaks every link handed out in the last 30 days, unless Caddy redirects the old form for that long. (rev. 3) The same holds for the `#range=` form: the browser must keep reading it while such links live. The ID lengths and encodings are equally fixed once rows exist.
- The token is never stored, so a lost edit link cannot be recovered (AC-1); the workout simply expires (UP-7).
- (rev. 2) The token is in the browser history under both owner paths. It was there under `/edit/` already; the published page adds no new place.

## Would choose differently if

- (31 Oct 2026) No longer conditional: F-22 sends `X-Robots-Tag` on `/edit/*` and `/published/*` ([F-22 design](F-22-caching-and-security-headers.md) H-8).
- A future feature needs the token server-side without a header (for example a download link that a browser follows): use a short-lived signed URL derived from a token check, never the token itself in a URL.
