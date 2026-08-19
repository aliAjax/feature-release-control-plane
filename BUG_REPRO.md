# BUG_REPRO：仓储读路径缺锁 + 内部引用逃逸

## Bug 是什么
repository 包 Published/List/ListVersions/ListAudit 不加读锁直接迭代内部 map，ListAuditAll 返回内部切片引用，Published/ListVersions 返回共享底层数组的 Rules/Dependencies，并发读写共享 map 与底层数组产生竞态和引用逃逸。

## 如何触发
```sh
go test -race ./internal/repository -run '^TestMemoryConcurrentReadsAndWrites$' -count=1
go test -race ./internal/repository -run '^TestMemoryConcurrentAuditAppend$' -count=1
go test -race ./internal/repository -run '^TestMemoryConcurrentVersionReads$' -count=1
go test -race ./internal/repository -run '^TestPublishedDoesNotAliasInternalRules$' -count=1
go test -race ./internal/repository -run '^TestListVersionsDoesNotAliasInternalRules$' -count=1
go test -race ./internal/repository -run '^TestListAuditAllReturnsCopy$' -count=1
```

## 错误信息
- `WARNING: DATA RACE`
- `published version was mutated through the returned slice`
- `version was mutated through the returned slice`
- `audit record was mutated through the returned slice`
