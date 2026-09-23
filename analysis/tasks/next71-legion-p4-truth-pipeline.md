# next71 — P4 第一步：`.ctp` 真源流水线 + 军团侧纠错（WIP，待实机）

> 目标：让阶段计时/门禁/投币/奖励这些**玩法数值**从客户端编译表读出来，服务端不再写死；
> 同时修掉 P3 里一个由误判"上下行侧"造成的真实缺陷。
> 状态：**代码完成，`go build`/`go vet`/21 包测试全绿，待实机**。
> 台账：`next64` §6.2 **X2 关闭**，**X3 部分闭环**，新增 **X7/X8**。

---

## 1. 本轮做了什么（以及为什么先做这些）

`next64` 的 P4 原本写成「在 CMD37 完成后下发 NOTI1474 阶段时限 → 用 NOTI2657 推进阶段」。
本轮取证把这两条都证伪了一半：

| 原计划 | 取证结论 | 处置 |
| --- | --- | --- |
| 用 NOTI1474 下发阶段时限 | `sub_143895B00` 的两条分支是**超时提示文案**（子类 5/6 → 本地化串 91161/91160 → 转发 NOTI2875），不是时钟下发 | **改**：时限来自 `.ctp`，客户端自读同一张表 ⇒ 服务端无需下发。登记 **X8** |
| 用 NOTI2657 推进阶段 | 144 B 结构由**被调类**解释（`sub_142ABF650` → vtable+1136），结构未解出 | **暂不发**：先落地真源，把 144 B 登记为 **X7** 阻塞项 |

⇒ 本轮把 P4 中**可验证、零猜测**的部分全部落地：真源流水线 + 目录层 + 阶段策略 +
一个真实缺陷修复。推进/计时的驱动部分等 X7 闭环。

---

## 2. `.ctp` 真源流水线（新增）

```
client-build/Script.inner.pvf
        │  cmd/apocalypseimport（新的正确读器）
        ▼
configs/apocalypse.generated.json        ← 提交进仓库（54 KB）
        │  internal/catalog.LoadApocalypseCatalog
        ▼
*ApocalypseCatalog  ──►  internal/legion.ApocalypseClock / BuildRunPlan
```

| 文件 | 作用 |
| --- | --- |
| `internal/catalog/pvf/ctp.go` | **`.ctp` 权威读器**（Go）：头部 36 B、记录头 44 B、cell、ref、尾表、串池；**6 条硬断言** |
| `cmd/apocalypseimport/main.go` | 重写：从 PVF 直读两张表 → 生成配置；带形状校验 |
| `configs/apocalypse.generated.json` | 生成物：`phaseClock` / `operations` / `records` / `trailer` / `duties` |
| `internal/catalog/apocalypse.go` | 装载 + 校验（**钉死 sha256**：`apocalypse.ctp` `d560876f…`、`dungeonskillinfo.ctp` `4353c3fb…`） |
| `internal/legion/phase.go` | `ApocalypseClock`（阶段序/时长/合计/下一阶段）、`RunPlan`（一次作战的真源描述） |

**硬断言（读器内置，两文件均通过）**：版本 == 1；遍历条数 == 头部 `record_count`
（65 / 14）；记录区止 == 尾表起点；尾表止 == 池起点；每条记录名可解析且可打印；
`parent` 落在合法范围。任一不满足即**拒绝装载**，不猜。

删除了旧 `internal/catalog/pvf/table.go` —— 它按「12 字节定宽 cell」解码，是格式解出前的
临时实现，留着会误导后来人。

---

## 3. 三处更正（同一日内自我纠错）

| # | 早前口径 | 更正 |
| --- | --- | --- |
| 1 | `[role per member limit]` 的**值 = 2** | 它 **`cells` 为空**；那个 `2` 是 **`flags` 字段**。`flags` 语义仍未确认，不得当角色数用 |
| 2 | 阶段时钟「首项 0 是阶段 0 占位、阶段 1..6 = 90/300/…」 | 正确读法是 **6 组 `(阶段号, 秒)` 对**：`(0,90)(1,300)(2,300)(3,300)(4,600)(5,600)` ⇒ **阶段 0..5，合计 2190 s**。两种读法时长集合相同，故实现值不变，但文档与代码统一按「对」读 |
| 3 | 作战号映射 `0/1/2/4 ↔ 1/2/3/5`（转述思路文档） | **作废**。源表声明 4 个 `[operation data set]` 块，`[index]` = **`1 / 2 / 3 / 5`**（没有 4）。服务端直接按源表校验，不再猜映射（台账 **X3** 更新） |

> 第 3 条同时说明一件事：思路文档里的"值映射"是**它自己推的**，源表里根本没有这层映射。

---

## 4. 军团侧修复：上下行侧判定（P3 缺陷）

**问题**：P3 的 `legionSession.handle` 开头有一句「副本内一律拒绝军团包」。但取证显示：

```c
// sub_14069B4E0 —— CMD2355（职责变更）唯一的调用者
if ( (unsigned int)sub_1459A90F0(qword_14E66C090) != 3 || sub_145EFAFB0(...) == 0 )
    return 0;                 // ← 世界状态必须是 3（副本内）
v10 = *(_DWORD *)(a1 + 4*v9 + 211);   // 该成员槽位的当前职责
if ( a2 == v10 ) return 0;            // 变化才发
sub_14069E3B0(a2);                    // 发 CMD2355
```

