# PVF 单一内容真源改造计划

更新：2026-10-03。用户要求 PVF 作为核心游戏内容文件，消除另一套人工维护的玩法 JSON，并明确 PVF 对服务端是只读资源。内容修改由服务端之外的编辑工具完成。本文记录目标、第一批实际审计和迁移缺口；不表示全量配置依赖已经解除。

## 2026-10-03：17 个 JSON 收口

按用户“先收口刚刚说的能删的”授权，删除 17 个顶层 JSON，126→109，共 23,584,750 字节（约 22.49 MiB）。只处理已经完成引用审计的这批文件，不扩展删除其余政策或 JSON 回退数据。

| 删除文件 | 处理依据与保留内容 |
|---|---|
| `cerashop.json` | 商城已仅从 PVF 准备，无运行/测试消费者；删除旧 `scripts/export_cerashop_catalog.py`，避免再生成无用途的内容表 |
| `dungeons.odyssey-merged-candidate.json` | 删除前与 `dungeons.odyssey-release.json` 字节/SHA256 完全相同，测试统一读后者 |
| `dungeons.odyssey-scenes-candidate.json` | 同上，保留 `dungeons.odyssey-scenes-release.json` |
| `odyssey-growth-candidate.json` | 同上，保留 `odyssey-growth-release.json` |
| `odyssey-weapon-box-candidate.json` | 同上，保留 `odyssey-weapon-box-release.json` |
| `skills.awakening-candidate.json` | 同上，保留 `skills.release.json` |
| 11 个 `pvf-*-candidate.json` | `all`、`boxes`、`cashshop`、`characters`、`content`、`direct`、`enhancement`、`item-shops`、`migration`、`next`、`odyssey`；当前引用仅在历史 profile 测试，默认档保留 |

22 个 Go 测试文件仅合并数据路径，测试逻辑保持。Python profile/启动器共 24 项检查保留，当前范围与政策检查读取 `pvf-default.json`，显式路径/校验/隔离检查用临时配置；新增对显式 profile 与 `--source-build` 同用时仍遵循 profile binary 的断言。两份交付清单只移除对应 7 个条目，不全量重生成；字面引用清单按当前源码重生成。

Go 1.26.5 清理前全量 `go test ./...`、清理后无缓存全量 `go test -count=1 ./...` 与 `go vet ./...` 均通过；Python 3.11.9 的 24 项 profile/启动器检查通过。测试日志与删除前文件身份核对记录位于 `.tmp/config-cleanup/`，隔离 Python 依赖也只放在该临时目录，不提交。

运行原启动等待测试时发现其未模拟 `_ensure_inner_pvf`，全局 `Path.is_file` 模拟让真实服务端内层 PVF 被轮换成备份。已原位恢复并核对 SHA256 `8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934`，该测试补上准备函数模拟及调用断言；Python 检查重跑通过，恢复资源后 Go 全量重新验证。客户端资源与服务端内层资源均无净改动。

本批没有修改生产 Go/Python 加载逻辑或默认 profile，没有编译/替换运行程序、访问玩家库或启动客户端。仅确认源码与配置清理，confirmed baseline 保持既有运行程序和实机范围。`pvf-lottery-policy.json` 的历史对照、仍强制读取的空 selection/content policy，以及探针/GM/JSON 回退消费者继续保留。下文为各阶段原始记录，旧数量、阶段入口和当时的解析缺口不表示当前状态。

## 第 0 批：真实运行依赖台账

