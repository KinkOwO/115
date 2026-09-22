# next56 — 城镇丢弃物品不消失（CMD18 通用删除闭环）移植

> 日期：2026-09-22 · 状态：**已落地分支 `fix/item-discard-town`，待实机验证**
> 来源：`E:\迅雷下载\丢弃修复.zip`（MERGE_REQUEST.md + 3 个源码文件）
> 相关：`cmd/wireprobe/material_delete_flow.go`（已实机验收的材料删除路径）

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
