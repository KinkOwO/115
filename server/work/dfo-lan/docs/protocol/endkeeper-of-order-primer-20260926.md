# 小深渊「秩序守护者」（100005014）天平机制取证

> 日期：2026-09-26
> 副本：`contents/2026/endkeeperoforder/dungeon/endkeeperoforder.dgn`（dungeon 100005014）
> 领主：`109019266`（`monster/named/scale_primer/*`，天平·引子）、`109019280`
> 目标：解释实机「天平能跳伤害数字但打不死」，并判定哪些环落在服务端
> 取真源：内层 PVF `server/work/client-build/Script.inner.pvf`（只读）；全部结论为 **cell 级**（见 §8）
> 状态：**机制已读到 cell 级；「进度如何累加」仍在客户端引擎侧，未决（§9）**

---

## 0. 一句话结论（2026-09-27 最终版）

**闭环只差一个数字。** 两个天花板 `c:primer_rarity_progress_max` / `c:oath_rarity_progress_max`
都由引擎内建函数 `getPrimerGrade()` / `getOathGrade()` 给出，实测**都是 72**；而机制的值域只有
`40..45`（8 档中的 6 档）+ `70/71`（rainbow1/2），脚本能赋的最大值是 **`71`（primer）/ `45`（oath）**。
死亡路径要求 `now >= max`，所以**只要 `max = 72` 就结构性不可能成立 —— 与玩家怎么打无关**。

实机两场（`…232505` / `…011956`）的天平状态**逐字冻结、两场完全相同**：
`triggers 1,0`（**`die_trigger=0`**）· `rarities 71, 72, 44, 72` · `omen 1,40,0,0` · `Action 20014` · HP 冻在 2%。
⇒ **脚本侧已经把档位推到自己的上限了**（`primer_now=71` 正是 rainbow2、最后一档），够不到的是天花板。

### 0.1 阻塞清单

| 级别 | 缺什么 | 说明 |
|---|---|---|
| **A · 唯一的硬阻塞** | `getPrimerGrade()` / `getOathGrade()` 返回**合法档位码**（≤71；正常应在 42–45） | 尚不知它读什么。**誓约装备已排除**（穿/脱两场日志逐字不变）；同族全是 EOO 副本进度 getter（含 `getDungeonFreeEntryClearCount`）⇒ 疑为**服务端该发的玩家/队伍进度**，我侧零实现 |
| **A 的副作用** | 召唤选路 | `nox_index_checker`（`oath_max==45` / `<45 && primer_max==45`）与 `nox_pos_checker`（`or(…,42,44,45)==1` / `==43`）全以 `{42,43,44,45}` 为条件 ⇒ **同被 72 打死** |
| **B · 次级（被 A 掩盖）** | `oath_now` 升到 `45`(primitive) 需要 `c:nox_die=1` ⇒ 奥尔泰尔必须登场并被击杀 | 召唤另需 `gm_trigger_on`（**某玩家 `t:getX() < 800`**）——**未验证** |
| **C · 产出（与击杀闭环无关）** | 小深渊征兆四档掉落（`109137366`–`109137369`） | 实测 `omen cnt_max=0`；服务端对 omen/oath **零实现** |
| **D · 服务端功能** | 115 级能力 / 赛季等级 / 引子-誓约等级系统 | 零实现；很可能就是 A 的输入源 |

### 0.2 最可能的死循环形态（推论）

`primer_now(71) < primer_max(72)` ⇒ 每帧都走 `only_primer_up`（升档），而 primer 已在脚本上限 `71`
⇒ **永远升不上去、也永远到不了 `==` 分支** ⇒ 状态彻底冻结（与观测到的「8 条快照一字不变」一致）。

### 0.3 已作废的说法（别再引用）

- 「开局 `[HP LIMIT ON] 99 PERCENT` 锁血」——**血能掉到 2%**，锁的是默认 `OFF` 的 `DIE_TRIGGER`。
- 「客户端本地创建、服务端无记录的对象会退化成可拾取物品」——全 30 个会话逐条解码，
  拾取**全是金币/物品**，见 **§11.1**。

---

## 1. 副本结构：两个 maze 是「权重二选一」，不是前后两关

```
maze 0  [size] 1 1   [maze chance rate] 992857   [map specification] "boss" 0 0 100016614
        [seal door map index] 100016614   [clear condition] [hunt boss] 109019266 1
maze 1  [size] 1 1   [maze chance rate]   7143   [map specification] "boss" 0 0 100016615
        [seal door map index] 100016615   [clear condition] [hunt boss] 109019266 1
```

- 权重合计恰为 1 000 000（992857 : 7143 ≈ **99.3% : 0.7%**）——与奖励表的百万不变量同源。
- **两个 maze 的通关条件相同**：都是打死 `109019266`。
- 每个 maze 只有 **1 个 room**（`size = 1 1`）⇒ 没有「相邻房间」可走，
  推进只能靠地图的**横向滚动**（`[arcade scroll area]` + `[arcade scroll check type] "not exist enemy"`）。
- 100016614 = `map/100016614_normal.map`；100016615 = `map/100016615_special.map`。

### 1b. 两张地图的差异只有一处

`[monster]` 段**逐字相同**（13 行）：

| 行索引 | 模板 | 选项 | `[monster create trigger]` |
| --- | --- | --- | --- |
| 0–10 | `109019402/109019403/109019404` | `[fixed] [normal]` | `1` |
| **11** | **`109019266`（天平）** | `[fixed] [boss]` | **`2`** |
| **12** | **`109019280`** | `[fixed] [normal]` | **`2`** |

后两行同坐标 `(3168, 191)`，且 `[event monster position]` **13 个值全为 0**。
`[passive object]` 段：map614 = 11 个，map615 = 12 个，**唯一差别是 map615 多
`109134975` @ `(3168, 190)`**（与天平同点）。

> ⚠️ `[monster create trigger]` 的语义**尚未确立**，不要照着猜：
> 全目录 369 张带该段的 map 里，**117 张非单调**（如 `1 1 2 2 1 1 2 2`、含 `-1`），
> 且 `max(trigger)` 与副本 maze 数**不吻合**（3200 个副本里只有 13 个相等）。
> 我最初「trigger = 房间序号」的假设**已被这两项统计否掉**，不采纳。

---

## 2. `109134975` = 天平入场演出装置（`special_entrance.obj`）

```
[basic action] ./Action/Basic.act
[etc action]   ./Action/Stay.act ./Action/Shade.act ./Action/End.act
[create var]   `o:scale_primer_get_X` `o:scale_primer_get_Y` `o:gray_and_stop_time`
```

四个动作串起来就是「天平通电」的完整时序：

| 动作 | 关键行为 |
| --- | --- |
| `Basic.act` | 检测场上存在 `109019266` → `[TELEPORT] [POS] MAP MAP MAP 120`（**瞬移到自己身上**） |
| `Stay.act` | 玩家靠近 `< 150` → `GO_SHADE`；`[CHECK TIME EX] 2300 [IMMEDIATE]` 后对 `109019266`/`109019280` 下 **`IMPROVED INVINCIBLE`（2300ms）** |
| `Shade.act` | 黑洞演出（`BossmapBlackholeEnd_00.ani` + 屏幕 HSBC）→ `END_TRIGGER ON` → `GO_END`（`SET CUSTOM ACTION 2`） |
| `End.act` | **`INVINCIBLE_OFF`（`REMOVE ALL`）+ `SET VISIBLITY EX [VISIBLE] 1`** |

而天平 `Primer_Proc.act` 每 100ms 检查：

```
[TRIGGER] SPECIAL_ENTRANCE_CHECKER [ENABLE] OFF [CHECK TIME EX] 100
  [WHICH PASSIVE] [CHECKUP] [IS INDEX] 109134975 [/IS INDEX] [IS ETC ACTION] 2
  [CHECKED NO] [>] 0
  → [DO BEHAVIOR NAME] ME VISIBLE
```

**⇒ map614 没有这个装置，天平的 `VISIBLE` 分支永不执行。**
但注意：天平的 `INIT` 行为只做 `HIDE BOSS AURA EFFECT` / `NAME HIDE ON` / `HIDE HP GAUGE`，
**没有隐藏自身** ⇒ 玩家**看得到**天平（只是没名字、没血条），这与实机现象一致。

---

## 3. 天平「可击杀」的充要条件

`END_TRIGGER` 默认 `OFF`，四个条件同时成立才切 `die_trigger_on`：

```
[TRIGGER] END_TRIGGER [ENABLE] OFF
  [COMPARE VAR] c:primer_rarity_progress_now >= c:primer_rarity_progress_max
  [COMPARE VAR] c:oath_rarity_progress_now   >= c:oath_rarity_progress_max
  [COMPARE VAR] c:omen_drop_process_cnt_max  <= c:omen_drop_process_cnt_now
  [CHECK TIME EX] o:die_delay_final
  [CHECKED NO] [>] 0
  → die_trigger_on
[BEHAVIOR] die_trigger_on
  [HP LIMIT ON] 1 PERCENT
  [SET TRIGGER ENABLE NAME] DIE_TRIGGER ON
```

- 开局有 **无条件的** `[TRIGGER] [LIMIT] 1 → HP_LIMIT_ON` ⇒ `[HP LIMIT ON] 99 PERCENT`。
  **这就是「跳伤害数字但打不死」的直接原因。**
- 另有**兜底快通道**，与 omen 无关：

```
[WHICH PLAYER]
[FILTER VAR] getEOOPartyOmenState(t:seatIndex()) [==] 1
[CHECKED NO] [<] 1
→ end_trigger_on_quick   (HP LIMIT 1 PERCENT + END_TRIGGER ON)
```

  **⇒ 只有「场上没有任何玩家处于 omen 状态 1」时才走这条**。实机 `omen : 1, …`
  说明玩家**在**状态 1 ⇒ 不走快通道，只能靠上面四条。

- 数组取值：

```
o:hp_limit_final  = o:arrayAt(`o:hp_limit`,   max(oath_rarity_progress_max, primer_rarity_progress_max) - 40)
o:die_delay_final = 0.9 * o:arrayAt(`o:delay_time`, 同上)
```

  ⇒ 两个 `*_max` 常态是 **40..47**（8 档，下标 0..7）；`== 70 / == 71` 是 rainbow 分支，
  走固定下标 6 / 7（`GO_END_RAINBOW1 / GO_END_RAINBOW2`）。

- 真正死亡（`DIE_TRIGGER`，默认 `OFF`）：

```
[TRIGGER] DIE_TRIGGER [ENABLE] OFF [UPDATE VAR] die_trigger_on
  [ON DAMAGE] [WHICH ME] [COMPARE VAR] o:getHpRate() <= 2
  [NOT] [IS GROUP ACTION] end [CHECKED NO] [>] 0
```

---

## 4. 关键触发器：`omen_drop_process_trigger`（默认 `ON`）

```
[TRIGGER] omen_drop_process_trigger [ENABLE] ON
  [COMPARE VAR] c:primer_rarity_progress_now >= o:omen_drop_process_rarity
  [ON DAMAGE]
  [WHICH MONSTER] [CHECKUP] [IS INDEX] 109019266 109019280 [/IS INDEX] [/CHECKUP]
  [IS ETC ACTION OR] 0 8 9 10 11 12 13 14 [/IS ETC ACTION OR]
  [CHECKED NO] [>] 1
  → start_omen_drop_process
[UPDATE VAR] start_omen_drop_process
  c:omen_drop_process_ing = c:omen_drop_process_cnt_max
  c:omen_drop_process_on  = 1
```

- `o:omen_drop_process_rarity = rr(40, c:primer_rarity_progress_max)` ⇒ 两个 max 初值都是 40
  ⇒ **阈值恰好 40**。
- 另一条出口 `end_omen_drop_process_trigger` 也做 `HP LIMIT 1 PERCENT + END_TRIGGER ON`。
- **⇒ 这是一个闭环**：要启动 omen 掉落进程，需要进度 ≥ 40；而进度何时上涨，见 §9。

---

## 5. `scale_primer.mob` 的 `[create var]` 初值（全部已取出）

| 变量 | 初值 | 说明 |
| --- | --- | --- |
| `o:die_frame_normal / rare` | 2 | 死亡收尾帧 |
| `o:die_frame_unique / legendary` | 4 | |
| `o:die_frame_epic / primitive` | 8 | |
| `o:die_frame_rainbow1 / rainbow2` | 4 / 8 | |
| `c:primer_die` | 0 | |
| `c:die_delay` | 1000 | |
| `c:fake_end_prob_prob` | **30** | **假结束**概率 |
| `c:is_fake_end_leg_epic` | 0 | |
| `o:fake_end_time_stop_length` | 1500 | 假结束时间停止时长 |
| `c:max_grade_now` | **40** | 与实机 `omen : 1, 40, …` 的 40 吻合 |
| `c:primer_rarity_progress_max` | **40** | |
| `c:primer_rarity_progress_now` | **0** | |
| `c:oath_rarity_progress_max` | **40** | |
| `c:oath_rarity_progress_now` | **0** | |
| `c:on_hit` | 0 | 命中计数 |
| `c:is_effect_on` | 1 | |

怪物本体属性：`[move speed] 0 0`、`[weight] 100000`、`[damagebox to height]` 等装置特征。

---

## 6. 服务端侧已确认的缺口

1. **`[maze chance rate]` 从未导入**（`grep Chance` 零命中，`DungeonMaze` 无该字段），
   `Start()` 选 maze 的规则是「同 quest 里 **index 最小者**」
   （`internal/dungeon/session.go` 约 L83–95）
   ⇒ **map614/`_normal` 恒定命中，map615/`_special`（0.7%）永不可达**。
   源注释本身已写「零售端按权重随机」，因此这是一个**可确证的实现缺口**；
   但它**不是**本次「打不死」的原因（两个 maze 的通关条件相同）。
2. **征兆（omen）系统在整个服务端零实现**（`grep -i omen` 零命中）。
   掉落物 id（`109137366` Unique / `109137367` Legendary / `109137368` Epic /
   `109137369` Primeval）由客户端 `make_omen_gem.act` 的 `[CREATE PASSIVEOBJECT]` 创建，
   **但「石头落地/拾取是否需要服务端参与」未判定** —— 这决定缺口在服务端还是纯客户端。
3. **小深渊没有任何掉落配置**：`configs/attunement-rewards.generated.json` 只覆盖
   100005066/67/68；实机日志里 11 只杂兵死亡**零掉落事件**。
4. `RoomCleared()` 把 `109019266` / `109019280` 当活怪（map 选项没有 `[dummy]` ⇒ `NonCombat=false`）
   ⇒ **只要它们不死，服务端侧也拒绝换房**（客户端侧另有一道
   `[arcade scroll check type] "not exist enemy"`）。这一条只在「客户端允许滚动」时才成为瓶颈。

---

## 7. 实机证据（2026-09-26 23:25 会话）

- `client_frame`：**`45`（`ENUM_CMDPACKET_MOVE_MAP`）= 0 次** ⇒ 客户端从未请求换图。
  `39`（`DIE_MONSTER`）= 11 次，entity 全在 `0x1000`–`0x100A`（= trigger=1 的 11 只杂兵），
  **没有任何超出投放范围的 entity**。
- `monster_death_ack / _confirmed / monster_experience_updated` 各 11，与 11 只杂兵一一对应。
- **零掉落事件**（无 loot / item / scene 行）。
- `opcode 2329 = ENUM_CMDPACKET_MONSTER_HISTORY_LOG` 的载荷是**纯 ASCII 文本快照**：

  ```
  01000000 HP : 94730.00, Action : 20014, triggers : 1, 0,
  rarities : 71, 72, 44, 72, omen : 1, 40, 0, 0, time_from_max : 0
  ```

  ⇒ 客户端确实在跑天平逻辑；`omen` 已到 40（= `c:max_grade_now`），
  对应玩家截图「您的波次登记数已超过限制」。
- `opcode 2127 = ENUM_CMDPACKET_PROCESS_SCAN` 是**反外挂进程扫描**
  （载荷是 `C:\Program Files (x86)\ASUS\…` 之类路径，272/400 字节），与本问题无关。
- `opcode 38 = USE_SKILL`、`283 = PI`、`2377 = SET_UNIFIED_OPTION`、`2423 = UPDATE_NEW_COMBAT_VALUE`
  —— 均与本问题无关。

---

## 7b. 第二场实机（2026-09-26 23:58）与第一场逐字相同

- 会话 `roles_persist_..._20260926_235817_459041_next37`，跑的 exe 已含「未知 entity 死亡确认」修复（`4B9628C7…`）。
- `monster_death_ack / _confirmed / monster_experience_updated` 各 11（0x1000–`0x100A`），
  **零掉落事件**，无超范围 entity，无 `MOVE_MAP`。
- `MONSTER_HISTORY_LOG` 共 20 条：**8 条文本完全相同**
  （`HP : 94730.00, Action : 20014, triggers : 1, 0, rarities : 71, 72, 44, 72, omen : 1, 40, 0, 0`），
  之后 **12 条空包**。首条 `HP : 4736540.00` ⇒ **HP 恰好被压到 2%**
  （4736540 × 0.02 = 94730.8）。
- **`Action : 20014` 两场恒定不变** ⇒ 动作组从未切换 ⇒ §4b 的状态机**一次都没跑**。
  玩家观察到的「特效/外观变化」是受击反馈，不是阶段推进。
- **修正 §3 的表述**：不是「HP 被锁在 99%」—— HP 能到 2%，伤害完全有效；
  「不死」的直接原因是 `DIE_TRIGGER` 默认 `[ENABLE] OFF`，需 `END_TRIGGER` 四条件先成立。
- 天平无血条是设计：`INIT` 行为里 `HIDE HP GAUGE 1` + `NAME HIDE ON`。
- 「一个对象还是两个」：**两个**。服务端投放 13 行，索引 11 = `109019266`（`[fixed] [boss]`）、
  索引 12 = `109019280`（`[fixed] [normal]`），**坐标同为 (3168,191)** ⇒ 碰撞箱重叠，
  这就是「左右两侧打都有伤害数字」的原因。而 §4b 的推进状态机检查的是 `109019280`。

## 7c. 诊断开关（本轮新增，默认关）

`internal/dungeon/session.go`：

```
DFO_SKIP_TRIGGERED_SPAWNS=1   →   dropTriggeredMonsters() 不投 [monster create trigger] > 1 的行
```

按 `SourceIndex`（怪物行序号）与 `[monster create trigger]` 段对齐。
用途：**一次实机就能判定那两只到底该不该由服务端投**。
默认不设 = 行为与原先完全一致（`go vet` + `internal/dungeon` 测试全通过）。
启动链已接线：`scripts/launch_local.py` 里 `env["DFO_SKIP_TRIGGERED_SPAWNS"] = "1"`（**标注「验完即删」**）。

**判据**：重启后打一场 ——

- 天平**消失** ⇒ 这两只是客户端机制按需创建的，服务端**不该常驻投** ⇒ 根因在投怪。
- 天平**还在** ⇒ 客户端自己会创建，服务端投怪无关 ⇒ 问题纯在客户端。

---

## 7d. 实现：把 `[monster create trigger]` 真值写进进图包（本轮新增，开关控制）

`protocol.StartMap` 的怪物记录是 18 字节：

```
add32(add16(p, 0), m.SourceIndex)      // 0(u16) + SourceIndex(u16)
add32(add16(p, m.Entity), m.Template)  // Entity(u16) + Template(u16)
append(p, m.Level, m.Rank, 0, 0, 255)  // ★ 这两个 0 至今没有逆向解释
add32(p, m.Team)
append(p, 0)
```

而 `[monster create trigger]` 与怪物行**严格并行**（小深渊 11 只 = 1、天平与 `109019280` = 2；
大深渊 `1/2/3` 三档）。所以「那两个 0 里是否有一个是创建时机」是一个可直接检验的假设 ——
不猜语义，**把真值填进去看行为**。

改动（**开关默认关，输出与原先逐字节一致**）：

| 位置 | 改动 |
| --- | --- |
| `protocol.DungeonMonster` | 新增 `CreateTrigger byte` |
| `protocol.StartMapState` | 新增 `EncodeCreateTrigger bool` |
| `protocol.StartMap` | 置位时把 `Rank` 后那一格写成 `m.CreateTrigger`（原本恒 0） |
| `internal/dungeon.fixedMonsters` | 按 `SourceIndex` 从 `[monster create trigger]` 段取值填入；越界或 >255 返 0（**数据层无副作用**） |
| `cmd/wireprobe/{dungeon_flow,tutorial_flow}.go` | 三处 `StartMapState` 构造点带上开关（读 `DFO_MONSTER_CREATE_TRIGGER=1`） |

验证：

- 新测试 `TestCreateTriggerOrdinalsFollowMonsterRows`（PASS 4.89s）：13 行严格对齐、索引 11/12 的模板正确、两张图一致。
- `go vet` 无输出；`internal/game/protocol` 与 `internal/dungeon` 全量测试通过
  （含字节级 `StartMap` 断言 ⇒ 默认行为未变）。
- 启动链已接：`scripts/launch_local.py` 里 `env["DFO_MONSTER_CREATE_TRIGGER"] = "1"`（标注「验完即删」）。

**判据**：重启打一场 ——

- 天平**不再与杂兵同时登场**（清完 11 只后才出现）⇒ 这一格确实是创建时机，且客户端会自行分阶段；
- **行为无任何变化** ⇒ 那两个 0 是别的字段，需回到 IDA 定位。

---

## 7e. 进图包字段实验证否，以及 IDA 首轮结果（2026-09-27 凌晨）

### 7e.1 `[monster create trigger]` 写进进图包 —— **已证否**

开关生效后实机：**天平出现了，但行为与之前一致**（`MONSTER_HISTORY_LOG` 仍为
`Action : 20014` 恒定，不推进）。⇒ 那两个 0 **不是**创建时机。
开关已从 `scripts/launch_local.py` 回退；代码保留、默认关。

### 7e.2 动作链完整闭合（`primer_00_normal_loop.act`）

```
INIT            [LIMIT] 1 [FRAME] 0
   [HP LIMIT ON] o:arrayAt(`o:hp_limit`, c:max_grade_now-40) PERCENT
   [SET TRIGGER ENABLE NAME] INIT_CHECKER ON
INIT_CHECKER    (默认 OFF) → ME TIME_CHECKER_ON
TIME_CHECKER_ON → 关 INIT_CHECKER、开 TIME_CHECKER
TIME_CHECKER    (默认 OFF)
   [CHECK TIME EX] o:arrayAt(`o:delay_time`, c:max_grade_now-40) → ME RARITY_CHECK_ON
RARITY_CHECK_ON → 关 TIME_CHECKER、**[SET TRIGGER ENABLE NAME] RARITY_CHECK ON**
RARITY_CHECK    ([ON DAMAGE], 默认 OFF) → §4b 的分支树
```

数组原值（`scale_primer.mob`）：

```
o:delay_time = [100, 900, 1100, 1300, 1800, 2300, 2300, 2300]
o:hp_limit   = [ 90,  80,   70,   60,   50,   40,   70,   50]
c:max_grade_now = 40（初值） ⇒ 下标 0 ⇒ HP LIMIT 90% / 计时 100
```

`[etc action definition]` 顺序是**先 START 后 LOOP**：0=`00_Normal_LOOP`、1-7=`01..07_*_START`、
8-14=`01..07_*_LOOP`、15-22=END。与 `[IS ETC ACTION OR] 0 8 9 10 11 12 13 14`（全部 LOOP 档）完美吻合。

### 7e.3 卡点（本轮结论）

升档分支要求 **`c:primer_rarity_progress_now == 40`**（或 `== primer_max`），而它**初值是 0**，
且在 43 个天平脚本里**只有读、没有写**（批量 `-filter rarity_progress` 零命中写入点）。
虽然 `RARITY_CHECK` 开关链会在开场后很快走完（计时 100），**但 0 ≠ 40 ⇒ 什么都不做**。
同理 `c:oath_rarity_progress_now`。升档动作本身是 `[UPDATE VAR] <名字>` 命名赋值块：
`primer_up_to_rare`→now=41、`oath_up_to_unique/legendary/epic/primitive`→42/43/44/45、`summon_orthaire`→o:summon_orthaire=1。

⇒ **必须有东西把 `primer_rarity_progress_now` 从 0 设成 40**，而它不在天平脚本里。

### 7e.4 IDA 首轮：headless 管线跑通，但关键字分派不走明文

命令（每次约 1–3 分钟；**先 `cp` 副本，绝不在原库跑**）：

```bash
cp /d/115us-backup/DFO.exe.i64.orig-20260923 /d/115us-backup/ida-work/updvar.i64
MSYS_NO_PATHCONV=1 /d/tools/ida94/idat.exe -A -L"<log>" -S"<script>.py" "D:/115us-backup/ida-work/updvar.i64"
```

- `[UPDATE VAR]`（取自 `analysis/dumps/xorstr_map.tsv`）在 **`0x14985D9D8`**，只有 **4 处 xref**，
  分属 `0x141e726f0` / `0x1459c9de0` / `0x145a14770` / `0x147cda6f0`。
- 四处**全是诊断消息**：`lea rcx, unk_14985D9D8` → `sub_146E8C7D0`（xorstr 解码）→
  构造 wstring → 注册到一个“断言/错误文本”表（handler `sub_141EB36F0` 里直接调
  `__crt_win32_buffer_debug_info::file_name` 打日志）。
- ⇒ **关键字分派不走明文字符串**。同样可推出 `pvfinspect` 报的
  `type=3 value=16331` **不能当“字符串池偏移”**，`16331`（0x3FCB）很可能就是脚本 VM 的操作码/ID。

**下一步**：用字节模式搜 `CB 3F 00 00`（小端 0x3FCB）并用反汇编复核假命中，
或反向：统计 `sub_146E8C7D0`（xorstr 解码器）的调用点，**调用最密集的函数即脚本解析器**。

---

## 8. 取证方法与工具（可复用）

```bash
# 地图/副本脚本（文本渲染即可）
go run ./runtime/mapprobe  -map 100016614
go run ./runtime/dgscript  -id 100005014

# cell 级读取（关键；新增 runtime/actcells，被 gitignore）
go run ./runtime/actcells -path "contents/2026/endkeeperoforder/monster/named/scale_primer/action/Primer_Proc.act" \
     -around "die_trigger_on" -span 55
go run ./runtime/actcells -path "<a.act>,<b.obj>" -filter rarity_progress
go run ./runtime/actcells -path "<x.act>" -from 196 -to 300

# PVF 内检索（离线跑；每次写 208 MB matches.json，用完立刻删）
./bin/pvfinspect.exe -source ../client-build/Script.inner.pvf -find "endkeeperoforder" -output /tmp/pvf
```

**两条必须记住的坑**：

1. **`.act` / `.obj` / `.mob` 的 `.txt` 渲染会丢 `type=10` cell**。
   这些 cell 装的是条件里的**变量名与操作数**，丢了就会看到
   `[COMPARE VAR] [>=]` 这种「空条件」，很容易误判成「编码未确立」。
   `Archive.Tokens()` 只解 type 3/6/8；补 type 10 用公开的 `Archive.ResolveString()`，
   **不要改 `Tokens()`**（`internal/catalog/droptable.go` 与 `cmd/gmtool/index.go` 在读 `Reference`）。
2. `pvf.Open()` 有 **512 MiB 上限**（`DefaultMaxBytes`），内层 PVF 760 MB ⇒
   用 `pvf.Load(pvf.Options{Path: …, MaxBytes: 2<<30})` + `pvf.OpenArchive(bundle)`。

---

## 4b. 完整推进链：一套纯客户端的「伤害驱动状态机」（本轮新挖出）

`primer_00_normal_loop.act` … `primer_07_rainbow2_loop.act` 八档各有一份，结构相同。
`primer_00_normal_loop.act` 的 `[ON DAMAGE]`（cell 120-290，默认 `[ENABLE] OFF`）是本机制的主干：

```
[TRIGGER] [ENABLE] OFF [ON DAMAGE]
  [BEGIN IF]
    [COMPARE VAR] c:omen_drop_process_cnt_max > 0
    [COMPARE VAR] o:omen_drop_process_rarity == c:primer_rarity_progress_now   ← 需要「相等」
    [COMPARE VAR] c:omen_drop_process_cnt_max <= c:omen_drop_process_cnt_now
  [ELSE IF] ... [END IF]
  [WHICH MONSTER] [CHECKUP] [IS INDEX] 109019280 [CHECKED NO] [>] 0     ← ★ 打的是 109019280
  [BEGIN IF]
    primer_rarity_progress_max > 69                 → primer_up_to_rainbow1 / GO_RAINBOW1
    [ELSE IF]
      primer_now == primer_max && oath_now < oath_max  → only_oath_up
          oath_now == 40 && is_oath_normal_loop     → OATH_UP_TO_UNIQUE
          oath_now == 42 && is_oath_unique_loop     → OATH_UP_TO_LEGENDARY
          oath_now == 43 && is_oath_legendary_loop  → OATH_UP_TO_EPIC
          oath_now == 44 && is_oath_epic_loop
              c:nox_die == 0 → summon_orthaire              ← 真 BOSS 登场
              c:nox_die == 1 → [DELAY DO BEHAVIOR 2000] OATH_UP_TO_PRIMITIVE
    [ELSE IF]
      primer_now < primer_max                         → only_primer_up
          primer_now == 40 → PRIMER_UP_TO_RARE
  [END IF]
```

要点：

1. **推进进度靠「对 `109019280` 造成伤害」**，不是打天平本体 `109019266`。
   `109019280` 与天平**同坐标 (3168,191)**、选项 `[fixed] [normal]`，在 `[monster]` 段是独立一行（索引 12）。
2. 升级动作全是 `[SET GROUP ACTION]`（把对象切到下一个动作组），因此**必须靠实机观察外观/特效变化**。
3. `oath_rarity_progress` 的档位是 **40 → 42 → 43 → 44 → primitive**，
   每档由 `c:is_oath_*_loop` 标志配上 `RARITY_CHECK_OFF` 收尾。
4. **真 BOSS 由 `c:nox_die == 0` 时的 `summon_orthaire` 召唤**；`nox_die == 1` 走 primitive 分支。
5. `c:omen_drop_process_cnt_max` 在 `omen_drop_maker` / `omen_drop_maker_1p` 的**三个 action 里零写入**，
   只在 `omen_drop_maker_1p.obj` 的 `[create var]` 里声明（初值 0），并由
   `end_omen_drop_process_trigger` 归零 ⇒ **它的「非零值」来自引擎侧**（与 `RARITY_CHECK_*` 同层）。
6. `omen_drop_maker/action/Basic.act` 是石头生成的门槛：
   第 0 帧要求 `getEOOPartyOmenState(o:targetindex) == 1`（否则 `destroy` 自毁），
   第 1 帧要求 `c:omen_drop_process_on == 1`（否则不切到 `make_omen_gem`）。

**⇒ 结论：整条链（`RARITY_CHECK_START` 登记 → `[ON DAMAGE]` 状态机 → `PRIMER_UP_*` /
`OATH_UP_*` → `summon_orthaire`）全部在客户端本地，PVF 文本里没有服务端参与点。**

---

## 9. 未决项

1. **`c:primer_rarity_progress_now` / `c:oath_rarity_progress_now` 由谁累加** ——
   它们通过 `[UPDATE VAR] RARITY_CHECK_START c:primer_…now c:oath_…now c:…max c:…max`
   登记给客户端引擎（C++ 侧），**在 `omen_drop_*` 系列 8 个文件里 `rarity_progress` 零命中**
   ⇒ PVF 文本取证到此为止，需要别的入口（引擎行为观测 / IDA / 实机对照）。
2. **征兆石（`109137366`–`109137369`）的落地与拾取是否需要服务端参与** ——
   决定 omen 缺口的分界线。
3. `[monster create trigger]` 的真实语义（已否掉两个假设，见 §1b 注）。
4. `[arcade scroll check type] "not exist enemy"` 是否把 `[fixture]` 装置计入 enemy ——
   决定「服务端不投这两只怪」是否必要。
5. **待业主（玩家）回答**：打天平的过程中，**地上有没有出现过发光的石头/结晶，能不能捡？**
   —— 这一问直接区分「石头没生成」还是「石头生成了但没落地/没被捡」。

---

*本文所有条件、变量与数值均来自内层 PVF 的 cell 级读取，未使用任何 IDA 结论，也未引用第三方文档。*


---

## 8b. 「打不死」的确定性根因（2026-09-27 取证，cell 级）

### 8.1 客户端自带的状态日志 = 权威观测口

`primer_proc.act` 末尾（cell 2070-2091）就是这条日志的定义，**格式串与参数表逐字可读**：

```
[TEXT]    HP : %f, Action : %d, triggers : %d, %d,
          rarities : %d, %d, %d, %d, omen : %d, %d, %d, %d, time_from_max : %d
[ARG VAR] o:getHP()
          o:getCurrentActionIndex()
          o:is_end_trigger_on           o:is_die_trigger_on
          c:primer_rarity_progress_now  c:primer_rarity_progress_max
          c:oath_rarity_progress_now    c:oath_rarity_progress_max
          c:omen_drop_process_on        o:omen_drop_process_rarity
          c:omen_drop_process_cnt_max   c:omen_drop_process_cnt_now
          o:safe_timer_from_max
```

实机（2329 = MONSTER_HISTORY_LOG）抓到的那条逐字段落位：

| 字段 | 实测 | 对应变量 | 判读 |
|---|---|---|---|
| HP | 4736540 -> 94730 | `getHP()` | 血被打到约 2% 后冻住 |
| Action | 20014 | `getCurrentActionIndex()` | 全程恒定 ⇒ **一次都没换过动作组** |
| triggers[0] | 1 | `is_end_trigger_on` | END 已开 |
| triggers[1] | **0** | **`is_die_trigger_on`** | **DIE 从未开**（阻塞点） |
| rarities[0] | 71 | `primer_rarity_progress_now` | 引子：rainbow2 档 |
| rarities[1] | **72** | `primer_rarity_progress_max` | **天花板 = 72** |
| rarities[2] | 44 | `oath_rarity_progress_now` | 誓约：epic 档 |
| rarities[3] | **72** | `oath_rarity_progress_max` | **天花板 = 72** |
| omen[0] | 1 | `omen_drop_process_on` | 征兆进程已启动 |
| omen[1] | 40 | `omen_drop_process_rarity` | 本轮目标等级 = 40 |
| omen[2] | 0 | `omen_drop_process_cnt_max` | 无玩家持有征兆 |
| omen[3] | 0 | `omen_drop_process_cnt_now` | — |

### 8.2 唯一的死亡路径与它的四道门

`primer_proc.act` cell 415-470：

```
[TRIGGER] END_TRIGGER [ENABLE] OFF
    [UPDATE VAR] end_trigger_on            -> o:is_end_trigger_on = 1
    [COMPARE VAR] c:primer_rarity_progress_now >= c:primer_rarity_progress_max
    [COMPARE VAR] c:oath_rarity_progress_now   >= c:oath_rarity_progress_max
    [COMPARE VAR] c:omen_drop_process_cnt_max  <= c:omen_drop_process_cnt_now
    [CHECK TIME EX] o:die_delay_final
    [CHECKED NO] > 0
      -> [DO BEHAVIOR NAME] ME die_trigger_on
[BEHAVIOR] die_trigger_on  ->  [HP LIMIT ON] 1 PERCENT + DIE_TRIGGER ON
[TRIGGER]  DIE_TRIGGER [ENABLE] OFF
    [ON DAMAGE] [WHICH ME] [COMPARE VAR] o:getHpRate() <= 2   -> 死
```

代入实测值：

| 条件 | 实测 | 结果 |
|---|---|---|
| `primer_now >= primer_max` | 71 >= 72 | 不成立 |
| `oath_now >= oath_max` | 44 >= 72 | 不成立 |
| `cnt_max <= cnt_now` | 0 <= 0 | 成立 |
| `[CHECK TIME EX] die_delay_final` | — | — |

**⇒ 前两条恒不成立 ⇒ `die_trigger_on` 永不执行 ⇒ DIE_TRIGGER 永远 OFF ⇒
血停在 2% 也死不了。** 这正是「能跳伤害、会抖动、颜色会变、但永远打不死」。

> 抖动来源：`oath_proc.act` 的 `hit_check_trigger` —— 引擎在命中时写 `c:is_hit = 1`，
> 脚本就 `[SET OBJECT SHAKING] 150ms`。**说明引擎确实会写这些 `c:` 变量**，
> 也说明伤害事件是通到脚本 VM 的。

### 8.3 天花板 72 从哪来：引擎 getter，且越界

`primer_proc.act` cell 0-9：

```
[UPDATE VAR] RARITY_CHECK_START
    c:primer_rarity_progress_now = 40
    c:oath_rarity_progress_now   = 40
    c:primer_rarity_progress_max = getPrimerGrade()
    c:oath_rarity_progress_max   = getOathGrade()
```

（`.mob` / `.dgn` 里的 `*_now = 0` 会被这一步覆盖成 40，所以那两个 0 不是问题。）

`getPrimerGrade` / `getOathGrade` 是**引擎脚本函数**。用解密字符串表拿到名字地址，
在 IDA 里两者都落在**同一个注册函数 `sub_147685170`**：

