# DFO 本地兼容服 · 36 轮开发方案（玩法闭合优先）

编写时间：2026-09-11。本文替代同名文件的首版（首版按 35 版文档的七项顺序、教程优先），按用户重新定的基础目标重排。

> **本文是施工前的方案。实际施工结果、定位到的根因、以及本轮未做的两项及其原因，见 `next36-status.md`——那份是当前状态的权威记录。** 本文第 11 节的缺陷清单已被用户实机反馈修正过：多选一奖励在低等级任务里一条都没有（并非问题所在），主线断点实际是 4873 的 `[meet npc]`、3156/3160 的 `[reach the range]` 和 3173 的 `[seek n meet npc]`。

**本文是方案，不是完成报告。** 文中"已核对"一律指静态读取源码与既有记录；**本轮没有重新运行游戏、没有重新跑测试**。所有性能判断标注为嫌疑，未经实测。

阅读顺序：`DFO-开发对接文档-35版.md` → 本文 → `next35-investigation.md` 顶部更新。

## 0. 本轮基础目标

用户定义的基础目标是**玩法闭合**：一个角色能反复完整跑完下面这个循环，不中断、不卡死、数值正确。

```
接任务 → 进图 → 刷怪 → 掉落拾取 → 通关 → 结算 → 回城 → 交任务 → 拿奖励 → 升级 → 解锁下一环
```

拆成可验收的六条：正常刷图、正常升级、能完成任务、任务奖励正确发放、刷图过程不卡顿、结算正常。其余 BUG 在主干闭合的基础上修复。

不属于本轮基础目标、但仍要做完的：新角色教程出生流程、推荐指引、装备穿脱验收、弓箭手技能页验收。这些排在主干之后（第 9 节）。

## 1. 与 35 版排序的差异

35 版文档把"新角色初始地图和任务"列为七项之一并排在接手顺序首位（A）。本轮把它降到最后一档，理由是：教程只影响**新角色的入口**，而主干循环坏了会让**所有角色**玩不下去；并且教程本身就是"进图→通关→回城→接首任务"的一个特例，主干稳了之后再接教程，风险和工作量都显著更小。

这是排序调整，不是取消。教程的状态机设计（原方案 P1 的全部内容）完整保留在第 9.5 节，随时可以前置。

## 2. 硬约束（全程适用）

来自用户与 35 版文档，本轮不得突破：

1. 数据与规则优先取自本客户端 PVF、配置与原生协议；90US/90CN 参考只看思路，消息字节、内存地址、状态编码一律不照搬。
2. 不重置、不迁移、不清空已有角色（LanTest01、6666 等）与 PostgreSQL；任何新增持久化对既有角色做**幂等回填**，回填后行为与现在一致。
3. 不覆盖 `F:/dnfop/DFO`；隔离客户端 PVF 已是 UI 补丁版，服务端规则仍追溯 `runtime/pvf_source/Script.inner.pvf`。
4. 编译产物用新文件名（`bin/wireprobe-dungeon36.exe`），不覆盖运行中的 35 二进制；启动参数与配置一起版本化（candidate36 / `*.current36.json`）。
5. 项目无 Git 历史：改动前对将被修改的源文件、配置、PVF 留备份与 SHA-256，记入 `docs/protocol/next36-status.md`。
6. 凭据（`runtime/storage/local.json`）不出现在文档、日志、截图、对接消息。
7. 合并测试：代码侧用 `go test ./...`、`go vet ./...`、`go run ./cmd/charactercheck` 收敛，实机集中做，全程只安排两次用户操作（第 10 节）。每次实机前书面列出要采什么、为什么非采不可。
8. 不把"发包成功 / 没报错 / 原生模拟通过"写成"功能完成"。验收记录格式固定为"角色、等级、动作、实际结果、日志时间"。
9. 性能优化不得以牺牲事务性、幂等性或断线恢复为代价；任何改动持久化时机的改法，charactercheck 必须新增对应的断线重连用例。

## 3. 主干链路与协议帧映射

这是本轮所有工作的坐标系。下表从 `cmd/wireprobe/main.go` 的分发逻辑与各 flow 文件核对得出。

