# 「调律之边界」掉落与 GO 下一轮：现状取证与实施计划

日期：2026-09-26（**勘察稿，尚未实机验证，未定 confirmed baseline**）
对象：千海之空（Sky of a Thousand Seas of Border）「调律之边界」`[dungeon type] boundary of attunement`
副本：`100005066` / `100005067` / `100005068`，地图 `100001016`（单图多轮）

外部参考（**只作结构对照，不作实现依据**）：

- `E:/下载/深渊进入下一入口修复记录.md`（来源 `F:\115-main\docs\调律之边界通关与GO继续挑战-20260920.md`，该工作副本在本机不存在）
- `E:/下载/国服掉落表打包.zip`（国服 `Script.pvf` build 2026-09-18，inner sha256 `cb5ad788…d88d38`）

---

## 1. 取证结论

### 1.1 我方客户端数据里，这个副本的掉落段是**完整的**

`bin/pvfinspect -source server/work/client-build/Script.inner.pvf -file "contents/2026/skyofathousandseasofborder/dungeon/orderoftheborder_epic.dgn"`
（内层 PVF，size 760530763，即 `dungeons.full.json` 的 `source.checksum 7ef2db59…d88e80`）

我方 `orderoftheborder_epic.dgn`（= 副本 `100005068`）含：

```
[dungeon type] `boundary of attunement`
[difficulty dropitem group list]
  [group info]
    [item index]           10326880 10326884 10420247 10419754
    [normal group index]   1 21251 1 21600
    [fame info]            0 0 0 0
    [special setinfo reward] 320 10326880 1 21279
    [special setinfo reward] 320 10326880 1 21468
    [special setinfo reward] 320 10326880 1 21470
    [special setinfo reward] 138 10326880 1 21310
    [reward multiple info] 3 1
  [group info] … [normal group index] 1 21251 1 21601 … [reward multiple info] 3 6
  [group info] … [normal group index] 1 21251 1 21602 … [reward multiple info] 3 12
[/difficulty dropitem group list]
[clear condition] [hunt boss] 109008634 1 [/clear condition]
```

国服 epic 的同段三段组（`21600/21601/21602`，倍数 `1/6/12`）**逐字段一致**；差异只在该副本的其它运行字段
（我方 `[maze chance rate] 990000` vs 国服 `1000000`；我方 `[worldmap pattern info] … 7` vs 国服 `6`；
我方 `[difficulty another dungeon] 100005066 100005067 100005068` vs 国服 `0 100005067 100005068`）。

⇒ **结论：不需要照抄国服掉落表。** 我方 PVF 自带同一套组表引用，实现应当直读我方 PVF。

### 1.2 我方服务端完全没有这套机制（这是"没落实掉落"的直接原因）

| 检查 | 结果 |
| --- | --- |
| `grep -rn dungeondroptablebygroup server/work/dfo-lan` | **0 命中** |
| `internal/loot/rules.go` `Parse()` | 只读 5 张**全局通用表**：`etc/itemdropinfo_monseter.etc` 的 `[drop prob]`/`[item drop ref table]`/`[monster type drop bonusrate]`/`[basis of rarity dicision]`，`etc/itemdropinfo_common.etc` 的 `[gold drop ref table]` |
| `internal/loot/rules.go` `Roll()` 自述 | 掉落模型 `reference90-gold-stack-v1`：按等级/稀有度在**角色可用装备池**里取，字典生成权重未恢复 ⇒ *"This is not official-server parity"* |
| 副本目录字段 | `dungeons.full.json` 的 `100005067`/`100005068` 均为 `HuntBoss: 0`（`HuntBoss` 只在 `Odyssey` 分支解析，见 `internal/catalog/dungeons.go:42`） |

即：**副本级的 `[difficulty dropitem group list]` / `[group info]` / `[normal group index]` /
`[special setinfo reward]` / `[reward multiple info]` 五段当前无人消费**；服务端对任何副本都只发"通用掉落"。

### 1.3 GO「进入下一轮」这条链在仓库里**完全不存在**

| md 所述 | 我方现状 |
| --- | --- |
| 装置 `109008634` 死亡 → `NOTI31` + `NOTI261`(单字节 `09`) 点亮右侧 GO | `NOTI261` 全仓 0 命中；`NOTI31`/通关走 `BossCheck`（CMD117）路径，`internal/dungeon/completion.go:10` 只认 boss 包 |
| 点 GO → CMD72 正文 `01 05 01` | `DecodeSettlementExit`（`internal/game/protocol/cards.go:61`）**要求 `option ≤ 3`**：`(p[0] != 1 && p[0] != 2) \|\| p[1] > 3` ⇒ `01 05 01` 直接 `unsupported exit action`，网关回 `SettlementExitRefused(5)` |
| 服务端只回 ACK72 `01 05`，等客户端自己发 CMD2062 | 未实现（option=5 到不了任何分支） |
| CMD2062 后按 `ACK2062 → NOTI2281 → NOTI27 → NOTI28 → NOTI2859 → NOTI29` 发 | `NOTI2281`/`NOTI2859` 0 命中；现有 `directMoveDungeon`（`cmd/wireprobe/dungeon_flow.go:226`）是"换一张新副本"的语义，不是"同副本下一轮" |

---

## 2. 缺口清单（按依赖排序）

1. **组表解析**：读 `etc/dungeondroptablebygroup.etc`（`[group] N` / `[creation rate] a b` /
   `[smart drop item] item weight …`）→ `group_id → (creation_rate, items[])`。
2. **副本掉落段解析**：把 `[difficulty dropitem group list]` 三段 `[group info]` 解析进 `catalog.DungeonDefinition`
   （新增字段，不动既有 `HuntBoss` 语义），字段：`item index` / `normal group index` / `fame info` /
   `special setinfo reward` / `reward multiple info`。
3. **发奖**：通关/回合结束时按组表掷取并落包（服务端权威：扣耐久/入包/存档在同一事务）。
4. **通关判定**：非 Odyssey 副本的 `[clear condition] [hunt boss] <template>`（本例为装置 `109008634`）要能成立，
   并驱动 `NOTI31`。
5. **GO 链**：
   - `DecodeSettlementExit` 放行 `option = 5`（需先确认客户端语义，见 §3 问题 3）；
   - option=5 分支：只回 ACK，不排包、不发卡、不切图；
   - 新增"等待 CMD2062 重入"状态；收到 CMD2062 时按 md 的顺序发 6 帧。

---

## 3. 待确认（阻塞项）

1. **「深渊」到底指哪个**：本记录按「调律之边界」`boundary of attunement` 推进；但掉落包里另有
   `etc/itemdropinfo_monster_hell.etc`（深渊派对怪掉落）、`etc/helldropepicitemtable.etc`（深渊史诗掉落表）、
   `etc/hellparty.etc`（深渊派对配置）这一整套 **深渊派对** 表。两者要不要都做，需用户明确。
2. **优先级**：先落"组表掉落"（§2.1–2.3）还是先落"GO 链"（§2.4–2.5）。
3. **CMD72 option=5 的客户端语义**：md 只说"点 GO 发 `01 05 01`"，未给客户端侧 handler 地址。
   按本项目纲领，客户端强制项应有 L0 证据（IDA）或实机帧日志；**建议先取一次实机帧**再改协议解码，
   否则属于猜包。
4. **实机现状**：用户现在进本后清怪，是否能看到通关结算？装置是否掉血/可击杀？有无任何掉落？
   这决定 §2.4 是"从零加"还是"只差一小段"。

---

## 4. 与国服对照时**不能照抄**的三处

- 副本 ID 映射不同：我方 `100005068` = `orderoftheborder_epic.dgn`，国服同段是 legendary；我方另有 `100005067`，
  且 `[difficulty another dungeon]` 引用的 `100005066` 在我方 PVF 中**不存在**。
- 我方 `[maze chance rate]`、`[worldmap pattern info]`、`[cost]`（`10419203 3` / `10362429 750`）均与国服不同。
- 国服包里的 `rewardboostinfo/*.ctp` 三个文件我方包内未解析出文本（0 字节），而本副本脚本**未**声明
  `[reward boost dungeon]` 内容 ⇒ 不能把国服的 boost 表直接套上。

## 5. 2026-09-26 落地进展（本轮）

### 已落地（数据面；未提交，未实机）

- `internal/catalog/droptable.go`（新增）两个解析器：
  - `ParseDropGroups(cells)` —— 把 `etc/dungeondroptablebygroup.etc` 解成 `[]DropGroup`
    （`ID` / `[creation rate]` / `[armor creation rate]` / `[target]` / `[drop item]` / `[smart drop item]`）。
  - `ParseDungeonDropBlocks(cells)` —— 把副本脚本的 `[difficulty dropitem group list]` 解成
    `[]DungeonDropBlock`；段值**逐字保留**：`Values` 收数值 cell，`Labels` 收字符串 cell。
- `internal/catalog/loot.go`：`LootCatalog` 增加 `DropGroupSource` + `DropGroups`（紧凑投影，非原始 cell：
  原始 cell 流 85,379 条，直接存进配置会让每份 loot 目录涨到约 6 倍）；`ImportLoot` 生成投影，
  `LoadLoot` 仅在目录声明了来源时校验，旧目录（无该键）照常加载。
- `internal/catalog/testdata/droptablebygroup_sample.json`：从真实 cell 流切出的 10 组夹具（856 cells）。

### 全目录扫描（874 / 3200 个副本声明该段；3564 个块 / 12926 个段）

| 现象 | 数量 | 处理 |
| --- | --- | --- |
| `[custom group info]` 空占位块（如 100004177） | 144 | 允许，如实保留空块 |
| 首块缺 `[group info]` 开头标签（100002889） | 1 | 隐式开块，块类型由结束标签确定 |
| 整段逐字节重复（5410） | 1 | 相同重复丢弃；内容不同则报错（避免合并导致双倍发奖） |
| 段标签存在但内容为空（100004448） | 1 | 允许，返回空 |
| 段内字符串 cell（type 6 / type 8） | 253 / 1953 | 归入 `Labels`。**注意：`pvfinspect` 的文本渲染会吞掉 type-8**，只有 cell 层看得到 |
| 组表内 `[target] "weapon legacy"` | 8 组 | 保留为 `DropGroup.Target` |

### 关键否定结论：`[normal group index]` 的编码**未确立**

3419 个 `[normal group index]` 段中：

- 2567 同时符合「长度前缀」读法（`count` 后跟 `count` 个组 id）与「成对」读法（`count group` 反复）；
- **758 只符合长度前缀**（例：`2 20001 20027 5 20002 20004 20006 20007 20044`）；
- **94 只符合成对**（例：100005262 的 `2 21251 3 1 17 3`、以及 `0 0` 这类退化值）。

两种读法各有独占样本 ⇒ **不发解码器**。该结论由
`TestNormalGroupIndexEncodingIsAmbiguous` 钉住：一旦某个读法变成全覆盖，测试失败，
必须回到 `droptable.go` 的注释处重新判断，而不是被顺手「修好」。

### 仍未落地（下一步）

1. **发奖语义**：`[group info]` 各段与组表如何组合、抽几次、`[reward multiple info]` 与
   `[special setinfo reward]` 的四个数字各是什么 —— 目前**无 L0 证据**，未实现，也未猜测。
   建议取证方向：客户端对 `NOTI2859`（md 所述「带上这一轮的奖励状态」）的读法，
   以及 NOTI31/回合结算包的处理路径。
2. **两份 tracked 配置未重新生成**：`configs/loot.level150.json` /
   `configs/loot.next25.json`。重新生成会给 `loot.next25.json` 带入一个与本次改动无关的漂移 ——
   物品 `6013`（`stackable/throw/bomber2.stk`，`[throw]`）会进入掉落池（1022 → 1023）。
   原因是 `internal/catalog/loot.go` 最后一次改动（`4ca1a4f`）晚于该配置的生成时间（09-11），
   属既有陈旧配置，需要用户确认后再一并刷新。
3. **GO 下一轮链（md 主体）**：按用户决定先取实机帧再改协议解码；当前 `DecodeSettlementExit`
   拒绝 `option > 3`，点 GO 的 `01 05 01` 会走 `SettlementExitRefused(5)`。

## 6. 实机取证前的静态预检（2026-09-26）

对 `configs/dungeons.full.json` 做子串定位（`109008634` 共 5 处）：

- **4 处**在副本脚本里，形式完全一致：`[clear condition] [hunt boss] 109008634 1 [/clear condition]`
  （对应 4 个 `[maze info]` 段）。
- **1 处**在地图脚本里，是怪物行：`[fixed] [normal] 109008634 1 …`
  ⇒ **装置确实是服务端可刷的固定普通怪，不是 Boss 行**。

结合代码：

| 环节 | 现状 | 结论 |
| --- | --- | --- |
| `HuntBoss` 字段 | `internal/catalog/dungeons.go:110` **只在 Odyssey 分支**解析 `[hunt boss]` | 本副本（非 Odyssey）`HuntBoss = 0` |
| 通关判定 | `internal/dungeon/completion.go:10 BossCheck`（CMD117）是唯一入口；非 Odyssey 要求 `Room.Boss && position == Maze.Boss` | 客户端为**普通怪**死亡不会发 CMD117 ⇒ **通关不成立** |
| 装置死亡 | 走 CMD39（普通怪死亡） | 该路径当前**不触发 `tryComplete`** |

⇒ 与 md「原来的通关只认普通副本的 BOSS 死亡包 CMD117，这张图没有这个包」**吻合**。
因此「能进本、进不了下一个门」的链条预判为：
**装置死 → 服务端不认通关 → 无 NOTI31 → 无 NOTI261（GO 不亮）→ 点 GO 发 CMD72 正文
`01 05 01`（option=5）→ `DecodeSettlementExit` 要求 `option ≤ 3` → 直接 refuse。**

**待实机确认的 4 件事**（本轮取帧目标）：

