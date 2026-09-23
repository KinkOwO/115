# next60 — `[import script]` 装备继承、admin 装载补齐、堆叠上限按源语义

## 0. 缘起

从"161 个自选盒的物品发不出去"这条诊断出发（`cmd/wireprobe/selection_box_audit_test.go`），
一路查到三个层面的缺陷，其中第一个是**全局根因**。

## 1. `[import script]` 继承（根因）

### 1.1 真源

`equipment/character/common/jacket/cloth/100050791.equ` 的完整内容：

```
[name]（空）
[attach type] `[trade]`
[usable period] 14
[value] 0
[move wav] `CLOTH_TOUCH`
[import script] `character/common/jacket/cloth/100050666.equ`   ← 定义在那边
[impossible contents] `disjoint` `upgrade` `amplify upgrade` `separate upgrade`
```

它是一个"薄壳"：`[rarity]`/`[equipment type]`/`[durability]` 全都不在自己身上，靠
`[import script]` 继承。源里这种形状的装备很多。

### 1.2 缺陷

`EquipmentCatalog.Reward` 只做字段校验，**没有任何 `[import script]` 处理**（全仓只有
`internal/catalog/world.go` 对地图的 `[import script]` 做了处理）。于是：

- `[rarity]`/`[equipment type]` 为空 → `special equipment reward requires additional source state`
- `[durability]` 为空且部位不在 `durabilityOptional` → `missing source equipment durability`

凡是走 `Bag.AddEquipment` 的路径全部中招：GM 发放、任务奖励、自选盒里的装备。

### 1.3 诊断口径的修正（避免误报）

第一版审计把自选盒的**每一件**都拿装备目录去查，报出 1438 件。这个数字是错的：

- 13930 个不重复模板里，只有 **12163 件**的归档路径在 `equipment/` 下；
- 其余是装扮（`avatar/`、`at_avatar/`，155 件）、宠物（717）、堆叠物（831）、
  以及 64 件不在索引里 —— 它们各有自己的发放目的地（Destination 1/2/4），
  本来就不该用装备目录校验。

按发放路径过滤后：**78 个盒子 / 543 件装备**。修掉 `[import script]` 后为 **0**。

### 1.4 修复

`internal/inventory/equipment.go`：

```go
func (c *EquipmentCatalog) Reward(id uint32) (uint16, error) {
	r, err := c.definitionResolved(id, 0)   // 沿 [import script] 补缺字段
	...
}

func (c *EquipmentCatalog) definitionResolved(id uint32, depth int) (EquipmentDefinition, error)
func importTarget(cells []pvf.Token) uint32   // 从路径文件名取模板号
```

规则：自身显式给出的字段优先；链深上限 8；基础件取不到（不在目录/链太深）时**退化自身**，
保持原来的报错行为，不悄悄放行。

**实机确认（2026-09-23）**：GM 发 `100050791` → `{"Slots":[19]}`，库里 `slot 19 dur=60`
（耐久继承自 `100050666`）。审计断言：装备路径 126008 件全部可发放。

## 2. `cmd/admin` 装载补齐

执行 GM 发放时才发现 admin 还有一处与网关不一致：它的装备目录**没挂 `equipment-full`**
（基础目录 `equipment.current35.json` 只有 1536 行），发 `100050791` 直接
`equipment definition missing` —— 也就是说 admin 过去只能发那几千件。

修复（`cmd/admin/main.go`）：

- 新增 `-equipment-full-catalog`（默认 `configs/equipment-full`），装载后打印
  `attached full wear catalog: N records`（实测 424216 条）
- 基础目录默认从 `equipment.current35.json` 换成 `equipment.current37.json`（与网关一致）
- `TestBuildAwarderAttachesFullWearCatalog` 锁住这条

（这与 2026-09-22 那批修的 `-item-index` 补挂是同类问题：admin 的目录装载一直比网关少。）

## 3. 堆叠上限按源语义

### 3.1 现象与真源

GM 发 1000 银币后，库里出现两叠：`slot 65 x23` + `slot 66 x1000`。

`10418036.stk` 的段名里**没有** `[stackable limit]`，只有
`[stackable type] `[unlimited waste]`` —— 类型名就写着 unlimited。
`items.index.json` 里 `[unlimited waste]` 共 314 条，其中 219 条**写了**显式上限
（1/999/1000），95 条没写 —— 银币/金币属于后者。

服务端过去的逻辑是"没写上限 → 用 bag 规则的 `missing_stack_limit`（=1000）"，
所以 23 + 1000 被拆成两叠。

### 3.2 修复

`internal/inventory/shop.go` 新增 `stackLimitFor`，`Bag.Add` 与商店购买 `addStackable` 共用：

| 情形 | 上限 |
| --- | --- |
| 源写了 `[stackable limit]` | 按源值（219 条 `[unlimited waste]`、`[booster]` 的 7157 条等都不受影响） |
| 类型名含 `unlimited` | `MaxInt32` = 2,147,483,647 |
| 其余 | bag 规则的 `missing_stack_limit` |

**取 `MaxInt32` 而不是 `MaxUint32` 的理由**：wire 里 amount 是无符号 u32，但客户端内部
按有符号 int 消费，超过 `2^31-1` 有显示成负数的风险 —— 那比"分叠"更难排查。

**实机确认**：再发 1000 → 并入原叠（23 → 1023），不再新开一叠。

## 4. `at_avatar` 判据

写审计用例时发现：`cmd/wireprobe/booster_flow.go` 判装扮用的是
`strings.Contains(itemPath, "/avatar/")`，而装扮目录还有 `at_avatar/` 这种命名，
那些装扮会被当成装备走 Destination 3。改为宽松匹配 `avatar`。
（`kind` 来自 `items.index.json` 时不受影响，只有靠 path 兜底判定的场合才暴露。）

## 5. 测试与实机确认

- 新增：`internal/inventory/equipment_import_test.go`（薄壳继承后的耐久 == 基础件耐久）、
  `internal/inventory/stack_limit_test.go`（unlimited 不受 `missing_stack_limit` 约束、
  显式上限仍然生效）、`cmd/admin/main_test.go` 的 `TestBuildAwarderAttachesFullWearCatalog`
- 更新：`selection_box_audit_test.go` 从"报告"改成"断言 0 容忍"；
  `odyssey_coin_grant_test.go` 的"第二次另起一叠"断言改为"并进同一叠"（行为变更点）
- `go vet` 干净、`go test -count=1 ./internal/... ./cmd/...` **20 包全绿**
- **实机确认（用户操作）**：GM 发 `100050791`（薄壳装备）→ 收到、耐久 60；
  GM 发 `10418036 x1000` → 收到、且并入原叠

## 6. 遗留

- 玩家侧表述已确认：自选盒开箱、GM 发放、任务奖励三条路径现在都能发源里的装备。
- 手册 P3 其余子项（创建补给、章节盒、七章奖励、毕业转换、等级主线）仍未做。
- 堆叠上限的"客户端真实表现"未实测（用户判断有兜底即可）：如需验证，可发一次
  `2147483647` 观察显示，再用 SQL 改回。
