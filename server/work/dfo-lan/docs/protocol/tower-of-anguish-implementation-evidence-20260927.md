# Tower of Anguish（Tower of Grief）实现取证，2026-09-27

## 当前实机阻断点

- 2026-09-27 00:41:56（北京时间）会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260927_003726_005416_next37`：客户端 CMD16 明文以 `fb130000` 开头，即副本 5115；服务端下一条记录 `dungeon_request_refused`，原因 `no resolved source maze for requested quest`。会话 `events.jsonl` 第 429～430 行。
- `catalog.ParseDungeon` 仅从 `[map specification]` 产生普通房间；5115 没有该节，于是迷宫 `Pending=["unsupported room specification"]`。`dungeon.Select` 跳过 Pending，最终返回上述错误。客户端没有进入本层，所以目前没有本层通关或下一层的实机帧。

## 当前 PVF 的源映射

源归档为 `server/work/client-build/Script.inner.pvf`，与 `configs/dungeons.full.json` 的 checksum `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80` 对应。以下文件均从该归档只读导出；工作副本在 `.tmp/tower-analysis-20260927/`。

- `etc/towerofgrief.etc` 的 `[top layer]` 为 100；`[each layer matching dungeon]` 明确列出 1→5115、2→5116、……、100→5214。另有 `[enterable count] 2`、`[account enterable max count] 1` 和各层奖励规则；这些字段的日切、账户范围及领奖时序尚未验证。
- `dungeon/towers/grief/5115_001.dgn` 到 `5214_100.dgn` 共 100 个副本，每个迷宫为 1×1，起点、Boss 坐标均为 (0,0)，没有 `[map specification]`。它们引用 `Dungeon/Towers/table/TowerOfGrief.tbl`，该表是 APC 数值修正表，不能从中推导楼层地图。
- PVF 中有恰好 100 张 `map/towerofgrief_down/100247_001.map` 至 `map/towerofgrief_up/100346_100.map`。逐张检查 `[dungeon]`：第 N 张地图的归属全部为 5114+N，100/100 匹配。第 1、2 层均含 `[ai character]` 敌方 Boss 行。当前 `dungeons.full.json` 未导入这 100 张地图，因为现有导入器只跟踪普通迷宫显式房间引用。
- `Tower of Dazzlement` 是另一组独立的副本 ID 和资源（7601 起）；不能与本塔共用未经验证的进度协议。

## 服务端可实施的第一阶段

1. 新增只作用于 `TowerOfGrief` 的源数据导入/加载覆盖层，模式可参考 `catalog.AttachTournamentQuestMaps`。从原 PVF 的 `etc/towerofgrief.etc`、`list/map.lst` 和每张 MAP 的 `[dungeon]` 建立 ID 映射；严格校验归档 checksum、100 层唯一性、地图 owner、SHA、1×1 起点/Boss 坐标。不依赖 `5115+N` 或 `100247+N` 作为运行时猜测公式。
2. 为每层补一间已解析房间并装入对应地图脚本，使 `dungeon.Select` 与 `newSession` 走现有选图、APC 生成和加载流程。须在加载 `dungeons.full.json` 重新解析 DGN **之后**附着覆盖层，否则 `LoadDungeons` 会覆盖房间结果。保持其 `source.checksum` 不变，不失效已有任务存档。
3. 用源数据做离线校验：100 层 owner 全匹配、首层/次层均能构造 Session 和 NOTI29 地图、源 APC 能解析；同时保证普通副本目录不变。此阶段只解决“CMD16 被拒”，不宣称客户端已能入图或通关。

## 第二阶段仍缺的客户端证据

- 本塔首层收到普通进图序列（CMD16 ACK、NOTI28、NOTI29）后，客户端是否还等待塔专用通知；要在当前 115 IDB 追完对应 reader、字段宽度、分支和消费时序，并由用户手动实机验证。
- 首层 Boss 死亡后实际会发哪些 C2S 帧（例如通关、结算或选下一层），客户端如何接收下一层状态。opcode 名表中有 CMD159 `DEATH_TOWER_STAGE_CMD`、NOTI929 `TOWER_OF_DESPAIR_INFO`、NOTI1286 `ACCOUNT_TOWER_DUNGEON`、NOTI1291 `TOWER_DUNGEON_CLEAR_INFO`、NOTI1319 `GRIEF_TOWER_COME_OVER_EVENT`；名称只提供搜索入口，不能当作本塔实际使用证明。NOTI1286 至少有两个注册回调：`sub_1452F9420 → sub_1452C5260` 连续读四个字节，复制成 DWORD 状态；`sub_143EF1660 → sub_143EF0D20` 读取一个 DWORD 并据值触发提示。已确认两个 handler 各消费四字节，完整包长尚无原生向量；与本塔入场/切层的关系未闭环。
- 进度、入场次数和奖励需先确认客户端状态语义，再设计 PostgreSQL 事务、日切及无损迁移；不能仅凭 `etc` 字段直接扣次数或发奖励。

## 入场候选版：attempt 1/3

- 服务端新增 `dungeons.tower-of-grief-maps.json` 源匹配覆盖层：由当前 PVF 的楼层表、DGN 脚本和 100 张 MAP 的 `[dungeon]` owner 生成；加载时校验归档 checksum、每层 DGN SHA、地图 owner、唯一性、1×1 迷宫形状。只为 Tower of Grief 附加房间，不改数据库、客户端和其他副本。导出命令：`go run ./cmd/towergriefimport -source ../client-build/Script.inner.pvf -output configs/dungeons.tower-of-grief-maps.json`。
- 候选程序 `bin/wireprobe-handoff-source.exe` 已更新；SHA-256 `33954DBDA9BCD3978BC410CEEB35ED12BBE0C825CB8BC19D5A8B49A7A32C8A50`。前一候选程序保留在 `.tmp/tower-analysis-20260927/wireprobe-handoff-source-before-tower.exe`。
- `TestTowerGriefSourceEntry` 验证原目录拒绝 5115，附加覆盖层后 5115、5116、5214 均能建立 Session、解析敌方 APC Boss 并编码 NOTI29。`go test ./...`、`go vet ./...` 和启动器只读 `--check --source-build` 均通过。
- **实机待验**：由用户手动运行候选版，点击 Tower of Anguish 第 1 层，记录是否成功入场、是否加载敌人；若能完成首层，再采集后续 CMD 与服务端回包。当前尚未实现楼层进度或下一层结算，不对未知客户端分支叠包。

## 首层 Boss 死亡后卡住：实机回报与取证

- 用户手动测试候选版，已成功进入 5115 首层、加载敌方 APC Boss。截图中倒地的是敌方 APC；玩家角色仍站立，所以这是 Boss 死亡后的通关结算卡住，不是玩家死亡结算。
- 会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260927_012225_417518_next37/events.jsonl`：237～254 行记录 CMD16/NOTI28/NOTI29 和 CMD37，证明入场覆盖层生效；292 行 CMD117，293 行 CMD39 确认 Boss 死亡，298 行 NOTI115，299 行 NOTI31；304 行客户端 CMD46，305～308 行服务端依次回普通副本 NOTI34、37、35、261。之后至 316 行没有 CMD159、下一层请求或结算退出。服务端没有拒绝 CMD46，也没有记录结算函数错误。
- 115 客户端 IDB 只读副本 `DFO-tower-analysis-20260927.i64`：`sub_1452BAD10` 注册 NOTI142→`sub_1452A9330`、NOTI143→`sub_1452B6A40`，并注册 CMD159 回调；NOTI142 reader 读取 4+2+1+1 字节，NOTI143 读取楼层地图/怪物等变长数据。`sub_1452F9420` 注册 NOTI1291→`sub_1452E91C0`，其 reader 顺序读取两个 1 字节值；首字节为 6 或 51 时分别取得一个塔相关状态对象，再把第二字节解释为布尔值写到对象 +672。
- 另一组 NOTI1319/CMD1389 在 `sub_1442A6B90` 注册，NOTI1319 handler `sub_1442A6B00` 读取 21 字节并驱动事件弹窗；不能据名称把它当首层通关结算。
- **当前判断**：普通地图/普通结算路径已经跑通，但塔专用初始化、清层、下一层状态机仍未闭环。下一步需从 115 客户端确认塔通知与普通 NOTI29/31/34 的时序，以及通关后应由哪个客户端动作请求下一层。未完成之前不发猜测的塔通知。入场候选仍计 `attempt 1/3`，本次仅取证，未增加试包次数。