| 环节 | 客户端请求 | 服务端应答 | 入口函数 |
|---|---|---|---|
| 城镇移动 / 切区 | CMD35 / CMD36 | ACK + NOTI23 + NOTI24 | `world_flow.go` `handle` |
| 接 / 放弃任务 | CMD31 / CMD32 | ACK | `main.go` 内联 → `quest.Service.Accept` |
| NPC 会面目标 | CMD33 | NOTI291 | `quest_flow.go` `questInteraction` |
| 副本门 | CMD15 (u32=0) | ACK15 + NOTI27 选本态 | `dungeon_flow.go` `dungeonGate` |
| 选本 | CMD16 | ACK16 +（NOTI9 可选）+ NOTI28 + NOTI29 | `selectDungeon` |
| 载入完成 | CMD37 (16 字节全零) | ACK37 + NOTI3 + NOTI30 + NOTI37 + NOTI36 | `finishDungeonLoading` |
| 怪物死亡 | CMD39 | ACK39 + NOTI38 +（NOTI37 经验）+（NOTI21 升级时）+ 通关包 | `monsterDeath` |
| 拾取 | CMD43 | ACK43 + NOTI39 + NOTI14 | `loot_flow.go` `pickup` |
| 房间移动 | CMD45 (160 字节) | ACK45 + NOTI29 | `moveDungeonRoom` |
| Boss 确认 | CMD117 | 通关包 | `bossCheck` |
| 通关包 | — | NOTI291 + NOTI115 + NOTI31 | `completeDungeon` |
| 结算 | CMD46 | NOTI34 + NOTI37 + NOTI35 + NOTI21 | `settlement_flow.go` `dungeonResult` |
| 卡片 | CMD69 / CMD70 / CMD71 | 各自 ACK | `card_flow.go` |
| 结算退出 / 下一房 | CMD72 | ACK72（+ 新副本会话） | `settlementExit` |
| 放弃退本 | CMD42 | ACK42 + NOTI3 + NOTI23 + NOTI24 | `leaveDungeon` |
| 选本态返回 | CMD132 | NOTI132 + 回城包 | `returnFromDungeonSelection` |
| 剧情暂停 | CMD191 | NOTI170 | `main.go` 内联 |
| 交任务 | CMD34 | ACK34 + NOTI13 + NOTI37 + NOTI291 + NOTI342 + NOTI21 | `quest_flow.go` `finishQuest` |

入场序列（SELECT 之后）固定为 `entry_flow.go` 的 `entryPayloads.packets()`：ACK4 → NOTI2826 账户选项 → NOTI2 基础 → NOTI2 附加 → NOTI19 技能 → NOTI13 金库 → NOTI13 背包 → NOTI13 穿戴 → NOTI23 → NOTI24 → NOTI36 疲劳 → NOTI124 完成 → NOTI37 经验 → NOTI342 已完成任务 → NOTI21 可接任务。

## 4. 代码核对结论与新发现

对照 35 版文档逐项核对源码，文档说法均属实。以下是核对中补充的、文档未写明且影响本轮排序的事实：

