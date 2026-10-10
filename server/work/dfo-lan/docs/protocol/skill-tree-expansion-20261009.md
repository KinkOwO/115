# 第二技能页（技能类型扩展券）2026-10-09

## 现象

技能窗口的第二个页签（"技能类型 2"）永远是锁的。业主说明：**正常要商城购买"技能类型扩展券"才能解锁**。

## 根因（已确证，不是猜测）

服务端下发的角色信息追加包（`internal/game/protocol/entry_addition.go` 的  
`UserInfoAdditionProbe`）里有一个"技能类型选择"字节，**被硬编码成 `0xff`**：

```go
p = append(p, 0xff)   // native unset selected skill-tree byte   ← 修复前
```

`0xff` 在客户端就表示"第二技能页从未解锁"，所以任何角色都开不了第二页。

### 三条独立证据

1. **86JP 完整 C# 服务端**  
   `E:/DNF/DNF-86JPA12/AUM管理组件/ServerS4A12-AUM/Server/DfoServer/Game/Skills/SkillTreeExpansionState.cs`  
   明确写死这套 wire 语义：
   ```
   0xff = 未购买（LockedWireValue）；0/1 = 已购买且当前为该技能页
   // PVF item 821（技能类型扩展券）购买后立即生效，不进入背包。
   ```
   并且该字节确实下发在 USERINFO 里（`Network/Builders/Init/UserInfoSubtype1Builder.cs:121`  
   在装备列表 / clone_title / name_tag 之后写 `SkillTreeIndex`，紧接两页技能）。
2. **115 客户端符号表**里有 UI 控件 `change_type_btn`（`analysis/dumps/xorstr_addr_to_text.json`  
   → `0x14A1D77E0`），正是官方说明里的"技能类型替换按钮"。
3. **逐字节回归测试反向证实**：`internal/game/protocol/entry_addition_test.go` 的  
   `TestAdditionConsumesBothNativeSkillTrees` 用独立原生游标 fixture 逐字节比对整个包；  
   改成"按状态取值、默认锁定位"之后**仍然通过** ⇒ 这个位置原本就是 `0xff`。

### 商品定位（PVF 真源）

`etc/(r)cerashop.etc`（提取：`.workbuddy/tmp/dfo-tool.exe pvfinspect`）：

```
3000150 821 1 0 0 390 0 0 `Dual Skill Build License` 0 0 -1 `` -1
```

即 **product 3000150 → template 821，390 Cera**，落在 `[item mod or ext]`，  
并出现在 `[not stackable buy]`（= 每角色限购一张，与官方说明一致）。  
中文物品表 `name_821 = 技能类型扩展券`。

## 改了什么

| 文件                                             | 改动                                                                                                                           |
| ---------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `internal/game/protocol/entry_addition.go`     | 新增 `EntryAdditionProbe.SkillTreeType`（0=未解锁 / 1=类型1 / 2=类型2）与 `SkillTreeLocked=0xff`、`skillTreeWireIndex()`；body 不再写死 `0xff` |
| `internal/character/service.go`                | `State.SkillTreeType byte`（**零值 = 未解锁**，老存档与漏赋值都保持锁定）                                                                        |
| `internal/character/detail.go`                 | 入场包传入 `state.SkillTreeType`                                                                                                  |
| `internal/cashshop/skill_tree_expansion.go`（新） | `TryPurchaseSkillTreeExpansion` + `SkillTreeLedger`：识别 3000150/821，走专路（`[item mod or ext]` 普通路径本来会拒绝）                        |
| `internal/cashshop/pilot.go`                   | `Purchase` 链里接上该 SKU                                                                                                         |
| `internal/database/cash_purchase.go`           | `PurchaseCashSkillTreeExpansion` + `unlockSkillTreeType`：**同一事务**里把 `skill_tree_type` 0→1                                    |
| `internal/cashshop/types.go`                   | `CashReceipt.SkillTreeUnlocked`                                                                                              |
| `cmd/wireprobe/shop_pilot.go`                  | `preparedBagLedger.PurchaseCashSkillTreeExpansion` 转发                                                                        |
| `cmd/wireprobe/client_dispatch_inventory.go`   | 购买成功后追加 `unlockRefresh`（mode0+mode1 USERINFO1）                                                                               |
| `cmd/wireprobe/request_scope.go`               | 登记 cmd260 取样（**只取证，不应答**）                                                                                                    |

### 为什么购买后能立刻生效

`unlockRefresh`（`cmd/wireprobe/quest_flow.go:31`）本来就是"改了 USERINFO1 解锁字节之后  
重发 mode0+mode1"的现成机制（皮肤幻化栏券走同一条路，同一字节 `+0x198`）。  
技能类型字节就在同一个包里 ⇒ 直接复用，不需要新协议。

## 怎么验收（一次）