| 注册 ID | 名字 |
|---|---|
| `0x852` = 2130 | `getPrimerGrade` |
| `0x853` = 2131 | `getOathGrade` |
| `0x854` = 2132 | `getEOOPartyOmenState` |

同族邻居（`0x14B2CAF38` 起，连续排布）：

```
getDistancePosX · getPrimerGrade · getOathGrade · getDungeonFreeEntryClearCount
getEOOOmenGrade · getEOOOmenIndex · getEOOPartyOmenGrade · getEOOPartyOmenState · isEOOPartyOmenUse
```

⇒ 这一族读的是**「末日守护者征兆（EOO）」的副本/队伍等级数据**，
紧挨着 `getDungeonFreeEntryClearCount`（副本通关进度）。

**而 72 这个值在脚本数据里一次都没出现**：对 28 个含该变量的文件全量 grep ——
可赋的值只有 `41/42/43/44/45/70`；比较值最高 `71`；`str=72` 命中 **0**。
脚本自身能到的最高档：`primer_now = 70/71`，`oath_now = 45`。

**⇒ 天花板 72 落在整套机制值域之外 ⇒ 该副本在当前客户端上不可能通关。**

### 8.4 顺带钉死的两个子机制

- `omen_drop_user_0..4`（cell 297-320）：按被动对象 `109134988 / 109137638 / 109137641 / 109137642`
  的命中数量，把 `c:omen_drop_process_cnt_max` 设成 **0~4** —— 即「本队有几个玩家持有征兆」。
- `omen_drop_process_trigger`（cell 252-286，ENABLE ON）：
  `primer_now >= o:omen_drop_process_rarity` + 对 109019266/109019280 造成伤害 +
  `[IS ETC ACTION OR] 0 8 9 10 11 12 13 14` + 计数 > 1
  ⇒ `start_omen_drop_process`（`ing = cnt_max`、`on = 1`）+ `end_omen_drop_process_trigger`
  （`[HP LIMIT ON] 1 PERCENT` + `END_TRIGGER ON`）。
  **⇒ 这解释了实测里 END 已开、血被压在 2%。**
- 另有兜底路径 `end_trigger_on_quick`：当场上没有任何玩家处于 omen state 1 时，同样开 END。

### 8.5 服务端现状与待办

- Go 侧对 `100005014` **只有注释与测试引用**，没有任何「征兆等级 / 通关次数」字段；
  `freeentry` / `clearcount` 零命中。
- 该机制的判定（进度、征兆石生成、颜色、动作组、死亡）**全部在客户端脚本 + 引擎**，
  服务端不参与。
- **待确认（决定下一步）**：`getPrimerGrade()` / `getOathGrade()` 读的是哪一个数据源
  —— 角色/队伍数据、副本进度，还是某个服务器包。
  定位方法：搜索立即数 `0x852` / `0x853` 的反汇编分支（VM 按 ID 分派），
  或反向查该注册函数的调用方。

---

## 9b. 天花板值的数据源：`getPrimerGrade()` / `getOathGrade()`（2026-09-27 取证）

### 9.1 定位路径（可复用）

1. 解密字符串表里这两个名字各出现一次：
   `0x14B2CAF60 getPrimerGrade`、`0x14B2CAF88 getOathGrade`。
2. IDA xref → 两者都落在**同一个注册函数 `sub_147685170`**，每个 case 的形态是
   `mov dword ptr [rbp+…+arg_8], <ID>` + `lea rcx, <名字地址>` + xorstr 解码 + 注册调用
   ⇒ **`getPrimerGrade` = 2130 (0x852)，`getOathGrade` = 2131 (0x853)**。
   同族：2127 `getDistancePosX`、2135 `getEOOOmenGrade`、`getEOOOmenIndex`、
   `getEOOPartyOmenGrade`、`getEOOPartyOmenState`、`isEOOPartyOmenUse`、`getDungeonFreeEntryClearCount`
   —— 是同一批 2026 内容脚本函数。
3. 这些名字集中在 `0x14B2CAF38`–`0x14B2CB0B8`；**同一个数据块里紧接着就是 `__FILE__` 风格的源路径**，
   于是拿到 `0x14B2D0800  D:\Work\Jenkins5\src\DNFShared\GameScript\PrimerCollectionScript.cpp`。
   与它同块的段名把整个类暴露了出来：

```
[max piece] [item exchange] [exchange rate] [item crafting] [required piece]
[primer disjoint] [common primer disjoint] [piece index] [partset index]
[item crafting index] [season id] [require minimum level] [oath cost key]
[30lv special reward] [penalty rule set] [default exp ratio] [exp ratio]
[dungeon categories] [dungeon category] · higher dungeon / normal dungeon
```

4. 数据文件 = **`etc/115lvability2/primercollection.cos`**（**UTF-16LE 纯文本**，
   `pvfinspect` 的文本渲染直接可读）：
   `[max piece] 9999`、`[exchange rate] 30 9`、`[rarity] epic → [required piece] 1000`、
   `[rarity] primeval → 1500`、**`[partset index] 16201 … 16212`（12 个引子）**。

### 9.2 同族还有一整套「赛季等级」

```
Contents/System/SeasonLevel/main.cos      SeasonLevelSystem::init
D:\…\DNFClient\RDAR\SeasonLevelSystem.cpp   Etc/115LvAbility/SeasonLevel.lst（1…102 级）
seasonLevelText_primer                    season_level_limit_cover
```

另有脚本条件名 `[equipment oathgrade]` / `[equipment setgrade]`（在 `0x14A7B9180` / `0x14A7B91B0`
那一片条件名里）⇒ **誓约装备自带等级**。

### 9.3 现状与结论

- **服务端零处理**：Go 侧 grep `season` / `115lvability` / `primercollection` / `oathgrade`
  全部零命中（只有 Elvenmere 的 `SeasonRewards`，是另一套）。
- ⇒ `getPrimerGrade()` / `getOathGrade()` 读的是**玩家侧的引子/誓约等级**，
  而这条链上唯一由服务端喂的正是**玩家数据（装备等级 / 赛季等级）**。
- 实测天花板 = **72**，而机制值域是 `40..45 / 70..71`（脚本可赋 41-45/70，比较值最高 71）。
  注：本仓库的深渊奖励池会给出 `Oath_*` 装备且 **`grade=116`**（比内容的 115 上限高一档），
  与「有效输入顶到 71、116 映射到 72」的形状吻合（**待证**）。

### 9.4 两个待办（都便宜）

1. **一次实机实验即可判定**：换 / 卸掉角色的誓约（或引子）装备后再打一场，
   看日志里 `rarities` 的第 2、4 个数（两个天花板）是否跟着变。
   变了 ⇒ 证明是**装备等级驱动**，完全落在服务端可控范围。
2. 或者按立即数 `0x852` 扫 IDA，直接看 opcode 2130 的分派实现读的是哪个字段。

### 9.5 复现命令

```
# 1) 名字地址
grep -an "getPrimerGrade\|getOathGrade" analysis/dumps/xorstr_map.tsv
# 2) 注册点 + ID（headless IDA）
idat -A -S runtime/ida_grade.py <idb>
# 3) 引子收集数据（UTF-16LE 文本）
./bin/pvfinspect.exe -source …/Script.inner.pvf \
    -files "etc/115lvability2/primercollection.cos" -output <dir>
#    ⚠️ 会在 output 目录写 208 MB matches.json，用完立刻删
```


---

## 10. 誓约/引子等级的来源（2026-09-27 取证）

### 10.1 「赛季等级 -> 誓约装备稀有度」是硬映射

`etc/115lvability2/oathsystemscript.cos`（UTF-16LE 文本）：

```
[base rarity section]
 `rare`      ... EquipmentOath/BaseStat/rare_stat.etc
 `unique`    ... unique_stat.etc
 `legendary` ... legendary_stat.etc
 `epic`      ... epic_stat.etc
 `primeval`  ... primeval_stat.etc

[seasonlevel oath item]
  1   100610094  rare_stat.etc
  30  100313750  unique_stat.etc
  60  100610095  legendary_stat.etc
  80  100313751  epic_stat.etc
  100 100610096  primeval_stat.etc
```

**⇒ 赛季等级 1 / 30 / 60 / 80 / 100 -> rare / unique / legendary / epic / primeval**，
而 `Etc/115LvAbility/SeasonLevel.lst` 的等级表是 **1…102**。

### 10.2 机制档位码 = 稀有度码

机制里的 `40/41/42/43/44/45` 与 `[base rarity section]` 的
common / rare / unique / legendary / epic / primeval **逐位对位**；
`70/71` 是更高一族的 rainbow1 / rainbow2。
⇒ `getPrimerGrade()` / `getOathGrade()` 返回的是**誓约 / 引子装备的稀有度档位码**。

### 10.3 结论（推断，待实机确认）

- 誓约装备的稀有度由**赛季等级**决定，赛季等级是**玩家进度**。
- 服务端对这套（`oath` / `primer` / `season` / `115LvAbility`）**零实现**。
- **天花板 72 越界**与**誓约栏位锁定**很可能是**同一个原因**：这套系统在服务端未激活。
- ⇒ 「卸掉誓约装备」**未必**能改变天花板（若天花板由系统状态而非那一件装备决定），
  但两种结果都有信息量：变了 = 装备驱动；没变 = 系统状态驱动（即必须补服务端能力）。

### 10.4 存档结构（卸装实验用）

- 角色态 = PG `characters.state`（JSON），穿戴在 `inventory.worn: [{slot,template,...}]`。
- 服务端栏位模型：`EquipmentBodySlot` 接受 0..47，**oath = 47**，引子相关移动用 37/45
  （`internal/inventory/equipment_family.go`）。
- 每件带 `Record []byte`（实例数据 = 强化/属性等）。
- `cmd/gmtool` 只有 `/api/grant`（发放），**没有卸装端点**。

### 10.5 服务端可动的三个方向（待定优先级）

1. 给角色补一个**合法的赛季等级 / 誓约状态**（哪怕是最小值），看天花板是否落回 40..71。
2. 查我们发放的誓约装备稀有度是否越界（本仓库深渊奖励给过 `Oath_*` 且 `grade=116`）。
3. 若确认是系统未实现，则这是「补 115 级能力/赛季系统」的功能项，而不是一个 bug fix。


### 8.6 那条状态日志的唯一触发条件（2026-09-27 补）

`primer_proc.act` cell 2025-2068：

```
[TRIGGER]                    (默认 ON，未命名)
   [CHECK TIME] 1000
   [ON DAMAGE]                                        <- 天平必须正在挨打
   [COMPARE VAR] o:safe_timer >= 60                   <- 出生后满 60 秒
   [WHICH ME] [NOT] [IS GROUP ACTION] end             <- 且不在 end 动作组
   [CHECKED NO] > 0
   [BEGIN IF]
      primer_max == 70 -> log + GO_END_RAINBOW1
      primer_max == 71 -> log + GO_END_RAINBOW2
      ELSE             -> log + GO_END
   [END IF]
```

`o:safe_timer` 由 `safe_destroy_timer` 每秒 +1（cell 1989-2007，`[TRIGGER] [CHECK TIME] 1000`），
即**天平出生后的秒数**。

**⇒ 全副本只有这一个 `[SERVER LOG MSG]`**（`-filter` 全量确认），
所以「读那两个天花板」**没有更早的口子**：必须让天平活着被连续输出 ≥ 60 秒。

⚠️ 2026-09-27 01:16 那一场的实机记录：进图 01:16:10 → 事件结束 01:16:34，
**全程只有 24 秒**，`2329`（MONSTER_HISTORY_LOG）**零条** ⇒ 没到 60 秒，读不到。

### 8.7 清完杂兵会召唤真正的隐藏 BOSS（2026-09-27 补）

`primer_proc.act` cell 741-786：

```
[TRIGGER] [LIMIT] 1
   [WHICH MONSTER] [CHECKUP] [IS INDEX] 109019402 109019403 109019404 [/IS INDEX] [/CHECKUP]
   [CHECKED NO] > 0
   -> [DO BEHAVIOR NAME] ME summon_nox
   -> [DO BEHAVIOR NAME] CHECKUP OBJECT destroy
[BEHAVIOR] destroy     -> [DESTROY]
[BEHAVIOR] summon_nox
   -> [SET TRIGGER ENABLE NAME] gm_trigger OFF
   -> [SUMMON MONSTER] [INDEX] 109019264 [LEVEL] -1 [POS] MAP 520 MAP 200 MAP 0 [NO EFFECT]
```

**⇒ 11 只杂兵（`109019402/403/404`）全部死亡时，天平的 `gm_trigger` 关闭，
并召唤 `109019264` = `OrderChroniclerOrthaire`（秩序记录者·奥尔泰尔，Lv135）
到坐标 (520, 200)** —— 就是「必出太初的隐藏 BOSS」。


### 8.8 判别实验：脱掉誓约后天花板**逐字不变**（2026-09-27 01:21 实机）

| | HP | Action | triggers | rarities | omen |
|---|---|---|---|---|---|
| 穿誓约（基线） | 94730.00 | 20014 | 1, 0 | **71, 72, 44, 72** | 1, 40, 0, 0 |
| **脱誓约**（本场） | 94730.00 | 20014 | 1, 0 | **71, 72, 44, 72** | 1, 40, 0, 0 |

**⇒ 天花板 72 与那件誓约无关。** 与 `装备评分` 是两回事的说法一起看，
72 更可能是「共享来源 / 内容常量」（赛季等级或默认值），而不是某一件装备。

（本场进图 17:20:49，首个 `2329` 出现在 17:21:56 —— 距出生 **67 秒**，满足 `o:safe_timer >= 60`，
所以这次的数据是有效的。）

### 8.9 第二个独立的阻塞：脚本召唤的 BOSS 退化成「可拾取物品」

`primer_proc.act` cell 741-786 的 `summon_nox`（11 只杂兵全灭时触发）
会 `[SUMMON MONSTER] [INDEX] 109019264`（= `OrderChroniclerOrthaire`）。实机表现：

- 杂兵开始死亡时（17:20:52.7~53.9），死亡上报载荷里**同时带了两个新实体 `0x100D` / `0x100E`**；
- 紧接着 17:20:54.495 / 17:20:55.549 客户端发出 **`pickup`（opcode 43）**，
  `pickup_scene_removed` 的载荷正是 `0d100000…` / `0e100000…`
  ⇒ **这两个实体被当成地面物品捡走了**；
- 用户确认「秩序记录者·奥尔泰尔似乎没有真的召唤」。

而 `c:nox_die` 只在 **BOSS 自己**的动作里被写：

```
contents/2026/endkeeperoforder/monster/boss/orderchroniclerorthaire/action/base/last.act   → c:nox_die
contents/2026/endkeeperoforder/monster/boss/orderchroniclerorthaire/action/last/last.act   → c:nox_die
contents/2026/endkeeperoforder/monster/boss/orderwatcher/action/base/last.act              → c:nox_die
contents/2026/endkeeperoforder/monster/boss/orderwatcher/action/last/last.act              → c:nox_die
```

而 `oath` 从 `epic(44)` 升到 `primitive(45)` 需要 `c:nox_die == 1`
（`primer_*_loop.act` 的 RARITY_CHECK：`if oath_now == 44 and is_oath_epic_loop` →
`nox_die == 0` 时 `summon_orthaire`；`nox_die == 1` 时才 `OATH_UP_TO_PRIMITIVE`）。

**⇒ 因为奥尔泰尔没有成为真怪，`nox_die` 永远是 0 ⇒ `oath_now` 被卡在 `44`（实测值正是 44）。**

**⇒ 观察到的规律**（与 2026-09-26 那次「不投 trigger>1」实验一致）：
**客户端本地创建、而服务端没有对应怪物记录的对象，会退化成可拾取物品被玩家收走。**

### 8.10 当前的两条阻塞（并列，缺一不可）

1. **召唤退化成物品**：脚本 `[SUMMON MONSTER]` 出来的 BOSS 没能成为真怪
   ⇒ `nox_die` 恒 0 ⇒ `oath_now` 卡在 44。**这一条看起来是服务端可以补的**
   （在 11 只杂兵死亡时把 `109019264` 作为真怪投进场景）。
2. **天花板越界**：`*_max` 恒 72，而机制值域是 40..45 / 70..71。
   已知与誓约装备无关（8.8），来源仍待定（引擎 opcode 2130/2131）。


---

## 11. 勘误与补充（2026-09-27 凌晨）

### 11.1 撤回：客户端创建的实体**不会**「退化成可拾取物品」

09-26 那轮我据「不投 `[monster create trigger] > 1`」的实验（会话 `…000631`）推断
「客户端本地创建、服务端无记录的对象会退化成地面物品被捡走，天平因此消失」。**这条是错的**，现撤回。

把**全部 30 个会话**的 `pickup_scene_removed`（NOTI39）逐条解码后，**没有一条是怪物**，全是金币或普通物品：

| 载荷长度 | 结构 | 含义 |
|---|---|---|
| **47 B** | `object u32` + `actor u16` + `flag=1` + **`amount u32`** + 36×0 | **金币**（`GoldPickupConfirmed`） |
| **19 B** | `object u32` + `actor u16` + 8×0 + `actor u16` + **`slot u16`** + 0 | **物品**（`PickupConfirmed`） |

实测对照：

| 会话 | object | 解码 |
|---|---|---|
| `…011956`（17:20） | `0x100D` / `0x100E` | **金币 3229 / 3584** |
| `…000631`（16:07，开了 skip 开关） | `0x100B` / `0x100C` | **金币 3229 / 2810** |
| `…233011`（15:30） | `0x100D` / `0x100E` | **金币 3133 / 3390** |
| `…211558`（大深渊 13:38） | `0x100D` / `0x100E` | **金币 2810 / 3422** |
| `…232505` / `…235817`（两场「打不死」） | — | **无任何拾取** |

**⇒ 两条结论**：
1. `…000631` 那场天平消失，**只是因为服务端没投它**（开关的全部作用），与「被捡走」无关；
2. **掉落物与投放怪共享同一 entity 空间**（`4096 + index`，掉落紧接怪物之后取号）
   ⇒ `0x100D`/`0x100E` = `4096+13`/`4096+14`，正好是 13 只怪之后的两个空号。
3. 金币量级：小深渊 **2800–3600**；普通副本（`…211800`/`…215000`）只有 **38–200**。

> 这条勘误同时说明：**日志里出现「超出投放范围」的 entity ≠ 有本地创建的怪**。
> 判据必须先解码载荷类型（金币/物品/怪），再看 entity 号。

### 11.2 补充：召唤（奥尔泰尔）的完整门槛链

`scale_primer/action/primer_proc.act` 的 cell 级（`runtime/actcells`）：

```
[TRIGGER] gm_trigger_on            ← 无 [ENABLE] OFF（默认开），[CHECK TIME] 100
    [WHICH PLAYER] [COMPARE VAR] t:getX() < 800   [CHECKED NO] > 0
    → gm_trigger_on: [SET TRIGGER ENABLE NAME] gm_trigger ON

[TRIGGER] gm_trigger               ← [ENABLE] OFF，[LIMIT] 1
    [WHICH MONSTER] [CHECKUP] [IS INDEX] 109019402 109019403 109019404
                                          [CHECKED NO] > 0
    → summon_nox
    → [DO BEHAVIOR NAME] CHECKUP OBJECT destroy
[BEHAVIOR] summon_nox
    → [SET TRIGGER ENABLE NAME] gm_trigger OFF
    → [SUMMON MONSTER] [INDEX] 109019264 [LEVEL] -1 [POS] MAP 520 MAP 200 MAP 0 [NO EFFECT]
```

另有两条**以 `oath_rarity_progress_max` 为键**的选路触发器（都在 INIT 里被打开）：

```
nox_index_checker:  oath_max == 45                → nox_is_orthaire
                    oath_max < 45 && primer_max==45 → nox_is_wathcer
nox_pos_checker:    or(oath_max, 42, 44, 45) == 1 → summon_nox_right
                    oath_max == 43                → summon_nox_left
```

**⇒ `oath_rarity_progress_max = 72` 时四条分支全不成立** —— 也就是说，
**「天花板 72」这一个数字同时杀死了死亡路径和召唤路径**。这与「奥尔泰尔从未登场」一致。

**未验证的候选门槛**：`gm_trigger_on` 要求**某玩家 `t:getX() < 800`**。
若入场点或战斗位置始终在 `x ≥ 800`（`[SUMMON MONSTER]` 落点固定在 (520,200)），
则 `gm_trigger` 永不 arm。**廉价验证**：进本后先往左走到底再清怪。

### 11.3 补充：两个 getter 的编译期身份（IDA）

- VM 的跳转表 = `jumptable 0x147685374`，属于 `sub_147685170`。
  该函数是**脚本编译器**（逐条 `name → index` 注册到 `qword_14F36E080`），**不是运行时实现**：

  | 索引 | 名字 | case 体入口 |
  |---|---|---|
  | 2127 | （`getDistancePosX` 一族） | `0x14768A3B7` |
  | **2130** | **`getPrimerGrade`** | `0x14768A3FE` |
  | **2131** | **`getOathGrade`** | `0x14768A445` |
  | 2132 | `getEOOPartyOmenState` | `0x14768A48C` |

- 同一注册区（`0x14B2CAF38`–`0x14B2CB0B8`）内的全部名字都是 **EOO（末日守护者）副本进度族**：
  `getPrimerGrade` / `getOathGrade` / `getEOOOmenGrade` / `getEOOOmenIndex` / `getEOOPartyOmenGrade` /
  `getEOOPartyOmenState` / `isEOOPartyOmenUse` / **`getDungeonFreeEntryClearCount`**。
- 类实现文件 = `D:\Work\Jenkins5\src\DNFShared\GameScript\PrimerCollectionScript.cpp`
  （`__FILE__` 串 @ `0x14B2D0800` → xref 到 `sub_1476A8AB0` = **`PrimerCollectionScript::verifyScript`**，
  是校验器，**不是** getter）。数据 = `Etc/115LvAbility2/primerCollection.cos`（引用者 `sub_1476A6F90`）。
  `oathSystemScript.cos` 由 `sub_1474AC000` 引用。

### 11.4 方法论（本轮新增，已并入技能）

- **`idat.exe` 不可用后台跑**：回合结束会连进程一起回收 ⇒ **必须前台 `await`**。
- 全镜像 `find_imm × N 值` 在 100 MB 级代码上 **> 10 min**，不可用。
  ⇒ 改用**「反汇编里的 `jumptable <addr> case <N>` 标签 → `get_func(case_ea)` → 定点 `decompile`」**，
  一次 1 分钟内出结果。
- 强杀 IDA 后 `updvar.i64` 会残留展开文件导致 `Permission denied`（result 4）：
  删 `updvar.id0/id1/id2/nam/til` 再从 `.i64` 重开。
- 云端脚本关键字（`[UPDATE VAR]` 等）**只有 4 处引用且全是诊断消息** ⇒ 关键字分派**不走明文字符串**；
  `pvfinspect` 报的 `type=3 value=16331` **不能**当字符串池偏移，它是 VM 的 token/操作码 ID。

---

## 12. 【定性收口】真正缺的东西：服务端从未推送「誓约 / 引子 / 末日守护者」状态通知（2026-09-27 02:15）

### 12.1 结论

`getPrimerGrade()` / `getOathGrade()` 读的不是装备、也不是本地数据，而是**服务端该下发的玩家级状态**。
这份状态由下面这组通知承载 —— 协议表（`analysis/dumps/opcodes.tsv`）里全都有，**但从未在链路上出现过**：

| 方向 | 编号 | 名字 |
|---|---|---|
| noti | 2836 `0x0B14` | `ENUM_NOTIPACKET_OMEN_OF_ORDER_PARTY_INFO` |
| noti | **2837 `0x0B15`** | **`ENUM_NOTIPACKET_ENDKEEPER_OF_ORDER_INFO`** |
| noti | 2838 `0x0B16` | `ENUM_NOTIPACKET_ENDKEEPER_OF_ORDER_REWARD` |
| noti | **2839 `0x0B17`** | **`ENUM_NOTIPACKET_OATH_SYSTEM_INFO`** |
| noti | 2840 `0x0B18` | `ENUM_NOTIPACKET_TAG_TOURNAMENT_OATH_SYSTEM_INFO` |
| noti | 2841 `0x0B19` | `ENUM_NOTIPACKET_GET_OATH_EQUIPMENT` |
| noti | **2842 `0x0B1A`** | **`ENUM_NOTIPACKET_PRIMER_COLLECTION`** |
| noti | 2799 `0x0AEF` | `ENUM_NOTIPACKET_SEASON_LEVEL_DUNGEON_HISTORY` |
| cmd | 2382 | `ENUM_CMDPACKET_OATH_SYSTEM_INFO` |
| cmd | 2381 / 2385 / 2386 / 2405 / 2420 / 2421 / 2422 | `PRIMER_TRANSFORM` / `PRIMER_COLLECTION_ITEM_CRAFT` / `…_PIECE_EXCHANGE` / `REQUEST_SEASON_LEVEL_OATH` / `REINFORCE_OATH` / `SELECT_OATH_REINFORCEMENT_OPTION` / `UPGRADE_PRIMER` |

**实机统计（30 个会话）：以上全部 0 入 0 出**；服务端 Go 侧 grep 亦零实现
（`internal/` 与 `cmd/` 命中的只有 cashshop 测试里的一个巧合数字与密文表常量）。

### 12.2 因果链（这就是「打不死」）

```
getPrimerGrade() / getOathGrade()   ← 读服务端该给的玩家状态；没收到 ⇒ 回落默认 72
        ↓ 写入
c:primer_rarity_progress_max = 72        c:oath_rarity_progress_max = 72
        ↓
死亡路径要求 now >= max：primer 71≥72 ✗   oath 44≥72 ✗     ⇒ die_trigger 永不开
召唤选路要求 oath_max ∈ {42,43,44,45}    ⇒ 四条分支全不成立
```

⇒ **不是伤害、不是操作、不是客户端版本、也不是掉落表。整条闭环只卡在「服务端没给这份状态」。**

### 12.3 内建函数表（本轮整张还原）

来源：编译器 `sub_147685170` 里 `LODWORD(v475) = <id>` 紧跟的解密字符串地址，再用
`analysis/dumps/xorstr_map.tsv` **按地址反查**：

| id | 名字 | id | 名字 |
|---|---|---|---|
| 2127 | `getDistancePosX` | 2133 | `getEOOOmenGrade` |
| **2130** | **`getPrimerGrade`** | 2134 | `getEOOOmenIndex` |
| **2131** | **`getOathGrade`** | 2135 | `getEOOPartyOmenGrade` |
| **2132** | **`getDungeonFreeEntryClearCount`** | 2136 | `getEOOPartyOmenState` |
| — | （此前把 2132 误标为 `getEOOPartyOmenState`，已更正） | 2137 | `isEOOPartyOmenUse` |

VM 分派跳转表 = `jumptable 0x147685374`；内建族（2116–2142）落在两张跳转表的**空隙**里，
由显式 `cmp/jz` 链处理。

### 12.4 方法学提示（踩坑）

- **按立即数字节搜 ID 会被噪声淹没**：`852h` 在镜像里有 271 处是**数据**；其中 `sub_147695320` 的
  `cmp eax, 852h` 经反编译证实是 `[duskyisland proof]`（地下城类别名）的**别名空间巧合**。
  必须按「指令操作数精确匹配」筛，且要意识到**不同子系统共用同一 id 空间**。
- `idat.exe` **放后台会在回合结束时被连进程一起回收** ⇒ 必须**前台等**。
- 强杀 IDA 后要删 `updvar.id0/id1/id2/nam/til` 再从 `.i64` 重开（否则 `Permission denied`，result 4）。

### 12.5 下一步（已缩到可执行）

IDA 只需读**客户端对 `noti 2837 / 2839 / 2842` 的处理函数**，拿载荷结构：
字符串地址 `0x14b03d220` / `0x14b03d2e0` / `0x14b03d3f0`，
`opcodes.tsv` 第 5 列的 descriptor 地址 `0x14ef38d58` / `0x14ef38d68` / `0x14ef38d80`。
拿到结构后服务端照发即可（并且是从登录起就该发的常驻状态）。

### 12.6 IDA 续挖（2026-09-27 02:2x）：名字表已完全定位，载荷结构仍在墙后

**已确证的结构（下轮可直接用）：**

| 项 | 值 |
|---|---|
| noti 名字数组 | **base `0x14EF334B0`，约 2900 项 × 8B，按 opcode 直接索引**（由 `noti 2836 → slot 0x14EF38D50` 反推） |
| cmd 名字数组 | **base `0x14EF38F60`**（`cmd 0 → slot 0x14EF38F60`） |
| 填充者 | `sub_140075000`（巨大的 `qword_X = sub_146E8C7D0(word_Y)` 初始化器，2908 行 / 55 KB） |
| 名字表的读者 | `sub_1459A1BB0`（**抓包日志**）、`sub_146D73A70`（**协议错误上报**，`qword_14EF38F60[opcode]`，含 `opcode >= 0x97C` 时用通用名的判断） |
| noti 分派 switch | **`sub_146753320`，跳转表 `jpt_146753395` @ `0x14679D898`，4175 项**（case 体集中在 `0x14675xxxx`） |
| 反编译状态 | `sub_146753320` / `sub_141192BC0`(381 KB) / `sub_1412327C0`(669 KB) **全部返回 `None`**（Hex-Rays 放弃） |

**⇒ 结论：名字层已经完全清楚，但「noti 索引 → 处理函数」这一跳落在一个 4175 分支、反编译失败的巨型函数里；
下一步要么升级到「按 `jpt` 表反查 + 汇编级读 case 体」，要么换路子。**

**已排除的干扰项（都是 alias 巧合，别再追）：**
- `0xB15/0xB17` 的数据命中 = RVA 字节巧合（如 `0x0B159520` 的 `15 0B` + 尾随零）。
- `stru_14B159520` 等 = 异常/展开表，不是包描述符。
- `sub_144FBFF30` 里的 `2837` = **UI 消息号**（`sub_146682140(ctx, 2837, …)`），不是包索引。
- `opcodes.tsv` 第 5 列 = **解码后的名字槽**（BSS，运行时填），不是 handler。

**两条更便宜的路（建议优先于继续硬啃）：**
1. **让客户端自己暴露**：在游戏里**打开誓约/引子界面**（`oathAndPrimer` / `SilentTruthUpgrade_Main` / `PrimerCollectionWindow`），
   看客户端是否发出 `cmd 2382 / 2405 / 2420 / 2422` —— 若能捕获**请求的载荷几何**，
   通常与 noti 同构，可据此推出结构（30 个会话里这几条 cmd **一次都没出现过**，说明 UI 从未被打开过）。
2. **升级 IDA 打法**：定位 `jpt_146753395` 的**基准索引**（找到写入 `var` 的判断），
   再按 `noti 2837` 反查 case 体，用**汇编级**读它调的 handler；或直接找 `[oath/primer]` 结构体的写入点。

### 12.7 IDA 续挖（2026-09-27 02:3x）：通知的**处理函数已全部定位**，但载荷是**惰性解析**

**分派器结构（已完全读通）：**

```
sub_146753320(ecx=包ID, rdx=载荷)          ← 巨型瘦分派器，栈帧 0x11628
  mov eax,[arg_0] ; dec eax ; cmp eax,104Eh ; ja def
  mov eax, jpt_146753395[rax*4] ; add rax, rcx ; jmp rax
  索引域 = 1..4175（jpt[0] = case 1，已验证）；jpt 表 @ 0x14679D898
  每个 case 形如：  call sub_146E9F2A0(classId) → 取对象 ; call <处理函数>(obj, payload, 1)
```

**三个目标的 case（已读出）：**

| noti | classId | 处理函数 |
|---|---|---|
| 2836 `OMEN_OF_ORDER_PARTY_INFO` | `0x728` | `sub_144FC1120` |
| **2837 `ENDKEEPER_OF_ORDER_INFO`** | **`0x9B8`** | **`sub_14412FBC0`** |
| 2838 `ENDKEEPER_OF_ORDER_REWARD` | `0x788` | `sub_14412FAD0` |
| **2839 `OATH_SYSTEM_INFO`** | `0x7C0` | **`sub_1448097B0`** |
| 2840 / 2841 | — | `sub_144808280` / `sub_144808840` |
| **2842 `PRIMER_COLLECTION`** | — | **`sub_145123B70`** |
| 2799 `SEASON_LEVEL_DUNGEON_HISTORY` | — | `sub_141FAE590` |

**构造链（逐层读通，全部是 C++ 分包对象框架的构造器）：**

```
sub_14412FBC0(obj, payload, 1)         # 派生类 ctor：设置 vtable + 清零派生字段
  └─ sub_145450460(obj, payload, 2837, name, 0)     # 基类 ctor
       └─ sub_145F6DA20(obj, payload, id, name, 0)
            └─ sub_14674C150(obj, payload, id, 0)
                 ├─ 清零基类字段 0..900
                 └─ sub_146752340(obj, payload, id)   ← 决定性的一层
                      ├─ *(_QWORD*)(obj + 848) = payload      # 只存指针
                      └─ **(_DWORD**)(obj + 40) = id          # 只存包 ID
```

**⇒ 载荷**不在此刻解析**：对象只是把 (缓冲区指针 @obj+848, 包 ID @obj+40) 存下来，
字段由**访问器惰性读取**。而这些访问器正是脚本内建 `getPrimerGrade()` / `getOathGrade()`
（`getEOOOmenGrade` / `getEOOOmenIndex` / `getEOOPartyOmenGrade` / `getEOOPartyOmenState` 同族）。

**⇒ 所以「字段偏移」只存在于内建访问器的实现里** —— 也就是必须拿到 VM 对内建 ID 2130/2131 的分派目标。
这也是 12.6 里那条路的真正终点。

**本轮新增可复用资产：**
- 分派器 `sub_146753320` + 跳转表 `jpt_146753395 @ 0x14679D898`（索引域 1..4175，`jpt[id-1]`）
  ⇒ **任意 `cmd`/`noti` 的客户端处理函数都可一键定位**（`call sub_146E9F2A0(classId)` 的返回对象 + 后面的 `call`）。
- 三个已读通的构造器层：`sub_145450460` / `sub_145F6DA20` / `sub_14674C150` / `sub_146752340`。
- 该家族对象的 vtable：`off_14A9940B0` / `off_14A214500` / `off_14A832850` / `off_14920D208`。
  **下一步若走 IDA：从 vtable 入手找访问器**（比继续找 VM 分派更直接）。

---

## 13. 【2026-09-27 03:0x】「72 是不是任务 ID」→ 否；但**深渊的解锁确实由任务门槛 + 名望门槛控制**

### 13.1 直接否证：72 不是任务 ID

| 项 | 实测 |
|---|---|
| 任务目录 | `configs/quests.generated.json`（源 = `list/quest.lst`），**2844 个任务** |
| 任务 ID 值域 | **649 ~ 23128** |
| **是否存在 ID 72** | **不存在**（最接近的也差得远） |

⇒ 72 与任务号无关。

### 13.2 但「引导任务解锁」这个方向**成立**：`etc/ContentsOpenCondition.ctp`

用仓库自己的 CTP 解码器（`runtime/ctpdump`）解出 **260 条记录 / 45 个 `[contents]`**，每个内容带
`[open condition]` = `[quest clear]` 或 `[quest accept]` + `[level]` + **`[fame]`（名望）**：

| 内容 | 条件 | 等级 | 名望 |
|---|---|---|---|
| **`SkyOfThousandSeas`**（千海之空总内容） | **quest clear** | 115 | — |
| **`100005014`（小深渊 EndkeeperOfOrder）** | **quest clear** | 115 | **13632** |
| **`100005067`（大深渊 legendary）** | **quest clear** | 115 | **13632** |
| `100005013` | quest clear | 115 | 91582 |
| `100005015` | quest clear | 115 | 58950 |
| `100005062` / `100005063` | quest clear | 115 | 34749 / 33249 |
| `100004194` | quest clear | 115 | 23016 |
| `Apocalypse` | quest clear | 115 | 73993 |
| `Asrahan` | quest clear | 110 | — |
| `115Lv_NormalDungeon` / `115Lv_MaleficDungeon` | quest clear | 115 | — |

**玩家名望 23,700 > 13,632 ⇒ 名望这一半是满足的；未满足的是「通关某个任务」。**
而服务端对 `ContentsOpenCondition` **零实现**（`grep` 空），也不存在相应的每内容开放状态下发。
⇒ 客户端拿不到「该内容已开放」，`SkyOfThousandSeas`/深渊在客户端侧处于**未开放**状态。

> 附：客户端字符串里另有 `[contentsInfo]` / `contentsInfoPanel` / `opencondition` 与韩文报错
> `etc/ContentOpenCondition.ctp 파싱에 실패했습니다!`（解析失败），说明这张表是客户端 UI 的硬依赖。

### 13.3 引导系统确实存在，且用「必需任务」绑副本

