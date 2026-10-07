# 115US 整合导入包（20261005）接入分析报告

> **执行状态（2026-10-06 更新）**：业主已定「**A 系列全部合并、B 系列拒绝、C 系列作为 mod（后续再调）**」，
> **本轮合并已执行完成**：`go build ./...` / `go vet ./...` / `go test ./... -count=1` 全部通过（42 ok / 0 FAIL，
> 与合并前基线一致）。实际落地：**新增 86 个文件、修改 100 个文件**。
> 执行结果、两处交付包缺件与自行补写的模块，见文末 **§十 执行结果**。
> 下文 §一~§九 保持分析时的原样（含当时的判断与实际执行的差异说明）。

- **交付包**：`C:\Users\Ricar\Downloads\115US-整合导入-改动交付-20261005`
- **目标树**：`server/work/dfo-lan`（当前部署树）
- **本机基线**：`HEAD = e1a5a3e2`（2026-10-05 22:38）
- **合并基线（merge base）**：`bfde3006`（2026-10-04，MR139）——已用 blob 哈希实测确认：交付包 `cmd/admin/main.go`
  等文件与 `bfde3006` 逐字节相同，且 `bfde3006` 是 `HEAD` 的祖先
- **当前存储**：`runtime/storage/local.json` = `driver=sqlite`；`bin/wireprobe-pvf.exe`（2026-10-05 21:21）已是
  去 PG 版本 —— **本机不存在 PostgreSQL 存档路线**
- **业主口径（2026-10-05/06 本轮）**：① 本轮**只出分析报告，不改代码**；② **保持 SQLite-only，拒绝包的
  双引擎层**（依据根 `AGENTS.md` §0.6）
- 分析方式：全量文件清单（996 项）三路对比（base / ours / theirs），`git merge-file --diff3` 逐文件三方合并，
  无任何工作区写入（仅 `.tmp/integration-20261005/` 下的分析产物）

---

## 一、结论速览

1. **交付包不是"需要覆盖的新版本"，而是同一棵树在另一台机器上的快照**：996 个受管文件中
   **634 个（64%）与当前树逐字节相同**——精确口径是 **629 个已被 git 跟踪**（我们自己的历史提交被包反射回来），
   另外 **5 个是"工作区里有、但本仓从未提交过"**（`cmd/readme.md`、`configs/readme.md`、
   `scripts/generate-sql.ps1`、`scripts/restore-next79-baseline1.ps1`、`scripts/set-ispins-mode.ps1`，
   与包内容一致 ⇒ 同样无需处置）。**这 634 个一律不动。**
2. **真正要接的内容约 173 个文件**：156 个可无冲突落地（103 新增 + 53 纯包侧修改），17 个三方干净合并。
3. **与当前树的冲突共 79 个**，其中**真正的分歧集中在两处**：
   - `internal/database/**`（46 个）与 `cmd/gmtool/storage*`、`sqlc.yaml`、`go.mod/go.sum`：
     **包的"双引擎（PG+SQLite）" vs 本机"SQLite-only"**。按业主口径**整块拒绝包的这一层**即可消掉绝大多数冲突。
   - `cmd/wireprobe/**`（约 45 个，其中 45 个为"两边都新增"）：多数是我们自己的文件被包反射，
     **确认内容一致后跳过**；剩少数需人工裁决（见 §五）。
4. **有 3 处需要业主拍板的语义分歧**（§五 A/B/C），另有 5 类"包里的落后实现"必须拒绝（§四）。
5. 包还带了**启动器侧新框架 `launcher-repo/`（modkit）**，属另一个仓（本机
   `C:\Game\dof\115us\115us-dfolauncher`，自身有未提交改动）；本轮业主已定**只做服务端分析**，该部分单列（§七）。

---

## 二、文件级分类（996 项受管文件）

