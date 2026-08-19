# Threat Model

| Threat | Mitigation |
|---|---|
| Cross-tenant read | Scope is required and subject tenant must match scope tenant. |
| Lost update | Revision and HTTP ETag preconditions. |
| Stale worker | Monotonic fencing tokens on release transitions. |
| Secret disclosure | Values support secret references only; audit/event payloads avoid secret material. |
| Event replay | Cursor and immutable outbox IDs enable client deduplication. |
| Tampered history | Each audit record links to the preceding SHA-256 hash. |
| Rule nondeterminism | SHA-256 bucket input is rule ID plus stable subject identity. |

Production deployments must terminate TLS, authenticate the actor at the gateway, store signing keys in a secret manager and rotate them by publishing a new key identifier before retiring the old one. Rate-limit evaluation by tenant and restrict pprof to an admin network.
