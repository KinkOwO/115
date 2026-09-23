# next54 — 外部《开槽任务修复说明》在本仓库可否复刻：对照与冲突分析

> 日期：2026-09-22 · 状态：**已落地批次 1+2+3（编译与测试通过，待实机验证）**
> 来源文档：`E:\下载\开槽任务修复说明-20260922.md`（其工程路径为 `F:\115-main`）
> 相关：`analysis/tasks/next50-odyssey-expanded-equip-slot.md`、`next51-equipslot-lock-state.md`

> 用户口径（2026-09-22）：外部说明修的是**剧情角色**的任务解锁链路，不是奥德赛角色；
> 其解锁值对奥德赛只作参考。因此本次以"**剧情角色能开槽**"为验收目标，奥德赛侧
> 只做可选的判据实验（见 §9.2）。

---

## 0. 一句话结论

**可以复刻，而且与本地已挂起的那条路线不冲突 —— 两者打的是同一条判据链的两端。**

四条要点：

1. **同源已确认**：该说明描述的就是本仓库这份代码。它引用的每一条任务配置判据，都在本地 `configs/quests.generated.json` 里逐值吻合（§2），连"NPC=-1 的另外 13 条"这种边角数字都对得上。
2. **一处已重叠**：`[slot expansion]` 奖励识别我们已在 `2bbdb43` 落地。**不要照搬文档的 `internal/quest/slot_reward.go`**，否则会出现两份并行实现。
3. **两处本地完全缺失**（这是复刻的主要收益）：
   - 解锁位**掩码累加落库** + **USERINFO1 投影**（文档 §5.3 后半 / §5.5）→ 直指"槽位锁着"这个原始症状；
   - subtype1 **对话豁免 + 替代 NPC 语义化**（文档 §5.1 / §5.2）。
4. **一处对我们不适用**：文档 §5.4 的 `SaveBag` 已知字段白名单。我们的 `SaveBag` 是**整体替换** `state["inventory"]`，不存在"合并旧值覆盖新值"的前提。

---

## 1. 同源证据（为什么可以按它复刻）

| 证据 | 说明 |
| --- | --- |
| **字节偏移 360** | 文档 §5.5 说"本次验证的 USERINFO1 payload 中，该字节偏移为 360"。本地 `internal/game/protocol/entry_addition.go:87` 的 `p = append(p, 0) // 14563d692` 正好在偏移 **360**：`1 + 2 + 2 + 250 + 2 + 8 + 4 + 91 = 360`。文档说的"原先该位置固定写 0"就是我们这一行。 |
| **协议地址** | 文档 §4 的 `14563d692` / `14563d6a8` 与本仓库该行的注释地址**完全同一处**。 |
| **CMD33 形态** | 文档 §4：`u16(33)、u16(任务ID)、u32(0)、u8(0)`，16 字节。本地 `cmd/wireprobe/quest_flow.go:119-126` 正是 `len(p) != 16` + `p[4:]` 全零校验 + `p[2:4]` 取任务号。 |
| **奖励语义** | 文档 §5.3 与本地 `internal/quest/index.go:52 slotExpansion()` 对 `[slot expansion]` + `[reward int data] 0..3` 的判定一致。 |
| **13 条 NPC=-1** | 文档 §5.1 说"NPC=-1 的另外 13 条任务还缺附加条件判定"。本地只读统计 `[meet npc]` + `[sub type] 1` 共 **53** 条，其中 objective NPC = -1 的**恰好 13 条**。 |

结论：这是一份**同仓库、同上游**（`gitgud.io/fuckworld/115`）的另一份工作副本，可以直接当作高可信的实现参考，但仍需按 §5 的本地差异做适配。

---

## 2. 配置逐值核对（只读，取自本地 `quests.generated.json`，共 2844 条）

| 任务 | kind | 等级 | `[sub type]` | `[int data]`（目标） | `[alternative npc index]` | `[reward type]` | `[reward int data]` |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 649 | `[meet npc]` | 60 | 1 | 28 | `28 → 100001447`（`[cleared quest] 13434`） | `[slot expansion]` | **0** |
| 650 | `[meet npc]` | 65 | 1 | 28 | 同上 | `[slot expansion]` | **1** |
| 2636 | `[meet npc]` | 90 | 1 | 28 | 同上 | `[slot expansion]` | **2** |

