# next43 — 账号材料仓库（Account Material Storage / Soul Storage）协议取证

> 目标：账号共享独立仓库，存各色 cube fragment、soul、(old) soul；这些物品入包时自动转入该仓库。
> 本文件记录 115 级客户端（client/DFO.exe.i64，会话 6e2b0ac1）的静态取证结果。

## 一、客户端证据（IDB 已验证）

### 1. 物品全集：17 个固定模板 ID

IDB 静态表 `0x14A9288E0`（连续 17 个 u32，另有 8 份副本散布各模块）：

| 组别 | 表地址 | 数量 | 模板 ID | 槽位 |
| --- | --- | --- | --- | --- |
| 各色晶块 cube fragment | `dword_14A9288E0` | 6 | 3033(black) 3034(white) 3035(red) 3036(blue) 3037(clear) 3262(gold) | 363..368 |
| 灵魂 soul | `unk_14A9288F8` | 6 | 10100115, 10100116, 10099773, 10099774, 10099775, 10158124 | 369..374 |
| 旧灵魂 (old) soul | `dword_14A928910` | 5 | 10361512, 10361513, 10361514, 10361515, 10361516 | 375..379 |

- 17 个 ID 在 `configs/items.index.json`（Script.inner.pvf 导出）全部存在，`stackable_type=[material]`。
- `stackable/10099001/10099773.stk` 等灵魂物品 `[attach type]=[account]`（账号绑定）；晶块为 `[free]`。

### 2. 槽位映射函数（反编译原文）

```c
sub_145ACA5E0(t) -> t in cube表  ? 363+i : -1      // 晶块 → 363..368
sub_145AD74A0(t) -> t in soul表  ? 369+i : -1      // 灵魂 → 369..374
sub_145AD6B70(t) -> t in oldsoul表? 375+i : -1     // 旧灵魂 → 375..379
```

### 3. 容器分类（list id = 35）

```c
sub_14500CE40(templateID):  // 模板 → 规范容器号
  if (sub_145ADB4E0(t) || sub_145ADC1C0(t) || sub_145ADBCA0(t)) return 35;  // 17 种材料 → list 35
  ...
sub_145ADB4E0 = is-cube-fragment（表1）
sub_145ADC1C0 = is-soul（表2）
sub_145ADBCA0 = is-old-soul（表3）
```

- 大量物品同步 reader（分解 ACK `sub_145273AC0`、`sub_145246160`、`sub_14524E5B0`、`sub_145269060`、`sub_145273560`、`sub_145288E10`、`sub_14528C100` 等）都先经 `sub_14500CE40(template)` 重定容器。
- 数量查询 `sub_145AD8790(template,...)`：若是晶块，直接从 `qword_14E683CD0`（账号材料仓库 manager）取数量。
- 消耗检查 `sub_145AFAA20(this, list=35, slot, count)`：从 CD0 按槽校验数量。

### 4. 管理器全局

- `qword_14E683CD0` = 账号材料仓库 manager（slot 数组在对象 +168，容量字段 +88；`sub_145AD8D10(mgr,slot)` 按槽取物品）。
- `sub_145A0E8C0(list)`：noti13/noti14 reader 的容器选择。注意 **list 35 在此函数里返回 `qword_14E683C80`**（与 list 0/28 相同的主背包 manager）。
- `sub_145AF7970(mgr, slot, item, ...)` = 通用插入；`sub_145AD4750(mgr, slot)` = 槽位删除（template=-1 触发）。

### 5. NOTI 0x0226 是死 opcode

- `ENUM_NOTIPACKET_ACCOUNT_MATERIAL_STORAGE_INFO = 0x0226 (550)` 名称字符串存在（0x14B0858B0）。
- 但 4 个 handler 注册函数（sub_1452BAD10 / sub_1452F9420 / sub_1453140E0 + wrapper sub_14599D5D0 的调用点）全量解析后，**0x226 没有注册 handler**（邻居 0x225/0x227/0x229 均有）。
- 结论：不得用 0x226 同步；应沿用普通物品行通道（NOTI13 全量 / NOTI14 增量）。

## 二、90 参考服先例（仅线索）

- `90dof`：账号晶块/灵魂仓库 = list-0 固定槽 354..365（6 晶块 + 6 灵魂），登录 op13 快照携带 `{slot, 固定模板ID, count}` 普通行（其余字节 0），消费走同一事务。`MergeAccountSharedInventory` 在每次 list-0 快照前把账号槽 overlay 进去。
- 115 客户端对应槽位是 **363..379（17 格）**，与 90 不同，以 IDB 为准。

