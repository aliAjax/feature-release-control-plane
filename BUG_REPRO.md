# BUG_REPRO：导入导出切片别名污染

## Bug 是什么
importexport 包 SortedConfigs 就地排序修改调用方切片；BuildExport 用 s[:0] 原地压缩只保留 Published 版本，丢掉其余版本。

## 如何触发
```sh
go test ./internal/importexport -run '^TestSortedConfigsDoesNotMutateOriginal$' -count=1
go test ./internal/importexport -run '^TestBuildExportIncludesAllVersions$' -count=1
```

## 错误信息
- `SortedConfigs mutated the original slice`
- `expected both versions in export, got 1`
