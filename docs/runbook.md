# Runbook and Failure Exercise

Start locally with `go run ./cmd/controlplane`; use `./build/smoke.sh` after `/readyz` returns 200. Docker development dependencies start via `docker compose up --build`; shut down with `docker compose down --remove-orphans`.

For a stuck rollout, inspect the release record and audit chain, acquire a new lease/fencing token, then call the rollback endpoint. Never reuse a token. If SSE delivery lags, scale the delivery worker, preserve the outbox and let clients reconnect with their last cursor. If evaluation latency rises, evict affected snapshots, reduce rule complexity, and compare cache-hit metrics before resuming waves.

Failure drill: create a scheduled version and release, stop one worker, verify another worker cannot act without a higher fencing token, reconnect an SSE client, verify it receives a distinct cursor, then roll back. Record the timeline in the audit export before cleaning up.
