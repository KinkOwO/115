# MOD 四层接口——从 0 开始的设计与实现计划（交接给新会话）

> 状态：**设计阶段，未实现任何代码**。本轮（2026-10-04）只做取证、设计与提交交接。
> 业主口径（2026-10-04）：**mod 四层接口从 0 开始**；数据库**保持双引擎**（PostgreSQL + SQLite，
> 见 `database-dual-engine-plan.md`）。两份计划都交由新会话实施。
>
> ⚠️ 本文与根 `AGENTS.md` §0.1「服务端优先、未明确要求不得动客户端 DLL」的关系：
> 业主本轮**明确要求**设计并接入客户端 DLL 接口 ⇒ 对本任务解除该默认关闭，但**实施仍需业主逐层点名**
> （见 §5 的里程碑门禁），不得一次性全开。

## 0. 当前仓库事实（2026-10-04 取证，全部只读）

| 事实 | 证据 |
| --- | --- |
| **`client/` 目录不在本仓库** | `Test-Path client` → False。`DFO.exe`、`Script.pvf`、权威 IDB `DFO.exe.i64` 都不在 ⇒ **NPK 层与 DLL 层的实测取证在本仓做不了** |
| 仓库内 `*.npk` 文件数 = **0** | `dir /s /b *.npk` → File Not Found |
| 全仓（排除工具链）搜 `npk` 只命中 2 处无关文本 | `analysis/tasks/hell-party-owned-waves-20261003.md`、`server/reference/analysis-tools/decode_literals.py` |
| **`client-patchs/` 只有 7 个文件**，全是字体补丁脚本 | `client-patchs/fontsize/{add_bitmap_strikes,build_cjk_font,condense_width,patch_font_hinting,patch_font_scale}.py` + `apply-stock-fontsize.cmd` + `restore-fontsize.cmd` |
| 根 `AGENTS.md` §1.1 提到的 **ngstub / plugin loader 在仓库里不存在** | 全仓按名搜 `ngstub`、`plugin.?loader` → 0 命中 |
| 根 `AGENTS.md` §1 引用 `client-patchs/AGENTS.md`，但该文件不存在 | `Test-Path client-patchs/AGENTS.md` → False |
| PVF 侧只有**只读归档读取器**，没有可被 mod 调用的函数面 | `internal/catalog/pvf/archive.go`：`Files()` / `IterateFiles()` / `FileInfo()` / `FileCount()` / `ReadText` / `ReadBytes` / `Snapshot()` / `ReleaseReadCaches()` |
| 服务端只有**代码内分发链**，没有插件机制 | `cmd/wireprobe/client_dispatch.go:56` `dispatch` → `dispatchClientType` / `dispatchAccountQueries` / `dispatchCharacterSkills` …（20+ 命名阶段）+ `observedGameRequest` 白名单；无 registry / 无外部加载 |
| 仓库里唯一的脚本执行能力是服务端自用 | `internal/reward/lua.go`（gopher-lua v1.1.2，仅奖励脚本） |

⇒ **四层里只有 PVF 层有底座，另外三层（NPK、服务端 mod 接口、客户端 DLL）在本仓既无实现也无资源。**

## 1. 四层边界（先定契约，再写实现）

| 层 | 一句话职责 | 输入 | 输出 | 谁消费 |
| --- | --- | --- | --- | --- |
| **L1 PVF 函数接口** | 让 mod **声明并读写**内容：注册自定义脚本函数、声明新物品/任务/副本条目或覆盖已有字段 | PVF 文本脚本（`.etc`/`.cos`/`.equ`/`.qst`…） | 带来源哈希的领域规则、错误诊断 | 服务端 catalog/领域层、客户端（经由 L2 的资源） |
| **L2 NPK 资源（PVF 索引定位）** | 让 mod **挂载资源**：新贴图/动画/音效/UI 图素，用 PVF 条目定位归档内路径 | NPK/IMG 归档 + PVF 里的路径引用 | 客户端可读的资源；服务端侧只需"路径 → 是否存在/校验和" | 客户端渲染与加载器 |
| **L3 服务端函数接口** | 让 mod **挂服务端行为**：新 opcode/命令、既有命令的钩子（前后置）、定时/事件回调、自定义领域动作 | Go 侧注册项 + mod 声明的脚本 | 与官方路径同样的事务/存档/回包语义 | `cmd/wireprobe` 网关与 `internal/workflow` |
| **L4 客户端 DLL 接口** | 让 mod **在客户端内运行**：稳定的导出 ABI、回调点（封包收发/资源加载/UI 事件）、日志与卸载 | DLL + 清单文件 | 客户端行为（补丁、注入、UI） | 客户端进程（`DFO.exe`） |

