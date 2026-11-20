# ADR-059 Interface language: decided in the head script, applied in the browser over English server output

- **Status:** accepted (20 Nov 2026), with the F-54 design v0.1
- **Date:** 20 Nov 2026
- **Features:** F-54 (introduces it); F-61, F-62 and every later feature with text (bound by it)
- **Supersedes:** none. Extends [ADR-026](ADR-026-activity-page-template-adopted-not-hydrated.md) item 5's head-script exception and [ADR-027](ADR-027-units-rewritten-in-place-one-units-store.md)'s in-place rewrite to the language; narrows F-11 U-8 ("codes never choose a text")
- **Depends on:** [ADR-032](ADR-032-response-headers-in-caddy.md) item 3, [ADR-024](ADR-024-upload-page-plain-typescript-entry.md), [ADR-005](ADR-005-workout-data-file-format-v1.md) item 1, [ADR-018](ADR-018-parse-outcomes-messages-and-log.md), [ADR-053](ADR-053-web-assets-served-from-the-data-directory.md)
- **Requirements:** DS-6, DS-1, INF-9, NFR Performance, NFR Privacy, TD I-7; OQ-223, OQ-226, OQ-228, OQ-281

## Context

Published pages are one pre-compressed static file per workout, written by Go in English and served by Caddy without Go. No cookie, link or request parameter may carry the language (DS-6), so the server cannot choose it. OQ-281 forbids a visible switch from English and caps the wait at 500 ms; OQ-226 allows only the chosen language's file, at most 5 KB gzip, outside the entry bundle, whose reserve is 2,074 bytes. Numbers and dates must follow the language, and the static page's numbers come from the data file's metadata (ADR-026 item 3).

## Decision

1. **Go speaks English only.** Pages, preview cards, API messages, notifications and logs stay English; the server never reads `Accept-Language` or a language. Translation happens in the browser.
2. **The head script decides the language once per page view** (stored choice, else the first matching browser language, else English) and writes it to `<html data-lang>`. Entries read it; they never decide themselves.
3. **Hold, then reveal.** For a language other than English, the head script hides the body (`visibility`, so the layout is final), starts the request for the page's language file named in the HTML by the build, and lifts the hold after 500 ms. The bundle applies the language only while the hold is still set; otherwise the page view stays English.
4. **Server markup is rewritten in place, text only:** fixed strings by `data-t` keys, view-model strings by `lib/page`'s builders fed from the **data file's metadata read from its streamed prefix**, so the reveal does not wait for the streams.
5. **Server messages are chosen by `error` code in the browser.** A committed list of codes, checked by a Go test and a frontend test, is the contract; the server's English `message` is shown only for an unknown code.
6. **De, es and fr are content-hashed JSON files per text group** (entry or lazy chunk), plain strings, at most 5 KB gzip; English stays in the code and the markup.

## Consequences

- No-JavaScript pages, preview cards and the iOS Shortcut stay English by construction (OQ-223, OQ-224).
- The data file format, the view model, the samples and Go's formatters do not change; English output stays byte-equal between Go and TypeScript.
- **Cost:** a non-English first view paints only after the language file, the bundle and the metadata prefix (at most 500 ms later). LCP must be measured in German.
- **Cost:** a slow file or a very large metadata block (long pool swims) shows English for that page view; the next view usually has the file cached (immutable).
- **Cost:** the head script grows and gets a new CSP hash; About and generic pages gain a script, ending their "no script" rule (F-15 P-12, F-29 AB-1).
- Server sentences of fixed-sentence codes live twice (Go and the English table); the code list test keeps them equal.

## Rejected

- **Per-language static pages or server choice** (`Accept-Language`, a language in the link): forbidden by DS-6, varies a cached static file, quadruples publishing.
- **Translations and CLDR formatting in Go:** a second copy of every text and a second number and date formatter per language that must match `Intl` exactly.
- **Waiting for the whole data file before the reveal:** a 4-hour ride often misses 500 ms, so viewers would see English most of the time.
- **Metadata embedded in the HTML:** a second copy of the metadata and a view-model change, for what the streamed prefix already gives.
- **Reformatting the English numbers and words in the markup:** needs keys for every word anyway and parses printed text.
- **All languages in the bundle or one file for all texts:** breaks the entry budget and the 5 KB limit.
- **Applying a late file after the cap:** the visible switch OQ-281 forbids.

## Reversibility

Cheap: browser code, the head script and one list file. Pages and assets are rebuilt on deploy; old pages keep their own files (ADR-053). No one-way door.

## Would choose differently if

- QA measures German first views on the reference phone falling back to English often: then move the language pass out of the entry into a small separate script that needs only the language file and the prefix, before asking the user to raise the 500 ms cap.
- The interface gains a right-to-left language: then `dir` and the layout rules need their own decision.
