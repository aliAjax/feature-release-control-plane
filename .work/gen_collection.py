import json, os
from pathlib import Path

OUT = Path('/Users/hutu/Desktop/go_projects/work_projects/003-feature-release-control-plane/2026-08-20/feature-release-control-plane/.work')
GO = "golang:1.23; go 1.23.0; GOTOOLCHAIN=local"

R = {}

def rec(record, sample_id, bug_id, task_type, category, uq, vc, grc, sc, contract):
    R[record] = {
        "sample_id": sample_id,
        "session_id": "",
        "bug_id": bug_id,
        "task_type": task_type,
        "bug_category": category,
        "repo_url": "",
        "go_version": GO,
        "repro_determinism": "deterministic",
        "user_query": uq,
        "trajectory": "",
        "verify_cmds": vc,
        "gold_root_cause": grc,
        "gold_patch": "",
        "success_criteria": sc,
        "verify_result": "",
        "harness": "",
        "generator_model": "",
        "worker": "",
        "creator": "",
        "qc_result": "",
        "qc_note": "",
        "sync_feishu": "",
        "coverage_contract": contract,
    }

# ---- 001 concurrency P2 bugfix ----
rec("001", 101, "frp-concurrency-drain-001", "bugfix", "concurrency并发问题",
"""订阅的 webhook 通知出问题了，失败的时候它不重试，已经取消的请求它还在往外发，把分发器关掉之后还能继续往里塞事件直接 panic。代码就在当前目录，跑一下测试就能看到，帮我把这块修好。""",
"""go test ./internal/notify -run '^TestRetryReturnsErrorAfterAllAttempts$' -count=1
go test ./internal/notify -run '^TestRetryHonorsCancelledContext$' -count=1
go test ./internal/notify -run '^TestEnqueueAfterCloseReturnsError$' -count=1
go test ./internal/notify -run '^TestDeliverReturnsErrorOnServerError$' -count=1""",
"""文件: internal/notify/retry.go, internal/notify/webhook.go, internal/notify/dispatcher.go 符号: RetryPolicy.Do, WebhookClient.Deliver, Dispatcher.Enqueue 机制: 重试循环吞掉失败与取消上下文，投递吞掉非 2xx 状态，关闭后入队继续向已关闭 channel 发送导致 panic，错误传播链和 channel 生命周期同时断裂""",
"""1. 失败的重试调用在 3 次尝试后必须返回非空错误；已取消的 ctx 不得触发任何一次回调。
2. 修复后连续 20 轮跑这组测试全部通过，没有一次闪失败。
3. 回退重试或关闭检查里的任何一处改动，对应命令立刻重新变红。
4. 全量 go test 无回归，只验证公开行为不关心实现写法。""",
{"version":1,"claims":[
 {"id":"retry_returns_last_error","description":"重试耗尽后返回非空错误","test":"TestRetryReturnsErrorAfterAllAttempts","command":"go test ./internal/notify -run '^TestRetryReturnsErrorAfterAllAttempts$' -count=1","paths":["internal/notify/retry.go"]},
 {"id":"retry_honors_cancel","description":"已取消上下文不触发任何回调","test":"TestRetryHonorsCancelledContext","command":"go test ./internal/notify -run '^TestRetryHonorsCancelledContext$' -count=1","paths":["internal/notify/retry.go"]},
 {"id":"enqueue_after_close_errors","description":"关闭后入队返回错误而非 panic","test":"TestEnqueueAfterCloseReturnsError","command":"go test ./internal/notify -run '^TestEnqueueAfterCloseReturnsError$' -count=1","paths":["internal/notify/dispatcher.go"]},
 {"id":"deliver_reports_server_error","description":"5xx 响应必须返回错误","test":"TestDeliverReturnsErrorOnServerError","command":"go test ./internal/notify -run '^TestDeliverReturnsErrorOnServerError$' -count=1","paths":["internal/notify/webhook.go"]}]})

