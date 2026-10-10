# next151 — 巴卡尔攻坚战（Bakal Raid）证据重建实现文档

> 2026-10-05 立项。目标：把巴卡尔攻坚战（军团本，contents/2022/bakalraid）以"不猜包"方式
> 在主仓库 `server/work/dfo-lan/` 重建实现。交接包（D:\115us\实时抓包\115us）只有新编译的二进制
> （内含巴卡尔），源码树没有那 19 个 raid_bakal 源文件，因此本任务 = 基于三层证据重建。
> 全程遵循根 AGENTS.md §0（服务端优先 / 真源优先 / 禁止猜包 / PVF 驱动玩法）。

## 0. 调查结论（已彻底验证，勿重查）

1. **源码缺失**：交接包 `server/work/dfo-lan/bin/wireprobe-pvf.exe`（30,871,040 B，
   2026-10-05 06:35 构建，go1.26.0，-trimpath，module=dfolan）的 pclntab 引用 19 个 raid_bakal
   源文件，但这些 .go 在交接包源码树、115us.rar（72393 条全量）、主仓库所有分支、D 盘全盘
   均不存在。用户确认 rar 是唯一来源。→ 朋友打包时二进制新、源码旧。
2. **缺失文件清单**（自二进制符号表提取，短名因 trimpath 合并为 13 个）：
   - internal/raid/{bakal_script(518行), bakal_opening(143行), bakal_buff(141行)}.go
   - internal/catalog/{bakal(413行), bakal_bidding(132行), bakal_rewards(150行), bakal_events(35行), bakal_stage(15行)}.go
   - internal/game/protocol/bakal.go（raid_bakal.go）
   - internal/workflow/bakal_rewards.go
   - cmd/wireprobe/{bakal_flow(184行), bakal_portal(86行), bakal_room_warp(48行), bakal_retreat(43行), bakal_party_buffs(13行)}.go
   - 有效代码行合计 1,921（pclntab file:line 精确统计），全量估 2,500~3,000 行。
3. **符号地图**（`go tool nm`，98 个 bakal 符号，约 70 函数）：
   - `internal/raid.BakalOpening` 状态机（28 方法）：Activate/advanceAnger/AuthorizeDungeon/
     AuthorizeSlot/ClearFinalDungeon/cloneScript/createScriptMonster/DefeatMonster/deleteScriptMonster/
     dispatchScript/EnterDungeon/executeScript/GiveupDungeon/grantBuff/grantWeightedBuffs/
     initializeScript/matchScript/PartyBuffs/presence/Remaining/ReportHealth/ReportSymbol/
     setScriptSymbol/SettlementIdentity/StageWarp/Tick/UseRaidBuff + PrepareBakalOpening
   - `internal/catalog`：importBakalRaidRules/loadBakalBidding/loadBakalRaidTables/
     parseBakal{BuffDefinitions,EnterSymbolRules,RaidMonsters,RaidSlots,Rewards,Script,Stage,Symbols}/
     readBakalCOSTokens/bakalInstructions/bakalInstructionSchema/bakalIntegerBlock/
     bakalNormalBiddingCount/bakalWeightedNumbers
   - `internal/game/protocol`：Bakal{BuffInfo,MonsterInfo,PartyInfo}Payload/BakalMonsterType/
     DecodeBakal{BuffUse115,CampReturn115,HealthReport115,RoomWarp115}（115 = 115 级客户端版本标记，非 opcode）
   - `internal/workflow`：BakalRewardService{Claim,Freeze,Recover}/BakalRaidAdmission/
     readBakalProgress/saveBakalProgress/ErrBakalWeeklyClearLimit
   - `main`（cmd/wireprobe）：worldSession.bakal{ConfirmDefeats,EnterSymbolPlan,LoadedBoss,
     NativeCampReturn,NativeRoomWarp,OpeningPlan,PartyAt,PortalCatalog,PortalSelection,Retreat,
     RoomPartyPlan,ScriptPlan,SettlementPlan,SymbolPlan}/enterBakalPortal/gameConnection.{tickBakalOpening,
     releaseEndedBakalOpening}/bakal{FinalActorDefeated,PortalInRoom,RaidAdmission,SymbolValuePlan}

## 1. 三层证据（互补闭环，均可复验）

| 层 | 路径 | 内容 |
| --- | --- | --- |
| A 二进制反汇编 | `D:\115us\实时抓包\115us\server\work\dfo-lan\bin\wireprobe-pvf.exe` | 33,154 行带 file:line 的反汇编（`go tool objdump`），逻辑完整定义 |
| B 交接包通关实录 | `D:\115us\实时抓包\115us\server\work\dfo-lan\runtime\roles_..._20261005_200137_561796_next37\events.jsonl` | 19,632 行，12:29:29~12:39:42 完整通关一轮（建团→投票→start→17 图→24 次加载→final_selection→clear_result→奖励→回营地），每包 plain_hex |
| B' 失败场次 | 同级 `20261005_125050_777745`（3 次拒绝+3 次撤退）、`20261005_004229_079980`（5 次入口拒绝） | 异常/拒绝路径实录 |
| C 官服抓包 | `D:\115us\实时抓包\captures\20261005-015111` | 巴卡尔两轮完整：建团656 / START 582? / 下图 572 / 2285 队伍×200 / 2286 怪物 / 2288 buff / 2290 怒气×11 / 2069 战报×50 / 2062 直移 / 588 结算×2。缺 stage3/终局 —— 由 B 补 |

已导出的辅助文件：`D:\115us\analysis-tools\output\bakal_timeline_200137.txt`（241 条时间线，含 opcode+hex 前缀）。

## 2. Wire 契约（自 B 层逐包还原，全部有原始 hex 背书）

巴卡尔走 **RAID 族**（与维纳斯/末世录的 2043/2290/2655 族完全不同）。
cmd 与 noti 共享 16-bit id 空间、双向含义不同（同 session.go 注释规则）。

### 2.1 入口（等待室 → 开战）