| 分类 | 数量 | 含义 | 建议处置 |
| --- | ---: | --- | --- |
| `identical` | **634** | 与当前树内容完全一致 | **什么都不做**（含 5 个"未跟踪但同内容"的 readme/脚本） |
| `add` | **103** | 包新增、当前树没有 | **落地**（扣除 §四 的 4 处路径问题后 99 个） |
| `take-theirs` | **53** | 包相对 base 改了、我们没动 | **取包侧** |
| `merged-clean` | **17** | 双方都改，但三方合并无冲突 | **取合并结果**（需逐一确认语义，见 §六） |
| `pkg-unchanged` | **58** | 我们改了、包没动 | **保持我方**（含 `cmd/wireprobe/listen.go`、`main.go`） |
| `CONFLICT-late`（new-both） | **45** | 包算"新增"，但当前树已存在同路径文件 | 逐个比对：一致则跳过；见 §四/§五 |
| `merge3 CONFLICT` | **34** | 文件内真冲突（共 46 处冲突块） | 按 §三/§四/§五 逐处裁决 |
| `ours-deleted`（REVIEW） | **35** | 我们在基线后删了该文件，包改过它 | 33 个=去 PG 时删的库测试（**忽略**）；2 个=启动链 Python（**忽略**） |
| 合计 | 996 | | |

### 冲突块性质分布（自动分类，共 46 处）

| 性质 | 数量 | 说明 |
| --- | ---: | --- |
| `SAME-TEXT` | **14** | 双方内容**完全一样**（都删掉了 base 的 `pgx` import），**只删冲突标记即可** |
| `postgres-related` | **16** | 纯 PG 相关（`pgx` / `PostgresDSN` / 双引擎注释）→ 按 SQLite-only **取我方** |
| `ours-deleted-content` | 4 | 我方删掉的内容（PG 旗标等）→ 取我方 |
| **真内容分歧** | **20** | 需逐处判断（其中 12 处属包侧新功能，见 §五） |

---

## 三、可直接落地的内容（无需裁决）

### 3.1 巴尔卡 raid 全套（包侧新增，最大一块）

`cmd/wireprobe/`：`raid_bakal_flow.go`、`raid_bakal_portal.go`、`raid_bakal_room_warp.go`、`raid_bakal_script.go`、
`raid_bakal_retreat.go`、`raid_bakal_rewards.go`、`raid_bakal_party_buffs.go`、`raid_team.go`、`raid_channels.go`、
`client_raid_entrance.go`、`border_reward_flow.go` 及其测试；
`internal/raid/**`（5）、`internal/catalog/raid_bakal*.go`（8）、`internal/loot/{border_plan,attunement_multiplier}.go`、
`internal/channelrefresh/source_raids.go`、`internal/dungeon/{raid_boss,raid_stage}.go`、
`internal/gamedata/source_raid.go`。

> 依据：包内记录 `records/合并记录-20261005-devpack服务端合并.md` —— 72 个新增文件即此批；实机链路
> `bakal_subparty_created → … → bakal_loaded ×5 → bakal_script_warp ×2` 已验证。

### 3.2 其它新增功能

- 蔚蓝号（Azure）：`cmd/wireprobe/azure_main_flow.go`、`internal/dungeon/azure_maze_native_test.go`、
  `internal/game/protocol/azure_finish_test.go`、`internal/loot/azure_*`
- 装备变更系统：`internal/catalog/equipment_transform_system.go(+test)`
- 晶体变换 / 维纳斯 / 誓约 / 地狱派对等：`internal/game/protocol/{primer_transform,legion_standby_party,...}.go`、
  `internal/workflow/*`、`internal/legion/*`
- 频道侧：`cmd/wireprobe/raid_channels.go`、`internal/channelrefresh/source_raids.go`
- 文档：`docs/protocol/*.md`（4 篇）、`docs/moon-channel-enablement-20260928.md`
- 配置：`configs/moon-solo.local.json`、`configs/raid-entries.local.json`
- 工具：`internal/toolcmd/accountlist/main_test.go`、`internal/toolcmd/sqliteconvert/**`（**注意**：见 §四-3）

### 3.3 可取的包侧修改（`take-theirs` 53 项）

多为测试与文档；其中代表性的是 `cmd/wireprobe/worn_random_option_restore.go(+test)`、
`internal/game/protocol/*`、`internal/inventory/*` 等。

---

## 四、必须**拒绝**的包侧内容（理由逐条）

