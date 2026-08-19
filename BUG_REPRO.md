# BUG_REPRO：webhook 投递生命周期错位

## Bug 是什么
notify 包的投递链路有三处协同失效：重试策略在耗尽后吞掉错误并返回 nil；已取消的 ctx 仍会触发一次回调；WebhookClient.Deliver 忽略 5xx 状态码；Dispatcher 关闭后 Enqueue 仍向已关闭 channel 发送导致 panic。

## 如何触发
```sh
go test -race ./internal/notify -run '^TestRetryReturnsErrorAfterAllAttempts$' -count=1
go test -race ./internal/notify -run '^TestRetryHonorsCancelledContext$' -count=1
go test -race ./internal/notify -run '^TestEnqueueAfterCloseReturnsError$' -count=1
go test -race ./internal/notify -run '^TestDeliverReturnsErrorOnServerError$' -count=1
go test -race ./internal/notify -run '^TestDispatcherConcurrentEnqueueClose$' -count=1
```

## 错误信息
- `expected a non-nil error after all attempts fail`
- `expected context.Canceled, got <nil>`
- `Enqueue after Close must return an error, not panic: send on closed channel`
- `expected an error for a 5xx response`
- `concurrent Enqueue after Close panicked: send on closed channel`