1. 装置死亡时客户端发的包号与实体 ID（预期 CMD39 + 该装置 entity）。
2. 服务端对该死亡包的应答（预期：不做通关处理，`events.jsonl` 里可能无对应 kind）。
3. 点 GO 时 CMD72 的**真实正文**是否为 `01 05 01`，以及服务端回的 `SettlementExitRefused(5)`。
4. 客户端在收到 refuse 后的表现（发呆 / 报错 / 崩溃）。

## 7. 实机前已完成的保护动作

- 备份 `D:/115us-backup/pre-live-abyss-20260926/`（91 MB）：`pgdata`（1394 文件，与源一致）、
  `local.json`、`launcher.local.json`、`bin/`（含本轮候选 exe）。
  **原因**：新 exe 带启动期存储迁移（`quest_hunt_migration`/`quest_reach_migration`/`odyssey_graduation`），
  首次起服会改写旧存档的任务进度模型。
- 确认运行时实际加载 `-dungeon-catalog configs/dungeons.full.json` +
  `-loot-catalog configs/loot.next25.json`（来源：`server/work/dfo_probe_tools/channel_probe.py:288/319`，
  由 `scripts/launch_local.py` 调起；启动器 `D:/115us-dfolauncher` 只是编排）。
  ⇒ 100005068 在目录里（能进本）；**掉落在没有任何消费者之前，实机不会看到任何变化**。

## 8. 实机会话逐帧分析（2026-09-26 13:12:42 / `..._131242_544699_next37`）

用户按要求进本。结论：**进本成功，但一轮未清就死亡，GO 的帧未取得。**

| 时刻 | 事件 |
| --- | --- |
| 13:14:47.721 | `dungeon_session_started {dungeon: 100005068, map: 100001016, maze: 0, monsters: 23}` —— **正是目标副本**，服务端刷出 23 只怪 |
| 13:14:47.8 | CMD390 / CMD283 / CMD469 / CMD585 —— 均标 `unimplemented_sample`（客户端杂项，暂记） |
| 13:14:49.084 | CMD37 → `dungeon_loading_ack` + `dungeon_actor_state` + `dungeon_loading_complete` + 经验/疲劳/外观恢复 |
| 13:14:49.403 ~ 13:15:07.495 | **CMD38 ×4**（开门）→ 四次都 `door_ack = 01` |
| **13:15:10.492** | **CMD40（角色死亡）** `f9074901000000…` → `player_death_ack` + `player_death_state` |
| 13:15:35.651 | `channel_refresh_finished {error: "EOF"}` —— 客户端退出 |

**关键否定事实**：整场 **没有一条 CMD39（怪死）**，也没有 CMD45 / 46 / 117 / 72 / 2062
⇒ 一轮从未清完，`CMD72` 正文（预期 `01 05 01`）**仍未取到**。
`gateway.err` 无错误（无 `SQLPWD`/28P01、无 panic），启动参数确认加载
`-dungeon-catalog configs/dungeons.full.json`、`-loot-catalog configs/loot.next25.json`。

### 「死亡后无法回城」的定性（**不是 bug**）

- 副本脚本声明 `[keep character death] 1` + `[disable exit] 3` ⇒ 客户端不给「放弃／回城」入口。
- **地图 `100001016` 已确认没有 `[cannot use coin map]`**（`catalog.Maps[100001016]`，624 cells，
  用仓库自身加载器核对）⇒ 复活币**允许**使用。
- 服务端复活链齐备：`cmd/wireprobe/dungeon_revive.go` 的 CMD41 三级回退
  （pilot 点数 → 背包复活币 → 扣 `lifeTokenCeraCost = 15` CERA）；前置只要求
  「已确认死亡 + 地图脚本允许」。
- **本场没有任何一条 CMD41** ⇒ 用户未点复活即退出，因此表现为"卡住"。

### 地图 100001016 的怪物行形态（顺带取证）

`[monster]` 段（第 282 个 cell 起，624 cells 共一段）：每只怪的模板是**数值 cell**，
`[fixed]` / `[normal]` 是**字符串 cell（type 6）**，形如
`T0 <template> T0 1 T0 0 T0 <x> T0 <y> … T6 "[fixed]" T6 "[normal]"`。装置 `109008634`
在地图脚本内（Python 定位在条目内 16,388 字符处）。

### 下次实机的差异要求

**必须把 23 只怪清完**（含装置 `109008634`）才能走到 GO；中途死亡应**点复活**而不是退出 ——
否则既取不到 GO 帧，又会复现"卡住"的观感。

## 9. 「打怪只掉 1 点血」的定性（2026-09-26 13:27，**非服务端缺陷**）

用户报告打怪只掉 1 点血，怀疑与测试账号装备有关。核对服务端规则后确认**服务端发的等级就是源声明值**：

`internal/dungeon/session.go:525-533` 的怪物等级规则：

```go
level := int64(v[2])
if v[1] == 1 {
    level += int64(basis)          // basis = 副本的 [basis level]
} else if v[1] != 0 {
    return nil, fmt.Errorf("unsupported monster level expression")
}
if level == 0 && basis > 0 { level = int64(basis) }
if level < 0 || level > 255 { return nil, fmt.Errorf("invalid monster level") }
```

地图 `100001016` 的怪物行形如 `109018067 1 0 1590 246 0 1 1`（`v[1]=1`、`v[2]=0`）
⇒ 等级 = `0 + basis`，而本副本 `[basis level] = 145`（`[minimum required level]` 才 115）
⇒ **服务端下发的怪物等级是 145**，与源一致。

角色为 115 级 ⇒ 30 级差下打出约 1 点伤害**符合预期**。**结论：GM 补发高阶装备是对症的**；
若补装后仍是 1 点伤害，再回头核对 `DungeonMonster.Level` 与客户端 MOB 表的口径
（但按当前证据，更可能是装备/等级差距，而非协议字段错误）。

## 10. 实机会话编号（便于回头找）

| 会话 | 用途 | 结果 |
| --- | --- | --- |
| `..._20260926_131242_544699_next37` | 首次实机 | 进本 100005068 成功，未清怪即死亡，退 |
| `..._20260926_131616_394591_next37` | 重启后体检 | 一切正常（曾登录 test-xl，town 241） |
| `..._20260926_132453_454748_next37` | GM 补装后重试 | 进行中 |

会话目录按时间取最新即可，无需用户记名字。

## 11. 【根因】GO 不亮 + 深渊无掉落 = 同一个原因：掉落目录等级上限低于怪物等级

2026-09-26 13:29 实机（会话 `..._20260926_132453_454748_next37`）：用户**清完了第一图的怪物**
（取图数 23），但右侧 GO **没有出现**。

### 证据链（三层，逐层收紧）

**① 实机日志**：进本后每一次怪物死亡（CMD39）都被服务端拒绝，理由逐字一致：

```
{"id":39,"kind":"dungeon_request_refused","reason":"drop source range is not imported"}
```

**② 代码判定**：`internal/loot/rules.go:154`

```go
if level == 0 || rank > 3 || int(difficulty) >= len(r.DifficultyBonus) || uint32(level)+3 > c.MaximumGrade {
    return out, fmt.Errorf("drop source range is not imported")
}
```

最后一项即「怪物等级 + 3 超过掉落目录的 `maximum_grade`」。

**③ 数值核对**（用仓库自身加载器跑 `internal/loot`，脚本 `runtime/lootaudit/`）：

| 目录 | maximum_grade | 允许的最高等级 | level=145 是否通过 |
| --- | --- | --- | --- |
| `configs/loot.next25.json`（**运行时实际加载**） | 130 | ≤ 127 | **否** |
| `configs/loot.level150.json`（已存在，未被运行时使用） | 150 | ≤ 147 | **是** |

两份目录的 `[drop prob]` / gold / grade 表**都覆盖到 200 级**，唯一差别就是导入上限。

**怪物等级来源**：`internal/dungeon/session.go:525-533` —— 地图行 `v[1]==1` 表示
「`[basis level]` + `v[2]`」；地图 `100001016` 的怪物行是 `109018067 1 0 1590 246 0 1 1`
⇒ 等级 = `0 + 145`。而本副本 `[basis level] = 145`（`[minimum required level]` 才 115）。

### 结论

**145 + 3 = 148 > 130 ⇒ 每次怪死都掷不出掉落 ⇒ 整条怪死请求被拒 ⇒ 击杀不被承认
⇒ 装置 `109008634` 的死亡也进不来 ⇒ 通关判定无从成立 ⇒ 没有 NOTI31/NOTI261 ⇒ GO 不亮，且没有任何掉落。**

这一条同时解释了用户最初的两个现象：
「深渊没落实掉落」与「能不能进下一个门」—— **它们是同一个根因**，
而不是 md 所推测的「通关只认 CMD117」。

### 可选修法（待用户确认）

- **A（最小、可逆）**：把运行时 `-loot-catalog` 由 `configs/loot.next25.json` 改为
  `configs/loot.level150.json`（同一份 PVF、校验和一致、上限 150）。改动点：
  `server/work/dfo_probe_tools/channel_probe.py` 一处；重启生效。
- **B（更一致）**：把 `configs/loot.next25.json` 按 `-max-grade 150` 重新生成，
  启动参数不动。注意这会顺带把物品 `6013`（`stackable/throw/bomber2.stk`）带进掉落池
  （1022 → 1023），那是 `internal/catalog/loot.go` 最后一次改动（`4ca1a4f`）之后的代码漂移，
  与上限调整本身无关。

**共同点**：两者都是把「掉落导入上限」提到覆盖 145 档内容。**上限 130 是 130 级时代的遗留**，
而千海之空是 145 档内容。

### 附带的设计观察（**根因修好后不会触发**，是否调整由用户定）

当前「掉落掷取失败」会**否决整条怪死请求**，于是数据缺口被放大成玩法阻断
（副本无法清完）。这与项目「不自欺、暴露错误」的取向不冲突（拒绝已记进 `events.jsonl`），
但「击杀」与「掉落」耦合在一条请求上，是这次问题被放大成"进不了下一个门"的原因。

## 12. 耦合点分析：掉落掷取失败会切断整条怪死处理（讨论稿，2026-09-26）

用户选择「先讨论耦合点」，故此处只做事实与方案，不落实现。

### 事实（逐层读代码得到）

1. **死亡本身已经记上了**：`internal/dungeon/session.go:239` `s.Dead[m.Entity] = true`，
   紧接着 `s.tryComplete()`。`ConfirmDeath` 内**没有**任何掉落调用。
2. **掉落是在发回执之前掷的**：`cmd/wireprobe/dungeon_flow.go:616-647` ——
   `plan` 先放进 `monster_death_ack(39)`，但**尚未发出**；随后 `w.drops.Death(...)`（第 644 行）
   一失败就 `return nil, err`。
3. **这一个 return 切掉了后面全部工作**：
   - `monster_death_ack(39)`、`monster_death_confirmed(38)` 都没发；
   - 第 655 行起的 `quests.GrantSeekingMonsterItems`（任务物品入包）跳过；
   - 第 702 行起的 **`quests.EnemyDeath`（杀怪推进任务）** 跳过；
   - 其后所有联动同样跳过。
4. 客户端是本地模拟的，它自己会把怪清掉，所以**表现是"清完了"**，但服务端这一侧：
   任务进度没推进、没有掉落、没有死亡回执，后续回合/通关联动也无从发生。

### 项目里已有同一失败模式的先例（重要）

`internal/dungeon/session.go:204-217` 的注释记录了 2026-09-12 的实机事故：

> Refusing it withheld the death confirmation, so **the client kept those monsters alive, the gate
> never opened**, and the room could only be left by returning to town.

当时的结论是：**不能为了服务端自己的原因扣住死亡报告**（于是 FFFF 击杀改为接受）。
本次是同一个失败模式从另一扇门复现 —— 扣住死亡报告的理由从「击杀者不认识」变成了「掉落掷不出来」。

### 备选方案

- **A（最小、与既有先例一致）**：把「掷掉落」改成尽力而为 —— 失败则记一条 event
  （如 `drop_roll_failed`，带 entity / level / rank / reason）并按「无掉落」继续，
  **回执与任务推进照常**。依据：死亡是客户端上报的事实（客户端强制），掉落是服务端的自由选择，
  两者不该绑在一条请求上。
- **B（更精细，推荐）**：引入类型化错误，只对**数据覆盖不足**类降级
  （`drop source range is not imported`、`missing source drop level`）并记 event；
  **内部不一致**类（`drop identity exhausted`、`invalid drop model tables`、`overlapping drop ranges`）
  仍然拒绝整条请求。理由：这两类问题的性质不同 —— 前者是数据缺口，后者是真缺陷，后者必须暴露。
- **C（不动代码）**：只把掉落上限提到覆盖 145。风险：同类事故会再犯，且玩家侧表现是
  「打怪没反应／门不开」，极难定位。

### 建议

**上限修复（根治：让 145 档真能掉东西）+ 解耦（防复发：把数据缺口从"阻断玩法"降级为"少掉东西 +
日志可见"）一起做。** 单纯做上限修复，下一次遇到更高等级或更大 rank 的内容会原样复发。

## 13. 方案 B 落地（2026-09-26 13:3x，**未提交／待实机**）

用户拍板「按 B 先试试」。同时把根因那半（掉落上限）一起补上。

### 13.1 类型化错误：把「覆盖缺口」与「模型缺陷」分开

`internal/loot/rules.go` 新增哨兵：

```go
var ErrOutOfDropRange = errors.New("drop source range is not imported")
```

归类（**只动这两处**）：

| 位置 | 原错误 | 现在 |
| --- | --- | --- |
| 等级/rank/难度越界守卫 | `fmt.Errorf("drop source range is not imported")` | `ErrOutOfDropRange`（文案不变） |
| prob/gold/grade 无对应等级行 | `fmt.Errorf("missing source drop level")` | `fmt.Errorf("%w: missing source drop level", ErrOutOfDropRange)` |

