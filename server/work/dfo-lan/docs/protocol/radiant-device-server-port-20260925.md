# 光辉盒子随机装置：服务端移植记录（2026-09-25）

状态：代码与静态检查完成，待用户手动实机回归。当前工作区没有本次故障的实机日志。

## 依据与边界

- 下游 `docs/todo/修复合并光辉盒子开启/docs/protocol/next78-box-open-fix-20260925.md` 记录了用户实机确认的 CMD681/495/2036 与 NOTI2551 链路；next44、next77 是较早的试验阶段，不复制其旧抽池。
- 当前服务端已有 CMD681 → `OpenBoxes` 的事务、导入奖励表与 NOTI2551 结构。本次只补窗口状态响应及开盒契约即时通知。
- 客户端结果窗调用路径在下游曾依赖客户端二进制改动；本次没有修改或启用客户端补丁，结果窗显示不列入本次服务端验收承诺。

## attempt 1/3：按下游已确认链路接线

1. CMD495 仅识别 8 字节 `{0x10|0x39, 1}` 装置请求，避免截走原有公告回报；CMD2036 仅识别已记录的 24 字节窗口/按钮形状。两者回 `(1,2036)` 的 136 字节状态，前两个 `u32` 来自角色存档中该盒子的 bonus/section 计数，其余字段维持零值。
2. CMD681 继续走现有 `OpenBoxes`，不引入另一套扣盒、抽池或数据库 schema。开盒成功后按背包刷新、NOTI2551、NOTI66 顺序推送；NOTI66 使用事务回执中的契约到期时间换算剩余秒数。
3. CMD2036 按钮形状目前只刷新窗口状态。下游 next78 的已确认开盒路径是 CMD681；若本地客户端只发 CMD2036 而不发 CMD681，需要先收集该次用户手动操作的 `events.jsonl`，再决定是否增加第二条扣盒入口，避免一次点击重复扣盒。

回滚范围：`cmd/wireprobe/main.go`、`cmd/wireprobe/radiant_box_flow.go`、`internal/game/protocol/cerashop_device*.go`、`internal/loot/box_open.go` 与 `internal/loot/box_window_test.go` 中本次改动；没有数据库结构变更。

静态验证：`go test ./...`、`go vet ./...` 通过。候选版已编译至 `bin/wireprobe-handoff-source.exe`，SHA256 为 `15CD7BAC4D6915A593F9975430A84BC495FF6F9553CC147D683E1BC500BB5FD4`；归档 39 版未动。

待实机：用户手动用 `server/Start-DFO.cmd --source-build` 启动候选版，分别试 ×1、×10，核对 CMD495→状态包、CMD681→扣盒和奖励、NOTI66 契约即时显示；如没有 CMD681 或结果窗不显示，保留该会话日志供后续取证。
