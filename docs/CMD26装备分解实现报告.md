# 装备分解 CMD26 实现报告（迁移用完整文档）

> 本报告依据开发会话上下文汇总，供迁移到其它机器/代码库使用，不依赖本地文件。
> 目标：将「装备分解（CMD 26 `ENUM_CMDPACKET_DISJOINT_ITEM`）」服务端实现完整迁移。

---

## 1. 背景与问题

### 1.1 客户端现象

玩家在游戏中点击「分解装备」（Disassemble / Disjoint）后，客户端**无任何反应**，卡在等待服务端响应的状态，直到连接被强制关闭。

### 1.2 根因

客户端实际发送的是 **CMD 26**（`0x001A`，`ENUM_CMDPACKET_DISJOINT_ITEM`），但服务端 `cmd/wireprobe/main.go` 从未注册 CMD 26 的分发逻辑。该请求被当作 `unimplemented_sample` 丢弃，**服务端未返回任何 ACK 包**，导致客户端一直阻塞等待。

### 1.3 实机抓包证据（权威基线）

最新运行日志目录 `server/work/dfo-lan/runtime/roles_persist_..._next37/events.jsonl` 中，客户端在 `2026-09-16T11:49:55.044406Z` 发送：

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

其中 `plain_hex` 是解密后的明文包体（16 字节），是本实现所有字段布局的**唯一权威依据**。

---

## 2. 协议逆向结论（客户端 `DFO.exe` 115 权威分析）

### 2.1 C2S 请求包编码（发送端 `sub_145AF5510`）

伪代码：

```c
v3  = sub_146D74000(a1);
sub_146D746E0(v3, 26);              // BEGIN_COMMAND 26 (0x1A)
sub_146D75CC0(v5,  mode);           // write u8  : 分解模式/机器类型 (0)
sub_146D76180(v10, 0xFFFF);         // write u16 : 机器槽位/默认分解机 (0xFFFF)
sub_146D75CC0(v12, count);          // write u8  : 分解物品数量
for (i = items; i != end; i += 4) {
    sub_146D76180(v16, item->slot);      // write u16 : 背包槽位
    sub_146D75CE0(v18, item->template);  // write u32 : 物品模板ID
}
sub_146D75AF0();                    // SEND
```

**字段布局（小端）：**

| 偏移 | 大小 | 字段 | 说明 |
| --- | --- | --- | --- |
| 0 | u8 | `mode` | 分解模式 / 机器类型，固定 0 |
| 1 | u16 | `tool_slot` | 工具槽位，0xFFFF = 系统默认分解机 |
| 3 | u8 | `count` | 本次分解装备数量 |
| 4 | count×6 | `entry[]` | 每项 = `u16 slot` + `u32 template` |

实机 `00ffff010b00ae690000000000000000` 对照：

- `00` → mode = 0
- `ffff` → tool_slot = 0xFFFF
- `01` → count = 1
- `0b00` → slot = 11
- `ae690000` → template = 27054 = 0x000069AE
- 末尾 6 字节为对齐/填充，忽略。

### 2.2 S2C 成功 ACK 编码（接收端 `sub_145273AC0`）

注册信息：

```asm
1452a2185: lea r8, sub_145273AC0
1452a218c: mov rcx, rbx
1452a218f: lea edx, [r9+1Ah]       ; CMD 0x1A = 26
1452a2193: call sub_1459A2FB0
```

处理函数 `sub_145273AC0` 读取流程：

1. 框架层读包体第 1 字节 `dl`：
   - `dl == 0`：拒绝/失败 → 跳 `loc_145274C03`，以 `r8w` 作为错误码 switch。
   - `dl == 1`：成功，进入后续解析。
2. 成功流程字段序列（对应读辅助函数）：
   - `sub_146EA09F0` (read_u8) : `count_deleted` 已删除装备数量
   - 循环 `count_deleted` 次：`sub_146EA1920` (read_u16) : `deleted_slot`
   - `sub_146EA09F0` (read_u8) : `list` 容器类型（0 = 背包）
   - `sub_146EA1920` (read_u16) : `tool_slot`（原样回传 0xFFFF）
   - `sub_146EA09F0` (read_u8) : `count_rewards` 产物行数
   - 循环 `count_rewards` 次：
     - `sub_146EA1920` (read_u16) : `reward_slot`
     - `sub_146EA0BA0` (read_u32) : `reward_template`
     - `sub_146EA0BA0` (read_u32) : `reward_amount`