# ---- 002 context P5 diagnosis ----
rec("002", 102, "frp-context-cancel-002", "diagnosis", "context相关问题",
"""过期扫描这里好怪，我明明把 ctx 取消了，ScanAll 还是继续往后扫，ScanActive 也一样停不下来。文件先不要改，帮我查清楚是哪里的问题。""",
"""go test ./internal/scheduler -run '^TestScanAllHonorsCancelledContext$' -count=1
go test ./internal/scheduler -run '^TestScanActiveHonorsCancelledContext$' -count=1""",
"""文件: internal/scheduler/expiry.go, internal/scheduler/window.go 符号: ExpiryScanner.ScanGroup, ScanActive 机制: 扫描循环不检查 ctx.Err 继续处理后续分组，取消传播在 ScanGroup 与 ScanActive 两处被切断，导致取消信号无法中止扫描""",
"""1. 已取消的 ctx 跑 ScanAll 和 ScanActive 都要立刻返回 context.Canceled，不能继续扫后面的分组。
2. 全程不得改任何代码文件，产生零代码差异。
3. 结论要落到具体文件、符号和取消传播被切断的位置。
4. 解释清楚为什么两个入口都停不下来。""",
{"version":1,"claims":[
 {"id":"scan_all_honors_cancel","description":"ScanAll 在 ctx 取消后返回 Canceled","test":"TestScanAllHonorsCancelledContext","command":"go test ./internal/scheduler -run '^TestScanAllHonorsCancelledContext$' -count=1","paths":["internal/scheduler/expiry.go"]},
 {"id":"scan_active_honors_cancel","description":"ScanActive 在 ctx 取消后返回 Canceled","test":"TestScanActiveHonorsCancelledContext","command":"go test ./internal/scheduler -run '^TestScanActiveHonorsCancelledContext$' -count=1","paths":["internal/scheduler/window.go"]}]})

# ---- 003 slice P6 bugfix ----
rec("003", 103, "frp-slice-alias-003", "bugfix", "slice相关问题",
"""导入配置时 SortedConfigs 会把原来的 Configs 顺序也改了，导出的 BuildExport 还会漏掉不是 published 的版本。你直接跑下测试会挂，帮我修一下。""",
"""go test ./internal/importexport -run '^TestSortedConfigsDoesNotMutateOriginal$' -count=1
go test ./internal/importexport -run '^TestBuildExportIncludesAllVersions$' -count=1""",
"""文件: internal/importexport/importer.go, internal/importexport/exporter.go 符号: Document.SortedConfigs, BuildExport 机制: 排序直接改原切片的底层数组造成别名污染，导出用 s[:0] 原地压缩丢掉非 published 版本，两处共享底层数组互相串数据""",
"""1. SortedConfigs 返回排序结果后原来的 Configs 顺序一点不变；BuildExport 必须包含每个 key 的全部版本。
2. 修复后连续 20 轮跑这两条测试全部通过。
3. 回退排序或导出的任一修复点，对应测试立刻重新变红。
4. 全量 go test 无回归，只验证公开行为。""",
{"version":1,"claims":[
 {"id":"sorted_configs_no_mutation","description":"排序不得改动原始切片","test":"TestSortedConfigsDoesNotMutateOriginal","command":"go test ./internal/importexport -run '^TestSortedConfigsDoesNotMutateOriginal$' -count=1","paths":["internal/importexport/importer.go"]},
 {"id":"export_keeps_all_versions","description":"导出包含全部版本不丢非 published 状态","test":"TestBuildExportIncludesAllVersions","command":"go test ./internal/importexport -run '^TestBuildExportIncludesAllVersions$' -count=1","paths":["internal/importexport/exporter.go"]}]})

# ---- 004 nil P4 bugfix (with stack) ----
rec("004", 104, "frp-nil-map-004", "bugfix", "nil相关问题",
"""采样器一 Add 就崩，报 nil 指针，聚合 Summaries 也报 assignment to entry in nil map。贴一段关键栈：\npanic: assignment to entry in nil map [recovered, repanicked]\ngoroutine 3 [running]:\ngithub.com/example/feature-release-control-plane/internal/metrics.(*Aggregator).Summaries(...)\n\tinternal/metrics/aggregator.go:52\ngithub.com/example/feature-release-control-plane/internal/metrics.TestSummariesReturnsMetricsForEachRelease(...)\n\tinternal/metrics/aggregator_test.go:13\n麻烦帮我修好。""",
"""go test ./internal/metrics -run '^TestReservoirAddDoesNotPanic$' -count=1
go test ./internal/metrics -run '^TestReservoirCountDoesNotPanic$' -count=1
go test ./internal/metrics -run '^TestSummariesReturnsMetricsForEachRelease$' -count=1
go test ./internal/metrics -run '^TestStoreReservoirDoesNotPanic$' -count=1""",
"""文件: internal/metrics/sampler.go, internal/metrics/aggregator.go, internal/metrics/store.go 符号: Sampler.NewReservoir, Aggregator.Summaries, Store.Reservoir 机制: 零值构造让 reservoir 的随机源/items/counts 与 store 的 map 都保持 nil，写入 nil map 与解引用 nil 指针直接 panic""",
"""1. Reservoir 的 Add 和 Count 不再 panic，Store.Reservoir 首次取用也不 panic；Summaries 正常返回各 release 的汇总。
2. 修复后连续 20 轮跑这四条测试全部通过。
3. 回退任意一处初始化改动，对应测试立刻重新 panic 变红。
4. 全量 go test 无回归，只验证公开行为。""",
{"version":1,"claims":[
 {"id":"reservoir_add_no_panic","description":"Reservoir.Add 不再 nil 解引用","test":"TestReservoirAddDoesNotPanic","command":"go test ./internal/metrics -run '^TestReservoirAddDoesNotPanic$' -count=1","paths":["internal/metrics/sampler.go"]},
 {"id":"reservoir_count_no_panic","description":"Reservoir.Count 不再写 nil map","test":"TestReservoirCountDoesNotPanic","command":"go test ./internal/metrics -run '^TestReservoirCountDoesNotPanic$' -count=1","paths":["internal/metrics/sampler.go"]},
 {"id":"summaries_no_panic","description":"Summaries 不再向 nil map 写入","test":"TestSummariesReturnsMetricsForEachRelease","command":"go test ./internal/metrics -run '^TestSummariesReturnsMetricsForEachRelease$' -count=1","paths":["internal/metrics/aggregator.go"]},
 {"id":"store_reservoir_no_panic","description":"Store.Reservoir 首次取用不再 panic","test":"TestStoreReservoirDoesNotPanic","command":"go test ./internal/metrics -run '^TestStoreReservoirDoesNotPanic$' -count=1","paths":["internal/metrics/store.go"]}]})

