# Domain Model and State Machines

`Scope` is the tenant/application/environment/namespace isolation key. A `Config` owns immutable numbered `ConfigVersion` records. Values are JSON encoded and tagged as string, number, boolean, JSON, secret reference, or template. Secret references never contain secret material.

```mermaid
stateDiagram-v2
  [*] --> draft
  draft --> reviewing
  reviewing --> draft
  reviewing --> scheduled
  scheduled --> rolling_out
  scheduled --> paused
  rolling_out --> paused
  paused --> rolling_out
  rolling_out --> published
  rolling_out --> rolled_back
  paused --> rolled_back
  published --> rolled_back
  draft --> expired
  reviewing --> expired
  scheduled --> expired
  paused --> expired
  published --> expired
```

```mermaid
stateDiagram-v2
  [*] --> pending
  pending --> running
  pending --> cancelled
  running --> paused
  paused --> running
  running --> succeeded
  running --> failed
  running --> rolled_back
  paused --> rolled_back
  succeeded --> rolled_back
```

Target rules are evaluated by descending priority. Time window, label, device attributes and experiment exclusion are checked before a deterministic SHA-256 bucket is compared with the configured percentage. The response contains each rejection explanation, matching rule, version and HMAC proof.