| 方向 | id | 名称 | 形状/语义（证据） |
| --- | --- | --- | --- |
| s2c | — | area_change → waitroom | town=152 area=2，map=contents/2022/bakalraid/map/town/waitroom.map，落点 (350,246) |
| c2s | 656 | 建团 | 32B：`0d060000 00 "Lansmt" 00... 07000000 00...`（角色名内嵌）。服务端建 raid：channel_type=82，member_max=12，source=bakal.etc（raid_created_waiting） |
| c2s | 657 | 建团相关（离队/更新） | 8B：`02000000 00000000`（n=4） |
| c2s | 2089 | START_RAID 开战 | 32B：`88f25f00 00000000 feffffff 0001 0000...`。响应见下组 |
| s2c | 2089 | 开战 ACK | 1B：`01` |
| s2c | 574 | RAID 状态机 NOTI | 2B：`0100` preparing / `0200` active / `0000` phase_ended（终局后） |
| s2c | 2343 | vote_start | 181B（45×u32 结构，含 01/02/03 选项槽；完整 hex 在 B 层） |
| s2c | 2344 | vote_state | 181B（同上，成员票态位） |
| s2c | 578 | real_member_assigned | 87B：`0100 5201 03 ... "Lansmt" ...`（含角色名+槽位） |
| s2c | 581 | countdown | 0B |
| — | — | bakal_prepared | 内部：ready_at = start+3s（solo 投票窗 3 秒）；raid_id=22151169（两场相同，疑似按日期/content 派生，待反汇编确认） |
| c2s | 12 | 创建小队 | 48B：`000009000000 "Party : 1" 04 ffffffff 05 ... 0707×8 ...` |
| s2c | 12 | 小队 ACK | 1B：`01` |
| s2c | 9 | subparty_created | 116B：`0100 0f27 0100 0152 ...`（0x0F27=9999 常量） |
| s2c | 2 | subparty_actor / subparty_details | 1760B / 2882B（角色装备外观全量，结构同普通 party 族，需按帧实现） |
| s2c | 14 | subparty_worn_restored | 5252B（穿戴外观恢复） |

### 2.2 开战广播（+3.7s 后一次性爆发串）

| 方向 | id | 名称 | 形状/语义 |
| --- | --- | --- | --- |
| s2c | 2285 | party（队伍总览） | 173B：`01 01 0000 0034 00000000 00000000 [idx u32+pos u32]×…`（0x19=25 槽位 + 位置对） |
| s2c | 2286 | monsters（8 首领槽） | 153B：`08` + 8×19B 组（`06000000 07000000 01000000 10270000 ffffff`：槽/图序/1027 怪模板/ffffff） |
| s2c | 2288 | buffs | 13B：`02 02 02 02 02 19 000000 ffffffff`（5×u8 buff 位 + 0x19 + ffffffff） |
| s2c | 572 | open_dungeons（17 张图） | 91B：`00 11 [u32 LE]×17` = 0x05F5ED4D..0x05F5ED5D（ED4D 疑营地/待机图，ED4E..ED5D 为 16 作战图；summary 口径"14 作战图"待实现期核对） |
| s2c | 584 | remaining | 9B：`00 0f270000 00000000`（u8 0 + u32 9999 + u32 0） |

### 2.3 作战循环（每张图，共 24 次加载）

顺序（B 层时间线 12:29:54~12:36:42 反复出现）：

1. s2c **2281** portal_ready：38B 全零（下一图入口就绪）
2. s2c **2285** room_party_location：173B（房间内成员落位更新）
3. 客户端走传送门：c2s **22**（24B，含两个地图 id 如 0x05F5E71B/0x05F5E3B1）或 c2s **707**（8B：`u32 地图id + 0`，n=16）→ enterBakalPortal
4. s2c **2194** source_boss：34B：`01 0X 00 5c10 0000 <pos u32> 0003 00 64000000 <hp?> <hp?> ffffffff 00000000`（首领/怪物实体标记+血量对）
5. c2s **2073** 加载完毕（0B）→ s2c **2073** ACK 1B `01`（×24 对）
6. 房内运行期：
   - s2c **2286** script_monsters / script_actor_health（20B 清怪 `01 idx u32 … ffffff`；39B/58B/77B/96B 多组生成）
   - s2c **2288** script_buff_inventory：13B（5 buff 位计数）
   - s2c **572** script_dungeon_states：11B `00 01 <u32 地图id> 0000 0000`（单图状态）
   - s2c **570** source_symbol：9B `01 <u8 符号id 0x66/0x67/0x6b…> <u32> <u32>`（×7751，怒气/符号主通道）
   - c2s **2069** 战报：184B（击杀上报，n=34）→ bakalConfirmDefeats
   - c2s **2062** 直移：48B `2e20d46c fc7f0000 … 0158edf505`（含目标图 id，n=27）
   - c2s **585** 战斗心跳（0/24/40/56B，×234）；c2s 38 位置（16B，×271）
7. 房间间跳转：c2s **2070** script warp（48B：两组坐标+参数）→ s2c **2070** ACK `01`（×9 对）→ 下一 2281

### 2.4 终局 → 结算 → 奖励 → 回营地（B 层独有，官服抓包缺）

1. 末图首领击杀（c2s 2069 上报 → 2194 出现 0101/0105 变体 + 2286 血量帧）
2. s2c **27** final_selection：36B 全零（终局选择面板）
3. s2c **2281** final_move：38B 全零（终局图入口）
4. c2s **1134**（32B：`01000000 ccf4f505 0e000000 01180000 00cd0000 00...`，0x05F5F4CC=终局图）→ 2073 加载闭环
5. 结算爆发串（同一 tick 12:39:42.954）：
   - s2c **2285** script_return_to_camp：173B（`0034`=52 营地房间归位）
   - s2c **13** source_reward_inventory：10141B（奖励清单全量，结构=通用奖励列表，头 `00 10 0038 0000...`）
   - s2c **588** clear_result：10B：`00 00 6202 0000 0000 0001`
   - s2c **574** phase_ended：`0000`
