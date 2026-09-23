# next74 — 末世录的真实门禁：**前置任务树** + 服务端缺失的目标模型

> 由实机诊断包 `diag-20260923-163532.zip` 触发。
> 本文更正 `next73` §3 的一处结论错误，并给出「为什么看不到末世录军团本」的**根因**。

## 1. 先说两个结论

1. **更正**：`next73` 说"任务 23099 没有前置任务（`prerequisites = null`）"是**错的**。
   那只读了 JSON 的结构化字段；**真源在脚本体里的 `[pre required quest]` cell**，
   而 23099 的前置是 **23055**，往上是一整棵树。
2. **服务端侧的真实阻塞**：这棵树的 **4 环服务端不提供**，因为它们的目标模型
   `[monster kill checkpoint]` **未实现**。客户端要求整条链完成才解锁入口
   ⇒ **末世录目前永远解锁不了**，且服务端日志里没有任何报错。

## 2. 前置任务树（从入口 23099 往回走）

```
22988 delezie_of_plague_05_1 ┐
22989 06_1                   │ 2025 狄瑞吉 / 瘟疫剧情
22990 07_1                   │ （22991..22996 为并行支线）
22997 08  ← 需要 22990/22993/22996
22998 09 → 22999 10 → 23000 11 → 23001 12 → 23027 13 → 23028 14
                             │
23053 grandconstellationturtlelibrary ┐ 2026 千海之空
23054 castleoftheapostate             │ ★ 服务端不提供
23055 labyrinthofparadox（悖论迷宫）  ┘ ★ 服务端不提供
23099 apocalypseantienbi（末世录入口，副本 100005220）  ★ 服务端不提供
```

遍历结果：**从 23099 可达 20 个任务，16 个已提供，4 个被卡**（`internal/quest` 的
新回归测试 `TestApocalypsePrerequisiteChainIsOffered` 会打印这份清单）。

被卡的 4 个：**23053 / 23054 / 23055 / 23099**。

## 3. 卡点是什么

`internal/quest/available.go` 只把「目标与奖励都能结算」的任务放进可接列表（NOTI21）：

```go
if !en.Implemented || !en.RewardUsable || !en.GrowUsable { continue }
```

`en.Implemented = (InitialProgress(d) 无错)`，而 `InitialProgress` 目前只认 5 种目标模型：

| 模型 | 来源 |
| --- | --- |
| `[meet npc]`（单 cell） | `progress.go:95` |
| `[clear map]`（单 cell） | `progress.go:98` |
| `[reach the range]` | `progress.go:101`（`ReachRange`） |
| `[seek]` + `[meet npc]` | `progress.go:104`（`SeekMeet`） |
| `[look cinematic]` | `progress.go:114` |

被卡 4 环的 `[type]` 都是 **`[monster kill checkpoint]`** —— **不在上表里**，
所以 `InitialProgress` 返回错误 ⇒ `Implemented=false` ⇒ 不进可接列表。

**这个模型在全量 2844 个任务里刚好只被这 4 个任务使用**（可验证：按 `kind` 统计
`[monster kill checkpoint]` 恰好 4 条，就是这 4 环）。

⇒ **实现这一个目标模型就能解锁整条 2026 千海之空 + 末世录主线。** 范围很小、收益很集中。

### 3.1 需要实现的结构（SOURCE 原文）

```text
[type]            [monster kill checkpoint]
[sub type]        -1
[int data]        109019626 1 8 109019627 3 5 109019628 1 3 109019629 2 1 109019630 -1 -1
[/int data]
```

即 `[int data]` 里是若干 **(键, 值, 值)** 三元组，键 ∈ `109019626..109019630`。
四个任务用到的组数与取值不同：

| 任务 | `[int data]` 三元组 |
| --- | --- |
| 23053 | `(109019626, 7, 3)` `(109019627, 7, 1)` `(109019630, -1, -1)` |
| 23054 | `(109019626, 1, 8)` `(109019627, 2, 5)` `(109019630, -1, -1)` |
| 23055 | `(109019626, 0, 9)` `(109019627, 3, 6)` `(109019628, 5, 2)` `(109019630, -1, -1)` |
| 23099 | `(109019626, 1, 8)` `(109019627, 3, 5)` `(109019628, 1, 3)` `(109019629, 2, 1)` `(109019630, -1, -1)` |