与文档 §2 的表格逐格一致。全目录统计：

- `[meet npc]` 任务 921 条，其中带 `[sub type] 1` 的 **53** 条（文档说"逐条推进 40 条已支持的 subtype1 目标"—— 差额属"其余条件未支持"的那部分）；
- 带 `[alternative npc index]` 的任务 **47** 条。

---

## 3. 逐条对照表

| 文档章节 | 本地状态 | 冲突 / 风险 | 建议 |
| --- | --- | --- | --- |
| §5.1 `AllowsRemoteNPCInteraction` | **缺失**（本地无 `meet_npc.go` 该函数、无 `[sub type]` 解析） | 无冲突，纯新增 | 复刻。注意放到 `internal/quest/meet_npc.go` 与本仓库 `cells()` 助手复用 |
| §5.2 `questInteraction` 豁免地图检查 | **缺失**。本地是硬检查 `if !w.service.HasNPC(w.state.Position, npc)` | 无冲突 | 复刻，但要连 `MeetNPC` 的 `npc` 校验一起改（见 §5） |
| §5.3 前半：`rewardUsable` 接受 `[slot expansion]` | **已有**（`2bbdb43`，`index.go:75-92`） | ⚠️ **重复实现风险** | **不照搬** `slot_reward.go`；把"索引→掩码"映射并入本地 `slotExpansion` 所在文件 |
| §5.3 后半：`bag.ExpandEquipFlags \|= unlock` + 同事务 `SaveBag` | **缺失**（全仓无 `ExpandEquipFlags`） | 无冲突。我们的 `Bag` 是 JSON、`SaveBag` 整体替换 inventory，实现更简单 | 复刻，落在 `finish.go` 的 `CommitQuestReward` 回调内 |
| §5.4 `SaveBag` 已知字段白名单加 `expand_equip_flags` | **不适用**。本地 `SaveBag` 直接 `fields["inventory"] = v`，不合并旧值 | 无 | **跳过**。只需在 `Bag` 结构体加 `json:"expand_equip_flags,omitempty"` |
| §5.5 `detail.go` 投影 + `entry_addition.go` 编码开槽字节 | **缺失**。`entry_addition.go:87` 现在是写死 `0` | 无冲突 | 复刻（偏移已核对为 360） |
| §5.5 领奖后重发 USERINFO0 / USERINFO1 | **缺失**。本地 `finishQuest` 只发 id 12/13/37/291/342/21 | 需注意帧序 | 复刻。复用现成能力：`w.characters.EntryBasicProbe` / `w.characters.EntryAddition`（二者都以 `kind,id = 0,2` 下发，见 `main.go:2311-2325`） |
| §4 判据 `145cf28f0`（槽 22/23/25 检查位 1/2/16） | 未核对 | ⚠️ **潜在冲突**，见 §4.3 | 复刻前先核对地址归属 |
| §5「NOTI328 不能替代」 | **已有实测结论**：attempt 1 闪退、attempt 2/3 无效，且 handler 尾部会把 `+0x697C` **恢复原值** | 无冲突 | 两边结论一致，互为印证 |

---

## 4. 三类冲突明细

### 4.1 必须避免的重复实现

文档把 `[slot expansion]` 判定放在新文件 `internal/quest/slot_reward.go`；本仓库已有 `index.go` 的 `slotExpansion()`（返回**槽索引** 0/1/2）并被 `rewardUsable`、`finish.go`、`slot_expansion_test.go` 三方引用。

→ 做法：**保留本地 `slotExpansion()` 的签名**，只把文档的"索引 → 位掩码"映射（`0→1`、`1→2`、`2→16`）作为新增函数（如 `slotUnlockMask`）加到同一文件。不要新建第二份 `[reward type]` 解析。

### 4.2 与已回退路线（next50 / next51）的关系：不冲突，是互补

| | 我们挂起的路线 | 文档路线 |
| --- | --- | --- |
| 入口 | NOTI328 / 扩展槽占位行（NOTI14） | 任务奖励落库 → **USERINFO1** |
| 落点 | 角色 `+0x697C` / 槽对象 `+0x358` | 角色 **`+0x198`** |
| 结论 | 已回退；`+0x358` 是构造期只读字段 | 未回退，离线验证通过 |