6. c2s **13**（8B 零）×6：奖励领取请求（与 s2c 13 同 id 双向）→ BakalRewardService.Claim/Freeze/Recover

### 2.5 拒绝/撤退路径（B' 中午场实证）

| 触发 | 响应 | 证据 reason |
| --- | --- | --- |
| c2s 2070 在未加载归属作战图时 | 拒绝 | `Bakal scripted warp outside loaded owned combat` |
| c2s 2074 原生回营地（撤退）非法时 | 拒绝 ×2 | `invalid native Bakal camp return` |
| 合法撤退 | s2c 2285 retreat_to_camp（173B，槽位归位 34/52 营地） | ×3 实录 |

cmd **2074** = 撤退（原生回营地）；**2261**（0B，×1）疑似放弃/退出待反汇编核对。

## 3. PVF 源绑定（§0.2 强制）

- 规则真源：`contents/2022/bakalraid/etc/bakal.etc`（bakal_prepared 事件直接给出）
- 地图：`contents/2022/bakalraid/map/town/waitroom.map`（等待室）；作战图 dgn 待自 dungeon.lst 拉取
- 实现时必须走 catalog 解析器直读 PVF（参照 apocalypse_import.go / venus 模式），
  不得在 Go 里硬编码图序列/怪物表/奖励表 —— 图 id 列表 0x05F5ED4D..5D、怪模板 1027、
  符号表、buff 权重（bakalWeightedNumbers/bakalNormalBiddingCount）均以 bakal.etc 为准。

## 4. 主仓库落点（对照既有结构）

| 包 | 文件（新增） | 内容 |
| --- | --- | --- |
| internal/legion | `bakal.go` | 协议常量/编解码/ACK（对齐 venus.go 风格：常量→解码→ACK 构造） |
| internal/legion | `bakal_opening.go` | BakalOpening 状态机（574 状态/授权/脚本 tick/符号/血量/撤退/结算身份） |
| internal/legion | `bakal_script.go` | 脚本帧调度（2286/2288/572/570 回放帧，参照 ispins_replay_frames 方法论） |
| internal/catalog | `bakal_raid.go` 等 | bakal.etc 解析（slots/monsters/symbols/script/rewards/bidding/stage） |
| internal/workflow | `bakal_rewards.go` | BakalRewardService（Claim/Freeze/Recover、周清上限 ErrBakalWeeklyClearLimit、进度 read/save） |
| cmd/wireprobe | 接线 | worldSession.bakal*（入口/传送/撤退/结算/符号）、gameConnection.tickBakalOpening |

命名对齐交接包符号（§0 第 3 条），便于日后与朋友的源码逐函数比对。

## 5. 实现顺序与验证

1. 协议层 bakal.go：§2 全表 + 拒绝路径，单测对照 B 层 plain_hex 逐字节
2. catalog：bakal.etc 解析器 + fixtures 测试（PVF 直读，§0.2 六步流程）
3. 状态机 + 脚本帧：以 B 层时间线为回放脚本（241 事件 → 帧序列），断言
   每个opcode 出现顺序/字节数与 plain_hex 一致
4. 奖励/结算/撤退：s2c 13/588/2074 路径 + 拒绝 reason 文案对齐
5. 反汇编校对（`go tool objdump -s`）关键函数：PrepareBakalOpening/advanceAnger/
   SettlementIdentity/BakalRewardService.Claim —— 语义存疑处以 A 层为准
6. 编译 + `go test ./...` + vet；实机由用户操作验收（§0 硬约束 6）

## 6. 已知缺口（不猜，留证）

- 官服 c2s 582/2290（怒气）在 B 层无对应（朋友服未触发）：以 B 层为准实现 2089/570；
  若实机出现 2290 再按官服抓包补
- raid_id 22151169 的派生算法：待反汇编 PrepareBakalOpening 确认
- 656 的 s2c 响应：B 层未见独立 bakal 事件（可能仅有内部态）—— 实现期核对
- s2c 2/14（1760/2882/5252B 大包）的完整字段语义：按普通 party 族结构 + 反汇编核对
- stage3/结算细节官服缺失：以 B 层 + 反汇编为准，不参考旧服

### 2026-10-05 impl4 实现笔记（BakalOpening 状态机收口后的证据修正）

- **结算 N13（10141B）已破解：标准背包全量同步帧，非巴卡尔专有**。完整 hex 已导出
  `D:\115us\analysis-tools\output\bakal_reward_n13_200137.hex`（源 events.jsonl
  200137 场 `bakal_source_reward_inventory`）。行结构：槽位 u16 + 物品 id u32 +
  数量 u32 + 定长零填充（内含 ffffff7f 未强化字段），约 50 行。奖励经服务端背包
  发放后推既有 N13；impl5 只需周清计数（[WEEKLY CLEAR LIMIT]/[WEEKLY REWARD
  LIMIT]=1）+ Claim/Freeze/Recover + 按 catalog 规则（ChannelSlotRewardItems/
  MonsterPieceRewards/Bidding）计算发放，N13 复用既有 inventory 推送。
- **2286 行的"第三字段"是行状态**：1=存活/生成，2=血量更新（12:36:34.183 的
  (1,24,2,5000) 半血对帧）；击败行=槽位0+状态1+HP0（031/100/229）。
- **开局花名册槽位表**（wire 证据，catalog 未投影槽位号）：
  (6,7)blona (2,12)sparazzi (5,23)basilisk (1,24)bakal (7,25)gerda (3,26)skasa
  (4,40)hisma (8,47)nympha；四 boss 血量行=槽位1..4。
- **+1s 发布对**（timeline 019/020）：153B 花名册原样重发 + 77B 四 boss 状态2行。
- **[SET TIMER] 的 sub 是位置号**：28 56 180=结算房 loc56（180s 窗口），28 24
  600=巴卡尔房 loc24，27 24 120=怒气窗。