## 三、补充取证（第二轮）

- **NOTI 0x27 GET_ITEM reader = `sub_1452D16F0`**：逐物品读取（8 字节保留 + u16 + u16 槽位 + u8），插入目标 = `sub_145A0E8C0(item->vtable[24]())`（物品类自身报告容器号）；存在则叠量（vtable+232），否则 `sub_145AF7970` 插入。
- `sub_145A0E750(list, slot)` = 通用按容器取物品；list 35 同样返回 C80 后按槽取。
- `sub_145AD5F80` = 按容器找空槽；space 35 时在 CD0 内部 map（对象+112）上检索。
- `sub_146688DA0` 等从 CD0 读取并调用 `sub_145AE4450`（记账/移除日志）。
- 静态范围内未找到 CD0 的显式插入点；C80 与 CD0 是两个不同对象（`sub_145EE0DD0` 中直接比较）。**客户端内部如何同步 CD0 不影响线上字节**：所有消费/显示路径都以 (slot 363..379, 固定模板, 数量) 语义工作。

### 实现决策（以 90 实机先例 + 115 已验证槽位为准）

- 服务端按账号持久化 17 个固定计数；登录 list-0 快照 overlay `{slot 363..379, 固定模板, count}` 行；发放/消耗走 NOTI14 行更新。
- 分解等多处 reader 已由客户端 `sub_14500CE40` 强制按模板重路由到 list 35 语义（客户端原生"入包即转移"）。
- **实机验证点**：灵魂仓库面板是否显示计数；若 list-0 行不生效，备选通道为 NOTI39 GET_ITEM 行或 noti14 list=35 行（同为普通物品行，字节不变只换容器号）。

## 四、服务端实现（v1，attempt 1/3，待实机验证）

### 存储

- `internal/database/account_materials.go`：`account_material_storage(account_id PK, counts jsonb)`（`MigrateAccountMaterials`，建表即迁，老存档无损）；`CommitAccountMaterialSweep` 单事务内锁角色行+账号行，同时改写背包 state 与账号 counts。无事件键：sweep 是状态派生的纯迁移，天然幂等。

### 领域

- `internal/inventory/account_materials.go`：17 模板↔固定槽映射（与 IDB 一致，测试锁定）；`AccountMaterials`（`account-materials-v1`）；`SweepAccountMaterials` 从背包提取（聚合同模板、溢出拒绝）；`Rows()` 只发非零行（与 90 实机先例一致；v1 无消耗路径，数量只增不减）。

### 协议

- `internal/game/protocol/inventory.go`：`InventoryRestoreSpace(space, rows)`——非 0 列表前缀为 `{space}+u16 count`（list 35 无 list0 的额外 u16，client reader `sub_1452D5A80` 证实）。

### 客户端同步时序（已从 IDB 闭环）

1. `NOTI13 list=35`（存储行 363..379）→ reader 置 manager flag（`sub_145ADC2A0` 分支1）。
2. `NOTI13 list=0`（背包全量）→ flag 已置，reader 收割 manager 363..379 槽进存储管线并刷新灵魂仓库面板（分支2 + `sub_145AF4360`）。

### 接入点（入包自动转移）

- 登录进城（main.go entry）：先 sweep（兼容旧档：把背包里已有晶块/灵魂迁入账号仓库），再发 list35+list0。
- 分解 CMD26（disjoint_flow.go）：Disjoint 提交后 sweep；ACK 奖励槽改写为固定存储槽（3037→367 等）；随后 list35+list0 权威刷新（替代原 NOTI14）。
- 拾取（loot_flow.go）：仅当拾取物是 17 模板之一时 sweep+重快照；NOTI39 落点槽改写为固定存储槽；普通物品走原 NOTI14 路径不变。
- 任务奖励（quest_flow.go）：奖励含 17 模板之一时 sweep，list35 包先于原 list0 重发。

### 已知边界 / 待办

- CMD19 移动仍只支持 list 0/2/3；若客户端允许从灵魂仓库面板拖回背包（list 35 移动请求），实机确认后再加。
- 消耗路径（灵魂升级/制作扣减）未实现：客户端消耗检查 `sub_145AFAA20(list=35)` 已定位，服务端待相关制作功能落地时接入。
- 金库（list 2）中已有的 17 模板物品不迁移（v1 范围外）。
- **实机验证点**：登录后灵魂仓库面板显示计数；分解装备后晶块入仓库面板；数量随分解递增；重登后保持。