两条路线**没有代码交集**，且文档路线正好答上了 next50 §5 留下的那个缺口：

> next50 的缺口是"行记录里哪个字段映射成 `+0x358`，解锁值是多少"。
> 文档给出的链条是：**USERINFO1 偏移 360 的字节 → 角色 `+0x198` → `145cf28f0` 检查槽 22/23/25 的 1/2/16 位**。
> 也就是说，解锁信息由"构造角色/槽对象的那份数据"带入 —— **与 next50 的结论方向一致，只是找到了具体的那个字节**。

⚠️ 但**尚不能断言这就是画锁的那个判据**：next51 §4 独立定位到另一处（`0x142384990`，`[this+0x660]` 的 `std::map<int,byte>`）。两个候选需要**实机判定**才能收敛（见 §6 批次 2）。

### 4.3 需要先核对的一处地址归属

next51 §1 已确证 `0x145cdd020` / `0x145cf15b0` / `0x145cdcda0` / `0x145d20e80` 是**升级箭头（装备对比）查询**的包装，属 `[角色+0x697C]` 那条链；而文档 §4 的判据 `145cf28f0` **与 `145cf15b0` 地址相邻、同处一片**。

→ 复刻前必须先确认 `145cf28f0` 是否落在同一函数族。若它其实是"装备对比/升级箭头"的谓词，那么 §5.5 的 `+0x198` 链条就不是画锁判据，复刻后槽位仍不会亮（但也不会更糟）。

---

## 5. 本地行为差异：文档修的那个症状，我们这边可能不复现

文档 §5.2 的旧实现是"**先解析替代 NPC**（→ 100001447），**再**无条件做地图检查" → 城镇 40/区域 2 没有 100001447 → 拒绝。

**本仓库没有替代 NPC 解析**：`questInteraction` 取的是 `d.ObjectiveCells[0].Value`（= 28，原始目标），检查的是"本图有没有 NPC 28"。城镇 40/区域 2 **有** NPC 28 → 检查通过 → `MeetNPC(2636, 28)` 也通过（本地 `MeetNPC` 要求传入 npc 等于 objective 值）。

**推论**：文档里"站在艾丽丝旁边仍完不成"这个症状，在本地**大概率不复现**。因此：

- §5.1 / §5.2 在本地属于**语义正确化**（对齐客户端 §4 的原生判定：subtype1 显式对话请求不要求目标 NPC 在地图内），**不是修当前症状**；
- 但**应当一并复刻**：一旦将来补上替代 NPC 解析，缺了这条豁免就会立刻引入文档里那个回归（第 4 次踩同一个坑）；
- 复刻 §5.2 时**必须同时改 `MeetNPC`**：本地 `meet_npc.go:17` 的 `uint32(d.ObjectiveCells[0].Value) != npc` 会让替代 NPC（100001447）被直接拒绝，光豁免网关检查没用。

> 本节的"症状不复现"是**代码推理结论**，需实机确认（记为待验证项）。

---

## 6. 建议的复刻顺序（三批，每批独立可验证、可回滚）

### 批次 1 —— 奖励掩码累加落库（低风险、无协议改动）

- `internal/inventory/bag.go`：`Bag` 加 `ExpandEquipFlags byte \`json:"expand_equip_flags,omitempty"\``；加一个 `\|=` 累加方法。
- `internal/quest/`：`slotUnlockMask(slot)` 映射 `0→1 / 1→2 / 2→16`；`finish.go` 在事务回调用 `ReadBag` → `|=` → `SaveBag`，并在收据里加 `UnlockedEquipment`。
- **存档兼容**：`omitempty` + 缺字段读作 0 → 老存档无损；**字段名必须与文档一致（`expand_equip_flags`）**，否则两边构建互相读不出对方开的槽。
- 验证：单测（掩码 0→1→3→19，乱序完成不丢位）+ 隔离库回放。

### 批次 2 —— USERINFO1 投影 + 领奖后刷新（★ 实机判定点）