## 结算通知归属确认（同日继续分析）

- 当前 DGN 5115 的源脚本明确带 `[tower of grief]`。115 客户端 `sub_145A38DF0` 将模块号 11 映射到 `MODULE_TYPE_DESPAIR_TOWER`，模块号 13 映射到 `MODULE_TYPE_GRIEF_TOWER`。`sub_1452FC5B0` 的分支又将模块号 11 路由到 `sub_145241C10`（状态指针 +360），模块号 13 路由到 `sub_140E54C40`（状态指针 +376）。
- NOTI1291 handler `sub_1452E91C0` 中，首字节 **6** 走 +360（Despair），首字节 **51 / 0x33** 走 +376（Grief）；第二字节非零写入该对象 +672 的布尔状态。Tower of Grief 的明确客户端 reader 输入为 `NOTI1291`（`ENUM_NOTIPACKET_TOWER_DUNGEON_CLEAR_INFO`），body 两字节 `{0x33, 0x01}` 会把该布尔状态设为真。`sub_1415D4740` 在 UI 对象处理时读取 +672，调用 UI 控件状态 setter；同一 setter 也被其他非塔 UI 使用，因此 +672 更准确地说是 UI 控件状态，尚不能证明是楼层进度或结算状态。单发它能否弹结算面板或切下一层也未证明。
- NOTI142/143/144/145/146 注册在 `sub_1452BAD10` 的 Death Tower 处理组；NOTI142/143 操作 `sub_145241C00` 返回的 +344 状态对象，而 `sub_1452FC5B0` 将 +344 路由到模块号 9。不能把这组包仅凭“爬塔”外形用到模块号 13 的 Tower of Grief。NOTI929 的名称是 Tower of Despair，NOTI1319 属活动事件弹窗，均不是目前可证的本塔通关主通知。
- **尚缺时序证据**：NOTI1291 应在 Boss 死亡确认（CMD39）之后、NOTI31 之前，还是在 CMD46 之后；是否还需普通 NOTI34/35，及如何触发下一层，当前 IDB 分支和本次实机日志尚未闭环。不能直接追加猜测消息进入实机候选。

