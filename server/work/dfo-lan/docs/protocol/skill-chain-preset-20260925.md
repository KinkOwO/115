# 技能连招保存与角色切换恢复：confirmed baseline（attempt 1/3，2026-09-25）

> 本文以下实机记录来自 `docs/todo/dnf115-skill-chain-fix-20260925-v2/` 的历史验收包。2026-09-26 已将连招相关源码移植到当前服务端并保留仓库后续功能；`go test ./...` 与 `go vet ./...` 通过，候选版 SHA-256 为 `9B61C16EB4B9DB5C1914542AF9F1E411E252E04C3F2022A2884C838310FE6D79`。用户已确认当前实现保存/恢复成功。

## 当前工作区验收（2026-09-26）

- 用户确认技能连招保存与恢复成功。
- 当前候选版：`bin/wireprobe-handoff-source.exe`，SHA-256 `9B61C16EB4B9DB5C1914542AF9F1E411E252E04C3F2022A2884C838310FE6D79`。
- `go test ./...` 与 `go vet ./...` 通过；角色存档使用向后兼容的可选 JSON 字段，无数据库迁移。
- CMD2347“重置连招”未单独验收，不属于本次确认范围。

## 用户行为与根因

角色 `memo` 配置技能连招后，窗口在当前会话内能显示顺序，但原服务端把 CMD2346 当作未实现请求，角色存档没有连招字段，也没有在重新选角时发送 NOTI2758。客户端内存尚在时会造成“界面似乎保存了”的假象。

连招不是给普通技能增加能力。当前客户端目录中存在独立主动技能 452：`skill/common/skillpresettype1.skl`（`<12::SkillChain1_name>`）。2026-09-25 实机日志的两次 CMD28 为 `18 -> 7`、`7 -> 8`；结合最终数据库快捷栏可逆推出技能 452 从技能栏槽 18 移到快捷栏槽 8。用户确认将该专用技能放到 W 后，副本内连招轮换正常。CMD39 是怪物死亡/掉落请求，不能作为每一步客户端技能施放的编号证据。

## 当前客户端协议闭环

- CMD2346 / `ENUM_CMDPACKET_SAVE_SKILL_PRESET`：发送函数 `0x145ef8270`，固定向请求 writer 写 27 字节。前 13 字节来自临时请求构造区且跨样本变化；稳定配置从 offset 13 开始，共 14 字节。现有两个原生样本的技能 WORD 与 UI 顺序逐项对应。
- CMD2346 回调 `0x145294a70`：以成功标志处理 ACK，不读取连招配置回显。
- NOTI2758 / `ENUM_NOTIPACKET_SKILL_PRESET_INFO`：回调 `0x1452e7e60` 调用原始 reader 精确读取 `0x1c`（28）字节，即两组 14 字节配置；`0x145eeaa70` 对两组上下文分别取技能 452，把四个有符号 WORD 技能 ID 写入其连招数组并恢复延迟状态。
- 配置每组布局：`u16 delay`、四个 `i16 skill_id`、四个原生状态字节。服务端保存并原样恢复完整 14 字节，不猜测或重算尾部状态。

## 服务端实现

- `internal/game/protocol/skill_preset.go`：验证 27 字节语义体与零填充，只提取稳定的 14 字节配置；构造固定 28 字节 NOTI2758；构造 CMD2346 成功 ACK。
- `internal/character/skill_preset.go`：在角色行锁与既有 `character_events` 幂等事务中保存 `skill_preset`。JSON 可选字段保证老存档兼容，不修改 PostgreSQL schema。
- `cmd/wireprobe/skill_flow.go`：接入 CMD2346；保存后只 ACK，避免把 NOTI2758 当作编辑窗口回显引发自动修正循环。
- `cmd/wireprobe/entry_flow.go`：有存档时在 NOTI19 技能树之后发送 NOTI2758；无存档时跳过，保持老角色原行为。
- `cmd/wireprobe/skill_preset_restore.go`：集中实现“技能树后恢复连招”；普通技能学习/退点、技能重置、自动技能、外观触发刷新、结算升级、转职、觉醒和 CMD331 等所有已知 NOTI19 路径都在其后立即追加 NOTI2758。这样客户端重建技能对象后会同步重建技能 452 的连招运行状态。
- 当前服务端已经向客户端投影两个技能上下文，但 CMD2346 没有可靠的上下文标识，因此将同一份保存配置复制到两组 14 字节上下文。这是当前服务端策略，不声称是官方服务器的多上下文存储模型。

## 实机确认基线

候选服务端会话：

`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_201114_624233_next37`

- `2026-09-26T03:11:59.1033193Z`：收到 CMD2346，配置为 `000005002e0009000d0000000000`。
- `2026-09-26T03:11:59.1090432Z`：返回 `skill_preset_saved`，ACK `01`。
- PostgreSQL 角色 `memo` 的 `state.skill_preset` 解码后与上述 14 字节逐字节一致。
- `2026-09-26T03:12:30.7311031Z`：切换角色再进入 `memo`，在 entry 序列发送 28 字节 NOTI2758，日志为 `skill_preset_restored`。
- 用户确认切换角色后连招顺序仍存在；此前同一客户端已确认专用技能 452 放到 W 后副本内连招可用。因此保存、切换角色恢复与执行构成 attempt 1/3 的确认基线。

### 学习技能后的刷新回归与修复

首次验收后发现：学习技能时服务端会发送 NOTI19，客户端由此重建技能对象并清掉技能 452 的内存连招状态；存档没有丢失，因此重新选角收到 NOTI2758 后又会恢复。修复后的会话：

`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_202711_963338_next37`

- `2026-09-26T03:28:22.1098541Z`：同一次普通技能操作依次记录 CMD29 成功响应、NOTI19 技能树、NOTI2758 连招恢复。
- `2026-09-26T03:28:31.1479261Z`：第二次操作再次记录相同的 29 → 19 → 2758 顺序。
- 两次 NOTI2758 都恢复同一份 28 字节双上下文配置；用户确认学习技能后不切换角色，连招仍然正常。根因和刷新时序修复均完成实机闭环。

## 验证与已知边界

- 协议、角色、网关定向测试通过。
- 排除项目运行时备份目录 `runtime/update-backup/**` 后，全部真实 Go 包测试及 `go vet` 通过。裸 `go test ./...` 会把备份中的不完整历史源码当成模块包而失败，这是既有目录边界问题。
- `go run ./cmd/charactercheck` 仍在既有疲劳时钟断言 `reconnect/backwards clock reset fatigue` 退出，与本次可选 JSON 字段无关。
- 实机接受二进制：`bin/wireprobe-handoff-source.exe`，SHA-256 `357D85927AE35EF1012A2F98DF74B9F78BB3C07BBC10238FC9C176C6F2F9E886`。
- 尚未单独验收 CMD2347“重置连招”按钮；本基线只覆盖保存、角色切换恢复和已确认的副本执行。
