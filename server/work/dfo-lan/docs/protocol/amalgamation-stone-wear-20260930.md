# 融合石穿戴候选 — 2026-09-30

状态：**2026-09-30 用户确认实机验证通过，纳入 confirmed baseline。** 本轮运行路径变更为 **attempt 1/3**。

## 本次范围

用户要求实施融合石功能；结合前一轮穿戴核查，本次接入 `[amalgamation stone]` 的穿戴、替换、卸下及已有穿戴存档恢复。未实现融合石升级、合成或完整战斗词条效果。

`WearService.wearable` 在 `Rules.Special` 开启时放行融合石目标槽 36–43。槽号取自客户端 CMD19，不把 `[amalgamation part]` 换算成普通防具或首饰槽。44–46、47 及其它范围拒绝融合石。表外类型先检查受限例外，再检查槽位表，修正 `!ok` 导致例外无法生效的短路；保留原有护石、晶体、副手、宠物幻化和光环幻化条件。

物品实例记录、耐久、期限、源配置的等级、职业及转职检查沿用既有流程。110 级融合石不继承 `[oath]` / `[primer]` 的 115 级硬门槛。沿用 CMD19、NOTI13/14 与现有存档事务；没有新增包字段或数据库结构。

## 证据与边界

### 当前工作区实机请求

`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_115741_859224_next37/events.jsonl`：

| 行 | 物品 | 客户端 CMD19 明文 | 解码目标 |
| --- | --- | --- | --- |
| 480 | 100251186 | `00200032b6f905010000000324000000000000000000ffffffff000000000000` | list0 槽32 → list3 槽36 |
| 535 | 100051351 | `00240097a9f605010000000324000000000000000000ffffffff000000000000` | list0 槽36 → list3 槽36 |

对应请求被旧代码以 `equipment does not fit destination slot` 拒绝。2026-09-30 的日志仍有同类拒绝。此次测试直接复用上述字节，经现有 `DecodeItemMove` 解码后调用移动逻辑。

### 当前装备导出

按 `configs/equipment-full.index.json` 的 Offset/Size 读取并解压 `equipment-full.data`，确认：

| ID | 类型 | amalgamation part | 最低等级 | rarity |
| --- | --- | --- | --- | --- |
| 100323404 | `[amalgamation stone]` | `[ring]` | 110 | 8 |
| 100251186 | `[amalgamation stone]` | `[shoes]` | 115 | 3 |
| 100051351 | `[amalgamation stone]` | `[coat]` | 115 | 3 |

`equipment-wear.full-candidate.json` 开启 special，但没有 `[amalgamation stone]` 固定槽映射，因此原 `!ok` 守卫拒绝这些物品。

### 权威 IDB 复核

只打开 `client/DFO.exe.i64` 的 runtime 隔离副本，未修改或保存权威原件。复制前后 SHA-256 相同：

`D5FB35A7C9B06BF7F3DFC46CE3F5354519F9B0C5E05554C15577C6A8EB1A180F`

- `sub_1470CB2A0`，`0x1470CB92A`：`[amalgamation stone]` 类型枚举为 50；`[primer]` 为 36，`[oath]` 为 47。**类型枚举 50 不是穿戴槽号。**
- `sub_145AE55A0`，`0x145AE598B`：物品类型 50 有独立使用分支，调用当前角色的虚函数 +5888；不能按普通部位的默认映射理解。
- `sub_145A6DD30`，`0x145A6DD71..0x145A6DDC1`：晶体空栏选择按 rarity 8 从 44 起，其余从 36 起，到 47 前结束。
- `sub_145A7C8F0`，`0x145A7C9A9..0x145A7CA2D`：晶体替换选择使用相同起点及上界。

**八栏 36–43 的完整范围取自用户提供的 MR 实机记录；本地历史日志直接证明融合石请求槽 36。静态分析未独立闭环融合石专用八栏选择分支。** 用户于 2026-09-30 确认本轮候选验证通过，该确认关闭了本候选的实机验收门禁。用户提供的报告记录了此前一次 15 次成功、1 次预期拒绝；本次工作区未收集到新一轮逐项运行日志，因此不将此前计数记作本轮数据。

## 回归与候选部署

新增 `internal/inventory/wear_amalgamation_test.go`：

- 三件不同部位、等级、稀有度的融合石 × 八栏穿戴，并逐字节检查实例记录、耐久、期限保留。
- 两条本地原生 CMD19 请求回放。
- 范围外、special 关闭、普通/未知类型、等级与职业限制；晶体与誓约边界不变。
- 满八栏后槽46溢出拒绝且原状态不变；已有栏位替换、空源右键卸下、存档读取与穿戴快照编码；保留金币及无关角色状态。

代码测试通过不等同于客户端显示和重登验收。构建只更新 `bin/wireprobe-handoff-source.exe`，未操作其它候选二进制；当前 bin 目录未发现 `wireprobe-dungeon39.exe`。

- 构建：Go 1.26.0、`go build -trimpath`，基于工作区 HEAD `51b0e33` 加本次源码变更（构建元数据 `vcs.modified=true`）。
- 已部署候选 SHA-256：`961AE5A7C8C5E9756D6BE6FB9C7F76655F385DEDE4566D0C973BE48647C09600`。
- 原候选备份：`runtime/amalgamation-wear-20260930/wireprobe-handoff-source.before-fusion-51b0e33.exe`，SHA-256 `BF3B203A92A641AD35F8904B8E5C364170A9F3E4158EA14C745D9916033C8720`。
- 未启动客户端或游戏服务。已有会话须由用户关闭并重新启动，才能使用新候选。

验证结果：全量 `go test ./...` 与 `go vet ./...` 通过。额外执行 `go run ./cmd/charactercheck`，临时隔离 schema 初始化返回 `ERROR: relation "character_quests" does not exist (SQLSTATE 42P01)`；该数据库集成检查未通过，不能计作存档数据库验收。检查器使用随机命名的独立 schema 并在退出时清理，没有修改玩家 schema；本次未修改检查器或迁移代码。
