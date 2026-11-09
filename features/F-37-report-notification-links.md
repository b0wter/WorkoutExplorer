# F-37 Report notification links

- **Phase:** 2
- **Owner:** both (backend first)
- **Lane:** critical (a new credential; deletes data)
- **Requirements:** INF-4 (changed 9 Nov 2026), INF-6 (new), INF-1, INF-3, INF-2, AC-4, SH-10, UP-7, NFR Privacy (Pushover), NFR Security (keys); extends F-27 (R-8, R-11; ADR-039 items 2 and Consequences need revising), F-29 (About text)
- **Depends on:** F-27, F-29, F-31

## Goal
The operator acts on a report from the phone: the Pushover notification opens the reported workout and offers a single-use link to delete it, with a confirmation step.

## Scope
- **Notification content (INF-4).** Readable reason instead of the code: `spam` "Spam", `illegal_content` "Illegal content", `personal_data` "Personal data", `other` "Other". The workout's viewing link (`<base URL>/w/{public_id}`) in Pushover's `url` field. The delete link (INF-6) in the message. Still never the reporter's message, the title or request data. `(+N more)` folding and the hourly cap stay as in F-27.
- **Delete key (INF-6).** Created for each stored report only when Pushover is configured: 128-bit random, valid only for that report's workout, single use, expires 30 days after the report (the architect may shorten it), stored only as a hash and deleted with the report and the workout (cascade, F-27 R-2). Carried in the delete link's URL fragment, like the edit token; sent to the server only in a header, never in a URL; never logged, never returned by any endpoint.
- **Confirmation page.** Opening the delete link never deletes. The page reads the key from the fragment and asks the server what the key authorizes, then shows enough to decide: workout title (or the default title, sport and date), start date, report number, reason and filed time, number of stored reports on the workout, expiry. A "Delete workout" button sends the confirm (`POST`); only that deletes. After a delete the page says "The workout has been deleted." Served with `noindex` and `no-referrer` (as every page, F-22), no Open Graph image (SH-11-style fixed card or none: architect), no cache.
- **Invalid states.** Unknown, malformed, used or expired key, and a workout that is already gone (owner delete, expiry, operator delete, another key) all get one clear message, e.g. "This delete link is no longer valid: it was already used, has expired, or the workout is gone." (OQ-114). Without JavaScript the page says that it needs JavaScript.
- **Deletion** runs the operator delete of F-27 R-8 unchanged (`lifecycle.Service.Delete`: row with its reports and keys, public folder under the lock, private folder, checkpoint), with the same SH-10 guarantee and the same log line (`by` = `operator`), without the key or its hash. Works for any state the report can point at (a reported workout is published).
- **Off without Pushover:** no key is created or stored, the confirmation endpoints answer not found.
- **About page (INF-1):** the Report notifications section says that Pushover receives the report number, the reason, the workout's viewing link and a single-use delete link, never the reporter's message.
- **Rate limit:** the key lookup and confirm requests are rate-limited per IP like reports (value: architect).
- Spec traceability updates in the technical design (architect): Hosting (Reports), the third-party line, Security (IDs and tokens), Glossary (delete key), Data model (`reports` or a new table), ADR-039 item 2 and its Consequences, F-27 R-11.

## Out of scope
- Any other operator action from the phone (listing reports, dismissing a report, hiding a workout). An admin page (not planned). Notifications to services other than Pushover. Keys for reports folded into `(+N more)` beyond those already sent (OQ-115). Showing the reporter's message on the confirmation page (OQ-116).

## Acceptance criteria
- [ ] With Pushover configured, a stored report sends one notification: message `New report #<id>: <Reason label>` plus the delete link; `url` is the workout's viewing link. The four reasons show the four labels above. No reporter's message, title or request data in any field (checked against a fake Pushover).
- [ ] With Pushover off, a stored report creates no key: the database holds no key hash, and the confirmation endpoints answer not found for any key.
- [ ] The database holds only the key's hash; the plain key appears in no table, log line, error message or HTTP response other than the outgoing Pushover request.
- [ ] Opening the delete link (`GET` of the page and the key lookup) deletes nothing, also when repeated; the workout stays online.
- [ ] The confirmation page shows the title, start date, report number, reason, filed time, report count and expiry of the reported workout, and a "Delete workout" button.
- [ ] Confirming deletes the workout exactly like `workout-explorer delete`: F-12's end-to-end list of places holding workout data is empty for it (row, reports, keys, public and private files), `/w/{public_id}` gives the generic page within 60 s (SH-10), and the log has the operator delete line without the key or its hash.
- [ ] The key works once: a second confirm or lookup with the same key gets the invalid-link message; so do keys of other reports on the deleted workout.
- [ ] A key for workout A never deletes or shows workout B, also when sent with B's public ID.
- [ ] A key past its expiry, an unknown key, a malformed key and a key whose workout is gone each get the invalid-link message, with the same status and body.
- [ ] The key never reaches the server in a URL: the server's request log and Caddy access log of a full open-and-confirm run contain no key.
- [ ] Every confirmation-page response carries `X-Robots-Tag: noindex`, `Referrer-Policy: no-referrer` and `Cache-Control: no-store`.
- [ ] Lookup and confirm requests are rate-limited per IP; a refusal is `429` and logs no address.
- [ ] The confirm request needs the key in a header and `application/json` (or the architect's equivalent), so a cross-site form cannot trigger a delete.
- [ ] The About page's Report notifications section names the viewing link and the single-use delete link, shown only when Pushover is configured.
- [ ] On the operator's phone (user check): the notification opens the workout; the delete link opens the confirmation page; confirming deletes the workout.

## Open questions

All resolved by the user on 9 Nov 2026 as suggested (decision log in [README](README.md)):

- **OQ-114:** one message for used, expired and unknown keys and a gone workout.
- **OQ-115:** reports folded into `(+N more)` get no delete link of their own; the architect may create keys only when a notification is sent.
- **OQ-116:** the confirmation page does not show the reporter's message.
- **OQ-117:** key lifetime 30 days; the architect may shorten it.
- **OQ-118:** INF-6 is Should within phase 2.
- **OQ-119:** risk of the delete link in Pushover accepted; stated on the About page.