## 参考端实现对照（2026-09-27）

| 参考端 | 实际覆盖范围 | 入塔、清层与结算 | 对当前 115 的用途 |
| --- | --- | --- | --- |
| `../ServerS4A21` | `DeathTowerData.TryLoadFromCatalog` 仅接受 `DungeonType == "tower of death"` | `DeathTowerCoordinator` 入塔发 142/143；CMD159 的 1/2 分别记战斗开始/清层；换层发下一张 143；最终发 144 排名、145 奖励、146 状态 | 仅作死亡之塔线索，不能直接套到 `[tower of grief]` |
| `../90dof` | `death_tower.go` 固定 dungeon 11000、45 层 | 同样使用 142/143、CMD159、144/145/146 | 同上；没有本塔 5115 的实现 |
| `../usdof` | 悲叹之塔 5115..5214、绝望之塔 11008..11107 | 首层按普通单图进场；悲叹之塔按账户保存最高已通关层，重复结算取最大值；进入选图界面前和 CMD46 普通结算后发 `S0/050E`（u16 层数）；旧楼层请求重定向到下一层 | MAP 反向索引和单调进度可作服务端设计线索。该 `050E` 仅在参考端单测覆盖结算；其“首层可进入”实机 baseline 明确排除了完整通关/奖励。当前 115 opcode 1294/`0x050E` 的原生命名为 `HONEY_TIME_EVENT_INFO`，不得把参考端的 `050E` 当作本客户端悲叹进度包。参考端没有给悲叹之塔发 `0x050B`。 |