# ---- 005 error P3 bugfix ----
rec("005", 105, "frp-error-sentinel-005", "bugfix", "error异常错误",
"""限流触发的时候拿到的 error 用 errors.Is 判断根本不是 ErrLimited，桶里没 token 的时候干脆返回 nil。帮我修掉。""",
"""go test ./internal/ratelimit -run '^TestBucketAllowReturnsErrLimited$' -count=1
go test ./internal/ratelimit -run '^TestRegistryAllowReturnsErrLimited$' -count=1
go test ./internal/ratelimit -run '^TestResetUnknownKeyReturnsFalse$' -count=1""",
"""文件: internal/ratelimit/bucket.go, internal/ratelimit/registry.go 符号: Bucket.Allow, Registry.Allow, Registry.Reset 机制: 桶内无 token 时返回 nil 错误，注册表用 %v 包一层丢掉了 ErrLimited 哨兵，Reset 对未知 key 也返回成功，错误链断裂且状态被误报""",
"""1. 第二次 Allow 必须返回 false 且 errors.Is(err, ErrLimited) 为 true；Reset 不存在的 key 要返回 false。
2. 修复后连续 20 轮跑这三条测试全部通过。
3. 回退错误包装或 Reset 判定的任一修复，对应测试立刻重新变红。
4. 全量 go test 无回归，只验证公开行为。""",
{"version":1,"claims":[
 {"id":"bucket_allow_sentinel","description":"桶耗尽时返回 ErrLimited","test":"TestBucketAllowReturnsErrLimited","command":"go test ./internal/ratelimit -run '^TestBucketAllowReturnsErrLimited$' -count=1","paths":["internal/ratelimit/bucket.go"]},
 {"id":"registry_allow_sentinel","description":"注册表限流错误可被 errors.Is 识别","test":"TestRegistryAllowReturnsErrLimited","command":"go test ./internal/ratelimit -run '^TestRegistryAllowReturnsErrLimited$' -count=1","paths":["internal/ratelimit/registry.go"]},
 {"id":"reset_unknown_reports_false","description":"Reset 未知 key 返回 false","test":"TestResetUnknownKeyReturnsFalse","command":"go test ./internal/ratelimit -run '^TestResetUnknownKeyReturnsFalse$' -count=1","paths":["internal/ratelimit/registry.go"]}]})

