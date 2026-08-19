# BUG_REPRO：配置域错误链断裂

## Bug 是什么
configdomain 包 DependenciesChecked/ValidateDependencyGraph/EvaluateChecked/ValidateRules 返回错误时用 %v 而非 %w 包装 ErrInvalidValue，errors.Is 无法识别哨兵错误。

## 如何触发
```sh
go test ./internal/configdomain -run '^TestDependenciesCheckedReportsCycle$' -count=1
go test ./internal/configdomain -run '^TestValidateDependencyGraphReportsCycle$' -count=1
go test ./internal/configdomain -run '^TestEvaluateCheckedRejectsEmptySubject$' -count=1
go test ./internal/configdomain -run '^TestValidateRulesReportsInvalidRule$' -count=1
```

## 错误信息
- `expected wrapped ErrInvalidValue, got invalid configuration value: ...`
