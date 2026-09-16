# NPC 商店买卖实现报告（CMD21 / CMD22）

> 目的：为迁移 / 交接提供完整、自包含的实现基线。以下内容全部来自已闭环的 IDA 分析、实机 live capture 与通过 `go test ./...` 的 Go 服务端代码，不依赖当前可能损坏的本地文件。

---

## 1. 目标

修复 NPC 商店买卖物品导致客户端卡死的问题：客户端发出 CMD21（购买）/ CMD22（出售）后，服务端此前不回应，导致客户端挂起。实现服务端协议解码、背包操作、事务持久化与流程接线。

---

## 2. 操作码

| Opcode | 名称 | 语义 |
|--------|------|------|
| CMD21 | `ENUM_CMDPACKET_BUY_ITEM` | NPC 商店购买物品 |
| CMD22 | `ENUM_CMDPACKET_SELL_ITEM` | NPC 商店出售物品 |

---

## 3. 请求报文格式（live capture + IDA 双重证实）

### 3.1 购买请求（CMD21，明文 24 字节 = 6×u32）

```
u32 template    // 物品模板 ID
u32 npcId       // NPC 标识（实机固定 1）
u32 actorId     // 角色标识（实机固定 2）
u32 count       // 购买数量
u32 category    // 分类/商店上下文（不同物品值不同）
u32 reserved    // 保留，实机为 0
```

**重要事实**：

- 这是 **live capture 实机确认的 24 字节格式**，并非 IDA 函数 `0x1418BC500` 的 21 字节布局。
- `0x1418BC500` 写死 `u32(0)+u8(0x11)`，与实机报文 `u32(1)+u8(7)` 不符。**该函数不是 NPC 商店购买发送者，而是修理商店发送者**（仅 vtable 调用，无代码 xref）。
- 真正的购买发送者仅通过 vtable 调用，函数未定位（见 §9 遗留）。

### 3.2 出售请求（CMD22，明文 24 字节 = 20B 内容 + 4B 对齐填充）

```
u32 npcId      // NPC 标识（实机 1）
u32 actorId    // 角色标识（实机 2）
u8  flag       // 实机固定 1
u8  list       // 列表类型，实机 0（主背包）
u16 slot       // 背包槽位号
u32 template   // 实机固定 = 1（类别标志，非真实模板 ID）
u32 price      // 实际物品身份（真实模板由槽位范围决定）
```

**关键事实**：

- `template` 字段恒等于 1，是一个**类别标志**，不能用于物品身份校验。
- 出售鉴权只依据「来源 + 槽位」（Source+Slot），不校验 template。
- 20 字节内容按对齐补齐到 24 字节（`DecryptPayload` 返回完整对齐块，不剪 padding）。

### 3.3 线层明文长度

`DecryptPayload` 返回完整对齐块（不裁剪 padding），明文长度 = `ceil(contentLen / alignment) * alignment`。

- CMD21（index=7，DFOBlowfish）对齐 = 8
- CMD22（index=8，XTEALE）对齐 = 8
因此 20/21 字节内容 → 24 字节明文，必须用 `len(p)==24` 校验，不是 32。

---

## 4. 响应报文格式（IDA 反编译证实）

### 4.1 购买成功 ACK

```
u8  1          // 成功标志
u32 npcId
u32 actorId
u32 template
u32 count
u32 category
181B item record   // 物品记录（slot, template, amount, ...）
u8  bonusCount     // 奖励数量，无奖励时 = 0
    bonusCount × (u32, u32, u16)
u32 newGold        // 扣款后的金币余额
u16 slot           // 入包槽位
```

无奖励时总共 209 字节。ACK 发送后紧跟 NOTI14（InventoryUpdate）权威背包全量刷新。

### 4.2 出售成功 ACK

```
u8  1          // 成功标志
u32 goldGained // 获得金币
u32 count      // 出售数量（单物品 = 1）
    count × (u8 list, u16 slot, u32 template)
```

单物品时 16 字节。ACK 发送后紧跟 NOTI14 背包刷新。

### 4.3 拒绝

`protocol.Refusal(4)` —— 通用拒绝码。客户端 dstr 显示「背包已满」，但这是通用码，不代表真实背包满。

---

