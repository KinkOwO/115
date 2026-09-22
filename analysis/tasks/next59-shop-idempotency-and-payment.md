# next59 — 奥德赛商店的两个真 bug：幂等键跨重启撞车 与 未读源价格表

## 0. 现象（用户实机报告，2026-09-23 凌晨）

1. 从**奥德赛商店**（用银币兑换）买到「开拓者的奥德赛初级装备自选礼盒」，右键开时客户端提示「仓库已满 / 库存已满」；
2. **重选角色后那个盒子自己消失了**；
3. 修掉 1、2 之后再买同一个盒子，**银币没扣、金币只掉 1**（盒子等于白送）。

## 1. 证据链（全部来自服务端自身记录，没有靠猜）

### 1.1 时间线（角色 test-jh = id 10）

| 时间（+08） | 来源 | 内容 |
| --- | --- | --- |
| 23:57:00 | `character_events` | `buy:10417798:1:1` → `{"template":10417798,"slot":66,"cost":1,"new_gold":3586,"npc_id":100001019}` |
| 00:03:13 | 同上 | `booster-open:10:<sha256(raw)>` → `Granted=[{100051397,1}]`（第一次开箱，成功） |
| 00:03:17 | 同上 | `ordinary-equipment-move-v1` → 把 slot16 的 `100051397` 与仓库里的 `100051399` 交换（**这是玩家自己的操作**，解释了库里 slot16 后来为什么是 …399） |
| 00:09:09 | 进程 | **服务端重启**（`wireprobe-handoff-source` PID 26868 的启动时间） |
| 00:11:43 | 会话日志 | `shop_buy_ack` + `shop_buy_inventory_updated`（**回了成功**）；`character_events` 里**没有**对应记录 |
| 00:12:12 | 会话日志 | 右键 slot 66 → `booster_action_refused: booster box not found at slot 66` → `plain_hex=000400`（Refusal 4）→ 客户端显示「库存已满」 |

### 1.2 关键推断

`loot.Service.Buy` 的幂等键是 `buy:<模板>:<数量>:<进程内自增seq>`。**重启把 seq 归零**，于是 00:11 的购买生成了与 23:57 完全相同的键 `buy:10417798:1:1` → `CommitCharacterEvent` 判定"已处理过" → **不发货、不改存档**；但 `buyItem` 仍然回了 `shop_buy_ack`，并且 `receipt` 是**旧事件**里读出来的（含 `slot:66`）→ 客户端据此画出一个服务端并不存在的盒子。

服务端 DB 侧同步核对：`test-jh` 的 `inventory.items` 当时只有 `slot 3/4/65`（药水、药水、银币），**没有任何盒子**——即"幽灵"完全在客户端，但成因是服务端谎报成功。

> 手册第 7.1 节把这类现象记作纯客户端「幽灵槽」。实机证明：**服务端也有一半责任**。

## 2. bug 2：商店从未读源价格表

### 2.1 真源

请求字段（新增的 `shop_buy_request` 日志给出）：`template=10417798 count=1 npc=100001019 actor=100003035 category=263`。

- `npc=100001019` 就是**银币物品**脚本里的 `[action type] [open shop] 100001019`（右键银币开的就是这家店）；
- 商店表在 `itemshop/100001019_aradodyssey.shp`（PVF 里 `100001019` 只命中 2 个文件，另一个是无关的地图）：

```
[NPC] 100003035              ← 表内 actor id（= 请求里的 actor）
[type] `[etc shop]`
[sell info] [tab] [sell item list]
[item] 1 [index] 10417798 [purchase amount] 1
  [need material] 10418036 100        ← 100 个银币
[/item]
...
```

`[need material] <货币模板> <数量>` 就是支付方式。其余价格（同表）：
`10417797` 银币60 / `10417799` 金币60 / `10417800` 金币100 / `10419745` 银币8或金币3 /
`10418028` 银币1或金币1(×3) / `10418029` 银币2或金币2(×2)；第二个 tab 的 4 件**没有** `[need material]`，仍走金币。

### 2.2 服务端缺什么

`loot.Service.Buy` 只有 `cost := r.Count * shopUnitPrice`（常量 `1`）→ 扣 `Bag.Gold`。**完全没有价格表**，所以银币不扣。

## 3. 修复

### 3.1 幂等键（`internal/loot/shop.go`）

```go
var shopBootStamp = time.Now().UnixNano()          // 本次进程启动标记

func shopEventKey(op string, seq uint64, fields ...uint32) string   // 内容 + 启动标记 + 计数
func shopEventKeyAt(boot int64, op string, seq uint64, fields ...uint32) string  // 便于测试
```

