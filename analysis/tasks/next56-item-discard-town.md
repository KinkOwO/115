# next56 — 城镇丢弃物品不消失（CMD18 通用删除闭环）移植

> 日期：2026-09-22 · 状态：**已落地分支 `fix/item-discard-town`，待实机验证**
> 来源：`E:\迅雷下载\丢弃修复.zip`（MERGE_REQUEST.md + 3 个源码文件）
> 相关：`cmd/wireprobe/material_delete_flow.go`（已实机验收的材料删除路径）
>
> ⚠️ **方案定位：第一版兜底，不是终态** —— 见文末「§六 后续研讨钩子」。

## 一、根因判定：成立

本地 CMD18 全量路由到 `deleteSkillMaterial`，该路径有两个硬门槛：

1. `w.activeDungeon == nil || !w.activeDungeon.Loaded` → 城镇直接拒绝；
2. `DecodeMaterialDelete` 只接受 slot 121..176、template 3037、reason 2 → 普通物品必拒。

城镇丢弃普通物品时服务端权威背包未删，客户端本地刷新后物品"回来"。包内给出的实机帧
（slot 65 / template 2660296 / count 2，hex `1100000010001a0b...`）与本地
`DecodeMaterialDelete` 的解析域完全对不上，根因判定与我们本地代码一致。

## 二、方案评估：合理，直接采纳

**设计**：CMD18 先试 `deleteSkillMaterial`（副本内材料消耗，已实机验收），失败落
`deleteItems` 通用删除。通用删除 = 材料流程去掉副本/材料约束的同款事务：

- 同款 `CommitCharacterEvent`（幂等 key `item-delete:{account}:{sha256(raw)}`）；
- 同款扣减校验（`Template` 不符或 `Amount < Count` 拒绝——防伪造多扣）；
- 同款清零行移除 + `SaveBag`；
- 同款 ACK（`DeleteItemsReply` 与 `MaterialDeleteReply` **逐字节同形状**，reason=2）+ NOTI14 绝对值行更新。

**基线核对（本轮无漂移）**：包内 MERGE_REQUEST 的「替换前」代码块与本地
`main.go:1308` 的 CMD18 分支逐行一致；包内三个文件依赖的 `add32`/`OrdinaryItem`/
`InventoryUpdate`/`CurrentItemRecordSize`/`CommitCharacterEvent` 签名与本地全部吻合。
不涉及任何我们已修复的文件 → 与前两批修复零冲突。

**细节审查通过的点**：
- 解码拒绝重复 slot、行数上限 56、count ≤ 100000、listType/condition 必须 0；
- op=1/2 都接受（是材料分支的超集，兜底语义正确）；
- 幂等 key 含 raw 哈希 + 账号，同帧重放安全。

**已声明的边界（不在本批范围）**：装备/宠物/账号仓库删除（listType ≠ 0）仍拒绝；
ACK reason 沿用 2；90cn 参考实现的 CMD47 落地模型未采用。

## 三、落地清单（与包内一致的 4 处）

| 文件 | 操作 |
|---|---|
| `internal/game/protocol/delete_items.go` | 新增（`DecodeDeleteItems` + `DeleteItemsReply`） |
| `internal/game/protocol/delete_items_test.go` | 新增（4 例：实机帧/op2/畸形帧/ACK 形状） |
| `cmd/wireprobe/item_delete_flow.go` | 新增（`worldSession.deleteItems`） |
| `cmd/wireprobe/main.go` | CMD18 分支按包内「精确替换」块替换（材料成功短路 → 失败落通用删除） |

## 四、验证

- `go build` / `go vet` 通过；`TestDecodeDeleteItemsLive` 等 4 例新测试通过；
- 全量 `go test ./internal/... ./cmd/...` 待记录；
- 实机（晚上）：城镇丢弃普通物品 → 消失、重进城镇不复原；副本内扔透明方块（材料）行为不变。

## 五、风险