- `entry_addition.go:87` 的写死 `0` 改为 `byte(s.ExpandEquipFlags)`（编码前校验 ≤ 255）；`EntryAdditionProbe` 加同名字段；`character/detail.go` 从存档投影。
- `cmd/wireprobe/quest_flow.go`：收据带解锁位时补发 USERINFO0 + USERINFO1（`EntryBasicProbe` / `EntryAddition`，都以 `0,2` 下发）。
- **这一步就是判定实验**：用**奥德赛角色 `test-jh`** 登录看三格是否亮。
  - 亮 → 文档 §5.5 的 `+0x198` 链条成立，next50/next51 的挂起路线可以正式收口；
  - 不亮 → 说明画锁判据仍是 next51 §4 的 `[this+0x660]` 那个 map，回到那条线（此时本批次无害，可保留）。
- ⚠️ 帧序：外观帧（`actor_appearance_ready` / NOTI2827）必须仍是最后，新增帧不要插到它们后面。

### 批次 3 —— subtype1 豁免 + 替代 NPC 语义化

- `meet_npc.go` 加 `AllowsRemoteNPCInteraction`；`questInteraction` 按文档 §5.2 改为条件豁免；`MeetNPC` 改为走解析后的目标 NPC。
- 边界：**只放行"正数 NPC + `[sub type] 1` + 单目标 + `[meet npc]`"**；NPC=-1 的 13 条**不放行**（缺附加条件判定）。
- 负向验证：普通对话任务仍拒绝错误地图；被动走近 NPC 的 `ProximityProgress` 不得因此跨图完成。

---

## 7. 风险清单