| # | 发现 | 位置 | 影响 |
|---|---|---|---|
| 1 | `Available` 是全目录线性扫描：对每条任务调 `InitialProgress`（解析 ObjectiveCells）并 `cells(d.Script.Cells,"[grow type]")` 遍历整个脚本 cell 数组 | `internal/quest/available.go` | 卡顿首要嫌疑 |
| 2 | `availableQuestPayload` 在**四处**同步调用：入场 SELECT、升级瞬间（`monsterDeath`）、结算（`dungeonResult`）、交任务后（`finishQuest`） | `quest_flow.go` + 三处调用方 | 卡顿点与"升级卡顿"现象吻合 |
| 3 | `Available` 对 `InitialProgress` 报错的任务直接 `continue` 跳过 | `available.go` | 后续任务"消失"不只是等级门槛，未实现的目标类型同样会让任务不出现 |
| 4 | 战斗热路径每只怪一次独立 DB 事务（`progression.Monster`），每次拾取一次（`loot.Pickup`），各带 5 秒 context | `dungeon_flow.go` / `loot_flow.go` | 打怪手感；与 1、2 叠加 |
| 5 | 怪物经验走 `experience.compat90.json`（model `reference90-solo-v1`），而**通关经验** `DungeonClear` 与**任务经验** `QuestExperience` 都已用当前源 `n_quest/questparameter.etc` | `internal/progression/` | 三种经验来源不一致，升级节奏对不上原版 |
| 6 | 重复结算 `if w.resultSent { return nil, nil }`、重复交任务 `if w.answeredQuests[r.ID] { return nil, nil }` 均返回空 plan，**不发任何响应包** | `settlement_flow.go` / `quest_flow.go` | 幂等正确，但客户端可能空等；原生期望需核对 |
| 7 | 教程目录虽已导出，但 `cmd/wireprobe` 没有任何 `-tutorial-*` 参数，`channel_probe.py` 的 candidate35 也不传，目录**完全未加载进服务端** | `main.go` flag 区 / `channel_probe.py` | 教程缺口比"状态机没接"更靠前 |
| 8 | 客户端自己有教程 CMD15 发送端 `146cce650`（发源副本 ID 而非 0），SELECT 响应的 `tutorial_flag` / `tutorial_completed` 由 `146CC6FD0` 读取并推导阶段 | `protocol/dungeon.go` 注释 + `dfo_probe_tools/tutorial_entry35.asm` | 服务端只需正确应答，不必像 90US 那样硬推入场包 |
| 9 | 请求解码校验很严：CMD37 必须 16 字节全零、CMD45 必须 160 字节且 `p[10]<=1`、CMD39 长度须 `>=64 && <=4096 && %8==0` | `protocol/dungeon.go` | 遇到未见过的合法变体会直接拒绝，表现为卡载入 / 过不去门 / 打死怪没反应 |
| 10 | `dungeon.Session` 各方法都有 `s == nil` 防护（`BossCheck`、`Completed`、`RoomCleared`、`ConfirmDeath`） | `internal/dungeon/` | 无活动副本时发 CMD117 不会 panic，这一类崩溃风险可排除 |

### 排查方法论

`events.jsonl` 里的 `*_refused` / `*_rejected` 事件本身就是缺陷清单的自动生成器。服务端每次拒绝都会记 `reason`，例如 `dungeon_request_refused`、`dungeon_gate_rejected`、`quest_submit_refused`、`equipment_move_refused`、`world_rejected`、`skill_refused`。阶段 A 的第一件事就是把一次完整刷图产生的全部 refused/rejected 事件按 `reason` 归类——卡在哪一步、为什么卡，绝大多数会直接写在里面，不需要猜。

配套的耗时基线同样从 `events.jsonl` 取：每个 `client_frame` 到该请求最后一个响应包之间的时间差即服务端处理耗时。

## 5. 阶段 A：闭环取证与缺陷落地

**目标**：在改任何代码之前，把主干循环的真实状态和卡点数据化。

**做法**：用现有 35 环境，让 LanTest01（6 级，已完成 3146）走一遍完整循环，只采数据不做判断：接一条当前可接的任务 → 进图 → 打完第一个房间 → 切下一房 → 通关 → 结算 → 退本 → 回城 → 交任务 → 看奖励 / 经验 / 等级。

**同一趟顺带采集**（避免为这几项单独占用一次操作）：怪物 HP 三组快照（进房未攻击 / 命中后 / 死亡前，用 `watch_monster_stats30.py`）、修正后的 PartyManager 状态（金币浮字用）、装备穿脱一次、弓箭手 6666 技能页画面。

**产出三份**：

1. **链路状态表** —— 第 3 节每一行的通过 / 失败 / 未触发，失败的附 refused reason。
2. **耗时基线** —— 每类请求的处理耗时，重点标注升级帧、结算帧、房间切换帧。
3. **缺陷实例表** —— 第 11 节清单里每条的实测状态（复现 / 未复现 / 新增）。

这一趟不改代码、不重启服务、不清数据。

