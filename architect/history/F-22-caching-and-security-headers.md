# F-22 Caching, fast removal and security headers: history

History of [architect/F-22-caching-and-security-headers.md](../F-22-caching-and-security-headers.md).

## 25 Nov 2026: stored-split head script built in F-73

H-4, the Owner line and the OQ-88 row now point to [F-73](../F-73-remembered-split-without-layout-jump.md). Text as it was:

- H-4, last sentence: "**The stored-split jump fix (decision log 29 Oct, F-13 D-10) is not built in F-22** unless OQ-88 says so". Why column: "Keeps the policy free of exceptions; the hash rule exists so the later fix is one frozen script and one hash".
- Owner: "**No frontend change** unless OQ-88 is accepted (then frontend first, backend adds the hash)."
- OQ-88 (resolved 31 Oct: not now): "Fix the layout jump with a remembered map split now, with an inline `<head>` script allowed by its CSP hash (decision log 29 Oct, F-13 D-10)? It makes F-22 frontend plus backend and revises ADR-026 item 5". Suggestion: "**Not now.** Keep the accepted jump; ADR-032 item 3 keeps the later fix to one frozen script and one hash". Blocks: "Only the H-4 exception; F-22 builds without it".