- `buy` / `sell` 都改用它；
- **没有**用纳秒时间戳当唯一成分：用例第一版当场证伪——Windows 的 `time.Now()` 只有毫秒精度，同毫秒两次调用得到相同的值。

### 3.2 谎报成功（`cmd/wireprobe/shop_flow.go`）

`applied == false`（幂等命中）时**直接拒绝**（`duplicate shop purchase request`），绝不再拿旧 receipt 回"成功 + 那个 slot"。`sell` 同样处理。

### 3.3 材料支付

| 文件 | 内容 |
| --- | --- |
| `cmd/itemshopimport` | 只读导出 `itemshop/**.shp` → `configs/itemshop-candidate.json`（**527 商店 / 7547 商品 / 829 KB**，155 个材料支付）；`-debug <id>` 打印单店解析 |
| `internal/catalog/item_shop.go` | `LoadItemShops` / `Materials(shopID, template)` / `Listed` / `PurchaseAmount`；同一模板多条报价时优先保留**可支付**的那条 |
| `internal/inventory/shop.go` | 抽出 `addStackable`；新增 `PayMaterials(materials, multiplier)`（整笔校验、按叠扣、扣空删行、不动金币）与 `BuyWithMaterials` |
| `internal/loot/shop.go` | 源里有 `[need material]` → 材料支付；否则保持金币 |
| `cmd/wireprobe/main.go` | `-item-shop` / `DFO_ITEM_SHOP`，默认按 release → candidate → 与 `-item-index` 同目录推导 |
| `server/work/dfo_probe_tools/channel_probe.py` | 显式传 `-item-shop`（网关 cwd 是项目根，相对路径不可靠） |

### 3.4 取证增强

`shop_buy_request` / `shop_buy_refused` / `shop_sell_request` / `shop_sell_refused` 及所有出站包都补记 `plain_hex`（手册 7.1 节点名的尾巴）。正是它这次给出了购买请求的完整字段。

## 4. 测试与实机确认

- `internal/loot/shop_key_test.go`：键跨重启不复用、同进程内计数不同即不同、buy/sell 不共用命名空间、生产路径必须带启动标记。
- `internal/catalog/item_shop_test.go`：真源价格断言（`100001019` + `10417798` → 银币 `10418036×100`；`10417799` → 金币 `10418035×60`）+ 畸形产物拒绝。
- `internal/inventory/itemshop_pay_test.go`：整笔校验 / 跨叠扣减 / 扣空删行 / 不改原背包 / 多件材料任一不足即拒 / 买 2 件账单价翻倍 / `BuyWithMaterials` 金币不变。
- `go test -count=1 ./internal/... ./cmd/...` **20 包全绿**，`go vet` 干净。
- **实机确认（2026-09-23，用户操作）**：
  1. 不再出现「库存已满」，重选角色后不再有消失的盒子；
  2. 新开出的装备耐久正常（**P1 的修复同时被确认**：耐久 = 源值 60）；
  3. 买 `10417798` → **银币 123 → 23、金币不变**。

## 5. 顺带纠正的两个结论

1. **「库存已满」的归因**：手册说是"自选盒落进随机池 → 查无定义"，实机证明该路径**从未被走到**（客户端两次请求都带了 selection，且就在源范围内）。真因是 §1 的幂等键 bug。
2. **「范围外的选择」是误读**：我一度把 ack 里的选择值按错位偏移解成 `100344517`（support），据此写了"客户端 PVF 与服务端不同版本"的注释。按正确偏移重解为 `0x05F6A9C5 = 100051397`——正是源列表第一件（上衣），服务端也如实发放了它。相关注释与用例已更正；`Resolve` 保持"只报告不拒绝"的宽松策略（两份 PVF 确实不是同一构建：`client/Script.pvf` 761,337,702 B 加密，`server/work/client-build/Script.inner.pvf` 760,530,763 B）。

## 6. 遗留

- **161 个自选盒的物品在 `AddEquipment` 路径下发不出去**（`cmd/wireprobe/selection_box_audit_test.go` 全量扫描：1438 件中 1038 件缺 `[durability]` 且部位不在耐久可选表、731 件 rarity/类型不满足、若干件目录无定义）。属独立任务。
- 手册 P3 其余子项（创建补给药水、章节盒、七章奖励补发、毕业转换、等级主线整理）未做。
- `loot.Service.Sell` 仍按固定逻辑给金币（源商店表的 `[sell info]` 段未建模）。