| # | 内容 | 数量 | 拒绝理由 |
| --- | --- | ---: | --- |
| 1 | `internal/database/**` 的 PG/双引擎层、`sql/sql/postgres/**`、`pgx` 依赖、`sqlc.yaml` 的 postgresql 段、`go.mod/go.sum` 的 `jackc/*` | ~46 | 根 `AGENTS.md` §0.6：**SQLite 唯一引擎**，PG 已于 2026-10-05 整体移除；本机 `local.json` 就是 `driver=sqlite`，引入即破坏当前运行环境 |
| 2 | 33 个库测试（`*_postgres_test.go`、`sqlc_postgres_test.go`、`store_postgres_test.go`、`fixture_postgres_test.go` … 及 `cash_purchase_test.go`、`skin_cargo_test.go` 等） | 33 | 实测：这些文件**全部**带 `if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" { t.Skip }` + `loadPostgresTestConfig()` + `timestamptz` DDL，且引用包侧类型（`CashOrder` 等）。**在 SQLite-only 树上既编不过也跑不了**；忽略**不丢任何覆盖** |
| 3 | `cmd/equipmentfull/main.go`、`cmd/gennames/main.go`、`cmd/itemshopimport/main.go`、`cmd/pvflist/main.go` | 4 | 违反 §0.4.1（不得新增 `cmd/<工具名>`）：`equipmentfull` 我们已有 `internal/toolcmd/equipmentfull`（包那份是旧的，差 8+/7-）；`itemshopimport`/`pvflist` 已被提交 `d792ebf1`（59 工具并入 dfo-tool）与 `2261c8c0`（删 39 个无引用旧工具）**明确删除**，回填=回退这两次收口；`gennames` 是全新工具，若要须落到 `internal/toolcmd/gennames` |
| 4 | `scripts/{ensure_inner_pvf,launch_local,bootstrap_local,prepare_inner_pvf}.py`、`test_postgres_storage.py` | 6 | 启动链已按业主口径**全面去 Python**（`launch`/`init-storage` 均在 Go 启动器内）；包侧这两处修改（环境键白名单/内层 PVF 哈希校验）只对 Python 链路有意义 |
| 5 | `server-work-dfo-lan-changes/wireprobe-new.exe`（包根） | 1 | 编译产物，非源码；按 §0.3.3 不入库 |

> 补充：`internal/database/sqlite*.go` 这些**同名文件**包侧与我们也不同（包是双引擎适配层，我们是单引擎实现），
> 因此第 1 条是"整层拒绝"，不是逐文件取长补短。

---

## 五、需要业主人裁决的语义分歧（3 处）

### A. NOTI108 频道开放门禁表：**7 条（我方）↔ 19 条（包侧）**

- 我方现状：`cmd/wireprobe/event_info_generated.go` 的 `eventInfoTableHex` 是 **7 条**表；
  `cmd/wireprobe/story_digest_test.go:293,323` 把 7 条钉死（`TestEventInfoTableRidesAnnounce` / `TestEventInfoTableBody`）。
- 包侧：改成 **19 条**（覆盖全部 raid 页签位），并入 `cmd/wireprobe/client_entry.go`（恒定下发，不再依赖
  `DFO_RAID_OPEN_EVENTS_PROBE`）。
- 我方历史：2026-10-04 业主指令「时间一次全开每天」→ 7×52 定长记录表；包记录里同一天的判断是
  「上游已把静态表升级为 19 条官方门禁记录（实机验证）」。
- **两个选项**：
  - **A1（推荐）取包侧 19 条**：与包实机验证一致，Raid 页签"全开"由服务端表驱动；
    代价：`event_info_generated.go` + `story_digest_test.go` 两个测试期望要同步改。
  - **A2 保留我方 7 条**：不动我方表；代价：包的 raid 频道门禁记录进不来，需另找下发路径。
- 影响面：Raid 页签是否显示全部 raid 分组（实机可见）。

### B. `cmd/wireprobe/main.go` / `config.go` 的 PG 旗标

我方已在去 PG 时删掉 `-postgres-*` 系列旗标（1 处 `ours-deleted-content` 冲突）。按业主 SQLite-only 口径
**取我方**（无需额外确认，列出仅为留痕）。同理 `internal/database/{mailbox,migrations}.go` 的
`ours-deleted-content` 冲突。

### C. `cmd/wireprobe/dungeon_flow.go`（3 处冲突块）+ `client_connection.go` / `world_flow.go` / `catalogs.go`

这些是"我方 raid 接线 + 包侧 raid 接线"的合并点：
`client_connection.go`：我方已把函数名改为 `autoPickSettlementCard`，包侧新增 `tickBakalOpening` 调用 ——
**两处都要保留**（包侧调用 + 我方改名），属机械合并，无需裁决，但需人工确认后落盘。

---

## 六、`merged-clean`（17 项）与 `CONFLICT-late`（45 项）的处置原则