- **新增已知缺口**（impl4 明确不实现，未猜包）：
  - 场景怪定时波次（timer 18 sub0 每5s，2286 槽位 9..13 的生成行）——依赖
    [RAID MONSTERS] 表（catalog 未投影；符号 parseBakalRaidMonsters）
  - buff 授予的加权随机槽位选择（符号 bakalWeightedNumbers）——GrantHook 注入，
    B 层实录：12:30:15 授槽3；hisma 清图授槽0,1
  - N570 怒气/伤害通道（7751 帧）不在导出时间线——怒气仅结构性累计
  - 2194 的 room/run 计数为服务端不透明铸造（B 层 cf/c9/ca... 非单调）——按
    单调铸造，字节不锁定
  - 2285 party 位置帧的成员槽位计数器=其他团员的加入时间戳（solo 服=全 25）
- 状态机落点 `internal/legion/bakal_opening.go`+`bakal_script.go`，回放测试
  `bakal_opening_test.go` 全绿（确定性帧逐字节 fixtures 对照）。

### 2026-10-05 impl5 实现笔记（BakalRewardService 完成后的 A 层证据修正）

- **双计数器语义（A 层 INCL 实证，Freeze.func1 :100/:156）**：`Clears` 只在不合格
  Freeze 时 +1（烧通关次数），`Rewards` 只在合格 Freeze 时 +1（预留本周奖励位）。
  Admission 用 `Clears >= [WEEKLY CLEAR LIMIT]` 拦截。Eligible 判定（:101）：
  `MinimalDungeonClearCount <= cleared && (WeeklyRewardLimit <= 0 || Rewards < WeeklyRewardLimit)`。
- **plan 身份四校验（Claim.func1 :173-:174，四个 memequal/compare）**：
  plan.Run==run、plan.Source==current.ConfigVersion、plan.Content==服务端目录身份串、
  current.ConfigVersion==savecontract.Identity()。plan 结构 112 字节 = 4×string +
  Items + Products + Eligible（go.shape 证据）。
- **发放形状（Claim.func1 :179-:198，本日修正）**：Claim 发放 `plan.Products`；
  仅当 Products==nil 才回退 Items，回退把每行转成 `loot.Award{Template, Amount:1}`
  （:186 `MOVL $0x1` 硬编码 1）——**绝不是 Items+Products 双份**。发放循环
  （步长 8=loot.Award）内做 template/amount 零值检查 → `invalid persisted raid
  reward item: <run>`（go:string.*+194881，34B）；回退转换前同样检查 Template>0
  （:183，TESTL+JLE）。结算尾段 :200-:206 = 重读账本 → mapdelete(plan) → save。
- **`catalog.BakalRewardEntry` 属 catalog 包（nm type:.eq 实证，40 字节，Template
  在偏移 0x18）**——已落 `internal/catalog/bakal_raid.go`；Category 字段是本重建
  的审计标签（记录来自哪张 bakal.etc 表），A 层 40 字节布局无法完整复原（源码丢失）。
- **Freeze 的随机性来自 crypto/rand（:118/:144 两次 crypto/rand.Int，:148 紧接
  OpenRewardBoxes）**——开盒种子是新鲜随机数，不是任何 run 派生值；replay 安全由
  commitEquipmentEvent 事件账本保证（重放恢复存档 receipt，不重跑规则）。其中
  :118 的那次属于丢失的加权竞价抽签（bakalweeklybidding.etc 的 grade→物品投影）。
- **服务 Store 字段用窄接口 `equipmentEventStore`**（*database.Store 天然满足），
  测试注入 equipmentEventFake；事件键 `bakal-clear:<run>`/`bakal-grant:<run>`，
  模型 `bakal-source-reward-v1`。
- **周键**：`database.IspinsWeekStart(now).Format(time.RFC3339)`；跨周 Freeze 清零
  两计数器但保留 Plans，Claim 不校验 week（A 层实证，周日冻结周一仍可领）。
- 错误文本（A 层逐字节）：`普通巴卡尔本周通关次数已用完`（errorString 42B，
  .data 0x1415bb1c0）、`native raid rules missing`（+102372）、`no owned source
  raid reward plan`（+174492, 32B）、`raid reward source/owner missing`（+174460）、
  `raid reward service missing`（+122255）、`invalid character state for raid
  rewards`（+248412）。
- 落点：`internal/workflow/bakal_rewards.go` + `bakal_rewards_test.go`（4 测试
  全绿：Admission 周门含中文错误原文、Freeze 短清/满清/本周二次、Claim 发放+
  replay 幂等+幽灵 run、跨周、存档契约门、Recover）。

### impl6 接线清单（A 层 nm 符号表，main 侧应建函数名）

- `main.(*gameConnection).tickBakalOpening` / `releaseEndedBakalOpening`
- `main.(*worldSession).bakalOpeningPlan` / `bakalConfirmDefeats` /
  `bakalEnterSymbolPlan` / `bakalLoadedBoss` / `bakalNativeCampReturn` /
  `bakalNativeRoomWarp` / `bakalPartyAt` / `bakalPortalCatalog` /
  `bakalPortalSelection` / `bakalRetreat` / `bakalRoomPartyPlan` /
  `bakalScriptPlan` / `bakalSettlementPlan` / `bakalSymbolPlan` /
  `bakalSymbolValuePlan` / `enterBakalPortal`
- `main.bakalFinalActorDefeated` / `main.bakalRaidAdmission`
- protocol 侧（二进制 internal/game/protocol）：DecodeBakalBuffUse115 /
  DecodeBakalCampReturn115 / DecodeBakalHealthReport115 / DecodeBakalRoomWarp115 /
  BakalBuffInfoPayload / BakalMonsterInfoPayload / BakalPartyInfoPayload /
  BakalMonsterType（impl2 已建 internal/legion/bakal.go 对应物，接线时对名）
- wireprobe 错误文本（二进制提取）：`missing active Bakal opening`、`Bakal
  loading without an owned dungeon`、`ambiguous Bakal room location`、`Bakal
  scripted warp outside loaded owned combat`、`invalid native Bakal camp return`
