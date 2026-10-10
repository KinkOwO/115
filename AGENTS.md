# AGENTS.md — DFO 115us 本地/局域网模拟服务端

> 本文件是 `DFO 115us` 项目的唯一 AGENTS 根真源；用户目录或其它副本仅作镜像，冲突时以本文件为准。
> 先读本文件，再按 §1 的触发规则读子目录 `AGENTS.md`；默认不读 `server/reference/` 与历史归档。
>
> 结构：§0 硬约束 → §0.1~§0.6 专题规范（C2S 计次 / PVF 规则 / **提交** / **目录** / **开发** / **存储：SQLite 唯一引擎**）
> → §1 项目地图 → §2 权威索引。任何 AI 在动手前必须读完 §0 与 §0.3~§0.6。

## 0. 硬约束速查（每条都必须遵守）

1. **服务端优先**：新功能、缺陷修复、协议补包、状态机调整默认只走 Go 服务端（`server/work/dfo-lan/`）。用户未明确要求当前任务使用客户端补丁/DLL 时，不得新增、修改、启用或扩展客户端 DLL/补丁；证据不足时暂停实现并记录缺口，不以 DLL 代替服务端闭环。详见 `client-patchs/AGENTS.md`。
2. **真源优先**：协议、字段、reader、分支、codec 以当前 115 级客户端 `client/DFO.exe` 及权威 IDB `client/DFO.exe.i64` 的 IDA 分析与实机动态命中为准；参考服源码（如 `86`、`90`、`ServerS4A21` 等）和历史文档只作线索，不是事实标准。
3. **禁止猜包**：字段、顺序、等待态或客户端消费路径未闭环时，停止服务端叠包试探，收集 IDA、服务端日志和用户手动 live 证据。
4. **数据库更新与存档兼容（强制）**：服务端**只有 SQLite 一个引擎**（单文件 `runtime/storage/dfolan.sqlite3`；PostgreSQL 支持已于 2026-10-05 整体移除，见 §0.6）。数据库/存档结构更新后，必须保证向前兼容或提供平滑迁移脚本，让已有玩家角色/物品存档无损升级。存档兼容是最高优先级；SQLite 库文件与 `runtime/storage/pgdata/`（历史 PostgreSQL 存档，只读留档，可救路径见 §0.6 第 3 条）同等对待——不得入库、不得随意删除。
5. **保护用户工作区**：不覆盖、不回滚、不提交用户的其它改动；提交前执行 `git status --short`，只暂存当前任务文件。严禁将 `server/work/dfo-lan/runtime/storage/` 下任何运行期文件（**`dfolan.sqlite3`、`-wal`/`-shm`、`local.json`、`local.<driver>.json`、管理租约**，以及历史 `pgdata/`）、大日志、临时生成物混入 Git。多 agent 并发时不要影响其他 agent 正在修改的文件，冲突且无法回避时暂停等用户确认。
6. **实机由用户操作**：不得无人值守启动客户端或代替玩家跑图；实机测试应尽量收集相关日志，避免重复测试浪费时间；准备完成后通知用户手动操作，再读取日志。
7. **用户确认后立即收口**：用户说"正常、修好、可以、冻结、暂时搁置"即视为收口信号，当轮更新 `CHANGELOG` 和 confirmed baseline，之后commit本次改动过的文件（不要commit其他agent正在改的文件）。
8. **异常先查资源与配置边界**：遇到莫名闪退、脚本查找 `-1`、资源键缺失或 UI 文本异常时，先排查 `client/Script.pvf`、`client/sk.dat`、导出的 JSON 配置与原版资源的差异；区分配置数据缺失与协议结构错误。
9. **DLL 日志硬规则**：若经用户明确要求编写或调试 DLL，每个 DLL 产生的日志、诊断文本默认必须解析自身模块路径，写入该 DLL 所在目录；不得依赖进程当前目录或将日志写入客户端游戏根目录。
10. **参考资料**：参考旧项目 `../90dof`、`../usdof`、`../ServerS4A21`、`../dfo115`，不能假设协议相同，必须实际分析。
11. **新 codec 先过身份门禁**：完整走完 IDA 逆向链和客户端原生向量验证，再命名算法或下结论。
12. **服务端技术栈门禁**：Go 1.26；存储**只有 SQLite**（单文件 `runtime/storage/dfolan.sqlite3` 及其 `-wal`/`-shm`，不需要起库服务；PostgreSQL 支持已于 2026-10-05 整体移除，见 §0.6）；日常启动由**仓库内 Go 启动器**编排（`bin/dfolauncher.exe`；2026-10-05 起已不用 Python）。
13. **一次只验证一个假设**：改动后必须通过测试与 vet；测试/候选/实机流程见 `server/AGENTS.md` §4。
14. **玩法规则由 PVF 脚本驱动**：等级动作、条件、奖励、数量、材料、费用、概率与内容关联，以当前 PVF 原生脚本为唯一内容定义；Go 负责解析、校验与执行，不再维护平行玩法表。新增或修改玩法前必须走 §0.2 的来源与重复规则检查。
15. **提交前必须过门禁并二次确认：任何 AI 在 `git add` / `git commit` 之前，必须先跑 `pwsh -NoProfile -File scripts/check-commit-hygiene.ps1`。脚本一旦报出**本地缓存/构建产物入库**、**不符合 §0.4/§0.5 规范**或**与当前环境不匹配（§0.3.4）**，AI 必须**立即停止提交**，把违规条目逐条报告给业主并**取得业主明确的二次确认**后才能继续；不得用 `-Force`、`--no-verify`、`git add -A` 或任何方式绕过。完整流程见 §0.3。
16. 如非必要禁止修改`AGENTS.md`，除非用户授权，必须保证`AGENTS.md`干净简洁，不需要写日期时间之类的无意义信息。