已完成入口 → profile/environment/default flags → PVF/JSON 读取与运行消费者的文档审计，见[PVF运行依赖台账](PVF运行依赖台账.md)。当前工作区的 launch_local.py 改动被保留并按现状记录：只有显式 --pvf-mode 才选择默认 PVF profile，根入口不自动传该参数。Phase 01 源码整合与离线验证已完成：wireprobe 内 `pvf_*.go` 共 0 个（含测试），32 个生产读取文件集中到 5 个 `internal/gamedata/catalogs*.go`，以 `Catalogs` / `PrepareCatalogs` 暴露，由 `main` 装配；运行期适配见 `catalog_runtime.go`，两个预热文件归入 `role_detail_prewarm*`，46 个原测试名称保留并聚合到 4 个 gamedata 测试文件和 `catalog_runtime_test.go`。Go 1.26.5 build/vet、Python 3.11 启动器测试 10/10、完整 54 域离线 prepared-check 通过；`go test ./...` 仍有 4 项在原 HEAD 已复现的失败，详见[运行依赖台账](PVF运行依赖台账.md)。实际归档的 NativeLotteryDiscovery/startup 与 NativeRolePrewarm 检查通过；未操作玩家数据库或客户端、未替换正式/源码 bin、未实机确认；本批仅收口源码提交。gamedata 作为 L4 组合/只读投影层的职责依据见[服务端依赖契约 §1、§2/R6、§4](../../../server/work/dfo-lan/docs/architecture-contract.md#1-分层)。本阶段不表示 configs 剩余内容依赖已清除；集中读取便于替换实现而不改业务逻辑，不新增运行时来源开关，读取错误不隐式切换实现。台账行号仍指向审计基点 HEAD b7a5692。未改变内容范围或数值。

## 上游同步与源码收口

按用户“同步上游代码后提交”的要求，合入 `upstream/main` 的 6 个新提交，最新为 `6eb61fc`，合并提交为 `05f14ed`。包含地图/物品/技能/任务按需读取、共享压缩字符串池、选角预热和历史未知背包/任务兼容修复；保留本分支已有公共流程精简。随后恢复本批抽奖自动发现、只读审计和 24 个 JSON 的清理。

冲突保留双方 CHANGELOG/交接记录及上游按需加载实现，角色源校验使用上游提前执行的非空锚点检查，移除重复检查。本批源码和删除清单按用户授权收口提交；未获得新增实机反馈，运行 confirmed baseline 保留上游第三批确认记录，不将此次合并或编译记为实机确认。

合并后当前 `8b2a9f83…` PVF 回归仍为 276 普通/2,477 装备抽奖池，271,492 个权重区间边界通过；无导出 JSON 的角色/抽奖准备通过。自选审计保持 16,757/16,741/2/5/9 的候选/接受/固定/未建模/拒绝统计。14 项 profile 检查通过，引用清单已按合并后源码重新生成。

合并后的 Go 1.26.0 全量 `go test ./...` 和 `go vet ./...` 均通过，包括上游旧背包预热兼容回归。旧默认启动器测试与用户启动设置的 5 项既有不匹配仍按下文记录，未修改用户设置。

合并后源码候选 SHA256 为 `aabb142d06f3bb6f479c2e7b93302708e3c2f6823d2727d2e0fc1166774bc3be`，下文 `43a965b5…` 为同步前候选记录。默认程序保留同步前文件。`.gitignore`、`launch_local.py`、`pgdata/postgresql.conf` 用户改动用同步前后文件哈希核对，保持原样并排除提交；没有将 pgdata 或临时目录保存到本任务 stash/提交，没有运行客户端或玩家数据库。

## 目标与配置归属

物品属性、价格、技能、职业、任务、副本、掉落及源中可表达的调服数值统一维护在 PVF 中。服务端只读发布后的资源，解析成各领域的内存规则，运行及加载过程不得反写 PVF。导出物和磁盘缓存只用于自动生成、验证和加速，不允许编辑它们改变玩法。

不采用“PVF 加一套差异 JSON”的长期方案。PVF 编辑、校验和版本发布由服务端之外的内容工具完成；游戏服务端不承担 PVF 编辑职责。GM 发放、角色修复等操作仍属于玩家状态管理。数据库地址、端口、日志级别、监听和部署路径保留为运维配置。

部分机制可能在客户端代码而非 PVF 中，需用当前 IDB 和实机证据确认。未定位到表不能推断 PVF 没有该规则，也不能把现有兼容公式搬进另一份文件后宣称完成单一真源。

## 第一批完成的工作

- 初次审计 160 个 JSON，生成 [字面引用清单](configs字面引用清单.md)，提供网关、业务包、工具、测试、脚本与配置中的引用位置；清单现按下文清理后的文件重新生成。该表不是运行时依赖证明；动态路径、外部启动器、GM 代理与历史二进制仍需追踪。
- 在现有 `cmd/pvfaudit` 增加 `-selection-scope`，从原生 LIST 绑定和 `[stackable type]` 自动发现所有自选类型脚本，逐项记录现有解析器的接受/未建模/拒绝结果。此模式不读手工 ID 清单或 JSON 游戏目录，不启动服务，不访问存储。
- 记录源版本及逐条问题的模板 ID、脚本路径、原始 SHA256 和错误，按照 ID 排序。报告详情可截断，统计覆盖全部候选；未建模或拒绝条目使命令返回 2，错误参数/读源失败返回 1。
- 修改 architecture.md 的 ADR-003，明确服务端只读 PVF、外部工具负责内容编辑，移除长期维护玩法覆盖文件的目标。

本批没有改变网关运行范围、协议、数据库结构、存档来源或客户端资源，也没有替换发布程序。自动发现可读内容不等于客户端及发放路径已经完整支持该内容。

## 当前源身份与真实审计

本次从当前 `client/DFO.exe`、`client/sk.dat`、`client/Script.pvf` 使用现有只读解封工具生成隔离副本，路径为 `server/work/dfo-lan/.tmp/pvf-source-rules/Script.inner.pvf`。

| 资源 | SHA256 |
|---|---|
| DFO.exe | `235f6281aebdfaff40d85fc46308bb1aaac151346d4c07ebd8117f92bb875ccd` |
| sk.dat | `59d78371335d22dae0eaeca2a827939576e85a5f5ede4ba286f0438d2fbc6eaf` |
| 客户端 Script.pvf | `2e11d8d6a65f04db5c57c229903e01c1cac56e8e190365fcb05e3e4259ce5146` |
| 隔离内层 PVF | `8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934` |

该源与旧导出目录的 `7ef2db59…` 不同。历史 54 项/63 投影记录不作为本次当前源的等价证明。没有执行玩家存档跨源迁移。

| 自选类型审计 | 数量 |
|---|---:|
| 当前 PVF 原生自选类型绑定 | 16,757 |
| 现有解析器与单条目录校验接受 | 16,741 |
| 固定 booster 内容 | 2 |
| 未建模内容 | 5 |
| 单条目录校验拒绝 | 9 |
| 旧手工政策清单 | 2,978 |
| 当前源额外绑定 | 13,779 |
| 旧清单 ID 在当前类型集合中缺失 | 0 |

2,978 个旧清单条目可用现有导入器完成当前源读取；自动扩大到所有 16,757 个条目时会遇到以下缺口。不能直接删掉清单、全局启用并忽略错误。

| 模板 | 当前解析缺口 |
|---|---|
| 10333625、10356172、10400485、10405993 | 重复分类 `[13,0]` |
| 10345171 | 重复分类 `[0,0]` |
| 10350659、490709000、490709003 | 重复分类 `[1,0]` |
| 50006227 | 解析出的奖励包含目录校验不接受的物品 |
| 10344930、10358468、10413667、50050910、50050911 | 未得到现有模型可用的分类或固定 booster 块 |

重复分类可能涉及源条件块、不同内容段或客户端合并行为。下一步须定位各脚本和原生 reader，确认分支与消费语义；本轮不将“取第一条/最后一条/合并全部”当作修复。5 个未建模条目也不自动视为无奖励。

小深渊 `100005014` 的当前原生 DGN 为 `contents/2026/endkeeperoforder/dungeon/endkeeperoforder.dgn`，原始 SHA256 `032aff9e1336515c78b689bae9a93159842c1fd02e8235ef4639f0879f277298`，迷宫权重为 `[992857,7143]`，第二迷宫占 0.7143%。手工策略中的 `[980000,20000]` 来自此前明确要求的 2% 调整。长期应由内容编辑流程表达这个选择；本批仅核对与记录，未替用户取消 2% 或写入 PVF。

完整本地证据位于 `.tmp/pvf-source-rules/`：`source-manifest.json`、`selection-discovery.json`、`selection-scope-report.json`、`selection-scope.stderr`、`selection-scope-test.log`。它们是自动审计产物，不进入 Git，也不作为手工维护的游戏配置。

## 第二批：抽奖先解除手工 JSON 依赖

按用户要求先迁移其他已能闭环的内容，自选礼盒的 14 条解析缺口暂留。

- 网关抽奖准备从原生 LIST 索引中的 `[upgradable legacy]` 类型自动发现脚本，读取完整 `[int data]` 奖励三元组，再按已有发放能力分类为普通/金币奖池或装备奖池。运行不读取 `pvf-lottery-policy.json`、`lottery-item-pools.json`、`lottery-equipment-pools.json`；只在明确启用旧基线对照时读取历史导出物。
- 当前 `8b2a9f83…` 源发现 2,871 个候选，276 个普通奖池、2,477 个装备奖池，与原有手工清单逐池、逐奖励完全一致。剩余 118 个保持不可用：108 个存在缺失奖励、3 个无效三元组、5 个不支持的内容结构、2 个不符合现有发放能力。每项启动日志记录模板、原生路径、原始 SHA256 和原因；整个奖池拒绝，不删除个别奖励改变概率。现有 CMD27 在无可用奖池时于扣物之前拒绝。
- 直读绑定不再要求固定的 276/2,477 数量或指定几个模板的历史奖励哈希。仍验证实际 PVF 来源、脚本身份、物品类型、奖励数量及权重；源内容变化不会被旧内容快照强行挡住。旧 JSON 回退保留历史向量校验。
- 默认 profile 移除 `DFO_PVF_LOTTERY_POLICY`。旧 `-pvf-lottery-policy` 参数保留兼容入口但明确忽略，不再读取路径；历史候选 profile 的旧环境项不影响自动发现。
- 修正目录准备中重复且无条件的角色源比较，空锚点按现有约定派生实际归档身份，非空异源仍由后续门禁拒绝。未修改存档身份或迁移路径。

真实源验证通过 271,492 个权重区间首尾边界，奖励、权重、数量和顺序一致；使用不存在的政策、索引与奖励 JSON 路径完成抽奖准备和绑定。另测新模板/新数量/新奖励权重可在源身份验证下绑定、异源拒绝及内存规则不会被调用方改写。源码候选待手动实机确认，默认已确认程序保持现有发布版本。

目录准备补测 `characters,lottery`，以不存在的角色导出、物品索引和奖池 JSON 路径准备 2,753 个奖池，通过实际源身份校验；仍读取尚未迁移的角色快捷栏政策。此检查不启动网关或存储。

## 无消费者 JSON 清理

用户明确授权直接删除用不到的 JSON。已检查模块引用、仓库内启动器/GM/小型 profile 的字面引用和动态配置选择；没有按模块清单中的零引用一概删除。外部 `channel_probe.py` 仍消费的账号选项、频道、疲劳、建号、选角和城镇探针配置保留。

删除 24 个已被替代且没有现行代码/测试消费者的文件，共 8,602,344 字节（约 8.20 MiB）。顶层 JSON 从 160 个减少到 136 个，引用清单重新生成；两份交付清单只移除对应的 9 个旧条目，没有全量重生成或收录临时目录。

| 归属 | 删除文件 |
|---|---|
| 旧导出/试验数据（8 个） | `character-rules.odyssey-pilot.json`、`characters.before23.json`、`characters.generated.next.json`、`characters.jobs-release.json`、`characters.skills-release.json`、`dungeons.skycastle-release.json`、`fatigue-items-candidate.json`、`lottery-item-7772.json` |
| 已被后续全量范围替代的逐批 PVF profile（14 个） | `pvf-adventure-candidate.json`、`pvf-closing-candidate.json`、`pvf-cube-candidate.json`、`pvf-fame-candidate.json`、`pvf-layer-revisits-candidate.json`、`pvf-lottery-candidate.json`、`pvf-mine-candidate.json`、`pvf-odyssey-routes-candidate.json`、`pvf-recommended-candidate.json`、`pvf-rewards-candidate.json`、`pvf-roster-backgrounds-candidate.json`、`pvf-script-warps-candidate.json`、`pvf-season-candidate.json`、`pvf-selection-candidate.json` |
| 旧手工奥德赛 profile（2 个） | `repair-profile.odyssey.full.json`、`repair-profile.odyssey.json` |

历史协议/逐批迁移文档中的这些文件名是当时的证据记录，现不作为启动入口。需要复现历史快照时从 Git 恢复对应版本。当前 profile、JSON 回退及现有对照测试仍读取的文件保留；抽奖手工政策仅被历史对照测试使用，未删。

清理后 Go 1.26.0 全量 `go test ./...` 通过，`go vet ./...` 通过，14 项 `test_repair_profile` 通过。`test_pvf_default_launch` 有 5 项既有失败：工作区用户改动已将默认启动改为 JSON，并新增 `--pvf-mode`，旧测试仍按默认 PVF 构造参数/断言。保留用户的 `launch_local.py` 改动，未据旧测试改回启动行为。便携 Python 缺少 pefile 时的测试导入失败已用本任务隔离环境复核；没有修改全局 Python 安装。

已编译 `bin/wireprobe-handoff-source.exe`，SHA256 `43a965b5ea15d355f74228987fec0125545a6558141713294d9ff235a2dd03d2`。当前默认程序 `wireprobe-pvf.exe` 编译前后 SHA256 均为 `3037b4ad3fee6ee15851efa35421b7c8dddf3b13af293391a69d2b0b882200a87`，没有覆盖；该值仅记录工作区实际程序，不替代历史 confirmed baseline。原源码程序备份在本任务临时目录。当前工作区启动设置下，手动验证 PVF 源码候选需显式使用 `--source-build --pvf-mode`；本轮没有启动客户端、网关或数据库。

## 剩余运行依赖与迁移顺序

以下是本轮已读调用路径，不是仅按文件名推断。

| 依赖 | 当前消费位置 | 下一步 |
|---|---|---|
| `pvf-selection-policy.json` | `pvf_selection_boxes.go` / `ImportSelectionBoxes` | 先闭环上述 14 条解析缺口与实际发放边界，再用源类型和源结构自动发现替代 ID 清单 |
| 抽奖三份历史 JSON | JSON 回退及可选基线对照 | PVF 正常运行已解除依赖；历史文件暂供回退/测试，不再人工同步 |
| `pvf-drop-policy.json` | `pvf_drop.go` | 追踪装备选取范围、最大等级与排除项的源依据；保留可追溯的限制原因直到证据闭环 |
| `pvf-box-policy.json` | `pvf_boxes.go` | 确认 COS 材料关联的自动发现，处理容器映射与缺失 stack limit 的客户端依据 |
| `pvf-scene-policy.json` | `pvf_scenes.go` / `pvf_dungeon_maps.go` | 分别核对源出生配置、训练图标记、停用原因和迷宫权重，不能用目录遍历顺序决定入口 |
| `pvf-character-policy.json` | `pvf_characters.go` | 核对源初始快捷栏/命令及客户端布局；避免每职业另维护一套栏位表 |
| `pvf-enhancement-policy.json`、`refine.json` | `pvf_enhancements.go`、`main.go` / inventory | 费用与物品定义已源读；概率、失败规则和锻造仍需源字段或原生机制证据 |
| `pvf-mine-policy.json` 及其历史版本 | `pvf_special.go`、`pvf_odyssey.go`、`pvf_black_purgatory.go`、`pvf_mine.go` | 追踪章节启用、rank 选币与本服概率覆盖；PVF 可表达的数值归还内容文件 |
| `inventory*.json`、`equipment-wear*.json`、`pvf-vault-policy.json` | `LoadBagRules`、`LoadWearRules`、`pvf_equipment_rules.go` | 区分源容量/费用与客户端行布局；布局须用 reader 证据，不能凭 PVF 同名字段猜偏移 |
| `experience.compat90.json`、`drop*.json`、`cards.compat90.json` | `main.go` / progression / loot | 查明当前 115 版本源公式和执行机制，旧兼容公式不作为当前版本事实 |
| 建号、疲劳、世界、出生点规则 | `main.go` 的直接加载 | 从 CHR/ETC/MAP 和原生机制查明，逐项减少手工默认值 |
| 原生封包样本、账号选项、运维连接 | `main.go`、storage、启动脚本 | 依用途整理；这些不是第二份 PVF 内容表 |

实施按一个已闭环的规则族逐批推进。每批先比较当前 PVF 与运行模型，再解除相应 JSON 读取；保留对未知结构的明确拒绝和逐条证据。目录迁移与删除旧导出物放在运行依赖解除、测试改为当前源验证之后。

## 内容版本与存档兼容

服务端之外的内容编辑与发布流程应保存 PVF 版本和回退资源；客户端与服务端由同一次内容发布选择匹配资源，并以只读方式消费。派生索引按源哈希失效并自动重建。

旧角色、任务、物品存档是否能用新 PVF，须逐系统验证。已有 ID 被删除、奖励数量变化、装备定义变化时不能通过修改 checksum 别名掩盖差异。需要迁移时提供可预览、可回滚的兼容步骤和验证，保护已有存档。本批审计没有进行资源发布或存档迁移。

## 复现与验证

在 `server/work/dfo-lan` 内执行配置引用审计：

```powershell
../../../tools/python/python.exe scripts/audit_config_references.py --output .tmp/configs-reference-report.md
```

输出目标必须不存在。只读源码和配置文件名，不输出配置值。

自选类型源审计沿用已有命令，不新增一个 cmd 入口：

```powershell
$env:GOTOOLCHAIN = 'go1.26.0'
go run ./cmd/pvfaudit -selection-scope -pvf-archive .tmp/pvf-source-rules/Script.inner.pvf -pvf-sha256 8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934 -difference-limit 200 -output .tmp/selection-scope-report.json
```

本次真实审计覆盖全部 16,757 条，退出码 2 表示上述解析缺口已经记录，不表示全部可运行。通过 `go run` 执行时 Go 会把程序退出码 2 报告为 `exit status 2`，包装进程可能返回 1；编译后直接运行可读取原始退出码。

新增真实归档测试通过：候选计数覆盖完整、报告限额不改变统计、问题顺序稳定、源索引不被修改、异源索引与负限额被拒绝。使用 Go 1.26.0 执行全量 `go test ./...`、`go vet ./...` 均通过；配置审计脚本在 Python 3.11 上生成的报告与系统 Python 输出一致，`git diff --check` 通过。
