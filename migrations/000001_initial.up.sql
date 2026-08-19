CREATE TABLE IF NOT EXISTS configuration (
 id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, application TEXT NOT NULL, environment TEXT NOT NULL,
 namespace TEXT NOT NULL, key TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', version BIGINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL, UNIQUE (tenant_id, application, environment, namespace, key)
);
CREATE TABLE IF NOT EXISTS configuration_version (
 config_id TEXT NOT NULL REFERENCES configuration(id), number BIGINT NOT NULL, state TEXT NOT NULL,
 value_kind TEXT NOT NULL, value_json JSONB, secret_ref TEXT, rules_json JSONB NOT NULL DEFAULT '[]', dependencies_json JSONB NOT NULL DEFAULT '[]',
 expires_at TIMESTAMPTZ, revision BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY (config_id, number)
);
CREATE TABLE IF NOT EXISTS release_plan (id TEXT PRIMARY KEY, scope TEXT NOT NULL, state TEXT NOT NULL, fencing_token BIGINT NOT NULL, revision BIGINT NOT NULL, idempotency_key TEXT UNIQUE, payload JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL);
CREATE TABLE IF NOT EXISTS outbox_event (id TEXT PRIMARY KEY, topic TEXT NOT NULL, aggregate_key TEXT NOT NULL, payload JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL, delivered_at TIMESTAMPTZ, attempts INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS audit_record (id TEXT PRIMARY KEY, scope TEXT NOT NULL, actor TEXT NOT NULL, action TEXT NOT NULL, resource TEXT NOT NULL, metadata JSONB NOT NULL, occurred_at TIMESTAMPTZ NOT NULL, previous_hash TEXT NOT NULL, hash TEXT NOT NULL);