1. 用一键启动器正常启动（**先看 `gateway.out` 第一行确认在跑哪个 exe**）。
2. 进角色 → 商城 → 道具 → 扩展 → 买 **"Dual Skill Build License"（390 Cera）**。
3. 期望：购买成功后技能窗口出现**技能类型 1 / 技能类型 2** 两个页。
4. 自己校验服务端：`events.jsonl` 里应有恰好一条  
   `{"kind":"cera_skill_tree_expansion_refresh", ...}`，  
   且角色存档（SQLite `runtime/storage/dfolan.sqlite3` 的 `characters.state`）  
   的 `skill_tree_type` 从无到 `1`。

## ★ 第一次交付后业主实测：买了券但仍未开启（2026-10-09 17:43）

服务端侧**完全正常** —— 17:41 会话实证：

```
{"applied":true,"before":11632980,"after":11632590,"charged":390,
 "deliveries":[{"product":3000150,"template":821,...}],
 "kind":"cera_purchase_committed", "character_id":6}
{"character_id":6,"kind":"cera_skill_tree_expansion_refresh"}
```

存档 `characters.id=6`（DPS）的 `skill_tree_type=1` 已落库。  
⇒ 说明**只改 mode1（UserInfoAddition）不够**。

### 补的第二处：mode0（UserInfoBasicProbe）

`internal/game/protocol/entry_userinfo.go` 里也有一处同样的硬编码：

```go
p = add16(p, 0)
p = append(p, 0xff)   // 0x14563a4a0 native unset value   ← 修复前
```

字段顺序与 86JP `UserInfoSubtype0Builder` 的  
`WriteUInt16(MoodValue); WriteByte(SkillTreeIndex)` **逐字段对应**，  
而 `0xff` 恰好是 SkillTreeIndex 的"未解锁"哨兵（同 `LockedWireValue`）。  
⇒ **mode0 与 mode1 都必须携带该字节**，与 86JP 的 Subtype0/Subtype1 两处下发完全一致。

自证：`TestMode0CarriesSkillTreeSelector` 钉死位置与三种取值 ——  
selector 在 **mode0 包 offset 286**，`0xff / 0x00 / 0x01`，且只动一个字节、包长不变。

## ★ 第三轮：cmd260 切换（实机样本驱动，已实现）

业主第二轮反馈"**无法切换**到第二技能页" —— 即**页签已经出现了**（说明 mode0 那处  
修复生效），只是点"技能类型替换按钮"没反应。

### 实机明文（本轮唯一真源）

17:51:09 业主点按钮，`events.jsonl` 的 `client_frame`：

```json
{"id":260,"kind":"client_frame","peer":"127.0.0.1:8499","plain_hex":"003e745e7a000000","type":1}
```

`body[0] = 0x00`，与存档当时的 wire 索引（0 = 技能类型 1）一致；其余 7 字节在 115 上  
没有任何证据，**原样回显、不做解释**（不猜字段含义）。

### 实现

| 文件                                           | 改动                                                                                                |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `internal/character/skill_tree.go`（新）        | `SetSkillTreeType`（未解锁则拒绝；与其它技能变更共用 `CommitCharacterEvent`，同 key 幂等）+ `SkillTreeWireIndex(State)` |
| `cmd/wireprobe/skill_tree_flow.go`（新）        | `changeAnotherSkillTree`：**以存档为准**取当前页（86JP 同样用 `repo.LoadSkillTreeIndex` 覆盖客户端上报值），toggle 0↔1，回包 |
| `cmd/wireprobe/skill_flow.go`                | `case 260` 接进技能命令 switch                                                                          |
| `cmd/wireprobe/client_dispatch_character.go` | 260 加入技能命令放行名单                                                                                    |
| `internal/game/protocol/*.go`                | `skillTreeWireIndex` 导出为 `SkillTreeWireIndex` 供 character 层复用                                     |

**回包形状**照 86JP `SkillHandler.BuildChangeAnotherSkillTreeAck`：  
`body[0]` 置 1（成功标志 —— 与 115 收包分发器"body[0] 当成功标志"的既有约定一致，  
见 `protocol.OpenSkinSlotReply` 的注释），`body[1]` 覆盖为服务端确认的新索引，其余原样回显；  
未解锁时 `body[1] = 0xff`（同 86JP 的 locked 分支）。

> ✅ **回包形状已被实机证实**（17:58 会话）：服务端回 `01013e745e7a000000`（切到第二页）  
> 与 `0100b2ce8d59000000`（切回第一页）之后，客户端两次都**真的切过去了**  
> （对应两条 `skill_tree_switched`）。所以 `[0x01, 新索引, body[1:]…]` 这组形状正确，  
> 86JP 的那次外推成立，不再是猜测。  
> 回退办法（若将来出问题）：把 `client_dispatch_character.go` 放行名单里的 `260` 去掉。