1. **材料格（slot 121..176）非 3037 物品**：副本内扔非透明方块材料 → 材料路径拒（template 校验）→ 落通用删除 → 会成功。这是行为变化（以前被拒），但语义正确——玩家扔了就该没。注意观察是否有意外。
2. **主背包栈数量>1 的整栈丢弃**：客户端一次发 count=整栈，扣到 0 后行被移除、NOTI14 发全零行——与材料路径同款，已验证形状。
3. **装备格丢弃**：若客户端对装备发 listType=0 请求，slot 允许 1..176，但装备存在 `bag.Equipment`/`Worn` 而非 `bag.Items`，会报 `item slot missing` 拒绝（不会误删）。装备丢弃是已知未覆盖项。

## 六、后续研讨钩子（本方案的第一版定位）

**现状**：本批只覆盖 `listType=0`（主背包）的通用删除；材料路径继续独占副本内
template 3037 的语义。其它删除面（装备、宠物、账号仓库、时装、listType≠0 的任何
新形态）要么被拒、要么行为未定义——**"全部落通用删除"在协议上不成立**，CMD18
的报文体是按 listType/op 分型的，不同类型的删除在客户端侧的消费路径（待删除确认
弹窗、装备卸下动画、仓库鉴权）大概率不同，盲目扩 `DecodeDeleteItems` 的接受域只会
制造兼容性债务。

**后续方向（研讨清单，按优先级）**：

1. **权威 IDB 取证（前置必做）**：在 `client/DFO.exe.i64` 中确认客户端**报删除帧前
   的分支**——不同 listType/op 组合下客户端期望的应答（reason 码、是否等待二次
   确认、NOTI14 还是别的行更新）。当前 reason=2 / NOTI14 组合只在主背包 + 材料两个
   场景被实机验证过，**不能外推**。
2. **实机帧采集**：分别触发装备丢弃、宠物放生、仓库删除，用探针记录各场景的
   CMD18 原始帧（对照 events.jsonl 的 `plain_hex`），确认 listType/op 的真实分型，
   再决定是扩 `DecodeDeleteItems` 还是新增独立 opcode 处理器。
3. **架构预案**：若分型坐实，把 `deleteItems` 收敛为「分发器」——按 listType 路由到
   `deleteMainBagRows`（本批）/ `deleteEquipment` / `deleteVault` 等子处理器，
   各自持自己的 ACK 语义；材料路径保持独立。
4. **兼容性红线**：扩接受域前必须先证明"客户端在什么条件下会发这种帧"，避免重蹈
   C2S 三次上限的叠包老路；每次扩型一个假设、可回滚、写 attempt N/3。
5. **观察窗口**：实机验证期留意 `item_delete_refused` 事件的 reason 分布——若出现
   非 "insufficient owned item" / "item slot missing" 的拒绝（如装备丢弃），即为
   上述缺口的第一手证据，回填到本清单第 2 条。

**触发条件**：实机验证发现丢弃类拒绝事件、或用户报告装备/宠物/仓库删除需求时，
从本节启动下一版方案研讨。第一版兜底在此期间保持不动。

**实机第一轮回填（19:41）**：
- CMD18 的 `listType=0` **不区分页**：NOTI13 list0 是统一槽位空间（`Bag.Rows()` 把
  金币/点券/`bag.Items`/`bag.Equipment` 合并下发），装备背包丢弃同样走 listType=0。
  已据此把装备背包行并入通用删除（事务先 Items 后 Equipment，count=1 整行移除）——
  这比预估的边界少一个：装备背包**已覆盖**，剩余缺口是宠物/账号仓库等真正
  listType≠0 的空间。
- 新观察点：投掷品（`[throw]`，template 6004）丢弃时**客户端不发 CMD18**——
  客户端本地拦截。下一版取证清单加一条：IDB 确认客户端丢弃前置条件（物品类型
  白名单/可丢弃标志），并换普通消耗品对照采集。