**不带哨兵的**（仍为真缺陷，继续拒绝整条请求）：`invalid drop model tables`、
`overlapping drop ranges`、`invalid source gold`、`drop identity exhausted`。

### 13.2 网关侧解耦：死亡报告不再被掉落失败扣住

`cmd/wireprobe/dungeon_flow.go`：

- 新增具名策略函数（B 的边界就在这一个函数里，可单测）：

```go
func fatalDropFailure(err error) error {
	if err == nil || errors.Is(err, loot.ErrOutOfDropRange) {
		return nil
	}
	return err
}
```

- 新增 `noteDropGap(event, entity, err)`，发一条 `drop_roll_skipped` 事件
  （带 entity / level / rank / template / reason）—— **降级也要可见**。
- `monsterDeath` 增加 `event func(map[string]any)` 参数（与本包既有 `useStackable`/`handle` 一致）；
  调用点 `main.go` 与 2 处测试同步更新。
- 掉落调用点改为：

```go
rows, err := w.drops.Death(w.activeDungeon, uint16(r.Entity))
if fatal := fatalDropFailure(err); fatal != nil {
	return nil, fatal
}
if err != nil {
	w.noteDropGap(event, uint16(r.Entity), err)
	rows = nil
}
```

⇒ 覆盖缺口时，`monster_death_ack(39)`、`monster_death_confirmed(38)`、任务物品入包、
**`quests.EnemyDeath`（杀怪推进任务）** 全部照常执行，只是这一只怪不掉东西。

### 13.3 根因那半：掉落目录上限提到覆盖 145 档

`server/work/dfo_probe_tools/channel_probe.py:320`：

```
-   str(project / "configs/loot.next25.json"),      # maximum_grade = 130
+   str(project / "configs/loot.level150.json"),    # maximum_grade = 150
```

同一份内层 PVF（校验和 `7ef2db59…` 一致），只有导入上限不同。
**可一行回退**；未改动任何生成产物（`loot.next25.json` 保持原样）。

### 13.4 新增测试

- `internal/loot/out_of_range_test.go`
  - `TestLevelAboveCatalogCeilingIsACoverageGap`：生产现场（145 vs 上限 130）必须归为覆盖缺口；
    另有一条按 `maximum_grade` 自适应的「越界一级」断言，重导目录后不会失效。
  - `TestModelDefectsDoNotLookLikeCoverageGaps`：空模型必须报缺陷且**不带**哨兵 —— 这是 B 的另一半。
- `cmd/wireprobe/drop_failure_policy_test.go`：`fatalDropFailure` 的三态（成功／覆盖缺口／模型缺陷）。

## 2026-09-26 13:44 实机会话：掉落为空的两个叠加根因

会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_134443_882748_next37`。
运行 `bin/wireprobe-handoff-source.exe`（`B7B6A85F…`，方案 B 版），
`-dungeon-catalog configs/dungeons.full.json`、`-loot-catalog configs/loot.level150.json`。

### 读数：方案 B 确认生效

| 事件 | 数量 | 说明 |
| --- | --- | --- |
| `monster_death_ack` (CMD39) | 23 | 与房间怪物总数一致 |
| `monster_death_confirmed` (CMD38) | 23 | 全部送达 |
| `monster_experience_updated` (CMD37) | 23 | 击杀经验链路也通了 |
| `dungeon_request_refused` | **0** | 上一场是 23/23 全拒，本场一条都没有 |
| `drop_roll_skipped` | **0** | 说明**没有**走降级分支，掉落是真掷了 |

⇒ 12:29 那场的 `drop source range is not imported` 完全消失。方案 B
（覆盖缺口降级 + 日志可见）与上限修复（`loot.next25` → `loot.level150`）都按预期工作。
任务推进（`quests.EnemyDeath`）也随之恢复 —— 它排在掉落之后，上一场被那个 `return` 切掉了。

### 但 23 条 `monster_death_confirmed` 的包体全是 `count=0`

包体固定 8 字节（`entity(2) + count(2) + 0000ff00`），`count` 全部为 0。
**没有任何错误、没有任何降级事件** —— 掉落成功执行了，产出就是空。

用仓库自己的加载器端到端复现（`runtime/droprepro`，60 次全新运行）：
**60/60 全空，0 个掉落**。这不是运气，是结构性的。

### 根因一：`lootService.Equipment` 从未赋值

`cmd/wireprobe/main.go` 构造掉落地服务时没有 `Equipment` 字段：

```go
lootService = &loot.Service{Store: …, Catalog: c, DropCatalog: dropCatalog,
                            Rules: r, BagRules: bag, Tables: tables}   // 无 Equipment
```

而 `internal/loot/pickup.go:22` 的 `Equipment *inventory.EquipmentCatalog` 是
`Roll` 里 `category == 2`（装备）唯一的候选来源：

```go
gear := equipmentCandidates(pool, rarity, level, grade)   // pool == nil ⇒ len(gear) == 0
if len(gear) == 0 {
    gear = equipmentCandidatesNearest(pool, rarity, level, grade)
}
if len(gear) == 0 {
    out.SkippedKinds = append(out.SkippedKinds, "equipment_grade_window_empty")
    continue                                              // 装备类直接丢弃，且不报错
}
```

`inventory.LoadEquipmentCatalog` 存在、`configs/equipment.current37.json`（3174 行，
校验和 `7ef2db59…` 与本仓库一致）也在，`cmd/admin` 与 `cmd/gmtool` 一直都在传它 ——
**只有 wireprobe 的掉落路径没接**。装备类掉落因此从未工作过一次。

`-loot-catalog` 的 flag 描述当时就写着 `equipment pending`，佐证这是已知待办，
但它静默丢弃而非报错，所以一直表现为"这个游戏怪不掉东西"。

### 根因二：本副本 dungeon 脚本首格就是 `[exclude gold drop]`

```
cell 0: {3, 185752815, [exclude gold drop]}
```

`internal/loot/dungeon_rules.go` 的 `filterDungeonAwards` 见到该标签就丢弃所有
`Template == 0`（金币）的 award。全库 273 处出现该标签。

⇒ 这个副本**设计上就不掉金币**，与装备池缺失叠加后，一次完整清场只剩
`stackable` 类的千分之几概率。

### 量化对照

| 装配 | 60 次全新运行的总掉落 |
| --- | --- |
| 现状（`Equipment = nil`） | **0** |
| 加载 `configs/equipment.current37.json` | **16**（15/60 次运行有产出） |

装备池 2794 件，其中 140 件落在 145 档的 grade 窗口 `[100,148)`。

### 「掉落绑定在柱子上」—— 玩家判断与数据一致

装置 `109008634` 是**房间第 23 只**、也是唯一的 `[fixed] [boss]`：

```
idx=22 entity=4118 template=109008634 rank=3 level=145  x=3916 y=326   opt=[[fixed] [boss]]
```

它 rank=3 ⇒ 概率远高于其余 22 只 rank=0 的杂兵
（`[monster type drop bonusrate]` 按 rank 取列：金币 49.5% 对 7.5%）。
会话最后一条 CMD39 的 `plain_hex` 以 `16 10` 开头 ⇒ entity 4118，**正是这只**。

所以玩家说"掉落应该绑在柱子上"是对的：**柱子的死亡本该是主要产出**。
它掉了 0 件，是因为金币被 `[exclude gold drop]` 过滤、装备被空池丢弃 ——
两个原因都落在它身上。后 22 只杂兵本来就只有约 8%，全空属正常。

### 已落地的修复

| 位置 | 改动 |
| --- | --- |
| `cmd/wireprobe/main.go` | 新增 `-equipment-catalog`（默认取 `DFO_EQUIPMENT_CATALOG`，沿用本项目 catalogs 的既有惯例）；加载后赋给 `loot.Service.Equipment`；启动时打印 `loaded equipment catalog: N rows, M droppable` |
| `cmd/wireprobe/main.go` | loot 启用但未给装备目录时 **`log.Fatal`** —— 静默空池正是本次故障的形态，不允许再发生 |
| `dfo_probe_tools/channel_probe.py` | 设 `DFO_EQUIPMENT_CATALOG` 为绝对路径（gateway 的 cwd 是整合包根，内置相对默认值解析不了 —— 该文件对所有其它 catalog 都是这么做的） |
| `cmd/wireprobe/border_drop_integration_test.go` | 门控回归：`BORDER_DROP_INTEGRATION=1` 时采样 60 次运行，总掉落 < 5 即失败。装备池一旦再被丢掉，这条会立刻红 |

**未改动**：`[exclude gold drop]` 的过滤行为。那是源数据的设计意图，不是缺陷。

### 仍未解决

1. **GO 点不动**。会话里 22 只杂兵 + 柱子全部确认死亡、GO 已出现，但
   **客户端的帧里一条 CMD72 都没有**（`01 05 01` 也未出现）。整场唯一的"未实现命令"
   CMD283/585/390 在进本前（05:45:21）就已出现，属杂项。
   ⇒ 需要确认：玩家是否真的点了 GO；点了之后客户端是没发包、还是发了别的包。
   这是 `[difficulty dropitem group list]` 之外的另一条独立链（NOTI261 方向）。
2. **装备池等级上限只有 107**。145 档内容只能掉到 grade 100–107 的装备
   （`equipmentCandidatesNearest` 会兜底到邻近稀有度）。要让 145 档掉对应等级的装备，
   需要重新导出更高等级的装备索引。
3. **副本专属奖励组仍未发奖**。`[difficulty dropitem group list]` 已解析成结构化数据，
   但 `[normal group index]` 的编码未确立（见前文），发奖语义无 L0 证据。

## 2026-09-26 14:12 实机会话：接线已生效，但掉率期望只有 0.25 个/场

会话 `..._20260926_141239_556576_next37`。启动日志已出现

```
14:12:46 loaded equipment catalog: 3174 rows, 2794 droppable, from …\configs\equipment.current37.json
```

⇒ 装备目录接线**确认生效**（`DFO_EQUIPMENT_CATALOG` 走通，`-equipment-catalog` 的 Fatal 守卫没有触发）。

但 23 条 `monster_death_confirmed` 的 `count` **仍全是 0**，柱子（最后一条 CMD39 = entity 4118）
也打死了。于是用 `runtime/droprepro -equipment … -runs 100` 量化：

| 采样 | 结果 |
| --- | --- |
| 100 次全新运行 | **76 次一件不掉（76%）**，24 次有产出，共 **25 个掉落** |

⇒ **单场期望 0.25 个掉落**。这不是缺陷，是这个副本的**通用掉落**本来就该这么低：

- 22 只 `rank=0` 杂兵：装备 0.27% + stackable 0.1% + 其他 0.08% ⇒ 单只约 0.4%
- 1 只 `rank=3` 柱子：装备 22.7% + stackable 0.2% + 其他 0.16% ⇒ 约 23%
- **金币完全缺席** —— `[exclude gold drop]` 把 rank3 的 49.5% / rank0 的 7.5% 全过滤了

⇒ 期望 ≈ 22×0.0045 + 0.23 ≈ 0.33（实测 0.25）。

**结论：这个副本设计上不靠普通怪的通用掉落。** `[exclude gold drop]` 本身就是证据 ——
源数据明确关掉了通用金币，产出必然走另一条通道，即 `[difficulty dropitem group list]`。

## 掉落段结构（`runtime/dropblockdump -dungeon 100005068`）

三段 `[group info]`，逐字如下（未解码）：

```
[item index]             10326880 10326884 10420247 10419754     ← 段0
                         10326880 10326884 10420248 10419754     ← 段1
                         10326880 10326884 10420249 10419754     ← 段2
[normal group index]     1 21251 1 21600   /  1 21251 1 21601   /  1 21251 1 21602
[fame info]              0 0 0 0
[special setinfo reward] 320 10326880 1 21279   <2::SetEquipmentReward>
                         320 10326880 1 21468   <2::SetOathPrimerReward>
                         320 10326880 1 21470   <2::RareEquipmentReward>
                         138 10326880 1 21310   <2::WeaponEquipmentReward>