- 结算 N13：`loot.Service.Bootstrap(role)` 全量背包（10141B）→
  `SetRewardInventory` splice 位置 2285→N13→588→574（impl4 已定）
- 领奖：c2s 13 → `BakalRewardService.Claim`；登录/进图 `Recover`；建团前
  `BakalRaidAdmission`；656 建团（名+channel 字节+type 7）

### 2026-10-05 impl6 接线笔记（cmd/wireprobe 完成）

- **落点 `cmd/wireprobe/bakal_flow.go`**（新文件）：dispatchBakal 挂
  beforeClientTypeDispatch（dispatchVenus 之后、dispatchLegion 之前），门控
  channelType==82 + verified；全部拒绝只落 `raid_entry_refused` 事件、无 s2c
  （B′ 拒绝实录逐字节复刻：ErrBakalWaitingRoomNotBound /
  ErrBakalStartRequiresMember / ErrBakalWarpOutsideCombat /
  ErrBakalInvalidCampReturn）。
- **CMD12 待机小队应答串**（时间线 007-011）：basic(N2) → detail(N2) →
  worn_restored(N14, inventory.WornSpaceUpdate) → created(N9,
  BakalSubpartyCreatedFrame) → ack(N12, 01)。与伊斯/维纳斯先例同族。
- **CMD2089**：DecodeBakalStartRaid 校验 feffffffffff 标记 → Start(name) 开战
  burst → 追加 ACK（时间线 000-005 顺序）。**CMD656**：Admission →
  PrepareBakalOpening → 事件 raid_created_waiting，无 s2c（B 层实录如此）。
- **进图（22/707）**：捕获的 map id → dungeon 对照不在证据里 → 服务端提名
  `NextPortalDungeon()`（按目录序第一张未清 boss 图），事件 bakal_portal_selection
  记录请求 id 与提名；EnterDungeon(dungeon, BakalLocationOfDungeon)。
- **CMD2069**：DecodeBakalBattleReport（dungeon u32@17/monster u32@21）→
  DefeatMonster(dungeon, location, now)。**CMD2073**：DecodeBakalLoadingDone(0B)
  → LoadingDone → ACK。**CMD2070**：body 坐标组→房间映射未捕获 →
  StageWarp(当前图 location) + ACK。**CMD2074**：DecodeBakalCampReturn(32B,
  option@13) → CampReturn。**CMD1134**：DecodeBakalFinalConfirm(map u32@4) →
  FinalConfirm。
- **结算**：tickBakalOpening 挂 serve() mineTicker（venusOperationClose 之后）；
  SettleDue(now) 时先 bakalSettleSplice（Freeze 入账 → w.role=saved →
  loot.Bootstrap(workflow.LootRole(role)) 全量背包 → SetRewardInventory）再
  Tick() 发结算串 2285→N13→588→574（B 层 237-240 同 tick）。
- **领奖/恢复**：CMD13 结算后 → Claim（B 层 6 次实录、无 s2c 应答）；
  待机期 CMD13 → 离队（BlackPurgatoryPartyGone 先例 N9 + 清 w.bakal）；
  登录 82 频道 → bakalRecover（Recover 补领，失败仅事件）。
- **注入**：gatewayRuntime.bakalRaidRules（pvfCatalogs.Bakal 直读）+
  bakalRewardService（newBakalRewardService，rules/store/loot 缺一即 nil=
  内容未装载）；worldSession 侧 bakal/bakalRun/bakalName/bakalCurrent/
  bakalLocation/bakalRules/bakalRewards。
- **记录在案的缺口（不猜包）**：① raid_id 派生算法不透明 → 用启动时间戳铸造
  `bakal-<UnixNano>`；② CMD2062 直移 → N2285 位置更新的 mint 规则未捕获 →
  仅事件记录；③ N2194 源头 boss 行的触发时机/HP 缩放不透明 → 不发送（显示
  由 N2286 覆盖）；④ buff 使用（N2288 消耗）无 c2s 实录 → GrantHook 已留
  注入位但不接线；⑤ CMD657 语义不透明 → 仅事件记录；⑥ CMD2261 B′ 上游拒绝
  → 不受理、仅事件；⑦ 加权竞价抽签（crypto/rand.Int :118）依赖丢失的
  bakalweeklybidding.etc 投影 → 缺口。

### impl7 — 全仓验证（2026-10-05）

- `go vet ./...`：exit 0，全仓干净。
- `go test ./...`：legion / workflow / wireprobe / loot / catalog 等全部 ok；
  唯一 FAIL 是 `internal/character` 的
  TestEntrySkillsPreservePayloadAcrossProfessionHashChange 与
  TestEntrySkillsRejectDifferentProfessionReference —— **既有问题**，
  与巴卡尔改动无关（巴卡尔未触碰 internal/character），不在本任务修复。
- 本轮 scenes 归档夹具未再出现失败（./... 范围内无 scenes FAIL 行）。

### impl8 — 核心链路审查修复候选（2026-10-06）

- 修复默认规则域遗漏、2062被吞/进图无Session与N28/N29、四首领提名导致终局不可达、2069冒充死亡、N13通知类型、冻结失败仍结束、发奖时机/背包同步及选角前恢复。加入真实PVF的网关、两阶段终局与事务失败/重放回归。
- 证据修正：2069的@21是地图；N2194是动态实体/模板/坐标而非run计数/HP；N570为symbol u32/value i32；结算自动执行Freeze→Claim→Bootstrap。实录六个CMD13均unimplemented_sample，不能作为领奖入口依据。22/707所列样本属于城镇移动，正常作战进图用2062。
- 原生DGN头/迷宫等级字段分域解析，17张图全部导入；`clientbakalslotscript.cos`、`bakalmonster.cos`、`.symbol`与同源MAP接回运行。开放图、初始位置/符号、倒计时、怒气窗/失败与战斗踢回计时由解析规则执行。
- 奖励实际读取`contents/system/raidsystem/raidreward.etc`的普通巴卡尔权重池。移除预览表/碎片rank表作为固定发奖表的错误；保留存档键、事件键、身份门与既有双周计数器语义。
- 全仓vet通过；全仓test仍仅上述两项character失败。相关包完整测试及真实PVF巴卡尔专项通过；候选build和启动器只读check通过，没有启动客户端/服务或访问玩家库。
- 隔离候选入口、哈希、源链与验证证据见 [核心修复记录](../../server/work/dfo-lan/docs/protocol/bakal-fix-20261006.md)。没有替换日常二进制，不扩展实机confirmed baseline。
- 待闭合：完整定时小怪/Buff/竞价和困难实机。N2285尾部经`bakalPartyAt→PartyBuffs`确认是Buff与到期值，撤销impl4“团员加入时间戳”说法；原始N2285空槽形状保留，不能拿它当成员名单。

