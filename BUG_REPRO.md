# BUG_REPRO：defer 吞错

## Bug 是什么
fencing 包 Store.Put/Delete 和 Manager.Release/Renew 用命名返回值配合 defer 把业务错误覆盖成 nil，陈旧版本/owner 的写入与释放全部静默成功。

## 如何触发
```sh
go test ./internal/fencing -run '^TestPutRejectsStaleWrite$' -count=1
go test ./internal/fencing -run '^TestDeleteStaleOwnerFails$' -count=1
go test ./internal/fencing -run '^TestReleaseStaleOwnerFails$' -count=1
go test ./internal/fencing -run '^TestRenewStaleVersionFails$' -count=1
```

## 错误信息
- `stale Put should fail, got <nil>`
- `deleting with a stale owner should fail, got <nil>`
- `releasing with a stale owner should fail, got <nil>`
- `renewing a stale version should fail, got <nil>`