> 实现新功能时，先网络搜索参考端与当前版本的变化，再从 `pvf` 确认改动，之后从 `IDA` 确认逻辑。

### 0.1 C2S 三次上限

- 计次对象是同一 opcode 或同一用户功能。
- 换 codec 假设、reserved/body 组合、字段布局，或据参考服改一次运行路径，均算一次；每次必须有可回滚记录，并写 `attempt N/3`。
- 纯读日志、IDA、参考代码且未改运行路径，不计次。
- 第 3 次仍未闭环，停止猜测并回报证据缺口；不得以 DLL 猜测替代服务端闭环。
- 第 4 次继续盲目试包或增加无依据的硬编码，不算交付；后续需由用户明确指定新的取证范围。

### 0.2 PVF 脚本驱动与单一规则流程

本节是玩法规则归属与开发流程的唯一规范；子目录说明、迁移计划和交接文档引用本节，不另立一套判断标准。用户所说的「ETC 驱动」在本项目指 **PVF 原生脚本驱动**，包括 `.etc`、`.cos`、`.qst`、`.stk`、`.equ` 等及其原生索引和跨脚本引用；按当前资源实际归属读取。

**职责边界**：PVF 定义玩法条件、等级动作、奖励及数量、材料与费用、概率、内容清单和关联。Go 将同一只读 Source 的脚本解析为领域规则，负责条件判断、动作执行、协议适配、事务、存档、幂等和错误处理。领域服务与网关消费已解析规则，不复制源表，也不为单个任务、副本或物品重新写一套专用规则。协议 opcode、字段布局、动作枚举到执行函数的映射及存档位定义属于执行契约，仍需按客户端证据实现。

每次新增、修复或重构玩法，必须依次完成：

1. **定位内容定义**：先查当前 PVF 的原生索引、脚本字段和引用链；记录路径、标签、源身份及适用模式。不能以旧 JSON、参考服或历史 Go 常量代替当前源。字段语义与触发时机沿用根规则，须有当前客户端 IDA / 已确认 live 证据。
2. **检查重复规则**：同时检查 Go、JSON policy 和既有 reader/领域服务，列出源中已有但仍手工维护的 ID、等级、数量、费用、概率和分支。发现与 ETC / PVF 脚本重复的定义时，必须在当轮主动提示用户，给出源路径与标签、重复代码/配置位置、影响及建议；说明已迁移、暂保留的理由或证据缺口。不得只在代码注释或最终提交中静默记录。未收敛项同步写入现有依赖台账或迁移计划，后续复查时继续提示仍有关联的重复规则。优先扩展既有解析器和通用执行能力；仅改成「PVF 直读」而仍保留一套 Go 玩法表，不算该规则收敛完成。
3. **按源绑定与模式执行**：通过现有 `gamedata.Source` / `PrepareCatalogs` 解析同一归档，形成领域规则后交给执行器。普通模式与奥德赛等模式各自取源定义的条件与动作；可以共享动作执行函数，不能因最终奖励相同而合并触发条件。索引和跨脚本关系可从源发现时，不另建人工路径或 ID 清单。
4. **处理证据缺口**：源字段缺失、语义或客户端消费路径未闭环时，记录缺口并继续取证，禁止以新 Go 常量、专用 switch 或 JSON 内容回退补成第二真源。运维参数、经用户明确指定的调服策略及尚待取证的兼容布局须明确归属、理由、消费位置与迁移条件；源已有同义定义的规则不能自动归为服主 policy。新玩法偏离原生源须依据用户明确要求并单独记录差异。
5. **验证源驱动与兼容**：对本次规则验证字段解析、条件/奖励执行及模式边界；适用时用改变源字段的小型输入证明结果随源变化，防止测试只复述旧常量。重构保留已有存档、事务事件键、重复领奖防护和已确认协议行为；运行历史 JSON 不参与回退。代码改动执行相关测试、全量测试和 vet，实机仍由用户操作。
6. **提交可追溯结果**：变更说明写明「源脚本/字段 → reader → 领域规则 → 执行与存档/协议」链、移除的重复定义及仍未闭环项。区分规则迁移、文件删除和实机验收；历史测试输入、审计指纹和已确认 baseline 不得冒充新的运行内容定义。

