# BUG_REPRO：零值构造 nil map/nil 指针 panic

## Bug 是什么
metrics 包零值构造不初始化内部状态：NewReservoir 返回 nil items/rand/counts，Count/Add 触发 nil map 写入与 nil 指针解引用；Summaries 写 nil map；Store 未初始化 reservoirs。

## 如何触发
```sh
go test ./internal/metrics -run '^TestReservoirAddDoesNotPanic$' -count=1
go test ./internal/metrics -run '^TestReservoirCountDoesNotPanic$' -count=1
go test ./internal/metrics -run '^TestSummariesReturnsMetricsForEachRelease$' -count=1
go test ./internal/metrics -run '^TestStoreReservoirDoesNotPanic$' -count=1
```

## 错误信息
- `panic: assignment to entry in nil map`
- `panic: runtime error: invalid memory address or nil pointer dereference`
