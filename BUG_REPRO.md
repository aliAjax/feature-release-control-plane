# BUG_REPRO：发布状态机跨方法错位

## Bug 是什么
application 包 AdvanceReleases 不处理 Retrying 状态使其卡在中间态，ListReleases/ListReleasesByState 过滤掉 Retrying 任务，RetryRelease 直接跳转到 Running 而非进入 Retrying。

## 如何触发
```sh
go test ./internal/application -run '^TestAdvanceReleasesPromotesRetrying$' -count=1
go test ./internal/application -run '^TestListReleasesIncludesRetrying$' -count=1
go test ./internal/application -run '^TestRetryReleaseEntersRetrying$' -count=1
go test ./internal/application -run '^TestListReleasesByStateIncludesRetrying$' -count=1
```

## 错误信息
- `expected Retrying to be promoted to Running, got retrying`
- `expected retrying release to be listed`
- `invalid release transition: failed -> running`