```
etc/guidesystem/100lvgrowthguide.etc      [dungeonIndex] 100000003  [necessaryQuest] 12172
                                          [dungeonIndex] 100000151  [necessaryQuest] 12177
etc/guidesystem/100lvfarmingguideregulartab.etc   [dungeon info] … [necessary questindex] 12121 / 12143 …
etc/guidesystem/100lvfarmingguidespecialtab.etc   同上（1..7 项）
UI/GuideSystem/IngameNoticeForSkyOfThousandSeasWindow.xui   ⇒ 千海之空有专门的游戏内提示窗口
```
另有一族引导任务 `contents/2022/110levelextensionsystem/guidequestsystem/quest/guide_quest_NN.qst`
（`[grade] `guide``，共 37 个），以及 EPLP 引导机制（`[check eplp next quest]` / `cmd 72 EPLP_COMMAND`）。

**千海之空主线链**（`contents/2026/skyofthousandseas_scenario/quest/`）：
`q23028`（瘟疫德莱齐 14）→ **q23053** GrandConstellationTurtleLibrary（副本 `100005114`）→ `q23054`（`100005115`）
→ `q23055`（`100005116`）；支线 `q23068~23080`（`Pledge_of_Light_01..12` + `Mist100`）。
其中 **`mist100.qst` 带 `[required season level] 100`** —— 赛季等级也是任务门槛。

### 13.4 一个新的闭环：深渊自己就是「赛季经验」的来源

`Contents/System/SeasonLevel/main.cos` 的 `[dungeon categories]` 把深渊登记为赛季经验副本：

| 副本 | 分类 | `[default exp]` |
|---|---|---|
| 100005066 | normal dungeon | **255** |
| 100005067 | normal dungeon | **1050** |
| 100005068 | normal dungeon | **2250** |
| **100005014（小深渊）** | normal dungeon | **336** |
| 100005013 | higher dungeon | 6000 |
| `oath` 类（仅 `100005266`，`[cs only]`） | oath | 2000/18000/108000/800000 |

而 `[exp level chart]` 给出 **1…102 级**的 `[acc exp]` + **`[add fame value]`**：
L1→4999/2415、L71→6839999/5315、**L72→7171999/5330**、L102→33563999/6075（累计名望 474,710）。
`Etc/115LvAbility/SeasonLevel.lst` = `SeasonLevel/N.etc`（等级属性表，如 `72.etc` → `[skill bonus rate] 42.30`
`[equipment buff] 8148`）。

⇒ **打深渊 → 赛季经验 → 赛季等级 → 名望 + 属性 → 解锁更高内容**。服务端这一环**完全没有结算**。

### 13.5 「72」在数据里的确切位置（新发现）

`etc/115LvAbility2/oathsystemscript.cos` 的 **`[remain parameter]`** 是一张**点数阈值 → 属性表文件**的映射：

```
25 → RemainParameter/1.etc   50 → 2.etc   75 → 3.etc  …  1000 → 40.etc  1025 → 41.etc
… 1800 → 72.etc … 3500 → 140.etc          （共 140 档，规则严格为 key = 索引 × 25，0 例外）
```

⇒ **`RemainParameter/72.etc` ↔ 誓约点数 1800**，其内容为 `[skill bonus rate] 42` / `[equipment buff] 7140`
（邻居：70→41/7000、71→41.5/7070、73→42.5/7210，线性）。

> 同目录另有 `equipmentoath/addparameter/1..45.etc`（1..45，`[skill bonus rate] = N`、`[equipment buff] = N×100`），
> **其值域 1..45 与机制里的档位码 40..45 同域**；以及 `equipmentoath/basestat/{rare,unique,legendary,epic,primeval}_stat.etc`。

### 13.6 ⚠️ 一处必须修正的旧结论

我此前写过「服务端没发 noti ⇒ 客户端回落默认 72」。**这个说法是错的**：

- `.dgn` 的 `[create var]` 里**数据默认值就是 40**（`c:primer_rarity_progress_max = 40`、`c:oath_rarity_progress_max = 40`）；
- 随后 `RARITY_CHECK_START`（`primer_proc.act` cells 0-10）用 `getPrimerGrade()` / `getOathGrade()` **覆盖**它们；
- 包对象的构造器会把字段清零（`sub_14674C150` 清零 0..900），若真走「没收到包」路径，值应是 0/40，**不会是 72**。

⇒ **72 是引擎 getter 主动算出来的**，来源就在誓约/赛季这一族本地状态里（正是服务端零实现的系统）。
结论不变（缺的是誓约/引子/赛季状态），但**因果叙述必须改成「引擎用本地状态算出 72」，而不是「回落默认」**。

### 13.7 频道：不是硬门槛，但值得记录

- 客户端 `etc/clientchannelinfo.etc` 共 **41 个 `[channelType]`**（45/50/67/76/81–120…），带
  `isLegion`/`isRaid`/`isSemiRaid`/`seriaRoomTown`/`isSpecialRegionChannel` 等；
- 服务端只开 **4 个**（`configs/channel.local34.json`）：`id=1/type=2`、`id=6/type=3`、`id=10/type=22`、`id=119/type=119`；
- `etc/channel_info.etc` 的 `[dungeon]` 组只含**旧版低号副本**（elven_guard 1-2、granfloris 3-9…），
  **现代 10000xxxx 副本不在其中** ⇒ 频道表不构成深渊的入场门槛（玩家本来就能进）。
  **⇒ 频道不是这次的阻塞点**；内容开放状态（13.2）才是。

### 13.8 下一步（按性价比排序）

1. **把「内容已开放」补上**：至少让客户端认为 `SkyOfThousandSeas` 与 `100005014`/`100005067` 已开放
   （需要先定案「`[quest clear]` 具体指哪个任务号」；CTP 只给标记不给任务号 ⇒ 需从引导/EPLP 侧或 IDA 追这一跳）。
2. **补誓约 / 引子 / 赛季状态推送**：`noti 2837/2839/2842`、`noti 2799`、`noti 2858`、`cmd 2405/2419`
   （已确认服务端零实现，30 个会话 0 入 0 出）。
3. **赛季经验结算**：`100005014` +336 / `100005066` +255 / `67` +1050 / `68` +2250（每通关一次）。
4. 仍待钉死：`getPrimerGrade()` / `getOathGrade()` 读的到底是誓约点数、赛季等级，还是两者混合。
   现在有两条现成判据：**誓约点数**（72 ↔ 1800）与**赛季等级**（1..102，`[add fame value]` 累计）。

### 13.9 【2026-09-27 02:5x】内容开放 ≠ 天平打不死的原因（业主反问成立）+ 本轮负面结果

**业主反问**：`100005067`（大深渊）与 `100005014`（小深渊）在 `ContentsOpenCondition.ctp` 里的条件**逐字相同**
（`quest clear` / 115 / fame 13632），而大深渊能打能结算 ⇒ **内容开放门槛不可能是天平打不死的原因。**

**⇒ 采纳**。据此把两条线彻底分开：

| 线 | 性质 | 对「天平可杀」的作用 |
|---|---|---|
| 内容开放（`ContentsOpenCondition` + 任务清关 + 名望） | 真实缺口，但影响的是**世界地图/UI 的开放状态** | **无**（且深渊本来就进得去 ⇒ 客户端不拦入场） |
| 誓约 / 引子 / 赛季**玩家状态**（`noti 2837/2839/2842/2799/2858`、`cmd 2405/2419`） | 真实缺口 | **就是它** —— 引擎 getter 靠这份状态算 `*_rarity_progress_max` |

**「补上任务这一环能否让天平可杀」⇒ 不能。** 天平的可杀性只由 `primer_now >= primer_max` 决定，与内容开放/任务无关；
大深渊即对照组（它不走这两个 getter，所以不受影响）。

#### 本轮追「内容 → 具体任务号」的**负面结果**（别再重复走）

1. `contents/2026/skyofthousandseas_scenario/quest/` 全部任务（q23053/54/55、q23068~23080、q23110）**没有一个**
   引用副本 `100005014` / `100005066` / `100005067` / `100005068`；全 2844 个任务里对这四个副本号**零引用**。
2. `etc/contentsguide/contentsguidetextinfo.etc` **只是内容引导弹窗的排版表**（`[CONTENTS INDEX]`/`[IMG INDEX]`
   + `[STR]`/`[FONT]`/`[POS]`），**不含任务映射**。
3. `ContentsOpenCondition.ctp` 的 `[quest clear]` 节点是**空标记**（`cells=0`），**表内不含任务号**。
4. 客户端 `[contentsInfo]` 段名只出现在 `etc/dungeonpvp/dungeonpvpcontentsinfo.etc`（PVP），不是这里。
⇒ **「内容→任务」这一跳在数据层没找到**，要从 IDA 或客户端 UI 行为反推。

#### 本轮追 getter 数据源的**负面结果**（IDA）

- `getPrimerGrade`/`getOathGrade` 属 EOO 族，管理器对象由 `sub_146E9F2A0(classId)` 取；**classId**：
  `2837 → 0x9B8`、`2839 → 0x7C0`、`2836 → 0x728`、`2838 → 0x788`。
- 全镜像扫 `call sub_146E9F2A0` **1695 处**（该访问器过于通用）；**按 classId 收窄后 96 处**，其中
  **94 处全在包分派器 `sub_146753320` 内**（每个 case = 取对象 + 调 handler），**只有 2 处在外面**：
  - `sub_141D217A0`（classId `0x9B8`）= **征兆 UI 网格绘制**（读纹理宽高 `width/4`、`height/3`，32 × 18 格）—— 不是 getter；
  - `sub_1463F70B0`（classId `0x788`）—— 待看。
- 三个 noti handler（`sub_14412FBC0` / `sub_1448097B0` / `sub_145123B70`）**全文读过：纯构造器**
  （SEH `__wind` 逐字段清零 + vtable 赋值），**没有任何载荷字段解析** ⇒ 字段布局确实只在访问器/getter 里。
- ⇒ **访问器入口这条路走空了**。下一步必须换入口：① 定位 VM 的 **builtin id → handler 表**（编译期表只给 `name→id`；
  运行时表一直没定位到，`jumptable 0x147685374` 是编译期那张）；② 从**虚表**（`off_14A214500` / `off_14A9940B0` /
  `off_14A832850` / `off_14920D208`）反查读 `+0x740..0x890` 区间的虚函数。

---

## 14. 【2026-09-27 03:1x】落地实现：服务端在机关血量触底时宣布它死亡（业主方案）

### 14.1 为什么这条可行（三条硬事实）

1. **服务端本来就收得到机关的血量。** 客户端每秒发一次 **CMD 2329**（`ENUM_CMDPACKET_MONSTER_HISTORY_LOG`），
   载荷实测 `272 B`，布局为：

   ```
   [0:4)   u32（值 1）
   [4:260) char[256]  ASCII 文本，含 "HP : <float>, Action : …, rarities : …, omen : …"
   [260]   u8 = 1      [261] u8 = 1
   [262:266) u32 LE = 怪物模板号   ← 实测 `82 80 7f 06` = 0x067F8082 = 109019266（定盘机关）
   [266:272) 6 × 0
   ```

   该文本只在 `safe_timer >= 60` **且机关正在挨打**时才发（`primer_proc.act` 的那条 `[SERVER LOG MSG]`），
   所以**第一条样本就是满血**（实测 `4736540`），随后掉到脚本自己钳住的地板（实测 `94730` = 2%）。
   **⇒ 服务端完全可以用它判断「玩家真的把它打到血底」。而此前服务端对 2329 零实现。**

2. **服务端有现成的「死亡广播」给客户端。** `NOTI 38`（`protocol.MonsterDeathConfirmed(entity)`）就是
   正常死亡路径发给客户端的那一条 —— 客户端本来就是按它来处理死亡/消隐的。所以由服务端主动补发它，
   走的是客户端**本来就认**的通道，而不是我们臆造一个包。

3. **只判死机关还不够。** `tryComplete` 的源领主路径（`internal/dungeon/completion.go` §142）要求
   `atSourceBossMap() && roomEnemiesDead()`，而 `roomEnemiesDead()` 要求**房间里不再有活着的可击杀目标**
   —— 同坐标 `(3168,191)` 的另一台装置 **`109019280 Scale_oath`（map 索引 12 行，`[fixed] [normal]`）同样是
   `[fixture]`、实测同样从未上报过死亡**。所以必须让它一起退场，否则通关判定仍然不成立。

### 14.2 实现（默认关闭）

新增 `cmd/wireprobe/scale_death.go`：

- `decodeScaleStatus(payload, want)`：从 CMD2329 载荷里取**模板号**（尾部小端 u32，且必须命中本次运行的
  关注集合——避免把别的机器人的日志当成本机关）与 **HP**（从 `"HP : "` 后取浮点）。
- `worldSession.scaleStatus(...)`：记录**见过的最高血量**（即满血），当
  `hp <= peak * 5%` 时触发。触发时：
  1. 先把 `109019280` 按**无主**处理（`ConfirmDeath(sibling, killer=65535, …)` ⇒ `unowned` ⇒ 不给掉落/经验），
     并补发一条 `NOTI 38`；
  2. 再**合成机关自己的 CMD39 载荷**（`entity u32 + killer u16 + 零填充到 64 B`，满足 `DecodeMonsterDeath`
     的几何要求），喂给**现成的 `monsterDeath`**。
  ⇒ 于是掉落 / 经验 / 任务 / 通关判定 / `NOTI 38` **整条既有路径原样复用**，没有另起一套逻辑。

接线：`request_scope.go` 的 `dungeonRequest` 放行 2329；`main.go` 的 dungeon switch 加 `case 2329:`；
开关 `-scale-death-from-hp` / `DFO_SCALE_DEATH_FROM_HP=1`（**默认关闭**）。

**默认关闭时行为与改动前逐字节一致**（`scaleStatus` 直接返回 `nil, nil`；既有测试全绿）。

### 14.3 触发线为什么定在「峰值血量的 5%」

脚本自己的死亡条件写的是 `[ON DAMAGE] [WHICH ME] o:getHpRate() <= 2`，实测地板就是 **2%**，
所以定 5% 留了余量。分母用**本次运行观察到的峰值血量**而不是某个写死的最大值，
是因为地形/难度会改 HP；而由于日志只在挨打时才发，**峰值就是满血**，这个分母是可靠的。

### 14.4 判据与回退

- **判据（实机一场即可）**：进小深渊 → 清 11 只杂兵 → 打到机关血底 →
  - ✅ 日志出现 `scale_status`（含 `hp/peak_hp/rate`）与 `scale_death_forced`；
  - ✅ 随后 `monster_death_confirmed`（机关 + 同坐标装置）；
  - ✅ **客户端出现 GO / 结算面板** ⇒ 这条路走通（说明客户端接受服务端主动补发的 NOTI38）。
  - ❌ 若 `scale_death_forced` 出现但客户端不认（无结算）⇒ 说明客户端另有本地校验，
    那就退到「服务端直接宣布通关」那一步（不看客户端的清关条件）。
- **回退**：删掉 `scripts/launch_local.py` 里 `env["DFO_SCALE_DEATH_FROM_HP"] = "1"` 那一行即可（代码保留、默认关）。

### 14.5 与「修好 72」的关系（必须写清）

这是**绕过**，不是修好：机关的脚本状态机仍然卡在 `now >= max`（71 ≥ 72 不成立）。
它换来的是**玩法闭环可用**（能打完、能结算、能拿奖励），并且顺带验证了一个独立未知量
——**客户端是否接受服务端主动补发的死亡广播**。真正的修复仍是补
`noti 2837/2839/2842/2799/2858` 与 `cmd 2405/2419`（§12、§13）。

默认关闭、**未提交**（按纪律：实机验证通过 + 业主确认后才 commit）。

---

## 15. 【2026-09-27 03:2x】实机验收：绕过生效、结算成立；以及「只给了一个紫装」的三条独立原因

会话 `runtime/roles_persist_..._20260927_031849_693311_next37`（客户端 03:18:49 起，03:21:51 回城）。
开关注释：`DFO_SCALE_DEATH_FROM_HP=1`，exe = `bin/wireprobe-handoff-source.exe`（`74A1142A…`，03:14:07 重编）。

### 15.1 通关链逐条落地（业主口述「确实结算了」= 成立）

```
19:21:02.059  C2S 2329  HP : 4736540.00 … rarities : 71, 72, 44, 72   → scale_status(rate 100)
19:21:02.561  C2S 2329  HP :  471394.00                             → scale_status(rate 9.95)
19:21:03.054  C2S 2329  HP :   94730.00                             → scale_status(rate 2.00)
19:21:03.054  scale_death_forced {entity 4107, sibling 4108}
19:21:03.069  scale_sibling_confirmed  (NOTI38 entity 4108)
19:21:03.069  monster_death_confirmed  (NOTI38 entity 4107) + monster_death_ack(39) + exp(37)
19:21:03.069  map_clear_quest_triggers(291) / boss_check_confirmed(115) / dungeon_clear_enabled(31)
19:21:03.107  dungeon_play_result(34) / dungeon_clear_experience(37) / dungeon_clear_reward(35)
19:21:03.107  eplp_rechallenge(261) = 09   ← 「继续挑战」按钮亮起
19:21:50.524  C2S 72 → settlement_exit_ack(72) / dungeon_return_area(23) / town_actor_state(3)
```

`boss_check_confirmed` 载荷 `01 01 0b10` = 数量 1、entity `0x100b` = 4107
⇒ **服务端认的通关目标就是天平本体**（不是真 BOSS）。链路与设计完全一致。

### 15.2 结算面板里到底有什么（`dungeon_clear_reward` = NOTI35，310 B，逐字段解码）

按 `internal/game/protocol/settlement.go` 的编码反解：

| 偏移 | 字段 | 值 |
|---|---|---|
| 0 | `BaseExperience` | **3,151,739** |
| 4 | `ScoreExperience` | 0 |
| 167+29 | result-window 标志 | 1 |
| 169+29 | `MonsterExperience` | 0 |
| 131.. | 卡牌组（8 组，组间交错） | 第 0 组 1 行：`template 0 / amount 627`（=**金币 627**）；第 1..7 组各 0 行 |
| 265+29 / 277+29 | 两个可选事件结果 | `0xFFFFFFFF`（缺席） |

⇒ **结算面板只承载「经验 + 金币卡」，不含物品**。玩家看到的「一个紫装」不来自这里。

### 15.3 这场到底掉了什么（把 NOTI38 的掉落行按 `OrdinarySceneDropRecord` 全解）

11 只杂兵（entity 4096..4106）+ 天平（4107）+ 同坐标副装置（4108）。掉落行 = `Object u32 + Item[181] + Aux u32 + Sentinel u16 + Owner u16`：

| 死亡 entity | 掉落 Object | `Item[0:2]` slot | `Item[2:6]` template | `Item[6:10]` | 判读 |
|---|---|---|---|---|---|
| 0x1006 | 0x100d | 1 | **0** | 3293 | 金币 3293 |
| 0x1004 | 0x100e | 2 | **0** | 2810 | 金币 2810 |
| 0x1009 | 0x100f | 3 | **0** | 3165 | 金币 3165 |
| **0x100b（天平）** | 0x1010 | 4 | **100130928** | 0 | **真正的装备**（`EquipmentRow` 写 template 在 [2:6]、durability 在 [11:13]，实测 `[11]=0x32=50`） |

`template 0 + amount` 就是金币（`OrdinarySceneDropRecord` 自己那条 `zero gold drop` 守卫印证）。
那件装备查全量目录：

```
runtime/equipinfo -id 100130928
  path = equipment/character/common/pants/harmor/100130928.equ
  [equipment type] = [pants]      [grade] = 107      [minimum level] = 105
  [rarity] = 2                    [attach type] = [free]     [item group name] = "ha pants"
```

**`[rarity] = 2` = 紫**，部位是重甲下装 ⇒ 业主说的「只给了一个紫装」= 就是它，而且是**天平自己那一格掉落**。
（`runtime/equipinfo` 本轮新增，被 gitignore。）

### 15.4 「只给一个紫装」的三条独立原因（都可证，别混成一条）

1. **本副本自己的奖励表还没接线。** `endkeeperoforder.dgn` 声明了
   `[reward boost dungeon]`、`[difficulty dropitem group list] [group info] [item index]`、
   `[special setinfo reward] ×4`、`[special custom reward info]`，但
   `etc/endkeeperoforder.ctp` **尚未导入**、`[normal group index]` 也**未接线**
   （`docs/protocol/border-of-attunement-drops-plan-20260926.md`「仍然未做」第 2 条）。
   实机启动日志佐证：`loaded attunement rewards (3 dungeons [100005066 100005067 100005068], …)`
   —— **不含 100005014**。
2. **跑起来的那条通用掉落路径，天花板就是紫。**
   `internal/inventory/equipment.go:290 Basic()` 要求 `[attach type] == "[free]"` **且 `[rarity] <= 2`**，
   不满足即「需额外源状态」被踢出掉落池 ⇒ 通用池**物理上给不出神器/史诗/太初**。
   grade 上限 107 也是同一条规则的产物（与 09-26 大深渊那轮「未决 A」是同一个开关）。
3. **必出太初的那条路（隐藏 BOSS）这一整场没被打开过。** 见 §15.5。

### 15.5 72 的第二个后果：它把 `[ON DAMAGE]` 状态机钉在 `GO_RAINBOW1` 上

§4b 那张表的第一条分支是：

```
[ON DAMAGE]（打的是 109019280）
  [BEGIN IF] primer_rarity_progress_max > 69  → primer_up_to_rainbow1 / GO_RAINBOW1
  [ELSE IF]  primer_now == primer_max && oath_now < oath_max → only_oath_up
        oath_now == 44 && is_oath_epic_loop
              c:nox_die == 0 → summon_orthaire        ← 真 BOSS（奥尔泰尔）登场
```

`primer_max = 72 > 69` **恒真** ⇒ 第一条分支永远命中 ⇒
`primer_now == primer_max`（71 == 72）永远不成立 ⇒ **`summon_orthaire` 是死代码** ⇒
`109019264 orderchroniclerorthaire`（Lv135，业主所说「必出太初」）**本场一次都没出现**。

实机侧证据：整场 `monster_death_confirmed` 只有 entity 4096..4111（13 条），
**没有 109019264 的创建、也没有它的死亡**；19:19:58（杂兵清完）到 19:21:03（天平退场）之间
服务端一次 spawn 事件都没记。

⇒ **同一个 72 同时造成两件事**：① 天平打不死（§8b）；② 隐藏 BOSS 永不登场 ⇒ 没有太初。
业主「可以绕过」成立，但**绕过死亡 ≠ 绕过奖励闸门**。

### 15.6 「72 是不是区域标记（最后的圣地 → 弃神广场）」—— 否定，但方向对了一半

**先看客户端自己怎么称呼它。** `primer_proc.act` 的日志格式串（cell 2074-2090）把它绑死为：

```
rarities : %d, %d, %d, %d
[ARG VAR] c:primer_rarity_progress_now  c:primer_rarity_progress_max
          c:oath_rarity_progress_now    c:oath_rarity_progress_max
```

⇒ 这个槽位是**稀有度进度/上限**，不是区域号。72 是**被当作稀有度上限消费**的。

**再说「它是不是那个区域的号」——用 `list/town.lst` 直接否掉：**

| 名字 | 城镇号 | 路径 |
|---|---|---|
| 最后的圣地 | **241** | `Town/FinalSanctuary_Scenario.twn`（区域地图 `commonmap/town/260326_skyofathousandseas/finalsanctuary/*.map`）|
| 弃神广场 | **242** | `Town/GodforsakenPlaza_Scenario.twn`（区域地图 `…/godforsakenplaza/*.map`）|

175 个城镇里**没有任何一个是 72**（60..68、70、75..80 有，69/71/72/73/74 都空）。
本场回城包 `dungeon_return_area`(23) 里的 `f1000000` = **0xF1 = 241** = 最后的圣地，也对得上。
⇒ **「72 = 那个区域的 id」不成立**。（`→` 是城镇层级：先到 241 再到 242，不是编号关系。）

**但业主的直觉有一半是对的：72 确实「属于内容」而不是「属于玩家」。**

- 45 个 `CMD2329` 样本、**跨全部会话**，四个数**逐字恒为 `71, 72, 44, 72`** ——
  连 `*_now` 都不动（脚本 `PRIMER_UP_*` 本该把它从 40 抬上来，实测从第一帧起就是 71/44）。
- 同一个 72 同时是 `primer_max` 与 `oath_max`（两个独立 getter），且 09-27 01:21 卸掉誓约装备后**逐字不变**。
- ⇒ 它是**内容级常量**，不是玩家进度；只是**被消费成稀有度上限**。

**值域旁证：** 只有 8 档 —— `40..45` + `70/71`，正好对应
`primer_00_normal_loop.act` … `primer_07_rainbow2_loop.act` **八份逐档脚本**。
**72 = 71 + 1 = 值域外一格**。这与 §13.6 的修正一致：不是「没收到包回落默认」（PVF 默认 40），
而是引擎在**服务端零实现的那套誓约/引子状态**上算出/取到了一个越界值。

⇒ 结论：**「72 指向某个区域」没有被数据支持**；但「这个常量应当由内容/区域侧提供」这个方向，
与「缺 noti 2837/2839/2842」是同一个待办，可以并行验证（见 15.7）。

### 15.7 下一步（按性价比，需业主拍板）

1. **把小深渊自己的奖励表接上（最高性价比，且不是策略变更）**：
   `etc/endkeeperoforder.ctp`（或 `etc/rewardboostinfo/endkeeperoforder/*.ctp`）导入 +
   `[difficulty dropitem group list] [normal group index] 1 21251` 接线。
   参考大深渊那套：`cmd/attunementimport -base <目录>` → `configs/*.generated.json`
   → `DFO_ATTUNEMENT_REWARDS` → `internal/loot`。
   ⚠️ 读 CTP 要开内层 PVF ⇒ **先退客户端**（`pvfinspect -find` 那类检索要约 406 MB 连续内存）。
2. **是否放开通用掉落池的 `rarity <= 2`**：这是**策略变更**（09-26 已挂「未决 A」），需要业主明确。
   不改的话，通用池永远只可能给紫。
3. **补 noti 2837/2839/2842**（治本，让 72 落回值域）—— 载荷结构仍在那面墙后（§12.6/§12.7）。
4. **隐藏 BOSS 的服务端兜底**（可选、需拍板）：像大深渊那样，在天平退场时按副本自己的奖励表结算，
   而不是等客户端 `summon_orthaire`。**这会改变行为**，必须先定「发什么表」。

---

## 16. 【2026-09-27 04:0x】把小深渊自己的奖励表接上（业主定的第一优先级）

业主定调：**深渊各有独立奖励表，不要放开通用池的 `[rarity]<=2`**；先接小深渊的表，再补 noti。

### 16.1 表在哪：两张，作用不同（顺手纠正一处旧叫法）

`pvfinspect -find endkeeperoforder` 命中 7135 条，其中只有两条奖励相关：

| 路径 | 大小 / sha256 | 是什么 |
|---|---|---|
| `etc/rewardboostinfo/endkeeperoforder/normal.ctp` | 4545 B / `f5c1966e98824dc8…` | **真正的掉落表**（`[dungeon index] 100005014`；2×`[fixed drop table]`(按 maze) + 2×`[additional drop table]` + **5×`[coupon drop table]`**）|
| `contents/2026/endkeeperoforder/etc/endkeeperoforder.ctp` | 1999 B / `97c75e3db000210b…` | **不是奖励表**：`[omen drop pos info]` / `[oath drop pos info]` / `[omen drop gravity]` / `[omen drop delay time]` / `[map loop length] 2600` / `[first omen clear count] 30` / `[omen tooltip info]` —— 是征兆/誓约掉落装置的**参数表** |

上一轮文档把后者写成「一张奖励表」，此处更正。

### 16.2 接入方式：生成器加 `-extra`，运行时零改动

`cmd/attunementimport` 原本硬编码 `{unique,legendary,epic}.ctp`；新增
`-extra <逗号分隔的归档路径>`，于是**一份** `configs/attunement-rewards.generated.json` 可以携带多个副本的表
（表自己声明 `[dungeon index]`，加载器拒绝重复的副本号）：

```
go run ./cmd/attunementimport -extra etc/rewardboostinfo/endkeeperoforder/normal.ctp
  unique.ctp     dungeon=100005066  fixed=1 additional=10 hidden=0 coupon=0
  legendary.ctp  dungeon=100005067  fixed=1 additional=12 hidden=0 coupon=0
  epic.ctp       dungeon=100005068  fixed=3 additional=14 hidden=6 coupon=0
  normal.ctp     dungeon=100005014  fixed=2 additional=2  hidden=0 coupon=5
  wrote configs/attunement-rewards.generated.json (25647 bytes, 4 tables)
```

`cmd/wireprobe` 侧**一行都没改行为**：`lootService.Attunement` 本来就按副本号查表，触发点是
`monster.Rank == 3 && monster.Template == d.Definition.SourceBoss`（小深渊 = `109019266` 天平）。
只加了一行启动日志，把 coupon 行「已导入但没接线」这件事保持可见。

**两条不变量在这个新表上同样成立**（这是敢写解码器的理由，导入器逐条硬校验）：
- `[fixed drop table]` maze0 十条权重 = `268667+200000+2000+272000+200000+36000+16000+1500+3333+500 = 1,000,000`；maze1 八条 = 同样恰好 1,000,000；
- `[additional drop table]` 的 `[select prob]` `985200+14800 = 1,000,000`，且各自 drop list 也各自合计 1,000,000。

档次分布（maze0 fixed）：normal 47.07% · rare 27.2% · unique 20% · legendary 3.6% · epic 1.6% ·
**primeval 0.15%** · luck15 0.3333% · luck30 0.05%；additional 有 1.48% 走第二档
（unique 67.57% / legendary 20.27% / epic 10.14% / primeval 2.03%）。
**⇒「太初」（primeval）就在 fixed 表里，不需要隐藏 BOSS 也能到** —— 这是接入后最大的收益。

### 16.3 `[coupon drop table]`：导入、校验、**不抽**

五行（`obtain prob` / `drop prob` / `drop list`）：

```
0: 100000 /       0 / 空
1: 100000 /  600000 / normal 978570→10416150, 12500→10417544, 8930→10417543
2: 100000 /  400000 / normal 995140→10417545,  2780→10417546, 2080→10417547
3: 100000 /  330000 / normal 999583→10417552,   417→10417554
4:      0 / 1000000 / normal 1000000→10417571
```

**语义未确立 ⇒ 不写解码器**（沿用本项目对 `[hidden drop table]` / `[normal group index]` 的处置）：
能自圆其说的读法至少三种（两次独立抽取 / 按客户端难度一行 / 一次门槛 + 一次抽取），而它们的合计
（obtain 400000、drop 2330000）**都不落在百万空间上**——也就是说，这里**没有**像 `[drop list]` 那样
能证伪猜测的不变量。四份表里也只有这一张有 coupon 行（大深渊三张都没有）。
所以：解析 + 结构校验（两个概率必须 ≤ 1,000,000；非空 drop list 必须合计 1,000,000）+ 计入
`Templates()`（这样目录缺条目仍会在启动期被拦）+ **不进 `RolledTemplates()`**（不抽）。
`TestAttunementCouponTablesAreCarriedButNotRolled` 把这条钉住，并断言「概率合计不等于百万空间」——
一旦哪天数据变了、读法变得可推，测试会先失败提醒复查。

### 16.4 验收

| 检查 | 结果 |
|---|---|
| `go build ./...` / `go vet` | 干净 |
| `go test ./internal/...` | 15 包全绿 |
| `go test ./cmd/...` | 全绿（含 `cmd/wireprobe` 38.9s）|
| `go test -run Attunement ./internal/loot/` | 绿（脚本表断言已更新为 4 个副本；新增 coupon 钉歧义测试）|
| **新增** `cmd/wireprobe/endkeeper_reward_integration_test.go`（`ATTUNEMENT_REWARD_INTEGRATION=1`）| **PASS** |
| 大深渊原集成测试复跑 | **PASS**（`empties` 仍恰好 `[12]`，新表没引入新的空面）|

新集成测试的**判据不是硬编码指纹，而是现算的**：从本副本自己的 fixed/additional 包装出发，用礼盒目录
展开出全部可达内容物，再筛出「通用掉落池**证明性地**付不出」的那些
（`EquipmentCatalog.Basic()` 要求 `[attach type]=="[free]"` 且 `[rarity]<=2`；小深渊的包装内容物是
`equipment/character/common/primer/*` 的 `[attach type]=[trade]` 装备 ⇒ 必然被拒）。实测：

```
40 runs: >=3 rows per clear from the boss, 28 table-only row(s) over 16 distinct template(s)
[100401592 x10 100401596 x1 100401600 x1 100401601 x1 100401604 x3 100401605 x1 100401608 x2
 100401620 x1 100401624 x1 100401628 x1 100401632 x1 100401633 x1 100401634 x1 100401636 x1
 100401640 x1 100610044 x1]
```

即：**天平每场至少多付 3 行**，其中 28 行（16 种）是通用池不可能付出的装备；同时校验
「非 BOSS 的怪一行都不多」「两场里都没有包装盒落地面」。

`runtime/endkeeperprobe`（新，gitignore）打印名册，用来确认守卫真的成立：
`sourceBoss=109019266`、天平 `entity=0x100b rank=3 level=145 srcIndex=11`、副装置 `0x100c rank=0`。

**新 exe**：`bin/wireprobe-handoff-source.exe` = **`003341E921F0B241…`**（09-27 04:00）。
⚠️ 启动器不自动重编，实机前请确认哈希。

### 16.5 还没做（按业主给的顺序，下一个就是它）

1. **`[coupon drop table]` 的读法**（见 16.3）—— 需要一份能证伪的观测，或业主给出玩法口径。
2. **noti 2837 / 2839 / 2842**：本轮**没有动**，因为载荷结构仍在那面墙后（§12.7 结论未变：
   三个 noti 的处理函数全是纯构造器，只把 `(payload 指针 @obj+848, 包 ID @obj+40)` 存下来，
   字段偏移只存在于**惰性访问器**里，而访问器就是 VM 内建 `getPrimerGrade`(2130) / `getOathGrade`(2131)）。
   两条可走的路（**都不该靠猜**）：
   - **① 让客户端自己说（便宜、且是 L0′ 权威）**：实机起服后，在游戏里打开**誓约 / 引子界面**
     （`oathAndPrimer` / `PrimerCollectionWindow` / `SilentTruthUpgrade_Main`），客户端会发
     `cmd 2382`（OATH_SYSTEM_INFO）/ `2405`（REQUEST_SEASON_LEVEL_OATH）/ `2420` / `2422`（UPGRADE_PRIMER）。
     服务端**已经把未实现命令的请求体采样记进 `events.jsonl`**（`retainRequestBody`，每连接每命令
     `BodySampleLimit` 份，**保留整帧**），所以只要把那一场的会话目录给回来，载荷几何就有了；
     这些 cmd 与 noti 同构，可据以推出 noti 结构。
   - **② IDA 找内建分派**：§12.3 说 2116–2142 族走**显式 `cmp/jz` 链**，于是「同时比较 0x852 与 0x853
     的函数」就是分派链，它的分支目标即访问器。脚本已写好 `runtime/ida_builtin.py`（扫 `cmp/sub`
     立即数、按函数归并、反编译同时命中两个 ID 的函数）。
     ⚠️ **当前跑不了：D 盘 0 字节可用**（`df` 报 200G 用满）。两个可回收项：`ida-work/updvar.id0/.id1/
     .id2/.nam/.til`（3.7 GB，可由 `updvar.i64` 重建，本项目 runbook 本来就要删它们）、以及
     **D 盘回收站 49 GB**（`Clear-RecycleBin -DriveLetter D -Force`）。清理之前请不要起服（pgdata 写不进去）。

---

## 17. 【2026-09-27 04:2x】客户端探针：把 `72` 的来源做成「可观测」，而不是继续猜

业主问：能不能反推 72，或者从客户端侧加探针。本轮两条都试了，结论如下。

### 17.1 先关掉一条路（别再走）：静态反推 2130/2131 已到尽头

- 假设过「内建族 2116–2142 由显式 `cmp/jz` 链分派」⇒ 全镜像扫 `cmp/sub` 立即数：
  **`0x852` 只命中 1 处**（`sub_147695320`，§12.4 已证是 `[duskyisland proof]` 别名空间巧合），
  **`0x853` 一处都没有** ⇒ 该形态**不存在**。
- 另一形态「数据里有 `{id → 函数指针}` 表」⇒ 扫全部段找 2130/2131 的 dword/qword：
  **561 处命中全在 `.text`**（都是指令编码），**0 处旁边 32 字节内有代码指针** ⇒ 该形态也**不存在**。
- ⇒ 「扫描立即数/表」这条路**关掉**。静态要继续只能换锚点（比如从 vtable 找访问器），
  或者干脆不做静态 —— 这正是**动态探针**更划算的原因。

### 17.2 客户端探针为什么可行：两件本轮钉死的事实

**（a）编译脚本是 `[u8 type][u32 value]` × 5 字节的平坦单元流。**
`Primer_Proc.act` 原始 10,465 字节 ÷ 5 = **2,093**，与读取器报的 `cells=2093` **恰好相等**
⇒ **单元 i 的值在 `i*5+1`**（i 从 0 起）。已复核：cell 2089 的值在偏移 10,446，其类型字节在 10,445 = `0x0A`(type 10)。
⇒ **改一个单元的值 = 同长度 4 字节写**，不需要写 `.act` 序列化器。