## 6. 阶段 B：数值正确

刷图和升级的地基。三件事，可并行。

### 6.1 怪物 HP 倍率

现状：普通怪未缩放 HP 652/733/817，有效 9/10/11，描述块 +0x30 倍率 ≈ 0.0143。写入函数 `145c08420` 从 `actor+6808` 复制倍率，调用者 `145c26fc0`、`142a1f0d0`，条件函数 `145d724f0`。

**已有证据文件（先读，不要重复 dump）**：34 运行目录的 `monster-hp-evaluated35.json`，以及 `dfo_probe_tools/` 下的 `hp_revision35.asm`、`hp_revision_origin35.asm`、`hp_revision_callers35.json`、`hp_limit_sources35.asm`、`hp_limit_predicate35.asm`、`hp_rate_setters35.asm`、`hp_rate_field_refs35.txt`、`hp_cap_rate_refs35.txt`、`hp_multiplier_writer_refs35.txt`、`monster_hp_direct_refs35.txt`、`monster_hp_fields30.asm`、`monster_hp_setup28.asm`。

**假设按可能性排序**：

1. `actor+6808` 由 NOTI29 怪物记录中的未命名字段初始化。`protocol.StartMap` 每条怪物写 `SourceIndex / Entity / Template / Level / Rank / 0,0,255 / Team / 0`，其中 `0,0,255` 三字节与末尾 `0` 从未被命名。静态入口：沿 `1452b7c3c → 145b0d910 → 145b219d0 → 145b144f0` 找谁写 `+6808`。
2. 倍率来自 NOTI28（`DungeonInfo`）的未命名 u16/u32 字段（难度、随机地狱位置之外的部分）。
3. 客户端按队伍人数或副本难度本地计算，`actor+6808` 只是缓存——这种情况修复点不在包字段，而在提供正确的难度 / 队伍上下文。

**判据**：有效 HP 等于源 PVF 怪物规则算出的值；受击、死亡、经验一致。**禁止固定乘 100 掩盖问题。**

**注意**：`dump_containing` 对分段函数只能取到一小段，中间地址必须落在指令边界；部分工具会覆盖 `startup_mode.asm` / `latest_xrefs.json`，关键输出另存独立文件。

### 6.2 经验模型统一

现状是三套来源混用：怪物经验 compat90，通关经验当前源，任务经验当前源。

从当前 PVF 补出怪物经验表与倍率（`cmd/progressionimport` 已导出等级 / SP 表，扩展其怪物经验相关段），输出 `configs/experience.current36.json`；`experience.compat90.json` 保留可切换，便于对照。

**判据**：击杀经验、通关经验、任务经验三者与原生结算界面显示一致。已有角色的累计经验不回滚，只影响后续增量。

### 6.3 任务奖励闭合

`ItemRewards` 的 `[job]` / grow 过滤与 `inventory.Awarder` 已在。要确认的是：

- `internal/quest/progress.go` 的 `ErrRewardPending`（奖励应用未实现）在 3146 那条链上是否会被触发，触发条件是什么。
- 经验、金币、道具三类奖励必须在**一次事务**里闭合，失败整体回滚，不允许出现"任务标完成但奖励没给"。
- `finishQuest` 里 `result.Applied` 为 false 时不发 ACK34，只发后续 NOTI——核对这是否是原生期望（重连恢复路径）。

## 7. 阶段 C：任务链闭合

### 7.1 目标类型扩展（主要工作量）

`InitialProgress` 现在只认单图 `[clear map]` 和单 NPC `[meet npc]`，其余返回"未实现"；而 `Available` 会把报错的任务直接跳过。合起来的效果是：**目标类型没实现的任务在列表里根本不出现**，表现为"没有后续任务了"，而不是"任务接不了"。

做法：用 `cmd/questaudit` 统计 3146→3151 这条链、以及 1–15 级各职业主线所需的全部目标类型与出现频次，按频次补齐。预期覆盖面最大的几类是多图通关、多 NPC 会面、猎杀指定怪、收集道具。每补一类配一个 charactercheck 用例。补不了的继续明确返回未实现，**不静默吞掉**——同时给 `Available` 加一条 event，把被跳过的任务 ID 与原因记进 `events.jsonl`，便于下一轮定位。