客户端解析后：本地移除 `deleted_slot` 装备 → 将产物放入 `reward_slot` → 调用 `sub_14536BAE0` 播放分解特效音效并弹出结算窗口。

**成功 ACK 字节布局：**

| 偏移 | 大小 | 字段 |
| --- | --- | --- |
| 0 | u8 | `1`（成功标志 dl=1） |
| 1 | u8 | `count_deleted` |
| 2 | count_deleted×2 | `deleted_slot[]` |
| … | u8 | `list` (0) |
| … | u16 | `tool_slot` (0xFFFF) |
| … | u8 | `count_rewards` |
| … | count_rewards×10 | `reward_slot(u16) + template(u32) + amount(u32)` |

### 2.3 S2C 失败 ACK

复用通用 `Refusal(code)` = `u8 0 + u16 code`，与现有 CMD15/43 等一致。本实现失败分支发送 `Refusal(4)`。

### 2.4 权威状态同步

成功 ACK 后，服务端紧接着下发 **NOTI 14（`InventoryUpdate`）**，将扣减装备与增加晶块后的背包全量快照同步给客户端，保证两端槽位一致。

---

## 3. 服务端实现清单（迁移必须包含的代码）

### 3.1 协议层 `internal/game/protocol/disjoint.go`

导出符号：

```go
type DisjointItemEntry struct {
    Slot     uint16
    Template uint32
}

type DisjointItemRequest struct {
    Mode     byte
    ToolSlot uint16
    Items    []DisjointItemEntry
}

func DecodeDisjointItem(p []byte) (DisjointItemRequest, error)

type DisjointRewardEntry struct {
    Slot     uint16
    Template uint32
    Count    uint32
}

type DisjointItemResult struct {
    DeletedSlots []uint16
    List         byte
    ToolSlot     uint16
    Rewards      []DisjointRewardEntry
}

func DisjointItemSuccess(r DisjointItemResult) ([]byte, error)
func DisjointItemRefused(code uint16) []byte
```

实现要点：

- `DecodeDisjointItem`：校验 `len(p) >= 4`，`count == p[3] != 0`，长度 `>= 4 + count*6`，逐项解析 slot/template，slot 为 0 时报错。
- `DisjointItemSuccess`：校验 `0 < len(DeletedSlots) <= 255`、`len(Rewards) <= 255`；按 §2.2 布局编码（复用 `add16`/`add32` 小端辅助函数，若迁移目标无此函数则需用 `encoding/binary.LittleEndian`）。
- `DisjointItemRefused`：直接 `return Refusal(code)`。

单元测试 `disjoint_test.go`：

- `TestDisjointItemDecodeLiveCapture`：用实机 `00ffff010b00ae690000000000000000` 向量锁定解码。
- `TestDisjointItemSuccessBuild`：锁定成功 ACK 编码（slot 11、template 3037、count 4 期望字节序列）。
- `TestDisjointItemRefused`：`Refused(4)` 期望字节 `000400`。

### 3.2 背包领域层 `internal/inventory/disjoint.go`

导出符号与常量：

```go
const ClearCubeFragmentID uint32 = 3037           // 无色小晶块模板ID
const DefaultDisjointCubeYield uint32 = 20        // 每件装备固定产出数量

type DisjointReward struct {
    Slot     uint16
    Template uint32
    Count    uint32
}

type DisjointResult struct {
    DeletedSlots []uint16
    List         byte
    ToolSlot     uint16
    Rewards      []DisjointReward
}

func (b Bag) Disjoint(
    c catalog.LootCatalog,
    r BagRules,
    requestedSlots []uint16,
    toolSlot uint16,
) (Bag, DisjointResult, error)
```

`Bag.Disjoint` 逻辑：

1. 校验：`requestedSlots` 非空、`r.Source == c.Source.Checksum`、装备槽范围 `r.EquipmentSlots[0] != 0`、材料槽范围 `[material]`（通常 `[121,176]`）配置存在且 `[0] <= [1]`。
2. **穿戴防拆保护**：若请求槽位命中 `b.Worn` 中的任一穿戴装备，返回错误。
3. 扣减装备：从 `b.Equipment` 中逐槽移除命中项，累计 `totalCubes += DefaultDisjointCubeYield`；找不到该槽装备则报错。
4. 晶块堆叠上限：默认 `r.MissingStackLimit`；若 catalog 中存在 3037 且 `StackLimit != 0`，用 `item.StackLimit`。
5. 发放晶块：
   - 优先堆叠到材料槽范围内已存在的 3037 堆（有剩余空间）。
   - 剩余部分找材料槽范围内第一个空槽新建 3037 堆。
   - 记录每笔 `DisjointReward{Slot, Template, Count}`。
   - 材料槽满仍发不完则返回 `material inventory is full` 错误。