**（b）变量/成员引用是一个「全局 32 位名字哈希」，跨文件恒定。**
`c:primer_rarity_progress_max` 在 `endkeeperoforder.dgn` 里是 **28411851**，
在 `Primer_Proc.act` 的 `[ARG VAR]` 里**也是 28411851**；`c:primer_rarity_progress_now` = 28376493 亦然。
⇒ 把某个探针槽位**改指向另一个变量**，同样只是 4 字节写。
（`.dgn` 里名字是 type 6、值在相邻 type 10；`.act` 的 ARG VAR 直接是 type 10，两者共用同一哈希空间。）

本轮从 `.dgn` 的 `[create var]` 取出**完整 44 个变量名 → 哈希**表（`c:max_grade_now` = 28410845、
`c:nox_index` = 28457547、`c:nox_die` = 28204847、`c:is_oath_epic_loop` = 28392105、
`c:fake_symbol_rarity_now` = 185959041 …）。这就是探针的「词汇表」，可复用。

### 17.3 探针怎么建的（零行为变更，只换一个上报字段）

日志定义在 `Primer_Proc.act` cell 2074–2092：`[TEXT]`(2075) + `[ARG VAR]`(2076–2090) 里 13 个 `type=10` 名字。
**保留前 8 个（HP / Action / 两个 trigger / 四个 rarity），把最后 5 个 omen/safe_timer 槽位换掉**：

| cell | 原 | 换成 | 用途 |
|---|---|---|---|
| 2085 | `c:omen_drop_process_on` | **`c:max_grade_now`** | 与 `*_max` 在 `.dgn` 里**共用同一个默认值槽** ⇒ 72 的第一嫌疑人 |
| 2086 | `o:omen_drop_process_rarity` | **`c:nox_index`** | 召唤（奥尔泰尔）相关 |
| 2087 | `c:omen_drop_process_cnt_max` | **`c:is_oath_epic_loop`** | 状态机此刻停在哪个分支 |
| 2088 | `c:omen_drop_process_cnt_now` | **`c:fake_symbol_rarity_now`** | 另一路稀有度 |
| 2089 | `o:safe_timer_from_max` | **`c:nox_die`** | 召唤分支条件 |

`[TEXT]` 里的标签保持原文（`omen : …, time_from_max : …`）不变 ⇒ **按位置读**：
第 9..13 个数就是上表的探针值。读取通道现成：**`CMD2329` 文本已由服务端整段记进 `events.jsonl`**。

### 17.4 构建链（全部只读客户端，产物在 `runtime/probe/`）

```
# 1) 换 5 个哈希（同长度）
go run ./runtime/actprobe -source runtime/probe/live.inner.pvf \
    -set 2085=28410845,2086=28457547,2087=28392105,2088=185959041,2089=28204847 \
    -out runtime/probe/live.Primer_Proc.act
# 2) 装进内层归档（只写新文件）
go run ./cmd/pvfpatch -source runtime/probe/live.inner.pvf -entry <Primer_Proc.act 路径> \
    -expected-sha256 e9cd815bc0943bd0837f7c21d8abb035319f595b5219d6d6d25614ddc865f6a5 \
    -replacement runtime/probe/live.Primer_Proc.act -output runtime/probe/live.patched.inner.pvf
# 3) 回封装成客户端格式（沿用 rewrap_pvfNN 那套；本轮脚本 runtime/rewrap_probe.py）
D:/115us/tools/python/python.exe runtime/rewrap_probe.py \
    runtime/probe/live.patched.inner.pvf runtime/probe/Script.probe.pvf
```

**验证（三层都过）**：
1. 回读：`actcells -source runtime/probe/live.patched.inner.pvf` 报 2085–2089 = `c:max_grade_now / c:nox_index /
   c:is_oath_epic_loop / c:fake_symbol_rarity_now / c:nox_die` ✔
2. 封装自证：`native_wrapper_roundtrip: true`，73 段；`segments=73` ✔
3. 源不动：`PVF_ENTRY_COPY_WRITTEN … source_unchanged=true`；客户端目录**只读** ✔

**产物**：`runtime/probe/Script.probe.pvf`（761,766,380 B，inner `48d2fd3f…`，wrapped `3c2a84b0…`）。

### 17.5 ⚠️ 副产品：**实机客户端的 `Script.pvf` 与我们冻结的内层不是同一份**

为了「不丢客户端多出来的东西」，本轮先把**实机的** `client/Script.pvf` 解封装（`runtime/unwrap_live.py`），
并且**用往返自证**：`rewrap(unwrap(live)) == live` **逐字节相等** ✔（封装只加密每 0xA00000 段的头 0x2800 字节，所以可逆）。

| | 字节 |
|---|---|
| 实机 `client/Script.pvf` | **761,764,350** |
| 我们冻结的 `client-build/Script.inner.pvf` | **760,530,763** |
| 差 | **1,233,587** |

而且解封装后逐段比对：**第 1/2/3 段的明文完全一致**，只有第 0 段头 0x2800 里从偏移 28 起不同
（典型的「归档头里的文件数/偏移随尾部增长而变」）。⇒ 实机归档 = 我们那份 + **尾部多约 1.23 MB 内容**。
**这值得单独查**：我们的 catalog 都生成自冻结内层，实机若真多出内容，就有「服务端目录 ≠ 客户端目录」的隐患。
（本轮不追。）

### 17.6 要跑探针，只剩两步（都要业主点头）

```powershell
# 备份（761 MB，D 盘现有 49 GB 空闲）
copy D:\115us\client\Script.pvf D:\115us\server\work\dfo-lan\runtime\probe\Script.before-probe.pvf
# 安装
copy D:\115us\server\work\dfo-lan\runtime\probe\Script.probe.pvf D:\115us\client\Script.pvf
```
然后照常起服、打一场（把天平打到血底即可），把会话目录给我。回退 = 把备份拷回去。

**读法**：`events.jsonl` 里 `id=2329` 的 `plain_hex`，第 4–260 字节是 ASCII 文本；
`omen : A, B, C, D, time_from_max : E` 里 **A..E 分别就是 `max_grade_now / nox_index / is_oath_epic_loop /
fake_symbol_rarity_now / nox_die`**。若 A=72 ⇒ 天花板就是 `max_grade_now`（装备/进度侧），
若 A=0/40 ⇒ 72 来自别处，再按同一手法换下一批探针（`c:primer_die`、`c:die_delay`、`c:is_fake_end_leg_epic`、
`c:is_oath_primitive_loop` …）。

> 也可考虑反过来：把 `c:primer_rarity_progress_{now,max}` / `c:oath_rarity_progress_{now,max}` 四个槽位
> 换成探针，等于每次上报 5 个探针值（现有四个 rarity 已知恒为 71/72/44/72，短期信息量为零）。
---

## 18. 【2026-09-27 04:5x】探针读数 + 72 的机械后果（cell 级闭环）+ 判死判据改锚

### 18.1 探针读数（会话 `..._20260927_043833_637073_next37`，20 条 CMD2329）

安装探针后照常打了一场，客户端**没有起不来**，日志照常上报。同一个槽位、同一条文本格式，
标签保持原文，按位置读：

| 槽位 | 探针前 (031849) | 探针后 (043833) | 探针变量 | 判读 |
| --- | --- | --- | --- | --- |
| `omen : A` | 1 (`omen_on`) | **71** | `c:max_grade_now` | **不是天花板** |
| `omen : B` | 40 (`omen_rarity`) | **0** | `c:nox_index` | 状态机在第 0 条 loop |
| `omen : C` | 0 (`cnt_max`) | **1** | `c:is_oath_epic_loop` | 停在 epic 分支 |
| `omen : D` | 0 (`cnt_now`) | **40** | `c:fake_symbol_rarity_now` | 机制值域内的默认值 |
| `time_from_max : E` | 0 (`safe_timer`) | **0** | `c:nox_die` | 未死 |

`rarities` 四个数（71/72/44/72）一字未动 ⇒ 探针只改了指定的槽位，没有副作用。

**A = 71 是一个可预测值，而且预测命中了** —— 这就是探针有效性的独立认证：
`primer_00_normal_loop.act` cell 5–9 里有

```
[UPDATE VAR] check_max_grade
  c:max_grade_now = max(c:oath_rarity_progress_now, c:primer_rarity_progress_now)
[/UPDATE VAR]
```

即 `c:max_grade_now = max(44, 71) = 71`。探针读到的正是它（同一文件 cell 72 也用它做数组下标
`o:arrayAt(o:hp_limit, c:max_grade_now-40)`）。

### 18.2 天花板 72 的来源：`getPrimerGrade()` / `getOathGrade()`（cell 级直接证据）

`primer_proc.act` cell 0–10：

```
[UPDATE VAR] RARITY_CHECK_START
  c:primer_rarity_progress_now = 40
  c:oath_rarity_progress_now   = 40
  c:primer_rarity_progress_max = getPrimerGrade()   <- cell 7，引擎内建 2130
  c:oath_rarity_progress_max   = getOathGrade()     <- cell 9，引擎内建 2131
[/UPDATE VAR]
```

`.dgn` 的 `[create var]`（`contents/2026/endkeeperoforder/dungeon/endkeeperoforder.dgn` cell 285–301）
给的默认值全是同一个字面量 `40`：`c:primer_die=0`、`c:die_delay=?`、
**`c:max_grade_now` / `c:primer_rarity_progress_max` / `c:oath_rarity_progress_max` = `40`**、
`c:primer_rarity_progress_now` / `c:oath_rarity_progress_now` = `0`。

⇒ 服务端从来没有、也不需要「写」这个 72；它是**客户端内建 getter 的返回值**。
⚠️ 顺带**作废 §早前的「72 数据位置 = `oathsystemscript.cos [remain parameter]`」**：
那张表是 (点数 → `RemainParameter/N.etc`)，例如 `1800 -> RemainParameter/72.etc`，
其中的 72 只是**文件序号**（同表还有 1775→71、2725→109），与机关品级无关。
（`etc/115lvability2/oathsystemscript.cos` 的真实结构：`[base rarity section]`（5 档）、
`[seasonlevel oath item]`（赛季等级 1/30/60/80/100 → 誓约装备 100610094/100313750/100610095/100313751/100610096）、
`[rarity ui infos]`、16 个 `[group]`、`[remain parameter]`、`[oath reinforce]`。）

### 18.3 为什么 72 是**非法值**：三处几何同时被它越界

**(a) 动作表只有 8 项（下标 0..7）。** `scale_primer.mob` 的
`[create array var]`（cell 218–237）实测：

```
o:delay_time = [100, 900, 1100, 1300, 1800, 2300, 2300, 2300]   8 项 = 8 档
o:hp_limit   = [ 90,  80,   70,   60,   50,   40,   70,   50]   8 项 = 8 档
```

`primer_proc.act` 里 rainbow1 用 `o:arrayAt(o:hp_limit, 6)`、rainbow2 用 `7`（写死）；
`[etc action definition]`（`scale_primer.mob` cell 242–…）是 3 行 × 8 列的网格：
`Primer_00_Normal / 01_Rare / 02_Unique / 03_Legendery / 04_Epic / 05_Primitive / 06_Rainbow1 / 07_Rainbow2`
× {`_LOOP`, `_END`, `_FAKE_END`}。

**(b) 收尾动作选不中 ⇒ 机关永远不死。** `GO_END` 的行为体最后一句是
`[SET GROUP ACTION] end (c:primer_rarity_progress_max-40)`（`primer_proc.act` cell 617–618），
`primer_00_normal_end.act` 则在 `o:die_frame_normal` 帧上 `[DESTROY]` 自己、并把同坐标的
`109019280` 一起 `CHECKUP OBJECT destroy`。天花板 72 ⇒ 下标 **32** ⇒ 8 项表选不中 ⇒
**不放结束动画、不 DESTROY**；而且 `[NOT][IS GROUP ACTION] end` 这个再入闸门永不置位，
于是「每挨一次打就重发一份日志」——实测 20 条，全部同一帧内容（与 43:833 场吻合）。

**(c) 唯一死亡闸门恒假。** `END_TRIGGER`（`primer_proc.act` cell 415–441，默认 `[ENABLE] OFF`）要求
`c:primer_rarity_progress_now >= c:primer_rarity_progress_max` **且**
`c:oath_rarity_progress_now >= c:oath_rarity_progress_max` ⇒ `71>=72` ✗、`44>=72` ✗ ⇒ `DIE_TRIGGER` 永不开。
`o:hp_limit_final = o:arrayAt(o:hp_limit, max(oath_max,primer_max)-40)`（cell 134）同样是下标 32 ⇒
越界（数组 8 项）；cell 152–170 的 `set_final_value_*` 三分支（`<70` / `==70` / `==71`）对 72 全不命中，
`o:hp_limit_final` 停在 `[create var]` 的默认 10。
`o:safe_timer_from_max`（唯一在「两侧都满级」时才自增的计时器，cell 2008–2024）因此**恒为 0** ——
与实机 `time_from_max : 0` 完全吻合。

**(d) 状态机永远走第一条分支 ⇒ 隐藏 BOSS 是死代码。** `primer_00_normal_loop.act` cell 160–172：
首分支是 `primer_max > 69 -> primer_up_to_rainbow1 + GO_RAINBOW1`；72>69 恒真 ⇒
`ELSE IF primer_now == primer_max && oath_now < oath_max`（唯一通往 `summon_orthaire` 的分支，
cell 243–250，条件是 `oath_now==44 && c:is_oath_epic_loop==1 && c:nox_die==0`）永不求值。
**注意**：探针 C 已经证明客户端正停在 `is_oath_epic_loop == 1`，也就是**离召唤只差这一个条件**。

### 18.4 ⚠️ 本轮实机：没有通关（兜底判据没满足），且发现一处新现象

| 现象 | 数据 |
| --- | --- |
| 8 条 `scale_status` 全是 `hp=94730 peak_hp=94730 rate=100` | 本次运行**先看到的就是地板**（前一场 031849 首样本是满血 4736540）⇒「峰值 5%」判据永不成立 |
| 没有 `scale_death_forced` / 没有 `dungeon_clear_*` | 天平没死、没结算 |
| C2S 39 只有 11 条，全是杂兵（entity `0x1000..0x100A`） | **客户端自始至终没有上报过天平的死亡** ⇒ 服务端兜底是唯一入口 |
| `dungeon_request_rejected{id:2329, reason:"checksum failed"}` ×12 | **本轮新现象**：20 条 CMD2329 里只有 8 条通过 `wire.Checksum`，另外 12 条被丢（且没有保留 body）。跨会话统计：此前各场 rej 均为 0。合并 (b) 的解释：客户端在 1.3 s 内连发 20 条同内容日志，其中一部分自带不一致的校验字节。**注意这会让「按样本数判断」更不可靠** |

### 18.5 判死判据改锚（`cmd/wireprobe/scale_death.go`）

复用 §18.3(b) 的结构事实：`primer_proc.act` cell 2025–2068 里，那段
`[SERVER LOG MSG]`（即 CMD2329）与 `[DO BEHAVIOR] GO_END` **在同一个分支**里
（条件是 `[ON DAMAGE]` + `o:safe_timer>=60` + 不在 `end` 组 + `[CHECKED WHICH ME] > 0`）。

⇒ **收到一条 CMD2329，就等价于「客户端刚刚决定收尾」**，这是本机关唯一的击杀入口；
服务端据此判死与设计意图一致，且**不依赖峰值**。改动：

* 判据一（主）：`scaleStatusReport.EndTrigger`（文本里 `triggers : <end>, <die>` 的第一位）
  为真 ⇒ 判死，事件里 `reason="client_end_trigger"`。
* 判据二（兜底）：`hp <= 峰值 × 5%`，`reason="hp_floor"` —— 保留它是因为
  **`[ON DAMAGE] o:getHpRate() <= 2` 那条收尾路径（cell 463–495）不发日志**，只能靠血量认。
* 开关名不变（`-scale-death-from-hp` / `DFO_SCALE_DEATH_FROM_HP`），避免动 `launch_local.py` 与
  `prune_unsupported`；注释里写清它现在不止管血量。
* 新增测试：`TestParseScaleEndTrigger`、`TestScaleStatusKillsOnClientEndWithoutAnyPeak`
  （**就是本轮的回归**：无峰值也要判死、两台装置一起退场、重复上报不再判死）、
  `TestScaleStatusNeedsEvidenceWithoutEndTrigger`（只有日志形状、没有收尾证据时不许判死）。

### 18.6 探针已回退 & 下一步

* 客户端 `Script.pvf` 已拷回原版：`5681103c…`（= 安装前逐字节相同），探针版 `3c2a84b0…` 仍留在
  `runtime/probe/Script.probe.pvf`。
* **下一步只剩一件事值钱：定死 `getPrimerGrade()/getOathGrade()` 该返回什么。** 两把钥匙：
  1. **一次性判定实验（便宜、结论最硬）**：把 `primer_proc.act` cell 7/9 的两个 getter 调用
     换成字面量（如 cell 7 → `71`、cell 9 → `44`），再打一场：若天平**自己**跑完结束动画、
     客户端发出 C2S 39、隐藏 BOSS 出现，则 18.3 的整条链被实机证实，也同时给出「服务端该送什么」。
     注意要合法值：`[SET GROUP ACTION] end` 的下标是 `primer_max-40`，所以**只有 40..45 走这条式子**，
     70/71 有专门分支（用 6/7）——**72 落在 ELSE 里用 32**。
  2. **找那个「谁在喂 getter」的包**：`getEOOPartyOmenState(t:seatIndex())`（cell 386）是同一族的
     队伍级状态，说明这块状态本来就是服务端送下来的；noti 2837/2839/2842 仍是首要嫌疑，
     缺的依旧是载荷几何（IDA 那面墙）。
### 18.7 【2026-09-27 05:1x】判定实验已装上：把 getter 换成合法字面量

§18.6 说的那件「只剩一件事值钱」的事已经落地。补丁点经复核**唯一**：
`getPrimerGrade()`(28460867) / `getOathGrade()`(28460901) 只出现在 `primer_proc.act` cell 7 / cell 9，
`RARITY_CHECK_START` 这个命名块也只在该文件定义与调用（8 个 `*_start.act` / `*_loop.act` 里都没有）⇒ 改这两格覆盖全部初始化路径。

| 格 | 原值 | 新值 | 含义 |
| --- | --- | --- | --- |
| cell 7 | `28460867` (`getPrimerGrade()`) | **`2998467`** | 字面量 **71**（= 机制值域最高档 Rainbow2）|
| cell 9 | `28460901` (`getOathGrade()`) | **`1341115`** | 字面量 **45**（= Primitive，比实测 `oath_now=44` 大一档）|

选这两个值的理由：**都必须落在 8 档值域里**（`[SET GROUP ACTION] end` 的下标是 `primer_max-40`，
只有 40..45 走这条式子、70/71 有专门分支用 6/7，72 才落进 ELSE 用 32）。两个字面量哈希**在原文件里本来就有**
（`71` 出现在 cell 181/244/487，`45` 出现在 cell 664/671/675），所以 VM 的表达式表能直接解析，不需要新造。

命令（全部只读客户端；`-expected-sha256` 就是原 entry 的 sha，由 actprobe 顺手打印）：

```bash
P=contents/2026/endkeeperoforder/monster/named/scale_primer/action/Primer_Proc.act
go run ./runtime/actprobe -source runtime/probe/live.inner.pvf -entry "$P" \
    -set "7=2998467,9=1341115" -out runtime/probe/primer_proc.grade.act
# -> entry sha e9cd815b…, patched sha c2c89fb2…
go run ./cmd/pvfpatch -source runtime/probe/live.inner.pvf -entry "$P" \
    -expected-sha256 e9cd815bc0943bd0837f7c21d8abb035319f595b5219d6d6d25614ddc865f6a5 \
    -replacement runtime/probe/primer_proc.grade.act -output runtime/probe/live.grade.inner.pvf
# -> PVF_ENTRY_COPY_WRITTEN source_unchanged=true
/d/115us/tools/python/python.exe runtime/rewrap_probe.py \
    runtime/probe/live.grade.inner.pvf runtime/probe/Script.grade.pvf
# -> native_wrapper_roundtrip true, segments 73, original_files_modified false
```

三层校验：① `actcells -source runtime/probe/live.grade.inner.pvf -path "$P" -from 0 -to 11`
回读出 `cell 7 str=71` / `cell 9 str=45` ✔；② 回封装自证 ✔；③ 源未动 ✔。
产物 **`runtime/probe/Script.grade.pvf` = `2390a160…`**（761,766,372 B），**已装到 `client/Script.pvf`**；
原版备份 `runtime/probe/Script.before-probe.pvf` = **`5681103c…`**（回退就是把它拷回去）。

**⚠️ 本轮故意关掉了服务端兜底**：`scripts/launch_local.py` 里
`env["DFO_SCALE_DEATH_FROM_HP"]` 由 `"1"` 改成 **`"0"`**。理由是让观测**无损判别**——
两套判死同时开着的话，服务端会抢在客户端动画之前把机关判死，就再也看不到客户端自己发的那条 C2S 39。

**看什么**（判据是硬的，不靠感觉）：

| 现象 | 结论 |
| --- | --- |
| 出现 `client_frame id=39` 且 payload 前 4 字节 entity = **`0b100000`**（= 0x100B = 4107，天平本体） | **客户端自己杀了它** ⇒ 72 就是唯一阻塞，§18.3 整条链实机成立 |
| 随后 `map_clear_quest_triggers` / `boss_check_confirmed` / `dungeon_clear_*` | 通关 + 结算按既有路径走完 |
| 还出现 `109019264`（奥尔泰尔）的创建或死亡 | 隐藏 BOSS 也活了（但它需要 `primer_max <= 69`，本轮给的是 71 ⇒ **预期看不到**，那是下一轮的事）|
| 仍然没有任何 0x100B 的 C2S 39 | 72 不是唯一阻塞（或我的分支读法有误），下一轮到别的变量上去 |

**⚠️ 注意**：`0x100B` 这个 entity 只在**第一间房**成立；同一会话若连刷第二场，entity 会重排（都从 `0x1000` 起）
⇒ 统计要按时间窗口切片。判死之后再把开关还原成 `"1"`。
### 18.8 【2026-09-27 05:1x】判定实验结果：**72 是唯一阻塞，实机证实**

会话 `..._20260927_051224_953188_next37`（**服务端兜底全程关着**，所以下面每一条都只可能是客户端干的）：

```
21:13:07-08  C2S39 entity=0x1000..0x100A  killer=11      11 只杂兵（既有路径）
21:13:35.905 C2S39 entity=0x100c(4108) killer=11          Scale_oath 自己退场
21:13:36.111 C2S39 entity=0x100b(4107) killer=11          Scale_primer —— 天平本体
21:13:36.111 monster_death_confirmed(38) entity=0x100b, count=6    天平付了 6 行
21:13:36.111 map_clear_quest_triggers(291) / boss_check_confirmed(115)=01010b10 / dungeon_clear_enabled(31)
21:13:36.160 dungeon_play_result(34) / clear_experience(37)=3,151,739 / dungeon_clear_reward(35) / eplp_rechallenge(261)=09
```

**⚠️ 这一场一条 `CMD2329` 都没有**（C2S 表里只有 2127/38/39/283/…）。因为那条日志与 `GO_END` 同分支、带 `o:safe_timer>=60` 前提，
而这场是天平出现后 ~28 秒就打完了 ⇒ 走的是**另一条**收尾路径 `[ON DAMAGE] o:getHpRate() <= 2`（`primer_proc.act` cell 463–495）。
`primer_max = 71` 落进它的 `== 71 → GO_END_RAINBOW2` 明支 ⇒ `[SET GROUP ACTION] end 7`（**合法下标**）⇒
`Primer_07_Rainbow2_End` 播完 → `o:die_frame_normal` 帧 `[DESTROY]` → 客户端自己发 C2S 39。

**天平付的 6 行**（`monster_death_confirmed(38)`，entity `0x100B`，count=6，每条 `Object u32 + Item 181B + Aux u32 + Sent u16 + Owner u16`，尾 `00 00 ff 00`）：

| # | Object | template | 身份（`configs/items.index.json` + `runtime/equipinfo`）|
| --- | --- | --- | --- |
| 1 | 0x100D | **0** × 3003 | 金币 3003（`template 0 + amount` = 金币）|
| 2 | 0x100E | **108030762** | `equipment/character/priest/weapon/scythe`，`[free]`、grade 107、**rarity 2** |
| 3 | 0x100F | **10362432** ×2 | `stackable/10362001/10362432.stk`，`[material]` |
| 4 | 0x1010 | **100401612** | `equipment/character/common/primer`，**`[primer]`、`[trade]`、grade 116、rarity 3** ← **关键** |
| 5 | 0x1011 | **10362481** ×1 | `stackable/10362001/10362481.stk`，`[virtual]` |
| 6 | 0x1012 | **1** ×1 | `stackable/coin.stk`（金币卡第 2 组）|

第 4 行是**证明性**的：通用掉落池 `Basic()` 要求 `[attach type]=="[free]"` **且 `[rarity]<=2`**，
而这件 `[primer]` 装备是 `[trade]` + `rarity 3` ⇒ **通用池物理上付不出** ⇒ 它只能来自专属奖励表那条线。
`dungeon_clear_reward(35)` 依旧只有经验（`baseExp=3,151,739`、`scoreExp=0`）+ 空的卡牌组 ⇒ **物品全在地面行上，符合设计。**

**隐藏 BOSS 确实出现了**（业主实机真值，权威）。这**更正**了 §18.3(d) 的「`summon_orthaire` 是死代码」：
那个分支在**高档位循环**里各有一份（`primer_02_unique` / `03_legendery` / `04_epic` / `06_rainbow1` / `07_rainbow2`
的 `*_loop.act` 都有，实测 `00/01/05` 没有），条件与在 `00_normal` 读到的一致
（`oath_now==44 && c:is_oath_epic_loop==1 && c:nox_die==0`）。而 `primer_max > 69 → GO_RAINBOW1` 那条**首分支只存在于 `00_normal_loop`**
（`07_rainbow2_loop` 里连 `69` 这个字面量都没有）。**真正的阻塞是同一个等式**：
召唤分支被包在 `primer_now == primer_max && oath_now < oath_max` 里，天花板 72 让这个等式**处处不成立** ⇒ 永远求值不到它。
补丁把 `oath_max` 给成 45（比实测 `oath_now=44` 大一档）也让 `oath_now < oath_max` 成立 ⇒
配合 `oath_now==44 && is_oath_epic_loop==1`（探针 C 早就证明客户端正停在这格）⇒ **召唤成立**。

**结论：72 就是唯一阻塞。** 一个越界的 getter 返回值同时堵死了三件事 —— 收尾动作下标、`END_TRIGGER` 的两个 `now>=max`、
以及通往隐藏 BOSS 的 `now==max`。改成合法值（71/45）后，**收尾动画、DESTROY、C2S 39、通关判定、专属奖励表全部按设计走通**。

**已回退**：`client/Script.pvf` 拷回原版 `5681103c…`；`scripts/launch_local.py` 的
`DFO_SCALE_DEATH_FROM_HP` 还原为 `"1"`。要再跑这个实验：`cp runtime/probe/Script.grade.pvf D:/115us/client/Script.pvf`
（并把开关临时置 `"0"` 以保持可判别）。

**下一步**：服务端侧的正解仍是「谁喂 `getPrimerGrade()/getOathGrade()`」—— 也就是 noti 2837/2839/2842 的载荷几何。
在那之前，客户端补丁是让内容**按设计**工作的唯一手段（服务端兜底只能保证「能通关」，给不了隐藏 BOSS 与结算档位）。
---

## 19. 【2026-09-27 05:2x】根治路线：还差什么、谁来取证

### 19.1 已知与未知（一句话版）

**已知**：`c:*_rarity_progress_max` 由客户端内建 `getPrimerGrade()`(2130) / `getOathGrade()`(2131) 给出（`primer_proc.act` cell 7/9），
本机恒返回 **72**；值域是 8 个档位码 `{40,41,42,43,44,45,70,71}`；喂合法值 ⇒ 收尾动画 / `DESTROY` / C2S 39 / 通关 / 专属奖励表全部按设计走通（§18.8 实机证实）。

**未知（= 唯一缺口）**：**谁在喂这两个 getter、字段长什么样。**
它不在任何脚本里（脚本只调用、不推导）；也不在静态数据里（`.dgn` 默认是 40；镜像里没有 `{id→函数指针}` 表）。所以只剩两条可能：
**(i) 某个服务端包**；**(ii) 客户端本地持久状态**（赛季等级 / 誓约装备 / 引子收藏）。两条都能被下面三轨合围。

### 19.2 三轨合围（按成本排序）

**Track 0 —— 让客户端自己说（几乎零成本，先做这条）**

在**城里**（不必进副本）分别打开：誓约 / 引子界面、引子收藏（PrimerCollection）界面、赛季等级界面，然后退出游戏。
客户端会为这些面板发 `cmd 2382 / 2405 / 2420 / 2422 …`，而服务端**本来就在采样未实现命令的整个请求体**
（`retainRequestBody`，见 `request_scope.go`；上限 8 条/命令）⇒ 回传会话目录即可拿到**请求体几何**。
这些请求通常与 noti 同构，等于免费拿到字段布局。

同时有一个**零成本、可能直接定案**的观测：**界面里显示的是什么？** 若 UI 自己显示「等级 72」或某个明显异常值 ⇒ 数据源基本锁定；
若 UI 显示正常而 getter 返回 72 ⇒ getter 读的是另一处状态，转而按 Track 1 做。

**Track 1 —— 差分注入 + 活体读数（决定性，要打一场但不用打完）**

两处改造，一是「可控输入」，二是「可读输出」：

* **客户端探针 v3**（基于 **stock** 内层 `runtime/probe/live.inner.pvf`，**不做**等级补丁）：
  1. 把 log 触发里的 `o:safe_timer >= 60` 改成 `>= 0`（`primer_proc.act` 里一个整数字面量）⇒ **碰一下天平就有一行读数**，不必打满；
  2. 把日志的两个 ARG VAR 槽换成 **`getPrimerGrade()`(28460867) / `getOathGrade()`(28460901) 本身**。
     这一步是关键：现在槽里打的是 `c:*_max`，那是**开场 `RARITY_CHECK_START` 读一次后缓存**的值；
     换成 getter 调用后，**每一行都是实时读数** ⇒ 注入任何状态后，下一次挨打立刻看到 getter 输出变了没有。
* **服务端注入器**（新增诊断开关，不动既有路径）：能按参数发**任意 noti（id + payload hex）**，并按需/定时重发。
  用它去试三件候选：noti **2837** `ENDKEEPER_OF_ORDER_INFO` / **2839** `OATH_SYSTEM_INFO` / **2842** `PRIMER_COLLECTION`，
  payload 先取「全 0 + 在偏移 k 处放 71」扫 `k = 0..64`（每换一个 k，碰一下天平）。
  **判据**：某 `(id, k)` 让实时 getter 从 72 变成 71 ⇒ **载体与几何同时到手**；之后换哨兵值（如 44/45/70）把真实字段逐个钉死。

这条路的好处是**完全不需要预先知道几何**：客户端自己就是 oracle。

**Track 2 —— IDA 反查访问器（我自己做，可与其他轨并行）**

D 盘现有 40 GB，IDA 路线重新可跑。已知事实足够开刀：noti 2837/2839/2842 的处理函数把
`(payload 指针 @obj+848, 包 ID @obj+40)` 存下来（§12.7）⇒ **扫「所有读 `obj+848` 的函数」就能找到那批访问器**；
其中返回 grade 的那个，字段偏移直接读出来。同一手法还可以去追
`getEOOPartyOmenState(t:seatIndex())` —— **它在本机是「能用」的**（收尾阶段确实进入了），
如果它也是服务端喂的，那说明**同类通路已经存在**，照抄即可。

### 19.3 收敛与验收

1. Track 0/1/2 任一命中 ⇒ 得到「载体 + 几何」；服务端实现该包（**stock 客户端、兜底开关关掉**）。
2. 验收判据（全部要，缺一不可）：
   * `CMD2329` 文本里 `rarities` 第二个数（`c:primer_rarity_progress_max`）变成**合法值**（不再是 72）；
   * 天平**未经服务端干预**自己发出 `C2S 39`（entity 0x100B）；
   * 隐藏 BOSS `109019264` 出现；
   * 通关 + 天平付 ≥3 行专属奖励（含 `[primer]`/`[trade]` 那一类）。
3. 收口：达到 2 之后，`scale_death.go` 的兜底退化为「**双保险**」（默认关或保留由业主定），
   客户端补丁不再需要 ⇒ 客户端回到**完全 stock**，取证权威性（L0）恢复。

### 19.4 顺带修掉的一个真 bug（本轮已改）

`observedGameRequest` 里**没有 `2329`**，而 `retainRequestBody` 的采样上限是 **8 条/命令**，且
`verified` 只在「这条被采样保留」时才计算（`main.go` 1238–1251）⇒ **第 9 条之后的 CMD2329 全部落到
「`verified == false`」分支，被 `dungeonRequest` 拒掉，而拒绝理由写成 `checksum failed`**。
所以 §18.4 记的「12/20 校验失败」**不是校验失败**，是采样上限的副作用 —— 而它恰好让「血量峰值」判据失效。
已把 `2329` 加进 `observedGameRequest`（每次请求都解密校验，不受采样限制）+ 回归测试
（`TestMonsterHistoryLogIsExemptFromTheBodySampleCap`）。
### 19.5 【2026-09-27 05:3x】探针 v3：一击出实时读数（业主提议「装备 oath 装备 / 星韵石」那轮用）

**先排除的一条**：业主打开**誓约界面 + 装备库**整场**没有产生任何上行包**
（会话 `..._2026052952_241431`：21:30:19–21:30:21 的入场握手之后，到 21:30:52 只有 `2127` 走动遥测；
`unimplemented_sample` 里也没有 `2382/2405/2420/2422`）⇒ **面板数据是客户端本地的**，
「让客户端自己说请求体」这条（Track 0）在此处拿不到东西；`client/` 下也没有任何会话期被改写的本地状态文件
（近 90 分钟内只有 `LagLog.txt`）。
⇒ 焦点回到「**本地状态里到底哪一项喂了 getter**」，而业主提议的差异法（装备 oath 装备 / 星韵石）正是最快的判据。

**探针 v3 的两处改动**（`primer_proc.act`，仍基于 stock 内层；entry sha `e9cd815b…` → patched `7bdb26ab…`）：

| 格 | 原值 | 新值 | 作用 |
| --- | --- | --- | --- |
| 2030 | `1443133`（字面量 **60**）| `32673`（字面量 **0**）| `[ON DAMAGE] o:safe_timer >= 0` ⇒ **碰一下天平就有一条 CMD2329**，不用等 60 秒 |
| 2085 | `c:omen_drop_process_on` | **`getPrimerGrade()`**(28460867) | 实时 |
| 2086 | `o:omen_drop_process_rarity` | **`getOathGrade()`**(28460901) | 实时 |
| 2087 | `c:omen_drop_process_cnt_max` | `c:primer_rarity_progress_max`(28411851) | **开场缓存值**，作对照 |
| 2088 | `c:omen_drop_process_cnt_now` | `c:max_grade_now`(28410845) | |
| 2089 | `o:safe_timer_from_max` | `c:nox_index`(28457547) | |

读法（日志尾段，标签保持原文、**按位置读**）：

```
..., omen : A, B, C, D, time_from_max : E
A = getPrimerGrade() 实时   B = getOathGrade() 实时
C = c:primer_rarity_progress_max（缓存）  D = c:max_grade_now   E = c:nox_index
```

**⚠️ 副作用（故意）**：`log` 与 `GO_END` 同分支，门槛降到 0 之后**天平挨第一下就会开始收尾**。
所以这一版只用来读数、不用来玩；**每试一种装备搭配就重进一次副本**。
读数不受影响：`log` 在 GO_END **之前**调用，即使 `primer_max` 仍是 72（那条 ELSE 的 `GO_END` 选不中动作）也照样发得出来。

产物 `runtime/probe/Script.probe3.pvf` = **`00965fc1554d…`**（761,766,371 B），**已装到 `client/Script.pvf`**；
原版备份 `Script.before-probe.pvf` = `5681103c…`。服务端兜底**再次故意置 `"0"`**（要纯客户端观测）。
服务端 exe `f1896864…` 已含 §19.4 的采样豁免修复 ⇒ **这一场的每一条 CMD2329 都会被保留**（不再只有 8 条）。

判据：若某搭配让 **A（实时 `getPrimerGrade()`）从 72 变成合法值** ⇒ driver 就是它，直接定案；
若 A 恒 72 ⇒ getter 不读装备，转 Track 1 的注入器或 Track 2 的 IDA 反查。
### 19.6 【2026-09-27 05:3x】第一轮读数 + 两个必须记下的结论

**读数**（会话 `..._2026052952_241431`，21:39:06，tpl=109019266，满血 4,736,540）：

```
HP : 4736540.00, Action : 20014, triggers : 1, 0, rarities : 71, 72, 44, 72,
omen : 1, 40, 0, 0, time_from_max : 0
```