- **`merged-clean`（17）**：三方合并无冲突，但**不等于语义正确**（可能出现"双方各自加了一段"的重复）。
  逐个人工复核 diff 后再落盘，重点看 `internal/database` 之外的 17 项。
- **`CONFLICT-late`（45）**：**先比对、再决定**——其中 52 项（含本类）经实测与当前树**逐字节相同**
  （包把它们算作"新增"，是因为 `bfde3006` 里没有；它们来自我们 `bfde3006` 之后的提交或未提交改动）。
  **处置：内容一致 → 跳过，不碰**。少数内容不同的必须单独判断，已知的只有：
  - `internal/database/{engine,queryset,sync,driver_errors,sqlite_*,*_dualengine_test}.go` → §四-1 **拒绝**
  - `docs/sqlite-dual-engine-design.md`、`docs/sqlite-operations.md` → 与 SQLite-only 口径冲突，**拒绝或改写**
  - `cmd/dfolauncher/{main,storagesync}.go`、`internal/launcher/{storage,initstorage}.go` → 我方已有更完整实现，
    **保持我方**（需逐项核对）
  - `internal/wfpisolate/*`（3）→ 保持我方（WFP Go 化是我方收口成果）

---

## 七、包内其它部分（不在本轮服务端范围）

| 目录 | 内容 | 处置建议 |
| --- | --- | --- |
| `launcher-repo/internal/modkit/**` + `cmd/modkit/main.go` + `docs/modkit.md` | **多 MOD 统一挂载框架**（清单→冲突检测→备份→原子应用→逐字节还原；CLI `verify/plan/install/status/uninstall`；7 项单测） | 落到启动器仓 `C:\Game\dof\115us\115us-dfolauncher\internal\modkit` + `cmd/modkit`（该仓有未提交改动，需另开一轮并单独提交） |
| `mods/*__mod.json`（3 个） | resources / pvf / client 三组 mod 清单 | 随 modkit 一起，属客户端资源安装，不动服务端仓库 |
| `pvf-kit/` | PVF 导入套件（`导入PVF差异.ps1`、两个工具 DLL、delta 重建脚本）+ 304MB delta 由原包重建 | 客户端资源侧；`.dll` 按 §0.3.3 需单独确认是否入库 |
| `records/` | 两份合并记录 + 115main4 方案 + 合并工具脚本 | 已有价值：建议把两份合并记录归档到 `docs/todo/`（本报告已引用其结论） |
| `server-changes.json` | added/modified/deleted 三张清单 | 已被本轮分析复用；可归档 |

---

## 八、本轮已执行的动作（可复现）

1. 全量清单三路对比：base(`bfde3006`) / ours(工作区) / theirs(包)，CRLF↔LF 规范化后逐文件比较。
2. `git merge-file --diff3` 三方合并 988 个受管文件，产出 `merge-report.csv`（分类）+ 46 处冲突块分类。
3. 基线验证（改动前）：`go build ./...` → **exit 0**；`go test ./... -count=1` → **exit 0（41 ok / 0 FAIL）**。
4. **未修改任何工作区源码**。分析产物全部在 `.tmp/integration-20261005/`（未跟踪）：
   `merge-report.csv`、`merged/`（合并结果）、`base/`（基线树）、`logs/baseline-{build,test}.txt`。
   备份：`.tmp/integration-20261005/`（`cmd/configs/docs/internal/scripts` 共 39.5MB，2081 文件）。

### 建议的落地顺序（业主批准后执行）

1. 先落 §三（156 项无冲突 + §六 的 `merged-clean` 17 项），**不碰** `internal/database`。
2. 消掉 34 个 `merge3` 冲突：14 处 `SAME-TEXT` 直接删标记；16 处 `postgres-related` + 4 处
   `ours-deleted-content` 取我方；剩 20 处按 §五 逐处裁决。
3. 逐项复核 45 个 `CONFLICT-late`：一致→跳过；不一致→按 §六 处置。
4. `gofmt` + `go build ./...` + `go vet ./...` + `go test ./... -count=1`，失败集合与基线（0 FAIL）**逐名比对**。
5. 编译产物（`wireprobe-pvf.exe` / `wireprobe-handoff-source.exe`）按 §0.5 单独换装，实机由业主操作。

---

## 九、未闭环 / 风险