### 7.2 等级门槛的展示语义

3147 要 7 级，之后 3148/3149/3150/3151 分别要 8/10/11/12 级，这是原版规则，**不放开**。要确定的是原生客户端对"未达等级任务"的期望：是根本不出现在 NOTI21 里，还是出现但客户端自行标灰并显示所需等级。现在 `Available` 直接过滤，如果原生期望展示，玩家看到的就是"没任务了"。

静态入口：客户端任务列表渲染函数对 NOTI21 列表项的等级字段处理。`protocol.AvailableQuests(level, ids)` 已经带了等级参数，先确认这个参数在原生侧的用途。

### 7.3 任务与副本的连接

`dungeon.Select` 里 `r.Quest != 0 && !accepted[uint16(r.Quest)]` 要求任务已接受，且 maze 的 `[quest connection]` 必须唯一匹配。任务本需要确认：接了任务后选本，客户端发的 `r.Quest` 是否与 maze 的 quest 字段一致；`chosen.Pending` 非空时会拒绝，这些 Pending 原因要在阶段 A 的日志里看有没有出现。

## 8. 阶段 D：流畅度与结算

### 8.1 先定指标

用阶段 A 的耗时基线做起点，定三个数：房间内普通请求处理 p95、房间切换到 NOTI29 送达、升级帧与结算帧的处理耗时。改完再用同一套动作复测，两组数字并列写进验收记录。没有基线就没有"修好了"的判据。

### 8.2 `Available` 预计算索引（优先做，风险最低）

服务启动时把任务目录预处理成索引：按职业和等级分桶，把每条任务的"`InitialProgress` 是否可解析"、"grow type 约束"、"前置任务集合"这些**静态属性**预先算好缓存。运行时只做"前置是否已完成"的动态交集，从几千次 cell 遍历降到一次哈希查表。

这个改动不涉及协议、不涉及持久化、不改变对外行为（同样的输入必须产出同样的 ID 列表），收益可能最大。做法上加一个 `internal/quest/index.go`，在 `quest.Service` 构造时建索引，`Available` 改为查索引；配一个测试断言"索引版与原版对同一角色返回完全相同的 ID 列表"，用现有角色数据跑。

### 8.3 战斗热路径的 DB 往返

把每只怪一次事务改为副本内累计、在天然边界落库：房间切换（CMD45）、通关（CMD117/最后一只怪）、退本（CMD42/CMD72）。边界选到每房间一次，断线最多丢一个房间的增量，配幂等收据保证重放不重复计。

**这个改动动持久化，风险最高，必须最后做，并且 charactercheck 必须新增断线重连恢复用例。** 如果 8.2 做完实测已经不卡，这一项可以降级为"记录待办"，不必强行改。

### 8.4 结算

- 计时 `receipt.Elapsed` / `BestElapsed` 与原生显示是否一致。
- 重复点击结算：现在返回空 plan 无响应（幂等对，但可能空等）。核对原生期望——若需要回一个 ACK，补一个不含奖励的应答分支。
- CMD72 的两个分支（下一房 / 退出）状态清理：`activeDungeon`、`drops`、`deathSent`、`completionSent`、`resultSent`、卡片状态、`selectingDungeon` 是否都清干净，`main.go` 里那段 `p.Name == "settlement_exit_ack"` 的处理要逐字段核对。
- 返回按钮不掉线（35 版验收清单第 6 条）。

## 9. 阶段 E：其余缺陷

按对主干的影响排序。

### 9.1 金币头顶浮字

金币已入账，NOTI39（party0 flag1 amount extraCount0）与 NOTI14 已发，从未发过 NOTI9。用阶段 A 采的 PartyManager 数据判断浮字分支到底读哪个字段；若确需 slot0 初始化，评估 `SoloPartyInfo`（99 字节，开关 `-solo-party-bootstrap`）的发送时机，并核对 `selectDungeon` 里 `r.Party==1 → 65535` 的映射不会影响正常副本。