**⚠️ 但这条是 stock 脚本发的，不是探针 v3** —— 槽位映射还是旧的（`omen : 1,40,0,0` = `omen_drop_process_on/rarity/cnt_max/cnt_now`）。
原因：`Script.pvf` **只在游戏启动时读一次**，而那一刻客户端是 05:29:52 起的、探针是 05:36 才装的
（同一会话目录 `...052952_241431` 的 events.jsonl 从 99,329 B 长到 160,349 B 就是证据：会话没换过）。
⇒ **换客户端 PVF 之后必须重启游戏**，本轮记一条。

**结论 1（来自业主截图，很有价值）**：誓约栏里**十个槽全是空的**（其中一个带锁），也就是**没有装备任何誓约装备**，
而这个状态下 `primer_max` 依然是 **72**。空槽没有品级可言 ⇒ **72 不是「已装备誓约装备的品级」**
（否则空槽应给 40 或 0）。概率上移到「**某个等级／收藏／全局状态**」而非「单件物品」。
业主提议的差异法仍然值得做（便宜，而且能**干净地排除**这条假设），但预期不变。

**结论 2（运营级坑，已修）**：**服务端兜底关掉 + 客户端 `DESTROY` 跑不起来 ⇒ 玩家会卡在副本里出不去** ——
天平不死 ⇒ 没通关 ⇒ 「返回城镇」灰掉（业主截图 2 就是这个），只能关游戏。
所以本轮把 `scripts/launch_local.py` 的 `DFO_SCALE_DEATH_FROM_HP` **还原为 `"1"`**：
读数不受影响（`log` 先发），但服务端会把天平判死 ⇒ **能通关 ⇒ 能回城换装备 ⇒ 才有快循环**。
（教训：**任何「关掉兜底」的读数实验都必须先确认玩家退得出去**。）

**备用读数通道（不用打补丁）**：stock 客户端的 `rarities : <now>, <max>, …` 里的 `<max>` 就是
`c:primer_rarity_progress_max`，它在 `RARITY_CHECK_START`（**天平开场**）读一次 ⇒ **本来就反映装备后的状态**。
所以若探针 v3 出问题，退回「stock + 让战斗撑过 60 秒再补一刀」也能拿到同一个判据
（stock 的日志前置条件是 `o:safe_timer >= 60`）。

**下一轮协议（探针 v3 生效后）**：重启游戏 → 进小深渊 → **碰天平一下**（读数即到，服务端随后判死 → 通关）
→ 返回城镇 → 换装备 → 再进 → 再碰一下。每轮只看 `omen : A`（实时 `getPrimerGrade()`）。
### 19.7 【2026-09-27 05:4x】探针 v3 首批读数 + 「行为变了」的真因是服务端去重表泄漏

**读数**（会话 `..._2026054130_969393`，探针 v3 生效；`omen : A, B, C, D` 按位置读）：

```
21:42:26.942  HP : 5207935  triggers : 1, 0  rarities : 70, 72, 40, 72  omen : 72, 72, 72, 70
21:42:59.736  HP :  94730  triggers : 1, 0  rarities : 71, 72, 40, 72  omen : 72, 72, 72, 71
21:43:00.242  HP :  94730  triggers : 1, 0  rarities : 71, 72, 40, 72  omen : 72, 72, 72, 71
```

* **A = 实时 `getPrimerGrade()` = 72**，**B = 实时 `getOathGrade()` = 72**（装备了星韵石的那一场同样如此）
  ⇒ **星韵石不影响 getter**。配合 §19.6 的「誓约栏全空也是 72」⇒ **装备假设被排除**（空 / 星韵石两种状态读数逐字相同）。
* 探针自身**内部一致性再次成立**：`D = c:max_grade_now = 70 → 71`
  = `max(oath_now=40, primer_now=70→71)`（同一行的 `rarities` 就是 40 和 70/71）⇒ 探针读数可信。
* **附带收获**：探针 v3 把日志门槛降到 0 之后，读数出现在**第一次挨打**，于是我们第一次看到**开场态**：
  `primer_now=70 / oath_now=40`。此前 stock 的读数都在打了很久之后，是 `71 / 44` —— 说明这两个 now
  是**在战斗中爬升**的（与被喂食的征兆流程一致），不是恒定值。

**「装备星韵石后行为变了」的真因 = 服务端去重表泄漏（我的 bug）**：

* 第 2 轮的死亡来自 **服务端**：`21:42:26.943 scale_death_forced{reason:"client_end_trigger"}` →
  `monster_death_confirmed` → `map_clear_quest_triggers` → `boss_check_confirmed` → 通关。这就是业主看到的
  「碰一下天平就死了」（探针把日志门槛降到 0 ⇒ 第一条日志在第一次挨打时就到，主判据立刻成立）。
* 第 3 轮（同一会话重进副本）从 `21:42:58` 起连续十几次 `hp=94730 rate=1.43%`，**却再没有任何 `scale_death_forced`**。
* 原因：`worldSession.scaleForced`（以及 `scaleHP` 峰值表）**只按 entity 记、从不清**，而同一会话里重进副本
  entity 会**从 `0x1000` 重新分配** ⇒ 第二场起 `scaleForced[4107]` 仍是 `true` ⇒ **永远不再判死**。
* 已修：新增 `worldSession.scaleRun`，在 `scaleStatus` 开头比较 `activeDungeon.RunID`，**换场就整表清空**
  （峰值表也要清：同一模板在不同难度/房间血量不同，5207935 与 4736540 都出现过）。回归测试
  `TestScaleStatusRearmsOnANewDungeonRun`。

**教训**：「关掉兜底」做实验之前必须先确认玩家退得出去（§19.6）；**「多场复用同一会话」的功能必须按 RunID 重置状态**
（entity 空间按场重排是既有事实，§2）。
### 19.8 【2026-09-27 05:5x】`etc/115lvability2/` 的实形 —— 「全局状态」有了具体候选

业主的判断（72 更像「115 级解锁的某种**全局状态**」而非单件物品）与证据一致（空栏 72 / 星韵石 72）。
于是把整套 **115 级誓约系统**的数据面摸了一遍（`etc/115lvability2/`，**447 个条目**）：

| 目录 | 数量 | 内容 |
| --- | --- | --- |
| `equipmentoath/remainparameter/` | **140** | `RemainParameter/N.etc`；`oathsystemscript.cos [remain parameter]` 里是 (点数 → 该文件) 的对照，如 `1800 -> 72.etc`、`1775 -> 71.etc` |
| `equipmentoath/addparameter/` | 48 | 附加参数 |
| `equipmentoath/<162xx_名字>/<1|2|3>/<rarity>.etc` | ~250 | **12 种誓约 × 3 阶 × 5 档**：`16201_shadow` / `16202_fairy` / `16203_gold` / `16204_dragon` / `16205_purify` / `16206_serendipity` / `16207_limitless` / `16208_nature` / `16209_valkyrie` / … 档位名 = `rare / unique / legendary / epic / primitive` |

`oathsystemscript.cos` 的段落（实读）：`[base rarity section]`（5 档 → `BaseStat/*_stat.etc`）、
**`[seasonlevel oath item]`（赛季等级 1 / 30 / 60 / 80 / 100 → 100610094 / 100313750 / 100610095 / 100313751 / 100610096）**、
`[rarity ui infos]`、**16 × `[group]`（誓约节点树）**、`[remain parameter]`、`[oath reinforce]`。

关键结构：

* **誓约是「节点树 + 等级 + 点数」**：每个 `[oath info]` 有 `[index] [name] [type] small… [max level] [point by level]`，
  例 `oath_smallnod_3` 的 `[max level] 100`、`[point by level] -1 25` ⇒ **这套系统里确实存在「等级 / 点数」这种全局量**。
* `[oath reinforce]` 有 `[max level available] 4` 与逐级的 `[required item]`（如 `10401346 100` / `10421371 40` / `0 500000`）。
* 档位码域上的对应关系清楚了：`40 common / 41 rare / 42 unique / 43 legendary / 44 epic / 45 primitive`，
  另有 `70 rainbow1 / 71 rainbow2`；**`45` 正是脚本里 `oath_up_to_primitive` 的目标值** ⇒ 脚本与数据面自洽。
* ⚠️ 但 **72 仍不等于上面任何一个档位码**，也不等于 `[remain parameter]` 的阈值本身
  —— 所以「誓约节点等级 / 点数」只是**候选载体**，还没被证明。

**下一步（我这边做，不需要业主配合）**：

1. **字符串表探针**：`internal/catalog/pvf` 的 `Archive.ResolveString(magicOffset)` 能把表达式哈希还原成名字
   （`actcells` 的 `str=` 就是这么来的）⇒ 写一个 `runtime/strexplore` 枚举**所有 `get*()` / `*Grade*` / `*Oath*` /
   `*Primer*` / `*Season*` 表达式及其哈希**，把客户端的取值入口列出来；再把候选**逐个塞进日志槽**读实时值
   （探针手法已验证可靠），谁返回 72 就是它。
2. **并行 Track 2（IDA）**：扫「读 `obj+848` 的函数」= noti 2837/2839/2842 存下的那份载荷的访问器；
   D 盘已腾出空间，这条路重开。
### 19.9 【2026-09-27 05:5x】字符串表探针：把「谁喂 getter」收窄到二选一，并造出 oracle

**最关键的一处认知修正**：脚本 cell 里那个 u32（如 `getPrimerGrade()` 的 28460867）**不是名字哈希，
而是字符串池偏移** —— `internal/catalog/pvf` 的 `resolveString()` 早就写着：
`偶数 n → UTF-8 池 start=n/2`；`奇数 n → UTF-16 池 start=n-1`。一个字符串在池里只存一份，
所以同一个名字在归档任何位置都是同一个 u32（这正好解释了此前误以为「全局哈希」的现象）。
**工具 `runtime/strexplore` 已用已知值自证**：`-needle "getPrimerGrade"` → `offset=28460867 "getPrimerGrade()"`，
与 `primer_proc.act` cell 7 里的值**一致**（`PoolBytes()` 是现成导出接口，不用改库）。

**三条硬结论**：

| 查询 | 结果 |
| --- | --- |
| `-needle "Grade()"` | **全客户端只有 3 条**：`getPrimerGrade()`(28460867)、`getOathGrade()`(28460901)、`t:getMonsterGrade()`(37200947)。⇒ 「grade 从哪来」只有**两个**入口 |
| `-needle "c:season"` | **0 条** |
| `-needle "c:oath"` / `-needle "c:primer"` | 6 条 / 7 条，**全部属于本副本自己的状态机**（`c:oath_rarity_progress_*`、`c:primer_die` …）|

⇒ **两个 getter 读的不是脚本变量，而是引擎态**。于是「谁喂它」只剩两种可能：
**(i) 某个服务端包**（noti 2837/2839/2842 那一族 —— 其 handler 把载荷整块存下来）；
**(ii) 客户端本地数据**。脚本层这条路**正式关闭**。

**于是造出了 oracle**（`cmd/wireprobe/oath_probe.go`，默认关闭）：

```
-oath-inject "2837:64:0;8:71;12:45"   或   DFO_OATH_INJECT="2837:64:0;8:71"      # 逗号分隔多条
```

* 含义：向客户端发 **任意 noti**（id / 长度 / 填充字节 / 哨兵偏移-值），挂在本副本**加载应答**里
  （`finishDungeonLoading`），所以天平开场读 getter 之前就已到位。
* **自动推进**：`oathInjectNext()` 每进一次副本取队列下一条 ⇒ **一次服务启动 + 反复进出副本**
  就能把整串候选扫完；发出去的包会被记成 `oath_inject` 事件（带 `plain_hex`），候选取哪条从日志就能认出来。
* **读数**：探针 v3 已把日志槽换成**实时** `getPrimerGrade()`/`getOathGrade()`，所以「碰一下天平」= 一次读数。
* 判据：某候选让实时值**从 72 变成别的数** ⇒ **载体到手**；再按哨兵偏移把字段逐个钉死。
  先扫长度（`2837:4:0,2837:8:0,2837:16:0,2837:32:0,2837:64:0,2837:128:0`），再扫字段。

回归测试 `TestParseOathInject`（含 5 条非法输入）；`go vet` 干净；`cmd/wireprobe` 全包绿。
### 19.10 【2026-09-27 06:1x】注入器把客户端打崩了 —— 根因是**客户端自己的长度护栏**（已修）

**事故**：第一串候选的第一条 `2839:8:71`（**8 字节**）发出去约 2 秒后客户端闪退。
现场留了真崩溃栈（会话 `..._20260927_060943_836500` 的 `client-direct.out`）：

```
<CrashDump version='1'>
  <Exception code='0xc0000005' where='0xc0000005' violationAddr='0x0000000000000000'/>
  <CallStack eip='0x0000000146ea0c30' checksum='0x0001000186613837'>
    Call 0x146ea0c30    Call 0x140575837    Call 0x1459a3c7c   Call 0x14599d423
    Call 0x1459a208d    Call 0x146d75116    Call 0x1459a5fcd   Call 0x146de4b9a
    Call 0x148860fa2    ... KERNEL32 / ntdll
```

`violationAddr = 0x0` = 空指针写。把 IDB（`D:/115us-backup/ida-work/updvar.i64`）打开，
反汇编 `0x146EA0C30` 得到**真相 —— 那不是解析越界，是编译器插的护栏陷阱**：

```asm
sub_146EA0BE0:                     ; 从全局游标 qword_14F1BF870、剩余量 dword_14F1BF878 读一块
  mov  esi, edx                    ; esi = 请求字节数
  test edx, edx / jle loc_146EA0C3B
  cmp  cs:dword_14F1BF878, esi     ; 剩余 vs 请求
  jl   loc_146EA0C30               ; 请求 > 剩余 → 跳进陷阱
  mov  r8d, esi / mov rdx, cs:qword_14F1BF870 / call memmove
  ...
loc_146EA0C30:
  mov  dword ptr ds:0, 0           ; ← 故意的空写 = 0xc0000005 @ 0
```

调用者 `sub_1405757F0`（**在通知分派链上**，见栈里的 `0x146D75116`）那一步读的是 **15 字节**
（`lea edx,[rax+0Fh]` → `call sub_146EA0BE0`），而我们只给了 **8 字节** ⇒ **请求 > 剩余 ⇒ 自杀**。

**两条结论**：

1. **客户端对 noti 载荷有长度下限**（至少这条路径要 ≥15 字节）。**载荷必须给足，不能拿长度当变量扫。**
   ⇒ 已加护栏：`oathInjectMinSize = 32`，`parseOathInject` 直接拒掉更短的（含 3 条新测试）；
   推荐起点改成 `2839:256:0`。**这是我犯的错：把长度扫描从 8 开始。**
2. **副产品：`2839` 在通知分派链里确实有活处理器** ⇒ 它是个**真候选**（不是空投）。
   而且栈本身给了我们 IDA 的落点：`0x146D75116` / `0x146DE4B9A`（与已知的包分派器
   `sub_146753320` 同一区域）⇒ 可以顺着分派表找 2839 的处理器，把**字段偏移直接读出来**。

**运营教训**：① 真机诊断工具必须**先设好下限/超时**再上；② 崩溃栈要从会话目录的
`client-direct.out` 取（`gateway.err` 里没有）；③ 一条候选崩一次，所以**先 IDA 读几何、再一次发对**，
比盲目扫 payload 划算得多。
---

## 20. 【破案】72 是客户端的硬编码哨兵 —— 处理器注册表与载荷几何全部落地（2026-09-27 07:xx）

本节推翻 §12.7 / §17.1 的两条判断，并且把「服务端该发什么、发多长」变成了可直接实现的东西。

### 20.1 找到了真正的处理器注册表（以及我自己的一个解析错位）

`sub_140006430` 是一张**按包号注册处理器**的表，写法固定为：

```asm
mov  rcx, cs:qword_14E66C090
lea  r8, <handler>          ; ← 处理器在前
xor  r9d, r9d
mov  edx, <packet id>       ; ← 包号在后
call sub_14599D5D0          ; 或 sub_14599D450
```

⇒ **`lea r8, <handler>` 出现在 `mov edx, <id>` 之前**。第一次解析时我从前向后配对，
于是每条 id 都配到了**下一个块的处理器**（错位一格）。修正后（`runtime/ida_handlers.py`
→ 离线重解 `runtime/reparse_registry.py`）得到 **1046 条注册**，落在
`D:/115us-backup/ida-handlers.json` / `ida-handlers-fixed.txt`。

**两个注册辅助函数，两张表**（别混）：

| 辅助函数 | 单例 | 用途 |
|---|---|---|
| `sub_14599D5D0` → `sub_1459A3DD0` | `qword_14E683700` | **网络包**表 |
| `sub_14599D450` → `sub_1459A2FB0` | `qword_14E6836F8` | 另一张表（同名 id 空间会撞车） |

**我们关心的处理器（已修正配对）**：

| id | 名字 | 处理器 |
|---|---|---|
| 2836 | `ENDKEEPER_OF_ORDER_PARTY_INFO` | `sub_140656A80` |
| **2837** | **`ENDKEEPER_OF_ORDER_INFO`** | **`sub_1406568F0`** |
| **2838** | **`ENDKEEPER_OF_ORDER_REWARD`** | **`sub_140656A00`** |
| **2839** | **`OATH_SYSTEM_INFO`** | **`sub_1405757F0`** |
| 2841 | `GET_OATH_EQUIPMENT` | `sub_1405756F0` |
| **2842** | **`PRIMER_COLLECTION`** | **`sub_140575920`** |
| 2382 | `OATH_SYSTEM_INFO`(cmd) | `sub_140575610` |
| 2383 | cmd | `sub_140575470` |

⇒ **§12.7 说的「处理器只是构造器、不读载荷」是错的**：`sub_1448097B0` 那一族是
**对象构造器**（由分派跳转表的 case 调用），而真正**读载荷的是这一层**。

### 20.2 每个包的载荷几何（解析器自己写的长度，精确）

每个解析器都是「**先一次定长读** `sub_146EA0BE0(&buf, N)`，再消费 buf」：

| id | 载荷长度 | 读出来的东西 |
|---|---|---|
| 2836 | **69** | 征兆/队伍信息（`sub_146EA0BE0(&v39, 69)`） |
| 2837 | **20** | `4 × u32` = **4 名队员的角色 ID**（客户端拿它查本地角色缓存里的誓约装备），第 5 个 u32 → 单例 `+760` |
| **2838** | **8** | **`2 × u32` → 单例 `+88` / `+92`** |
| 2839 | **15** | 交给 `sub_14057D140(单例216B, obj, buf)` |
| 2841 | **6** | u16 + u32 |
| 2842 | **104** | 整块 → 单例 `+680..783`（引子收藏位图/列表） |

**这直接解释了 §19.9 的闪退**：当时给 2839 发了 **8** 字节，而它要 **15** ⇒
`dword_14F1BF878 (剩余) < 15` ⇒ 跳进 `loc_146EA0C30` 的空写陷阱。
**「长度下限」不是经验值，是按包查表**（已落进 `oathInjectRequired`）。

### 20.3 ⭐ 72 的真身：客户端单例构造器里的两个字面量

`sub_1406560D0` 是 968 字节单例 `qword_14E6388B8` 的构造器：

```c
*(_QWORD *)a1 = off_1492D76A0;
*(_QWORD *)(a1 + 80) = &off_1492D7908;
*(_DWORD *)(a1 + 88) = 72;      // ←★
*(_DWORD *)(a1 + 92) = 72;      // ←★
```

而 **noti 2838 的解析器 `sub_140656A00` 就是往这两个字段写**：

```c
v3 = 72; v4 = 72;
sub_146EA0BE0(&v3, 8);          // 载荷 [0:4] -> v3, [4:8] -> v4
*(_DWORD *)(v0 + 88) = v3;
*(_DWORD *)(v0 + 92) = v4;
```

**实测正是两个 72**（`rarities : 71, 72, 44, 72` 里的 `primer_max` / `oath_max`）。

**⇒ 72 = 「服务端从没告诉过我」的硬编码哨兵，不是任何数据推导出来的。**
这同时解释了 §9 的所有反常：与装备无关、与星韵石无关、跨 45 个会话逐字恒定。

单例的其余部分**一切成四** —— `+96` 四条 20 字节记录、`+200` 四个 0x58 对象、
`+624` 四个 0x20、`+764` 四条 12 字节 —— 与「**四档征兆**」严丝合缝
（`+752 = 30`、`+760 = 900`）。

### 20.4 玩家口述真值与 PVF 逐条吻合（新增 L0′ 佐证）

业主提供的国服玩法：征兆持有 1–3 个时在「获得一个征兆 / 结算征兆奖励 / 无变化」中随机一个，
**第四档征兆对应太初星蕴石的掉落**。在客户端数据里全部找到对位：

| 证据 | 位置 |
|---|---|
| `hit_normal / rare / unique / legendary / epic / primitive / rainbow1 / rainbow2` | `scale_primer.mob` 动作组 = `Primer_00..07` 八档 |
| `Symptom_1/Unique` `Symptom_2/Legendary` `Symptom_3/Epic` **`Symptom_4/Primeval`** | `../Animation/omen_end/...` ⇒ **第四档 = Primeval(太初)** |
| `[primer disjoint] unique 12 / legendary 28 / epic 50 / primeval 100` | `etc/115lvability2/primerCollection.cos` |
| `getEOOPartyOmenState(t:seatIndex())` / `getEOOPartyOmenGrade(t:seatIndex())` | 池内表达式（EOO = Endkeeper Of Order，按座位号取） |
| `c:omen_drop_process_cnt_max` / `cnt_now` / `c:omen_drop_process_rarity` | 天平状态机（业主说的「计数器」确实存在） |
| `set_omen_drop_process_rarity = rr(40, c:primer_rarity_progress_max)` | 池内表达式 ⇒ 「随机一个」的来源 |
| `oath_max == 45 → nox_is_orthaire` | §11 已录 ⇒ **45（primitive）才召唤奥尔泰尔**，与「第四档 = 太初」同源 |

### 20.5 §17.1「静态反推到尽头」需要加一句限定

- 内建表（2127–2142）确实在**编译器** `sub_147685170` 里（§12.3 成立）。
- 但 `852h` / `853h` 也在**包表**里各注册一次：`sub_14599D5D0` 侧 `852 → sub_142894020`
  （它其实是一次 **94 字节的包解析**），`sub_14599D450` 侧 `852 → sub_146DB7590`、
  `853 → sub_146DB7540`。后者的 852 实体 `sub_146DC1AA0` 是**拼物品提示文本**的函数
  （还会再读 16 字节、按 4 个 u32 查物品名）—— **都不是脚本 getter**。
- ⇒ **两个 id 空间确实会撞号**（§12.4 的警告再次成立），**getter 实现仍未定位**；
  但**已经不需要它了** —— 哨兵在 20.3 就找到了。

### 20.6 实验（已装好，等实机）

`cmd/wireprobe` 的注入器下限已从「一律 32 字节」改为 **`oathInjectRequired` 按包查表**，
并挂上队列（每进一次副本自动推进一格，`scripts/launch_local.py` 的 `DFO_OATH_INJECT`）：

```
2838:8:0;0:45;4:45     ← 两个档位都给 45(primitive) —— 预期同时打通死亡闸门与奥尔泰尔
2838:8:0;0:71;4:45     ← 分辨 [0:4]/[4:8] 哪个是 primer
2838:8:0;0:45;4:71
2838:8:0;0:71;4:71
2837:20:0              ← 对照：4 个角色 ID 全 0
2842:104:0             ← 对照：空引子收藏
```

**判据**（读探针 v3 日志槽里的**实时** `getPrimerGrade()` / `getOathGrade()`）：

| 观测 | 结论 |
|---|---|
| 某条 2838 让读数从 `72,72` 变成合法值 | **载体 + 几何同时定案**，接着把真实档位接上去即收工 |
| `0:45;4:45` 与 `0:71;4:71` 都变 | 就是这两个 u32，顺序再用 `0:71;4:45` / `0:45;4:71` 分清 |
| 六条全是 72 | 2838 不是载体；下一轮换 2837/2842/2839 |

**落地正解（无论实验怎么走）**：这三个/四个包都要在**登录与进本时**推送（常驻状态），
而不是拿到结果才发 —— 所以最终实现位置应在登录握手 + 副本加载应答两处。
---

## 21. 【定案 + 落地】noti 2838 就是载体，四轮差分把偏移钉死（2026-09-27 07:1x）

### 21.1 四轮差分（会话 `..._20260927_071012_487097`）

每轮进本前由注入器发一条 8 字节的 **2838**，读探针 v3 日志槽里的**实时** `getPrimerGrade()` /
`getOathGrade()`；服务端兜底开着（保证能退出），所以每轮都通了关。

| 轮 | 载荷 (LE u32 ×2) | `rarities` 里的 `primer_max` / `oath_max` | 探针 A=`getPrimerGrade()` | 探针 B=`getOathGrade()` | `c:nox_index` |
|---|---|---|---|---|---|
| 1 | `2d000000 2d000000` = 45, 45 | **45 / 45** | **45** | **45** | **109019264** |
| 2 | `47000000 2d000000` = 71, 45 | **71 / 45** | **71** | **45** | **109019264** |
| 3 | `2d000000 47000000` = 45, 71 | **45 / 71** | **45** | **71** | 0 |
| 4 | `47000000 47000000` = 71, 71 | **71 / 71** | **71** | **71** | 0 |

**⇒ 载体 = noti 2838 `ENDKEEPER_OF_ORDER_REWARD`，载荷 8 字节，`[0:4)` = primer、`[4:8)` = oath。
4/4 与实时 getter 逐字吻合，且两轮交换注入证明两个 u32 互不干扰。**

### 21.2 附赠：隐藏 BOSS 的选路闸门被亲眼打开

`c:nox_index`（探针槽 E）**只在 `oath_max == 45` 时**变成 `109019264`
= `orderchroniclerorthaire`（奥尔泰尔），`oath_max = 71` 时是 0。
这与 §11 的 `oath_max == 45 → nox_is_orthaire`（以及 `or(oath_max,42,44,45) → summon_nox_right`）
**逐条对上** ⇒ **45（primitive，第四档「太初」）才是通往隐藏 BOSS 的档位**，
而 70/71 虽然也是合法档（走 rainbow 收尾），但四条选路都不成立。

### 21.3 落地：`cmd/wireprobe/oath_info.go`

- `oathInfoPayload(primer, oath)` → 8 字节（两个 LE u32）。长度必须是 8：解析器
  `sub_146EA0BE0(&v3, 8)` 只读 8，短了会踩长度护栏。
- `oathInfoPackets()` → `{"oath_system_grades", 0, 2838, payload}`，挂在**副本加载应答**的
  同一个 plan 里，**在天平开场执行 `Primer_Proc.act` cell 6–9 之前**（`dungeon_flow.go` 的 loading 分支）。
- 开关 `-oath-grades` / `DFO_OATH_GRADES`，形式 `primer,oath`；**默认 `45,45`**。
  `parseOathGrades` 只接受八档 `{40..45,70,71}` —— 域外值会在**启动时**报错而不是被发出去
  （发 72 等于把「打不死」原样复制一遍）。
- 诊断注入器 `-oath-inject` 保留但已**拆下**（`DFO_OATH_INJECT = ""`）。

### 21.4 下一步验证（已装好，需要一次实机）

**客户端已恢复原版 `5681103c…`**（探针 v3 的「碰一下就收尾」会让天平不可能自然死亡，
所以这次必须用 stock），服务端 = 新 exe **`64101208…`**（09-27 07:2x。vet 干净、全量 21 包全绿），`DFO_SCALE_DEATH_FROM_HP = "1"` 保留（避免卡在副本里）。

**要看的三件事（按顺序）**：

1. **天平自己死**：stock 客户端的日志条件是 `safe_timer >= 60`，正常快打**不会**发 CMD2329
   ⇒ 如果天平死了且 `C2S39 entity=0x100b` 先到，那就是**脚本的 `END_TRIGGER` 自己开的**
   （不是服务端兜底）。判据：`scale_death_forced` 的 `reason` 是 `client_end_trigger` 而非 `hp_floor`，
   且天平血量**没掉到 5%**。
2. **隐藏 BOSS**：`109019264` 是否真的被创建/出现（`c:nox_index` 已确认指向它；
   但 `gm_trigger_on` 还带 `t:getX() < 800` 这个未验证门槛，见 §11.2）。
3. **奖励**：结算里是否出现 `[primer]` 装备（专属奖励表已在真付，§18.8 已证）。

若第 1 条不成立（天平仍不死）⇒ 说明 `primer_now` / `oath_now` 爬不到 45，
下一个变量是 `c:omen_drop_process_cnt_max`（业态：征兆计数器）—— 那属于 2836/2837 的档位。
---

## 22. 【根治已实机验证】天平自己死 + 太初是 0.181% 的概率掉落（2026-09-27 07:2x）

会话 `..._20260927_072127_533863`，客户端 = **原版 stock**（`5681103c…`），
服务端 = `64101208…`，档位由正式的 `oathInfoPackets()` 下发（`2d000000 2d000000` = 45, 45）。

### 22.1 天平自己死 —— 兜底一次都没触发

```
23:22:06.808  oath_system_grades  2d0000002d000000      ← 只发了一次，进本时
23:22:10.4xx  11 只杂兵逐个死亡（C2S39 + monster_death_confirmed）
23:22:42.448  C2S39 entity=0x100c (Scale_oath)   killer=11
23:22:42.451  C2S39 entity=0x100b (Scale_primer) killer=11   ← 天平本体，客户端自己上报
23:22:42.457  map_clear_quest_triggers / boss_check_confirmed / dungeon_clear_enabled
23:22:42.492  dungeon_play_result / clear_experience / clear_reward / eplp_rechallenge
```

- **全场没有一条 `scale_death_forced`** ⇒ **服务端的判死兜底全程没被触发**。
- 战斗从 23:22:11 到 23:22:42 = **31 秒的正常战斗**，不是探针那种「碰一下就收尾」。
- 全场没有 CMD2329 —— stock 客户端的日志条件是 `safe_timer >= 60`，31 秒打不完，本来就不该发。
- ⇒ **72 哨兵修掉后，脚本按设计走完收尾、客户端自己发 C2S39、服务端照常结算。这一条闭环成立。**

### 22.2 附带推论：隐藏 BOSS 其实**也按设计跑完了**

死亡闸门是 `primer_now >= primer_max` **且** `oath_now >= oath_max`（本例 45/45）。
而 §12 已证：`oath_now` 到 `44`(epic) 之后要升 `45`(primitive) **必须** `c:nox_die == 1`，
而 `c:nox_die` 只在**奥尔泰尔/监视者自己的 `last.act`** 里被写。

⇒ **天平能死，就说明奥尔泰尔确实登场并被击杀了。** 这与业主实机观察一致。
**整条隐藏 BOSS 链路全在客户端本地跑完**，服务端一个含 `109019264` 的包都没收到
（本场 13 次死亡上报 = 11 杂兵 + 2 装置，`0x1000..0x100c`，没有第 14 个实体）。

### 22.3 太初不是保底 —— 它是 0.181%/场的概率掉落

**数据面**：`etc/rewardboostinfo/endkeeperoforder/normal.ctp` 里

| 段 | 内容 |
|---|---|
| `[fixed drop table]` maze 0 | 10 项，含 `primeval` item **10416110** 权重 **1500/1,000,000 = 0.15%** |
| `[fixed drop table]` maze 1 | 8 项，含 `primeval` item **10416118** 权重 1500/1e6 |
| `[additional drop table]` effect 2 | `selectProb 14800/1e6`（1.48%）× 其中 `primeval` item 10416144 权重 20270/1e6 |
| **`[hidden drop table]`** | **不存在**（导入器认识这个段名，见 `cmd/attunementimport` 的 `hiddenTableColumn`；大深渊 `epic.ctp` 才有那 13 条 key 0..11 的太初） |

**实测分布**（`runtime/tierdist`，用**服务端自己的 `Roll`** 跑 200 万次）：

```
dungeon 100005014 maze 0   primeval 0.1814%  (~1 in 551 clears)
dungeon 100005014 maze 1   primeval 0.1810%  (~1 in 552 clears)
   unique 20.96% · rare 27.22% · legendary 3.93% · epic 1.73% · luck15 0.33% · luck30 0.05%
```

与声明权重逐项吻合（unique 200000/1e6 = 20%、rare 272000/1e6 = 27.2%、
primeval 0.15% + 附加 ≈ 0.181%）。

**这一把掉的是哪一档**：地面上的 primer 装备是 **`100401612`**，而它**只出现在
`10416107`（unique 档）的 pool1** 里（primeval 档 `10416110` 的 pool1 是
`100401599/100401603/…` 另一组）⇒ **本次滚到 unique（20%），完全正常。**

⇒ **「没掉太初」是概率，不是缺陷。** 「奥尔泰尔必出太初」这个说法在**小深渊的数据里找不到支撑**
（没有 hidden 段、脚本里也没有直接发太初的 cell）。

### 22.4 要「保底太初」的话，需要业主拍板（这是策略变更）

服务端目前对隐藏 BOSS 是**瞎的**：召唤与击杀全在客户端本地。要做出保底，需要两件事：

1. **让服务端知道奥尔泰尔的存在/死亡**（现在是零包）。可选的实现：由脚本参数在服务端侧生成它，
   或按「天平死亡且档位为 45」推断「隐藏 BOSS 已达成」。
2. **定保底规则**（发哪张表/哪件、是否仍受 0.181% 约束、要不要按场次递进）。

这两条都**改变行为**，且没有源数据可依，所以必须业主拍板，不能自行加。
---

## 23. 【对账】官方玩法说明 ↔ 数据 —— 已实现 / 未实现（2026-09-27 07:3x）

业主给出官方说明（小深渊 = 最终调律者）：入场 62 张深渊票 + 8 疲劳；基础掉落 115 级装备
（紫/粉/传说/史诗/太初）、银币、小鸟票；两个特殊事件 **深渊裂缝** / **神秘好运**；
**征兆系统**（通关随机累积 1–4 阶段，满 4 阶段必定触发结算，4 阶段 = **太初星蕴石保底**）；
天平 NPC 商店（星辰/共鸣/超越天平）。

### 23.1 ⭐ 征兆系统的表找到了：`[coupon drop table]` 就是它

`normal.ctp` 的五条 coupon 行（唯一的结构只有 `[obtain prob]` + `[drop prob]` + `[drop list]`），
把每行 `[drop list]` 里的礼盒展开后，与官方「四阶段」**逐条吻合**：

| coupon 行 | `obtainProb` | `dropProb` | 条目 | 主产出档位 | 官方对应 |
|---|---|---|---|---|---|
| 0 | 100000 | **0** | **无 drop list** | — | 「**无变化**」那一路 |
| 1 | 100000 | 600000 | **3** | `10416150` → **unique(粉)** 组 w978570 + 自选箱 `10417544`/`10417543` | 「1阶段：粉星蕴石 / 账绑粉自选 / 账绑粉~太初随机」 |
| 2 | 100000 | 400000 | **3** | `10417545` → **legendary(传说)** 组 w995140 + 2 追加 | 「2阶段：传说星蕴石 / 账绑传说自选 / 账绑传说~太初随机」 |
| 3 | 100000 | 330000 | **2** | `10417552` → **epic(史诗)** 组 w999583 + 1 追加 | 「3阶段：史诗星蕴石 / 账绑史诗~太初随机」 |
| 4 | **0** | **1000000** | **1** | `10417571` → **primeval(太初)** 组 w**1000000**(=100%) | 「**4阶段：太初星蕴石（保底必得）**」 |

**两条独立的对位证据**：① **条目数 3/3/2/1** 与官方文本的斜杠分段数逐条相同；
② **档位 unique→legendary→epic→primeval = 粉→传说→史诗→太初**；③ 第 4 行权重恰 1000000/1000000。

**⇒ 业主说的「必出」= 太初星蕴石，就是 coupon 第 4 行；它 100% 必得，无需任何新的表。**
⇒ **而我们的服务端没有实现征兆系统**：coupon 表**导入了、校验了，但不抽**
（`AttunementRewards.Roll` 只走 fixed + additional，见 §20/§22）。

**仍未定的只有两列语义**：`[obtain prob]` / `[drop prob]` 合计分别 400000 / 2330000，
都不落在百万空间 ⇒ 不是互斥权重；官方文本「从 获得一个征兆 / 结算征兆奖励 / 无变化 中随机一个」
暗示它们是**两个独立子事件的概率**。**这一条属于服务端策略，需要业主拍板后才能定实现。**

### 23.2 两个特殊事件：**没有实现，而且本表里没有独立段**

`normal.ctp` 全部段只有四类（`ctpdump` 的 trailer 列）：

```
[additional drop table] x2   [coupon drop table] x5
[dungeon index] x1           [fixed drop table] x2
```

**没有 hidden 段**（大深渊 `epic.ctp` 才有），**也没有「特殊事件」段**。
脚本侧：在 `contents/2026/endkeeperoforder/` 的池区域里搜 `crack` / `rift` / `luck` / `special`
**没有任何本副本的触发器名**（命中全是别的职业/副本特效）。

因此两个特殊事件只有两种可能，**都还没证实**：