## 5. 背包槽位规则（BagRules）

```
quick_slots        = [0, 8]
equipment_slots    = [9, 64]     // 未穿装备（放在背包里的装备）
[throw]            = [65, 120]   // 消耗品/投掷物（药水、礼盒等）
[material]         = [121, 176]  // 材料
[quest]            = [177, 232]  // 任务物品
[material expert job] = [233, 288]
[avatar emblem]    = [289, 344]
[material]4        = [345, 359]
MissingStackLimit  = 1000
```

**stackable_type → 槽位范围映射**（与参考服 `dungeonDropStackableSlotRange` 一致）：

| stackable_type | SlotStart | SlotEnd |
| --- | --- | --- |
| `[material]` | 121 | 176 |
| `[material]...4` | 345 | 359 |
| `[quest]` | 177 | 232 |
| `[material expert job]` | 233 | 288 |
| `[avatar emblem]` | 289 | 344 |
| **default（消耗品/投掷物）** | **65** | **120** |

---

## 6. 实现代码清单

| 文件 | 内容 |
| ------ | ------ |
| `internal/game/protocol/shop.go` | `DecodeBuyItem`（6×u32=24B）、`DecodeSellItem`（20B+4B pad=24B）、`BuyItemSuccess`、`SellItemSuccess`、`Refusal` |
| `internal/game/protocol/shop_test.go` | 布局 pin 到 live capture 值，拒绝零 template/count、错误长度、多物品路径、非零 padding |
| `internal/inventory/shop.go` | `Bag.Buy`（catalog 无关，按 stackable_type 分槽）、`Bag.Sell`（槽位范围分派）、`stackableSlotRange` |
| `internal/inventory/shop_test.go` | 购买金币扣款/入包、金币不足、出售堆叠/装备、穿戴拒绝 |
| `internal/loot/shop.go` | `Service.Buy`/`Service.Sell`，`CommitCharacterEvent` 幂等收据；`shopEventSeq` 原子计数器 |
| `cmd/wireprobe/shop_flow.go` | `buyItem`/`sellItem` 处理器：解码→服务→ACK+NOTI14 |
| `cmd/wireprobe/main.go` | CMD21/22 分支（在 CMD44 与 CMD143 之间），失败发送 `Refusal(4)` |
| `cmd/wireprobe/request_scope.go` | 21/22 加入 `observedGameRequest` 白名单 |
| `docs/protocol/next39-npc-shop.md` | IDA + live capture 证据记录 |

---

## 7. 关键决策与历史修复

### 7.1 真源优先：live capture 覆盖 IDA

`0x1418BC500` 的 21B 布局（4×u32+u8+u32，硬编码 0/0x11）与实机 24B 报文（6×u32）不符。按 AGENTS.md「真源优先」，live capture 为权威。

### 7.2 catalog 无关的买卖

NPC 商店物品模板 ID 1-3175 **不在** loot catalog（catalog 从 6001 开始）。购买直接放入正确槽位范围，不依赖 catalog 的 stackable-type 查找。

### 7.3 出售 template 字段恒等于 1

live capture 证实所有 sell 报文 template=1（类别标志）。服务端**不得**用它做物品身份校验。槽位范围决定物品在哪个数组。

### 7.4 槽位范围分派（出售）

- equipment_slots [9,64] → `b.Equipment`（未穿装备）
- throw [65,120] / material [121,176] → `b.Items`（堆叠物）
- `b.Worn`（穿身上的）拒绝

**槽位重叠问题**：slot 12-25 同时属于 equipment [9,64] 和 worn [12,25] 范围。旧代码先查 Worn 数组导致「穿件物品」误拒。修复：equipment 范围先查 `b.Equipment`，仅当 equipment 无该槽位时才检查 Worn。

### 7.5 幂等 key 唯一性（两次修复）

- **问题 1**：`buy:<template>:<count>` key 让同一物品多次购买被当 replay 拒绝（applied=false），不扣金币不加物品。
- **问题 2**：改成 `time.Now().UnixNano()` 后，客户端快速连发多个 CMD21（每个 count=1）时纳秒级时间戳可能碰撞，导致批量购买只入 1 个。
- **最终修复**：用进程级 `atomic.AddUint64(&shopEventSeq, 1)` 单调计数器，保证每次 buy/sell key 绝对唯一。