参考端的结算差异解释了当前卡点：死亡之塔的 142～146 流程是另一玩法；悲叹之塔参考实现仍走普通 CMD46 结算并额外同步进度。当前 115 客户端唯一已闭环到悲叹状态对象的通关通知是 NOTI1291（`0x050B`，`33 01`），但参考端均未提供其在本客户端上的有效发送时机，也未证明它单独能弹结算或切层。后续应以 115 IDB 的 `NOTI1291` 消费路径和手动实机动态结果决定时序，不照搬 `050E`。

## 当前 115 opcode 表筛查（2026-09-27）

以下编号来自 `analysis/dumps/opcodes.tsv`。名称只是检索线索；“已命中”指本次 5115 层实机日志中出现，不代表它属于塔专用协议。

| 路线 | C2S CMD | S2C NOTI | 证据边界 |
| --- | --- | --- | --- |
| 本次 5115 进场、击杀、普通结算 | 16 `SELECT_DUNGEON`、37 `FINISH_LOADING`、117 `BOSS_DIE_CHECK`、39 `DIE_MONSTER`、46 `SET_PLAY_RESULT` | 28 `DUNGEON_INFO`、29 `START_MAP`、115 `BOSS_DIE_CHECK`、31 `ENABLE_CLEAR_DUNGEON`、34 `PLAY_RESULT`、37 `EXP`、35 `CLEAR_DUNGEON_REWARD`、261 `EPLP_RECHALLENGE` | 全部已在 `..._20260927_012225_417518_next37/events.jsonl` 命中；这些消息之后仍卡在首层。 |
| 塔账号/清层状态候选 | 无明确专用 CMD | 1286 / `0x0506` `ACCOUNT_TOWER_DUNGEON`、1288 / `0x0508` `USER_APC_INFO_TOG`、1291 / `0x050B` `TOWER_DUNGEON_CLEAR_INFO` | 1286 有两个 reader：一个依次读取 4 个 1 字节字段并复制到对象 +5072，另一个将相同 4 字节解释为 DWORD 状态码用于提示；两者的共同语义及触发时机未闭环。1288 目前仅名称线索；1291 的 reader 已确认两字节，首字节 `0x33` 路由到 Grief 状态对象，第二字节设 UI 控件布尔状态。三者本次日志均未发送。 |
| 死亡之塔（另一模块） | 159 / `0x009F` `DEATH_TOWER_STAGE_CMD` | 142～146 / `0x008E`～`0x0092`：信息、开图、排名、奖励、状态 | IDB 路由到模块号 9；本塔 Grief 为模块号 13。本次日志没有 CMD159。 |
| 绝望之塔（另一模块） | 1224 / `0x04C8` `DESPAIR_TOWER_EVENT_UNIV` | 929 / `0x03A1` `TOWER_OF_DESPAIR_INFO`、1185 / `0x04A1` `DESPAIR_TOWER_EVENT_UNIV` | 本塔是 Grief，不可仅凭爬塔外形移用。 |
| Grief 活动事件 | 1389 / `0x056D` `GRIEF_TOWER_COME_OVER_EVENT` | 1319 / `0x0527` 同名通知 | 已追到活动弹窗分支，尚无首层清怪/换层关联。 |

