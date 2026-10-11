# next58 — 第三方补丁复刻手册的落地（P1 开箱耐久 / P2 admin 补挂 / P3 子项 6 自选盒）

## 0. 来源与差异盘点

来源：用户提供的第三方手册《115us / DFO 单机兼容服 —— 补丁复刻手册（2026-09-22）》
（其工程与本仓库同源，但**没有 Git 仓库**，故其手册给的是 unified diff 与"锚点字符串"）。

与我们仓库逐项盘点后的结论：

| 手册条目 | 我们仓库盘点结果 | 处理 |
| --- | --- | --- |
| 第 2 章 P1 开箱装备耐久恒 0 | `booster_flow.go` 的 Destination 3 确实写死 `Durability: 0`；`EquipmentCatalog.Reward`、`WearService` 都在 | **已复刻** |
| 第 3 章 P2 `cmd/admin` 发不出非掉落物品 | `cmd/admin/main.go` 与手册"原状"一致（内联装载、无补挂）；`LootCatalog.SupplementStackables` 已存在 | **已复刻** |
| 第 4 章 P3 奥德赛十项 | **11 个新源码文件 / 7 个测试 / 3 个数据文件全部不存在**，所有事件串 0 命中 | 按用户选择先做**子项 6** |
| 子项 2（等级赠装）、子项 7（掉落标签） | 手册结论是"核对后不改" | 未动 |
| 子项 5（银币/金币掉落） | `configs/odyssey-currency.json` 已在，`channel_probe.py` 原本就会注入 | 未动（手册也只是"显式化"） |

手册提到的取证脚本目录 `runtime/_probe/`（`parse_booster.py`、`dump_fields.py`）在本仓库**不存在**，需要时再补写。

## 1. P1 — 开箱常规装备耐久 0

- 根因：Destination 3 分支把 `Durability` 写死 0，而任务/GM 发放（`Bag.AddEquipment`）与掉落都走
  `EquipmentCatalog.Reward(id)` 读源 `.equ` 的 `[durability]`。客户端把耐久 0 当成"已损坏"：面板 0/60、穿上不生效。
- 实现：新增 `boosterEquipmentDurability(wear, id)` → `wear.Catalog.Reward(id)`；解析不出只记日志按 0 发放，
  **不拒绝开箱**（"拿不到东西"比"拿到 0 耐久"更糟）。
- 测试：`cmd/wireprobe/booster_flow_test.go` 的 `TestBoosterEquipmentGrantUsesSourceDurability` 同时断言存档
  `Durability` 与**下发 NOTI13 行偏移 11**，避免"存档对了但客户端还是 0"的假修复。
- **存量核对（只读，2026-09-22）**：`characters` 表 11 个角色的 `inventory.equipment` 里，
  四个报告模板 `100051398 / 100101272 / 100101273 / 100101275` **一条都没有**（文本级扫描亦为 0）⇒ **无需改档**。
  现存 4 行 0 耐久分别是 `100302054 [amulet]`、`100313767 [wrist]`、`100323647 [ring]`、`100332642`，
  都在 `durabilityOptional` 部位，源里本就没有 `[durability]` 段，**0 是正确值**。

## 2. P2 — `cmd/admin` 发不出非怪物掉落物品

- 现象：`bin/admin.exe -item 10418036x1000` → `item 10418036: invalid equipment award`。
- 根因：`inventory.Awarder.Grant` 用 `Catalog.Items[id].Kind == "stackable"` 选分支；模板不在目录里时 Go map
  返回零值 `Kind == ""`，于是落进装备分支。奥德赛银币/金币按设计只出现在 `items.index.json`（599771 条），
  不在 `loot.next25.json`（1022 条，怪物掉落）；网关 `cmd/wireprobe` 用 `SupplementStackables` 补挂，admin 从未做。
- 实现：`cmd/admin/main.go` 新增 `-item-index` 与 `buildAwarder`/`fileExists`，装载顺序与网关一致；
  显式路径不存在或补挂失败一律 **fatal**（半装载目录下静默发货比拒绝更糟）。
- 测试：`cmd/admin/main_test.go`（补挂后两个货币可达且 `Kind == "stackable"`；省略 flag 时在 loot 目录旁自动发现；
  坏 loot 目录/坏 index/坏 bag rules 必须拒绝）+ `internal/inventory/odyssey_coin_grant_test.go`
  （真实 `Awarder`：发 1000 落消耗品槽 65..120，再发 1000 另起一叠而不是被拒）。

## 3. P3 子项 6 — 自选礼盒报「库存已满」（根因修复）

### 3.1 根因

`[booster selection]` 自选盒的源语义走 `[booster select category]`，与固定内容盒的 `[booster info]` 在源里互斥，
因此它们**天然不在** `booster-catalog.json`（`cmd/boosterexport` 只导 `[booster info]`）。
`openBoosterItem` 落到通用随机池分支后回 `booster %d has no reward pool defined`，上层统一回 `Refusal(4)`，
客户端把任何 `0x0004` 都显示成「The target inventory is full」——**别按字面去查背包容量**。