### 7.6 批量购买显示错误（ACK amount 字段）

客户端「一次买多个」实际是快速连发多个 CMD21（每个 count=1），或单个 CMD21 count=2（药水/礼盒）。服务端 DB 正确累积（如买 4 次×count=2 → amount=8），但 ACK 的 item record 用 `receipt.Count`（单次购买量）作 amount，客户端用它覆盖背包显示（显示 2 而非 8）。
**修复**：ACK item record 改用该槽位的**实际堆叠总量**（遍历 `b.Items` 找到对应 slot 的 `Amount`）。

### 7.7 消耗品误入材料栏

旧代码硬编码分配 material 槽 [121,176]，导致药水/礼盒等消耗品进材料标签页。
**修复**：添加 `stackableSlotRange`，catalog 无条目（NPC 商店物品）默认进 throw 槽 [65,120]（消耗品栏）。

### 7.8 CMD21/22 采样上限（checksum failed）

CMD21/22 不在 `observedGameRequest` 白名单，走 `BodySampleLimit=8` 采样路径。第 9 次起 plaintext 不解密 → `verified=false` → `checksum failed`。
**修复**：21/22 加入白名单，每次解密。

---

## 8. 已验证状态

- **购买**：购买成功，物品入包（正确槽位），金币扣款，ACK+NOTI14 发送。
- **出售**：堆叠物和部分装备成功出售；ACK+NOTI14 发送。
- **批量购买**：DB 正确累积（amount=8 for 4×count=2），ACK amount 修复后客户端显示正确。
- **消耗品分类**：药水/礼盒入 throw 槽 [65,120] 消耗品栏。
- `go test ./...` 全绿。
- 最终二进制：`bin/wireprobe-handoff-source.exe`（约 16.9 MB）。

---

## 9. 遗留问题（迁移后继续）

### 9.1 商店定价为占位符

`shopUnitPrice = 1` 金币/单位。PVF `initItemShopScript` / `etc/itemshop/*` 价格表未提取。等真实价格表后替换为 per-template 查找。

### 9.2 真正的购买发送者未定位

`0x1418BC500` 是修理商店发送者，非 NPC 商店。NPC 商店购买发送者仅通过 vtable 调用。live 报文格式已确认，但 IDA 发送函数未定位。可搜索 vtable `0x14974d4e8` 的兄弟函数，或扫描 `BA 15 00 00 00`（mov edx, 21）匹配写 6×u32 的函数。

### 9.3 NPC 商店物品 stackable_type 未从 PVF 导入

当前用「catalog 无条目 → 默认 throw」的启发式。正确做法是从 PVF `stackable/stackable.lst` 读取所有 stackable 物品的 `[stackable type]` 字段，生成 template→stackable_type 映射表，替换启发式。PVF Go 库当前不支持本客户端 760MB 的 PVF 格式（超 512MB 限制 + `unsupported archive format`），需检查 `sk.dat`（当前 client 目录无此文件）或换用参考服 Python 解密链。

### 9.4 幂等 key 会随重启重置

`shopEventSeq` 是进程级计数器，重启后归零。若需跨重启幂等，需持久化计数器或用 UUID（但需考虑性能）。

---

## 10. 迁移核对清单

- [ ] `internal/game/protocol/shop.go` + `shop_test.go`
- [ ] `internal/inventory/shop.go` + `shop_test.go`（含 `stackableSlotRange`）
- [ ] `internal/loot/shop.go`（含 `shopEventSeq` 原子计数器）
- [ ] `cmd/wireprobe/shop_flow.go`（含 stackTotal 修复）
- [ ] `cmd/wireprobe/main.go` CMD21/22 分支
- [ ] `cmd/wireprobe/request_scope.go` 21/22 白名单
- [ ] `docs/protocol/next39-npc-shop.md`

确认点：

- 购买请求 24B = 6×u32
- 出售请求 24B = 20B + 4B pad，template 恒=1
- 出售鉴权只查 Source+Slot
- 消耗品默认 throw 槽 [65,120]，材料 [121,176]
- ACK item record amount = 槽位实际堆叠总量
- `len(p)==24` 而非 32