# ---- 006 defer P7 diagnosis ----
rec("006", 106, "frp-defer-swallow-006", "diagnosis", "defer相关问题",
"""租约这块 Release 陈旧 owner 的时候不报错，Store.Put 写同版本也不报错。代码先别动，帮我查清楚原因。""",
"""go test ./internal/fencing -run '^TestReleaseStaleOwnerFails$' -count=1
go test ./internal/fencing -run '^TestPutRejectsStaleVersion$' -count=1""",
"""文件: internal/fencing/lease.go, internal/fencing/store.go 符号: Manager.Release, Store.Put 机制: 命名返回值配合 defer 把业务错误覆盖成 nil，Put 的解锁 defer 也把 stale 错误吞掉，defer 清理逻辑把原始错误静默丢弃""",
"""1. Release 用陈旧 owner/version 释放必须返回 ErrStaleLease，Put 写同版本也必须返回 ErrStaleLease。
2. 全程不得改任何代码文件，产生零代码差异。
3. 结论要落到具体文件、符号和吞错的那段 defer。
4. 解释清楚为什么两个入口的错误都变成了 nil。""",
{"version":1,"claims":[
 {"id":"release_stale_owner_fails","description":"陈旧 owner 释放返回 ErrStaleLease","test":"TestReleaseStaleOwnerFails","command":"go test ./internal/fencing -run '^TestReleaseStaleOwnerFails$' -count=1","paths":["internal/fencing/lease.go"]},
 {"id":"put_stale_version_fails","description":"同版本 Put 返回 ErrStaleLease","test":"TestPutRejectsStaleVersion","command":"go test ./internal/fencing -run '^TestPutRejectsStaleVersion$' -count=1","paths":["internal/fencing/store.go"]}]})

# ---- 007 concurrency P1 diagnosis (with race stack) ----
rec("007", 107, "frp-concurrency-lock-007", "diagnosis", "concurrency并发问题",
"""并发跑 Published 和 PutVersion 一直报 data race，贴一段：\n==================\nWARNING: DATA RACE\nRead at 0x00c00019b5f0 by goroutine 8:\n  github.com/example/feature-release-control-plane/internal/repository.(*Memory).Published()\n      internal/repository/memory.go:179\nPrevious write at 0x00c00019b5f0 by goroutine 9:\n  github.com/example/feature-release-control-plane/internal/repository.(*Memory).PutVersion()\n      internal/repository/memory.go:132\n不要改代码，帮我查清楚哪里没锁住。""",
"""go test -race ./internal/repository -run '^TestMemoryConcurrentReadsAndWrites$' -count=1
go test -race ./internal/application -run '^TestServiceConcurrentEmitAndSubscribe$' -count=1""",
"""文件: internal/repository/memory.go, internal/application/service.go 符号: Memory.Published, Memory.List, Service.emit 机制: Published/List 读路径不加读锁直接迭代内部 map，emit 广播订阅者时不加 streamMu 锁，并发写与读共享 map 产生竞态""",
"""1. -race 下跑这两条并发测试必须稳定报 DATA RACE，并指向 Published/List 和 emit 的共享 map。
2. 全程不得改任何代码文件，产生零代码差异。
3. 结论要落到具体文件、符号和缺失锁的读路径。
4. 解释清楚竞态为什么只在并发时出现。""",
{"version":1,"claims":[
 {"id":"memory_race_reported","description":"并发读写 Memory 稳定报竞态","test":"TestMemoryConcurrentReadsAndWrites","command":"go test -race ./internal/repository -run '^TestMemoryConcurrentReadsAndWrites$' -count=1","paths":["internal/repository/memory.go"]},
 {"id":"service_race_reported","description":"并发 emit 与 subscribe 稳定报竞态","test":"TestServiceConcurrentEmitAndSubscribe","command":"go test -race ./internal/application -run '^TestServiceConcurrentEmitAndSubscribe$' -count=1","paths":["internal/application/service.go"]}]})

# ---- 008 slice P6 variant bugfix ----
rec("008", 108, "frp-slice-compaction-008", "bugfix", "slice相关问题",
"""HealthWindow 的 Snapshot 拿完以后再 Record 会把已经拿到的快照冲掉，SortAuditByTime 还会把传进去的数组顺序改了。你跑下测试就能看到，帮我修好。""",
"""go test ./internal/releasedomain -run '^TestSnapshotIsolationAfterCompaction$' -count=1
go test ./internal/application -run '^TestSortAuditByTimeDoesNotMutateInput$' -count=1""",
"""文件: internal/releasedomain/analytics.go, internal/application/operations.go 符号: HealthWindow.Snapshot, SortAuditByTime 机制: Snapshot 返回共享底层数组的子切片，后续 Record 原地压缩把已返回快照的数据冲掉；SortAuditByTime 直接排序调用方切片造成底层数组污染""",
"""1. Snapshot 返回的快照在后续 Record 压缩后保持不变；SortAuditByTime 不改动调用方传入的切片。
2. 修复后连续 20 轮跑这两条测试全部通过。
3. 回退快照隔离或排序拷贝的任一修复，对应测试立刻重新变红。
4. 全量 go test 无回归，只验证公开行为。""",
{"version":1,"claims":[
 {"id":"snapshot_isolated_after_compaction","description":"压缩后快照不被冲掉","test":"TestSnapshotIsolationAfterCompaction","command":"go test ./internal/releasedomain -run '^TestSnapshotIsolationAfterCompaction$' -count=1","paths":["internal/releasedomain/analytics.go"]},
 {"id":"sort_audit_no_mutation","description":"排序不改动输入切片","test":"TestSortAuditByTimeDoesNotMutateInput","command":"go test ./internal/application -run '^TestSortAuditByTimeDoesNotMutateInput$' -count=1","paths":["internal/application/operations.go"]}]})