[reward multiple info]   3 1   /   3 6   /   3 12
```

三条独立观察，合起来指向「三段 = 三轮」：

1. **`[item index]` 的第三个值三段连续**：`10420247 / 10420248 / 10420249`（+1 递增）。
2. **`[normal group index]` 的第二个组号三段连续**：`21600 / 21601 / 21602`。
3. **`[reward multiple info]` 的第二个数三段递增**：`1 / 6 / 12`。

⇒ **每个房间（maze）各有一段奖励**，越靠后的轮次给得越多（1 → 6 → 12）。
本副本 `mazes` 恰好是 **3 个**，且三个 maze 的 room 都用同一张地图 `100001016`。

另有两处**新的结构证据**，可用于收窄上一条未决项（`[normal group index]` 的编码）：

- `[special setinfo reward]` 的形态是 **`<权重/tier> <物品ID> <数量> <组ID>`**：
  `… 10326880 1 21279` 里 `1 21279` 就是「数量 1 + 组 21279」，与
  `[normal group index] 1 21251 1 21600` 完全同构
  ⇒ **支持「(数量, 组ID) 成对」读法**，而不是长度前缀。
  （两者在 count 恒为 1 的段上等价，但 `[special setinfo reward]` 给出了旁证。）
- `[special setinfo reward]` 带**具名类别标签**（`SetEquipmentReward` / `SetOathPrimerReward` /
  `RareEquipmentReward` / `WeaponEquipmentReward`）—— 那是**客户端字符串引用**，
  说明客户端也读这张表（很可能用于奖励预览）。这给了下一步一个 L0 取证入口。

**仍未做**：谁触发这段发奖（本轮推测是「打死柱子 / 通关」时按 maze 轮次取段），
以及发奖的具体算法（`[item index]` 与 `[normal group index]` 如何组合、`[reward multiple info]`
的第一个数 3 是什么）。**需要客户端侧证据**，不能照猜实现。

## GO 仍然说不通：连续两场都没有一条 GO 相关包

`..._141239_556576` 这一场：打完 23 只（含柱子 entity 4118），**CMD72 / CMD45 / CMD46 /
CMD117 / CMD261 / CMD2062 全部为 0 条**。上一场（`..._134443_882748`）同样如此。

进场后的未实现命令仍是 `CMD585 / CMD283 / CMD390` —— 它们在进本前（06:13:25）就已出现，
是杂项，不是 GO。

⇒ **GO 出现是客户端本地判定；点击不产生任何上行的包。** 客户端在等服务端的某个状态/许可。
这条链（md 指向 `NOTI261`）尚未实现，而且它现在**是首要阻塞**：
进不去下一轮，就永远看不到第 2、3 段的奖励。

### 勘误：`[difficulty dropitem group list]` 的段选取规则**未确立**

上一节按本副本的观察提出「三段 = 三轮（三个 maze）」。**用全目录统计一跑就被推翻**：

`runtime/dropblocksurvey` 遍历 873 个带掉落段的副本：

| 关系 | 数量 |
| --- | --- |
| 段数 == maze 数 | **70** |
| 段数 < maze 数 | 69 |
| 段数 > maze 数 | **734** |

反例（结构清楚、不是解析异常）：

- `dungeon 100002742 / 100002937 / 100002721 / 100003145`：**4 段、1 个 maze**
- `dungeon 100005109`：**6 段、2 个 maze**
- `dungeon 100005062`：**1 段、12 个 maze**
- `dungeon 75 / 77`：**4 段**

⇒ **段数既不等于 maze 数，也不等于难度数。段与轮次/难度的对应关系没有证据。**
之前那三条「连续」观察（`[item index]` 第 3 值 +1、`[normal group index]` 组号 +1、
`[reward multiple info]` 1/6/12）在本副本内成立，但**不足以推广**，也不能据此选段。

同一次统计还**削弱了「(次数,组ID) 成对」的旁证**：

```
dungeon 100005109  [normal group index] = 1 10900 4 9001 10000 10002 8001
```

7 个数（奇数）⇒ 成对读法在此不成立，而长度前缀读法成立
（`(1,[10900])`, `(4,[9001,10000,10002,8001])`）。
`[special setinfo reward]` 的 `<tier> <物品ID> <数量> <组ID>` 形态只是**巧合同构**，
不能当读法依据。⇒ **`[normal group index]` 的编码结论保持「未确立」不变**，
`internal/catalog/droptable.go` 继续逐字保留、不发解码器。

### 仍然成立的硬结论：这些物品是「通关奖励凭证」

`[item index]` 里的 ID 全部是 `stackable` 且 `stackable_type = [etc]`，其 PVF 定义有一条
决定性字段：

```
[icon]
`WorldMap/RewardButton.img` 5      ← 10326880（世界地图·奖励按钮图集，第 5 帧）
`WorldMap/RewardButton.img` 16     ← 10420247（同图集第 16 帧）
```

`WorldMap/RewardButton` = **世界地图的「奖励」按钮图标**。配合 `[name]`/`[explain]` 为空、
`[grade] 1`、`[rarity] 4`、`[attach type] [free]`、`[move wav] SCRAP_TOUCH`
⇒ **这些是发放给玩家、代表「本轮奖励已发放/待领取」的凭证物**，不是装备也不是消耗品。

⇒ 「`[item index]` 列出的是该段要发的物品」这一点可以确定；
**不确定的是「哪一段在哪一刻生效」以及「各组怎么抽」**。

## 外部修复文档核对（`E:/下载/attunement-repair-20260926.md`，2026-09-26）

来源是 `jack155454654a` 的整理（就是目前开着 `!56`/`!57` 的那位）。代码基线 **`1d691eb`**。

### 先按纪律核对：**这套实现我们完全没有**

| 文档提到的东西 | 在我们仓库的核对结果 |
| --- | --- |
| `AttunementBoss` / `attunementDeathExpired` / `finishAttunementDeath` | **0 命中** |
| `LoadAttunementDrops` / `attunementBossRolled` / `cardSkip` / `DFO_ATTUNEMENT_REWARDS` | **0 命中** |
| `internal/loot/attunement.go` | **缺失** |
| `configs/attunement-candidates.json` | **缺失** |
| `scripts/export_attunement_rewards.py` / `repair_attunement_arrow.py` | **缺失** |
| 提交 `4ef0078` / `eb7dbf7` / `c67ec13` / 基线 `1d691eb` | **都不在我们的历史里** |

⇒ 它是**另一条独立线**上的实现，**不能照抄、也不存在"已经包含"**。
（`cmd/wireprobe/player_death.go` 我们倒是有，但里面没有它的符号。）

### 再核「结构主张」：**几乎全部在我们自己的数据上逐字成立**

（纪律：外部文档的**结构主张**常与事实一致，**时序主张**最容易错。）

| 文档主张 | 我方核对（工具 `runtime/taglist`、`runtime/maptags`）|
| --- | --- |
| 源脚本有 `[dungeon type]`，值为 `boundary of attunement` | **成立** —— `100005067` cell 183、`100005068` cell 265，值逐字相同 |
| `[limit party count]` = 1（单人） | **成立** —— cell 103 / 185，值 `1` |
| `[hunt boss]` 给出最终领主 | **成立** —— cell 99，`109008634 1` |
| 奖励表在 `etc/rewardboostinfo/skyofathousandseasofborder/{unique,legendary,epic}.ctp` | **成立** —— 三份都在（`data_type=4`，6352 / 6960 / **11280** 字节）|
| `rewardboostinfo.lst` 是索引 | **成立** —— 7 条目，含 `SkyofaThousandSeasOfBorder/{Unique,Legendary,Epic}.etc` |
| 副本 `100005066/67/68` = unique / legendary / epic | **部分** —— 我方目录**只有 `100005067`（legendary）与 `100005068`（epic）**，`100005066` 不存在 |
| 地图 `100001016` 是**四段滚屏**，左边界 `1 / 1110 / 2213 / 3325` | **成立，逐字** —— `[arcade scroll area]` 里四个 `[area]`，第一个数分别是 1、1110、2213、3325 |
| GO 箭头来自 `Screen_Arrow_Maker`，有 `arrow_on.act` / `arrow_off.act` | **成立** —— `contents/2026/endkeeperoforder/passiveobject/screen_arrow_maker/action/{arrow_on,arrow_off,basic}.act` 都在 |

### 三条改变方向的结论

**1. GO 不是协议问题，是客户端 PVF 资源问题。**

地图 `100001016` 是**同一张地图内的四段滚屏**（`[arcade scroll area]` 的 `[area]` 逐个右移）。
「滚屏走到下一段」**不等于**「切换到另一张地图」。文档的核心论断：

> 显示 GO 不等于存在下一张地图。

箭头由 `Screen_Arrow_Maker` 的 `arrow_on.act` 在条件满足时创建；它没有区分「调律途中」与
「最终区域」，于是**到了最后一段仍然显示 GO**，玩家走过去自然没反应。
他们的修复是在创建分支加 `左边界 < 3000` 条件、并在 `左边界 >= 3000` 时复用 `arrow_off.act`
删除箭头（3325 > 3000 ⇒ 最后一段不再有箭头）。

⇒ **我们前两轮"等服务端发 NOTI261 / 改 CMD72 解码器"的方向是错的。**
会话日志里连续两场**没有任何 GO 相关包**，正因为**客户端压根不需要发** —— 它只是在等一个
本不该存在的"下一段"。这解释了那个反常现象。

⇒ 这条修复要**改客户端 PVF 资源**。项目纪律与文档本身都强调：**必须用户明确许可**才能动。

**2. 深渊专属奖励在 CTP 表里，按难度选档。**

`epic.ctp`（我方 `100005068`）11280 字节、`legendary.ctp` 6960 字节、`unique.ctp` 6352 字节
⇒ **难度越高表越大**。CTP 是 `data_type=4` 的二进制，`pvfinspect` 的文本导出对它返回空，
但已经确认可以用 `-files` 拿到原始字节（`runtime/ctp2/00-epic.ctp.bin`，
sha256 `52810da52a566db5e43acf9e50f84a1083b54eaf557e371ba62ff47265821d59`）。
字节流里能看到成组的浮点（如 `41 2B E6 F8` ≈ 10.7、`41 63 DF C2` ≈ 14.2），与文档所述
「百万权重空间」一致。**字段布局仍需独立取证**（IDA 或逐步字节分析），不能照抄他们的代码。

**3. 通关判定用源领主，不等 CMD117。**

文档：`tryComplete` 只在该源领主地图内、指定模板的怪物为 `rank 3`、非 APC、非非战斗、
`Team != 0`、且实体已登记死亡时，才设置通关。这与我方现状（`HuntBoss` 只在 Odyssey 分支解析，
本副本 `HuntBoss=0`）是**同一个缺口**。

### 不能照抄的地方（纪律）

- 他们的符号、文件、配置、导出脚本、PVF 修补脚本**一律不能直接搬**（基线 `1d691eb` 与我们不同）。
- 文档明确写了：**「不要对已包含修复的分支重复套补丁，不要用旧文件整体覆盖合并后的新实现」**
  —— 而我们是**没有**这些修复的分支，所以只能**照它的"结构发现"自己实现**。
- `c67ec13` 那个提交还**夹带了启动器更新**，文档自己提醒不要连带搬入。
- 文档自己也声明：未重新运行测试/编译/起服，且 `CHANGELOG` 里仍挂"待实机验证"。
  ⇒ **它的"已验证"程度有限，我方落地后仍需自己实机验收。**

### 箭头资源的实际内容（已逐字读取，2026-09-26）

四个文件全部 `data_type=1`（文本），已用 `pvfinspect -files` 导出并读完：

| 文件 | 大小 | role |
| --- | --- | --- |
| `screen_arrow_maker.obj` | 50 B | passive object 定义 |
| `action/basic.act` | 325 B | **显示/隐藏的判定在这里** |
| `action/arrow_on.act` | 650 B | 创建箭头（`make_arrow`）+ 维护 `arrow_off_trigger` |
| `action/arrow_off.act` | 470 B | 删除箭头（`delete_arrow`）+ 维护 `arrow_on_trigger` |

`screen_arrow_maker.obj`：

```
[layer] `[normal]`  [pass type] `[pass all]`
[basic action] `./Action/Basic.act`
[etc action] `./Action/Arrow_off.act` `./Action/Arrow_on.act`
```

**`basic.act` 才是决定显示/隐藏的地方**（与文档"改 `arrow_on.act`"的说法有出入）：

```
[TRIGGER] [FRAME] 0
  [BEGIN IF]
    [WHICH MONSTER] [IS TEAM] `MONSTER TEAM` [CHECKED NO] [>]
      [DO BEHAVIOR NAME] `ME` `go_arrow_off`      ← 场上还有怪 ⇒ 隐藏
    [ELSE IF]
      [DO BEHAVIOR NAME] `ME` `go_arrow_on`       ← 场上没怪   ⇒ 显示
  [END IF]
[/TRIGGER]
[BEHAVIOR] `go_arrow_off` [SET CUSTOM ACTION] 0
[BEHAVIOR] `go_arrow_on`  [SET CUSTOM ACTION] 1
```

⇒ **箭头显示的唯一判定是「场上还有没有 `MONSTER TEAM` 的怪」。**
这正好解释了实机观察：**清完怪 → GO 出现**（与 `[arcade scroll check type] "not exist enemy"` 同源）。
它**不看**当前处于第几段滚屏 —— 这就是"到了最终段仍显示 GO"的机制。

`arrow_on.act` 的创建分支（`INIT` 里三组条件，都落到同一个 behavior）：

```
[DO BEHAVIOR NAME] `CHECKUP OBJECT` `make_arrow`
[BEHAVIOR] `make_arrow`
  [CREATE ANIMATION] [PATH] `../Animation/go_ver4_left_arrow.ani` [ID] `arrow` [SCREEN MODE] `MAIN LAYER`
  [SET TRIGGER ENABLE NAME] `arrow_off_trigger` `ON`
```

`arrow_off.act` 的删除（文档说"复用它的删除行为"，成立）：

```
[DO BEHAVIOR NAME] `CHECKUP OBJECT` `delete_arrow`
[BEHAVIOR] `delete_arrow`
  [DELETE CREATE ANIMATION] [ID] `arrow` [MODE] `MAIN LAYER` [ONLY TARGET] [FADE OUT TIME] 0
  [DELETE ANIMATION OBJECT] `go_guide_arrow`