**只有实机看到头顶数字且入账一致才算修复**，打开开关没报错不算。

### 9.2 装备穿脱验收

代码已接线（C1/19 → `equipmentState.handle`），缺实测：穿脱、交换、拒绝不合职业 / 等级、数量不变、属性与外观、重登 NOTI13 恢复。`equipment.current35.json` 的 1536 条只覆盖基础装备，复杂品级 / 强化明确列为未做。

### 9.3 弓箭手技能页验收

PVF 已补 3/7/511 到 `[archer] none` 段，`ForAdvancement` 已修。缺实测：新布局显示、4 级只可学 3（7 需 10 级）、拖动、加减点、学后快捷栏、重登保存。

### 9.4 推荐指引

198=1 已接线并实测到达客户端。缺：关闭后 / 换角色 / 重登 / 提交任务后不再反复弹的验收；CMD2377 一般设置保存未实现（先 dump 请求布局，再决定落库到账户级还是角色级）。

### 9.5 新角色教程出生流程（设计保留，可随时前置）

**目标**：新建普通职业角色首次进入游戏，按 `tutorial-routes.current35.json` 的路线走片头（`intro_after_tutorial=false` 先播、`true` 后播）→ 本职业教程副本 → 回到 `Town 38 / Area 0 / (1677,222)` → 可接第一条任务；重登不重播；既有角色不受影响；活动专属路线（枪手 grow5 → 100002493 / 任务 12486）不被普通角色触发。

**已有资产**：`internal/catalog/tutorial.go`（`ParseTutorialFlows` / `Normal` / `ImportTutorials`）、`cmd/tutorialimport`（输出 `runtime/tutorial-source35/routes.json` 与 `dungeons.json`，65 张地图）、`configs/tutorial-routes.current35.json`（15 条普通 + 1 条活动；鬼剑士 7115、弓箭手 100003327 必须 uint32；普通路线均无 `first_quest`，首任务只能来自任务目录）。客户端证据：`tutorial_entry35.asm`、`tutorial_entry_chain.asm`、`tutorial_flags.asm`、`tutorial_ack.asm`、`tutorial_accept_sender.asm`。

**必须先回答的六个问题**：

| # | 问题 | 静态入口 |
|---|---|---|
| Q1 | 客户端何时调 `146cce650` 发教程 CMD15？由 SELECT 的 `tutorial_flag` / `tutorial_completed` 决定，还是入场后某 NOTI？ | 回溯 `146cce3d0 → 146cce650`；`146CC6FD0` 里 `[rax+0x14]`、`ecx<0x1f`、`1459b53c0(…,1)` 的含义 |
| Q2 | 教程 CMD15 之后客户端期望什么？ACK15+NOTI28+NOTI29（跳过选本态），还是仍需 NOTI27 再 CMD16？ | CMD15 应答处理器的教程分支 |
| Q3 | 片头由客户端按 PVF `[intro path]` 自播还是需服务端信号？两种顺序如何切换 | 客户端对 `Video/Movie/*_tutorial_intro.avi` 的引用条件 |
| Q4 | 教程结束信号是什么：CMD143 某 flag、CMD46 结算、CMD42、CMD72，还是别的？ | `tutorial_flags.asm`、`tutorial_accept_sender.asm`(14519fe80) |
| Q5 | 返城包是否与 `leaveDungeon` 一致，坐标改为路线的 Town/Area/Position | 现有 `leaveDungeon`、`world.Service.Enter`、`Store.SaveWorld` |
| Q6 | 首任务：`Available` 在 level=1、本职业、无前置时返回什么 | `configs/quests.generated.json` + `cmd/questaudit` |

**状态机**：新增表 `character_birth`（`character_id` PK、`stage smallint`、`dungeon integer`、`route_version text`、`updated_at`），置于 `internal/database/birth.go`。**迁移时把所有现有角色一次性回填 `stage=3`（已完成）**，只有迁移之后新建的角色才是 0——这是"不重置已有角色"的实现方式，charactercheck 必须有断言。阶段定义：0 新建未开始 / 1 已进教程副本 / 2 副本已结束未回城 / 3 完成。SELECT 时读 stage 决定走教程模式、回城模式还是现状。不复用 `character_tutorial_flags`（那是 CMD143 的 UI 指引 flag，与出生进度是两回事）。