| 候选 | 位置 | 观测 |
|---|---|---|
| `[additional drop table]` effect 2 | `selectProb 14800/1e6 = 1.48%` | 产出 `unique/legendary/epic/primeval`（**粉~太初**），与官方「稀有~太初」只差一档 |
| `[fixed drop table]` 里的 `luck15`/`luck30` | 权重 **3333 / 500**（0.33% / 0.05%） | 礼盒 `10416119`/`10416120` 的 **pool1 `draw=14` / `draw=29`** ⇒ 一次给 14/29 件（像「好运爆发」） |

⇒ **要落实这两个事件，需要业主提供判定依据（或确认上面的候选）**，不能自行挑一个。

### 23.3 天平 NPC 商店：不在副本脚本里

星辰/共鸣/超越天平（紫/粉/传说/史诗灵魂兑换 + 必得粉/传说/史诗星蕴石）属于 **NPC 商店系统**，
不在 `endkeeperoforder` 的脚本与奖励表里 ⇒ 与本次小深渊道具链是两套东西，**未实现**。

### 23.4 结论表

| 官方机制 | 状态 |
|---|---|
| 天平可击杀 → 通关/结算 | ✅ 已实现并实机验证（§22，兜底零触发） |
| 专属奖励（fixed + additional）→ 含太初**装备** 0.181% | ✅ 已实现（§22 定量） |
| 2838 档位下发（让收尾/死亡/隐藏 BOSS 走通） | ✅ 已实现 |
| **征兆系统（4 阶段 → 太初星蕴石保底）** | ❌ **未实现**（表已导入；机制与档位已定位，见 §23.1） |
| **深渊裂缝 / 神秘好运** | ❌ **未实现**，且本表无独立段（候选见 §23.2） |
| 天平 NPC 商店 | ❌ 未实现（另一套系统） |

## 24. 【实现】征兆系统（omen）：4 阶段 → 太初星蕴石保底（2026-09-27 08:0x）

§23.1 已经把表找到了：`etc/rewardboostinfo/endkeeperoforder/normal.ctp` 里那 5 行
`[coupon drop table]` 就是征兆阶段表。本节记录**读法**（它此前一直缺）、**实现**与**验收**。

### 24.1 ⭐ 读法：两列概率是「三选一」的前两个分支，行号 = 结算前持有数

一次通关只做**一次** `roll(1_000_000)`，落点决定唯一分支：

| roll 落点 | 结果 |
| --- | --- |
| `roll < [obtain prob]` | **获得一个征兆**（持有 +1） |
| `roll < [obtain prob] + [drop prob]` | **结算该阶段奖励**（抽该行 `[drop list]` 一次），持有**归零** |
| 其余 | **无变化** |

选中哪一行由**结算前玩家持有的征兆数**决定（0 → 第 0 行，4 → 第 4 行）。

这个读法有两条**互相独立**的真值撑着，缺一条都不够：

1. **官方对玩家的说明**是「征兆持有 1-3 个时，会从 *获得一个征兆 / 结算征兆奖励 / 无变化*
   中随机一个」——那就是三个分支，不是两次独立判定。
2. **这两列的合计是文件里唯一不落在百万空间的**（本表 obtain 400000、drop 2330000）。
   §23 之前把它读成「没有不变量 ⇒ 不可证伪 ⇒ 不敢实现」；恰恰相反：三选一的模型
   **不需要**它们落，因为「无变化」拿走了剩余空间 —— 这正是那个"缺掉的不变量"的答案。

三选一同时消掉了旧读法的一个自相矛盾：若两列独立，「获得征兆」与「结算奖励」可以同场发生，
而官方说的是三选一。

**行数 = 5（阶段 0..4）**，与官方四阶段一一对应，三条独立对位：

| 行 | obtain | drop | 条目数 | 主产出档位 | 官方文本 |
| --- | --- | --- | --- | --- | --- |
| 0 | 100000 | **0** | 0 | — | （还没攒到：不结算） |
| 1 | 100000 | 600000 | **3** | `unique`(粉) | 粉星蕴石 / 账绑粉自选 / 账绑粉~太初随机 |
| 2 | 100000 | 400000 | **3** | `legendary`(传说) | 追加 传说星蕴石 / 自选 / 传说~太初随机 |
| 3 | 100000 | 330000 | **2** | `epic`(史诗) | 追加 史诗星蕴石 / 史诗~太初随机 |
| 4 | **0** | **1000000** | **1** | **`primeval`(太初)** | 追加 **太初星蕴石（保底必得）** |

第 4 行 `drop = 1000000`（满空间）就是官方那句「累积满 4 阶段必定触发奖励结算」，
它在数据里是**逐字**的。

### 24.2 实现

**新增**

| 文件 | 内容 |
| --- | --- |
| `internal/loot/omen.go` | 读法（`OmenStage` / `AdvanceOmen`）、`OmenTemplates` / `OmenStageTemplates`、`ValidateOmen`，以及 `payableTemplates`（固定+追加+征兆，供启动期开箱校验） |
| `internal/loot/omen_ledger.go` | `OmenLedger`：按角色 ID 记账（持有数 + 最近一次结算 + 自增序号） |

**接线**

- `loot.Session` 新增 `Omen *OmenLedger` 与 `omenRolled`；`loot.Service` 新增 `Omen`。
- 推进点在 `Session.Death` **调律专属奖励那一块内部**，与固定/追加奖励**同一个事件、同一把锁**：
  本副本的通关判定与源领主（天平 `109019266`）的死亡本来就是同一个事实。征兆的包装与固定奖励
  **合并后一起交给 `OpenRewardBoxes`** —— 分两条路就等于「同一件东西有两个分布」。
- 包装展开、掉落行编码、拾取全部复用既有路径，没有新的发放通道。
- 启动校验面从 `RolledTemplates()` 扩到 `payableTemplates()`：一旦开始抽征兆行，一个开不出产物的
  包装必须同样在启动期硬失败（`ValidateBoxes`），而不是落到玩家脚下。

**开关**

| flag | env | 默认 | 说明 |
| --- | --- | --- | --- |
| `-omen-rewards` | `DFO_OMEN_REWARDS=1` | **关** | 打开征兆系统；`scripts/launch_local.py` 已置 1（实机验证用） |
| `-omen-hold N` | `DFO_OMEN_HOLD=N` | `-1` | **诊断**：把玩家直接放到第 N 阶段，每个会话只应用一次。用来一眼验保底，不必刷场次 |

**事件**：`omen_clear{ dungeon, held, after, gained, paid, stage, templates }` ——
「这把给了什么」从此能从 `events.jsonl` 直接读出来，不必反推掉落物属于哪一档。

### 24.3 验收（用服务端自己的代码 + 真实目录）

- `go build ./...`、`go vet` 干净；**全量 21 包全绿**（4m21s）。新 exe = `7f8f4a1b…`（09-27 07:47）。
- `internal/loot/omen_test.go`（8 条）：五行数值、三选一互斥、阶段 0 永不结算 / 阶段 4 必结算且
  内容固定、无表副本连种子都不动、账本能累积到保底、`ValidateOmen` 拒绝「声称会结算却没列表」。
- `cmd/wireprobe/omen_reward_integration_test.go`（`ATTUNEMENT_REWARD_INTEGRATION=1`，2 条）：

```
TestOmenStagesPayTheirOwnTier
  omen stages 1..4 paid 936 row(s) over 48 template(s)
TestEndkeeperClearPaysTheOmenGuarantee
  the guarantee paid in 12/12 clears over 12 reachable template(s):
  [100401599 x1 100401603 x3 100401611 x1 100401615 x3 100401631 x1 100401639 x2 100401643 x1]
```

第一条的判据是**派生**的、不写死任何物品 id：把四行各自的 `[drop list]` 递归展开成内容集，
断言**四个集合两两不相交**（串档就是读错了行），再断言 400 场结算里实际产物只落在自己那一格。
48 = 4 档 × 12 件 primer 装备，与「四档星蕴石」逐档对上。

第二条走完整链路（`dungeon.Select` → `ConfirmDeath` → `Session.Death`），把玩家放到第 4 阶段：
产物里**每一场**都能找到太初档装备，且这些装备**通用掉落池付不出**（`EquipmentCatalog.Basic`
要求 `[free]` 且 `[rarity]<=2`，而 primer 装备是 `[trade]`）⇒ 它们只可能来自征兆，不可能是杂兵/通用奖励。

### 24.4 ⚠️ 已知限制（实机前必读）

1. **账本在内存里，重启归零。** 征兆是跨场累积的玩家状态，正式落地要接角色存档
   （`attend_*` 无关；这里指 `storage.Character.State` 或一张新表）。本轮为「先让玩家试试看」，
   先做进程内记账；一次游戏会话内连刷多场完全正常。
2. **结算后持有归零是推断。** 数据只有第 4 行 100% 结算这一条硬约束；若不消耗，玩家此后每场
   必得太初（显然过强）。这一条单独写在 `omen.go` 顶部，改的时候只有一处。
3. **客户端 UI 还看不到阶段数。** noti **2836 `OMEN_OF_ORDER_PARTY_INFO`**（69 字节）才是喂
   `getEOOPartyState(t:seatIndex())` 的载体，本轮**没有下发**：已知它 `[0..67]` 是 4×17 字节记录
   （每条 = 4×u32 + 1 个标志字节）、`[68] == 1` 才进入「有队伍征兆信息」分支（`sub_140656A80`），
   但那 4 个 u32 的语义**还没有真值**。给长度对不会有护栏崩溃（与 §19.9 那次给 2839 发 8 字节
   不同），但语义错会让 UI 显示错误。按「先确保掉落物准确」的口径，本轮不发。
4. **两个特殊事件（深渊裂缝 / 神秘好运）仍未实现**，原因不变：`normal.ctp` 里没有对应段（§23.2）。

### 24.5 与官方说明的对账（更新 §23.4）

| 官方机制 | 现在 |
| --- | --- |
| 天平可击杀 → 通关/结算 | ✅ 已实现并实机验证 |
| 基础掉落：紫/粉/传说/史诗/太初 **装备** | ✅ 已实现（太初 0.181%/场） |
| **征兆系统（4 阶段 → 太初星蕴石保底）** | ✅ **本轮实现**（开关默认关，实机开关已置 1） |
| 深渊裂缝 / 神秘好运 | ❌ 无表，待业主给判定依据 |
| 天平 NPC 商店（星辰/共鸣/超越） | ❌ 另一套系统，未做 |

## 25. 【修复】包装展开统一到所有来源；「通用掉落 / 深渊专属」的边界（2026-09-27 08:1x）

### 25.1 实机反馈的归属：那批盒子出自展开功能上线**之前**的版本

玩家反馈：背包里有 4 个同款盒子，客户端文案是「**稀有武器外（不实际发放礼盒，以开封状态发放）**」，
**都打不开**。

从日志定位到确切来源：

| 项 | 值 |
| --- | --- |
| 会话 | `..._20260926_153358_804562`（本地 09-26 15:34；`events.jsonl` 的 time 是 **UTC**，+8 即本地）|
| 副本 | 大深渊 `100005068` |
| 怪 | `entity 0x1016`（5 行掉落），时间 `07:34:54Z` |
| 盒子 | `10419728`(epic) / `10419731` / `10419732` / `10419734` |

这 4 个模板**都是 `100005068` 的表项**（`fixed` tier=epic 与 `additional`），而且**都在
`booster-catalog` 里有开箱定义** ⇒ 它们本该被展开。

**原因是版本**：奖励递归展开（`OpenRewardBoxes`）是在 **09-26 17:00** 随 `!60` 合入上游的，
而这一场发生在 **09-26 15:34** —— **早于**那次合并。所以这是历史遗留，当前版本不会再产生。

### 25.2 但玩家指出的方向是对的：展开此前只挂在调律那一块

修这条时顺手发现一个**真缺口**：`OpenRewardBoxes` 原来只在 `Session.Death` 的**调律专属奖励块内部**
调用，也就是说

- **通用掉落池**（`RollWithBonus`，全游戏每一只怪都走的那条）的产物，
- 以及章节盒（`ChapterDrop`）、

**从来没有走过展开**。源对所有包装的说明都是「不实际发放礼盒，以开封状态发放」，所以
「哪条线给出的包装」不该影响结论 —— 包装落到玩家脚下就是错的。

已改为**所有来源统一展开一次**（`internal/loot/session.go`，紧接在各奖励源拼接之后、编码落地面之前）。

两条性质保证它不是一次冒险的改动：

1. **没有包装时连随机数都不消耗**。`OpenRewardBoxes` 只在真的开箱时 `pick`，所以对不含包装的
   掉落（也就是当前全部普通副本），这条改动**逐字节等于旧行为**。
2. **金币不会被吃掉**。通用掉落用 `template 0` 表示金币，而奖励表用同一个 id 表示「本次没有」的空槽；
   `template 0` 在物品目录里是 `stackable/gold.stk`（`[waste]`），会被 `Item()` 判为可发放 ⇒ 原样放行。

### 25.3 ⭐ 「通用掉落」与「深渊专属」是两个独立的东西（回答业主的疑问）

这两个概念容易混，它们在代码里是**两条互不相干的线**：

| | 通用掉落（`RollWithBonus`） | 深渊专属（调律表 + 征兆） |
| --- | --- | --- |
| 表 | `configs/loot.*.json` + `drop.compat90.json` 规则 + 装备池 | `configs/attunement-rewards.generated.json` |
| 触发 | **每一只怪**死亡（按等级/rank/难度/任务加成） | 只在**源领主**（rank 3 且模板 == `SourceBoss`）死亡时 |
| 作用域 | 全游戏 | **只对表里声明了 `[dungeon index]` 的副本号生效** |
| 对其它副本 | —— | `Roll` / `AdvanceOmen` 查不到表就**原样返回种子**，零消耗、零影响 |

**所以「深渊掉落独立开」这件事本来就已经是这样**，不需要额外隔离：表按副本号索引，别的副本连
随机数都不碰。业主担心的「污染普通地图」在两条线上都不会发生：

- **不会往普通地图发深渊的东西**：`TestAbyssTablesStayInsideTheirDungeons` 故意把一个**普通副本**
  接上深渊表再跑 6 场，断言产物里**没有任何**深渊专属模板；
- **不会被深渊改写普通掉落**：同一条测试断言「包装展开对普通副本是惰性的」——
  没有包装时不动随机数，产物逐字节等同旧行为。

⚠️ 唯一需要留意的是**展开这张网**：它是全局的（所有来源），但它只做「包装 → 内容」的替换，
**不改变哪些模板会掉**。所以它对普通地图的影响是「把一个打不开的盒子换成一个能用的东西」，
而不是「多给或少给」。

顺带记一条边界：**包装在物品目录里必须要么能开、要么被明确拒绝**。启动期的 `ValidateBoxes`
现在走 `payableTemplates()`（固定 + 追加 + 征兆），把「开不出产物的包装」在启动时就拦下来；
`OpenRewardBoxes` 对目录不认识、又带容器标记的模板会**跳过并记账**，而不是发个死盒子出去。

## 26. 【排障】「捡起来没进背包」的粉色盒子 —— 服务端链路正确（2026-09-27 08:1x）

玩家反馈：地上掉了个「粉色的装备像盒子一样」，**捡起来背包里找不到**。

逐层取证（会话 `..._20260927_080612`，小深渊）：

| 层 | 证据 | 结论 |
| --- | --- | --- |
| 掉落 | `monster_death_confirmed`(38) 天平 `entity 0x100b` 4 行：`10362432×2`、**`100401592`**、`(1)×2` | ✓ 按表发奖；`100401592` 是 `10416106` 展开出来的 **primer 装备** |
| 拾取 | `pickup_ack`(43) + `pickup_scene_removed`(39) + `pickup_inventory_updated`(14) | ✓ 三件套都发了 |
| 落点 | 收据 slot = **13**，与 NOTI14 载荷里的 `00 0d` 逐字一致 | ✓ 进**装备背包**（`AddEquipment`，不是堆叠物槽）|
| 存档 | DB `characters.state.inventory.equipment`：slot 11 与 slot 13 **都是 `100401592`** | ✓ 已持久化（一件是本场，一件更早）|
| 编码 | NOTI14 的 row（`slot u16 + template u32 + …`）与全量 `inventory_restored` 里同一件的字节布局**逐字相同** | ✓ 增量包格式正确 |

⇒ **服务端这条链没有缺口**：掉落、拾取、入包、存档、同步包全部正确。

剩下的只可能是**客户端渲染/刷新**，两个分支各需一次玩家侧动作即可判别：

1. **增量包不刷新装备页**。NOTI14 刻意只带被拾取的那一行（全量会让所有格子闪「新获得」高亮）。
   若客户端只在**全量** `equipment_bag_resynced`(13) 时才重画装备页，装备类拾取后装备页就不更新。
   **判别**：重新登录一次（走全量 restore）—— 若那两件 primer 装备**出现**，即此分支。
2. **客户端不在装备页渲染 `[primer]` 装备**（只在星蕴石/引子栏渲染）。玩家 `worn` slot 36 已穿 `100401640`
   （同为 `[primer]`）⇒ 客户端**认**这个类型，但「背包里显示」是另一回事。
   **判别**：穿脱一次装备（触发 NOTI13 全量）后看装备页。

⚠️ 顺带一条形态说明：`100401592` 的客户端图标本来就是「像盒子」的（它在
`equipment/character/common/primer/`，是**星蕴石/引子的容器**），所以玩家看到的形态与它的身份一致 ——
**不是掉错了东西**。

## 27. 【排障】「捡了看不到」的最终定位：服务端逐层正确，剩下的是**背包槽位布局**（2026-09-27 08:2x）

### 27.1 这一轮把服务端所有环节钉死了

对 `10362432`（小深渊天平掉落，客户端名为「稀有武器外（不实际发放礼盒，以开封状态发放）」）的完整取证：

| 层 | 硬证据 |
| --- | --- |
| 掉落 | `monster_death_confirmed`(38) 天平 `0x100b` 4 行，含 `10362432 × 2` |
| 拾取 | `pickup_ack`(43) + `pickup_scene_removed`(39) + `pickup_inventory_updated`(14) 三件套齐全 |
| 槽位 | 收据 slot = **121**；`Add()` 按 `[material] → [121,176]` 选空槽，合法 |
| 增量包 | NOTI14 行 = `79 00`(121) + `40 1e 9e 00`(10362432) + `78 01 00 00`(376) |
| 全量包 | **重登后**的 `inventory_restored`(13) 里，同一行仍是 `79 00` + `10362432` + 376 |
| 存档 | DB `inventory.items` slot 121 携带该堆叠 |

⇒ **服务端「发了、存了、两种包都发了、槽位合法」全部成立。** 玩家在客户端看不到它，不是服务端丢包。

### 27.2 ⭐ 现在最可疑的一条：槽位范围是**手工参考**，不是从客户端读出来的

`configs/inventory.current37.json` 的 `model` 是 `reference90-bag-v1`，`source` 指向我们冻结的内层
（`7ef2db59…`）。但**全仓库只有 `internal/inventory/bag.go` 的校验代码引用这个 model 字符串，没有任何生成器** ——
也就是说这两个范围：

```
[throw]    → [65, 120]
[material] → [121, 176]
equipment_slots [9, 64] · quick_slots [0, 8]
```

是**人为写下来的**，不是从客户端脚本里读出来的。而 §17 已经发现：**实机 `client/Script.pvf` 比我们冻结的内层多约 1.23 MB 尾部内容**
（`rewrap(unwrap(live)) == live` 自证过）。两件事叠起来就是同一个风险：**实机客户端的背包布局可能与我们这份参考不一致**
⇒ 服务端算出的 slot 会落在客户端**不渲染的格子**上。

⚠️ 另外 `Add()` 对**未在 `slots` 里声明的类型**是回退的（含 "material" → [121,176]，否则 → [65,120]），
所以 `[virtual]`（`10362481`「迷雾工商协会银币」）会落到 [65,120] —— **这是回退，不是读到的规则**，同样待考。

### 27.3 下一步（按性价比）

1. **要一张材料页截图**（玩家侧，30 秒）：看 **slot 121（材料页第 1 格）** 与 **slot 122**（那里有老物品 `3166`）分别有没有东西。
   三种结果分别指向「单格问题 / 材料页整体 / 名字渲染」。
2. **对比实机客户端的背包布局**（我这边做）：解开实机 `client/Script.pvf` 的内层，找到定义槽位范围的脚本，
   与 `inventory.current37.json` 逐字段对照。**如果两者不同，这就是根因**，修法是「按实机布局重新生成这份配置」，
   而不是改掉落逻辑。
3. 名字那条线暂缓：物品名是**本地化 key**（`<13::name_10362432>` / `<3::name_100401592>`），
   不是字符串池文本 —— 新工具 `runtime/itemname` 已经能把它读出来（`-ids` 打印 value + resolved + reference），
   但要把 key 变成中文还需要找到本地化表，本轮没找到（`-find "stringtable"` 只命中 `etc/avatarabilitystringtable.etc`）。

### 27.4 本轮被证伪的两个假设（记下来，别再走）

- ❌「拾取只发 NOTI14，客户端不重建新行 ⇒ 重登（全量）就能看到」—— **玩家重登后仍然看不到**，而全量包里那一行
  确实带着正确 slot ⇒ 假设不成立。
- ❌「`100401592` 在背包里但客户端不渲染」—— 复查后是**玩家自己在 `08:12:27~30` 用三次装备移动（C2S 19）把它移到了 worn**，
  此后全量包里就再没有它 —— 它不是"看不到"，是"已经不在背包里了"。

### 27.5 新增工具

`runtime/itemname`：按模板号打印物品的 `[name]` / `[stackable type]` / `[equipment type]` 等字段
（`value` + 池解析 + reference 三种形式都给），并可用 `-sections` 列出该脚本声明的全部段名
（用来判断一个 `[material]` 物品里是否藏着 `[booster info]`）、`-grep` 做字段内搜索。

## 28. 【结论】「稀有武器外」不是包装器；深渊表里也不存在漏展开的礼盒（2026-09-27 08:5x）

业主的假设：「`稀有武器外` 本身只是一个**包装器/索引**，真正掉落时才向下递归随机出真实装备，
所以服务端的拾取一直拒绝它」。**机制方向完全正确 —— 那正是我们已经实现的**（`OpenRewardBoxes`
递归到产物，包装本身绝不落地）。但用原始脚本核对后，**这套表里没有"漏网的包装器"**：

### 28.1 深渊奖励表 126 个模板：全部是包装，且全部被正确识别

用新工具 `runtime/itemname -sections` 扫了 `attunement-rewards.generated.json` 里**全部 126 个**模板的原始段名：

```
('stackable', '[booster]')   126 / 126      带 [booster info] 段: 126 / 126
带 booster info 的类型集合: [('stackable', '[booster]')]
```

⇒ 每一个都是 `[booster]` 且都声明了 `[booster info]`。**没有任何一个 `[material]`/`[virtual]` 物品
藏着 `[booster info]`** ⇒ 服务端「按 `stackable_type` 判容器 + 按 `[booster info]` 展开」的判定
在这套数据上是**完备**的，「漏展开」不成立。

### 28.2 那两件具体物品都不是包装器

| 模板 | 原始段名 | 判定 |
| --- | --- | --- |
| `10362432`（材料页 376 个那件） | `[name] [explain] [flavor text] [grade] [rarity] [usable job] [attach type]=[account] [minimum level] [icon] [field image] [stackable type]=[material] [material] [move wav]` | **纯材料**；`[attach type]=[account]` = **账户绑定** ⇒ 正是截图里那枚「迷雾工商协会银币」 |
| `100401592` / `100401640`（`[primer]`） | `[name] [grade] [rarity] [usable job] [attach type]=[trade] [minimum level] [equipment buff] [fame value] [icon] [field image] [equipment type]=[primer] [primer] [item group name]=primer …` | **真装备**，不是包装器 |

### 28.3 拾取没有被拒绝

三次拾取（`08:17:24/26/26`）全部 `pickup_ack`(43) 成功，且材料页**确实**出现了两格：
**376**（= `10362432`）与 **9**（= `3166`）。业主自己也确认「材料栏是 2 个正确的材料」。

### 28.4 仍未知：「稀有武器外」是哪个模板

- 物品名是**本地化 key**（`10362432` → `<13::name_10362432>`、`100401592` → `<3::name_100401592>`），
  不是字符串池文本；
- 在 `client/` 整个目录（含所有明文文件）里搜 `name_10362432`（ASCII 与 UTF-16）**零命中**
  ⇒ 名字表在 PVF 压缩层内，本轮还没定位到它（`-find "stringtable"` 只命中 `etc/avatarabilitystringtable.etc`）；
- `08:13:54` 天平掉的 4 件（金币 3519 / `10362432`×2 / `10362481`×1 / 金币卡）**都能对上账**，
  里面没有第三个可疑名字 ⇒ 「稀有武器外」**不是这一场掉的**。

⇒ 需要一次**带图标的截图**（地面那件 / 客户端背包的「装备」标签）才能把它落到具体模板上。

### 28.5 本轮新增的权威手段

`runtime/itemname`（已编译为 `bin/itemname.exe`）：

```
bin/itemname.exe -ids 10362432 -sections                       # 列出脚本声明的全部段名
bin/itemname.exe -ids 100401592 -fields "[name],[equipment type]"
bin/itemname.exe -ids 10362432 -grep booster                   # 字段内搜索
```

**判断「一个物品是不是包装器」从此有直接证据**：看它有没有 `[booster info]` 段、
`[stackable type]` 是什么 —— 不再需要靠 `stackable_type` 字符串或第三方文档推断。

⚠️ 内存纪律：客户端在跑（`DFO.exe` 1.68 GB）时，任何加载内层 PVF 的工具都会
`VirtualAlloc errno=1455` OOM。这次实测两遍，最后靠**先退客户端** + **用编译好的 exe（省掉 go run 的编译内存）** 才跑通。

## 29. 【真 bug】`10362480` 是容器，但被"类型判定"漏掉 —— 落地即打不开（2026-09-27 09:0x）

### 29.1 本场日志（会话 `..._20260927_085700`，目录名本地 08:57 / `events.jsonl` 里是 UTC 00:58）

| 事件 | 内容 |
| --- | --- |
| 11 只杂兵 | 全部 `n=0`（通用掉落为空，与本副本 source `[exclude gold drop]` 一致）|
| 天平 `0x100b` | 一次 4 行：`(10362432, 2)`、`(100401608, 1)`、`(10362480, 1)`、`(1, 1)` |
| `omen_clear` | `{held:0, gained:false, paid:false, stage:0}` ⇒ 本场征兆掷点落在"无变化"，没发征兆奖励 |

来源可以逐行对上：fixed 抽到 **`10416107`**（unique 档，`w=200000/1e6`），它的三个池分别给
`10362432×2`（pool0）、`100401608`（pool1，12 选 1）、`10362480`（pool2，14 选 1）；
additional **effect 1** → `10416752`（其唯一内容就是金币卡 `1`）→ `(1, 1)`。

⇒ **地面没有出现我们表里的任何包装盒 id**，展开链本身是通的。

### 29.2 但第 3 行 `10362480` **自己就是容器**，而两处判定都把它漏了

`bin/itemname.exe -ids 10362480 -all`（34 cells）原文：

```
[stackable type]  [virtual]                    ← 关键：不是 [booster]
[smart drop group id]  21250
[mulit open limit]  1
[booster info] [etc] 1 490000001 10000 1 [/etc] [/booster info]
[instantly open]                               ← 「以开封状态发放」
```

判定写的是**类型字符串**，不是**结构**：

- `cmd/boosterexport/main.go:211`：`isBooster := strings.Contains(item.StackableType, "booster")`
- `cmd/wireprobe/booster_flow.go:199`（`Container()`）：`strings.Contains(item.StackableType, "booster")`

⇒ `[virtual]` 的容器进不了 `configs/booster-catalog.json`，`Container()` 也返回 false ⇒ **原样落地**，
玩家拿到一个打不开的盒子（客户端文案「不实际发放礼盒，以开封状态发放」正是 `[instantly open]` 的语义）。

`internal/loot/reward_box.go` 的接口注释其实**已经写对了**这个语义 ——
"Container reports whether a template is a box by the source's own metadata, **even when this build cannot open it**"
—— 但实现只认类型字符串。这是"接口说对了、实现没跟上"的一处。

### 29.3 规模（`runtime/boxsurvey`，扫 `items.index.json` 里 175,555 个 stackable）

| stackable type | 总数 | 有 `[booster info]` | 有 `[instantly open]` | **有 info 但不在 catalog** |
| --- | ---: | ---: | ---: | ---: |
| `[booster]` | 37,387 | 37,387 | 155 | **6,125** |
| `[cera booster]` | 2,502 | 2,502 | 0 | **34** |
| `[virtual]` | 191 | 178 | 173 | **178** |
| `[etc]` | 9,198 | 15 | 0 | **15** |
| `[smart drop]` | 10 | 10 | 0 | **10** |
| 其余（`[booster selection]` / `[booster random]` / `[dungeon and life booster]` …）| — | 有 | 0 | 0 |

两类缺口成因不同，必须分开修：

1. **类型被过滤掉**（`[virtual]` 178 / `[etc]` 15 / `[smart drop]` 10 / `[cera booster]` 34）——
   它们在 `boosterexport` 第 211 行**在解析之前**就被 `continue` 掉了。`10362480/10362481` 属于这一类。
2. **解析出 0 个池就静默丢弃**（`[booster]` 6,125）—— 第 224 行 `if len(pools) > 0` 才写条目，
   所以"有 `[booster info]` 但本构建解析不出来"的盒子**不可见**，既不展开也不报错。

### 29.4 那 `10362480` 该给什么？—— 现在还不知道，而且**不能**就地展开

- 它的 `[booster info]` 体里唯一候选是**保留 id `490000001`**（全归档 226 处引用该 id），
  不是可发放物品 ⇒ 单看 booster 体推不出内容。
- 真正的内容应由 **`[smart drop group id] 21250`** 指向的组表决定。
  `10362481` 是 `21251`，而 `endkeeperoforder.ctp` 里的 `[normal group index] 1 21251` 说明
  这两个号就是本副本"普通组"的编号 —— 也把 09-26 那条"`[normal group index]` 编码未确立"往前推了一步。
- **阻塞**：组表 `etc/dungeondroptablebygroup.etc` **我们自己的解析器读不了** ——
  `catalog.ParseDropGroups` 报 `unknown drop group section [create random option level]`
  （**冻结内层与实机内层都报**）⇒ 09-26 落的"组表解析"目前**不可用**；
  `configs/loot.*.json` 里也确实没有 `drop_groups` 键（那两份配置是 09-23 生成的）。
- ⇒ **现在不能把 `10362480` 展开**：按现有语义展开会产出保留 id ⇒ 发 0 件，
  等于把一次掉落静默删掉，比"落地一个打不开的盒子"更糟（玩家至少看得见）。

### 29.5 物品名仍无法从数据侧解析（并更正 §27.1 的一处归属）

- `[name]` / `[explain]` / `[flavor text]` 都是**本地化键**（`<13::name_10362480>`、`<13::explain_10416107>`、
  `<3::name_100401608>`），不是字符串池文本。
- 本轮把 `runtime/strexplore` 的一个真 bug 修掉了：它按"每个字节后面补 0"拼 UTF-16，
  **中文 needle 永远搜不到**（改成 `utf16.Encode` 后才有可能命中）；即便如此，池里对
  「不实际发放礼盒」仍是 **0 命中**。
- `pvfinspect -find` **只搜路径不搜内容**（`-find "explain_10416107"` → 0 命中，实测确认），
  本机 `client/` 明文目录里搜 `name_10362432` 也是 0 ⇒ **本地化表尚未定位**，
  因此**中文名一律不给结论**。
- 本场唯一的 ×2 是 `10362432`，而截图里 `迷雾工商协会银币(2EA)` 正好是 2 个 ⇒
  **该名对应 `10362432`**。§27.1 把 `10362432` 记成「稀有武器外（不实际发放礼盒，以开封状态发放）」
  **缺证据，标为待确证**（不过 `10362432` 确实是这 4 件里**唯一**带 `[explain]` 段的 ——
  `10362480`/`10362481`/`100401608` 都没有，所以"带说明的那件"只可能是它）。
- 顺带认出一件：`100401608` = `[primer]`、grade 116、rarity 3、图标
  `19_OathPrimer/Primer/Primer_04_Dragon` ⇒ **龙**档星蕴石，与截图 `龙战：朦胧光辉星蕴石` 的档名吻合。

### 29.6 本轮新增/修正的工具

| 工具 | 作用 |
| --- | --- |
| `runtime/boxsurvey` | 全量扫描"有 `[booster info]` 却不在 catalog"的 stackable，按类型分桶（就是 29.3 那张表）|
| `runtime/dropgroup` | 用 `catalog.ParseDropGroups` 打印指定 `[smart drop group id]` 的组内容（本轮因此发现解析器读不了实机表）|
| `bin/itemname -all` | 新增：逐 cell 打印 type/value/resolved（先前的 `-sections` 只看得到标签）|
| `runtime/strexplore` | **修 bug**：CJK needle 的 UTF-16 编码（原实现中文永远 0 命中）|

### 29.7 修复方向（**已实现：无**。按纪律，未验证不动代码）

1. **判定改结构**：`boosterexport` 与 `Container()` 都改成"脚本里有没有 `[booster info]`"，
   而不是 `stackable_type` 字符串（item index 需要带一个 `booster_info` 标志，或在 importer 里生成）。
2. **不许静默丢**：`boosterexport` 对"有 `[booster info]` 但解析出 0 池"的条目要**报数并列出**，
   否则 6,125 个盒子会一直隐形。
3. **先修 `ParseDropGroups`**（新增/忽略 `[create random option level]` 段并加测试），
   才可能知道 `10362480/10362481` 该给什么；**在拿到组内容之前不要展开它们**。
4. **本地化表**仍未定位：物品中文名只能靠玩家读，或继续找 `<13::name_*>` 的载体。

## 30. 【修复】`10362480/10362481` 真正展开成装备：容器判定改结构 + 智能掉落组回填（2026-09-27 09:3x）

### 30.1 结论

**`[booster info]` 段的存在才是"这是容器"的判据，`stackable_type` 只是标签。** 改成结构判定后，
两个 `[virtual]` 载体进入目录并**真的展开**成装备；它们 booster 体里的保留 id `490000001` 被换成
各自 `[smart drop group id]` 指的组内容：

| 载体 | 组 | 内容 | 落地形态 |
| --- | --- | --- | --- |
| `10362480` | **21250** | **42 件装备**，各 weight 30（`101001149` / `101011315` / … / `117020241`）| 抽取 1 件 |
| `10362481` | **21251** | **11 件**，各 weight 10（`100051304` / `100101187` / … / `100391038`）| 抽取 1 件 |

**规则的数据依据**（`runtime/smartdrop`，全归档 40,220 个带 `[booster info]` 的 stackable）：
**226 个** booster 体含保留 id，且**全部**声明了 smart drop group；**0 个**含保留 id 而没有 smart group；
**0 个**反例。回填后导出器报 `smart drop substitutions = 226` —— 与统计一致。

### 30.2 组表终于能读了（三个真数据形态 + 一条新策略）

`etc/dungeondroptablebygroup.etc`（85,379 cells / 1,223 个组 / 12 种段）此前**整张表都读不了** ——
`ParseDropGroups` 遇到不认识的段就拒整张表，而 09-26 落的「组表解析」因此**从未真正跑通过**（`loot.*.json` 里也没有 `drop_groups` 键）。

| 形态 | 数量 | 处理 |
| --- | --- | --- |
| `[create random option level]`（固定 **3** 个数，如 `23 4 16`）| 13 | 新增 `RandomOptionLevel`，**原样保留**（语义未确立）|
| `[zero price drop]`（**无载荷**，后接 `[smart drop item]`）| 13 | 新增 `ZeroPriceDrop`，只记存在 |
| `[smart drop item]` 开合之间为空（组 `20363`）| 1 | **允许**（与副本脚本的空段同性质）|
| `[drop item]` 宽度为**奇数**（组 `21469`：11 个裸模板、无权重）| 1 | **跳过该组并报 id**（新的 `unreadable` 返回值）。邻组 `21468/21470/21471` 都写显式 `(模板, 1)` 对 ⇒ 这是**畸形**，不是第二形态；不猜权重 |
| 组 id 重复（`50013` 两块**逐字节相同**）| 1 | 相同 ⇒ 丢重复（与副本掉落表同策略）；不同 ⇒ 跳过并报 id |

⇒ **1,220 个组载入，只有 `21469` 被拒**。`[drop item]` 的主体确实是 `(模板, 权重)` 对：
`21468` 是 12 对 `(1041932x, 1)`；若读成"裸列表"，那 24 个数里会出现 12 个 `1`＝**金币卡**，显然不是。

### 30.3 改动清单