同一名称包含 `TOWER` 的 `BITAN_TOWER`、`BOSS_TOWER`、`TOWER_OF_DAZZLEMENT`、`TOWER_OF_GRAVE`、塔防及季节活动等 opcode 属其他系统，不列入 5115 的待发送集合。参考端悲叹进度使用的 `0x050E` 在当前 115 表里是 `HONEY_TIME_EVENT_INFO`，不能用作此塔进度通知。

## 本轮 IDA 追踪进度（2026-09-27）

- 在当前 115 IDB 的独立只读副本复核：NOTI1286 在 `sub_1452F9420` 注册到 `sub_1452C5260`，连续读取四个单字节字段；随后 `sub_14023A610` 将这四字节拷入对象 +5072，并把 +5076 置为 1。另一注册点 `sub_143EF1660` 将同一编号交给 `sub_143EF0D20`，读取 DWORD 并按 0、4、6、7、8 处理 UI/提示。它并非已证的结算包。
- NOTI1291 在 `sub_1452E91C0` 只读取两个单字节字段，按 6/51 选择 Despair/Grief 对象，再调用通用 `sub_1415D6850` 设置对象 +672。`sub_1415D4740` 用该值控制 UI 元件；没有楼层编号、奖励、切图字段。该 setter 还被非塔界面调用，因此先前将 +672 直接称为“已通关进度”过强。
- 普通 NOTI34 的注册点是 `sub_1452BAD10 → sub_1452B1960`；它走普通结果 UI，尚未见对 Grief 模块号 13 的专用转发。NOTI1288 在已检查的主通知注册器 `sub_1452F9420` 中没有注册，不能由名称推断其为此处所缺的结算包。
- `sub_1452FC5B0` 确实将模块号 13 路由到 Grief 对象的虚函数 +368，但它注册在 NOTI24 `AREA_USERS`（`sub_1453140E0`），属于场景用户更新，不是本次 CMD46 结算的直接回包。不能据此推导下一层按钮协议。
- 当前仍缺：Grief 对象的结算 UI 入口、NOTI1291 的可靠发送时机、点击后触发的客户端 CMD，以及 NOTI1286 四字节各自含义。未补包、未新增实机 attempt。

## 悲叹之塔专用结算通知：attempt 2/3（候选，待实机）

- 继续追 `sub_140E54C40` 的调用点，找到 `sub_144ED5E00` 直接注册 NOTI1255 / `0x04E7` `ENUM_NOTIPACKET_TOG_CLEAR_REWARD` 到 `sub_144ED79A0`。同组 NOTI334 `TOD_CLEAR_REWARD` 的 handler `sub_144ED76E0` 结构相同，但目标是模块 11、地图基数 11007；1255 的目标是模块 13、地图基数 5114。这比仅凭 opcode 名称更直接地确认了悲叹之塔归属。
- `sub_144ED79A0` reader 依次取 little-endian `u32`、`u16` 楼层、`u8` 项数，以及每项两个 `u32`。读完后把 Grief 对象 +672 置 1；若当前模块号为 13，则将 `u16` 写进塔结算 UI 对象 +92，逐项更新奖励行，再调用该 UI 对象虚函数 +48 显示。若模块号为 1，则会以 `5114 + 楼层` 查找地图并调用 `sub_144F609D0`；这不是 1255 在当前塔战斗场景里的主要消费分支。首个 `u32` 在当前 handler 的界面调用中未形成已证的数值用途，因此候选包暂置零；不把它解释成货币或评分。
- 首层实机 CMD46 后仅收到普通 NOTI34/37/35/261，缺少 1255。服务端本轮仅增加一个塔限定分支：从已验证的 `etc/towerofgrief.etc` 覆盖层保存楼层编号，CMD46 的普通结算 NOTI35 之后发送 `NOTI1255`，body `00 00 00 00 01 00 00`（首层、零奖励项）。这是**一次**结算入口假设，不额外发送 1286/1291，也不改客户端。下一层具体 CMD 和奖励字段语义仍待用户手动实机观察。
- `NOTI1286` 对象 +5072 的设置函数也被 `NOTI1337 ADVENTURE_LV_EXP` 调用，不能把它命名为账户塔进度。`NOTI2` mode1 中的尾字节用于计算 `5115` 的剩余入场次数，不能当成已通关层。`Npc/DespairTower.npc` 的 ID10000 是 5115 地图里出现的 NPC 线索，不能据此认作切层门。
- 本候选 `go test ./...`、`go vet ./...` 通过，已构建 `bin/wireprobe-handoff-source.exe`，SHA-256 `5542F70339BBF8EC2ED0EB3AE31CE42CE1826F15937D72BCF350CFC5F673C868`。需要玩家手动进入 5115 并击杀 Boss，观察结算 UI、是否收到 `tower_grief_clear_reward` 日志及之后的 CMD。实机未确认前不将其升级为 confirmed baseline，也不提交。

