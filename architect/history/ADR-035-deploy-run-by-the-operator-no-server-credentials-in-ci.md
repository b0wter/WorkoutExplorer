# ADR-035 history

- **Original (31 Oct 2026):** title "Deploys are run by the operator from a CI artefact; CI holds no server credentials". Item 1: CI uploads `build/workout-explorer` and `build/web/` as a short-lived run artefact (14 days) after a green `make -k ci` on every push. Item 2: the operator's command deploys the artefact of HEAD's green CI run after checking the version string (the short commit hash).
- **Rev. 1 (1 Nov 2026):** added item 3 (the Dockerfile is not a deploy path; CI uploads no image, pushes to no registry) with the F-31 design v0.4.
- **Rev. 2 (2 Nov 2026):** user decision (decision log 2 Nov 2026, operator deploy from GitHub Releases). Run artefacts on branch pushes dropped; the deploy source is a GitHub Release created on a `vX.Y.Z` tag by a separate job with `contents: write`; the version is the tag. Consequence "a deploy needs GitHub CLI access to download the artefact" and the 14-day expiry are gone.