### impl9 — 152/1等待室建团候选（2026-10-06）

- 用户02:42手动会话证明：当前PVF `town/bakal_raid.twn`中152/1为waitroom、152/2为camp1。ETC等待室pair不得直接用作世界area键；新增源party-list区域/等待室NPC绑定，保留原pair审计，不硬编码area1。
- 撤销impl6“656无s2c”结论：完整二进制发送owned details N578与ACK656，后续1353/1220和650/585也有应答。其记录/字段按`raid_entrance.go`反汇编与DWARF结构恢复；新增实际16B请求/位置/归属ID回归。
- 确认实际客户端为`D:\115us\DFO\DFO.exe`（PID37976，版本2.38.2.34）；client.log与进程映像路径一致，客户端未改。
- attempt 2/3；专项、vet、56域只读准备通过，全仓仍两项已记载character失败。默认入口已指向新party候选；哈希与证据详见核心修复记录。用户当前进程未被关闭，需手动重启加载，尚未作新的实机成功确认。

### impl10 — 动态频道监听与建团后控制确认（2026-10-06）

- 09:03启动失败为后续频道取首端口加序号碰到系统占用50556；动态模式改为各频道独立绑定`:0`并公布实际端口。12个真实临时监听器及固定端口兼容回归通过。09:13用户会话已正常启动并建团。
- 09:13两次建团应答成功，但没有2089开战请求。发现2121控制握手漏答；按交接二进制`client_raid_entrance.go:178..185`与`raid_entrance.go:288..292`补齐Kind1/CMD2121/01，两个当前实机样本与非法输入回归通过。不代客户端发起投票，按钮阻塞因果尚待用户手动验证。
- 同功能attempt 3/3；全仓vet与启动/profile25项测试通过，全仓test仅两项既有character失败。最新control候选已接入默认入口；完整会话、证据和哈希见核心修复记录。若仍失败，先取具体提示与客户端消费路径，不继续盲改。当前完整Buff、怪波次、竞价及困难模式缺口不变。

### impl11 — 用户指定官服抓包与团长编队窗口新取证（2026-10-06）

- 09:30实机已返回2121ACK，但仍无2089/12；不认定控制确认已经修复团长UI。
- 以`captures/20261005-015111`与当前`D:\115us\DFO\DFO.exe`只读取证，N578注册点144CD5547、回调144CE30B0、头reader144CDBF80确认：旧重建误将MemberCount写到模式标志，扩展头缺5字节，内嵌团长错位。按官服前缀逐字节回归修复，初始有效成员/位置恢复，成员上限仍由PVF驱动，名望复用既有服务。
- 661 action0原生请求与N578 mode3、ACK661闭合，新增拥有者/源位置范围门禁；661 attempt1/3。用户明确指定新证据范围，不继续此前同功能的盲试。
- 全仓vet通过，全仓test仍两项既有character失败；formation候选接入默认profile，用户手动验收待完成。[详细编队证据](../../server/work/dfo-lan/docs/protocol/bakal-formation-20261006.md)。

### impl12 — 编队恢复后的成员等待城镇校验（2026-10-06）

- 用户10:02截图与同会话661确认团长编队窗口、红/黄队分配已恢复；先前Ozma连接是用户点错频道，已澄清。当前仍未收到2089，原生提示dstr101037245。
- 原生144CEC920→144CF4C80→144CF4F04比较成员+62的u16与源等待城镇值；144CDC190负责该字段读取。旧MemberArea=area1应为MemberTown=town152，按真实位置修正建团、1353查询与661回复；保留位置门禁。
- 回归覆盖152/1建团→152/2移动→成员记录旧值1→661更新为源等待城镇152。全仓vet、专项与启动25项测试/check通过，全仓仍两项既有character失败。最新location候选接入默认入口；开始攻坚待用户手动验收。

### impl13 — 成员area纠正与战斗修复候选

- impl12的城镇ID解读已被官服area6/交接area2与用户开战实机否定，应保留后续用户的MemberArea修正。当前hall-create版已实机开战并进入100003160。
- 最新104333会话8次CMD39因经验标签跨DGN头/maze合并歧义被拒，N38未发。经验与通关经验限定DGN头作用域，实际源header0/maze0.83回归通过；保留源字段校验。
- N2286初始化从旧交接state1修正为官服placement state0；HP刷新state2与死亡state1分支保留。开战N578改为实际RaidRecruitment，不再把actor2样本当运行成员。
- 默认combat候选接入原启动游戏.cmd；真实源专项、全仓vet、25项Python入口回归及check通过。全仓test仍仅两项已记载character失败。完整Buff关联/授予与移动怪定时波次仍未闭合，地图图标与尸体消失待用户手动验证。[战斗修复记录](../../server/work/dfo-lan/docs/protocol/bakal-combat-review-20261006.md)。

### impl13 — 官服wire否定城镇值：member+62是待机区域号（2026-10-06）