| 文件 | 改动 |
| --- | --- |
| `cmd/boosterexport/main.go` | 判定改**结构**（`[booster info]` 存在）；载入组表并**回填保留 id**；对"有 info 但解析不出池"的条目**报数**（此前静默丢 6,125 个）；非 `[booster]` 类型的结构容器写一条**无池条目**，让 `Container()` 看得见 |
| `cmd/wireprobe/booster_flow.go` | `Container()` 改为**目录成员即容器**，类型字符串降级为对旧目录的回退 |
| `internal/catalog/droptable.go` | 新增两段；空物品段允许；奇数段**跳过**；相同重复丢弃；`ParseDropGroups` 新增 `unreadable` 返回值 |
| `internal/catalog/loot.go` | 投影新增 `drop_groups_unreadable` |
| `configs/booster-catalog.json` | **重新生成**：42,504 条 / 83.98 MB（旧版 86.59 MB 备份在 `runtime/booster-catalog.before.json`）|
| 新夹具 | `internal/catalog/testdata/droptablebygroup_newsections.json`（15 组 / 574 cells，由 `runtime/groupfix` 从真表切出）|

### 30.4 验收

- `go build` / `go vet` 干净；**全量 21 包全绿**（4m26s）。新 exe = **`2ba4901da165…`**。
- 新增 `cmd/wireprobe/smart_drop_carrier_test.go`：两个载体各自开出 42/11 件、`draw=1`、首行权重正确；
  `Container()` 为真；**400 次开 `10416107` 从来没有把载体交给玩家**（共付出 113 种模板）。
- `internal/catalog/droptable_test.go` 新增 4 条：新段（含真数字）、奇数段跳过并报 id、空物品段允许、
  相同重复丢弃 / 不同重复跳过。
- 深渊门控集成测试全过；小深渊集成测试里已经能看到 21251 的成员落地
  （`100051304 / 100101187 / 100151128 / 100201100 / 100251140 …`）。
- 仍不可开的 6 个容器（`10417539/40/48/49`、`10420581/94`）在**新旧目录里都是 MISS**，
  属既有缺口（`[booster selection]`，需玩家自选），**不是本次引入**。

### 30.5 仍然未解

1. **物品中文名**：`[name]/[explain]` 是本地化键（`<13::name_10362480>`）。本机 `client/` 明文、PVF 字符串池、
   `-find`（只搜路径）三条路都 0 命中。⚠️ `internal/catalog/pvf/l10n.go` 是**为汉化补丁准备的底层能力**
   （枚举全部字符串引用点 + 向池尾追加文本 + 重写引用点 + 重新封装），**这是下一条值得走的路**：
   按 ID 定位 `[name]` 那个 token 的引用点，看它是否被补丁改写成了池尾的某个偏移。
2. 组 `21469` 的读法。
3. `[create random option level]` 三个数（`23 4 16` / `7 4 4`）的语义。

## 31. 【评估】`D:\115版本模块dump` 对我们有没有用（2026-09-27 09:5x）

业主拿到一份运行时模块 dump：主模块 `DNF.exe_主模块_0x140000000_285.7MB.bin`（说明：「64 位 PE，基址 0x140000000，
SizeOfImage=0x11DB9000，**运行时解密镜像**」）+ 78 个运行期模块。结论：**它不是我们这一版客户端，不能与既有 IDB 混用**。

### 31.1 两个 PE 不是同一构建

| | dump（`DNF.exe`） | 我们跑的 `client/DFO.exe` |
| --- | --- | --- |
| TimeDateStamp | `0x6AACE553` = 1789715795 | `0x6A969118` = 1788252440（**早 16.9 天**）|
| SizeOfImage | `0x11DB9000`（299.6 MB）| `0x10486000`（272.9 MB）|
| 节区 | `.text .rdata .data .pdata _RDATA .rsrc` **`.tvm0`** `.std ×3` | 同前 + **`.edata/.idata/.tls/.themida`** |
| 文件 | 299,601,920 B（= SizeOfImage，整镜像）| 258,972,712 B（**Themida 加壳**，TimeDateStamp 被清零）|

**RVA 不通用（已实测）**：我们在 IDB 里反汇编过的那个"故意空写陷阱" `0x146EA0C30`（§19.9 崩溃点，
应形如 `C7 05 …`＝`mov dword ptr ds:0,0`），在这份 dump 的同一 RVA 处是 **`74 05 40 32 ed eb 03 0f b6 e9 83 e1`** ——
完全不同的代码。⇒ **两者不能互换引用**。

### 31.2 内容探针：没有我们缺的东西

对 dump 与本地 exe 各做一轮字符串探针，**命中数逐项相同或同为 0**：

| 探针 | dump | exe |
| --- | --- | --- |
| `getPrimerGrade` / `getOathGrade` / `getMonsterGrade` | 0 | 0 |
| `smartdrop` / `SmartDrop` / `droptablebygroup` / `booster info` / `instantly` | 0 | 0 |
| `Oath` / `Primer` / `omen` | 7/4/2 | 6/4/2 |
| `10362480` / `21250` 等数字 | 0 | 0 |
| 中文（`深渊裂缝`/`神秘好运`/`征兆`/`星蕴石`，UTF-16 与 GBK）| 0 | 0 |
| RTTI `.cpp` 源文件名 | 17 种 | 19 种 |

⇒ 游戏的数据与文案都在 PVF 里，**不在主程序**；这份 dump **不会**给出物品中文名、也不会给出特殊事件的名字/表。

### 31.3 它真正有用的地方（以及没用的地方）

- **有用**：它是**运行时解密后的完整镜像**（我们的 exe 是 Themida 壳 + 时间戳清零）。若将来需要重做静态分析，
  一个未加壳的整镜像比壳文件更省事；也可作为**第二构建做差分**（同一函数两个版本对比，回答"官方改了什么"）。
  但**只有当我们关心"两个版本之间的差异"时才值得用** —— 关心"我们这一版为什么这样"时，仍应以 `client/DFO.exe` + IDB 为准（L0）。
- **没用**：78 个运行期模块里最大的是 `InitAceClient`（70 MB，ACE 反作弊）、`CSharpRailRegisterEvent`（WeGame Rail）、
  `OPLAT_BrokerPush`、gcloud voice 与系统 DLL ⇒ 与服务端复刻无关（不过它解释了客户端为何可能抵抗被改写文件）。
- **建议**：把它登记为**次级参考**，并在文档里写清两个构建的标识（时间戳 / SizeOfImage / 节区名），
  避免以后误用地址。若**玩家实际跑的是这份更新的构建**（而不是我们 `client/` 里这一版），那才是需要认真处理的问题 ——
  因为本轮所有实机结论都来自 `client/DFO.exe`。
## 32. 【复核】隐藏 BOSS 的档位门槛，以及「每次都出」的真实边界（2026-09-27 11:5x）

**缘起**：业主实机反馈「每次打天平都会打到最后的隐藏 BOSS，这是不对的 —— 隐藏 BOSS 代表必定出太初」，
随后追问「如果有誓约装备难道就一直有隐藏 BOSS 吗，感觉也不合理吧」。

> **先纠正一个容易走偏的前提：客户端不读装备。** §12 已证 `getPrimerGrade()` / `getOathGrade()`
> 读的是**服务端下发的玩家状态**（noti 2838 的 `[0:4)` = primer、`[4:8)` = oath）。
> §8.8 那场「脱掉誓约、天花板逐字不变」正是此因：当时服务端从未下发，值恒为兜底 `72`，
> 装备根本无从体现。⇒ 「有誓约就出」**不是**客户端的规则，是服务端选的策略。

### 32.1 只有档位「恰好 45」才召唤奥尔泰尔

§4b 的 `[ON DAMAGE]` 阶梯只在 **`oath_now < oath_max`** 时升级，所以能爬到哪一档、以及会不会召唤，
完全由 `oath_max` 决定：

| `oath_max` | 阶梯终点 | 会不会 `summon_orthaire` |
| --- | --- | --- |
| **45**（primeval 誓约） | `oath_now = 44`，此时 `44 < 45` **成立** ⇒ 进 `is_oath_epic_loop` 分支 | **会**（`c:nox_die == 0` 时） |
| 44（epic 誓约） | `oath_now = 44`，此时 `44 < 44` **不成立** ⇒ 不再进任何分支 | **不会** |
| ≤ 43 | 爬到 `oath_max` 就停 | 不会 |

这与 §11.2 的 `nox_index_checker` 独立吻合：`oath_max == 45 → nox_is_orthaire`；
`oath_max < 45 && primer_max == 45 → nox_is_wathcer`（另一个隐藏 BOSS「守望者」是它的互补分支）。
⇒ **rare(41) / unique(42) / legendary(43) / epic(44) 都不会出隐藏 BOSS。**

### 32.2 但 45 是「确定性」的：档位到位就**每场都出**

阶梯里没有任何随机数（`is_oath_*_loop` 是状态标志，不是掷骰）⇒ 同一角色只要 `oath_max == 45`，
**每次通关都会召唤奥尔泰尔**。⇒ 业主担心的「一直有」**成立**；
也就是说，「稀有」这件事**只能靠档位门槛表达**，客户端这一侧没有别的旋钮。

### 32.3 另一条断链：奖励表里根本没有誓约装备

拿 `configs/oath-grades.json` 的 189 个 ID 去 `configs/attunement-rewards.generated.json` 全表比对
（小深渊 `etc/rewardboostinfo/endkeeperoforder/normal.ctp` + 大深渊
`etc/rewardboostinfo/skyofathousandseasofborder/{unique,legendary,epic}.ctp`）：
**誓约 / 引子装备 0 件**。⇒ 按「按穿戴装备算档位」的规则，
**正常玩法里隐藏 BOSS 永远不会出**。两个极端都不对（要么场场出、要么永不出）。

### 32.4 档位映射（已落地：`internal/inventory/oath_grade.go`）

誓约 / 引子装备的 `[rarity]` 全量实测只有 5 档，与 `oathsystemscript.cos` 的 `[base rarity section]` 逐位对位：

`2 → 41 rare · 3 → 42 unique · 6 → 43 legendary · 4 → 44 epic · 8 → 45 primeval`

⚠️ **数值序 ≠ 机制序**（legendary = 6 排在 epic = 4 **之前**）⇒ **必须查表**，`40 + rarity` 会把两档换位。
一件对应装备都没穿 ⇒ `OathGradeNormal = 40`。生成器 `cmd/oathgradeimport`，产物 `configs/oath-grades.json`（189 件）。

### 32.5 本轮实测现场

- 角色 `test-xl`（id 11）发放前：`worn` = primer `100401640`(r3→42) + `100401592`×2(r2→41)
  ⇒ **oath = 40 / primer = 42**，按新规则不出隐藏 BOSS。
- 服务端 11:39 启动（exe `3b3d6c6f…`，含 §32.4 的档位换算），启动日志已打印
  `oath grades: derived from worn oath/primer gear (189 known items)`。
- 已用 `cmd/admin`（走既有事务 + 幂等 + 审计路径）发两件做 A/B，落 `inventory.equipment` slot 14 / 15：
  **`100610096`**（oath，rarity 8 → 45）与 **`100313751`**（oath，rarity 4 → 44）。

### 32.6 结论与待办（业主 2026-09-27 已拍板）

- **档位规则改为「进度制」**：每通关一次深渊累加进度，**攒满 N 次那一场才下发 45（必出一次）后归零**。
  这样保留「该出了」的保底语义（隐藏 BOSS = 必出太初），又不会场场出，也不依赖玩家去凑装备。
  **N 待定，建议 10。** 落地前先跑 §32.5 的 A/B（只穿 epic 应**不出**、换 primeval 应**出**），
  验证 §32.1 的阶梯判据。

### 32.7 实机复核（2026-09-27 11:5x）：A/B 前半通过；**誓约槽脱不下来**

- 11:49:44 下发的 noti 2838 载荷 = **`2a000000 2d000000`** ⇒ primer **42** / oath **45**
  （oath=45 只可能来自 rarity 8 的誓约 `100610096`）⇒ **隐藏 BOSS 确实出现** ⇒ §32.1 的判据实机成立。
- ⚠️ **客户端没有「誓约槽 → 背包」这个动作**：本场 4 条 `equipment_move_committed` 全部是
  「装备栏 → `worn`」（目标槽 `47` / `39` / `40`），而且**没有任何一次"脱"被服务端拒绝**
  （`refused` 类事件 0 条）⇒ 不是服务端拦的，是**客户端只能替换、不能清空**。
  ⇒ 玩家一旦穿上 primeval 誓约就**永久** `oath = 45`。
  ⇒ 这比 §32.2 描述的「每场都出」更糟，**进一步支持 §32.6 改用进度制**。
- 为验证阴性一侧，已直接改库摘下 `worn` 里的 slot 47（并把那件还回装备栏），改前原样备份在
  `runtime/_c11_backup_20260927-115338.json` 与 `D:/115us-backup/char11-state-20260927-115338.json`。
  改后 `worn` 无誓约、primer 最高 42 ⇒ 预期 2838 = `2a000000 28000000`（42 / 40）⇒ **不出隐藏 BOSS**。

### 32.8 落地：档位改为「通关保底」（2026-09-27 12:1x–12:3x）

业主拍板三项：**N = 5 场**、计数范围**只有小深渊 `100005014`**、旧的「按穿戴装备算档位」**退场**（只留诊断开关）。

**规则**
- **进本**（`finishDungeonLoading`）：该角色在该副本上的通关数 `>= 5` ⇒ 下发 `primer=40 / oath=45`（必出一次）；否则 `40 / 40`。
- **通关**（`completeDungeon`）：到阈值就**归零**，否则 `+1`。归零刻意放在**通关时**而不是进本时 ⇒ 掉线/退出不吞已攒场次。
- `primer` 恒 40 ⇒ 第二个隐藏 BOSS「守望者」（`oath_max < 45 && primer_max == 45`）**暂不出现** —— 它要另有一条保底。

**实现**
- `internal/database/oath_progress.go`：表 `character_oath_progress(character_id, dungeon_id, clears, updated_at)`；
  `MigrateOathProgress` / `OathProgressClears`（无记录 = 0）/ `BumpOathProgress`（角色行外 `FOR UPDATE`，到期归零）。
- `cmd/wireprobe/oath_progress.go`：`oathGradePrimeval = 45`、`oathDefaultProgressClears = 5`、`oathDefaultProgressDungeons = "100005014"`。
- `cmd/wireprobe/oath_info.go`：`oathInfoPackets()` 改为返回 error；`derivedOathGrades()` 走保底，
  `oathGradesForPity(due)` 是纯决策（到期 `(40,45)`、否则 `(40,40)`）。装备表只在 `-oath-grades-from-gear` 打开时加载。
- 开关：`-oath-progress-clears`（`DFO_OATH_PROGRESS_CLEARS`，默认 5）、
  `-oath-progress-dungeons`（`DFO_OATH_PROGRESS_DUNGEONS`，默认 `100005014`）、
  `-oath-grades-from-gear`（`DFO_OATH_GRADES_FROM_GEAR`，默认关）。
- 启动会打印 `oath grades: hidden-boss pity every 5 clear(s) of 100005014`；
  每次通关打印 `oath progress: dungeon <id> clears <a> -> <b> (pity every 5)`。

**验证**：`go build -p 1 ./...` OK；`go vet ./internal/... ./cmd/...` 干净；
`go test ./...`（日常；`-p 1 -count=1` 串行且禁 test cache，仅留给发布验证）**21 包全绿**；`CASH_INTEGRATION=1 go test -run TestOathProgressPity ./internal/database/`
**PASS**（真库：建表 / 新角色读 0 / 推进 `(0,1)→(1,2)→(2,0)→(0,1)` / 分副本独立 / 非法键拒绝）。
候选程序 `bin/wireprobe-handoff-source.exe` = `23adbe74…`（12:39，**待实机**）；旧的 `3b3d6c6f…` 备份在 `D:/115us-backup/bin-before-pity-20260927/`。

**尚未实机**：要打满 5 场，确认**第 6 场**出隐藏 BOSS、且该场通关后计数归零（下一轮从 1 开始）。

### 32.9 保底实机验收通过；以及隐藏 BOSS 的**产出缺口**（2026-09-27 13:0x）

**保底通过。** 会话 `..._20260927_125915_255302_next37` 的 `gateway.err` 有 6 条
`oath progress: dungeon 100005014 clears a -> b (pity every 5)`，逐条为
`0→1 · 1→2 · 2→3 · 3→4 · 4→5 · 5→0`，与 6 条 2838 载荷**逐条对上**：
前 5 场 `2a00000028000000`（40/40），**第 6 场 `2a0000002d000000`（40/45）** ⇒ 隐藏 BOSS 登场；
该场通关后计数归零。库里 `character_oath_progress = (11, 100005014, 0)` 佐证。

**⚠️ 但隐藏 BOSS 没有任何专属奖励**
- 小深渊表 `etc/rewardboostinfo/endkeeperoforder/normal.ctp` 的 **`hidden` 段是 `null`**
  （大深渊那三张 `skyofathousandseasofborder/*.ctp` 才有 `hidden` 段）。
- 服务端也没有「隐藏 BOSS 额外奖励」这条路径 —— 本轮只改了 2838 的档位。
- 实证：第 6 场与第 5 场的 `dungeon_clear_reward` **逐字节相同**（`7b1730…`），
  `dungeon_play_result` 只差通关耗时那个 u32。
⇒ **打死隐藏 BOSS 与普通通关同酬**；「没看到太初星蕴石」不是被崩溃遮住，而是本来就没有这条产出。

**📌「必有太初」的保底在征兆（omen），不在隐藏 BOSS**

小深渊 `coupons` 五行：阶段 0 `entries=[]`（**数据里就没有奖励条目**，与实测
`omen_clear{stage:0, gained:false, paid:false}` 吻合）；阶段 1/2/3 `obtainProb=10%`、`dropProb` 60/40/33%；
**阶段 4 `obtainProb=0`、`dropProb=1000000`（100%）必给 `10417571`**。
该盒子在 `booster-catalog.json` 里展开成 12 个候选（`100401598/602/606/610/614/618…`），
而 `configs/oath-grades.json` 里这一段 `[rarity] = 8` ⇒ **正是 primeval（太初）星蕴石**。
⚠️ 但 omen 石生成要求 `getEOOPartyOmenState() == 1`，而 **noti 2836 从未下发** ⇒ 阶段 4 走不到。

**❓ 结算后闪退（未定位）**
- Application 事件日志与 `%LOCALAPPDATA%\CrashDumps` 都**没有**新记录 ⇒ 客户端是**自己退出**，不是访问违例。
- 崩溃点在结算卡片界面：`card_layout_ack` 之后**缺** `card_inventory_committed` / `card_selection_ack` / `settlement_exit_ack`，
  且那一场**一次 `pickup_ack` 都没有**（普通场有 5 次）。
- **反证**：11:49 那场同样是 `oath=45` + 打死隐藏 BOSS，却**正常结算到底**
  ⇒ 不是 `oath=45`、也不是「打死隐藏 BOSS」的必然结果。需要复现才能定位。
- 已把 `character_oath_progress.clears` 从 0 **放回 5**（崩溃那场消耗了保底但玩家什么都没拿到），
  下一场会**再次**触发隐藏 BOSS，兼作复现测试。

## 33. 【取证】征兆 UI 是一类**注册式 popup window**；2836 的载荷读取点仍未找到（2026-09-27 13:2x）

**缘起**：业主「先把征兆的 UI 搞定」，之后要按官服重新对齐征兆玩法。

### 33.1 UI 是注册式 popup window（`analysis/dumps/xorstr_map.tsv`）

三个 popup 窗口类型名（UTF-16 字符串）：

```
0x1496535B0  POPUP_WINDOW_TYPE_ENDKEEPER_OF_ORDER_PARTY_OMEN_WINDOW
0x149653630  POPUP_WINDOW_TYPE_ENDKEEPER_OF_ORDER_OMEN_WINDOW
0x1496536A0  POPUP_WINDOW_TYPE_ENDKEEPER_OF_ORDER_BALLON_WINDOW
```

**注册点 `sub_141192BC0`**：把窗类名字符串存进全局并配一个 popup 类型 id ——
`qword_14E64B610` + `dword_14E64B618 = 0x0F8A`(3978)、`qword_14E64B620` + `dword_14E64B628 = 0x0F8B`(3979)。
另有两处**按名字注册进注册表对象**（`lea rcx,<registry>; call sub_140C59DA0(registry, key, value)`）：
`sub_1457A7110` → `qword_14E681AB8`；`sub_1412327C0` → `qword_14E64C3C8`。
两者都是**巨型生成式初始化函数**（栈帧 `0x351F8`），是 UI/字符串注册表，不是业务逻辑。

### 33.2 整组 UI 控件名（与三个 xui 一一对应）

`omen_start_%d` · `omen_loop_%d` · `omen_end_%d` · `omen_keep_%d` · `omen_slot_loop_%d` ·
`omenInfo_main` · `omenInfo_icon_%d` · `omenInfo_txt_%d` · `Omen_%d` · `omen_%d` ·
`box_normal_omen` · `box_mid_omen` · `omen_drop_process_on`（后者就是 `.act` 里那个变量名）。

xui 路径：`Contents/2026/EndKeeperOfOrder/Xui/{myOmen,partyOmen,balloonOmen}.xui`。

### 33.3 ⚠️ 修正 §12.7 的一条结论

§12.7 写的「2836 → classId `0x728` → `sub_144FC1120`」**不可靠**：
`sub_146E9F2A0(0x728)` 全库 **27 个调用点全在巨型分派器 `sub_146753320` 内**，而且是 **27 个不同的 case**
（case 2492 → `sub_146811120`；case 2875 → `sub_1415F9C10(obj, payload, 0xB3B, 1)` …）。
⇒ `0x728` 是**多个包共用的对象类**，不等于「2836 的专属 classId」。
分派器每 case 的形态是：`mov ecx,<classId>; call sub_146E9F2A0` → 判空 → `mov rdx,payload; mov rcx,obj; call <handler>`。

### 33.4 2836 的载荷读取点：仍未找到（已排除三条路）

- 2836 对象 vtable = **`off_14A6C0798`**（48 槽真 vtable；`off_14A6C0B60` / `off_14A6C0B90` 是**数据表**，槽里是随机 64 位值，不是函数指针）。
  该 vtable 的方法里**没有**任何读 `+350h`（载荷指针 `obj+848`）的指令。
- **控制组已跑**（技能 §7.4）：`sub_146752340` 里恰好 1 条 `mov [rsi+350h], r15` ⇒ `+350h` 过滤器有效，别处的 0 是真 0。
- `qword_14E64B610` / `dword_14E64B618`（含 `0xF8A`）**各只有 1 个 xref = 注册时那次写入，静态镜像里没有读者**
  ⇒ **打开征兆窗不走这两个全局**，而是走注册表对象。

### 33.5 `primerCollection.cos` 与征兆无关

`etc/115lvability2/primercollection.cos`（18 KB）= `[item exchange]` / `[item crafting]` / `[primer disjoint]`
（引子收集与合成），**不是征兆状态**；`contents/2026/endkeeperoforder/` 下**没有任何 `.cos`**。

### 33.6 下一步（按性价比）

1. **翻 `contents/2026/endkeeperoforder/` 的 `.act` / `.obj`**，看有没有 `[SHOW POPUP]` 类动作直接点名征兆窗 ——
   若有，**触发点是客户端脚本**，服务端只需喂状态（工作量最小，也最能解释「征兆窗什么时候弹」）。
2. **从注册表对象反查**：`qword_14E681AB8` / `qword_14E64C3C8` 的读者 = popup 管理器 ⇒ 打开征兆窗的调用形态。
3. **查 `0xF8A` / `0xF8B` 这两个 id 的使用点**（按立即数扫要注意假命中，§7.3 / §10）。
4. **钉死 `getEOOPartyOmenState` 的读取源**（VM 内置 2133–2137，`PrimerCollectionScript.cpp` 族）——
   这才是「征兆石为什么不生成」的正主，也是 2836 载荷几何的真正出口。

**产物**：`D:/115us-backup/ida-omen/`（IDB 副本 + 4 个脚本 + 4 份输出），**原库未动**。

## 34. 【取证·结论】征兆 UI 由 noti 2836 驱动；服务端只需喂状态（2026-09-27 13:3x）

**缘起**：业主「先把征兆的 UI 搞定」，并明确「4 阶段本身是官方设计，只是 4 阶段流转的模式需要对齐」。
本轮按 §33.6 的候选 ① 先翻 EOO 的 `.act`/`.obj`，结果**否掉了「脚本里有 `[SHOW POPUP]`」这条路**，
但顺着一张 popup 注册表把整条链路挖通了。

### 34.1 `.act`/`.obj` 里没有开征兆窗的动作（候选 ① 否）

`contents/2026/endkeeperoforder/` 全量 7,114 个文件：`.ani` 5,946 / `.als` 522 / `.act` **435** / `.obj` **88** /
`.lua` **11**（**全是怪物 AI**）/ `.xui` 4。其中 `passiveobject/` 下的 108 个 `.act`/`.obj` 全部 dump 后统计动作关键字，
**没有任何 popup / window / UI 类动作**（出现的是 `[BEHAVIOR]` / `[TRIGGER]` / `[NOTICE]` / `[CREATE ANIMATION OBJECT]` 之类）。

**但 dump 出了新线索**：`getDungeonFreeEntryClearCount()`、`getHellDungeonBonusItemMaxRarity()`，
以及成组出现的 `getEOOPartyOmenGrade(seat)` / `getEOOPartyOmenState(seat)`。
而且 `text_omen.obj`（`check_omen` / `make_text` / `wait_bosskill`）与 `effect_eoo.obj`
**读的正是那几个 omen 内置函数** ⇒ 它们在画**场内的**征兆表现，不是 UI 面板。

### 34.2 内置函数 id（`sub_147685170` 的注册表，逐块可读）

| id | 名称 | 名字串 |
| --- | --- | --- |
| `0x854`(2132) | `getDungeonFreeEntryClearCount` | `0x14B2CAFB0` |
| `0x855`(2133) | `getEOOOmenGrade` | `0x14B2CAFF8` |
| `0x856`(2134) | `getEOOOmenIndex` | `0x14B2CB020` |
| `0x857`(2135) | `getEOOPartyOmenGrade` | `0x14B2CB048` |
| `0x858`(2136) | `getEOOPartyOmenState` | `0x14B2CB080` |
| `0x859`(2137) | `isEOOPartyOmenUse` | `0x14B2CB0B8` |

实现（都读同一个单例 `qword_14E6388B8`）：

- `getEOOPartyOmenState` → `sub_1406578F0(s, seat)` = `*(u8*)(s + 20*seat + 112)`
- 「前导非零 u32 的个数」→ `sub_1406578B0(s, seat)` 数 `s + 20*seat + 96 + 4j`（j=0..3）
- `getEOOPartyOmenGrade` 家族 → `sub_140657720(s, seat)` = `*(u32*)(s + 176 + 4*seat)`
- `isEOOPartyOmenUse` = `sub_1406578F0(...) == 1`（`sub_1417FF2F0`）
- 四个包装函数都在 `if (*(u32*)sub_145B2DF00(ctx) == 212)` 里取值 —— **212 是 `[dungeon type]` 的枚举值**
  （`dungeon/endkeeperoforder.dgn` 里写着 ``[dungeon type] `endkeeper of order` ``），与服务端的副本号 `100005014` 无关。

### 34.3 ★ 载荷几何：noti 2836 = 4 × 17 字节 + 1 个尾字节 = 69

注册表 `sub_140657920` 只登记三个 id，且形态是 `lea r8,<handler>` **在前**、`mov edx,<id>` 在后：

```
0B16h -> sub_140656A00   (2838, 8 字节 -> 单例 +88/+92，即已修的 72 哨兵)
0B15h -> sub_1406568F0   (2837, 20 字节 -> sub_140658D80 + 单例 +760)
0B14h -> sub_140656A80   (2836, 69 字节 -> 征兆队伍状态)
```

`sub_140656A80` 第一件事就是 `sub_146EA0BE0(&buf, 69)`，随后按 **每条 17 字节、共 4 条** 解释
（`v12 = (char*)v12 + 17`、`v23 += 17`），**68 + 1 = 69** 与读取长度精确吻合。
17 字节 = **4 × u32 + 1 × u8**；尾部第 69 个字节（`HIBYTE(v47)`）是**标志位**：
`== 1` 时先用「旧状态」把可视化刷一遍再套用载荷，否则直接套用。

套用路径（对每个座位 i）：

| 载荷字段 | 去向 | 谁读 |
| --- | --- | --- |
| `u32[0]` | 槽 `+96 + 20i + 0` | `getEOOPartyOmenState` 家族 / 奖励预览（`sub_140283D60(manager, id, 1)`） |
| `u8[16]` | 槽 `+96 + 20i + 16` | **`getEOOPartyOmenState(seat)`** |
| `u8[16]`（本人） | 单例 `+192` | — |
| `u32[0]`（本人） | 单例 `+176` | **`getEOOPartyOmenGrade(seat)`** |

`sub_1406590A0` 里 `v6 = a1 + 4*(a2 + 4*(a2+6)) - a3` 展开即 `a1 + 20*a2 + 96`，
所以「座位 i 的槽基址 = `+96 + 20i`」是读代码算出来的，不是猜的。

### 34.4 值域由脚本坐实（grade ∈ 1..4 = 四档石头）

- `effect_eoo/action/basic.act`：`getEOOPartyOmenGrade(t:seatIndex()) == 1 / 2 / 3 / 4`
  → 播 `omen_effect_1_start` … `omen_effect_4_start`；
- `omen_drop_1/action/basic.act`：`state == 1` **且** `grade >= 1` → 给该座位出征兆石；
- 目录 `passiveobject/omen_drop/omen_1..4` 正好是 unique / legendary / epic / primeval 四档。
- `free_noti/action/basic.act`：`c:primer_die == 1` 且 `getDungeonFreeEntryClearCount() == 29`
  → 对本人弹 `[NOTICE]`（引用 `<9::Notice_End_Free>`）。

### 34.5 ★★ 开窗是客户端自己的事：服务端既不需要也无法"打开"UI

全代码扫立即数 `0xF8A..0xF8D` 后：

- **`sub_140658530` = 开窗**：`if (当前副本类型 == 212) { 0xF8C 未开则开; 0xF8B 未开则开 }`，然后清空 4 个座位槽；
- **`sub_140658690` = 关窗**：`0xF8C / 0xF8B` 已开则关。
- 这两个函数**只被数据引用** —— `0x1492D76D0` / `0x1492D76D8`，正是 EOO 单例 vtable
  `off_1492D76A0` 的 `+0x30` / `+0x38` 槽 ⇒ 它们由**框架按内容生命周期自动调用**，没有业务代码去"开窗"。

窗口 id（`sub_141192BC0` 注册）：**0xF8A** / **0xF8B = PARTY_OMEN** / **0xF8C = OMEN(个人)** / **0xF8D = BALLON**。
`sub_140658720`（2836 处理器的下游）在本人座位变化时直接更新 **0xF8C / 0xF8B** ——
它取的是 UI 容器对象并写 `a1[21*v5 + 197]` 这样的 4 个槽位，与 `myomen.xui` 的 4 槽布局一致。

⇒ **结论：服务端要做的事只有一件 —— 发 noti 2836。** 客户端进 EOO 副本时会自己把窗口开好，
收到 2836 就把每个座位的状态填进去；场内的 `effect_eoo`、征兆石生成读的也是同一份状态。

### 34.6 本轮产出的代码（诊断注入，默认关）

`cmd/wireprobe/omen_info.go`（+ 5 条测试）：

- `omenInfoPayload(seats, states, flag)` — 按线格式拼 69 字节；
- `parseOmenInfo(spec)` — 两种写法：**138 个十六进制字符**（原始载荷），或
  `"2,0,0,0,1;0;0;0;0"` 这类可读写法（4 个座位段 + 尾标志段，座位段 = `u32,u32,u32,u32,u8`）；
- `worldSession.omenInfoPackets()` — 在副本加载应答里下发（与 `oathInfoPackets` 同一时机、同一位置）；
- 开关 `-omen-info` / `DFO_OMEN_INFO`，**默认空 = 不发**；启动会打印注入的十六进制以便核对。

**为什么先做成注入器**：官方那套「4 阶段如何流转」还没对齐，而 17 字节记录里
`u32[1..3]` 的确切语义只证到「参与前导非零计数」这一步。用它先把 UI 点亮、并把字段语义实测钉死，
等玩法对齐后再把 `parseOmenInfo` 换成真正的状态机。

### 34.7 顺带取到的事实

- `dungeon/endkeeperoforder.dgn`：`[clear condition] [hunt boss] 109019266 1`（通关 = 打死天平）；
- `[maze chance rate]` **两个值 992857 与 7143**，合计恰 **1,000,000** ⇒ 这是两段迷宫的选取概率（候选 ③ 有解了）；
- `[dungeon type]` = "endkeeper of order"、`[minimum required level]` 115、`[recommended level]` 115 115；
- `[pathgate object]` 10 个 id（`109084219..109084226`）、`[normal group index] 1 21251 1 21476`（**21251** 又是那个掉落组）。

**产物**：`D:/115us-backup/ida-omen/`（IDB 副本 `omen.i64` + 10 个脚本 + 10 份输出），**原库未动**。

## 35. 【更正·取证】征兆的「1 阶段结算」发生在**天平**，而且整条掉落链是**客户端脚本**驱动

**缘起**：业主更正 —— 「其实 UI 正确和玩法联动，**在天平的时候会结算 1 阶段奖励**」。
（这同时解释了上一轮那句「在阶段中就结算了，我不确定是征兆几阶段」。）

**取证入口**：`monster/named/scale_primer/action/primer_proc.act`（天平自己的 proc，2,093 cells）
—— 它是全 EOO 目录里唯一大量提到 omen 的脚本（41 处）。征兆的掉落过程 `omen_drop_process` 就在这里。

### 35.1 启动点：天平被打 + 稀有度阶梯到顶 → `start_omen_drop_process`

```
[TRIGGER] omen_drop_process_trigger   [ENABLE] ON
    [COMPARE VAR] c:primer_rarity_progress_now >= o:omen_drop_process_rarity
    [ON DAMAGE] [WHICH MONSTER] IS INDEX 109019266          ; 就是天平
                [IS ETC ACTION OR] 10 11 12 13 14  [CHECKED NO] > 1
        -> [UPDATE VAR] start_omen_drop_process
               c:omen_drop_process_ing = c:omen_drop_process_cnt_max
               c:omen_drop_process_on  = 1
        -> [DO BEHAVIOR NAME] ME end_omen_drop_process_trigger
               [HP LIMIT ON] 1 PERCENT                      ; 天平被打到 1%
               [SET TRIGGER ENABLE NAME] END_TRIGGER ON
```

**注意它的语义**：`o:omen_drop_process_rarity` 是**触发阈值**（和 `c:primer_rarity_progress_now` 比大小），
不是奖励的稀有度 —— 它决定「天平被打到阶梯的哪一格时开始掉征兆」。阈值由**客户端自己掷**，按 primer 档位分派三条：

| 宏 | 取值 |
| --- | --- |
| `set_omen_drop_process_rarity` | `o:omen_drop_process_rarity = rr(40, c:primer_rarity_progress_max)`（`primer_max < 70`）|
| `set_omen_drop_process_rarity_rainbow1` | `rs(40, 70)`（`primer_max == 70`）|
| `set_omen_drop_process_rarity_rainbow2` | `rs(40, 70, 71)`（`primer_max == 71`）|

分派条件与 §20 的阶梯一致。⚠️ **这条不是奖励掷骰**，只是「打到第几格开始掉」，别和 §24 的
`roll(1e6)`（`[coupon drop table]` 三选一）混为一谈。

### 35.2 石头由 `omen_drop_maker` 生成，门槛就是 `getEOOPartyOmenState/Grade`

`passiveobject/omen_drop_maker/action/basic.act`：`c:omen_drop_process_on == 1` → `go_next`；
`make_omen_gem.act` 在 frame 1..4 逐级判定

```
getEOOPartyOmenGrade(o:targetindex) >= N   (N=1..3；N=4 是 == 4)
且 getEOOPartyOmenState(o:targetindex) == 1
且 t:seatIndex() == o:targetindex                       ; 只画自己那一份
   -> [CREATE PASSIVEOBJECT] INDEX 109137366 + (N-1)     ; 四档石头
```

`109137366/367/368/369` = 四档石头对象。`omen_drop_1..4/action/init.act` 里写死了
`[WHICH MONSTER] IS INDEX 109019266`，即**石头飞向天平**并撞击；`basic.act` 再用
`getEOOPartyOmenState/Grade` 复核一次，不成立就 `destroy`。

### 35.3 为什么"天平那一下"必须等掉落做完

`process_end_flag.act` 维护两个计数器：

```
c:omen_drop_process_cnt_now += 1        ; 每颗石头处理完
c:omen_drop_process_ing     -= 1
```

而放行"结束"的 `END_TRIGGER`（`end_trigger_on`）条件是**四条同时成立**：

```
c:primer_rarity_progress_now >= c:primer_rarity_progress_max
c:oath_rarity_progress_now   >= c:oath_rarity_progress_max
c:omen_drop_process_cnt_max  <= c:omen_drop_process_cnt_now    <-- 征兆掉完
[CHECK TIME EX] o:die_delay_final
```

⇒ **必须先走完征兆掉落，才能触发 `die_trigger_on`。** 这正是"在天平的时候结算"的机制含义：
天平被打到顶 → 掉征兆 → 掉完才允许结束 → 再按 §32 的阶梯决定隐藏 BOSS。

