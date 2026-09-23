# Documentation routing

This index helps agents find the smallest useful set of project documents.
Files under `docs/` describe the currently implemented system unless they say
otherwise. Planning and work status live under `.agents/`.

## Start here

For implementation work, read:

1. [Current progress](../.agents/PROGRESS.md) for the completed state and next
   milestone.
2. [Project plan](../.agents/PROJECT_PLAN.md) for scope, constraints, and
   milestone intent.
3. [Current architecture](architecture.md) for the component map.
4. The task-specific document from the table below.

Use [design notes](../.agents/design-notes.md) when a task involves an open
decision or a later milestone. Use
[completed milestones](../.agents/done-milestones.md) when the reason for an
already shipped behavior matters.

## Route by task

| Task or question | Read |
| --- | --- |
| Service boundaries, components, data flow, or ingress | [Current architecture](architecture.md) |
| SQLite schema, IDs, transactions, connection setup, or database failure | [Database behavior](db.md) |
| Owner cookies, invitations, guest access, or mutation authorization | [Bin access](bin-access.md) |
| Expiry renewal, cleanup, or deletion | [Bin lifecycle](bin-lifecycle.md) |
| Per-bin limits, `MAX_STORE`, `507` responses, backup, or restore | [Storage capacity and backups](storage-capacity.md) |
| VPS prerequisites, deployment, health checks, diagnostics, or code rollback | [Deployment runbook](deployment-runbook.md) |
| Public ingress verification, rate limits, shutdown, or exposure checks | [Production verification runbook](production-verification-runbook.md) |
| Backup creation, off-host transfer, retention, or isolated restore drill | [Database backup runbook](database-backup-runbook.md) |
| Routes, JSON fields, status codes, or SSE wire behavior | [HTTP API](http-api.md) |
| Vue behavior, session loading, request rendering, or capacity UI | [Frontend behavior](frontend.md) |
| Deferred product ideas and explicit v2 exclusions | [V2 deferred](v2-deferred.md) |
| Why a durable technical decision was made | [Architecture decision records](adr/) |
| Setup, build, tests, and a first product overview | [Repository README](../README.md) |

## Source-of-truth order

When documents disagree, use this order to investigate:

1. Current code and focused tests establish actual behavior.
2. Documents under `docs/` explain that implemented behavior.
3. `PROGRESS.md` records what was completed and what comes next.
4. `PROJECT_PLAN.md` and `design-notes.md` describe intended or pending work.

Resolve a mismatch instead of silently choosing one: update stale documentation
when behavior is already settled, or discuss a genuine requirement gap before
changing product behavior.
