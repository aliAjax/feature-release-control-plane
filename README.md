# Feature Release Control Plane

A pure Go 1.23 progressive configuration control plane for multi-tenant SaaS services. It provides tenant/application/environment/namespace isolation, immutable configuration versions, explainable targeting, fenced release transitions, immutable audit records, transactional-outbox-shaped delivery and Server-Sent Events. It deliberately does not provide an administrative UI or generic RBAC product.

## Implemented behavior

- Versioned keys with string, number, boolean, JSON, template and secret-reference value types.
- Draft, review, schedule, rollout, pause, publish, rollback and expiry version transitions.
- Deterministic SHA-256 bucketing, percentage rollout, time windows, labels, device attributes, priorities and mutually exclusive experiments.
- Optimistic revision checks, release fencing tokens, idempotency keys, audit hash chaining and in-memory outbox deduplication.
- `/api/v1/evaluate` response with a rule-by-rule explanation, ETag and HMAC version proof.
- HTTP SSE stream with cursor IDs, reconnect-safe event IDs, heartbeats and bounded subscriber backpressure.
- Readiness, health and standard-library pprof endpoints.

The development adapter persists in memory so the service can start without infrastructure. PostgreSQL migrations and Compose infrastructure are supplied for the production adapter boundary; the production PostgreSQL/Redis adapters are deliberately not represented as working code yet and must not be enabled as though they were active.

## Quick start

Requires Go 1.23 or later.

```sh
cp .env.example .env
go run ./cmd/controlplane
curl -fsS http://127.0.0.1:8080/readyz
./build/smoke.sh
```

Use a second terminal for the worker if desired:

```sh
go run ./cmd/worker
```

To build an isolated image and local dependencies:

```sh
docker compose up --build
docker compose down --remove-orphans
```

## API example

```sh
curl -sS -X POST http://localhost:8080/api/v1/configs \
  -H 'Content-Type: application/json' -H 'X-Actor-ID: release-manager' \
  --data '{"scope":{"tenant_id":"tenant1","application":"billing","environment":"staging","namespace":"flags"},"key":"new_checkout","description":"Enable new checkout"}'

curl -sS -X POST http://localhost:8080/api/v1/configs/CONFIG_ID/versions \
  -H 'Content-Type: application/json' \
  --data '{"value":{"kind":"boolean","raw":true},"rules":[{"id":"canary","priority":10,"percentage":10,"hash_attribute":"device_id","required_labels":{"cohort":"beta"},"enabled":true}]}'

curl -sS -X POST http://localhost:8080/api/v1/evaluate \
  -H 'Content-Type: application/json' \
  --data '{"scope":{"tenant_id":"tenant1","application":"billing","environment":"staging","namespace":"flags"},"subject":{"tenant_id":"tenant1","labels":{"cohort":"beta"},"attributes":{"device_id":"d-123"}}}'
```

The complete contract is in [api/openapi/openapi.yaml](api/openapi/openapi.yaml). The protobuf contract lives in [api/proto/config_resolver.proto](api/proto/config_resolver.proto); generated gRPC stubs are not committed because the transport adapter is intentionally pending.

## Verification

```sh
make fmt vet test build
make count
go run ./cmd/controlplane & pid=$!
./build/smoke.sh
kill -TERM "$pid"; wait "$pid"
```

`build/smoke.sh` proves health, config creation, immutable version creation and validation. The API uses structured JSON logs and emits no configuration secret values, because secret values are supplied only as references.

## Design and operations

- [Architecture and capacity model](docs/architecture.md)
- [Domain model and state machines](docs/domain.md)
- [Data dictionary](docs/data-dictionary.md)
- [Threat model](docs/security.md)
- [Operational runbook and failure drill](docs/runbook.md)

Use the SQL migration tool selected by the deployment platform to run `migrations/*.up.sql` exactly once, record its migration state, and run all state/outbox writes in one transaction in the production repository adapter.