## attempt 2/3 实机回报：结算出现，但楼层没有推进

- 用户截图显示首层 Boss 倒地后出现 `Reward` 结算画面。`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260927_031600_762244_next37/events.jsonl` 在 19:17:50 与 19:18:14 UTC 两次记录 CMD46；两次均回 NOTI34/37/35、NOTI1255 `00 00 00 00 01 00 00`、NOTI261。客户端均未发送塔专用切层 CMD；先发 CMD72 完成普通结算，后回城。第一次回城后同一账号再次发送 CMD16 请求 5115，服务端再次准入首层。因此可以确认本候选只解除了结算卡住，没有实现楼层进度。
- 官方 Neople [Tower of Anguish 规则](https://www.dfoneople.com/news/updates/743/Tower-of-Anguish)说明按账号每日入场；[官方活动说明](https://www.dfoneople.com/news/events/749/Grand-Opening?page=6)进一步说明通关后当天不可再入场。当前 115 PVF `etc/towerofgrief.etc` 给出独立的 100 个楼层 DGN、`[enterable count] 2` 与 `[account enterable max count] 1`。这些证据支持每层独立结算、跨次入场推进，而不是击杀一层后立即在同一 run 自动换图；具体 115 客户端账号进度通知与选层 UI 仍须静态/动态确认。
- 服务端当前 `character.ProgressionService.Clear` 只保存副本最佳时间，不保存账号最高已通关塔层；客户端第二次 CMD16 仍请求 5115。下一步应闭环 115 客户端如何接收/显示已通关层并选下一层，再实现兼容存档的账号进度与准入。不要将参考服的 `0x050E` 照搬到此版本，也不要把 NOTI1255 楼层字段直接改为 2 来伪造下一层。

## attempt 3/3：账号楼层进度与再次入场候选（待实机）

- 用户选择原版逐日挑战流程，并补充结算菜单截图：只有 Retry、Select a Different Dungeon、Return to Town，没有“直接进入下一层”。因此首层后出现最终 Reward 画面本身符合每层独立结算；缺失的是后续入场的楼层进度。
- 新增独立的 `account_tower_grief_progress` 表，只增加账号级最高通关层、最近通关服务日和 run ID，不修改现有角色 `state`。首次读取时从该账号所有角色已存的 `dungeon_best_times` 导入连续最高楼层；用户已完成的 5115 记录可迁移为最高层 1。旧清关记录没有可靠时间戳，因此导入时不推断其当日次数；新候选产生的清关按 09:00 UTC 日切记录并限制当天再次挑战。
- CMD46 先读账号进度，验证正在结算的层是下一层，再沿用已实机解锁结算的 NOTI34/37/35/1255。角色清关保存后单调推进账号楼层，同一 run 重试幂等。NOTI261 不再点亮塔副本的 Retry；CMD72 重开请求也拒绝塔重试。
- 实机已证客户端第二次 CMD16 仍请求 5115。因此 CMD16 在下一次合法入场时，用 `etc/towerofgrief.etc` 与 100 张 MAP 校验后的楼层表，把静态首层请求映射为账号的下一层 DGN；对跳层、当天重复入场、满 100 层拒绝。这个请求重映射是 **attempt 3/3 的单一路径假设**，需玩家手动验证客户端最终加载的地图和 Boss 是否确为第二层。
- 未增加选图阶段的 NOTI1255 或参考端 `0x050E`：前者在选图模块虽有静态消费分支，但准确发送时机与 UI 效果仍未实机闭环；不以新包猜测层数显示。若候选不能进入第二层，停止试包，保留日志并继续 IDA/实机取证。
- `go test ./...`、`go vet ./...`、隔离 PostgreSQL 临时 schema 的 `TestTowerGriefProgressPostgres`、启动器只读 `--check --source-build` 均通过。独立的全量 `go run ./cmd/charactercheck` 在执行到本塔逻辑前即报既有疲劳回拨检查错误 `reconnect/backwards clock reset fatigue: <nil>`；本次没有修改该检查或相关存储。候选 `bin/wireprobe-handoff-source.exe` SHA-256 `3A2140DB913CDD0454EAD7C126A934CA3B4F8C48F5FC688E64E0666CCB5D131E`。等待用户手动启动候选并选塔；需记录选图 UI、CMD16 请求、实际 NOTI28/29 地图与 Boss、通关后是否限制当天重入。未确认前不升级 confirmed baseline、不提交。

## attempt 3/3 实机回报：第二层已通关，选图显示仍是第一层

- `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260927_034247_123927_next37/events.jsonl`：19:44:03 UTC 客户端 CMD16 仍请求 5115（`fb130000`）；服务端按已存楼层进度送 NOTI28 `fc130000` = 5116，并送 NOTI29 的 MAP 100248。19:44:13 客户端 CMD46 通关，服务端 NOTI1255 的楼层字段是 2（`00000000020000`）。因此截图中虽然卡片仍标 `Floor 1`，实际打过的是第 2 层。
- 同一会话 19:44:24～19:44:29 UTC 再次 CMD16 请求 5115，六次均被服务端以 `Tower of Grief entry is unavailable today` 拒绝。PostgreSQL 只读核查 `account_tower_grief_progress`：测试账号 `highest_cleared=2`、`cleared_day=2026-09-26`（09:00 UTC 服务日）。下次合法入场目标应为第 3 层；按本候选日切为 2026-09-27 09:00 UTC（北京时间 17:00）。
- 用户截图显示卡片 `Lv.090 (Floor 1)` 和 `Daily Entry Count: 1/1`，但服务端确有最高层 2 记录。当前仅完成服务端路由与存档；客户端选择器的层数字段并未同步。IDA `sub_144ED79A0` 表明 NOTI1255 在模块 1 时会将其 u16 层号变为 `5114+floor` 传入 `sub_144F609D0`，但模块切换时机、UI 更新效果、入场次数显示字段仍缺动态闭环。第 3/3 次运行路径假设已实机验证，后续不能为了修正文字继续盲发 1255 或参考服 050E；应在新的明确取证范围内继续静态/动态追踪。

## 奖励与多塔隔离候选（待实机）

- 用户要求按地图设定在结算时给奖励，并让不同塔分别保存进度、每日入场次数。`etc/towerofgrief.etc` 的 1–49/51–99 层只列 `tower_grief_reward_normal`，50/100 层列 `tower_grief_reward_special`，**没有物品模板 ID、数量或概率**；5115 DGN 与 100247 MAP 也无这些数值。不能由奖励类名推断 Old Golden Coin。该金币属于另一个 `etc/dungeonetc/towerofgravedungeoninfo.etc` 系统，其阶段奖励盒与 NOTI2044/2045、CMD1893/1894 走独立协议。悲叹之塔当前 NOTI1255 仍发送零物品行；新增的行编码器只表达已由服务端入账的 `(模板 ID, 数量)`，未连接到无来源的数值。
- 只读分析 115 客户端 `sub_144ED79A0 → sub_146B1BC10 → sub_144D52CB0 → sub_144D53A80`：NOTI1255 行首 DWORD 是物品模板 ID，次 DWORD 是数量，首 DWORD 报头含义仍未确认。之前 `analyze_batch` 自动审批拒绝，理由是共享 IDA 会话可能被修改；此结论来自允许的只读反编译，未更动权威 IDB。
- 普通通关经验已有实际入账。第二层 `clear` 回执为 Base 1,256,149、Score 150,736，合计 1,406,885；`ProgressionService.Clear` 在同一角色事务中增加经验，NOTI37/35 同步显示。当前证据不支持再额外发一次塔经验。
- 新增 `account_tower_progress`，主键 `(account_id,tower_key)`，分别保存最高通关层、服务日、当日入场次数和最后通关 run；首次迁移保留旧 `account_tower_grief_progress` 并复制现有数据，不修改角色存档。入场成功时原子扣除该塔当天额度，通关时幂等推进该塔楼层。运行时 `TowerRuntime` 把每座塔经来源核验的层表、每日上限和日切交给共享入口/结算逻辑。当前仅悲叹之塔的 100 层覆盖层已核验并启用；其他塔要分别完成地图覆盖层、客户端结算通知及奖励来源闭环后接入。
- 隔离 PostgreSQL 测试验证旧悲叹记录迁移、同塔每日 1 次、另一塔每日 3 次且互不占用、单次 run 幂等。物品奖励尚缺原服 115 的 `tower_grief_reward_normal/special` 对应表或可验证 live 结算向量；未猜物品 ID/数量，也未给悲叹之塔发坟墓之塔金币。
- 当前 PVF 导出覆盖层现带每层 `reward_rule` 和账号日额度，加载时做完整性检查。`go test ./...`、`go vet ./...`、隔离 PostgreSQL `TestTowerGriefProgressPostgres` 均通过。已构建候选 `bin/wireprobe-handoff-source.exe`，SHA-256 `3EA787C13D7ECA25753080979D03AB0F8A676DD252801D913ED9126A09CABA7F`。本候选未实现未知的物品数量；若实机回归，仅检查入场次数是否在入场时扣除、下一层仍可在次日进入、已有经验是否入账。
- 追加通用塔物品发奖接口：已核实的每层物品表可以作为 `TowerRuntime.Items` 注入，`ProgressionService.ClearWithTowerRewards` 在角色 `clear:<run>` 事务中同时保存经验、物品与回执；重放复用回执。结算从回执生成背包更新，悲叹之塔 NOTI1255 的物品行只取已入账物品。当前覆盖层只带奖励规则名，没有物品表，因此 `Items` 为空、不发送背包奖励行。该接口仍需有明确奖励表的塔接入并实机验证非空行显示。构建哈希与全量测试须在这次补码后重跑，以最终结果为准。
- 补码后 `go test ./...` 与 `go vet ./...` 均通过，最终候选 `bin/wireprobe-handoff-source.exe` SHA-256 为 `233D04E1014B2E67F2E4E30FEB63799D84ADC50FC08B11052EB63DA32253C101`。此前 `3EA...` 哈希只对应追加发奖接口前的中间候选。

## WIP 分支快照与测试期每日限制

- 按用户要求将目前塔功能收录到 `codex/towers-wip`，这是未完成快照，并未升级为 confirmed baseline；前述候选和实机证据仍按各节当时状态理解。
- 为连续测试，当前共享塔入口和 `ReserveTowerEntry` 暂不拦截每日次数，仍分别记录每座塔的入场日期与次数；顺序楼层和满层检查保留。恢复原版限制时须同时恢复入口和数据库原子检查。