1. **A 项（7↔19 门禁表）未定** → 决定 Raid 页签实机表现，必须业主拍板。
2. 包的 `internal/launcher/**` 与我方实现**同源但不同代**（包在另一台机上也算"启动器侧"）；若将来要合并启动器仓，
   需以启动器仓 `115us-dfolauncher` 为真源重新做一轮三方对比，**不能**用本报告结论直接覆盖。
3. `merged-clean` 17 项的**语义**尚未逐个人工复核（本轮只做到机器合并无冲突）。
4. 包内 `cmd/equipmentfull/main.go` 相对我方 `internal/toolcmd/equipmentfull/main.go` 多 8 行少 7 行，
   **尚未判断哪边新**；若该工具仍在使用需单独比对。
5. 本报告不构成实机验收；实机仍由业主操作，AI 只读日志。

---

## 附：关键证据路径

- 包内合并记录：`records/合并记录-20261005-devpack服务端合并.md`、`records/合并记录-20261005-115main4合并.md`
- 包内问题清单：`问题与修复-详细记录.md`
- 包内机器可读清单：`server-changes.json`（added 728 / modified 268 / deleted 200）
- 本报告分析产物：`.tmp/integration-20261005/merge-report.csv`（996 行逐文件分类）


---

## 十、执行结果（2026-10-06，A 全并 / B 拒 / C 留待 mod）

### 10.1 落地规模

| 项 | 数量 |
| --- | ---: |
| 新增文件（本包带来、原树没有） | **86** |
| 修改文件 | **100** |
| 明确拒绝（B 系列） | 69 |
| 无需动作（与当前树逐字节一致） | 634 |

### 10.2 验证（真实输出）

- `gofmt`：本次落地文件已按 gofmt 规整（只动本任务文件，未做全树重排）。
- `go build ./...` → **exit 0**
- `go vet ./...` → **exit 0**
- `go test ./... -count=1` → **exit 0，42 ok / 0 FAIL**
  （合并前基线失败集合同样为空；日志 `.tmp/integration-20261005/logs/{baseline-test,after-test3}.txt`）

### 10.3 A 系列落地明细

- **巴尔卡 raid 全套（A1）**：`cmd/wireprobe/{raid_bakal_*,raid_team,raid_channels,client_raid_entrance,border_reward_flow}*`、
  `internal/raid/**`、`internal/catalog/raid_*`、`internal/dungeon/raid_{boss,stage}.go`、
  `internal/game/protocol/raid_*` + `border_reward.go`、`internal/workflow/raid_bakal_rewards.go`、
  `internal/gamedata/source_raid.go`、`internal/channelrefresh/source_raids.go`、`internal/loot/{border_plan,attunement_multiplier}.go`。
- **蔚蓝号 / 晶体变换 / 装备变更 / 军团维纳斯 / 誓约 / 组队频道**：按 `merge-report.csv` 的
  `add` / `take-theirs` / `merged-clean` 三类落地。
- **配置**：`configs/raid-entries.local.json`、`configs/moon-solo.local.json`（**门禁已提示需业主确认**，见 10.6）。
- **文档**：`docs/protocol/{creature-state-dungeon-swap,equipment-inherit-chain,party-matching-board,party-trade-auction}*.md`、
  `docs/moon-channel-enablement-20260928.md`。

### 10.4 与 §五 A 的差异（事实订正）

原分析曾判断「我方 7 条 ↔ 包侧 19 条」需要裁决。**实测为误判**：当前树
`cmd/wireprobe/event_info_generated.go` 的 `eventInfoTableHex` 已是 **1141 字节 / 19 条**，
且与包内文件**逐字节相同**；`story_digest_test.go` 的期望（`wantLen = 1141`、count=19）也已经是 19 条。
故 **NOTI108 门禁表本来就一致，无需任何改动**。

### 10.5 交付包缺件与自行补写（★ 需要业主知悉）

交付包 `server-work-dfo-lan-changes` 里**有 4 个共享文件的改动引用了包里从未定义的符号**，
且这些符号在本仓全部历史（`HEAD` / `origin/main` / `fork/main` / merge base）里都不存在 ——
说明**对方部署树里有几个从未进入交付包的源文件**。缺件符号与受影响文件：