```

### ⚠️ 改法的可行性**尚未验证**，不要当成"改一行就行"

两处不确定：

1. `arrow_on.act` 的 `[COMPARE VAR]` 后面**操作数是空的**（直接接 `[==]` / `[<]` / `[>]`）。
   要"再加一个左边界条件"，必须先弄清这个语法实际比较的是什么、操作数写在哪里。
2. 文档说 3000「比较的是滚屏区左边界」—— 但**在地图脚本里那是 `[arcade scroll area]` 的 `[area]`
   第一个数**（1/1110/2213/3325），那是**地图数据**；act 脚本里怎么读到"当前段左边界"这个运行时量，
   文档没给、我也还没证。

⇒ 结论：**"终点不显示 GO" 可以通过改 PVF 资源实现，但具体改法与能否生效需要先做一期取证**
（读懂 `[COMPARE VAR]` 的语法 + 确认 act 引擎能不能取到滚屏段位置），**不是照文档抄一个常量**。

### 但有一点现在就能确定：**GO 与协议完全无关**

箭头是 `passiveobject` 的纯客户端表现，显示与否只由 `basic.act` 的「场上还有没有怪」决定。
⇒ **服务端发任何包都不会让它消失，也从来不需要客户端为此发任何包。**
这正是实机日志里"连续两场零 GO 相关包"的原因，也说明
**修 GO 不必等 `NOTI261` / `CMD72` 那条链**（前两轮的方向已作废）。

## 「卡住」的服务端根因已定位（2026-09-26）—— 并由此确定 PVF 修改只解决哪一点

### 我方 `tryComplete()` 的确切缺口

`internal/dungeon/completion.go:83` 的 `tryComplete`，在 `s.completionTarget == 0`
（即客户端**没有发 CMD117 BOSS_CHECK**）时只有三条免 CMD117 的路径：

1. dungeon 26 maze 3 的特例（与本副本无关）；
2. 「源 boss 房间 + **没有可战斗 boss** + 敌人全死 + 有可上报的 display boss」；
3. 「分层剧情终图 + **没有可战斗 boss** + 敌人全死 + 有可上报的 display boss」。

三条都要求 **`!s.hasFightableBoss()`**：

```go
func (s *Session) hasFightableBoss() bool {
	for _, m := range s.Monsters {
		if !m.NonCombat && m.Team != 0 && (m.Rank == 3 || m.APC && m.Rank >= 5 && m.Rank <= 8) {
			return true
		}
	}
	return false
}
```

而本副本的最终领主 `109008634` 是 **`rank=3`、`Team=100`、`NonCombat=false`** ⇒
`hasFightableBoss()` 返回 **true** ⇒ `!hasFightableBoss()` 为 **false** ⇒
**两条路径都不成立** ⇒ 落到末尾 `return`，**状态永远不置为完成**。

⇒ **只要客户端不发 CMD117，这个副本就永远不通关。** 而文档确认该玩法**不发 CMD117**。
⇒ 我方现状与文档描述的缺口**完全一致**，且根因已精确到函数分支。

（`atSourceBossMap()` 本身是满足的：`roomprobe` 显示该房间 `boss=true`、位置等于 `[maze.Boss]`。）

### 于是「卡住」的完整因果链

| 步 | 发生了什么 | 谁的责任 |
| --- | --- | --- |
| 1 | 玩家滚到第 4 段（柱子 x=3916 落在最后一段 3325 起）打死柱子 | 客户端玩法 |
| 2 | 服务端 `ConfirmDeath` 记下死亡（日志 23 条 ack/confirmed） | 已正常 |
| 3 | `tryComplete()` 因 `hasFightableBoss()` 为真而 **不通关** | **服务端缺口** |
| 4 | 客户端本地判定「场上无怪」⇒ 显示 GO 箭头（`basic.act`） | 客户端 |
| 5 | 玩家点/走箭头 ⇒ **没有第 5 段滚屏** ⇒ 没反应 | 客户端资源 |
| 6 | 玩家体感：**卡住、进不去下一个房间** | = 3 + 5 |

### 因此：改 PVF 只解决第 5 步，不解决「卡住」

- **改 PVF 解决的唯一一件事**：最终段那个**走不通的箭头**不再显示，消除"还有路"的误导。
- **它不解决**：玩家为什么走不了（因为没有下一段）、为什么没结算（因为不通关）、
  为什么没奖励（奖励链未接）。这三条都在**服务端**。
- **但修好第 3 步之后**，打完柱子的瞬间就会通关（有结算），
  玩家**不再需要**那个箭头去"进入下一轮" ⇒ 此时第 5 步才成为纯粹的收尾体验问题。

⇒ **正确顺序：先补服务端的通关判定（可独立完成、不碰客户端），PVF 箭头作为最后收尾。**

## 服务端实现：调律领主的通关判定（2026-09-26）

按「先服务端、再决定是否动 PVF」的顺序落地。**只改服务端，不碰客户端资源。**

### 识别条件（三字段同时成立）

| 字段 | 值 | 来源 |
| --- | --- | --- |
| `[dungeon type]` | `boundary of attunement` | 与 Odyssey 用的 `[dungeon mode script]` 是**不同字段**，所以调律不是 Odyssey |
| `[limit party count]` | `1` | 单人 |
| `[hunt boss]` | `(模板ID, 1)` | 源领主 |

出货目录里**只有 `100005067` / `100005068` 命中**（`runtime/attunementsurvey` 可复算），
两处的源领主都是 `109008634`。

**没有放宽现有 `HuntBoss`**：非 Odyssey 副本里带 `[hunt boss]` 的有一大批
（survey 列出 `400001557`、`500000052` 等），把 Odyssey 专用解析放宽到全体会静默改判它们。
所以新增独立字段 `DungeonDefinition.AttunementBoss`。

### 两处改动

1. `internal/catalog/dungeons.go` —— 新增 `AttunementBoss` 并在 `ParseDungeon` 里解析。
2. `internal/dungeon/completion.go` —— `tryComplete` 在 `completionTarget == 0` 块内新增第四条路径：
   源领主模板的 rank 3 怪已登记死亡即置 `completed`；
   `CompletionTarget()` 对调律按**源领主模板**取实体（而不是"房间里第一个 rank3"）。

### 落地时踩到的两个坑（都已钉进注释与测试）

**坑一：严格校验会把整份目录加载搞挂。**
第一版写成「`[limit party count]`/`[hunt boss]` 不合法就 `return d, fmt.Errorf(...)`」，
结果 `LoadDungeons` 直接 panic —— 出货目录里确实存在 `[dungeon type]` 是调律、
但 `[hunt boss]` 形态不同的脚本。改为**不成立就保持 0**（识别，不是校验）。

**坑二：`sectionCells` 会累积同名的每一段，而这里恰好有多段。**
`sectionCells` 对同名标签是**累加**而非只取第一段，于是「恰好一对」的判定漏掉了 `100005068`：

```
100005067  [hunt boss] ×1 → [109008634, 1]                      ⇒ 2 个 cell ✓
100005068  [hunt boss] ×3 → 每处都是 [109008634, 1]，共 6 个 cell ⇒ 判定失败 ✗
```

**原因是史诗档一张脚本里 3 个 maze 各声明一次同一领主**
（`runtime/huntcount` 可复算）。`100005067` 只有 1 个 maze，所以**只测一个副本发现不了**。
现在的规则：接受「所有段都指向同一领主」，其余形态（真正多目标）保持 0、不猜测。

### 回归测试（`internal/dungeon/attunement_completion_test.go`）

| 测试 | 钉住的行为 |
| --- | --- |
| `...SourceBossDeathCompletesWithoutBossCheck` | 杂兵死**不**通关；领主死即通关（全程无 CMD117）；`CompletionTarget()` == 领主自己的实体 |
| `...RequiresTheSourceBoss` | `AttunementBoss == 0` 时，rank3 死也不通关（不误伤别的副本） |
| `...IgnoresOtherRankThree` | 同房间里**其它** rank3 不满足条件（按模板取，不是按 rank 取） |

### 为什么这样就够了（通知会自动发）

`monsterDeath` 末尾每次都会调 `completeDungeon()`，而它只判断
`!Completed() || completionSent` ⇒ 直接发
`boss_check_confirmed`(NOTI115) + `dungeon_clear_enabled`(NOTI31)。
⇒ **只要 `completed` 变真，通知链路自动走通**，无需另接。

## 实机验证通过：通关链路完全走通（2026-09-26 14:45 会话）

会话 `..._20260926_144435_494268_next37`。**服务端修复确认生效**：

```
06:45:43.783  boss_check_confirmed     (NOTI115)   ← 通关判定成立
06:45:43.783  dungeon_clear_enabled    (NOTI31)
06:45:43.806  dungeon_play_result / dungeon_clear_experience / dungeon_clear_reward
06:45:43.848  card_scroll_ack / card_layout_ack    ← 翻牌
```

玩家侧确认：**出现结算界面**（"是否继续？/ 继续挑战 F10 / 选择其它地下城 F11 / 返回城镇 F12"）。
⇒ `AttunementBoss` 识别 + `tryComplete` 新分支 + `CompletionTarget` 按模板取，三者都对。

### 勘误：GO 不是缺陷，PVF **不需要改**

玩家向其他玩家核实后确认：**终点那个 `GO` 就是「继续挑战」（F10）**，
出现在结算面板里，是正常功能。不是"指引去下一张地图"的误导箭头。

⇒ 本文件前面关于「GO 是 PVF 资源缺陷、应加左边界 < 3000 条件」的判断**作废**。
`basic.act` 按「场上还有没有怪」决定箭头显隐，只是它的**实现机制**，不代表该机制是错的。
**结论：客户端 PVF 不需要为本问题做任何修改。** 前两轮"等服务端发 NOTI261/CMD72"的方向同样作废
（那时点不动 GO，真正原因是**还没通关**——没有结算面板，GO 自然不可用）。

### 至此唯一未解决项：专属奖励

该场 23 条 `monster_death_confirmed` 仍然全是 `count=0`。装备池接线已生效（`2794 droppable`）、
通关链路已通，**剩下的空白就是「深渊专属奖励」这条链本身从未实现**。

## 专属奖励的结构（从 CTP 直接读出，不是我方猜测）

**我方已有 CTP 解析器** —— `internal/catalog/pvf/ctp.go`（`Archive.CTP(path)` → `CTPTable`），
`apocalypse.ctp` 一直在用。`pvfinspect -file` 对它返回空只是因为那个命令只处理 `DataType == 1`。

`etc/rewardboostinfo/skyofathousandseasofborder/epic.ctp`
（11280 字节，sha256 `52810da52a566db5e43acf9e50f84a1083b54eaf557e371ba62ff47265821d59`，
`version=1`，`records=90`）解出来是：

```
列（trailer）:
  [additional drop table]  14 条  [10 15 20 25 30 35 40 45 …]
  [dungeon index]           1 条  [0]
  [fixed drop table]        3 条  [1 4 7]
  [hidden drop table]       2 条  [80 86]

记录:
[0]  [dungeon index]      = 100005068          ← 这份表属于哪个副本
[1]  [fixed drop table]                        ← 固定奖励，每个 maze 一份
  [2]  [maze]             = 0
  [3]  [drop list]        = 914300→10419728 "epic",  85700→10419729 "primeval"
[4]/[7] 同上，[maze] = 1 / 2（三份内容相同）
[10] [additional drop table]                   ← 概率附加（共 14 份）
  [11] [effect index]     = 2
  [12] [select prob]      = 90000              ← 百万权重空间
  [13] [drop count]       = 3
  [14] [drop list]        = 274500→10419730 "normal"
                            200000→10419731 "normal"
                            272000→10419732 "rare"
                            200000→10419733 "unique"
                             36000→10419734 "legendary"
                             16000→10419735 "epic"
                              1500→10419736 "primeval"
                            （合计正好 1,000,000 ⇒ 权重分母 100 万）
