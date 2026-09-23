# CMD331 技能指令改键：confirmed baseline（2026-09-23）

## 目标与边界

目标是让技能窗口内修改指令键、点击保存、关闭窗口重开及重选角色后都保持新键位。开始取证时源码没有 `internal/character/skill_commands.go` 和 CMD331 接收分支；只在 `State` 添加 `SkillCommands []byte` 不会实现此功能。当前源码候选已经两次迭代并通过实机确认，C2S 尝试计数为 **2/3**。

## 已确认事实

- 当前 115 客户端 opcode 表把 CMD331 / `0x014B` 命名为 `ENUM_CMDPACKET_SKILL_COMMAND_CUSTOMIZING`，CMD332 / `0x014C` 命名为 `ENUM_CMDPACKET_SKILL_COMMAND_ALL_DEFAULT`（`analysis/dumps/opcodes.tsv`）。
- 2026-09-22 旧会话在入场 NOTI19 后先发 CMD332 空正文，接着发 CMD331，解密正文为 16 字节全零；不能将这条全零包解释成一次玩家改键。
- 2026-09-23 用户在技能窗口将图中技能的指令改为 Space 并保存后，服务端记录 CMD331，正文 `01460001080000000000000000000000`（16 字节）。同一会话随后又记录相同正文一次。来源：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260923_143154_311234_next37/events.jsonl` 第 195–196 行；截图由用户提供。
- 同一会话后续两次手动保存分别产生 `01460001080000000000000000000000` 与 `02080001044600010800000000000000`（第 2321–2322 行）。后一条可按 `count=2`、两组四字节记录 `08000104` / `46000108`、零填充来切分；前一条可按 `count=1`、记录 `46000108` 切分。旧记录仍包含于新包，表明客户端发送当前改键列表快照。
- 用户把同一技能改成多键组合后，明文为 `01460003000004000000000000000000`（第 3051 行）：`count=1`、技能 ID `0x0046`、指令数 `3`、指令字节 `00 00 04`。因此条目是可变长度 `u16 skill ID + u8 token count + token bytes`，末尾零为传输块填充。
- 当前服务端 NOTI19 的每条技能消息有重复的 protobuf field 4 `uint32` 指令向量（`internal/game/protocol/skills.go`）；`internal/character/learning.go` 原先仅从职业目录的 `SkillCommands` 复制该向量。CMD331 保存的指令经该向量恢复后，用户实机确认重选角色能回显 Space。
- IDB 临时副本中的 `sub_1401A3500` 确认 NOTI19 技能行 field 4 为重复的 protobuf `uint32`。`sub_1452E6C50` 遍历该向量并交给 `sub_145EF12D0` 设置技能指令；后者只接受 1–5 个 token，最后一个必须是 4/5/6/8，且只能有一个此类动作键。opcode 名字符串的引用落在通用注册函数；单搜 `331` 立即数混有 UI 控件编号及其他枚举值，不能用来认定 CMD331 发送函数。

## attempt 1/3：源码候选实现

- 接收端按原生样本解析 CMD331 的完整快照，校验条目、技能 ID、指令数、客户端 reader 允许的 token 范围，并剥离传输零填充。
- 快照保存在现有角色 JSON 状态的 `skill_commands` 字段；未调整 PostgreSQL schema，原有字段在更新时合并保留。
- NOTI19 入场投影时，仅对快照中列出的技能覆盖源指令；空快照回退至源指令。
- 不发送未经取证的 CMD331 ACK 或额外 NOTI。`go vet ./...` 通过；`go test ./...` 唯一失败为此前已有的 `TestAppearanceProbeCarriesCloneAndLookAvatars` 重复外观槽位；本任务定向测试及 `cmd/wireprobe` 包测试通过。用户手动实机确认重选角色后能回显 Space，但保存当下的技能窗仍显示旧指令。

## attempt 2/3：保存后立即刷新（实机确认）

- CMD331 保存成功后复用现有技能学习/移动流程的 NOTI19 全技能刷新；该 packet 的客户端 reader 已在本次 IDA 检查中确认会应用 field 4 指令向量。仍不发送未经确认的 CMD331 ACK。
- `go vet ./...` 通过，`go test ./...` 仍仅有上述外观测试失败，`bin/wireprobe-handoff-source.exe` 已重新编译。等待用户手动验证当前技能窗是否立即更新。
- 实机日志 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260923_150832_722465_next37/events.jsonl` 记录 07:09:16 的 CMD331 快照 `02080002010846000108000000000000`，随后依次出现 `skill_commands_saved` 与 `skill_commands_refreshed`（NOTI19）。用户确认改键后当前窗口立即更新，关闭重开及重选角色后仍显示新键位；此行为成为已确认基线。

## 尚缺的证据

1. CMD332 与 CMD331 空列表的关系尚未单独取证；“重置所有指令”暂不纳入此基线。

若实机回显失败，先读取服务端 `skill_commands_saved/rejected` 和入场 NOTI19 日志，并回到 IDA 读取链取证；不要叠加猜测包。