- 用户10:13实机（会话20261006_100853，建团成功、camp1/camp2间移动、无2089）证明impl12的town152解读错误：客户端仍弹"party members are not all in the same area"。
- C层官服直连铁证（captures/20261005-015111，01:55:15，频道92团本N578 moyius记录）：member+62 = 6——非城镇、非固定2，是该raid自己的待机区域号。B层佐证：巴卡尔开战burst N578（12:29:29）member+62 = 2，官服建团位置town 152 area 2，四个团本建团area全为2。
- 结论：客户端谓词144CF4C80比较的是成员记录的area与raid脚本待机区域的area号；巴卡尔待机区 = bakal.etc [WAITING ROOM] (152,2) = camp1.map。
- 修复：RaidRecruitment.MemberTown改回MemberArea，建团/1353/661三处写玩家当前area；bindBakalWaitingRoom弃用[open party list area]扫描（其area1/waitroom.map绑定会拒绝官服实录建团位置152/2），直接绑定ETC对。
- 回归翻转：建团必须在152/2（camp1），152/1被拒；成员记录u16 == WaitingRoomArea。新候选bin/wireprobe-bakal-waiting-area-candidate.exe接入默认profile（SHA256 1407FE611BD1E93D84414711811A3961916086976AC9A304274D7EEEE3A376E6）。全仓vet、专项、启动25项测试/check通过。用户需在camp1（Start Raid Commander所在区）建团并开战。

### impl14 — 用户纠正建团流程：大厅建团+营地编队开战（2026-10-06）

- 用户实证：攻坚队在waitroom（area1，Commander Irine）创建，右侧camp1（area2）编辑队员并开战。impl13把建团门禁收紧到area2是过度修复，用户10:33在waitroom建团十余次全被拒（会话20261006_103207，waiting_room_refused）。
- 修正OwnsWaitingRoom：接受area1与area2两个建团位（用户实测waitroom建团 + 官服12:14:06在152/2建团），seriagate闸口（area0）仍拒。成员记录保持玩家实时area：waitroom建团→area1，走至camp1后661/1353刷新为area2，开战谓词通过。
- 新候选bin/wireprobe-bakal-hall-create-candidate.exe接入默认profile（SHA256 0CC2DF71EA2863FE7F792BDD0D1B663EEDB2D5D92CEB624C51B3C0871BEB3293）。专项、25项启动回归与check通过。

### 2026-10-07 复活／消耗品预算与竞拍后端

当前源5币开局、回营至少2币和5药剂保底，source AddCoin/StackablePotion补充接入实际消费入口；C41成功减额并同步N2285剩余字段，C44源[stackable dungeon limit]标记限定物品在新事务中检查，已提交才减额，回执重放不重复扣。普通换房保持预算。普通／困难bidding_item、周竞拍数量／分组／物品权重已解析；已实现拥有者通关回执授权、一次冻结及单人扣款发奖基础事务，保留无关存档字段。不混入自动通关奖励。

竞拍网关仍未开放：现有官服记录缺真正Kind0 N2059/N2060/N2061与C1902交互，禁止拿其它军团同号Kind1 ACK作竞拍原生向量。出价、轮次、流拍和界面同步需要继续闭合；不宣称可用。详见server/work/dfo-lan/docs/protocol/bakal-budget-bidding-20261007.md。默认budget-bidding候选beccc7ad，专项、vet、25启动检查通过，全量仅两项既有character兼容失败；实机由用户操作，当前旧会话不替换。

### 2026-10-07 死亡／主动撤退恢复等待

原生C40→约10秒回营→N578成员AREA后u32恢复截止时间，第一轮15秒、第二轮30秒，另一新团再次15/30。144CDC190 member+64消费字段闭合，已从42字节保留区单独写出u32，剩余布局保持。等待档位从同源REVIVAL TIME15/30/45/60消费；死亡未复活和主动撤退累计，源KICK OUT DUNGEON NO PENALTY／指挥官回营不累计。复活成功取消死亡回营预约，预约绑定run与死亡序号并串行tick处理；排除普通N33死亡回城计时器。到期只发一次N578正常截止值，进图门禁与客户端Unix秒边界一致。

候选recovery，指纹63e345740142978c1c11eb396502697b5106b5baa18641afbfa7e2239b9f9dad，默认接线；专项、vet、25启动检查通过，全量仍两项既有character失败。详见server/work/dfo-lan/docs/protocol/bakal-recovery-20261007.md。用户仍手动测试旧会话；白屏及解除的实机画面尚未确认，不写成confirmed baseline。

### 2026-10-07 二阶段、同格点传送、作战元信息及布洛娜空房

011723会话2070循环拒绝来自错误的RoomCleared门禁：源PROC在HP阈值时切换，首领仍活着。新增同源PROC读取；验证源首arena/actor/解锁条件后写PHASE/HP、实际N29第二房、加载后创建第二模板，重复2070只ACK。同格点原生45 Record=1在源move map even enemy下复用场景，不重生actor。N578准备／作战／结束state/phase/开始Unix原先始终未更新，现按原生mode2同步；图示勾选退出警告仍待实机，不伪造Ispins窗口包。

021400用户确认二次开始不重复生成，精确recovery 63e34574另冻确认快照。02:19:26杀布洛娜、02:19:30退出、02:19:43重新进其空arena0,1，随后无出口请求直到02:20:38失败；已在该源位置尚未CREATE复活时改进普通路线0,0。原生回归覆盖。新候选cinematic-lifecycle a170aadf默认接线，显式DFO_PVF_CORE_TEST_ARCHIVE原生专项／vet／25启动检查通过，普通全量仍两项character失败。此前裸test跳过原生用例，本轮已补跑预算／恢复及Bakal专项并纠正验收说明。详细文档bakal-cinematic-lifecycle-20261007.md，未实机验收新候选。

### 2026-10-07 重开遗留怪物图标

023721三个run首领随机位置38→36→37；N2286是增量表，旧开场只发非空roster，空位置未清导致残留头像。新开场按源LOCATION先发type0/mode1清当前及历史图标，再发实际roster/INIT；142543160互证模式1写历史type。二轮左下41实际没有placement/rank3，不在空位强造首领。map-reset候选9e318988默认接线，显式原生专项、vet、25启动检查通过，普通全仓仅两项既有character失败。实际图标清空待实机确认，详见bakal-map-reset-20261007.md。

### 2026-10-07 用户确认与本体首次入口核对