6. 返回 `Bag`（已扣装备、已加晶块）与 `DisjointResult`（含 `DeletedSlots`、`ToolSlot`、`Rewards`、`List=0`）。

单元测试 `disjoint_test.go` 依赖 `testBagCatalog(t)`（复用 `shop_test.go` 中的辅助函数，加载 `configs/loot.next25.json` 与 `configs/inventory.current37.json`）：

- `TestBagDisjointSuccess`：槽 11 删除，材料槽 121 由 10 堆叠到 30。
- `TestBagDisjointNewSlot`：无现存 3037 时在空槽 121 新建 20。
- `TestBagDisjointWornRejected`：穿戴槽 15 被拒绝。
- `TestBagDisjointMissingItemRejected`：不存在槽 99 被拒绝。

### 3.3 事务层 `internal/loot/disjoint.go`

导出符号：

```go
type DisjointReceipt struct {
    DeletedSlots []uint16                       `json:"deleted_slots"`
    ToolSlot     uint16                         `json:"tool_slot"`
    Rewards      []protocol.DisjointRewardEntry `json:"rewards"`
    Source       string                         `json:"source"`
}

func (s *Service) Disjoint(
    ctx context.Context,
    role storage.Character,
    r protocol.DisjointItemRequest,
) (storage.Character, DisjointReceipt, bool, error)
```

逻辑：

1. 校验 `role.ConfigVersion == s.Catalog.Source.Checksum`；`r.Items` 非空。
2. 提取所有请求槽位 `slots`。
3. 构造幂等 key：`fmt.Sprintf("disjoint:%d:%d:%d", r.ToolSlot, r.Items[0].Slot, r.Items[0].Template)`。
4. 调用 `s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.Checksum, key, s.Rules.Model, ...)`：
   - 闭包内 `inventory.ReadBag(current.State)` → `b.Disjoint(...)` → `inventory.SaveBag(current.State, b)`。
   - 将 `DisjointResult` 转为 `protocol.DisjointRewardEntry`，`json.Marshal` 成收据返回作为事务事件体。
5. 通过 `s.Store.CharacterEventReceipt` 读回收据并 `json.Unmarshal`，校验 `Source` 与 `len(DeletedSlots) == len(slots)`。
6. 回写 `saved.WireID = role.WireID`，返回 `(saved, receipt, applied, nil)`。

> 幂等：重放相同 key 只返回首次结果，不二次扣装备/发晶块。

### 3.4 网关流转 `cmd/wireprobe/disjoint_flow.go`

```go
func (w *worldSession) disjointItem(p []byte) ([]outboundPacket, error)
```

逻辑：

1. 前置校验 `w != nil`、`w.role.ID != 0`、`w.loot != nil`。
2. `protocol.DecodeDisjointItem(p)`。
3. `w.loot.Disjoint(ctx, w.role, r)`（5s 超时）。
4. `inventory.ReadBag(saved.State)`。
5. `protocol.DisjointItemSuccess(...)` 构造 ACK（`List=0`、`ToolSlot`、`Rewards=receipt.Rewards`）。
6. `protocol.InventoryUpdate(b.Rows())` 构造 NOTI 14。
7. `w.role = saved`。
8. 返回两个 outboundPacket：
   - `{"disjoint_item_ack", 1, 26, ack}`
   - `{"disjoint_item_inventory_updated", 0, 14, update}`

### 3.5 分发注册

**`cmd/wireprobe/main.go`** 在 shop（CMD21/22）分发之后、CMD143 之前插入：

```go
if worldState != nil && bootstrapped && frame.ID == 26 && lootService != nil {
    if !verified {
        event(map[string]any{"kind": "disjoint_rejected", "id": frame.ID, "reason": "checksum failed"})
        continue
    }
    plan, e := worldState.disjointItem(plaintext)
    if e != nil {
        event(map[string]any{"kind": "disjoint_refused", "id": frame.ID, "character_id": worldState.role.ID, "reason": e.Error()})
        if e = sendPayload(1, frame.ID, protocol.Refusal(4)); e != nil {
            return
        }
        continue
    }
    for _, packet := range plan {
        if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
            return
        }
        event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID})
    }
    continue
}
```