**跨层不变式（照抄本仓既有铁律，mod 不得破坏）**：

1. **PVF 是内容唯一真源**（根 §0.2）：mod 只能通过 L1 的注册面声明内容，**不得**引入第二份 JSON/数据库内容表。
2. **服务端不改写 PVF**（ADR-003）：L1 的"覆盖"只能是内存视图 + 来源哈希，落盘由内容工具负责。
3. **存档兼容最高优先级**（根 §0.4）：L3 的每个 mod 动作都必须走既有事务/回执/幂等，且新字段对旧存档可缺省。
4. **DLL 日志必须写自身模块目录**（根 §0.9）：L4 的日志规则在 ABI 里就要体现（提供 `LogPath()`），不依赖进程 CWD。
5. **不猜包**（根 §0.3）：L3 新增 opcode 的字段布局必须有 IDA/实机证据；没有就只登记缺口。

## 2. 各层接口设计要点（待新会话按此定稿）

### L1 PVF 函数接口

- **注册面**（建议）：`mod.RegisterScriptFunc(name string, fn ScriptFunc) error`，其中 `ScriptFunc` 只接收/返回 PVF 原生类型（整数/浮点/字符串/数组/表），**不暴露 Go 结构体**，避免 mod 与内部实现耦合。
- **内容注册/覆盖**：`mod.DeclareContent(kind, path, fields)`；覆盖必须携带**来源哈希 + 优先级**，并在加载报告里列出被覆盖的条目（沿用本仓"显式失败、不静默换源"的纪律）。
- **冲突策略**：同 `(kind,path)` 多 mod ⇒ 默认**冲突即报错**（可配置为按优先级叠加），并在启动日志逐条列出。
- **缺口**：PVF 是否支持"函数"概念本身需要先取证（当前 reader 只解析数据，未见函数可调用面）⇒ **第一步是确认客户端/服务端对 PVF 脚本的求值模型**，否则 L1 只能做"数据注册"。

### L2 NPK 资源（PVF 索引定位）

- **定位链**：PVF 条目里引用资源（贴图/动画路径）→ 解析成 `(npk 相对路径, 索引项)` → 客户端加载器按该二元组取资源。**服务端只需实现路径解析与存在性校验**（可做离线审计：哪些 mod 引用缺失）。
- **挂载方式（三选一，需定稿）**：① 追加式 NPK（新文件，加载器按索引合并）；② 覆盖式（同路径替换，需哈希校验）；③ 内存注入（L4 拦截加载）。
- **缺口**：本仓**无客户端、无 NPK、无解析器** ⇒ 需要先取一份真实 NPK 与客户端加载器证据（或由业主指定另一棵树/目录）。

### L3 服务端函数接口

- **现状是显式 `if` 链**（`dispatchClientType` → …）⇒ mod 面必须在它**之前**插入一个可注册的钩子层，且顺序语义要明确（mod 优先 / 官方优先 / 只观察）。
- **建议形状**：`mod.Hook{Opcode uint16, Phase Pre|Post|Replace, Handler}`；`Replace` 必须显式声明并写进启动报告（否则与官方路径打架时无法定位）。
- **事务**：mod 的写操作只能拿到**本仓已有的领域能力**（`internal/workflow` 的 ItemService 等），不得直接拿 `*database.Store`（正好与双引擎计划的接口倒置对齐，见 `database-dual-engine-plan.md` §2）。
- **缺口**：没有依赖注入容器（AGENTS 明确"暂未引入依赖注入库"）⇒ 注册表用包级显式注册 + 启动期校验，不引入 DI 框架。

### L4 客户端 DLL 接口