⚠️ **这 5 个键名还没解出来**（它们在源里是 `[int data]` 内的裸数字，不是 `[...]` 名字）。
在拿到键的语义之前**不要实现**：这是典型的"结构对、含义错"风险区。
下一步探针：在客户端里反查 `109019626` 这类立即数（它们应当是客户端自己的参数键 id，
可能在 dstr / 任务参数名表里），或从 `[monster kill checkpoint]` 的客户端消费点反读。

## 4. 名声是**第二条**独立门禁

`apocalypse.ctp` 的 `[recommend fame]`（98,171 / 105,881 / 105,881 / 73,993）不是凭空来的 ——
指南支线任务 **23128**（`contents/2026/apocalypse/quest/guide/side_apocalypse.qst`）里写着：

```text
[type]                [legion content clear with difficulty]
[go guide]            239                 ← 指南条目
[fame value]          73993               ← 与作战④的 [recommend fame] 完全一致
[fame error message]  430325271           ← 名声不足时的专用提示
[grade]               [side]
[level]               115
```

⇒ 名声门槛是**任务级的正式要求**（带专门的错误文案），不是"推荐值"装饰。
截图里 VENUS 的 `需要高级名声 41,929(17,804)` 就是这个字段的展示形态。

## 5. 本轮实测到的第二个真实 bug（**已修**）：末世录目录从未装载

诊断包的 `gateway.err`：

```
warning: load apocalypse catalog (configs/apocalypse.generated.json):
         open configs/apocalypse.generated.json: The system cannot find the path specified.
warning: no apocalypse catalog; legion operation confirmations are not validated
```

原因：网关的工作目录是**项目根**，所以所有配置都必须由启动器传**绝对路径** ——
`server/work/dfo_probe_tools/channel_probe.py` 里对 selection-boxes / item-shop /
random-option-catalog 都有这条注释，**只有我新加的 `-apocalypse-catalog` 漏了**，
用的是 flag 的相对默认值。

已修：`channel_probe.py` 补上绝对路径（且**不放在 `.exists()` 后面** ——
文件缺失时让服务端打出它尝试的绝对路径，这样才能区分"cwd 问题"与"文件缺失"）。

⇒ 就算入口门禁全过，这次实机也不会出现 `legion_run_planned`：
真源表没装载，`CMD2045` 只会记一条 `legion_catalog_missing`。

## 6. 诊断包的另一个信息：客户端**从未发出 CMD2043**

本次会话 247 条事件里，`client_frame` 的 id 集合是
`{1,2,4,8,31,35,67,120,143,171,283,390,407,433,469,495,585,637,707,782,848,1421,1422,1438,1499,1554,1557,1563,1593,2127,2178,2377,2423}` ——
**没有 2043/2354/2045/2355**，也没有任何 `legion_*` 事件。

⇒ 客户端在**发 2043 之前**就被挡下了，与 `next73` 的判定链（7 道门禁）一致。
结合本文的结题：**最可能是前置任务链没完成**（客户端自己知道整条链的状态）。

顺带：`available_quests_restored` 只有 `plain_bytes: 261` —— 可接任务列表非常短，
与"链条上的任务都没被提供"吻合。

## 7. 结论与下一步

> **2026-09-23 晚更新**：第 1 项已落地，见 §9。前置链现在 **20/20 全部提供**。

| 优先级 | 事项 | 性质 |
| --- | --- | --- |
| 1 | 解出 `109019626..109019630` 这 5 个键的语义，再实现 `[monster kill checkpoint]` | **服务端功能**，解锁整条链 |
| 2 | `channel_probe.py` 传绝对路径 | **已修**（`a9ed80f` 之后的提交） |
| 3 | 确认该角色的高级名声（门槛 73,993）与队伍状态 | 实机数据 |
| 4 | 回归测试 `TestApocalypsePrerequisiteChainIsOffered` | **已加**，链条断了会直接红 |

**纪律**：`[monster kill checkpoint]` 的 5 个键含义未定之前不要实现 ——
猜错会造出"能接但永远交不掉"的任务（`available.go` 的注释明确警告过这种后果）。

