# F-28 Report form

- **Phase:** 1 MVP
- **Owner:** frontend
- **Requirements:** INF-2 (form), DS-3; Technical design: Front end (Implementation notes)
- **Depends on:** F-13, F-27

## Goal
A viewer flags a workout that should not be online in a few taps.

## Scope
- Report link in the footer of every published workout page opens a short form.
- Reason (spam, illegal content, someone else's personal data, other), optional message with a 1,000-character limit and counter.
- Posts to `POST /api/reports`; confirmation on success; plain error on failure or rate limit.
- No contact fields.
- Not shown on the edit page (assumption, see README).

## Out of scope
- Server handling (F-27).

## Acceptance criteria
- [ ] The Report link opens the form on phone and desktop without leaving the page.
- [ ] Submitting without a reason is not possible; submitting with a reason and no message succeeds and shows a confirmation.
- [ ] The message field stops at 1,000 characters.
- [ ] The form asks for no name, email or other contact details.
- [ ] A rate-limit or server error shows a plain message and keeps the entered text.