### 3.2 真源结构（`server/work/client-build/Script.inner.pvf`，只读）

`stackable/10417001/10417789.stk`（奥德赛武器盒）typed cells：

```
[stackable type] `[booster selection]` 0
[booster category num] 2 17 5          ← 17×5 = 85 个类别
[booster selection num] 1
[booster select category]
  0 0                                   ← 段首两个数字 = category [2]byte
  [booster equipment grade] 4
  [recommend] 2 <id> <id>
  [equipment] <id> <count> <id> <count> …
[/booster select category]
```

`stackable/10307001/10307659.stk` 这类**索引误标**盒只有 `[booster info]`，没有 `[booster select category]`。

### 3.3 导出（只读，可复现）

`cmd/selectionboximport`：按上面的 token 形态解析；默认**有界**导出 =「booster 目录候选 ∪ loot 目录 ∪
`configs/*.json`（跳过 >8 MB 的大导出）引用到的模板」，产物 `configs/selection-boxes-candidate.json`
（**2878 盒 / 13 MB**，全量则 16750 盒 / 78 MB）；`-debug` 打印单盒解析结果，`-bounded=false` 全量。

一致性验证（与既有 `configs/odyssey-weapon-box-release.json` 逐项比对）：
**85 个 category 全等、items 0 处不符、脚本 sha256 同为 `d67f5042a5e17ef30581e297f030561de39f92ad5e215d742ec067aeaad96bec`**。
有界集合里 `fixed = [490022952]`（误标固定盒），`unparsed = [10358468]`。

### 3.4 实现

- `internal/catalog/selection_boxes.go`：`LoadSelectionBoxes`（模型/来源/盒键/脚本哈希/类别/物品全量校验）、
  `ByTemplate`、`IsFixed`、`Resolve(template, category, picks)` —— 把玩家的选择限制在该盒该类别的源集合内，
  越界、错类别、重复、空选择都给出精确错误（**服务端校验，不采信客户端**）。
- `cmd/wireprobe/booster_flow.go`：在奥德赛武器盒特判之后、通用事务之前接住自选盒（武器盒保持原链路）：
  - 有选择 → `Resolve` 校验，通过后**交给既有目的地分发**（装备/装扮/宠物/堆叠物混装；盒 `10335328`
    就同时带装备与 1000/10000 数量的材料），不自己另写一套发放；
  - 没有选择 → 回一个空成功 `selection_box_awaiting_pick`（让客户端弹出选择列表），不再回通用失败码；
  - 越界选择 → 直接拒绝。
- `cmd/wireprobe/main.go`：`-selection-boxes` / `DFO_SELECTION_BOXES`，默认按
  `configs/selection-boxes-release.json` → `configs/selection-boxes-candidate.json` 寻找。

### 3.5 测试

- `internal/catalog/selection_boxes_test.go`：真源目录（武器盒 85 类别、首类别 items、脚本哈希）、
  `Resolve` 的正例与四类反例、误标盒 `IsFixed`，以及畸形产物必须被拒（外模型/空 boxes/键不匹配/零 item/坏哈希）。
- `cmd/wireprobe/selection_box_test.go`：真实自选盒 → 选择装备 → 发放且扣盒；越界选择被拒；
  无选择的打开请求回 `selection_box_awaiting_pick` 且不改动背包。

### 3.6 待实机验证（重要）

1. 右键装备自选盒：应弹出选择列表并能正常开箱；`events.jsonl` 里**不应**再出现该盒的 `booster_action_refused`。
2. ⚠️ "**无选择的打开请求回一个空成功**"这一步是依据手册推断的（手册只说"先拦截、杜绝落空"）。
   服务端在遇到这种请求时会打印
   `selection box %d at slot %d asked without a pick (category=%d): awaiting client selection`，
   实机跑一次即可判定：若日志里从不出现这行、却仍报「库存已满」，说明客户端其实带了选择而在解码环节丢失，
   下一步应查 `protocol.DecodeBoosterUseRequest` 的试探式解析（该解码器有多种试探格式 + fallback）。
3. 回归：奥德赛武器盒（10417789）走的仍是既有 `selectOdysseyWeapon` 链路，装扮/宠物自选盒走既有目的地分发。

### 3.7 `[stackable]` 内容段建模 —— 每日礼包「选 x45 只到 1 个」（2026-10-11）

**现象（业主实机）**：领到「Royal Pass 每日登录礼包」`590722589`，开箱选
`Radiant Harmonious Crystal (x45)`，进背包只有 **1** 个。

**根因**（`gateway.err` 原文）：

```
selection box 590722589: category [0 0] has no exported item set
  (unmodelled content block or unknown category); the pick goes down the generic path
```

