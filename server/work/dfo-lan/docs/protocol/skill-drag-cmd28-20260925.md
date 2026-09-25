# CMD28 技能栏拖拽不刷新技能树（实机确认）

日期：2026-09-25。状态：用户实机验证通过。attempt 1/3。

## 问题与修复

技能拖放与交换由 CMD28 处理。旧响应在 CMD28 成功 ACK 后追加 NOTI19 全量技能树，导致客户端再次刷新技能信息，播放加点音效，并以不含 VP variation 的技能树覆盖当前 VP 面板。

现由 `skillTreeRefreshRequired` 对 CMD28 始终返回不刷新。CMD28 仍按原顺序解码技能树和槽位、调用 `MoveSkill` 保存布局、返回成功 ACK。CMD29 普通加点仍刷新技能树；携带 VP variation 的 CMD29 仍不追加 NOTI19；CMD2179 自动加点后的布局保存和必要刷新、技能重置与登录时技能/VP 恢复均保留。

没有客户端、DLL、PVF、数据库结构或玩家存档改动。

## 回归与实机确认

- `TestSkillTreeRefreshPlan` 覆盖 CMD28 应用和幂等响应均只产生 ACK28、不产生 ID19，并检查 ACK 内容；同时覆盖普通 CMD29 刷新、VP Apply 不追加 ID19、被拒 CMD29 刷新。
- `cmd/charactercheck` 技能持久化检查现在按拖拽前两个槽位的实际技能，确认重开存储后源技能落在目标槽、目标技能（若有）落在源槽。
- 用户确认本次实机验证通关。
- `go test ./...` 与 `go vet ./...` 均通过；统一构建脚本成功构建候选版。
- 候选程序：`bin/wireprobe-handoff-source.exe`，SHA-256：`3D42C7CE6052D9AB710836781EAC3380A5F09D794F449BCA5FCAB0B38C0513D8`。
- 构建前候选程序已备份为 `bin/wireprobe-handoff-source.exe.bak-before-skill-drag-20260925-215417`；归档 39 版未覆盖。

## 回退

停止服务后，可用上述 `.bak-before-skill-drag-*` 文件恢复构建前候选程序。该修复没有数据库迁移，不需要回滚角色存档。