已确认的审计例子：普通装备槽解锁来自 `.qst` 的 `[slot expansion]` 奖励；奥德赛等级动作来自 `aradodyssey.etc` 的 `[level action]`，创建奖励继续引用礼包 `.stk`。这些规则分别按源读取并复用槽位或发奖执行能力。2026-10-03 源码候选已迁移奥德赛槽位解锁与创建奖励引用链，移除对应副本 switch、奖励 ID/数量与护甲清单；等级动作的觉醒执行及其它待取证项仍须按台账推进。源码与真实归档测试不表示新的实机验收。

### 0.3 提交规范（强制，2026-10-04 业主定调）

#### 0.3.1 提交前门禁：警告 + 二次确认（硬规则）

任何 AI（以及人）在暂存/提交前，必须按顺序做：

1. **跑门禁脚本**（只读，不改工作区）：
   
   ```powershell
   # PowerShell 7（有 pwsh 时）
   pwsh -NoProfile -File scripts/check-commit-hygiene.ps1          # 检查已暂存内容（默认）
   pwsh -NoProfile -File scripts/check-commit-hygiene.ps1 -All     # 检查工作树全部改动（含未跟踪）
   # Windows PowerShell 5.1（无 pwsh 时的等价写法，已实测可用）
   powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\check-commit-hygiene.ps1 -All
   ```
   退出码：`0` 通过；`2` **需要二次确认**（发现缓存/产物、规范违规，或与环境不匹配）；`1` 脚本自身错误。
   > 注意：该脚本是 **UTF-8 带 BOM** 的 `.ps1`，5.1 才能正确解码中文；改它时不要去掉 BOM（§0.4.2）。
   > **实测坑（2026-10-05）**：`edit` 这类改写工具会**静默去掉 `.ps1` 的 BOM**，改完必须复跑门禁。
2. **零命中才可直接提交**。
3. **一旦命中（缓存/构建产物、违反 §0.4/§0.5，或 §0.3.4 环境不匹配）**：
   - **立即停止**本次暂存/提交，不得先提交再报告；
   - 把每条违规**逐条**报告业主：路径、类别、为什么算违规（引用具体条款）、影响、建议处置（移出暂存 / 加 .gitignore / 改放正确目录 / 单独存档）；
   - **取得业主明确的二次确认**后才可继续。判据：业主明确说出「确认」「按现状提交」「这条允许入库」一类表态。**沉默、未回复、含糊回应都不算确认**；
   - 业主授权放行的例外条目，必须在提交信息正文里写明「例外条目 + 授权人 + 授权原话摘要」。
4. **禁止绕过**：不得用 `-Force`、`--no-verify`、`git commit -a`、`git add -A`、手工拼 `git commit` 等方式跳过门禁；也不得为了让门禁通过而删除/移动业主的文件。
5. 门禁脚本只覆盖「能机械判定」的部分；§0.5 的 build/vet/test 与证据纪律仍需 AI 自查并在回复中报告实际结果。

#### 0.3.2 暂存与提交纪律

- 提交前先 `git status --short`；**只暂存本任务文件**，绝不 `git add -A`。
- 多写者并发时，不暂存、不提交他人正在修改的文件（若无法回避，先暂停并请业主裁决）。
- 一次假设一次提交；不把无关改动、格式化噪音、行尾转换混进同一提交。
- 提交信息用中文，至少写清：**改了什么 → 依据/证据 → 验证结果 → 未闭环项与风险**；引用相关文档路径或 `runtime/roles_*/` 会话目录。
- 收口提交（§0 第 7 条）才更新 `CHANGELOG` 与 confirmed baseline；未取得业主收口信号前，不得把候选写成"已确认"。

