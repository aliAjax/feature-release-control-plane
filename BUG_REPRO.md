# BUG_REPRO：过期扫描取消传播断点

## Bug 是什么
scheduler 包的 ScanGroup/ScanActive 忽略传入的 ctx，取消后仍继续扫描；ScanAll 使用钉在结构体上的 s.ctx 而非调用方 ctx，导致取消信号无法中止扫描。

## 如何触发
```sh
go test ./internal/scheduler -run '^TestScanAllStopsOnCancelledCtx$' -count=1
go test ./internal/scheduler -run '^TestScanActiveStopsOnCancelledCtx$' -count=1
```

## 错误信息
- `expected context.Canceled, got <nil>`