| # | 风险 | 处理 |
| --- | --- | --- |
| R1 | **索引 ≠ 掩码**。`slotExpansion` 返回 0/1/2；直接 `\|= byte(slot)` 会得到 1/2/**4**，第三个槽（耳环应为 **16**）错 | 必须走显式映射 `0→1 / 1→2 / 2→16`，并加单测钉死 |
| R2 | 赋值覆盖而非按位或 → 后完成的任务清掉先开的槽 | 强制 `\|=`，并测乱序完成 |
| R3 | `byte` 上限 255，越界截断 | 编码前校验 |
| R4 | JSON 字段名与对方不一致 → 两套构建互相读不出已开的槽 | 统一用 `expand_equip_flags` + `omitempty` |
| R5 | **文档自身尚未收到实机成功确认**（其原文），验证是离线原生回放 + 隔离 schema | 我们复刻后必须自己实机验收，不继承其结论 |
| R6 | 新增 USERINFO 帧打乱进城帧序 | 只在领奖路径追加；外观帧保持最后 |
| R7 | `finishQuest` 现有 `answeredQuests` 去重与 `result.Applied` 分支 | 追加帧放在既有顺序之后，不改变去重语义 |
| R8 | 与 `F:\115-main` 副本长期分叉 | 见 D3 |

---

## 8. 待用户决策

- **D1 复刻范围**：只做批次 1+2（拿到实机判定即止）／一次做到批次 3？
- **D2 实机判定**：是否用**奥德赛角色 `test-jh`** 直接做批次 2 的实机实验？（成本最低，且能一次性判定 next50/next51 的悬案）
- **D3 副本关系**：`F:\115-main` 与本仓库是否要合并/对齐？（否则同一问题会在两边各修一遍，且存档字段一旦不一致就互相读不出）

> 本文为只读分析，未修改任何服务端代码、配置或存档。

---

## 9. 落地结果（as built，2026-09-22）

### 9.1 实际改了什么

| 文件 | 改动 |
| --- | --- |
| `internal/inventory/bag.go` | `Bag` 增加 `ExpandEquipFlags byte \| json:"expand_equip_flags,omitempty"`（按位或累加，绝不赋值） |
| `internal/inventory/expand_slots.go`（新） | `UnlockEquipSlots(raw, mask)`：`ReadBag → \|= → SaveBag`；mask 0 原样返回 |
| `internal/quest/index.go` | 新增 `ExpandSupport/ExpandMagicStone/ExpandEarring = 1/2/16` 与 `slotUnlockMask`（索引→位，**不沿用** `slotExpansion` 的索引语义） |
| `internal/quest/finish.go` | 事务内累加解锁位；收据新增 `unlocked_equipment` |
| `internal/game/protocol/entry_addition.go` | 偏移 360（原生 `14563d692`）由写死 `0` 改为 `s.ExpandEquipFlags` |
| `internal/character/detail.go` | 从原始 `state` 投影 `inventory.expand_equip_flags`（**不走** `inventory.ReadBag`，坏背包不得把登录变成拒绝） |
| `cmd/wireprobe/quest_flow.go` | ① 收据带解锁位时补发 USERINFO0+USERINFO1+穿戴行（`unlockRefresh`）② `questInteraction` 改用条件豁免 |
| `internal/quest/meet_npc.go` | 新增 `AllowsRemoteNPCInteraction`（`[sub type] 1` 判定） |

**刻意没做的**（对照 §3 的结论）：

- 没有新建 `internal/quest/slot_reward.go`（`slotExpansion` 已在 `index.go`，不造第二份）；
- 没有做 `SaveBag` 白名单（本仓库 `SaveBag` 整体替换 `inventory`，前提不存在）；
- **没有引入 `[alternative npc index]` 解析**。本仓库取原始目标 NPC 28，客户端对 subtype1 只按"任务自己解析出的目标"判身份后即发 CMD33，因此服务端按任务号推进即可；豁免让它在 town40/area2 和 town139 两种站位下都能通过。将来若要补替代解析，**必须保留 `AllowsRemoteNPCInteraction` 豁免**，否则会立刻复现外部说明里那个回归。

### 9.2 奥德赛侧：为什么"仅登录"看不出变化

奥德赛角色（`test-jh`）的存档里没有这些解锁位 —— 它没做过也没法做 649/650/2636。
所以本轮落地后，奥德赛角色登录**不会有任何变化**（表现与之前一致：三格仍锁）。

要验证"外部说明的解锁值对奥德赛是否有效"，**必须先让存档带上位**，这是测试数据而非迁移：

1. 先停服务端（避免缓存与写回覆盖）；
2. 备份该角色整行（导出 `state`）；
3. 只改这一个字段：把 `inventory.expand_equip_flags` 设为 `19`（1|2|16）；
4. 起服务端登录看三格：**亮 → 说明 `+0x198` 这条链就是画锁判据**，next50/next51 的悬案可收口；
   不亮 → 画锁判据仍是 next51 §4 的 `[this+0x660]` 那个 map，回到那条线（本批次无害，可保留）；
5. 无论结果如何都把这一个字段还原。

> 若希望把第 3 步做成可复用的能力（外部说明的 §5.5 与 next51 §4c 都指向"GM 解锁应做在服务端"），
> 建议在 `cmd/admin` 增一个带 `-grant-id`/`-reason` 审计语义的解锁子命令，而不是手工改库。

### 9.3 剧情角色验收路径（今晚）

1. 起服务端 → 进城镇；
2. 角色需满足等级（649 需 60、650 需 65、2636 需 90），接取对应开槽任务；
3. 完成对话目标 → 交任务；
4. 期望：对应格子（辅助装备 22 / 魔法石 23 / 耳环 25）解锁；**重登后仍然解锁**；
5. 三个任务**乱序**完成也应保留已开的槽（掩码是累加）。

失败时的分诊：任务接不到 → 看 `rewardUsable`；对话完不成 → 看 `quest NPC ... absent from current source area`；交任务后仍锁 → 看收据里有没有 `unlocked_equipment`，以及帧日志里有没有 `reward_actor_addition_sent`。

### 9.4 验证状态

- `go test -count=1 ./internal/... ./cmd/...` 全绿；`go vet ./internal/... ./cmd/...` **零输出**（此前 `detail.go` 的非法 UTF-8 已修复）。
- 新增 6 个测试文件，覆盖掩码累加/乱序/老存档/超宽值、偏移 360、存档→载荷投影、豁免边界与普通对话仍拒错图。
- **未做**：实机验证（含外部说明要求的重登恢复）。外部说明自身也尚未实机确认。
- 二进制：`bin/wireprobe-handoff-source.exe` 已重编，现役版本备份 `bin/wireprobe-handoff-source.exe.bak-20260922-1533`。