| 缺失符号 | 引用于 | 判断来源 |
| --- | --- | --- |
| `townPartyHub` / `townParty` / `newTownPartyHub` | `cmd/wireprobe/{bootstrap,world_flow}.go` | 城镇组队枢纽；包内文档 `party-trade-auction-20260929.md` 提到 `cmd/wireprobe/party_flow.go` —— **该文件不在包内** |
| `raidEntriesConfig` / `loadRaidEntriesConfig` | `cmd/wireprobe/client_connection.go` | raid 入口 N537 覆盖包（键表见 `configs/raid-entries.local.json`） |
| `partyHandle` / `partyDisconnect` | `cmd/wireprobe/{client_dispatch_world,client_connection}.go` | 同上（组队族 C12/C13/C14 分派与断线清理） |
| `dropCreatureEggs` | `internal/loot/session.go` | 宠物蛋掉落过滤；包注释自证原文件名 `egg_drop_filter.go`，**该文件不在包内** |

**处置（本轮已做）**：

1. 上述 4 个共享文件**回退到我方 `HEAD` 版本**，避免引入无法编译的引用；
2. 逐项把 raid 功能真正需要的**结构体字段**补回我方文件（都是数据声明，无行为猜测）：
   `gatewayRuntime.{raidEntrances,raidTeams,channelSpawns}`、
   `worldSession.{raidWaiting,bakalOpening,bakalRules,bakalParty,bakalTown,bakalSettledRun,bakalRewardRetryAt,channelSpawns}`、
   `gameConnection.{raidOwnerRole,ownedRaidID}`、`entryPayloads.ChannelEventInfo`；
3. 补回 `prepareRuntime` 的 **channelSpawns 投影 + 82 落点**（照包内注释与既有 73 频道先例，与包侧同源）；
4. **`dropCreatureEggs` 忠实重写**为 `internal/loot/egg_drop_filter.go`：以本仓已有的
   `inventory.EggHatchOutputs`（PVF 解析出的「蛋模板 → 孵化产物」表）为判定真源，不新增平行清单。
   **若与包作者原实现语义有差，以实机日志校正**；
5. `cmd/gmtool` 补回 `handleUnavailableAPI`（包内 `unavailable_api_test.go` 依赖它；501 + 认证，不谎报成功）。

**仍然缺口（未接入，等业主提供源文件或另行决定）**：城镇组队枢纽
（`partyHandle` / `partyDisconnect` / `townPartyHub`）与 **raid 入口 N537 覆盖包的加载**
（`loadRaidEntriesConfig` + `config.go` 的 `RaidEntriesConfig` 旗标）未接线；
因此**组队族 C12/C13/C14 分派**与**raid 入口次数覆盖**当前不生效（其余 raid 链路已可用）。

> 补件请求（给包作者／另一台机器）：`cmd/wireprobe/party_flow.go`（或定义 `townPartyHub`/`townParty` 的那个文件）、
> 定义 `raidEntriesConfig`/`loadRaidEntriesConfig` 的文件、`internal/loot/egg_drop_filter.go`。
> 拿到后可直接替换上面的补写实现。

### 10.6 提交前门禁结果（**尚未提交**）

`scripts/check-commit-hygiene.ps1 -All` → **exit 2（需二次确认）**，命中 3 条：

| 路径 | 类别 | 条款 | 建议 |
| --- | --- | --- | --- |
| `.launcher-resource-cache.json` | 新增顶层文件 | §0.4.3 根目录白名单 | 与本次整合无关（启动器运行期缓存）；**不入库**，建议补 `.gitignore` |
| `server/work/dfo-lan/configs/moon-solo.local.json` | 新增服务端 configs JSON | §0.4.1 / §0 铁律 1–3 | 包带来的本地运行配置（`*.local.json` 一类）；待 C 系列 mod 方案定案后评估是否仍需留仓 |
| `server/work/dfo-lan/configs/raid-entries.local.json` | 同上 | 同上 | 同上；且它是 10.5 里 `loadRaidEntriesConfig` 的输入数据 |

按 §0.3.1：**这三条取得业主明确确认后才可提交**；未确认前不暂存、不提交。

### 10.7 未做 / 未闭环

1. **未提交**（等门禁二次确认）；
2. 未编译发布二进制、未换装 `bin/*.exe`、未启动客户端（实机一律由业主操作）；
3. C 系列（资源组 / PVF 组 / 客户端原生组 / modkit / pvf-kit）**未动**，按业主要求待合并完成后再议 mod 形态；
4. 10.5 的组队枢纽与 raid 入口覆盖包未接线；
5. `dropCreatureEggs` 与 `handleUnavailableAPI` 为补写实现，**未经实机验证**。
