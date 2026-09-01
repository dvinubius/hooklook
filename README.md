# Webhook Inspector

A small, self-hosted request bin written in Go. Create a temporary endpoint, send it arbitrary HTTP requests, and inspect what was captured.

This is a learning project focused on Go's HTTP model, safe handling of untrusted payloads, SQLite persistence, and a small complete service.

## Project guidance

- [Plan](.agents/PROJECT_PLAN.md)
- [Current progress](.agents/PROGRESS.md)

## Status

Scaffolded; implementation has not started.

## Intended first step

Create a Go module and a minimal `net/http` server with `GET /health`. The planned request-bin flow begins in Milestone 1.
