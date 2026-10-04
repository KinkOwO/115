# DFO 本地服 — 对接工作记录：个人金库（Personal Vault）物品存取实现（next42）

> 面向后续维护与多端同步。配套交接文档见 `docs/金库功能实现迁移报告.md`。

## 一、背景与问题

### 1. 现象

玩家在城镇打开个人金库（Safe I / Old Iron Safe）时：

1. 从背包将装备或物品拖入金库，客户端弹出错误提示「The target inventory is full ...」（目标容器已满），无法存入。
2. 从金库取出物品至背包后，金库原槽位残留显示「0 金币」（Gold），且图标无法被正常清理。

### 2. 根因分析

1. **放入报错「目标容器已满」**：
   - 客户端物品移动使用 **CMD 19**（`0x001A`，`ENUM_CMDPACKET_MOVE_ITEMSPACE`）。
   - 原服务端在处理 CMD 19 时，只允许容器类型 `0`（背包）与 `3`（穿戴栏）。当接收到金库容器类型 `2` 时判定为非法参数，返回拒绝码 `4`（`protocol.Refusal(4)`）。客户端收到拒绝码后弹出通用错误提示「The target inventory is full」。
2. **取出后残留 0 金币**：
   - 当物品移出槽位清空时，旧逻辑生成了模板为 0 的槽位更新（`template = 0`）。
   - 在客户端物品同步消费函数 `sub_1452E9810` 中，`template == 0` 被解释为金币（Gold），只有下发 `template == 0xFFFFFFFF`（`-1`）时，客户端才会调用 `sub_145AD4750` 执行原生槽位清空/删除。

---

## 二、协议逆向与报文布局（权威 IDB 逆向）

### 1. 容器类型定义（`u8 List`）

- `0`：背包（普通背包道具/材料/装备栏）
- `2`：个人金库（Safe I / Old Iron Safe）
- `3`：穿戴栏

### 2. CMD 19 C2S 请求包（客户端发送端 `sub_145AF6990`）

有效载荷共 29 字节（尾部填充 3 字节零，对齐为 32 字节）：

| 偏移 | 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| 0 | SourceList | u8 | 源容器类型（0: 背包, 2: 金库, 3: 穿戴） |
| 1 | SourceSlot | u16 | 源槽位编号 |
| 3 | SourceItem | u32 | 源物品 Template ID |
| 7 | Count | u32 | 移动数量 |
| 11 | DestinationList | u8 | 目标容器类型 |
| 12 | DestinationSlot | u16 | 目标槽位编号 |
| 14 | DestinationItem | u32 | 目标物品 Template ID（空槽为 0） |
| 18 | Extra | u32 | 固定 0 |
| 22 | Selection | u32 | 固定 0xFFFFFFFF |
| 26 | Flags | u8 × 3 | 固定 0 填充 |

### 3. CMD 19 S2C 响应包（客户端接收端 `sub_145283750`）

客户端首字节读取状态码 `a2`：

- `a2 == 1`：成功，继续读取成功字段。
- `a2 != 1`：失败，弹出错误提示。

成功报文字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| SourceList | u8 | 源容器类型 |
| SourceSlot | u16 | 源槽位 |
| Count | u32 | 实际移动数量 |
| DestinationList | u8 | 目标容器类型 |
| DestinationSlot | u16 | 目标槽位 |
| Flag | u8 | 固定 0 |

客户端解析完成后，若 `SourceList == 2 || DestList == 2`，命中 `LABEL_87` 调用 `sub_145ADD1E0` 执行本地内存转移并播放移动音效。

### 4. 金库数据同步（NOTI 13 与 NOTI 14）

- **登录/入城恢复（NOTI 13，`sub_1452D5A80`）**：
  `u8 list_type=2` + `u16 slots` + `u16 item_count` + `item_count` 组 181 字节物品结构。若无物品则 `item_count = 0`，跳过后续结构体。
- **槽位增量更新（NOTI 14，`sub_1452E9810`）**：
  `u8 space=2` + `u16 count` + `count` 组 181 字节槽位数据。
- **清空槽位核心规范**：
  在 `sub_1452E9810` 中，当物品 `Template == 0xFFFFFFFF`（`-1`）时，客户端触发 `sub_145AD4750(manager, slot)` 删除本地槽位物品。必须使用 `EmptyOrdinaryItem(slot)`，不得使用 `OrdinaryItem(slot, 0, 0)`。

---

## 三、架构设计与分层实现

### 1. 协议层（`internal/game/protocol/`）

- `vault.go`：新增 `PersonalVaultRestore(slots uint16, items [][CurrentItemRecordSize]byte)`，支持编码含物品的金库 NOTI 13。
- `inventory.go`：修改 `InventorySpaceUpdate`，允许 `space == 2`（金库空间刷新）。
- `empty_item.go`：新增 `EmptyOrdinaryItem(slot uint16)`，生成 181 字节槽位清空记录（template=0xFFFFFFFF）。
- `vault_test.go` & `empty_item_test.go`：单元测试覆盖空金库、带物品金库及清空槽位编码。

### 2. 存储层（`internal/database/vault.go`）

- 实现 `CommitVaultMove` 原子事务：
  - 加行锁 `SELECT ... FROM characters ... FOR UPDATE`。
  - 加行锁 `SELECT ... FROM character_vaults ... FOR UPDATE`。
  - 在原子事务闭包内完成背包与金库状态更新。
  - 更新 `character_vaults` 表。

### 3. 领域逻辑层（`internal/inventory/vault.go`）

- `Vault` 结构与 `VaultItem`（区分装备与堆叠物品）。
- `MoveVaultItem` 纯领域转移状态机：
  - Case 1: 背包 → 金库（支持装备、堆叠道具，自动合并同类未满堆叠，校验金库容量越界）。
  - Case 2: 金库 → 背包（支持装备、堆叠道具，校验装备栏/材料栏范围与占用冲突）。
  - Case 3: 金库内部移动/互换/合并。
- `vault_test.go`：测试用例覆盖装备转移、堆叠拆分、堆叠合并、容量越界、金库内互换等。

### 4. 网关编排层（`cmd/wireprobe/`）

- `vault_flow.go`：实现 `(w *worldSession) moveVault(...)`，编排事务提交、CMD 19 ACK、NOTI 14 背包刷新及 NOTI 14 金库受影响槽位增量刷新。
- `world_flow.go`：在 `worldSession` 中注入 `vault *inventory.VaultService`。
- `main.go`：初始化 `worldSession` 时注入 `vaultService`。
- `equipment_flow.go`：在 CMD 19 入口优先分发 `moveVault`，且装备清空槽位统一修复为 `EmptyOrdinaryItem`。

---

## 四、验证结果

1. 单元测试：
   - `protocol`: `TestPersonalVaultRestoreWithItems`, `TestEmptyOrdinaryItem` 全部 PASS。
   - `inventory`: `TestVaultMoveBagToVault`, `TestVaultMoveVaultToBag`, `TestVaultMoveWithinVault`, `TestVaultSerialization`, `TestVaultBootstrapPopulated` 全部 PASS。
   - `go test ./...` 全绿。
2. 静态检查：`go vet ./...` 零告警。
3. 编译产物：`wireprobe` 构建成功。