**代码改动**：`storage/birth.go`（新）、`catalog/tutorial.go` 加 `LoadTutorialRoutes`、`dungeon/session.go` 加 `SelectTutorial`（允许 `d.Tutorial`、跳过最低等级 / 队伍 / 任务连接检查、`NoFatigue` 生效）、`dungeon_flow.go` 加 `tutorialGate` 与 `finishTutorial`、`main.go` 加参数与 stage 分支、`charactercheck/birth_check.go`（新）、`channel_probe.py` / `launch_local.py` 的 candidate36 参数。

**前置风险**：65 张教程地图可能含现有 `fixedMonsters` 不支持的怪物行（随机怪、未知 option），会被直接拒绝。**接教程前先用导入器对全部 65 张图跑一遍 `fixedMonsters`，把拒绝原因列成清单**，在这一步暴露而不是在实机上黑屏。

## 10. 实机操作安排

**第一次（取证）**：阶段 A 那一趟完整刷图，顺带 HP 三组快照、PartyManager 状态、装备穿脱、弓箭手技能页。操作前书面列出步骤与要看的现象，控制在一次登录内。

**第二次（验收）**：阶段 B/C/D 与 E 的代码完成、`go test/vet/charactercheck` 全过之后，按下面的清单一次走完，记录"角色、等级、动作、实际结果、日志时间"。某项失败则修正后只补测该项，不要求整轮重来。

**验收清单**（主干在前，35 版第 8 节的其余条目并入）：

1. 主干循环：接任务 → 进图 → 刷怪 → 拾取 → 通关 → 结算 → 回城 → 交任务 → 奖励到账 → 升级 → 下一环可见可接。连续跑两轮不中断。
2. 刷图流畅：房间切换、打怪、升级、结算无可感卡顿；耗时数字对照阶段 A 基线。
3. 数值：怪物满血、受击、死亡、掉落、经验、疲劳与源规则一致；金币入账与头顶浮字一致。
4. 结算：计时、评级、奖励、返回城镇；重复点击不重复结算、不掉线。
5. 任务：接取、推进、提交、奖励、后续任务可见；未达等级提示合理；推荐指引不反复抢占界面。
6. 装备：穿脱合法装备、拒绝不合职业 / 等级、数量不变、属性外观正确、重登一致。
7. 弓箭手 4 级未转职：基础技能显示、已学可用、未达等级不可学、拖动加减点、学后快捷栏、重登保存。
8. 新角色（若教程已接）：片头 / 初始地图 / 教程任务顺序正确、完成后返城、重登不重播。
9. 基础设施：启动、选频道、进城、换角色、重进无黑屏回退和频道错误；赛丽亚房间进出、背包、设置、金库、角色删除、退出游戏无锁死闪退；小地图、下一房间门正常。

Sky 截图工具持续报 `SetIsBorderRequired 0x80004002`，不能代点游戏、不猜坐标；需要画面证据时明确请用户截图。

## 11. 待确认缺陷清单

分三档。**已确认**指 35 版文档记录的用户实测现象；**强嫌疑**指本轮读代码推断、未经实测；**待核对**指需要实机或进一步静态分析才能定性。

### A 档 · 已确认（用户实测）

| # | 现象 | 现状 |
|---|---|---|
| A1 | 怪物 HP 过低（有效 9/10/11），刷图不正常 | 根因未定，证据充分 |
| A2 | 金币拾取无头顶数值 | 已入账，显示未通 |
| A3 | 后续任务看不到（3147 起） | 等级门槛 + 目标类型未实现，双重原因 |
| A4 | 推荐指引窗口反复自动弹出 | 198=1 已接线，待验收；CMD2377 未实现 |
| A5 | 装备不能穿戴 | 35 已接线，待实测 |
| A6 | 弓箭手基础技能页不显示 | 35 已补 PVF，待实测 |
| A7 | 新角色直接进城，跳过教程 | 目录未加载 + 状态机未接 |
| A8 | 升级卡顿 | 35 版列为历史回归项，根因未定 |
| A9 | 频道黑屏 / 频道错误 | 34 版修复，需回归 |
| A10 | 结算计时与返回按钮异常 | 35 版列为历史回归项 |