[80]/[86] [hidden drop table]
```

三条重要事实：

1. **`[maze]` 索引证实了「固定奖励按迷宫选」** —— 3 个 maze 对应 3 份 `[fixed drop table]`，
   这正是文档所说"`Roll` 按当前迷宫选择对应固定奖励"。本副本三段内容相同，所以三档一致。
2. **权重分母是 1,000,000**，与文档的"百万权重空间"逐字吻合；`[select prob]` 也在同一空间
   （90000 ⇒ 9%）。
3. 掉落物的名字是**稀有度档位**（`normal / rare / unique / legendary / epic / primeval`），
   物品 ID 连续（`10419728..10419736`），与 `[difficulty dropitem group list]` 里那些
   `[item index]` 同段 —— 它们应是**奖励容器/凭证**，需要再展开一层才是最终装备。

**仍未确证的部分**（实现前要定）：固定表的 `[drop list]` 是**整份发出**还是**按权重抽一个**；
`[select prob]` 是"每个附加表各判一次"还是"先抽一个表再判"；`[hidden drop table]` 的触发条件；
以及容器展开链（`10419728` 这类物品打开后给什么）。这些需要客户端消费代码或实机反证，
不能只凭表结构推断。

**工具**：`runtime/ctpdump -path <ctp> [-limit N]` —— 用仓库自己的 CTP 读取器把任意
`rewardboostinfo/*.ctp` 解成「列 + 记录 + 权重对」，可直接复核上面每一条。

## 继续挑战 / 右侧无缝续刷（`ABYSS-SEAMLESS-20260919.md` + `-LOADING-`）

### 核对：文档描述的两处根因与我们仓库逐字吻合

| 文档说法 | 我方现状 |
| --- | --- |
| 「旧版未发 `NOTI261`，所以发送结算并未启用按钮与右侧箭头」 | **`NOTI261` 全仓 0 命中** ✓ |
| 「旧版解码只支持选项 0～3」 | `internal/game/protocol/cards.go:74` 的 `DecodeSettlementExit`：`if (p[0] != 1 && p[0] != 2) \|\| p[1] > 3 { … "unsupported exit action" }` —— **一字不差** ✓ |
| 「把现有 CMD2062 当成小深渊续刷入口」 | 我方 `CMD2062` 确实被当成 `DUNGEON_DIRECT_MOVE` 的续刷 gate ✓ |
| IDA 地址（`1452f9bb6` 注册、`141801490` `isRetryDungeonEnable`、`144d30026` 右侧区域、`146ab83a0` 发 CMD72、`141c61790/141c618c0` 无缝标记） | 我方无对应取证记录（那是对方的逆向成果，**未复核**） |

⇒ 这两条缺口（**不发 NOTI261** + **CMD72 选项 5 被顶回**）是真实存在的，
且正是「继续挑战按钮置灰 / 右侧续刷不工作」的直接原因。

### ⚠️ 两份文档互相矛盾：ACK72(option5) 之后要不要发 NOTI27

| 文档 | 说法 |
| --- | --- |
| `ABYSS-SEAMLESS-20260919.md` | 「选项5保留ACK中的5，随后发送新一轮NOTI28/1766/2611/29，**不插入**返回地下城选择界面的NOTI27」 |
| `ABYSS-SEAMLESS-LOADING-20260919.md` | 「上一版把 option5 **误认为**需要省略所有入图重置。当前源分支在 ACK72(option5) 后**补发 NOTI27**，再发送 NOTI28/29」 |

后者明确在**纠正**前者（"上一版误认为…"），且给出机制：原客户端 `146d3dba0` 含
`CNSelectDungeonModule::onExitModule_SeamlessLoading` 并调用 `146d44d90`，
**NOTI27 同样调用这个清理函数**（清理 `controller+57ef95/+57ef98` 与旧地图状态），
原生回放确认状态 `1/17 → 0/0`。

⇒ **实现时应以 `-LOADING-` 那份为准：ACK72(option5) → NOTI27 → NOTI28 → NOTI29。**
两份文档都注明「仍未做实际视觉验收」，所以这一顺序落地后仍需实机确认（黑屏是否消失）。

### 勘误：我上一轮对 NOTI261 方向的「作废」是错的

上一节写了「前两轮『等服务端发 NOTI261/CMD72』的方向作废」。**这个否定过头了，撤回。**

玩家核实的是「**终点 GO = 继续挑战，是正确功能**」⇒ 那只证明
**PVF 不需要改**（箭头/按钮本身没错），**并不证明服务端不用发 NOTI261**。

两者的正确关系是：

- **显示** GO / 结算面板：客户端本地行为（`basic.act` 按「场上还有没有怪」决定）——
  **无需服务端参与，PVF 也不用改**；
- **可用**（按钮不置灰、右侧箭头出现）：按本文档，取决于 **`NOTI261` 的值 9**
  （`isRetryDungeonEnable()` 只在状态 == 9 时为真）。

⇒ 而当时「点 GO 没反应」的真正原因是**两者叠加**：既没通关（无结算面板），
也没发 NOTI261（按钮不可用）。**前两轮的方向是对的，只是缺了「PVF 不用改」这一半结论。**

### 待定项（实现前要定）

1. `NOTI261` 的正文与触发时机：文档说「已完成并已提交的小深渊，在 NOTI35 **之后**发送；
   剩余疲劳与门票/免票满足条件时发 9，不满足发 1」。我方要在哪条链上取「已完成/门票/疲劳」待定。
2. 选项 5 的准入：文档限定「源小深渊 `100004307`、指定球 `109017719` 及本角色已完成副本」。
   本副本（`100005068`）是否走同一套准入，需与实机核对。
3. 无缝标记（`141c61790/141c618c0`）是**原版 ACK72 自己的行为**，服务端不直接改客户端位置或二进制
   —— 这条与我方「不改客户端」的纪律一致。

## A 落地：继续挑战 / 无缝续刷（2026-09-26）

### L0 依据：opcode 名称来自我们自己的 dump

`analysis/dumps/opcodes.tsv` 直接给出了这条链每一环的名字，**不依赖外部文档的口径**：

```
cmd   72  0x0048  ENUM_CMDPACKET_EPLP_COMMAND
noti 261  0x0105  ENUM_NOTIPACKET_EPLP_RECHALLENGE      ← 再挑战
noti  27  0x001B  ENUM_NOTIPACKET_ENTER_SELECT_DUNGEON
noti  28  0x001C  ENUM_NOTIPACKET_DUNGEON_INFO
noti  29  0x001D  ENUM_NOTIPACKET_START_MAP
noti  35  0x0023  ENUM_NOTIPACKET_CLEAR_DUNGEON_REWARD
cmd   37  0x0025  ENUM_CMDPACKET_FINISH_LOADING
cmd 2062  0x080E  ENUM_CMDPACKET_DUNGEON_DIRECT_MOVE
cmd 1426  0x0592  ENUM_CMDPACKET_SELECT_CARD_SKIP
```

⇒ 「72 = EPLP_COMMAND、261 = EPLP_RECHALLENGE」由 dump 命名确认，
与外部文档「NOTI261 是允许再次挑战的通知」一致。

### 四处改动

| 文件 | 改动 |
| --- | --- |
| `internal/game/protocol/dungeon.go` | 新增 `EplpRechallenge(state)` + 常量 `EplpRechallengeReady = 9` / `EplpRechallengeBlocked = 1` |
| `internal/game/protocol/cards.go` | `DecodeSettlementExit` 放行选项 5（新增常量 `SettlementExitSeamless = 5`），范围守卫由 `p[1] > 3` 改为 `p[1] > 3 && p[1] != 5` |
| `cmd/wireprobe/card_flow.go` | `settlementExit` 新增 `case protocol.SettlementExitSeamless`，准入要求**已通关**，然后复用 `restartDungeon()` |
| `cmd/wireprobe/settlement_flow.go` | `dungeonResult` 在 **NOTI35 之后**追加 `NOTI261`；新增 `canRechallenge(ctx)` 决定发 9 还是 1 |

### 两个设计决定（都有理由）

**① 选项 5 复用 `restartDungeon()`，不另写一套入场序列。**
`restartDungeon` 内部就是 `dungeonSelectionHead()` + `dungeonEntryPlan()`，而

```go
func dungeonSelectionHead() []outboundPacket {
	return []outboundPacket{
		{"dungeon_gate_ack", 1, 15, []byte{1}},
		{"dungeon_selection_sent", 0, 27, protocol.EnterDungeonSelection()},
	}
}
```

⇒ **它天然就是「ACK15 + NOTI27 + 入场序列」**，正是较晚那份文档（`-LOADING-`）要求补发 NOTI27 的形状。
**同时**：ACK 里的 option 原样保留 5（`SettlementExitSuccess` 直接用 `r.Option`），
所以客户端仍走无缝重置路径，不会被改成普通重开的 option 0。

**② 亮 9 的条件与 `restartDungeon` 的准入共用同一套疲劳判定。**
这样「按钮亮着」⇔「按下去能成」，不会给玩家一个点了会失败的入口。
拿到的是 `w.fatigue.State` + `Rules.RoomCost`，与 `restartDungeon` 第 163-171 行逐字同源。

### 测试

`internal/game/protocol/seamless_rechallenge_test.go` 三条：

| 测试 | 钉住 |
| --- | --- |
| `...OptionIsAccepted` | `01 05 01` 被接受为 state 1 / option 5；且 `KeepsDungeonSelection()` 为假（它不保持选图流） |
| `...WideningDoesNotAcceptEverything` | **选项 4 与 6 仍然被拒** —— 放宽范围不等于全放行 |
| `...Body` | NOTI261 正文恰好 1 字节，9 / 1 |

### 待实机确认（不声称已完成）

1. **NOTI261 的两个值只来自外部文档**（我们的 L0 只确认了 opcode 名称与「读 1 字节」这条路）。
   9 / 1 的实际效果要实机看：结算面板里「继续挑战」是否由置灰变可点、右侧是否出现箭头。
2. **无缝续刷是否真的不黑屏**：较晚那份文档说明上一版「只发 NOTI28/29、漏掉 NOTI27」时客户端黑屏
   （收到 ACK72 后不回报 CMD37）。我们走的是补发 NOTI27 的那条，**需要连续向右续刷两轮**验证。
3. **准入范围**：文档把选项 5 限定在「源小深渊 `100004307` + 指定球 `109017719` + 本角色已完成副本」。
   我们目前的准入是**通用**的「已通关即允许续刷」，没有按副本 ID 收窄 —— 若实测发现别的副本
   不该出现这个入口，再补 ID 白名单。

## B 落地：调律之边界（深渊）专属奖励（2026-09-26）

实机验证 A 已通过（会话 `..._150623_606082`：一轮内连刷三场，
`eplp_rechallenge` ×3、CMD72 真帧 `01 05 01` ×2 + `01 02 01` ×1，三场都出了结算与翻牌）。
本节落地 B。

### 为什么不是"照文档写"

`attunement-repair-20260926.md` 描述了一条完整的实现（`LoadAttunementDrops` /
`attunement-candidates.json` / `attunementBossRolled`），但按纪律核过：**那些符号与文件在本仓库
0 命中**，提交也不在我们的历史里。所以本节只采用它**能在我们自己的数据上复核**的部分，
其余自己取证。

### 数据面：源 CTP → 生成配置

**我方已经有 CTP 解析器**（`internal/catalog/pvf/ctp.go`，`Archive.CTP()`；
`apocalypse.ctp` 一直在用）。三档表：

| 文件 | `[dungeon index]` | fixed | additional | hidden |
| --- | --- | --- | --- | --- |
| `unique.ctp` | 100005066 | 1 | 10 | 0 |
| `legendary.ctp` | 100005067 | 1 | 12 | 0 |
| `epic.ctp` | 100005068 | 3 | 14 | 6 |

新增 `cmd/attunementimport` → `configs/attunement-rewards.generated.json`（21 KB），
沿 `cmd/apocalypseimport` 的既有形态：从冻结的 PVF 直读，产物可复现、可与源哈希比对。

### 读取规则（由不变量钉住，不是猜的）

```
[dungeon index]           1 个数
[fixed drop table]        按 maze 各一份： [maze] + [drop list]
[additional drop table]   ×N： [effect index] + [select prob] + [drop count] + [drop list]
[hidden drop table]       按 maze 各一份： [maze] + N × [drop list]
[drop list]（fixed/additional）= (档位名, 权重, 物品) 三元组重复
[drop list]（hidden）          = 一个前导索引 + (档位名, 键, 物品) 三元组重复
```

**两条不变量使"三元组顺序是 (档位, 权重, 物品)"成为可证事实而不是约定**：

1. 每份 `[drop list]` 的权重合计**恰好 1,000,000**；
2. 整个 `[additional drop table]` 集合的 `[select prob]` 合计**恰好 1,000,000**。

若顺序是 (档位, 物品, 权重)，那么"权重"会是一串物品 ID（`10419728` 之流）而合计成百万 ——
不可能。导入器把两条都当**硬校验**：不成立就报错退出，绝不写出半可信的表。
⇒ 这也是 §B 敢发解码器的理由（对照 §3b 的教训：单样本规律不许当结论）。

**第 2 条同时定死了抽奖次序**：14 个附加分支不是"各判一次"，而是**从百万空间抽一个**。
否则它们的和正好等于百万毫无理由。

### 抽奖与发放

```
最终领主（源 [hunt boss] 模板 + rank3）确认死亡
  → 固定表：按当前 maze 取那一份，百万权重抽 1 件
  → 附加表：百万空间抽 1 个分支，再从该分支的 [drop list] 按权重抽 [drop count] 件
  → 追加进本轮的地板掉落（已有链路）→ 拾取 → 入包
```

**E 档一次清场 = 1 + {3,5,10,1} 个礼盒。**

奖励物**全部是 `stackable` + `stackable_type=[booster]` 的礼盒**（不是装备、也不是图标）：
`10419728` 的 PVF 定义里有 `[field image] Item/FieldImage.img`（会落地）与
`[booster info]` 奖池。我方 **booster 目录（`configs/booster-catalog.json`，42301 定义）
与开箱链路（CMD160 + `cmd/wireprobe/booster_flow.go`）都已实现**，
所以**直接发盒子即可端到端可用**，不需要服务端展开容器。

⇒ 这一点**故意不照文档做**：文档说"递归展开内部容器、只发真实装备"。
理由有二：① 源表没给"开盒"的随机性，展开就得替客户端决定盒内抽取，是两个关注点混在一起；
② 展开会把这个礼盒的存在从玩家体验里抹掉，而我们**没有任何证据**说明原服不发盒子。
盒子本身是真实物品、能落地、能开、能交易（`[attach type] [trade]`），这就是源数据的字面读法。

### `[hidden drop table]` 解析但**不发**，理由是可验证的

它的中间那个数**不是权重**：`epic.ctp` 里它按 0,1,2,…11 递进、与物品 ID 同步
（12 件连号，明显是"每职业一件"的指纹），权重不可能长这样。
⇒ 触发条件**未确立**，按纲领不发猜测的实现。

而且**本副本根本到不了它**：hidden 只存在于 epic 表的 `maze=1` 与 `maze=2`，
而我们的运行期 `maze` 恒为 0（实测三场连刷 `dungeon_session_started` 全是 `maze: 0`；
`dungeon.Select` 又按最小 index 确定性取 maze）。
⇒ 这是个**自动消失的未决项**，不是被绕过。

### 影响面与惰性

- 表只声明 3 个副本：`100005066/100005067/100005068`。
  我方目录只有后两档（`100005066` 不在 `dungeons.full.json` 里），**零误伤**。
- 端点集成测试反向核对：**目录里每一个 `AttunementBoss != 0` 的副本都必须在表里**
  （否则会有静默拿不到奖励的深渊副本）—— `marked dungeons 2`，全覆盖。
- `Roll` 对**没有表的副本**原样返回种子：不消耗本轮掷骰序列 ⇒ 这条线对全游戏其它副本完全惰性。
- 触发条件是**源脚本自己写的 `[hunt boss]` 模板**，不是"房间里有个 rank3"，
  也不是"副本已通关"。**故意不叠"已通关"那道闸门**：它与领主死亡锚在同一事实上，
  多一道闸门只会在时序不同时静默扣下奖励 —— 正是本轮要消灭的失败形态。

### 装配（三处，都有可见性）

| 位置 | 内容 |
| --- | --- |
| `internal/loot/attunement.go` | 目录 + 不变量校验 + `Roll` + `ValidateTemplates` |
| `internal/loot/session.go` `Death` | 钩子（与既有 Odyssey 章节盒同构），配 `attunementRolled` 一次性标记 |
| `cmd/wireprobe/main.go` | `-attunement-rewards`（默认取 `DFO_ATTUNEMENT_REWARDS`）+ 启动日志 |
| `dfo_probe_tools/channel_probe.py` | 导出该环境变量的**绝对路径**（gateway 的 cwd 是整合包根，内建相对默认值解析不了） |

启动日志：`loaded attunement rewards (3 dungeons [100005066 100005067 100005068], 94 reward templates) from …`。
缺表时打一条**显式告警**（`boundary-of-attunement clears pay no exclusive reward`），不静默。
**模板必须是已知 stackable 才允许启动**：未知模板会在拾取时被当装备拒收，
而玩家那时已经看到它掉在地上了 —— 这正是"静默失败"的形态，所以宁可启动失败。

### 测试

| 测试 | 钉住 |
| --- | --- |
| `TestAttunementRewardsLoadsTheShippedTable` | 真表可加载、三副本绑定、epic 带 hidden 而 legendary 不带 |
| `TestAttunementRewardsRefusesBrokenInvariants` | 8 条破坏路径全部被拒（权重和不等于百万、prob 和不等于百万、零权重、空物品、无副本、副本重复、分支不抽、model 不符）—— 靠**改动真实文档**生成，不是手写夹具 |
| `TestAttunementRollIsInertWithoutATable` | 无表副本：0 奖励 **且种子不变** |
| `TestAttunementRollPaysTheFixedBoxAndTheAdditionalBranch` | 4000 次抽样：件数必属 {4,6,11,2}、模板全在表内、primeval 实占比落在 8.57% 的窄带 |
| `TestAttunementRollStaysInsideTheMazeItWasAskedFor` | 按 maze 查表（不是"第一份表赢"）；未覆盖的 maze 不发固定面 |
| `TestAttunementValidateTemplates` | 缺模板 / 非 stackable 均被拒 |
| `TestAttunementBossPaysTheExclusiveBoxes`（门控 `ATTUNEMENT_REWARD_INTEGRATION=1`） | **真实 100005068 端到端**：20 轮 → 44 个专属礼盒 / 14 种；**非 boss 怪零专属产出**；目录里所有深渊副本都在表内 |

### 待实机确认（不替你宣布完成）

1. 打完柱子是否**落地**出礼盒（应当：1 个固定 + 若干附加）；
2. 拾取后能否**入包**（走 `[booster]` 的既有兜底槽位 65–120）；
3. 在背包里**开盒**是否正常出物（走既有 booster 链路）。
4. 一件仍**未动**的事：本副本杂兵的通用掉落路径没改。实测它本来就近乎零
   （`[exclude gold drop]` + 装备窗口只到 107），文档主张"杂兵不套用普通池"，
   但那是**改动行为**、且我们没有 L0 证据，所以留作未决项，不在本轮动。

### 校验（2026-09-26 15:32）

`go build ./...` + `go vet ./internal/... ./cmd/...` + `go test ./...`（日常；`-p 1 -count=1` 串行且禁 test cache，仅留给发布验证）
→ **exit 全 0，23 个包 ok、0 FAIL**；两条门控集成测试（`BORDER_DROP_INTEGRATION=1`、
`ATTUNEMENT_REWARD_INTEGRATION=1`）都通过；改动文件 gofmt 通过。候选 exe：

```
bin/wireprobe-handoff-source.exe
  9C9F8CD20039DBF9D21B127E0420DF82DFCFA1B85620E3D638C00E0F18DA45E6   19,622,912 B   15:32
（上一版 DBD90952…454B34(15:03) 已被覆盖；更早的 EFE29625/3DF45724/… 见 MEMORY.md 备份行）
```

新文件与改动清单：

| 文件 | 状态 |
| --- | --- |
| `cmd/attunementimport/main.go` | 新 |
| `internal/loot/attunement.go` | 新 |
| `internal/loot/attunement_test.go` | 新 |
| `cmd/wireprobe/attunement_reward_integration_test.go` | 新（门控） |
| `configs/attunement-rewards.generated.json` | 新（21,686 B，生成物） |
| `internal/loot/{pickup,session}.go`、`cmd/wireprobe/{main,dungeon_flow}.go`、`dfo_probe_tools/channel_probe.py` | 改 |

**未提交**（按纪律等实机验证）。`DFO-115US单机一键启动器.exe` 在 git 里仍显示被改动 —— **不是本项目动的**
（估为启动器自更新），收口提交时别带上它。

## 勘误 + 取证：「掉落等级偏低」不是索引缺失（2026-09-26 15:5x）

本文件「已知边界」一节原本写着「既有装备索引最高 grade 107 ⇒ 需另行重导更高等级的装备索引」。
**这句话是错的，撤回。** 取证如下。

### 事实一：索引里有高等级装备，而且运行期已经加载

| 目录 | 行数 | `[grade]` 范围 | 用途 |
| --- | --- | --- | --- |
| `configs/equipment.current37.json` | 3,174 | 1–107 | **掉落池**（`-equipment-catalog`）|
| `configs/equipment-full.*` | **424,216** | 1–**122** | 全量定义（`-equipment-full-catalog`）|

`101001153`（115 级剑，`grade=122`）与 `100313553`（`grade=122`）**都在全量索引里**，只是不在掉落目录里。
而全量目录**在运行期确实被加载**：实机 `gateway.err` 有一行
`separate wear catalog: 424216 records; original reward/drop catalog: 3174`。

⇒ **重导索引不会改变任何事。**

### 事实二：107 的上限是**掉落池规则**的必然结果，而且是有意为之

对全量目录抽样 12 万条（其中非 avatar 59,055 条）：

| 口径 | 结果 |
| --- | --- |
| 全部非 avatar | grade 1–122；`>=100` 的 9,077 条，**全部**落在 `[100,148)` |
| 按掉落池规则（`[attach type]==[free]` 且 `[rarity]<=2`）筛 | 接受 2,394 条（4.05%）；**grade 上限恰好 107** |

被拒的理由分布：`attach != [free]` 共 50,458 条（`[trade]` 39,362、`[avatar trade]` 6,592、
`[sealing]` 3,064、`[trade delete]` 1,166、`[account]` 64…），`rarity > 2` 或缺失 4,953 条。

`internal/inventory/equipment.go` 里这件事是**写明的设计**：`Reward()` 的注释说它
「keeps every structural check Basic makes … **but not Basic's two drop-pool rules**: a reward may be
account- or character-bound, and it may be rarer than rare」，并附了实机依据（任务 21650 的 `100261068`）；
`Basic()` 则把这**两条额外规则**用于掉落的池子。`internal/inventory/equipment_catalog37_test.go`
正是钉住这件事的：`current37` 之所以比 `current35` 宽，只是为了**能发任务奖励**，
测试注释写着这些行「**must not quietly become monster drops**」，并断言
`wide.Basic(100261068)` 必须失败、`Reward` 必须比 `Basic` 宽。

⇒ **所以要提高掉落等级，改的是规则（谁可以掉），不是索引（谁能被描述）。** 这是策略变更。

### 事实三：深渊的专属奖励**本来就能给高等级装备**

把三档 CTP 的 94 个礼盒按 `configs/booster-catalog.json` **递归展开到底**：

| 难度表 | 礼盒数 | 展开后装备件数 | 底层 stackable 类型 |
| --- | --- | --- | --- |
| `unique.ctp`（100005066） | 27 | **48** | `[virtual]` 57、`[booster]` 20、`[material]` 8 |
| `legendary.ctp`（100005067） | 27 | **49** | `[virtual]` 56、`[booster]` 27、`[material]` 11、`[etc]` 3、`[booster selection]` 1 |
| `epic.ctp`（100005068） | 24 | **49** | `[virtual]` 45、`[booster]` 19、`[material]` 10、`[booster selection]` 1 |

其中的装备是 **`grade=116`、`rarity=3/4`、`[minimum level]=115` 的 `[oath]` 系列**——
**比通用掉落池的 107 上限更高**。它们能发出来，是因为礼盒内容的授予走
`boosterEquipmentDurability → WearService.Catalog.Reward`，而那份目录挂着 `Full`（全量定义），
**不走掉落池**。

⇒ **「深渊给不给高等级装备」这件事已经成立**；通用掉落池的 107 上限与它无关。

### 于是真正要定的是这个（三选一）

| | 做法 | 代价 |
| --- | --- | --- |
| **A** | **不动**。承认现状：深渊的高等级产出走礼盒内容，通用掉落池维持 `[free]`+rarity≤2 | 零风险；但需接受「通用掉落永远不超过 grade 107」 |
| **B** | **放宽掉落池到 `Reward()` 口径**（即删掉 `Basic` 的两条额外规则） | 池子从 3,174 涨到约 5 万行量级；**全游戏所有副本的掉落都变**；推翻一条有实机依据 + 测试钉住的既有决定，必须说明那条决定的适用边界 |
| **C** | **只给需要的内容开一条更宽的池**（例如 145 档/深渊），通用池不动 | 新机制，改动面比 B 小、零回归；但要定义"谁用宽池"的判据 |

倾向：**先确认症状到底是哪一个**。若玩家的体感是「盒子打开后大头是虚拟道具/材料」，
那落点在**礼盒内容的权重**（B/C 都治不了）；若目标是「深渊的杂兵也该掉装备」，
那才是 B 或 C。取证工具：`runtime/equipfullsurvey.py`（全量目录的 accept/grade 交叉统计）、
`runtime/ctp_show.py`（CTP 逐格）、以及 `configs/booster-catalog.json` 的递归展开。

## B 落地：奖励就地「展开一层」（2026-09-26）

### 玩家侧真值（本轮的决定性输入）

- **打死后直接出现**（不是先拿罐子再自己开）；并且**开过罐子，能出东西但不对**。
- 三样东西：**装备直接掉装备**、**星蕴石发一个粉色的罐子**、**誓约掉一个随机的书**。

⇒ 上一轮「直接发盒子、服务端不展开」的选择**是错的**。源把每件奖品都包了一层
`[booster]`，玩家该看到的是**开一层之后**的东西。

### 展开深度 = 一层（推断，但有反证支撑）

`RewardBoxDepth = 1`。依据是三条真值只有一种深度能同时满足：

| 玩家说法 | 展开一层 | 递归展开到底 |
| --- | --- | --- |
| 装备**直接掉装备** | `10419728.pool2 → 100401592`（装备）✓ | ✓ |
| **誓约掉一个随机的书** | `pool1 → 10420672`（书族 `consumption_2.img 914`）✓ | ✗ 书会被继续展开成 `10419719` → 某件具体誓约装备 |
| **星蕴石发一个粉色的罐子** | `fx=2001 → 10409486 → 10409489`（罐子）✓ | ✗ 罐子会被展开掉 |

⇒ 玩家明确说收到的是**书**和**罐子**，而书/罐子**本身就是奖品**（由玩家自己开）。
再往下开会把源真正要发的物品溶掉。第三方文档写的是「递归展开内部容器」，与这两条真值冲突，
**不采纳**。

### 落地（只碰服务端）

| 位置 | 内容 |
| --- | --- |
| `internal/loot/reward_box.go`（新） | `RewardBox` / `RewardBoxPool` / `RewardBoxCandidate`、`RewardBoxSource` 接口、`OpenRewardBoxes`、`RewardBoxDepth` |
| `internal/loot/attunement.go` | 新增 `RolledTemplates()`（只含可发的 fixed+additional，**排除 hidden**）与 `ValidateBoxes()` |
| `internal/loot/session.go` | `RewardBoxes` 字段；`Death` 在调律分支 `Roll` 之后**就地展开** |
| `internal/loot/pickup.go` | `Service.RewardBoxes` |
| `cmd/wireprobe/booster_flow.go` | `boosterBoxSource`：把既有礼包目录接到展开上 |
| `cmd/wireprobe/main.go` | 接线 + **硬失败**（有奖励表却没有礼包目录 ⇒ 拒绝启动）+ 启动日志打出空槽模板 |
| `cmd/wireprobe/dungeon_flow.go` | 会话透传 |

### 规格（全部由源数据判定，不是猜）

- **空槽靠数据判**：池里某个模板在**物品目录里不存在** ⇒ 那是源的「本次没有」，发 0 件，
  **不发明成物品**。可发集合里只有 `12` 一个（固定载体 pool2 的 47.45%）。
  `12` 是保留 id：`items.index` 在 0–199 的稠密段里恰好缺 **12 / 13 / 17**。
- **hidden 表不进展开校验**：它有 48 个包装指向另一个保留 id `490000001`，而 hidden 只在
  maze 1/2，**运行期 maze 恒为 0** ⇒ 永远到不了，也不该当成可发奖励。
  于是 `RolledTemplates()` = **46**，`Templates()` = 94。
- **开箱规则与既有开箱路径一致**：`draw_count` 缺省为 1、零权重按 1 算、权重不覆盖时取最后一个
  候选。这样「手动开罐」与「掉落时展开」对同一件东西给同一个分布。
- **没有实现通用 `-1` 迷宫回退**：本配置的 `[maze]` 只有 0/1/2，源里没有 `-1` 行；
  真出现了再按证据加。

### 离线验证（2000 次运行，`runtime/boxprobe`）

| 落地面 | 观测 | 源权重 |
| --- | --- | --- |
| `100401592` 装备 | 547 / 2000 = **27.4%** | 27.20% |
| `10420672` 等书 | 239 / 2000 = **12.0%** | 12.00% |
| `10409489` 星蕴石罐 | 151 / 2000 = **7.6%** | 7.50% |
| `10420594` 自选箱 | 34 / 2000 = **1.7%** | 2.00% |
| `10419720` 虚拟道具罐 | 506 / 2000 = **25.3%** | 25.35% |
| `1` 金币 | 1646 / 2000 = **82.3%** | 82.24% |

**外层包装出现在地面上的次数 = 0**（2000 次运行）；每场最少 4 行、平均 7.04 行。
⇒ 分布与权重逐项吻合，说明展开路径与字段读法都对得上。

### 测试

- `internal/loot/reward_box_test.go`（新，7 条）：内容而非包装落地、`draw_count`、缺省为 1、
  空槽发 0 件、**恰好一层**（内层包装原样落地）、不可开的东西原样透传、nil 源惰性、确定性。
- `internal/loot/attunement_test.go`（+2 条）：`RolledTemplates` 排除 hidden；`ValidateBoxes`
  拒绝不可开的包装，并把空槽报出来。
- `cmd/wireprobe/attunement_reward_integration_test.go`（重写）：真实 `100005068`，60 次运行，
  断言**地面上没有任何外层包装**、每次至少一行保底材料、装备/书/星蕴石罐都出现过，且
  **非 boss 怪从不携带调律专属行**（70 个保底材料是调律独有的指纹 —— 通用掷骰一次只给 1 个）。

### 仍未做（说清楚）

- **`490000001` 未解**：只出现在 hidden 表，运行期到不到，未展开。
- **装备池等级上限仍是 107**：与本轮无关，另案（见上文勘误）。
- **杂兵的通用掉落未动**：第三方文档主张「杂兵不套用普通掉落池」，那是**改行为**且无 L0 证据。

### 校验（2026-09-26 16:41）

`go build ./...` + `go vet ./internal/... ./cmd/...` + `go test ./...`（日常；`-p 1 -count=1` 串行且禁 test cache，仅留给发布验证）
+ 两条门控集成 → **全 0，23 个有测试的包 0 FAIL**；改动文件 gofmt 通过。

候选 exe（源码 = 本次改动）：

```
bin/wireprobe-handoff-source.exe
  E8BFA54AD3F4FB5498AD48F1D67AD2940698AA98CF8530F32D0B246819F4ADFC   16:41   19,617,280 B
（上一版 9C9F8CD2…DA45E6 为 15:32）
```

启动日志新增一行（可用来现场确认装配）：

```
loaded booster catalog (N definitions, M item index entries)
loaded attunement rewards (3 dungeons [...], 94 reward templates) from ...
attunement reward wrappers open one layer; 1 empty-face templates: [12]
```

## 奖励不再发包装：递归展开到产物（2026-09-26 晚）

### 触发：玩家侧的实机真值与截图

玩家反馈**打出来的盒子类道具无法打开**，并给出客户端截图：`神器星蕴石` 的说明写着
**「(不实际发放礼盒,以开封状态发放)」**。⇒ 上一轮「展开一层」仍然不对：**地面上不该出现任何盒子**。

同时玩家给出了千海天（115 级）两套深渊的产出说明，其中**令牌深渊（= 本副本「调律之边界」）**：

> 入场消耗：对应品级令牌×3 + 深渊票（神器85张/传说350张/史诗750张）
> **保底掉落：对应品级的115级装备（神器/传说/史诗）**
> 随机额外掉落：稀有~太初星蕴石随机罐（账绑）、粉~太初誓约随机罐（账绑）
> 注意：大深渊保底的是现版本115级粉~史诗装备，**不保底誓约和星蕴石**；罐子均为账号绑定，不可交易。

### 改动（服务端）

`internal/loot/reward_box.go` 的 `OpenRewardBoxes` 从「展开一层」改为**递归展开到产物**：
反复把包装换成它的内容，直到没有包装为止；`RewardBoxSource` 增加第三个方法 `Container`，
用来识别「**本 build 打不开的盒子**」——它同样**不许落地**（自选箱需要玩家做选择，服务端替不了）。

分类只在一处发生（可展开 / 打不开的盒子 / 空槽 / 产物），这是本轮唯一一个实现陷阱：
第一版在池内先判「不在物品目录 ⇒ 空槽」，于是**可展开的包装也被当空槽丢掉**，
`TestOpenRewardBoxesUnwrapsToTheProducts` 一次就把它抓出来了。

三种不发放的情形都会**记进 `SkippedKinds`**（落进实机 `drop_rules_pending` 事件），不静默：
- `attunement_empty_prize_<id>`：源用保留 id 表示的「本次没有」；
- `attunement_unopenable_box_<id>`：本 build 打不开的盒子；
- `attunement_reward_nesting_too_deep`：嵌套超过 8 层的防御（出货表最深 3 层）。

`ValidateBoxes` 改为遍历**整棵树**，启动期报告空槽与打不开的盒子；启动日志会多一行
`warning: attunement rewards name N box(es) this build cannot open; they will not be paid: [...]`。

### 递归展开后的落地面（`runtime/boxprobe`，2000 次运行）

```
包装落地面 = 0（既无奖励项包装，也无嵌套包装）
打不开的盒子（报告但不发）= 10420581 10420594（两个自选箱，各占一条 2% 的分支）
每场行数：最少 3、平均 7.02
family 分布：
  equipment      901 行 / 2000 场（`primer/100401592` + `equipment/character/common/oath/*` 48 件）
  stackable    13137 行（材料 `10362432`/`10400396`/`10403609`、金币 `1`、以及约 25 种 `[virtual]`）
```

### ⚠️ 残留不确定，必须说清

1. **`[virtual]` 这批（约 25 种）身份未确证**：`grade=1`、`rarity` 3/4/6/8、`[trade]`、
   **`[name]` 段与 `[explain]` 段全为空**、图标与 94 个奖励项**同为 `consumption.img 1589`**。
   它们最可能是**星蕴石**（档位名与其父包装 `10419730..10419736` 的 normal/rare/unique/legendary/
   epic/primeval 对得上），但**客户端没有可用中文名表**可核，所以这是推断而非确证。
   若实机发现这些物品不可用/显示异常，退路是**保留其父包装**（即回到「展开一层」）—— 切换点只有一处。
2. **文档说罐子是「账绑」，我们的产物不是**：`10403609`（primestella 那一支的产物）的
   `attach type=[trade]`，`10362432`/`10400396` 才是 `[account]`。若「账绑」是关键约束，
   需要另做「投递时改用账号绑定版本」的处理。

### 本轮暴露的另一个缺口：**副本自己的「保底装备」组从未接线**

玩家给出的产出说明写着大深渊**保底掉落对应品级的 115 级装备**，而我们目前**一件保底装备都没有**。
根因已在源数据里定位：

副本脚本（`orderoftheborder_epic.dgn`）自带一段 `[difficulty dropitem group list]`，
其三段 `[group info]` 里写着 `[normal group index] 1 21251 1 21600/21601/21602`，
而 `etc/dungeondroptablebygroup.etc` 的 **`[group] 21251` 是一组 11 件真实 115 装备**：

```
100051304 jacket/cloth   100101187 pants/cloth    100151128 shoulder/cloth
100201100 belt/cloth     100251140 shoes/cloth    100301847 amulet
100313550 wrist          100323440 ring           100345985 support
100354160 magicstone     100391038 earring        （各 weight 10 ⇒ 随机一件）
```

`[group] 21600/21601/21602` 则是 `10420063/64/65`（`[booster]`）。
另外 `[special setinfo reward]` 四行带**语义标签**，指向另外四组：
`21279 <SetEquipmentReward>`、`21468 <SetOathPrimerReward>`、`21470 <RareEquipmentReward>`、
`21310 <WeaponEquipmentReward>`；这几组的内容是 `[etc]` 堆叠物
（`10401449..`、`10419326..`、`10419544`、`10336310`）。

**现状**：`internal/catalog/droptable.go` 能解析这张表，`LootCatalog` 也有 `DropGroups` 字段，
但**出货的 `configs/loot.*.json` 里 `dropGroups` 是空的**（当时为避开无关漂移没有重导），
所以运行期**根本没有这张表**，更没有消费它的代码。⇒ 「保底装备」是下一轮要补的正题。

### 另记：普通深渊（小深渊，「最终调律者」）不在本轮范围

玩家给的说明里另有一整套小深渊系统（入场 62 票 + 8 疲劳、**征兆系统 1~4 阶段**、
深渊裂缝 / 神秘好运特殊事件、**天平 NPC 商店**用装备灵魂兑换星辰/共鸣/超越天平、小鸟票 10 万）。
它是**另一条内容线**，与本副本无关，本轮只登记不实现。

## 小深渊（普通深渊「最终调律者」）：定位、实机取证与「源领主」泛化（2026-09-26 晚）

### 它是哪个副本：`100005014` `endkeeperoforder`

玩家给了副本卡截图（标题「深渊 : 最终调律者」，`Lv. 115`，画面是天平）。定位依据全部是 L0 资源：

- `contents/2026/endkeeperoforder/` 下有 **`passiveobject/omen_drop{,_1..4}`、`text_omen`**（= 玩家文档里的
  **征兆系统**）、`oath_drop_maker` / `oath_camera`（誓约）、`special_entrance`、`move_map`、
  `screen_arrow_maker`（GO 箭头）、`common/{unique,legendary,epic,primeval}drop*.ani`、
  以及 `etc/endkeeperoforder.ctp`（一张奖励表）。
- `monster/` 顶层 `.mob`：`dreadrift_hell` / `harvex_hell` / `ruinbound_hell`（深渊派对怪）、
  `orderwatcher`、以及 **`scale_oath` / `scale_primer` / `scale_sandbag`** —— **天平就是它**。
- 脚本（`contents/2026/endkeeperoforder/dungeon/endkeeperoforder.dgn`）：
  `[minimum required level] 115`、`[basis level] 145`、**`[use fatigue only start dungeon] 8`**
  （对上玩家文档的「8 疲劳」）、`[limit party count] 4`、`[keep character death] 1`、
  **`[clear condition] [hunt boss] 109019266 1`**（两个 maze 各声明一次）。
- 地图 `100016614_normal.map` / `100016615_special.map` / `100016616_special_phaseshift.map`：
  maze 0/1 各 13 行 —— 11 只 `rank=0` 杂兵（`109019402/3/4`）+ **`109019266` `rank=3` `[fixed] [boss]`**
  （天平，坐标 3168,191）+ `109019280` `rank=0`（**与 boss 同坐标**）。

### 实机（会话 `..._20260926_211558_616900`）

```
13:37:54.733  dungeon_session_started {dungeon:100005014, map:100016614, maze:0, monsters:13}
13:38:02 起  CMD39 ×11  → monster_death_ack / confirmed / experience 各 11 条
             （11 只杂兵 + entity 4108 = 109019280，全部正常处理）
之后         只有 CMD38（开门）与 CMD18/CMD48 的反复，直到 13:38:57 断开
```

**boss `109019266`（entity 4107）一次都没有上报死亡**，也没有 CMD117、没有结算事件。⇒ 两个独立问题：

1. **天平（`109019266`）打不死。** 玩家在 boss 的同一坐标能把 `109019280` 打死，却打不死 BOSS 本身。
   服务端**不下发 HP**（客户端按自己的模板数据算伤害与血量），所以这不是服务端的数值问题 ——
   需要另查（候选：该 boss 的受击判定不在普通攻击上，或客户端在等一个脚本事件）。
2. **就算它死了也不会结算。** 我方 `tryComplete` 的三条免 CMD117 路径全部要求 `!hasFightableBoss()`，
   而它是 `rank=3` 的可战斗 boss；`AttunementBoss` 的识别又只认
   `[dungeon type] == boundary of attunement` ⇒ `completed` 永远不会置真。
   **这一条本轮已修。**

### 修复：把「源领主」从玩法专属放宽成脚本自己的声明

`DungeonDefinition.AttunementBoss` → **`SourceBoss`**：

- **解析**：在 `[clear condition]` 段内读 `[hunt boss] <模板> <数量>`（不再限定 `[dungeon type]` 与
  `[limit party count]`），要求**所有段指向同一个模板且数量为 1**；不成立就保持 0。
  作用域限定在 `[clear condition]` 是刻意的：`[hunt boss]` 也出现在别的块里，只有通关条件是对结算的声明。
- **通关判定**：`tryComplete` 在 `completionTarget == 0` 块内新增/改写那条路径，并补上与上面两条
  display-boss 路径**同形的两道守卫** —— **在脚本声明的 boss 房间**（`atSourceBossMap()`）
  且 **房间里没有活着的可击杀目标**（`roomEnemiesDead()`）。这条路径在
  `completionTarget == 0` 之内 ⇒ **客户端会发 CMD117 的副本根本走不到它**。
- **身份与奖励**：`CompletionTarget()` 与掉落的奖励触发（`loot.Session.Death`）同步改用 `SourceBoss`。
  奖励仍由 CTP 表把关（`Roll` 按副本查表，查不到原样返回种子）⇒ 放宽触发器**不会**给别的副本发奖。

**影响面**（`runtime/sourcebossaudit` 可复算，逐副本核对自己地图里的 rank-3 `[boss]` 行）：

| 项 | 数 |
| --- | --- |
| 出货副本总数 | 3200 |
| 声明了源领主的副本 | **325** |
| 其中值能对上自己地图里 rank-3 `[boss]` 行的（路径可能生效） | **279** |
| 对不上的（运行期惰性，永不触发结算） | 46 |
| 值 < 100000、疑似不是模板 id 的 | 19 |

⇒ 泛化的实际可达面是那 279 个「声明了一个真实 boss 的副本」，而且只在这类副本**不发 CMD117**
时才生效。「调律之边界」`100005067/68` 与「最终调律者」`100005014` 都在其中。

### 仍然未做

- **天平（`109019266`）为什么打不死**：未解。在没有实机复现前不动服务端（候选见上）。
- **小深渊的奖励**：`etc/endkeeperoforder.ctp` **不是奖励表**（它是征兆/誓约掉落装置的参数表）；真正的掉落表是 `etc/rewardboostinfo/endkeeperoforder/normal.ctp`，**2026-09-27 04:0x 已接入**（生成器 `-extra` + 原运行时零改动，见 `endkeeper-of-order-primer-20260926.md` §16）。`[difficulty dropitem group list]` / `[normal group index] 1 21251` 仍未接线；`[coupon drop table]` 五行导入但按「语义未确立」不抽。
  **2026-09-27 03:2x 实机复核（天平判死绕过生效、已结算）**：本场只掉出`1 × 100130928`（重甲下装，`[rarity]=2` 紫、grade 107、min lv105）+ 金币 3293/2810/3165 + 3,151,739 经验；`dungeon_clear_reward`(NOTI35) 里**只有经验与 627 金币卡**。原因是这三条独立缺陷叠加：① 本表未接线；② 通用掉落池 `Basic()` 上限 `[rarity]<=2`；③ 天平 `primer_max=72>69` 钉死 `GO_RAINBOW1` 分支 ⇒ `summon_orthaire` 死代码、隐藏 BOSS `109019264` 从未登场。详见 `endkeeper-of-order-primer-20260926.md` §15。
- 「小深渊」玩家文档里的其它系统（征兆 1~4 阶段、深渊裂缝、神秘好运、天平 NPC 商店）都还没做。