# ---- 009 error P3 variant bugfix ----
rec("009", 109, "frp-error-wrap-009", "bugfix", "error异常错误",
"""依赖图有环的时候 DependenciesChecked 返回的 error 用 errors.Is 判不到 ErrInvalidValue，空 subject 的 EvaluateChecked 也一样。帮我修一下。""",
"""go test ./internal/configdomain -run '^TestDependenciesCheckedReportsCycle$' -count=1
go test ./internal/configdomain -run '^TestEvaluateCheckedRejectsEmptySubject$' -count=1""",
"""文件: internal/configdomain/graph.go, internal/configdomain/targeting.go 符号: Graph.DependenciesChecked, EvaluateChecked 机制: 返回错误时用 %v 而非 %w 包装 ErrInvalidValue，哨兵错误在错误链中断开，errors.Is 无法识别""",
"""1. DependenciesChecked 遇到环、EvaluateChecked 遇到空 subject，都要返回可被 errors.Is(err, ErrInvalidValue) 识别的错误。
2. 修复后连续 20 轮跑这两条测试全部通过。
3. 回退任一 %w 包装改动，对应测试立刻重新变红。
4. 全量 go test 无回归，只验证公开行为。""",
{"version":1,"claims":[
 {"id":"dependencies_checked_cycle_wrapped","description":"环错误可被 errors.Is 识别","test":"TestDependenciesCheckedReportsCycle","command":"go test ./internal/configdomain -run '^TestDependenciesCheckedReportsCycle$' -count=1","paths":["internal/configdomain/graph.go"]},
 {"id":"evaluate_checked_empty_subject_wrapped","description":"空 subject 错误可被 errors.Is 识别","test":"TestEvaluateCheckedRejectsEmptySubject","command":"go test ./internal/configdomain -run '^TestEvaluateCheckedRejectsEmptySubject$' -count=1","paths":["internal/configdomain/targeting.go"]}]})

# ---- 010 其他 P8 diagnosis ----
rec("010", 110, "frp-state-retrying-010", "diagnosis", "其他问题",
"""重试状态的任务从列表里消失了，Retrying 之后也回不到 Running。文件先不要改，帮我查清楚原因。""",
"""go test ./internal/releasedomain -run '^TestReleaseTransitionIncludesRetrying$' -count=1
go test ./internal/releasedomain -run '^TestMemoryRepositoryUpdatePersistsState$' -count=1
go test ./internal/application -run '^TestListReleasesIncludesRetrying$' -count=1""",
"""文件: internal/releasedomain/model.go, internal/releasedomain/repository.go, internal/application/read.go 符号: Release.Transition, MemoryRepository.Update, Service.ListReleases 机制: 状态机转换表漏掉 Retrying 到 Running 的边，Update 写回旧状态不持久化新状态，ListReleases 过滤掉 Retrying 导致列表可见性与状态流转错位""",
"""1. Retrying 必须能流转到 Running，Update 后 Get 读到的状态是写入的新状态，列表也要包含 Retrying 任务。
2. 全程不得改任何代码文件，产生零代码差异。
3. 结论要落到具体文件、符号和状态机三处错位。
4. 解释清楚任务为什么会从列表消失。""",
{"version":1,"claims":[
 {"id":"retrying_transition_allowed","description":"Retrying 可流转到 Running","test":"TestReleaseTransitionIncludesRetrying","command":"go test ./internal/releasedomain -run '^TestReleaseTransitionIncludesRetrying$' -count=1","paths":["internal/releasedomain/model.go"]},
 {"id":"update_persists_state","description":"Update 持久化新状态","test":"TestMemoryRepositoryUpdatePersistsState","command":"go test ./internal/releasedomain -run '^TestMemoryRepositoryUpdatePersistsState$' -count=1","paths":["internal/releasedomain/repository.go"]},
 {"id":"list_includes_retrying","description":"列表包含 Retrying 任务","test":"TestListReleasesIncludesRetrying","command":"go test ./internal/application -run '^TestListReleasesIncludesRetrying$' -count=1","paths":["internal/application/read.go"]}]})

for k,v in R.items():
    (OUT/f'collection_{k}.json').write_text(json.dumps(v, ensure_ascii=False, indent=2)+'\n')
print("wrote", len(R), "files")
