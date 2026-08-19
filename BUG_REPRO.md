# BUG_REPRO：健康窗口快照底层数组共享

## Bug 是什么
releasedomain 包 HealthWindow.Snapshot/Tail 返回共享底层数组的子切片，后续 Record 原地压缩把已返回快照冲掉；MemoryRepository.Create/Get 直接存/取输入切片引用，调用方改动污染仓库。

## 如何触发
```sh
go test ./internal/releasedomain -run '^TestSnapshotIsolationAfterCompaction$' -count=1
go test ./internal/releasedomain -run '^TestTailIsolationAfterCompaction$' -count=1
go test ./internal/releasedomain -run '^TestCreateCopiesInputSlices$' -count=1
go test ./internal/releasedomain -run '^TestGetCopiesStoredSlices$' -count=1
```

## 错误信息
- `snapshot mutated after compaction`
- `tail mutated after compaction`
- `stored VersionRefs was aliased`
- `stored release was mutated through Get`
