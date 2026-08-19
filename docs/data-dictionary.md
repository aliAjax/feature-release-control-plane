# Data Dictionary

| Entity | Key fields | Retention |
|---|---|---|
| configuration | scope, key, revision | retained while namespace exists |
| configuration_version | config_id, number, state, value | immutable; expiry changes serving state only |
| release_plan | fencing token, waves, state | 180 days after completion |
| outbox_event | event id, topic, delivery timestamp | 14 days after delivery |
| audit_record | previous hash, hash, actor | 7 years or compliance policy |