另有 `end_trigger_on_quick`：本人 `getEOOPartyOmenState(t:seatIndex()) < 1`（**没带征兆**）时直接
`[HP LIMIT ON] 1 PERCENT` + `END_TRIGGER ON`，跳过掉落 —— 所以**空手打**是快路径，
这也解释了为什么"没征兆的那几场很快就结算了"。

### 35.4 `cnt_max`（掉几颗）也是客户端自己数的

```
[WHICH PASSIVE] [IS INDEX] 109134988 109137638 109137641 109137642
[BEGIN IF] [CHECKED NO] == 1 -> omen_drop_user_1 -> c:omen_drop_process_cnt_max = 1
           == 2 -> user_2 -> 2      == 3 -> user_3 -> 3
           == 4 -> user_4 -> 4      == 0 -> user_0 -> 0
```

`[CHECKED NO]` 是"命中该 checkup 的对象个数" ⇒ **地图上这 4 类标记对象的数量 = 本次掉几颗**。
⚠️ 这 4 个索引**不是 PVF 文件名**（全库按名搜 0 命中），它们在别处定义，本轮未定位；
但可以确定：**"掉几颗"由客户端数出来，不由服务端下发**。

### 35.5 ⇒ 对服务端实现的三条直接后果

1. **`rr(40, primer_max)` 是客户端的掷骰**，但它掷的是「掉落从阶梯哪一格开始」，**不是奖励稀有度**。
   已复核本目录所有 `rr(` 用法：`primer_proc.act` 的 8 次是 `o:delay_time` 的抖动，`omen_drop_N` 各 2 次是位移动画
   ⇒ **奖励侧没有第二处掷骰**。§24 的 `roll(1e6)`（`[coupon drop table]`）与这条触发链**是两件不同的事**，
   服务端那套仍然只在「结算时给什么」这一层起作用。
2. **整条掉落链的门槛是 `getEOOPartyOmenState/Grade`（= noti 2836）。** 不发 2836 ⇒ `state=0/grade=0`
   ⇒ `make_omen_gem` 一颗石头都不生成 ⇒ 天平照旧走 `start_omen_drop_process`（`cnt_max=0`、`cnt_now=0`）
   ⇒ **"进程跑了但里面是空的"**。这与实测 `omen_clear{held:0, stage:0, gained:false}` 完全一致。
3. ✅ **服务端的结算挂钩点本来就在"天平那一下"，不需要改**（此条为对 §35 初稿的**自我更正**）：
   `internal/loot/session.go` 的 `Session.Death` 里，征兆推进的条件是

   ```go
   if s.Attunement.Enabled() && d.Definition.SourceBoss != 0 &&
      monster.Rank == 3 && monster.Template == d.Definition.SourceBoss && !s.attunementRolled {
           ... s.Omen.Advance(...) ...
   }
   ```

   而 `DungeonDefinition.SourceBoss` 正是从副本脚本的 `[clear condition] [hunt boss] <模板>`
   解析出来的（`internal/catalog/dungeons.go:176`），小深渊的值就是 **109019266（天平）**。
   ⇒ **天平确认死亡的那一刻就结算了**，与客户端 `start_omen_drop_process` 同一时点，
   **和业主说的"在天平的时候结算"完全一致**。（上一稿我误以为它挂在 `completeDungeon()`。）

### 36. 【更正·实机】「档位」= 该座位记录里**非零 u32 的个数**，不是 u32 的值（2026-09-27 14:3x）

**缘起**：业主看到场上 4 颗不同颜色的石头 + 征兆 UI，问「当前是哪个档位，我看似乎是 4 档征兆」。
一查，**业主是对的**，而 §34.3 / §34.4 把访问器认错了。

#### 36.1 记录的真正语义

`record = [4 × u32][u8]`（17B）：

- **4 个 u32 = 该座位持有的征兆 ID（最多 4 个）**。ID 会被 `sub_1406590A0` / `sub_140658D80`
  拿去 `sub_140283D60(qword_14E683B38, id, 1)` 查表（奖励预览向量写在单例 `+208+88j`）。**不是档位**。
- **u8[16] = 状态字节**，进单例 `+112+20*seat`（`sub_1406578F0`）；`== 1` 时 `isEOOPartyOmenUse`（2137）为真。

#### 36.2 脚本里的 grade（1..4）= 持有数

`getEOOPartyOmenGrade(seat)` 的实现是**数该座位槽里前导非零 u32 的个数**：

```c
// sub_1406578B0
v2 = a1 + 20*seat;
while (*(u32*)(v2 + 4*j + 96) != 0) { ++count; if (++j >= 4) return 4; }
return count;   // 0..4
```

而 `+176 + 4*seat`（`sub_140657720`）存的是**征兆 ID 数组**（`sub_140658D80` 从「本人那条记录」
的 4 个 u32 拷入）—— §34.3 把它当成 grade 访问器是**错的**。

这条更正解释了脚本里全部用法：`make_omen_gem.act` 的 `grade >= 1 / >= 2 / >= 3 / == 4` 四帧
（每帧造一颗石头）、`effect_eoo` 的 `grade == N` 选 `Symptom_N/*Symptom_00.ani`、
以及 tooltip 的「1–3 个三选一 / 4 个 100%」。

#### 36.3 实机证据（业主截图 + 事件日志）

- 本场（会话 `..._20260927_143712_703750_next37`）下发的 2836：**4 条记录各 `u32=(1,1,1,1)`、
  state=1、flag=0** ⇒ 每个座位「持有数」= **4** ⇒ **4 档**。
- 客户端表现吻合：场上**四颗石头**（unique / legendary / epic / primeval 全出）、屏幕特效取
  `Symptom_4/PrimevalSymptom`、征兆 UI 落在 **第 4 格**。
- 同场天平最后掉 3 件（`10362432` 银币 ×2 等），**没有**征兆石 —— 石头是客户端 `omen_drop_maker`
  造的 passive object，飞向天平后由 `omen_drop_process_cnt_now` 计数，**不进普通掉落列表**。
  **4 档 = tooltip 的「100% 必给」档**；服务端侧保底是否兑现另算（§35.5）。

#### 36.4 怎么指定档位（诊断注入）

档位 = 该座位记录里**非零 u32 的个数**，要几档就写几个非零：

| 档位 | `DFO_OMEN_INFO`（4 个座位写一样，免得受座位号影响）|
| --- | --- |
| 1 档 | `1,0,0,0,1;1,0,0,0,1;1,0,0,0,1;1,0,0,0,1;0` |
| 2 档 | `1,1,0,0,1;1,1,0,0,1;1,1,0,0,1;1,1,0,0,1;0` |
| 3 档 | `1,1,1,0,1;1,1,1,0,1;1,1,1,0,1;1,1,1,0,1;0` |
| 4 档 | `1,1,1,1,1;1,1,1,1,1;1,1,1,1,1;1,1,1,1,1;0` |
| 无 | `0;0;0;0;0` |

**预测（用于交叉验证）**：1 档应只见 **1 颗**石头（`SymptomUniqueStoneStart`）、屏幕特效
`Symptom_1/UniqueSymptom`、UI 亮**第 1 格**；4 档则四颗全出、亮第 4 格。
⚠️ 若 1 档仍出 4 颗，说明「四颗」不是 `make_omen_gem` 造的石头，需重查 —— 这也是这条更正的判别实验。

### 37. 1 档实机复核通过；以及「太初星蕴石」来源的两处更正（2026-09-27 14:5x）

**业主验证**：把注入值换成 1 档（`u32=(1,0,0,0)` / state=1）后 —— **场上确实只有 1 颗石头**，
征兆 UI 只亮**第 1 格**。⇒ §36.2 的「档位 = 每座位记录里非零 u32 的个数」**两端都实机成立**
（1 档 1 颗 / 4 档 4 颗），`>= 1 / >= 2 / >= 3 / == 4` 这条 `>=` 阶梯不需要再逐档验证。

**更正 ①**：此前把本轮掉的 `100401620` 说成「太初星蕴石」是**错的**。查 `oath-grades.json`：
`100401620` 与截图 tooltip 里的 `100401608` 都是 **rarity 3（unique，机制档 42）** 的引子，不是 rarity 8。

**更正 ②**：「rarity 8（太初）引子 = 征兆阶段 4 保底盒专属」也**不准确**。全库恰好 **12 件** rarity 8 引子
（`100401599/603/607/…/643`），它们是**很多盒子**的候选：

| 载体 | 说明 |
| --- | --- |
| `10417571` | 征兆阶段 4 的必给盒：**唯一一个池 = 12 件 rarity 8，权重 100%**（`12000/12000`）⇒ 开出来必是太初 |
| `10416110` / `10416118` | 小深渊普通奖励盒：3 个池，其中 **pool1 = 12 件 rarity 8**（另外两池是普通物） |
| `10416119` / `10416120` | 小深渊普通奖励盒：**pool1 里 rarity 8 只占 0.15%**（`1500/1,000,000`），多数是 rarity 2/3 |
| `10415194` / `10417807` / `10417885` / `10416127..132` / `10416140` | 同样是候选 |

⇒ 「太初」既有**必给**的口子（征兆阶段 4），也有**普通奖励盒里按权重**的口子；
「隐藏 BOSS / 征兆 = 太初的唯一来源」这个说法**不成立**（§32.9 的「隐藏 BOSS 无专属奖励」仍然成立）。

**本轮天平（entity 0x100B）实际掉 5 件**，来源都能对上（走的是深渊普通奖励盒的展开链）：

| idx | template | 是什么 | 来源佐证 |
| --- | --- | --- | --- |
| 2 | `0` + 3358 | 金币 | — |
| 3 | `10362432` | Merchant Guild Silver Coin | `10416103` 的候选里有它 |
| 4 | `100401620` | 引子（rarity 3） | 由奖励盒展开 |
| 5 | `1` | `stackable/coin.stk`（硬币） | `10416752` 的候选 = `[1]` |
| 6 | `102030802` | fighter 武器（boxglove） | 由奖励盒展开 |

**仍缺**：无数组 `u32`（征兆 ID）该填什么 —— 现在填的 `1` 只是占位。真实实现要填**玩家实际持有的征兆 ID**
（ID 会被 `sub_140283D60(qword_14E683B38, id, 1)` 查表做奖励预览），这要等「征兆怎么获得」的玩法定下来。

## 38. 【官服规则】征兆系统的完整玩法（业主 2026-09-27 提供）与实现差距

> 本节是**外部权威输入**（业主从官服/玩家社区取得），不是我们的推断；下面凡是「客户端的表」都是我实测的，
> 两者**逐项吻合** —— 所以这一节可以作为实现规格。

### 38.1 官方规则（原文要点）

1. **无征兆通关**：有概率激活第一个征兆（**第一个永远是「神器」**）。
2. **持有征兆通关**：从三种效果里随机触发一种 —— ① 无事发生；② **额外激活更高品质的 1 个征兆**
   （神器→传说→史诗→太初）；③ **获得征兆奖励并重置征兆**。
3. **满 4 个（神器~太初全激活）通关**：**直接结算奖励**。
4. **奖励可以兼得**：结算时按**已激活的每个阶段**各给 1 个 —— 例：激活神器+传说+史诗时结算 ⇒ 三段奖励各 1 个。
5. **进入异空间**：在常规爆装之外**额外掉 2 个光辉灵魂结晶**。（社区 1710 次 → 17 次异空间 ≈ 1%）
6. **幸运事件**（彩虹柱子）：小幸运 **×15**、大幸运 **×50**。（1710 次 → 小幸运 7、大幸运 1）
7. **特殊商店**：本次统计 10 种（小鸟票 14 次/100000 … 传说玛虎 6 次/350000），体验服无黑钻故只有 1 格。

### 38.2 官方奖励表 ↔ 客户端的 `[coupon drop table]`（逐位吻合）

| 阶段 | 官方奖励（件数）| 客户端行 | obtainProb | dropProb | 条目数 | 主奖励盒 → 内容（实测展开）|
| --- | --- | --- | --- | --- | --- | --- |
| — | （无征兆时只有「激活」）| 行 0 | 100000(10%) | 0 | **0** | — |
| **神器** | 星蕴石 + 套装星蕴石自选礼盒 + 神器~太初星蕴石自选套装罐子 = **3** | 行 1 | 10% | 600000(60%) | **3** | `10416150` → **12 × rarity 3（神器）** |
| **传说** | 同上结构 = **3** | 行 2 | 10% | 400000(40%) | **3** | `10417545` → **12 × rarity 6（传说）** |
| **史诗** | 星蕴石 + 史诗~太初星蕴石自选套装罐子 = **2** | 行 3 | 10% | 330000(33%) | **2** | `10417552` → **12 × rarity 4（史诗）** |
| **太初** | 星蕴石 = **1** | 行 4 | **0** | **1000000(100%)** | **1** | `10417571` → **12 × rarity 8（太初）** |

三条独立对位：**件数 3/3/2/1 相等**、**主奖励盒内容的稀有度恰好是该档**、**行 4 的 100% = 「满 4 个直接结算」**。
且**每行 `obtainProb + dropProb + 余数` 恰好 = 1,000,000** ⇒ 两列是**互斥三选一的靠前两段**，第三段是「无事发生」——
这与官方规则第 2 条和游戏内 tooltip 完全一致。

### 38.3 与现有实现的差距

现有 `internal/loot/omen.go` 的 `AdvanceOmen` 已经实现了「三选一 + 行号 = 持有数 + 归零」，**但有 4 处与官方不符**：

| # | 差距 | 现状 | 官方 | 影响 |
| --- | --- | --- | --- | --- |
| 1 | **结算不兼得** | 只抽当前行一次 ⇒ 发 **1** 件 | 对**已激活的每一档**各抽一次 ⇒ 发 **N** 件 | 严重少发（满 4 档：1 件 vs 4 件）|
| 2 | **持有数不持久** | 内存账本（重启归零）| 「持有征兆」跨场次持续 | 玩家攒不起来 |
| 3 | **noti 2836 未接正式状态** | 只有诊断注入固定值 | 按真实持有状态下发 | 客户端显示不出真实档位 |
| 4 | **满档判定** | 走同一条 roll 分支（靠行 4 的 100% 间接实现）| 官方明说「满 4 直接结算」| 结果等价，但语义应写显式 |

### 38.4 待业主拍板（实现规格里唯一还开放的部分）

- **A. 「征兆」是物品还是纯状态？** 官方文案像「携带 Omen of Order」，且 2836 的 4 个 u32 是 **ID 数组**
  （客户端拿去做奖励预览）。⇒ 走「真实物品」更贴官方，但要先定物品 ID；走「纯服务端状态」实现最快。
- **B. 隐藏 BOSS（oath=45）与征兆的关系**：现在是「通关保底 N=5」（我们自定）。有了官服征兆规则后，
  是否改成**与征兆挂钩**（例如满档结算后 / 太初档触发时才可能出隐藏 BOSS）？这是「推进模式重设计」的核心。
- **C. 异空间 / 幸运事件 / 特殊商店**要不要这一期做？各自都缺表：
  - 异空间：`.dgn` 的 **`[maze chance rate]` = 992857 / 7143**（合计 1e6）⇒ **0.7143% 特殊迷宫**，
    与社区实测 17/1710（≈1%）同量级，**很可能就是异空间的入口概率**（待验证）；还需定「光辉灵魂结晶」的物品 ID。
  - 幸运事件：乘以倍数（×15 / ×50）作用在掉落上，需要找到该事件的表/触发点。
  - 特殊商店：仓库里已有商店系统，需要定「特殊商店」的商品池与刷新规则。

## 39. 【定案 A2/B1/C1 + 落地】征兆 = 角色存档级状态；隐藏 BOSS 由满档结算驱动

> 业主 2026-09-27 15:1x 对 §38.4 的三个开放项拍板：
> **A2** —— 征兆**不是道具**，是**存档级别的占位标记**（绑定角色）；做成服务端会话状态
> 会变成「角色共享」，不对。
> **B1** —— 隐藏 BOSS（oath=45）改由**征兆满档结算**驱动，取代我们自定的「通关 N 场保底」。
> **C1** —— 这一期只做征兆三件套；异空间 / 幸运事件 / 特殊商店**列为待办**，等前面的
> 测试跑完再推进（见 §39.4）。

### 39.1 A2 有数据撑：全库没有一件「征兆」物品

把内层 PVF 的两张本地化文本表按**值**检索（`runtime/tablegrep`）：

| 检索 | 结果 |
| --- | --- |
| `Stackable.uv.str` 含 `Omen` | 85 条。除与「Camirak the Omen Bird（预兆之鸟）」重名的无关项，**只有 `Omen of Order Reward (CS)` / `Endkeeper of Order Omen reward`** —— 全部是**奖励盒** |
| `equipment.uv.str` 含 `Omen` | 0 条 |
| 两张表含 `征兆` / `徵兆` / `命運` | 0 条（表是英文源，没有中文键） |

而 noti 2836 记录里的 4 个 u32 也不是「拿在手里的东西」：它们会被
`sub_140283D60(qword_14E683B38, id, 1)` 拿去**查表做奖励预览**（§34.3）。

⇒ 官服的「携带 Omen of Order」= **角色存档里的一个标记**，UI 是客户端按 2836 画出来的。
四个档各对应**该档的奖励盒**，这就是 u32 该填的值：

| 档 | 表行 | 主奖励盒（该行 `[drop list]` 里权重最高的一条） |
| --- | --- | --- |
| 神器 | 1 | `10416150` |
| 传说 | 2 | `10417545` |
| 史诗 | 3 | `10417552` |
| 太初 | 4 | `10417571` |

`loot.AttunementRewards.OmenStageIDs` 从表里派生这四个值（不硬编码），集成测试
`TestOmenStageIDsMatchTheOfficialBoxes` 把它们钉死。

### 39.2 落地：一张存档表 + 两个时刻

新增 `character_omen_state(character_id, dungeon_id, held, orthaire_pending)`：

| 列 | 含义 | 写入时刻 |
| --- | --- | --- |
| `held` | 当前持有档数 0..4 | **天平死亡（结算那一刻）** —— 官方「结算征兆并重置」 |
| `orthaire_pending` | 「下一场该出隐藏 BOSS」 | 满档结算时置位；**通关确认之后**清除 |

**为什么拆两列**：两者归零的时刻不同。`held` 在结算那一刻就归零，而隐藏 BOSS 的机会
要到通关确认之后才兑现 —— 进本就清会让掉线/退出吞掉已经攒到的那一次奥尔泰尔
（与 `oath_progress.go` 同一条教训）。两列各用一个**单列 upsert** 写，不用
「读整行 → 改一列 → 整行回写」，否则后写的那一列会把先写的抹掉
（`TestOmenStateColumnsAreIndependent` 就是这条判据）。

服务端这一半的链路（`cmd/wireprobe/omen_state.go`）：

| 时刻 | 动作 | 产物 |
| --- | --- | --- |
| 进本 loading | `loadOmenRunState` 读存档 | `w.omenHeldRun` / `w.omenOrthaierDue` |
| 同上 | `omen_info.go` 按 `held` 编载荷 | **noti 2836**：前 N 个 u32 = 各档奖励盒，state = 1 |
| 同上 | `oath_info.go` 按 `pending` 选档 | **noti 2838**：`oath=45`（召唤奥尔泰尔）或 normal |
| 天平死亡 | `noteOmenSettlement` 写回 | `held` 落库；满档额外置 `pending` |
| 通关确认 | `clearOmenOrthaier` | 清 `pending`（这次机会已经兑现） |

⇒ **2836 与 2838 从同一份会话状态推出来**，不再是一次注入、一次读库。

开关 `-omen-state` / `DFO_OMEN_STATE=1`（启动脚本已打开）。它要求 `-omen-rewards`
同时开着，否则**启动期硬失败** —— 阶段表才是推进持有数的那台机器，只开状态会让存档
永远停在 0，而现象只是「UI 一直是空格子」，很难查。

### 39.3 B1 带来的语义变化

| | 旧（09-27 上午） | 新（B1） |
| --- | --- | --- |
| 隐藏 BOSS 触发 | 通关 N = 5 场（我们自定） | **征兆集齐四档并在天平结算过**，下一场出 |
| 出处 | 无 | 官方四档 + §35「结算发生在天平」 |
| 旧开关 | `-oath-progress-clears` | 退化成诊断，仅在 `-omen-state` 关着时生效 |

### 39.4 C1 之后的待办（业主明示：先测完再推）

| 线 | 现状 | 缺什么 |
| --- | --- | --- |
| 异空间 | `.dgn` 的 `[maze chance rate]` = 992857 / 7143（合计 1e6）⇒ **0.7143%**，与社区实测 17/1710 ≈ 1% 同量级 | 入口判据（是这一条吗）与「光辉灵魂结晶」的物品 id |
| 幸运事件 | 疑似 `[additional drop table]` 的 effect 2（1.48%）与 fixed 里的 `luck15` / `luck30` | 触发点，以及 ×15 / ×50 作用在掉落的哪一层 |
| 特殊商店 | 仓库里已有商店系统 | 「特殊商店」的商品池与刷新规则 |

## 40. 【定案】本私服的掉落调参层（**与官服的显式差异**，2026-09-27）

> 业主定调：官方比例是按**长期反复刷取**的生态设计的，本服务器是**单人模拟端**，
> 玩家能刷的次数远少于官方生态，照搬会让「刷了很多场却什么都没拿到」变成常态。
> 两条改动 —— 征兆「无事发生」减半；fixed 池低档按比例向高档倾斜。
> 另：业主明确「不宜太极端」，所以倾斜幅度做成了可调参数（默认 25%），而不是「减半」。

### 40.1 为什么不能照搬官方

把 §38/§39 的表按 1710 场折算，官方生态下单场的期望是：

| 来源 | 单场期望 |
| --- | --- |
| fixed 落到普通 + 稀有 | **74.3%** |
| fixed 落到四档合计 | 25.4% |
| additional | 98.52% 是银币 |
| 征兆「无事发生」 | 低档位 90%（持有 1 时 30%）|

单人端每周的刷取次数比官方生态低一到两个数量级，「长期平均」根本轮不到兑现。

### 40.2 两条变换（`internal/loot/attunement_rebalance.go`）

| 变换 | 规则 |
| --- | --- |
| `OmenHalveIdle` | 每行「① 无事发生」的份额**减半**；腾出的份额在 ②（再激活一档）与 ③（结算并重置）之间**对半分**（奇数多出的 1 给 ③）。行 0 没有 ③（官方该行 `[drop prob]` 就是 0），所以全部并入 ② |
| `FixedTiltPercent` | fixed 池里**普通 / 稀有**的权重各减 N%，减掉的按**神器 : 传说 : 史诗 : 太初 的现有比例**补给高档 |

第二条里「按现有比例」是刻意的：四档被放大**同一个倍数**，所以稀有度阶梯的形状不变，
变的只是「好东西的总量」。**不采用**「减半」（会把神器推到 49%，等于废掉稀有档），
也不采用等分或「越稀有分得越多」（那会重写神器与太初的相对关系）。

两条变换都在**源校验之后**才动手：先证明「表读对了」，再谈「我们想改哪里」；
改完当场复核权重不变量（每份 `[drop list]` 与每组 `[select prob]` 仍恰好 1e6），
所以改完的表与源表在结构上同样合法。

### 40.3 征兆三选一：改前 / 改后

| 结算前持有 | 改前 无事 / +1 / 结算 | 改后 无事 / +1 / 结算 |
| --- | --- | --- |
| 0 | 90% / 10% / — | **45% / 55% / —** |
| 1 | 30% / 10% / 60% | **15% / 17.5% / 67.5%** |
| 2 | 50% / 10% / 40% | **25% / 22.5% / 52.5%** |
| 3 | 57% / 10% / 33% | **28.5% / 24.25% / 47.25%** |
| 4 | — / — / 100% | 不变（满档直接结算）|

### 40.4 fixed 池：倾斜幅度对照

| 倾斜 | 普通 | 稀有 | 神器 | 传说 | 史诗 | 太初 | 高档合计 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 0%（官方）| 47.07% | 27.20% | 20.00% | 3.60% | 1.60% | 0.15% | 25.4% |
| 15% | 40.01% | 23.12% | 28.79% | 5.18% | 2.30% | 0.22% | 36.5% |
| **25%（默认）** | **35.30%** | **20.40%** | **34.65%** | **6.24%** | **2.77%** | **0.26%** | **43.9%** |
| 35% | 30.59% | 17.68% | 40.51% | 7.29% | 3.24% | 0.30% | 51.3% |
| 50% | 23.53% | 13.60% | 49.30% | 8.87% | 3.94% | 0.37% | 62.5% |

这张表由 `TestRebalanceTiltTable` **直接打印**，不是手算；同一条测试还断言「幅度越大
高档越多」是单调的。

### 40.5 开关与作用域

- `-attunement-rebalance` / `DFO_ATTUNEMENT_REBALANCE=1`
  ⚠️ **2026-10-01 业主改主意：已关掉**（`configs/pvf-default.json` 里显式 `"0"`）。
  理由：业主实机发现「征兆几乎每场都出第一档、每次都结算」正是本层调参的结果，
  且拿 1710 场国服实测一比，**国服的稀有 27.7% ≈ 官方原表 27.20%** ⇒ 国服用的就是倾斜 0。
  本层保留为**可再打开的入口**（改 profile 即可），默认关。
- `-attunement-fixed-tilt N` / `DFO_ATTUNEMENT_FIXED_TILT`（默认 25；0 = 固定池不动；≥100 被拒）
- **默认关闭** ⇒ 表与官方逐字节一致，`TestRebalanceDisabledIsANoOp` 守这条
- 作用域天然只在小深渊：大深渊的三张表**没有普通 / 稀有档**
- 幸运事件的两个档（`luck15` / `luck30`）**不参与**倾斜，权重原样保留
- 启动日志会把改前 / 改后的每行数字打出来 —— 免得以后有人拿这份表去对官方数据时误判

### 40.6 幸运事件：不需要落地，它已经在跑（§38.4 里 C 的第二条）

「小幸运 ×15 / 大幸运 ×50」在服务端**没有一行专门代码**：

- 两个档就是 fixed 池里的 `luck15`（0.3333%）与 `luck30`（0.05%），与社区实测
  7/1710 = 0.41%、1/1710 = 0.059% 同量级；
- 盒子是 `10416119` / `10416120`，游戏名就叫 **"Endkeeper of Order Mystical Fortune … Reward (CS)"**；
- **倍数写在盒子里**：它们的 `pool1` 的 `draw` 分别是 **14 / 29**（普通盒是 1），
  所以抽中后走通用开箱路径就会一次开出那一堆 —— 不需要服务端另写逻辑；
- 它们**不在**启动日志的「本 build 打不开的盒子」名单里（那份名单是
  `10417539 10417540 10417548 10417549 10420581 10420594`）⇒ 已经能正常开。

⇒ 这一条**不是待办**；剩下的是实机确认，以及（可选的）「触发时给个信号」。

## 41. 【落地】异空间 = 小深渊 maze 1：按 `[maze chance rate]` 掷骰选图（2026-09-27）

> 业主探索后的三个决定：**① 触发概率放大到 2%**（官方 0.7143% 对单机端太低，与 §40
> 掉落调参层同一个理由）；**② 只做小深渊**（白名单，不动另外 66 个同样声明了该字段
> 的副本）；**③ 奖励与官服一致**（不加码）。

### 41.1 结论：不需要新表、不需要新物品

天平的掉落表里**本来就有异空间那一份** —— `[fixed drop table]` 是**按 maze 分段**的
（§38 已经读到这点，当时没意识到它的意义）。两张 maze 的 8 个奖励盒**只差第 0 池**：

| | map614（maze 0，普通）| map615（maze 1，**特别**）|
| --- | --- | --- |
| `pool0` | `10362432 × 2` = **Merchant Guild Silver Coin** | **`10415192 × 2` = Splendor Soul Crystal（光辉灵魂结晶）** |
| 地图路径 | `…/map/100016614_normal.map` | `…/map/100016615_special.map` |
| `[passive object]` | 11 个 | **12 个** —— 多一个 `109134975` @ (3168,190)（天平黑洞演出装置，§2）|
| 起点 `[dungeon start area]` | (498, 193) | **(443, 161)** |
| 背景层 | 25 层（多一层 `Background/Tile_Ex_2.ani`）| 24 层 |
| `[maze chance rate]` | 992857 | 7143 |

> ⚠️ 订正：§1b 当时只比了 `[monster]` / `[passive object]` **两个段**，所以说「差异只有一处」。逐 cell 全量比之后，两张地图还差**起点坐标**与**背景层**（普通图多一层 `Tile_Ex_2`）。所以实机有三件事可以当场确认进的是哪张图：**起点位置**、**天平的黑洞入场演出**、**掉落的材料**。

**社区口径「进异空间：在常规爆装之外额外掉 2 个光辉灵魂结晶」逐字吻合**：那 2 个材料
就是 `pool0` 的两件 —— 普通迷宫给银币，异空间给光辉灵魂结晶。概率也对得上：官方
0.7143% vs 社区实测 17/1710 ≈ 0.99%（泊松 95% 区间约 [6.3, 21.3]，17 落在里面）。

### 41.2 缺口只有一处：没人读 `[maze chance rate]`

`internal/dungeon/session.go` 的 `Select()` 原来按「同 quest 里 **index 最小者**」选图
（那条规则本身是对的，§1 记的 288 个多 maze 副本靠它保持稳定），结果是 **maze 1 永远
走不到**；`DungeonMaze` 也从来没有这个字段。

奖励侧不用改：`internal/loot/session.go` 早就是
`Attunement.Roll(result.NextSeed, d.Definition.ID, uint32(d.Maze.Index))`。

### 41.3 为什么必须白名单，而不是「通用支持」

扫 `dungeons.full.json`（它**保留了原始 cells**，可以直接用 Python 扫源真值）：
**67 / 3200 个副本**声明了 `[maze chance rate]`，而它们的**量纲并不统一**：

| rates 形态 | 副本数 | 合计 |
| --- | --- | --- |
| `[[90],[10]]` | 40 | 100 |
| `[[190]×5,[25]×2]` | 2 | 1000 |
| `[[3000]×3,[1000]]` | 1 | 10000 |
| `[[2375]×4,[250]×2]` | 2 | 10000 |
| `[[992857],[7143]]` | 1（小深渊）| **1000000** |
| `[[75000]×12,[8333]×11,[8337]]` | 3 | 1000000 |
| `[[0],[200000],[0]]` | 1 | 200000 |

「按权重归一化」只在**单个副本内部**成立。一次性给 67 个副本启用，等于替 66 个我们
没验证过的副本改行为 ⇒ 服务端**不替它们做决定**：加一张副本 = 往 overlay 加一行，
而不是一次全局行为变更。

### 41.4 实现

| 层 | 位置 | 内容 |
| --- | --- | --- |
| 数据 | `configs/dungeons.maze-chance-rates.json` | 只列 `100005014`：`source_rates [992857,7143]`（官方原值，只作对照）+ `rates [980000,20000]`（服务端实际用的 98% : 2%）|
| 生成 | `cmd/mazechanceimport` | 从 full.json 的 cells 读官方原值；`-maze 1=2%` 把指定 maze 拉到给定概率，**其余按源比例分剩下的份额**（与 §40「按现比例补给高档」同一原则）；取整误差补给最后一项，保证合计恰 1e6 |
| 加载 | `internal/catalog/maze_chance.go` | `AttachMazeChanceRates` 校验源 checksum、副本 sha256、权重个数、**至少两张可选**；`ReadMazeChanceRates` 读源真值 |
| 字段 | `DungeonDefinition.MazeChanceRates []uint32` | **不由通用解析填充** —— 只有白名单副本会被填，其余副本一个字节不变 |
| 选图 | `internal/dungeon/session.go` 的 `chooseMaze` | 有权重且候选 > 1 ⇒ 按候选集归一化掷骰；否则**原样**「index 最小者」。权重个数与 maze 数不符、或全零 ⇒ 也退回确定性规则（宁可保守，也不按错位的表掷骰）|
| 日志 | `cmd/wireprobe/maze_chance.go` | 启动时把「哪些副本按权重选图、每张多少」念出来 —— 权重是我们改写过官方的，不念出来以后对不上账 |

`source_rates` / `rates` 分成两个字段是刻意的：让「我们对官方数值做了什么改动」永远
只有一个 diff 的距离，而不是沉在一次性的生成脚本里。有一条测试专门守这一点。

### 41.5 诊断开关 `DFO_MAZE_FORCE`

2%（更别说官方那 0.7143%）靠手刷撞不到，而「进异空间会不会加载 special 地图、天平的
黑洞演出对不对、掉的是不是光辉灵魂结晶」必须能确定性复现一次：

```bash
DFO_MAZE_FORCE=100005014:1   # 本次启动永远进 maze 1（= 异空间）
DFO_MAZE_FORCE=100005014:0   # 反例：永远进普通图
```

它只作用于**已经启用权重的副本**，其它副本不动；验证完记得去掉。

### 41.6 实机验证清单

1. **强制进异空间**（`DFO_MAZE_FORCE=100005014:1`）：地图应是 `100016615_special`
   （天平有黑洞演出），天平掉落里应是 **2 个 Splendor Soul Crystal**（`10415192`）
   而不是 2 个银币（`10362432`）。
2. **反例**（`:0`）：普通图 + 2 个银币，与之前实机一致。
3. **概率**：去掉开关重启，确认启动日志打印
   `dungeon 100005014 rolls its maze by [maze chance rate]: maze 0 98.0000% · maze 1 2.0000%`。
   概率本身只能靠大量场次观察，不必手工统计。

### 41.7 仍然待办：特殊商店

线索已经拿到 —— `explain_10415183/184/185` 三个道具的文案是

> Use to open the **Scales UI**. You can consume Souls to operate the scales. You can
> obtain 1 guaranteed reward or 1 chance reward at a set rate, up to **9 / 19 / 39** times.

保底奖励分别是 Unique / Legendary / Epic Crystal，概率奖励含 Splendor Soul、
Endkeeper of Order Entry Material、Rare–Primeval Crystal ⇒ 天平还有一套「消耗灵魂
操作」的玩法，很可能就是官方说的特殊商店。

### 41.8 实机发现的第二处偏差：材料槽被「开箱」吃掉了（2026-09-27 17:3x）

强制进异空间的第一次实机：**地图对了**（星空背景 + 天平黑洞演出，`entry: maze 1 -> map 100016615`），
但把天平那一帧 `monster_death_confirmed` 解出来，掉的是

| template | 数量 | 物品名 |
| --- | --- | --- |
| 10362432 | 2 / 3 / 2 / 2 | Merchant Guild Silver Coin |
| 1 | 1 | （空槽）|
| **10415191** | **1** | **Splendor Soul** |
| 100101193 | 1 | 装备 |

期望是 **`10415192 ×2`（Splendor Soul Crystal / 光辉灵魂结晶）**。两个独立缺陷叠在一起：

1. **数量丢失**：`OpenRewardBoxes` 展开包装时**忽略 `Award.Amount`** —— 一份包装只开一次，
   所以「×N 的包装」只会发一份产物。（已修：按 `Amount` 开 N 次；`Amount 0` = 源里没写，按一份。）
2. **不该展开**：`10415192` 在物品目录里是 `stackable_type = "[booster]"`，被
   「是盒子就递归展开」的判据命中。但它其实是小深渊 maze 1 固定表的**材料槽** ——
   maze 0 的同一槽位是 `10362432`（银币），**那个根本不带 `[booster info]`**，一直按原样发。
   两张表在这一格上是镜像的（一个给银币、一个给结晶）⇒ 官方给的也是结晶本身，
   玩家在 Scales UI 里自己消耗。（已修：具名豁免 `payAsIsWrappers`。）

**为什么判据只能是具名**：全库 37,387 个 `[booster]` 可堆叠物里，**30,230 个没有
`stack_limit`**，其中既有真礼盒（`box_08sealed` / `booster_gate*` / `package_*`）
也有材料 —— 「能不能堆叠」与 `stackable_type` 字符串都分不开这两类，所以逐个列并写明依据。

同时确认**另外两个同形状的盒子展开是对的**，不要顺手改：

| template | 名字 | 池内容 | 结论 |
| --- | --- | --- | --- |
| `10416752` | Endkeeper of Order Oath Not Obtained | `[(1, 1000, 1)]` = 空槽 | 这是「本次没拿到誓约」的**空奖占位** ⇒ 展开成什么都没有才对 |
| `10418293` | Doom Oracle Orb (500) | `[(10362429, 1000, 500)]` | 名字里的 **(500)** 就是含量 ⇒ 展开是对的 |

## 42. 副本内「秘密商店」：**未随本 MR 提交**

`etc/secretshop.etc`（内层 PVF，422 KB）确认是**服务端数据**（客户端 114,614 条 xorstr 里
商店类文件名都在，唯独没有它），协议也已逆出来（noti 279/280、cmd 296/297），接线跑通过一半：
**商店会出现，但货架始终是空的**。

这条线**没有随本 MR 提交**（代码备份在本地），完整记录 —— 表结构 / 概率模型 /
两次实机 A/B / 窗口侧反编译拿到的两个时序与结构问题 / 下一步怎么打 —— 见：
**`secretshop-open-thread-20260927.md`**（§41.7 里那条 Scales UI 线索也归入这条线）。
