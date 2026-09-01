# Webhook Inspector

Small, self-hosted Go request bin for learning practical HTTP/backend work. Keep v1 simple and production-shaped, not internet-scale.

## Working together

- Read `.agents/PROGRESS.md` and `.agents/PROJECT_PLAN.md` at the start of implementation work.
- Guide the user by default: recommend one focused next step and explain it briefly. Give detailed steps when asked; write code only when explicitly requested or when the user asks for help/review.
- Treat the plan as guardrails. Let design choices and code boundaries emerge during implementation.
- Before moving on, confirm the current milestone works and update `PROGRESS.md` with the outcome and next step.
- When helpful, relate Go concepts to TypeScript/JavaScript.

## Technical constraints

- Prefer one executable, `net/http`, SQLite, and the standard library.
- Start concrete; add packages, interfaces, or dependencies only after a real need appears.
- v1 excludes accounts, auth, queues, caching, rate limiting, distributed infrastructure, and an SPA.
- Keep `README.md` current with how to run and use the app. Add focused tests with behavior; use temporary real SQLite for persistence tests.