### B 档 · 强嫌疑（读代码发现，未实测）

| # | 判断 | 位置 |
|---|---|---|
| B1 | `Available` 全目录扫描 + 每条任务解析 cell，在升级 / 结算 / 交任务 / 入场四处同步调用 → A8 升级卡顿的首要嫌疑 | `quest/available.go`、`quest_flow.go` |
| B2 | 战斗热路径每只怪一次独立 DB 事务，每次拾取一次 → 打怪手感发涩 | `dungeon_flow.go`、`loot_flow.go` |
| B3 | `Available` 静默跳过 `InitialProgress` 报错的任务 → A3 的第二个原因，且日志无记录 | `available.go` |
| B4 | 三种经验来源不一致（怪物 compat90、通关与任务用当前源）→ 升级节奏对不上原版 | `internal/progression/` |
| B5 | 请求解码严格校验（CMD37 须 16 字节全零、CMD45 须 160 字节、CMD39 长度 %8）遇合法变体直接拒 → 可能表现为卡载入 / 过不去门 / 打死怪无反应 | `protocol/dungeon.go` |
| B6 | 重复结算、重复交任务返回空 plan 不发任何响应包 → 幂等对，但客户端可能空等 | `settlement_flow.go`、`quest_flow.go` |
| B7 | 教程目录完全未加载进服务端（无 `-tutorial-*` 参数）→ A7 的最前置原因 | `main.go`、`channel_probe.py` |

### C 档 · 待核对

| # | 待核对项 |
|---|---|
| C1 | `loot.Pickup` 在 `w.drops` 为 nil（未击杀直接拾取）时的行为 |
| C2 | 结算计时 `Elapsed` / `BestElapsed` 与原生显示是否一致 |
| C3 | 卡片流程 CMD69/70/71 的完整性与 `cardPlan` 冻结时机 |
| C4 | CMD72 两个分支的状态清理是否覆盖全部字段 |
| C5 | 小地图、疲劳显示、金库、背包、设置的当前实际状态 |
| C6 | 原生对"重复点击结算"期望的响应形态 |
| C7 | `chosen.Pending` 非空导致的选本拒绝在实机是否出现过 |
| C8 | `finishQuest` 中 `result.Applied=false` 分支（重连恢复）是否符合原生期望 |

**待用户补充**：实际游玩中遇到但上表没有的现象，按"角色、等级、在哪一步、看到什么"描述即可，会并入对应档次。

## 12. 交付物与回退

- `bin/wireprobe-dungeon36.exe` 及 SHA-256；`configs/*.current36.json`；`docs/protocol/next36-status.md`（备份、哈希、证据、验收记录）；`next36-investigation.md`（调查过程，顶部更新优先）。
- 回退：后端回退到 35 必须同时回退 candidate 参数（36 的新参数 35 二进制不认识）；新表 `character_birth` 对 35 无害可保留；PVF 回退沿用 35 版第 9 节流程，仅限隔离客户端。

## 13. 风险与未决

- B1 是代码层面推断，不是实测结论。若阶段 A 的耗时数据显示卡顿不在这里，8.2 的收益要重新评估，排查方向转向包顺序与客户端等待超时。
- HP 倍率若来自客户端本地表（6.1 假设 3），修复点不在服务端包字段，工作量和不确定性都会显著上升。
- 任务目标类型是开放集合，本轮只承诺 3146→3151 链与 1–15 级主线所需类型。
- 8.3 改持久化时机风险最高，若 8.2 已解决卡顿则降级为待办，不强行改。
- 怪物经验切到当前源后，已有角色累计经验不回滚，只影响后续增量。
- 教程 Q1/Q2 若静态回溯无法闭合，只能靠实机断点确认，届时需要新建角色的额外一次取证。