### 构建（第三轮）

三方同一 **30947840 字节 / 17:55 / md5 `35d4da6ed73402d765d66ee13cb8b8a3`**；  
备份 `bin/wireprobe-pvf.exe.bak-20261009-1756-pre-cmd260.exe`。  
字节自证：`skill_tree_switched` / `source-change-skill-tree-v1` /  
`second skill page is not unlocked` 在新 exe 各 1 次、旧备份 0 次。

## ★ 第四轮：业主实测两处反馈（都已修，均为服务端 bug）

### 反馈 1「切不回第一技能页」= 幂等键设计错了

实机日志（17:58 会话）：

```
09:58:51  发 003e745e7a000000 → skill_tree_switched        切到 1 ✓
09:58:53  发 01b2ce8d59000000 → skill_tree_switched        切回 0 ✓
09:58:53  发 003e745e7a000000 → skill_tree_switch_replayed ✗ 被当重放
09:59:15~ 反复交替发这两个包   → 全部 replayed              ✗
```

**客户端在同一个会话里复用同一个 body**（切第二页发 `003e745e7a…`、切第一页发  
`01b2ce8d59…`，反复点击就是这两个包交替）。我拿 body 的 sha256 当幂等 key  
⇒ 第二次点击变成"重放"不生效，**但回包仍照发新索引** ⇒ 服务端存档与客户端显示错开。

**修法**：不再用 body 当键，改用「**客户端上报的当前页 vs 服务端存档**」判据 ——  
一致 = 新点击（执行 toggle）；不一致 = 重发/状态漂移（不切换，只回包纠正）。  
每次真实切换用 `time.Now().UnixNano()` 作 key；重发由该判据挡住。两层保护。

### 反馈 2「第二技能页无法加点」= 三处入口闸写死 `Tree != 0`

`events.jsonl` 实证：客户端发的加点包 **tree = 1**，服务端直接  
`skill_refused: unsupported skill purchase`。

三处同款门（下游逻辑其实**都**支持 tree=1）：

| 位置                                                     | 修复前             | 修复后            |
| ------------------------------------------------------ | --------------- | -------------- |
| `protocol/skill_mutation.go` `DecodeSkillPurchase`     | `r.Tree != 0`   | `r.Tree > 1`   |
| `character/learning.go` `Learn`                        | `req.Tree != 0` | `req.Tree > 1` |
| `character/learning.go` `MoveSkill` / `MoveSkillTotal` | `req.Tree != 0` | `req.Tree > 1` |

（对照：`boostup_vp.go:32` 本来就写的是 `req.Tree > 1`，可见底层一直是按双页设计的。）

并在 `Learn` 内补「tree=1 但**未解锁**则拒绝」，与 86JP 的 `IsUnlocked` 检查同义。

**离线自证**：把实机那 160 字节真实包喂给修好的 `DecodeSkillPurchase` ——  
得到 `tree=1 entries=22 mode=1 preset=2`、`entry0: id=301 delta=5 refund=0`，  
**全部通过**（修复前在 Tree 那一行就返回了）。

### 构建（第四轮）

三方同一 **30948352 字节 / 18:03 / md5 `1fc8f4bd9d6c6b1b02b80e976115bf23`**；  
备份 `bin/wireprobe-pvf.exe.bak-20261009-1806-pre-tree1.exe`。  
字节自证：`skill_tree_switch_corrected` 在新 exe 1 次、旧备份 0 次。

## ★ 第五轮：reset 跳页 + 其余 Tree 闸（18:05 会话）

业主反馈："在第二技能页点 **reset，会跳转到第一技能页**。"

### 根因 1：NOTI19 里**根本没有"当前页"**

解析官方抓包（`analysis/captures/official_20261002/20261002-211855_.../frames.jsonl`  
的 op19，共 188 条，取一条 592B 逐字段解）：

```
top : field1 = 115 (level) , field2(bytes) ×2          ← 两棵树
tree: field1 = SP , field2 = TP , field3 = skills…     ← 只有这三样
```

⇒ **官方 NOTI19 不含任何"当前页"字段**（我们多写的树级 `field 4` 官方也不发）。  
客户端重建技能窗口时只能回到第一页 —— 而 reset 恰好会重建，于是跳页。  
（这也解释了为什么 cmd260 的回包能切页：页选择只由 USERINFO 的 SkillTreeIndex 决定。）

**修法**：reset 成功、且当前页为第二页时，追加 `unlockRefresh`（mode0+mode1，  
把 SkillTreeIndex 重新送过去）。见 `skill_flow.go` 的 `case 483`。

### 根因 2：还有两处 Tree 硬闸（"第二页拖技能 / 存快捷栏"失败的元凶）

实机 18:07 日志里出现过 `skill_refused: unsupported skill slot total tree`：