#### 0.3.3 二进制与大文件

- 新增单个 blob **> 5 MB** 前必须先问业主（是否需要、是否走 LFS、是否只该留在本机）。
- `server/work/dfo-lan/bin/*.exe`（候选/归档程序）默认**不入库**，仅在业主明确要求时提交；`bin/wireprobe-dungeon39.exe` 是已入库基线，**严禁覆盖**。
- 事实订正（2026-10-04 核实）：本仓库**没有 `.gitattributes`**，虽然本机 git 配置里存在 `filter.lfs.*`，但没有任何路径被 LFS 接管；`GIT-MANAGEMENT.md` **并不存在**（旧索引条目已删）。新增二进制按普通 blob 处理，超过阈值时按上一条征询业主。

#### 0.3.4 环境匹配（强制，2026-10-05 业主定调）

门禁的第三类校验：**提交内容必须与本机正在跑的这一套环境对得上**。只报「事实对不上」，不猜意图；命中即按 §0.3.1 处理（停止提交 → 逐条报告 → 明确二次确认，或先把环境/配置改成一致再提交）。

门禁的 `[环境不匹配]` 段实际校验：

| 项 | 判据 | 本机 2026-10-05 实测命中 |
| --- | --- | --- |
| 存储档 | 活动 `runtime/storage/local.json` 的路径必须在本机存在；与已跟踪 `local.example.json` 的 `driver` 不一致时只提示 | 2026-10-05 起**示例档与活动档都是 `driver=sqlite`**（§0.6）；PG 移除后 `local.postgres.json` 已不参与任何路线，档里写了 `driver=postgres` 会被服务端明确拒绝 |
| 配置里的路径 | 示例/本地配置中写的相对路径必须在本机存在 | 2026-10-05 起 PG 已移除，示例档不再有 `postgres_bin`；该行只在确有相对路径配置时命中 |
| profile 程序 | `configs/pvf-default.json.binary`、`server/launcher.local.json.server_binary` 指向的程序必须存在 | 缺 `bin/wireprobe-pvf.exe` 时启动找不到程序 |
| 启动链配置 | 启动链（Go：`internal/launcher/gateway.go`）引用的 `configs/channel.local*.json` 必须存在 | next37 档引用的频道档没落地 → 该档启动失败 |
| 脚本引用 | 改动过的 `.cmd`/`.ps1` 不得引用「仓库内 `tools\`」（本机 tools 在仓库外，该分支不可解析） | 新脚本写 `..\..\gm-tool\python\python.exe`（Python 是 GM 工具专属依赖，已移出 `tools\`，放在整合包外的 `gm-tool\python`；2026-10-05 起启动链已无 Python 分支，不需要它） |
| 脚本编码 | `.ps1` 必须 UTF-8 **带 BOM**；`.cmd` 必须 CRLF | 用会丢 BOM 的编辑器改门禁脚本 → PS 5.1 按 GBK 解码，脚本直接语法崩（当日实际踩到） |

环境不匹配的两条出路：**改环境**（补文件/改配置，使两边一致）或**改提交**（不入库/换档位）。两条都要在提交信息里说明。

> **「已跟踪」豁免（2026-10-05 补）**：门禁的缓存/产物规则只对**新增**文件判违规。若路径**已被 git 跟踪**
> （例如历史遗留的 `server/work/dfo-lan/runtime/storage/local.example.json`、
> `runtime/storage/pgdata/postgresql.conf`（PG 时代留下的已跟踪文件）），修改它属于正常编辑，门禁只输出 `[提示]` 并建议
> `git rm --cached` + 补 `.gitignore`，**不阻断提交**——因为此时门禁已无法阻止它入库，真正要做的是清理跟踪关系。
> 判定用 `git -c core.quotePath=false ls-files`（不能用 `-z`：NUL 分隔在 PowerShell 里会粘成一个字符串）。

### 0.4 代码目录规范

#### 0.4.1 落位铁规

| 内容 | 必须放在 | 禁止 |
| --- | --- | --- |
| 运行/维护脚本（`.cmd`、`.ps1`） | `scripts/` | **根目录不得新增 `.cmd`**（2026-10-04 已把 13 个根入口收进 `scripts/`） |
| Go 服务端源码 | `server/work/dfo-lan/`（入口 `cmd/{wireprobe,admin,gmtool,dfo-tool}`，工具实现 `internal/toolcmd/<name>`） | 新增 `cmd/<工具名>` 目录；把工具塞进 `wireprobe` |
| 服务端文档 | `server/work/dfo-lan/docs/`（协议/取证类进 `docs/protocol/`） | 把交接/协议记录写在仓库根或 `server/` 根 |
| 计划/台账/迁移清单 | `docs/todo/`（PVF 类进 `docs/todo/pvf/`） | 与交付文档混放 |
| 逆向分析产物 | `analysis/tasks/`、`analysis/dumps/`（权威 Dump） | 往 `server/` 或根目录丢分析中间件 |
| 导出的全量 JSON | `server/work/dfo-lan/configs/`（**仅历史基线/策略**） | 新增「导出 JSON → 运行期读取」链路（见 §0 铁律 1–3） |
| 客户端补丁/DLL | `client-patchs/`（默认不启用） | 未经业主明确要求新增/启用 DLL |
| 临时/生成物 | 未跟踪目录：`.tmp/`、`server/work/dfo-lan/runtime/`、`runtime/pvf-cache/`、`__pycache__/` | 任何情况下入库 |

#### 0.4.2 脚本编写规范

- `.cmd`/`.ps1` 一律先 `cd /d "%~dp0.."`（或等价）**回到仓库根**，再写根相对路径；`%~dp0` 只用于脚本自身所在目录里的文件。
- 移动脚本后必须核对：所有 `%~dp0` 派生路径与 cwd 相对路径分别按新层级重算，改完要逐个落盘验证目标存在（可参考 `scripts/check-commit-hygiene.ps1` 的检查思路）。
- **编码与行尾（照做否则脚本跑不起来）**：
  - `.ps1`：**UTF-8 带 BOM** + CRLF。Windows PowerShell 5.1 对无 BOM 的 `.ps1` 按 ANSI(GBK) 解码，中文注释会直接让脚本语法报错（`scripts/check-commit-hygiene.ps1` 就是带 BOM 的，改它时不要去掉 BOM）。
    **中文只放在 `.ps1` 里**：PowerShell 按 BOM 正确解码中文，`& "<中文名>.cmd"` 也按 UTF-16 正确处理。
  - `.cmd`：UTF-8 **不带 BOM** + CRLF + 首部 `chcp 65001`（BOM 会让 `@echo off` 那行异常）。
    **整份文件保持纯 ASCII**（含 `rem` 行；中文提示交给带 BOM 的 `.ps1` 或 `.py` 打印）。
    2026-10-05 两次实机踩到：`chcp 65001` 只对**其后被重新读取的行**生效，而裸 LF 会让 cmd 按行解析错位、
    把相邻行粘成一条——UTF-8 中文一旦出现在 `.cmd` 里，cmd 会把乱码片段当命令执行，实测
    `'hell' is not recognized ...`（`powershell` 被吃掉前 6 个字符）、`'�在' is not recognized ...`、
    `'强制时才用' is not recognized ...`（**来自 `rem` 行**，所以判据不能只查执行行）。
    修法与实测：把 `scripts/` 下入口全部改成纯 ASCII 后，CRLF / 裸 LF / 有无 BOM 四种组合全部正常。
    门禁已机械校验：`.cmd` 带 BOM、出现裸 LF、或含任何非 ASCII 字节，都报 `[环境不匹配]`。
  - `.md`/`.go`/`.json` 等：UTF-8 不带 BOM。
- 脚本要幂等、可重入，并在输出里明确「做了什么 / 下一步」（如 `scripts/停止游戏环境.cmd`）。
- 只读检查脚本必须先自证「不修改工作区」，并在帮助注释里写清退出码含义（参照 `scripts/check-commit-hygiene.ps1`）。

#### 0.4.3 命名与落位检查

- 新增顶层目录/顶层文件前必须问业主；根目录只保留：`AGENTS.md`、`CHANGELOG`、`README`/`开发对接文档.md`、`MERGE-RECORD-*.md`、`.gitignore`、`.gitattributes`(如新增)、业主的发布用 exe。
- 上述落位由 `scripts/check-commit-hygiene.ps1` 机械校验；违反即触发 §0.3.1 的警告 + 二次确认。

### 0.5 开发规范（强制）

1. **门禁必过（先编译，再提交）**：改动 Go 代码后执行 `go build ./...`、`go vet ./...`、`go test ./... -count=1`；全量测试的失败集合必须与既有基线**逐名比对**（新增失败即阻断）。改动协议/领域逻辑必须补对应测试。
   - **提交前必须真实编译通过**：只跑单测不算数；`go build ./...` 必须退出码 0（有编译产物交付时，产物要能真的构建出来并在提交信息里给出产物路径/大小/自述版本）。
   - 编译环境：`GOPROXY=https://goproxy.cn,direct`、`GOPATH=C:\Game\dof\115us\tools\gopath`、`GOCACHE=...\gocache`、`GOTOOLCHAIN=local`；**不得**因为"本机编不过"就跳过或改成只跑子包。