- **ABI 最小面**（建议）：`version`、`init(host_api*)`、`shutdown()`、`LogPath()`；host 侧提供：封包收发钩子、资源加载钩子、UI 事件、定时器。
- **加载器**：本仓缺失（ngstub/plugin loader 都不在）⇒ 需先决定加载器归属（本仓 or 另一棵树），并遵守根 §0.9 的日志目录规则。
- **缺口**：无客户端可测；DLL 与客户端版本绑定关系（`DFO.exe` 哈希/版本）必须写进 ABI 前置校验。

## 3. 我建议的实施顺序（垂直切片优先）

不做"四层各做完再联调"，而是**先用一个最小 mod 打通全链路**，用它把四层的契约同时钉住：

| 阶段 | 交付 | 为什么这个顺序 |
| --- | --- | --- |
| **P0 取证** | ① 客户端与 PVF 的脚本求值模型；② 一份真实 NPK + 加载器行为；③ 加载器/DLL 源码归属；④ L3 钩子插入点的既有实证（哪条分发链、顺序约束） | 三层都缺实测证据（根 §0.3 不许猜） |
| **P1 垂直切片** | 一个样例 mod：**新增一个物品/NPC**（数据在 L1、图标在 L2、服务端行为在 L3、客户端显示靠 L2；暂不需要 L4） | 这是能用最少的层数验证 L1–L3 契约的最短路径 |
| **P2 L1 完整** | 注册面 + 覆盖 + 冲突报告 + 来源哈希；离线审计器（引用缺失/冲突清单） | 内容侧是所有其它层的前提 |
| **P3 L3 完整** | 钩子注册表（Pre/Post/Replace）+ 启动报告 + 权限与失败隔离（单个 mod 崩溃不拖垮网关） | 服务端优先是本仓默认路线，收益最直接 |
| **P4 L2 完整** | 索引定位 + 挂载/覆盖 + 校验；服务端离线审计复用 | 依赖客户端资源 |
| **P5 L4 ABI** | ABI 定稿 + 加载器 + 日志规则 + 版本门禁；先做只读钩子，再开放写 | 风险最高，放最后且逐钩子开 |
| **P6 收口** | 每个阶段独立 commit；实机验收由业主操作；`CHANGELOG` + confirmed baseline 更新 | 本仓纪律 |

## 4. 每个阶段的门禁（照本仓既有纪律）

1. 每个假设单独 commit（根 §0.5「一次假设、一次 commit」）。
2. 改协议/业务后跑 `go test ./...` + `go vet ./...`；涉及 mod 注册的必须补"注册冲突/失败隔离/旧存档缺字段"回归。
3. 新增 opcode 或字段布局：必须先有 IDA/实机证据（根 §0.3），否则只登记缺口。
4. 实机由业主操作（根 §0.6）；本会话/新会话不得无人值守起客户端。
5. 不覆盖业主工作区改动；`git status --short` 后只暂存本任务文件。

## 5. 需要业主在实施前拍板的 5 个问题

1. **加载器与 DLL 源码归属**：本仓新建 `client-patchs/` 完整实现，还是接入另一棵树？（本仓现有 `client-patchs` 只有字体脚本）
2. **`client/` 资源是否能回到本仓**（至少 `Script.pvf` + 一份 NPK）：L2 与 L4 的所有实测都依赖它。
3. **mod 的信任模型**：只允许业主自签名的 mod，还是允许第三方？这决定 L4 是否要做沙箱、L3 是否要权限位。
4. **L1 的覆盖语义**：只允许"追加"还是允许"覆盖既有条目"？覆盖会直接影响存档兼容与联机一致性。
5. **首个样例 mod 选什么**（建议：一个新物品 + 一个 NPC 对话），用来同时压测 L1/L2/L3 三层的契约。

## 6. 别踩的坑（本仓既有教训，直接适用）

- **不要 PowerShell 往返含中文的源码文件**（会用 ANSI 读坏编码）；查源码用 read/grep 工具。
- 网关的**失败分支不回 Error 包**（2259/2381 前车之鉴：两次都在收到 Error 后 `0xC0000005`）。
- `observedGameRequest` 白名单不登记的 opcode，第 `BodySampleLimit(8)` 帧之后 `verified` 不再计算 ⇒ 新命令"静默失效"且症状与"没实现"一模一样。
- 图鉴/存档类写操作必须走既有 `CommitCharacterEvent` / `CommitAccountVault` 事务与幂等键，不能自己开事务。