| 位置                                                  | 命令             | 前 → 后               |
| --------------------------------------------------- | -------------- | ------------------- |
| `protocol/skill_mutation.go` `DecodeSkillSlotTotal` | cmd2179 自动加点布局 | `Tree != 0` → `> 1` |
| `protocol/skill_mutation.go` `DecodeSkillMove`      | cmd28 拖放技能     | `Tree != 0` → `> 1` |

加上第四轮三处（`DecodeSkillPurchase` / `Learn` / `MoveSkill` / `MoveSkillTotal`），  
**一共五处**入口闸原本写死只认 tree 0（`boostup_vp.go` 一直是 `> 1`）。

配套改 `skill_slot_total_test.go`：旧断言把 `tree=1` 当 "non-zero tree" 拒掉，  
现在改为 tree=1 合法、越界的 tree=2 仍拒。

### 构建（第五轮）

三方同一 **30950400 字节 / 18:14 / md5 `a38dbabe8bfe63279298ad3642da2d10`**；  
备份 `bin/wireprobe-pvf.exe.bak-20261009-1818-pre-skillmove.exe`。

## ★ 第六轮：auto-set 同样跳页，而且作用在错误的页（业主提问）

业主问："autoset 会不会也导致这种情况" —— **会**，而且它比 reset 还多一个毛病：

1. **同样跳页**：auto-set 与 reset 是**同一个 cmd483 的两种形态**，同样发  
   `skill_state_restored`(NOTI19) ⇒ 同样让客户端重建技能窗口 ⇒ 同样跳回第一页。  
   同一个 case 里还有 **cmd2347（预设/连招重置）**，以及 **cmd2179（自动加点的快捷栏  
   布局，走 `skillMutationResponsePlan` 也会重绘 NOTI19）** —— 都会。
2. **还会作用在错误的页**：`cs.ResetAutoSet(ctx, w.role, key, 0, 7)` **写死 tree 0**  
   —— 在第二页点自动加点，被洗掉的是**第一页**。该函数本身支持 tree  
   （`resetAutoState` 里就有 `tree > 1` 的校验），只是调用点写死了。  
   auto-set 的 body 是 opaque（早期按 (tree,mask) 读会得到 tree 111 / mask 40），  
   所以只能**从存档取当前页**。

**修法**：抽出 `skillTreePageRefresh(role)`（当前页是第二页才发 mode0+mode1），  
在**四个会重绘 NOTI19 的出口**统一调用：cmd483 reset 分支、cmd483 auto-set 分支、  
cmd2347、cmd2179。auto-set 的 tree 改为从存档当前页取。

## ★ 第七轮：快捷栏 / VP 面板也写死了第一页（业主提问引出）

业主："第二页的 autoset 技能加点会和第一页的技能加点一样，会自动排列技能栏。"

**完全正确** —— 服务端拿到了 `req.Tree` 却没用，三处写死 index 0：

| 位置                                                | 命令                 | 症状                          |
| ------------------------------------------------- | ------------------ | --------------------------- |
| `character/learning.go` `MoveSkillTotal`          | cmd2179 自动加点的快捷栏布局 | 第二页 autoset 排出的布局被存进**第一页** |
| `character/learning.go` `MoveSkill`               | cmd28 拖放技能         | 第二页拖技能会覆盖第一页的快捷栏            |
| `character/skill_variation.go` `VariationRestore` | VP（进化/突破）面板恢复      | 切到第二页后，面板仍显示第一页的选择          |

`MoveSkillTotal` 里最直白：

```go
rows, e := s.skillRows(current, state, 0)   // ← req.Tree 从没被使用
state.SkillSlots[0] = map[uint16]uint16{}
state.SkillSlots[0][v.ID] = v.Slot
```

**修法**：三处一律改用「请求页 / 存档当前页」。  
`VariationRestore` **签名保持不变**，内部按 `State.SkillTreeType` 取页 ——  
学习、拖放、重置都不切页，所以"当前页"就是玩家正在操作的那一页；这样它  
7 个调用点一处都不用动。

> 顺带记录：`tag_character.go` 的 `TagCharacterSnapshot`（APC 助战快照）也取 tree 0，  
> 但它的语义是"取这个角色的技能快照"，**暂时没动** —— 一次只改一个变量。

### 构建（第七轮）

三方同一 **30952960 字节 / 18:23 / md5 `be26da1c1138d0060b6b977c7ad900e5`**；  
备份 `bin/wireprobe-pvf.exe.bak-20261009-1826-pre-slotpage.exe`。


## ⛔ 仍然没做的

- 第二页技能的"初始页克隆"：86JP 解锁时会克隆初始技能页；115 的  
  `knownSkills(state, tree)` 本来就把 `initialSkills` 叠加到**任意**一棵树上，  
  所以第二页天然不是空的，**不需要**克隆。