2. **一次一个假设 + 可回滚**：每次实验单独提交；候选程序与默认程序分离（`bin/wireprobe-handoff-source.exe` vs `bin/wireprobe-pvf.exe`），需要回退时按文档的备份路径操作。
3. **候选隔离**：源码编译只输出 `bin/wireprobe-handoff-source.exe`；**严禁直接覆盖** `wireprobe-dungeon39.exe` 与已确认默认程序；发布默认程序只在业主实机验收通过后按 `server/Build-Server.ps1 -UpdatePVFDefault` 执行。
4. **实机由业主操作**：准备就绪后通知业主操作，AI 只读日志；不得无人值守启动客户端或代跑。
5. **证据纪律**：不猜包、不用 DLL 兜协议、不把日志采样当结论；结论与「未闭环」分开写；引用具体日志行/文件/会话目录。
6. **内容真源**：玩法一律按 §0.2 走 PVF 直读；不新增平行玩法表、专用 switch 或 JSON 回退。
7. **存档与 schema 兼容**：数据库/存档结构变更必须提供向前兼容或迁移脚本，并在回复里说明兼容范围。
8. **回复必报**：本轮实际执行的命令与结果（build/vet/test 的真实输出结论）、改了哪些文件、未做与未闭环项、以及是否需要业主介入。
9. **提交前自检清单**（与 §0.3.1 一起用）：
   - `git status --short` 里只有本任务文件；
   - 门禁脚本通过（或已取得二次确认）；
   - build / vet / test 结论已实际取得；
   - 文档与索引同步（改了入口/目录/规范就要同步 AGENTS、README、开发对接文档）；
   - 没有把 `runtime/`、`pvf-cache/`、`.tmp/`、日志、`pgdata/`、SQLite 库文件带入暂存区。