真源 `stackable/cash/590722589.stk` 的分类块写的是 **`[stackable]`** 而不是 `[equipment]`：

```
[booster select category]
0 0
[stackable]
10327284 90   10327285 45   10362432 50
[/stackable]
[/booster select category]
```

`ParseSelectionCells` 原本把 `[stackable]` 与 `[avatar]/[etc]/[cera]` 一起当「未建模段」跳过
⇒ `Resolve` 的件数表为空（`checked=false`）⇒ `openBoosterItem` 里 `count := req.Amount`（=1），
只有 `selectionCounts` 命中源件数才会走 `declared * req.Amount`
⇒ 回落到**礼盒自身数量**（源 `[daily reward list]` 写的就是 `590722589 1`）。

**全量形状实测**（一次性审计程序，16757 个 `[booster selection]` 候选）：

| 项 | 数 |
| --- | --- |
| 带 `[stackable]` 的盒子 | 3178 |
| `[stackable]` 块（成对 `模板 数量`） | **17660** |
| `[stackable]` 块（**5 个数字一条**，形状未定） | 33（如 10404663 的 `10404771 1 2 115 108 …`） |

**实现**（`internal/catalog/selection_cells.go` + `selection_boxes.go`）：

* `[stackable]` 按与 `[equipment]` **同一口径**建模：读 `(模板, 数量)` 对。
* **只建模成对形态**：数字个数为奇数 / 出现非正数 ⇒ 整块不建模，并在 `Sections` 里记
  `selectionSectionStackableUnparsed`（= `"[stackable] (unparsed)"`）；
  `hasUnmodelledSection` 照旧认它 ⇒ 那 33 个块**语义与改动前完全一致**（宁可少一次校验也不猜）。
* `hasUnmodelledSection` 放行 `[stackable]`（它已是可逐项比较的段）。
* `booster_flow.go` 的「命中源范围」日志补上解析结果，便于实机判读：
  `… in category [0 0]: [{10327285 45}]`。

**影响面核对**（同一审计程序，改前 → 改后）：

| | 改前 | 改后 |
| --- | --- | --- |
| boxes / fixed / unparsed / rejected | 16749 / 2 / 3 / 3 | **16749 / 2 / 3 / 3**（分类计数一处未动） |
| `590722589` 分类 `[0 0]` | sections=`[[stackable]]` **items=0** | **items=3**：`10327284×90`、`10327285×45`、`10362432×50` |

`[equipment]` 分支**一行未改** ⇒ 既有 16749 个盒子的装备条目按构造不变。
`odyssey_create_rewards.go:64` 依赖根包的 `Sections == ["[stackable]"]`：成对形态下仍是这个值
（同文件 68 行本来就要求该块数字成对）⇒ 奥德赛创建包不受影响。

**测试**：

* `internal/catalog/selection_cells_test.go`
  * `TestParseSelectionStackablePairs`：用**真实脚本 token** 断言类别 `[0 0]` +
    三项 `90/45/50` + `Sections == ["[stackable]"]` + 判定为「已建模」。
  * `TestParseSelectionStackableRecordShapeStaysUnmodelled`：5 数字一条的块必须**不建模**，
    且仍算「未建模」。
* `cmd/wireprobe/selection_box_test.go`
  * `TestSelectionBoxGrantsTheSourceStackableCount`：端到端开箱，断言落在**下发帧**
    `booster_open_ack`（条数 13..15、模板 15..19、数量 19..23）= **45**，再核存档次之
    ——「存档对了但帧里还是 1」这一类只有看帧才拦得住。

**自检（2026-10-11 17:5x）**：

```
gofmt（转 LF 副本校验 4 个文件）              → 空
go build ./internal/... ./cmd/...            → 0
go vet ./internal/catalog/ ./cmd/wireprobe/  → 0
go test -count=1 ./internal/catalog/ ./cmd/wireprobe/ ./internal/gamedata/
   → ok 118.3s / ok 177.6s / ok 8.9s
二进制 0e868eaf…cf5d（31,437,824 B），bin/ 与根 runtime/dev-local/ sha 相同
selectionPairs 2（新函数名进 DWARF）· readinessRadar 0（负对照）
```

**给业主的验收**：重启后开一个新礼包 → 选 `Radiant Harmonious Crystal` → 背包里应是 **45** 个。
服务端日志会从
`category [0 0] has no exported item set … generic path`
变成
`selection box 590722589: 1 pick(s) matched the exported source range in category [0 0]: [{10327285 45}]`。

## 4. 尚未落地（待用户安排）

P3 子项 1（创建补给药水 10418028×30）、3（章节最终领主的章节盒，默认关闭）、4（七章奖励补发）、
8（满级毕业转普通角色）、9（按职业整理等级主线）、10（毕业转换时机）。手册只给"文件清单 + 关键签名 +
常量 + 事件键"，属按真源重建，建议逐子项评审后推进。
