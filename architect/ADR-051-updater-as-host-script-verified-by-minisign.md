# ADR-051 The updater is a host shell script; signatures are checked by the distribution's minisign against a pinned key

- **Status:** accepted (15 Nov 2026), with the F-52 design v0.1
- **Date:** 15 Nov 2026
- **Features:** F-52
- **Supersedes:** none
- **Depends on:** [ADR-050](ADR-050-deploys-run-on-the-server-from-a-signed-release.md)
- **Requirements:** INF-8, NFR Security; F-52 OQ-210, OQ-217

## Context

The updater must check a minisign (Ed25519) signature on the host before anything from a release is unpacked or run, without adding trust in GitHub, with no more rights than `deploy.sh`. The Go binary runs only inside containers without egress. Whatever does the check must itself be trusted from somewhere other than the release it checks.

## Decision

1. **The updater is a POSIX shell script, `update.sh`,** in the deploy directory, next to the scripts it drives. It ships in the signed deploy archive and is replaced by the deploy it runs; the checking code of a run is always the copy installed before that run.
2. **The verifier is the `minisign` package of the host's distribution** (Debian bookworm 0.11, trixie 0.12). The pin is the distribution's package archive, which the host already trusts for every security update. The default prehashed signature format is read by both versions.
3. **The public key is pinned by the hoster in `.env` (`WX_UPDATE_PUBLIC_KEY`)** and passed to `minisign` as a value. It is never read from a release, the repository or a download. The hoster takes it from `deploy/minisign.pub` on GitHub and confirms it against the DNS TXT record on the operator's domain. A new key is pinned by hand (OQ-217).

## Rejected

- **A subcommand in the Go binary** with our own minisign check: our own crypto parsing, a host-side role for a container binary, and the first trusted copy would be a binary downloaded from GitHub with checksums only.
- **minisign from its GitHub releases or a Go verifier fetched at install:** trust in GitHub, the thing INF-8 excludes.
- **GPG, signify, sigstore/cosign:** GPG's keyrings and trust model for one key; signify cannot read minisign signatures; sigstore adds online third parties.

## Consequences

- One more host package (`apt install minisign`) and one manual step at install: the first release is verified by hand with the same key.
- The updater inherits shell's limits: version components and archive members are checked with plain tools; the engineer keeps it small and `make ci` checks that it parses.
- A host whose distribution lacks `minisign` cannot use the updater or the release layout.

## Would choose differently if

- Hosters on distributions without a `minisign` package appear: then a static verifier built from a pinned, reviewed source by the hoster, still never downloaded from the release host.
- The updater's logic grows beyond selection, checking and state (for example several channels): then a small Go tool built and shipped like the binary, with the first copy verified by hand.