10. **提交前必须先合并远端：本仓库有并发写入者（多会话/多代理/业主本人），**不允许在过期基线上提交**。顺序固定为：
    
    1. `git fetch origin`、`git fetch fork`（fork = `RicardoLz/115`，`origin` 只读）；
    2. 若落后就**合并**：`git merge <远端分支>`（**禁止** `rebase`、**禁止** `push --force`）—— 冲突必须**保留双方意图**逐处解决，不许整份覆盖对方；
    3. 合并后**重新执行** `go build ./...`、`go vet ./...`、`go test ./... -count=1`（合并可能引入编译/测试破坏），通过后才提交；
    4. 推送前再 `fetch` 一次确认没被别人抢先；被抢先就回到第 2 步。
    - 提交信息里要能看出"已合并远端"：合并提交用默认 `Merge ...` 文案即可；普通提交若在合并后重跑过门禁，请在正文注明"已合并 <远端>/<分支> 并重跑门禁"。

### 0.6 存储：SQLite 唯一引擎（强制，2026-10-05 业主定调；同日移除 PostgreSQL）

> 业主口径：**只留 sqlite**。SQLite 是唯一引擎（单文件、不需要起库服务）。PostgreSQL 16.4 支持已于
> 2026-10-05 **整体移除**：引擎/连接池/DSN 判定、`pgx` 依赖、`internal/database/sql/postgres/**`、
> 启动器的 `initdb`/`pg_ctl`/`createdb` 与 25438 端口探测、PG 启动脚本与路线档、
> `tools/tools-pg-bin.zip`（含 `tools/manifest.json` 的 `pg-bin` 包）全部删除；`sqlc.yaml` 只保留 sqlite
> 条目，`internal/database/sqlcgen` 保留为内部查询/类型层但**不再生成**。服务端与启动器两侧的兜底统一到
> SQLite（`internal/database.EngineForConfig` 的兜底、`internal/launcher.StorageConfig.DriverName`）。

