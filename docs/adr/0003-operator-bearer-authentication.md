# ADR 0003: Protect administrative routes with a separate operator bearer token

## Status

Accepted

## Context

Hooklook needs operator-only routes to issue and manage finite-use bin-creation
tokens. The current development-only `/admin/bins` endpoint also exposes every
captured request and must not be publicly available without protection.

V1 has no accounts or user management. A small self-hosted service does not
need an account system merely to support its operator.

## Decision

Every `/admin/*` route requires an `Authorization: Bearer <token>` header with
one configured, non-expiring operator token. The operator token is distinct
from issued bin-creation tokens: it grants administration privileges and must
never be accepted as a substitute for a finite-use creation token.

The operator token is deployment configuration, not application data. It is
manually rotated by replacing the configured value and restarting or reloading
the service. Do not log it. Compare credential material in constant time when
the authentication middleware is implemented.

Public inbound bin routes remain unauthenticated by design. `POST /api/bins`
uses the issued finite-use creation token, while `/admin/*` uses the separate
operator token.

## Consequences

- The development `/admin/bins` endpoint must be removed, disabled by default,
  or placed behind the operator authentication before deployment.
- Token administration needs no account model, sessions, or token expiry
  infrastructure.
- A leaked operator token has full management access until manual rotation, so
  it belongs only in deployment secret storage.
