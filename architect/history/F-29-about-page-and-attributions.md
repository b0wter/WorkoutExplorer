# F-29 About page and attributions: history

## v0.1 (3 Nov 2026)

First design. Operator name and contact as two settings (`WORKOUT_EXPLORER_OPERATOR_NAME`, `_CONTACT`), filled into the template by `serve`; the whole privacy text in the template. Open: OQ-105 (theme choice in browser storage), OQ-106 (wording; a legal basis, b proxy access log, c hosting provider, d supervisory authority), OQ-5 for F-32.

- 4 Nov 2026: OQ-105 and OQ-106 suggestions accepted. OQ-106 (b) was: "keeps no access log", as DP-15 advises, if the operator confirms.

## v0.2 (4 Nov 2026), user decisions

1. Page split (AB-2, AB-3, AB-8): template holds only facts true for every deployment; operator facts come from an operator file read by `serve` at start (`WORKOUT_EXPLORER_OPERATOR_FILE`), placeholder when missing. The two operator settings of v0.1 removed before any release. PT rows gained a Home column; PT-1, PT-11, PT-12a (backups, split from PT-12 and PT-1), PT-15a (legal basis and complaints, split from PT-15) are operator rows.
2. OQ-106 (b) withdrawn: the template makes no access-log claim; the proxy's logging is stated in the operator file (PT-11).
3. OQ-5 settled: the FIT Protocol licence gets a sources entry (no origin).