1. **真源**：`server/work/dfo-lan/docs/sqlite-operations.md`（库文件位置、管理租约、迁移步骤、已知边界，以及
   **历史 pgdata 的可救路径**）。配置是 `server/work/dfo-lan/runtime/storage/local.json`（活动档）；
   `local.sqlite.json` 是同一档位的备份名。档里遗留的 `postgres_dsn`/`postgres_bin`/`postgres_data` 键
   **已被忽略**（不报错），但不再有任何代码读它们。
2. **存档只有一个家**：`runtime/storage/dfolan.sqlite3`（连带 `-wal`/`-shm`）＋管理租约文件。
   `runtime/storage/pgdata/`（66.1 MB 历史 PostgreSQL 存档）与 `pgdata.stale-*` 现在是**只读留档**：
   **绝不删除、绝不入库**（与 §0 铁律 4/5 一致），读它要走第 3 条。
3. **历史 pgdata 的可救路径（唯一一条）**：PG 引擎已不在当前源码里，所以救那批存档要
   ① 用 `git worktree add` 或 `git checkout <移除 PostgreSQL 的那个提交之前>` 拿到还带
   `dfo-tool sqliteconvert` 的源码；② 在那里 `go build ./cmd/dfo-tool`，用 `sqliteconvert` 把 pgdata
   **单向**搬到一个**独立的** SQLite 文件（不要直接覆盖 `dfolan.sqlite3`）；③ 再用当前版本打开那个文件。
   注意本机**已经没有 PG 二进制**（`../tools/pg` 与仓库内 `tools/pg` 都不存在），所以第 ② 步还要自己准备
   PostgreSQL 16.4 便携版。删掉 `pgdata` 这条路就彻底断了 —— 这就是它必须留在磁盘上的唯一理由。
4. **引擎判定只剩一种结局**：显式 `driver=sqlite`（或什么都不写）→ SQLite；写了 `driver=postgres` →
   **明确报错**（"PostgreSQL support was removed…"），**不会**被静默降级成 SQLite —— 那会把服务端指向
   另一个库，玩家看到的是「存档像丢了 / 登录失败」。改这条规则要同时改
   `internal/database.EngineForConfig`、`internal/launcher` 的镜像与两侧同一张表的用例。
5. **存档兼容的义务仍在**（§0 铁律 4）：SQLite 的 schema 由 `sql/sqlite/migrations/0001_initial.sql`
   一次性建全（`Migrate*` 在 SQLite 上是诚实的空操作）。新增结构必须给向前兼容或迁移说明并写明范围。
6. **排障顺序**：先看服务端启动日志里**真正打开的库**（`storage: engine=sqlite target=<文件>`），
   再看活动档 `driver` 与 `sqlite_path` 是否一致，最后才怀疑存档本身。SQLite 档的常见坑（管理租约、库文件路径、
   WAL）见操作手册 §4；`scripts\storage-route.cmd show` / `clear-guard` 是对应的只读/自愈入口。
7. **`--source-build` 与存储无关**：候选程序（`bin/wireprobe-handoff-source.exe`）走同一条 SQLite 路线，
   实机入口取 `scripts\启动游戏-SQLite.cmd --source-build`。

## 1. 项目地图与子索引触发规则

| 路径               | 子索引                    | 何时读                                                 |
| ------------------ | ------------------------- | ------------------------------------------------------ |
| `server/**`        | `server/AGENTS.md`        | 修改 Go 服务端代码、协议、数据库、存储、配置与领域逻辑 |
| `analysis/**`      | `analysis/AGENTS.md`      | 使用 IDA / 逆向分析 / 协议探测分析 / 查阅分析 Dump 资产 |
| `client-patchs/**` | `client-patchs/AGENTS.md` | 用户明确要求 DLL / 客户端补丁时（默认不读）            |
| `client/**`        | 沿用根规则                | 涉及 115 级客户端运行资源与权威 IDB                   |
| `gm-tool/**`       | `gm-tool/README.md`       | 使用或修改 GM 管理工具与 Web 界面                      |
| `scripts/**`       | 沿用根规则 §0.3/§0.4      | 新增、移动或修改运行/维护脚本                          |
| `docs/**`          | 沿用根规则                | 计划、台账、迁移清单                                   |
| `tools/**`（**现在仓库外** `../tools/`） | 沿用根规则 | 便携环境（Python 3.11、Go；PostgreSQL 16.4 已于 2026-10-05 随引擎移除） |

- 子目录没有 `AGENTS.md` 时，沿用根规则；**子索引不得与 §0.3~§0.6 冲突**，冲突以根文件为准。
- 读取顺序：本文件 → 相关目录 `README` → `开发对接文档.md`。
- 权威 IDB `client/DFO.exe.i64` 位于 `client/` 目录；不得随意覆盖或并发损坏。

