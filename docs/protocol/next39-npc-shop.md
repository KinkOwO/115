# DFO 本地服 — 对接工作记录：NPC 商店买卖（CMD21 / CMD22）协议取证

> 面向后续维护与协议对接。记录 115 级客户端实机 live capture 与 IDA 逆向推导，
> 作为 NPC 商店买卖功能（CMD21 / CMD22）的权威协议标准。

---

## 1. 协议操作码与语义

| Opcode | 宏名称 | 客户端语义 |
| --- | --- | --- |
| CMD21 (0x0015) | `ENUM_CMDPACKET_BUY_ITEM` | 玩家在 NPC 商店购买物品 |
| CMD22 (0x0016) | `ENUM_CMDPACKET_SELL_ITEM` | 玩家向 NPC 商店出售物品 |

---

## 2. 请求报文格式（Live Capture + IDA 标定）

### 2.1 购买请求（CMD21，明文 24 字节 = 6×u32）

`DecryptPayload` 返回 8 字节对齐块（DFOBlowfish cipher index=7）：

```text
+00: u32 template  // 物品模板 ID (esi)
+04: u32 count     // 购买数量 (ebp，来自 UI 编辑框)
+08: u32 npcId     // NPC 标识
+12: u32 actorId   // 角色 / 窗口上下文标识（实机常为 2）
+16: u32 category  // 分类 / 商店上下文标志（不同物品有差异）
+20: u32 reserved  // 保留字段，实机观察值 0
```

**真源关键事实与 Bug 根因剖析**：

- 实机抓包确认为 24 字节（6×u32），严格通过 `len(p) == 24` 校验。
- 真正购买发送者已在 `client/DFO.exe` 完整标定为 `0x1467e7b30`（由 `0x146a5b697` 等 UI 确认流程调用）：
  `esi` 传入物品模板，`ebp` 传入 UI 输入框折算的购买数量 `count`。发送时首个字段写入 `esi`（+00），第二个字段写入 `ebp`（+04）。
- **历史 Bug 根因**：历史迁移文档误将 offset 4 标定为 `npcId`，而将 offset 12 标定为 `count`。由于 offset 12 在客户端实机运行期恒为窗口/actor 上下文 `2`，服务端解码层 `DecodeBuyItem` 从 offset 12 取值，导致无论玩家在客户端输入何种数量，服务端始终按 `2` 个进行扣款和入包。修正后读取 offset 4，彻底解决该问题。

### 2.2 出售请求（CMD22，明文 24 字节 = 20B 内容 + 4B 对齐填充）

`DecryptPayload` 返回 8 字节对齐块（XTEALE cipher index=8）：

```text
+00: u32 npcId     // NPC 标识（实机 1）
+04: u32 actorId   // 角色标识（实机 2）
+08: u8  flag      // 实机固定 1
+09: u8  list      // 列表类型，实机 0（主背包）
+10: u16 slot      // 背包槽位号
+12: u32 template  // 类别标志（实机固定 1，非真实物品模板 ID）
+16: u32 price     // 实际物品身份上下文
+20..23: 4B 零填充 // 对齐补齐（必须为 0）
```

**关键事实**：

- `template` 字段恒等于 1，仅作类别标志，服务端不可用于校验物品身份。
- 出售鉴权完全依据「来源 + 槽位」（Source + Slot）。

---

## 3. 响应报文格式

### 3.1 购买成功 ACK（CMD21，无奖励物品时共 209 字节）

```text
+00: u8  1              // 成功标志
+01: u32 npcId          // 回显 NPC 标识
+05: u32 actorId        // 回显角色标识
+09: u32 template       // 回显物品模板 ID
+13: u32 count          // 回显购买数量
+17: u32 category       // 回显分类
+21: [181]byte record   // 物品记录（slot, template, amount...）
+202: u8 bonusCount     // 奖励数量，无奖励为 0
+203: u32 newGold       // 扣款后的角色金币余额
+207: u16 slot          // 入包槽位
```

ACK 发送后，紧跟发送 NOTI14（`InventoryUpdate`，全量背包刷新），保证客户端与服务端数据权威一致。

### 3.2 出售成功 ACK（CMD22，单物品共 16 字节）

```text
+00: u8  1              // 成功标志
+01: u32 goldGained     // 本次出售获得金币
+05: u32 count          // 出售物品条数（单物品为 1）
+09: u8  list           // 背包列表类型（0=主背包）
+10: u16 slot           // 出售物品所在槽位
+12: u32 template       // 出售物品模板 ID
```

ACK 发送后同样紧随 NOTI14 刷新。

### 3.3 失败响应

服务端统一回应 `protocol.Refusal(4)`（通用拒绝码，客户端显示「背包已满」）。

---

## 4. 领域逻辑与关键实现细节

1. **槽位映射与 catalog 无关性**：
   NPC 商店模板 ID（1-3175）不在普通掉落的 loot catalog 中。`Bag.Buy` 采用启发式分槽规则：未配置类别时默认分配至 `[throw]` 范围（槽位 65-120，消耗品栏）；若显式提供 `[material]` 等标签则分派至相应材料范围。
2. **槽位重叠与出售分派**：
   未穿戴装备范围 `[9, 64]` 与已穿戴范围 `[12, 25]` 在槽位号上有重叠。出售时必须优先检索背包装备数组 `b.Equipment`；仅当装备栏无该槽位且该槽位处于已穿戴数组 `b.Worn` 时，才作为穿戴装备拒绝出售。
3. **高并发购买幂等 key**：
   连续快速购买时，时间戳方案可能发生碰撞。服务端引入进程级原子单调自增计数器 `shopEventSeq`，构造 `buy:<template>:<count>:<seq>` 保证每次事务持久化 key 绝对唯一。
4. **批量购买显示修正**：
   客户端连续快速发多个 CMD21 或单包 count>1 时，ACK 报文中的 181B 物品记录必须填入该槽位的实际堆叠总量（`stackTotal`），而非单次增量，防止客户端覆盖显示造成显示数量偏小。
5. **解密白名单**：
   CMD21 与 CMD22 加入 `observedGameRequest` 白名单，避开采样丢弃与 checksum 校验失败。