⇒ **CMD2355 是副本内上行包**，原守卫会把玩家的职责变更全部拒掉。

**修法**：按包分流，而不是一刀切：

| 包 | 侧 | 依据 |
| --- | --- | --- |
| CMD2043 / CMD2354 / CMD2045 | **城镇** | 入口门禁 `0x1406afcc0` 要求 world state == 1；作战界面是城镇界面，2045 在进本前发出 |
| **CMD2355** | **副本内** | 上面那段调用者代码（state == 3） |

拒绝时都会写明确的 `legion_refused` 原因，所以万一判错也只表现为一条可读日志，不静默。

---

## 5. 新增的观测（实机用得上）

| 事件 | 时机 | 内容 |
| --- | --- | --- |
| `legion_run_planned` | CMD2045 确认作战时 | 作战号、块行号、**阶段序 + 各阶段秒数 + 合计**、`allow_coin` 是否配置及取值、`gate_schedule`、`gate_flow`、`reward_label` + 奖励元组、`recommend_fame`、`card_symbol_index`、`member_limit_class` |
| `legion_role_recorded` | CMD2355 | 玩家改后的职责值 |
| `legion_catalog_missing` | 未装配置时的 CMD2045 | 明确说明"未校验"，不假装通过 |
| 启动日志 | 服务端启动 | `loaded apocalypse table (65 records, 4 operations, 6 phases, 2190s total)` |

**为什么先观测再驱动**：源表是客户端自己也在读的东西。把服务端读到的真值打到日志里，
首次实机就能直接对照客户端行为 —— 客户端显示的倒计时、门禁时刻、可用作战是否与源表一致。
这比先写一个猜出来的阶段状态机更靠得住。

---

## 6. 验证

| 项 | 结果 |
| --- | --- |
| `gofmt -l`（本轮 8 个文件） | 干净 |
| `go vet ./internal/... ./cmd/...` | 0 |
| `go test -count=1 ./internal/... ./cmd/...` | **21 包全绿**（原 20 + `internal/legion`） |
| 新增测试 | `internal/catalog/apocalypse_test.go`（装载 + 10 种拒绝路径 + 提交物固定）、`internal/legion/phase_test.go`（时钟序/合计/Next、`RunPlan` 的"缺列即缺列"、未声明作战被拒） |
| 二进制重建 | `bin/wireprobe-handoff-source.exe`；串确认 `legion_run_planned` / `legion_catalog_missing` / `legion_role_recorded` / `apocalypse-catalog` 各命中 1 |
| 生成物一致性 | Go 读器算出的 `sha256` 与独立 Python 读器**一致**（`d560876f…`），记录数、4 个作战、`allowCoin=[-1 8]` 仅作战①全部对得上 |

**测试抓到的一个真实错误**：我在测试里把阶段合计写成 `2400`，实测 `2190`
（90+300+300+300+600+600）。这正是"断言必须算"的价值 —— 手算过的数字不能进文档。

---

## 7. 实机验收要点

1. 启动日志应出现 `loaded apocalypse table (65 records, 4 operations, 6 phases, 2190s total)`；
2. 进城镇 → 触发军团入口 → 选作战 → 确认后日志应出现 `legion_run_planned`，
   其中 `phase_seconds` 应为 `[90 300 300 300 600 600]`、`total_seconds: 2190`；
3. `operation` 字段是客户端实际发出的作战号 —— **这是确认 X3 的关键**（源表声明 `1/2/3/5`）；
4. 若出现 `legion_refused`，`reason` 会直说是哪一侧判错 / 哪个作战号未被声明；
5. 进本后改职责：应出现 `legion_role_recorded`（这一条在修复前必然被拒）。

---

## 8. 剩余取证（P4 的驱动部分）

| 项 | 内容 | 下一步探针 |
| --- | --- | --- |
| **X7** | `NOTI2657` 的 144 B 结构 | ① 定位向 `obj+312` map 的插入点（对象池注册处）取类 → 读 vtable slot 142（offset 1136）；② 或反向搜客户端**填充** 144 B 结构的写者（`0x90` 作为 memcpy/memset 尺寸） |
| **X8** | `NOTI1474` 的子类 5/6 各自文案与触发条件 | 已解到"是提示文案"；若要主动提示，需确认服务端该在什么时刻发、以及 handler 参数的取法（不走游标） |
| — | 阶段推进时点（谁在驱动：客户端自走还是服务端广播） | 单机场景下客户端很可能自行推进；首次实机看是否**不需要** 2657 也能过阶段。若确实不需要，X7 可降级为"仅多人同步用" |

---

## 9. 回滚

* 代码改动集中在：`internal/catalog/pvf/ctp.go`（新增，删 `table.go`）、
  `internal/catalog/apocalypse.go`（新增）、`internal/legion/phase.go`（新增）、
  `cmd/apocalypseimport/main.go`（重写）、`cmd/wireprobe/legion_flow.go`（守卫分流 + 目录层）、
  `cmd/wireprobe/main.go`（一个 flag + 一处装载 + 一处结果结构）。
* 数据库：**无改动**（符合根 `AGENTS.md` §4）。
* 回滚方式：`git revert` 本轮提交即可；`configs/apocalypse.generated.json` 是纯新增。
* 若只想临时关掉目录层：启动时不传 `-apocalypse-catalog`（或传空），
  军团族仍按 P1–P3 行为工作，只在确认时多一条 `legion_catalog_missing`。