### 1.1 根目录职责

| 路径                                                   | 职责                                                         |
| ------------------------------------------------------ | ------------------------------------------------------------ |
| `client/`                                              | 完整冻结客户端运行目录、运行时 `Script.pvf`、`sk.dat` 与权威 IDB `DFO.exe.i64` |
| `server/`                                              | 服务端总目录：Go 服务端源码（`work/dfo-lan/`）、启动编排与探针（`work/dfo_probe_tools/`）、配置与参考资料 |
| `scripts/`                                             | **全部运行/维护脚本**：`启动游戏.cmd`、`启动服务端.cmd`、`启动游戏-奥德赛.cmd`、`停止游戏环境.cmd`、`检查环境.cmd`、`配置环境.cmd`、`移除tools.cmd`/`还原tools.cmd`、`一键打包.cmd`、`打包增量更新.cmd`、`伊斯-*.cmd`，以及 Python/PS 工具与 `check-commit-hygiene.ps1` |
| `gm-tool/`                                             | **旧独立 GM 工具（命令行 `admin.exe` + 网页管理服务 `gmweb.exe`）已于 2026-10-05 移除**（二进制不在包内、网页版的 Python 代理也随之删掉）；只留数据与历史文档（`configs/`、各说明 `.md`、`dashboard/tests/pe_analyze.py`）。GM 现在只有**启动器内嵌**的那一个（启动器仓库 `gm/` → `gmbridge.exe`，Go，两条存储档通用） |
| `analysis/`                                            | 逆向分析工作区、分析脚本与核心 Dump 资产（`analysis/dumps/`） |
| `client-patchs/`                                       | 客户端补丁与 DLL 源码（ngstub, plugin loader 等，默认不开启） |
| `docs/`                                                | 计划、台账与迁移清单（`docs/todo/`）                         |
| `server/开发对接文档.md`                               | 核心交付基线：已实现功能边界、未完成系统与验收路线           |
| `tools/`（**仓库外** `../tools/`）                     | 便携环境：Python 3.11.9、Go 1.26（2026-10-04 已移出仓库；PostgreSQL 16.4 已于 2026-10-05 随引擎移除） |

## 2. 权威索引

| 文件                                                         | 用途                                                         |
| ------------------------------------------------------------ | ------------------------------------------------------------ |
| `AGENTS.md` §0.3 / §0.4 / §0.5                               | **提交规范 / 代码目录规范 / 开发规范**（唯一真源）           |
| `scripts/check-commit-hygiene.ps1`                           | 提交前门禁：缓存产物与规范违规检测（配合 §0.3.1 的警告+二次确认） |
| `开发对接文档.md`                                            | 核心交接基线：已实现功能边界、未完成系统与验收路线           |
| `server/README-先看这里.md` + `server/work/dfo-lan/docs/sqlite-operations.md` | 运行环境、启动流程、双库路线与故障排查说明                   |
| `server/README-先看这里.md`                                  | 服务端源码、双版本可执行文件与启动脚本交接说明               |
| `server/work/dfo-lan/docs/architecture.md`                   | 局域网多人架构设计与 ADR 决策记录                            |
| `server/work/dfo-lan/docs/protocol/`                         | 历史协议取证与分析记录（next27 ~ next38-equipment-display）  |
| `server/work/dfo-lan/bin/wireprobe-dungeon39.exe`            | 39 版归档基准服务程序（已实机验证进城与装备显示），严禁覆盖  |
| `server/work/dfo-lan/bin/wireprobe-handoff-source.exe`       | 源码候选版服务程序（代码测试通过，待实机回归）               |
| `client/DFO.exe.i64`                                         | 唯一权威 115 级客户端 IDB，禁止直接打开                      |
| `server/work/dfo-lan/configs/`                               | 游戏全量导出配置（装备、任务、地图、技能等规则驱动数据；**仅历史基线**） |
| `analysis/dumps/`                                            | 权威逆向 Dump 资产库：CMD/NOTI Opcode表、XORSTR地址表、DSTR翻译表、PVF目录树 |
| `analysis/dumps/README.md`                                   | Dump 资产说明、快速查阅指南、IDAPython 注释工具与逆向解密链备忘 |

> 旧索引里的 `GIT-MANAGEMENT.md` 经 2026-10-04 核实**不存在**，其内容职责已并入 §0.3；不要再引用该文件。
