# DFO 本地服 — 对接工作记录：装备分解 CMD26 实现（next41）

> 面向后续维护与多端同步。配套交接文档见 `docs/CMD26装备分解实现报告.md`。

## 一、背景与问题

### 1. 现象

客户端玩家在背包中打开系统分解机分解装备时，点击确定后界面无反应，长时间阻塞等待服务端响应，直到连接强制超时。

### 2. 根因分析

客户端发送的操作为 **CMD 26**（`0x001A`，`ENUM_CMDPACKET_DISJOINT_ITEM`）。在历史版本中，服务端未在网关主循环分发注册 CMD 26，被判定为 `unimplemented_sample` 直接丢弃，未向客户端返回任何 ACK 或错误响应，导致客户端一直阻塞在等待态。

### 3. 实机真源抓包

在 `runtime/roles_persist_..._next37/events.jsonl` 中捕获实机明文帧：

```json
{
  "bytes": 29,
  "checksum_ok": true,
  "hex": "011a001d000000e3d13642a600f173c67453ddb37f3383815a31abfe63",
  "id": 26,
  "kind": "client_frame",
  "peer": "127.0.0.1:41839",
  "plain_hex": "00ffff010b00ae690000000000000000",
  "time": "2026-09-16T11:49:55.044406Z",
  "type": 1,
  "unimplemented_sample": true
}
```

明文包体共 16 字节，前 10 字节为有效结构：

- `00`：mode = 0（系统分解机）
- `ffff`：tool_slot = 0xFFFF（默认分解机）
- `01`：count = 1（分解数量）
- `0b00`：slot = 11（装备背包槽位）
- `ae690000`：template = 27054（装备模板 ID）
- 剩余 6 字节为零填充。

---

## 二、协议逆向与报文布局

### 1. C2S 请求包（客户端发送函数 `sub_145AF5510`）

```c
v3  = sub_146D74000(a1);
sub_146D746E0(v3, 26);              // BEGIN_COMMAND 26 (0x1A)
sub_146D75CC0(v5,  mode);           // write u8  : 模式 (0)
sub_146D76180(v10, 0xFFFF);         // write u16 : 分解机槽位 (0xFFFF)
sub_146D75CC0(v12, count);          // write u8  : 分解物品数量
for (i = items; i != end; i += 4) {
    sub_146D76180(v16, item->slot);      // write u16 : 背包槽位
    sub_146D75CE0(v18, item->template);  // write u32 : 物品模板ID
}
sub_146D75AF0();                    // SEND
```

### 2. S2C 响应包（客户端接收函数 `sub_145273AC0`）

- 框架读取包体首字节 `dl`：
  - `dl == 0`：失败，以 `u16 code` 走拒绝分支。
  - `dl == 1`：成功。
- 成功字段序列：
  - `count_deleted` (u8)：被删除装备数量
  - 循环 `count_deleted` 次：`deleted_slot` (u16)
  - `list` (u8)：容器列表（背包为 0）
  - `tool_slot` (u16)：回传分解机槽位（0xFFFF）
  - `count_rewards` (u8)：产物种类数
  - 循环 `count_rewards` 次：
    - `reward_slot` (u16)：放入产物的背包槽位
    - `reward_template` (u32)：产物物品模板 ID
    - `reward_amount` (u32)：产物数量

客户端解析完成后触发本地扣除装备、增加物品并调用 `sub_14536BAE0` 播放分解特效与音效。

### 3. 同步机制

成功 ACK 下发后，服务端立即下发 **NOTI 14（`InventoryUpdate`）**，推送全量背包快照，保证双端槽位状态权威一致。

---

## 三、架构设计与分层实现

1. **协议层 (`internal/game/protocol/disjoint.go`, `disjoint_test.go`)**：
   - 实现 `DecodeDisjointItem`：校验最小长度、物品数量及合法槽位。
   - 实现 `DisjointItemSuccess`：构建小端紧凑 ACK 报文。
   - 实现 `DisjointItemRefused`：构建错误码响应。
   - 单元测试锁定实机抓包向量与报文编码。

2. **背包领域层 (`internal/inventory/disjoint.go`, `disjoint_test.go`)**：
   - 校验源配置与背包规则，拦截空请求与无效槽位。
   - 穿戴防拆保护：检测命中已穿戴装备 (`b.Worn`) 立即拒绝。
   - 逐槽扣除装备，按基础固定基线（每件装备 20 个无色小晶块，Template 3037）产出。
   - 优先堆叠至现有材料栏（`[material]` 121~176）未满堆叠，超出后寻找空槽开辟新堆叠。

3. **事务层 (`internal/loot/disjoint.go`)**：
   - 生成幂等事务 Key `disjoint:<toolSlot>:<slot>:<template>`。
   - 通过 `Store.CommitCharacterEvent` 保证角色状态更新与事件收据落库原子性。

4. **网关与主流转 (`cmd/wireprobe/disjoint_flow.go`, `main.go`, `request_scope.go`)**：
   - 注册 CMD 26 至 `observedGameRequest` 白名单。
   - 在主分发循环中处理 CMD 26，执行事务扣减、构造 ACK、补发 NOTI 14。

---

## 四、已知边界与后续演进

1. **产物丰富度与公式**：当前按固定 20 个无色小晶块产出。后续需结合 PVF 装备品级（白/蓝/紫/粉/橙/神话）、强化等级计算有色小晶块与灵魂产出。
2. **账号共享仓库对接**：115 级客户端晶块与灵魂存放于账号维度的独立仓库侧边栏中。目前临时写入角色个人背包材料栏，后续需扩展账号共享存储字段。
3. **副职业分解机**：当前实现了系统默认分解机（`tool_slot=0xFFFF`），玩家自建分解机、磨损与暴击奖励待后续扩展。
