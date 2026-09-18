# Webhook Inspector

Small, self-hosted Go request bin for learning practical HTTP/backend work. Keep
v1 simple and production-shaped, not internet-scale.

## Working together

- Read `.agents/PROGRESS.md` and `.agents/PROJECT_PLAN.md` at the start of
  implementation work.
- Before moving on, confirm the current milestone works and update `PROGRESS.md`
  with the outcome and next step.
- Do not run frontend test (browser preview, playwright etc.); i'm eyeballing
  the results myself

## Technical constraints

- The frontend doesn't support a screen width below 1024px.
- Start concrete; add packages, interfaces, or dependencies only after a real
  need appears.
- v1 excludes accounts, auth, queues, caching.
- Add focused tests with behavior; use temporary real SQLite for persistence
  tests.
- Keep `README.md` current with how to run and use the app. 

## Ignore

- .devnotes.md