**`cmd/wireprobe/request_scope.go`** 的 `observedGameRequest` 白名单中加入 `26`：

```go
case 0, 1, ..., 15, 16, 19, 26, 28, ... , 1554:
```

---

## 4. 涉及文件与 Git 记录

### 4.1 新增文件

| 文件 | 职责 |
| --- | --- |
| `internal/game/protocol/disjoint.go` | CMD26 请求解码 / 成功与拒绝包构建 |
| `internal/game/protocol/disjoint_test.go` | 实机向量与编码锁定 |
| `internal/inventory/disjoint.go` | `Bag.Disjoint` 装备扣减与晶块发放 |
| `internal/inventory/disjoint_test.go` | 分解成功/空槽/穿戴拒绝/缺项拒绝 |
| `internal/loot/disjoint.go` | `Service.Disjoint` 事务持久化 + 幂等收据 |
| `cmd/wireprobe/disjoint_flow.go` | `disjointItem` 业务编排 |
| `docs/protocol/next41-disjoint-item.md` | 逆向证据、包布局、已知未实现项 |

### 4.2 修改文件

| 文件 | 修改点 |
| --- | --- |
| `cmd/wireprobe/main.go` | 插入 CMD26 分发分支 |
| `cmd/wireprobe/request_scope.go` | 白名单加入 opcode 26 |
| `CHANGELOG` | 记录本次新增特性 |

### 4.3 Git 提交

- Commit: `2ef208d`，消息 `feat(protocol): implement equipment disjoint CMD26 and reward flow`。
- 共 10 文件，+706 行，-1 行。

---

## 5. 构建与验证

```bash
# 单元测试（协议/背包/事务/网关）
go test -count=1 ./internal/game/protocol ./internal/inventory ./internal/loot ./cmd/wireprobe

# 全仓测试与静态检查
go test ./...
go vet ./...

# 构建服务端可执行文件
go build -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe
```

全部通过（`go test ./...` 全绿，`go vet ./...` 零告警）。

---

## 6. 已知未实现部分（后续迁移/完善路线）

实机验证已确认「客户端发 CMD26 → 收 ACK + NOTI14 → 装备扣除 + 分解动画/结算弹窗」链路打通。但以下为**已验证待完善**项：

1. **产物种类与计算公式不全**
   - 原版：按装备品级（白/蓝/紫/粉/橙/神话）、等级、强化等级产出各色小晶块（无色/红/白/蓝/黑/金）+ 对应品级装备灵魂（普通/高级/稀有/神器/传说/史诗）。
   - 当前：固定每件产 20 个无色小晶块（3037）。后续需解析 PVF 分解掉落倍率与灵魂公式。

2. **产出物存储位置未接 115 账号共享仓库**
   - 原版：115 客户端背包右侧独立侧边栏「Cube（晶块仓库）」与「Soul（灵魂仓库）」，晶块/灵魂属全账号角色共享，不占个人材料栏格子与负重。
   - 当前：临时写入角色背包普通材料栏 `[material]`（槽位 121~176）。后续需扩展账号维度晶块/灵魂存储字段与专用同步协议。

3. **分解机高级机制**
   - 系统默认分解机（`tool_slot=0xFFFF`）已通；玩家副职业分解机、分解机耐久消耗与修理、分解暴击（Jackpot 大量晶块产出）尚未实现。

---

## 7. 迁移落地清单（checklist）

- [ ] 复制 §4 全部新增文件到目标代码库对应目录。
- [ ] 将 `main.go` / `request_scope.go` 的改动合并入目标（注意与目标已存在的 shop CMD21/22 分支共存）。
- [ ] 确认目标仓库具备 `catalog.LootCatalog`、`Bag`/`BagRules`、`inventory.ReadBag/SaveBag`、`loot.Service`、`storage.Character`、`CommitCharacterEvent`、`protocol.InventoryUpdate`、`Refusal`、`add16/add32` 等前置符号（均为本项目既有公共接口）。
- [ ] 确认配置 `configs/inventory.current37.json` 的 `[material]` 槽位范围与 `missing_stack_limit` 在目标环境一致。
- [ ] 运行 §5 构建与测试命令。
- [ ] 实机回归：分解装备 → 收 ACK + NOTI14 → 装备扣除 + 晶块入包 + 分解动画结算弹窗。
