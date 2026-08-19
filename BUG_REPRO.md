# BUG_REPRO：限流错误链断裂

## Bug 是什么
ratelimit 包 NewBucket/Registry 用 %v 包装错误导致 errors.Is 无法识别 ErrLimited/ErrInvalidConfig；Bucket.Allow 无 token 时返回 nil；Registry.Reset 对未知 key 返回 true。

## 如何触发
```sh
go test ./internal/ratelimit -run '^TestBucketAllowReturnsErrLimited$' -count=1
go test ./internal/ratelimit -run '^TestRegistryAllowReturnsErrLimited$' -count=1
go test ./internal/ratelimit -run '^TestResetUnknownKeyReturnsFalse$' -count=1
go test ./internal/ratelimit -run '^TestNewBucketWrapsInvalidConfig$' -count=1
```

## 错误信息
- `second allow should be limited, got ok=false err=<nil>`
- `Reset on an unknown key should report false`
