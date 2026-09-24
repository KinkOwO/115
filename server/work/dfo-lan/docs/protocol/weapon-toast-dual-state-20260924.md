# 进城武器未装备提示双态（2026-09-24）

状态：**attempt 2/3，用户实机确认修复通过**。本记录和候选版作为武器提示双态的确认基线；上一次是 2026-09-23 在 NOTI24 前添加 NOTI14 穿戴刷新。

## 依据与本次假设

配套技术记录 `docs/todo/修炼场修复/TECH-20260924-training-room-entry-fixes.md` §2 给出的当前客户端判定点 `0x146cfdd20` 从玩家穿戴槽 12 读取武器；非空时不应弹 `Weapon not equipped.`。已有入城顺序将 NOTI14 前置，但并未在这段前置帧发送带完整穿戴绑定的 `AppearanceProbe`，且无武器时也执行前置刷新。此次只验证这一处分支。

## 修改

- `inventory.HasWornWeapon` 从角色完整存档的 worn 列表判断槽 12 的非零模板，不将背包装备槽 12 当作已穿武器。
- 有武器时，在 NOTI24 前依次发送 `AppearanceProbe`、穿戴槽 NOTI14、穿戴窗口 NOTI14。
- 无武器时不发送上述前置帧，让客户端继续按空槽显示原生提示。入场完成后的装备与外观刷新保留原顺序。
- 不涉及数据库结构、存档写入或客户端文件。

## 代码检查与验收

- `go test ./...`、`go vet ./...`：通过。
- 候选版 `bin/wireprobe-handoff-source.exe` 已编译，SHA-256：`520927BE6285789B0D470CBE28E0DB3F129C164C1251F20DF37DC8CF4CB7C257`。
- 用户确认实机修复通过。
- 候选程序 SHA-256：`520927BE6285789B0D470CBE28E0DB3F129C164C1251F20DF37DC8CF4CB7C257`。

## 回滚

回退本次 `entry_flow.go`、`main.go`、`internal/inventory/wear.go` 及关联检查文件的改动，即恢复 2026-09-23 的入城前置帧行为。