## 8. 复现

```bash
cd server/work/dfo-lan
# 前置树 + 服务端是否提供（失败时会打印每个卡点的目标结构）
go test -count=1 -run TestApocalypsePrerequisiteChainIsOffered -v ./internal/quest/
# 全量目标模型覆盖普查（哪些 kind 整个内容块都不可达）
go test -count=1 -run TestAuditObjectiveKinds -v ./internal/quest/
```

## 9. 实施：两种"客户端把关"的目标模型（2026-09-23 晚）

### 9.1 决定与理由

键语义仍未解出，但**不等于要干等**。`progress.go` 里已经有一条被明确记录的先例
（`[look cinematic]`，`progress.go:107`）：

> 该目标**没有服务端可校验的条件**，客户端在本地放完过场才让玩家提交；
> progress 0 = 服务端接受这次提交，且**只发任务自带奖励、不额外铸任何东西**。

`[monster kill checkpoint]` 与 `[legion content clear*]` 属于同一情形：
**检查点/通关是客户端侧的条件**（客户端在一次副本运行里记录检查点，达标后才允许提交），
而服务端目前既没有解码参数、也没有把军团运行追到"通关"这一步。

两条路的取舍：

| 做法 | 结果 |
| --- | --- |
| 不提供（现状） | 5 个任务静默不进可接列表 ⇒ 整条 2026 千海之空 + 末世录 **永久不可达，且日志无任何报错** |
| **按客户端把关（采纳）** | 任务可接可交，服务端只发任务自带奖励；**与 `[look cinematic]` 同一条已记录的降级口径** |
| 按语义实现 | 键未解 ⇒ 只能猜 ⇒ 可能造出"能接但永远交不掉"的任务，比上面两条都差 |

⇒ 采纳中间那条，并**按用户既定纪律把降级写进台账**（`next64` §6.2 **T8 / T9**）。

### 9.2 实现要点

| 位置 | 内容 |
| --- | --- |
| `internal/quest/progress.go` | `MonsterKillCheckpointShape` / `LegionContentClearShape`：**只校验形状**（cell 类型、三元组长度、`(-1,-1)` 终止符、键递增、内容号非负）。形状不符 ⇒ 保持"未实现"，不猜 |
| 同上 | 两个新模型常量 `monster-kill-checkpoint-client-gated-v1` / `legion-content-clear-client-gated-v1`，命名里带 `client-gated` 让降级一眼可见 |
| `internal/quest/apocalypse_chain_test.go` | `knownBlockedQuests` **清空**：链必须全提供；保留"未知断链直接 Fail"的机制 |
| `internal/quest/legion_objectives_test.go` | 形状接受/拒绝用例 + **23128（内容 6）必须被提供** + 全 catalog 该 kind 逐个校验 |
| `internal/quest/kind_audit_test.go` | 全量目标模型普查：按 `[type]` 分组打印 settleable / unsettleable，用来一眼看出"哪些内容块整体不可达" |

### 9.3 验证结果

| 项 | 结果 |
| --- | --- |
| 前置链 | **20 可达 / 20 offered / 0 blocked** |
| `[monster kill checkpoint]` | 4/4 quests 提供 |
| `[legion content clear]` | 5/5 提供（含内容 0） |
| `[legion content clear with difficulty]` | 1/1 提供（23128 = 末世录指南） |
| `[legion operation clear]` | **仍不提供**（仅 22544，2023 达斯岛证明任务，obj `[103,17,1]`，首数不在内容号体系内）—— 有意不做 |
| `go vet` / `go test` | 0 项 / 20 包含绿（`internal/character` 那条上游既有红测试除外） |

### 9.4 仍未闭环的部分（诚实清单）

- 键 `109019626..109019630` 与 `[legion content clear*]` 尾部数字**语义未解** ⇒ T8/T9 仍是降级。
- 名声 73,993 的门槛是**客户端在接取时把关**，服务端未校验（与既有其他任务一致）。
- 军团运行本身仍未闭环（X7 阶段推进、X9 军团信息、X11 奖励），
  所以"进本打一轮"这件事还没通；本轮解决的是**解锁链路**。