用户确认二阶段及三类退出勾选确认窗完成，竞拍UI仍未接入。本体首次右侧小房间对应原生C2062的2,1；023721会话02:51:27走入中央1,1/map100007434后已发送109014482创建，同源LOCATION24 SPECIFIC XY也是1,1。未强制搬移首次入口，不把发送包当成客户端画面确认。补充巴卡尔专用命令完整日志保留、拒绝时所属房间状态、真实C2062→中央C45→加载→N2194回归。默认map-reset保持；诊断候选暂未接线。详见bakal-entry-review-20261007.md。提交卫生脚本仍缺失，未绕过门禁。

### 2026-10-07 三区首领与本体小房间出口审查

按真实归档maze0覆盖17图91房、176条有向相邻路线及759组清场返回目标，检查战斗中源MoveMapEvenEnemy门禁和清场后组包。三龙实际rank3 ConfirmDeath→源clear→2070→普通房加载通过；已clear禁止重新进图不妨碍当前图内返回，RaidManaged不进入普通Completed。既有二阶段存活切换/同格点传送回归通过。023721无2070拒绝，2062仅2次已clear/2次恢复等待，日志有三龙成功返回与本体第二模板加载。未发现新的清场后错误锁门，未改玩法或默认程序；不能把服务端返回矩阵当作全部客户端机关实机验收。详见bakal-room-routes-audit-20261007.md。

### 2026-10-07 125124实机Final Strike与右上UI复查

用户确认终幕循环，Skip才回营。12:58:49进入源100007771/109015394，约63秒Skip触发演员死亡及通关回营，不能作为自然结束通过；本地manager300000ms与raid源180秒兜底已取证，未有超过180秒的自然等待样本。12:57:20怒气1400→3400→5400→7400，末端11/22/51实际进攻怪各+2000，源失败满值8000，三路合计75个百分点。NPC截图全0，但N2288库存实际2/2/2/2/2→4/4/3/3/3，当前13B reader布局对应；缓存/显示时序缺口未闭合，未猜改协议或宣称修好。新增真实源逃逸惩罚/最终期限回归通过；默认map-reset保持。详见bakal-final-ui-review-20261007.md。

### 2026-10-07 NPC库存同步候选（attempt1/3）

原生库存/显示各有初始化清零路径；增加首次房间加载结束后的owned库存快照，N2288 usedkind25/targetffffffff，不触发技能或补库存，重复37/2073不重复发。C2072完整日志保留，新终幕加载事件记录源deadline/timeout/movie times。stock恢复候选8d9f5293接入默认及隔离profile，显式原生专项/vet/Python25检查/launch --check通过，全量仍仅两个既有character失败。NPC显示待实机；自然动画结束尚未闭合，未改源180秒兜底，不以库存候选冒充终幕修复。详见bakal-inventory-restore-candidate-20261007.md。

### 2026-10-07 建团窗口周次数2/1与后端拒绝不一致（attempt1/3）

缺失N1434使客户端getter返回-1，源上限1减-1显示2/1。已闭合原生1434完整796B向量、144CE0120 reader、144CF16E0(kind,1)入场已用值、144CF3450(kind,1,0)奖励已用值及1420B2C90减法显示链。新只读BakalWeeklyUsage共用于准入和UI，保持旧账本语义/周键/待领计划；场景就绪、建团前、结算后、重选及跨周同步，未知字段不猜填，不解除源周门禁。weekly-quota候选8177f587默认/隔离接线；真实归档专项、vet、Python25/launch --check通过，全量仍仅两个既有character失败。待用户手动重启验收窗口0/1。详见bakal-weekly-quota-sync-20261007.md。

### 2026-10-07 用户请求当前账号次数恢复CMD

根目录新增“恢复当前账号巴卡尔次数.cmd”，当前启动账号probe的所有角色仅清零clears/rewards，通过新增统一dfo-tool bakalreset子命令与既有ApplyGrant归属/事务/审计执行；week、plans、竞拍及其余存档保持。脚本要求先停止游戏环境，只准备存储；未替用户执行实际重置。双账号/同账号双角色临时SQLite审计与保存兼容专项、vet通过；全量仍两个既有character失败。工具已编译df61c03c，帮助分支已验证。详见docs/bakal-weekly-reset-tool-20261007.md。

### 2026-10-07 合并前产物清理

删除27个可重建文件（25份过期bin候选、1份无消费者临时exe、1份重复PVF导出），共978,113,827字节；它们原已忽略，不报成提交体积减少。当前默认/在用程序、两级回退、确认来源/快照、恢复工具及有消费者的隔离launcher保留。28份专用源码、26份必要回归、4份原生向量/说明列入合并清单；共享文件混合改动单独审核，不删除测试或其它用户改动。根恢复CMD补缺工具时源码构建，避免新检出依赖忽略exe。runtime/pgdata/postgresql.conf已跟踪的本地修改需排除提交；全量两项既有character失败和缺失提交卫生脚本继续如实列出。未暂存/提交/操作游戏。详见docs/bakal-merge-cleanup-20261007.md及bakal-merge-file-manifest-20261007.md。

### 2026-10-07 总仓合并兼容修复候选

cc03724e后当前profile漏bakal-raid，恢复域并保留其它策略；Go恢复入口按CSV首列检测所有wireprobe*.exe，包括handoff/candidate，补保护与help回归。配置/help契约对齐总仓VenusFlipGear及BoostUpEvent默认，用户覆盖仍生效。新scripts/bakal-reset.cmd使用Go修复启动器候选6b440363，原在用启动器未热替。build/vet、实际PVF专项/默认profile只读准备均通过（17 DGN/52位置/4 Boss，storage_accessed/runtime_started=false）；全量恢复到仅两个既有character来源失败。用户明确“不恢复了，全部按最新的来”，使用当前SQLite，旧PG不迁/不删。未操作游戏/写存档/提交；终幕自然结束与竞拍UI仍未完成。详见docs/protocol/bakal-post-merge-fix-20261007.md。
