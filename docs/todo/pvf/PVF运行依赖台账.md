# PVF 运行依赖台账（第 0 批）

## 2026-10-03：当前只读入口与分层约定

当前主服务的运行内容准备入口是 `gamedata.PrepareCatalogs`，返回 typed `gamedata.Catalogs`；启动装配在访问玩家存储前完成所选领域的读取、来源校验及跨目录准备，运行消费者使用已准备目录。已经退休的 JSON 内容域缺少原生目录时应明确拒绝，不通过文件存在性或旧路径重新选择内容源。运维、客户端布局、调服策略和不可变历史测试夹具仍有各自边界，不能据此宣称全部 JSON 都可删除。

诊断与导出工具使用 `gamedata.Open` / `gamedata.Source` 作为只读资源入口，再调用领域方法读取同一归档；需要完整运行装配时复用 `PrepareCatalogs`。两者属于现有 gamedata 门面：前者负责工具的源访问，后者负责运行目录组合。工具应按既有来源门禁打开归档、处理读取失败并关闭资源；导出使用明确输出路径，不维护另一套运行 JSON 真源，也不新增平行的 archive opener 或 loader。

| 层次 | 当前职责 |
|---|---|
| `gamedata.PrepareCatalogs` / `Catalogs` | 主服务运行准备、领域选择、来源和跨目录校验、typed 结果及生命周期。 |
| `gamedata.Open` / `Source` | 工具的只读源访问与领域投影门面；复用底层及领域读取实现。 |
| `internal/catalog/pvf` 与现有领域导入器 | 归档目录、文件、字符串及脚本读取和解析；领域语义仍由相应 catalog/inventory/character 等实现。 |

集中入口不要求搬走领域解析器，也不表示全仓已经完成迁移。仍有历史诊断命令直接调用 `pvf.LoadArchive`，其它尚未退休域也保留独立 loader 或旧对照分支，需后续按实际消费者迁移。本轮相关诊断优先复用现有门面；此约定不增加客户端协议、源数值或实机验收结论。

下面保留第 0 批历史审计及随后整合记录；旧快照中的默认模式、文件路径和行号不是当前实现声明，判断现有依赖应重新追踪入口和消费者。

2026-10-03 后续收口已将 progression、materials、periods、skins，以及 Hell Party / Tournament Quest / Tower of Grief / Tower of Dazzlement 覆盖域改为只使用准备好的 PVF 目录，并删除对应 8 个 JSON。其余历史段落仅记录旧调用路径，不代表上述 JSON 仍是当前依赖；当前字面路径清单已重生成，逐域状态见 [单一内容真源改造计划](PVF单一内容真源改造计划.md)。

随后删除了只被测试引用的 `loot.level150.json`；完整历史掉落数据仅保存在 SHA256 校验的测试夹具中。`loot.next25.json` 仍服务 GM/JSON 模式消费者，不能据此批次类推删除。

## 第 0 批历史审计

审计日期：2026-10-02。代码审计基点：HEAD b7a5692；本文件行号均指该快照，实际工作树中其他 agent 已修改 cmd/wireprobe 的行号，集成后须复核。审计范围为源码、默认 profile、启动器、admin/GM 与外部探针。server/work/dfo-lan/scripts/launch_local.py 有用户改动，本表按当前行为描述。本批只更新文档，不改运行代码、数值、客户端、协议、存档或启动行为，也没有启动服务、客户端或数据库。

这是入口调用路径审计，不是单纯的字符串扫描：从实际入口追到 profile/environment/default flags、配置读取，再到目录构造与运行消费者。configs字面引用清单.md 是线索清单；零命中不证明没有消费者。PVF 候选表示代码已接入原生读取路径，不代表源规则完整、客户端语义已由 IDB 闭环或已逐玩法实机确认。

## 入口与选择路径

| 入口 | 当前读取路径与证据 | 结论 |
|---|---|---|
| 根启动游戏.cmd、启动游戏-奥德赛.cmd、启动服务端.cmd | 调用 launch_local.py 并透传用户参数；服务端入口额外传 --server-only。自身没有加 --pvf-mode。 | 当前 launch_local.py 只有显式 --pvf-mode 才选默认 PVF profile（:96-103,168-181）。根入口不带该 flag 时使用 launcher.local.json 的 server_binary，并保留 JSON 配置路径。入口注释声称 PVF 默认与当前实现不符。保留并如实记录此用户改动。 |
| launch_local.py profile/environment | --repair-profile、--json-mode、--pvf-mode 互斥；PVF 模式选 configs/pvf-default.json。repair_profile.py 按 key 类型校验路径、域名和开关（launch_local.py:21,96-112,148-181; scripts/repair_profile.py:11-92）。 | PVF profile 的 binary 为 bin/wireprobe-pvf.exe，DFO_PVF_CATALOGS 列 54 个域，DFO_PVF_VERIFY_BASELINES=0。--json-mode 清理继承的 DFO_PVF_*。 |
| 内层 PVF 准备 | Profile DFO_PVF_ARCHIVE=../client-build/Script.inner.pvf；当 DFO_PVF_CATALOGS 非空时启动器调用 _ensure_inner_pvf（launch_local.py:115-144,219-220）。 | 直读输入是客户端包裹链生成的内层 PVF；启动器对 DFO.exe/sk.dat/外层 Script.pvf 和 manifest 做校验。Profile DFO_PVF_SHA256 为空，未固定到特定哈希。 |
| cmd/wireprobe 网关 | -pvf-catalogs 从 DFO_PVF_CATALOGS 取默认；archive/hash/cache dir 也有环境默认值；main.go:184 调 preparePVFCoreCatalogs（main.go:50-72,184-204; pvf_catalogs.go:125-164）。 | 没选 PVF 域时 preparePVFCoreCatalogs 返回空目录，后续沿 JSON loader/default flag 运行。选择域读取失败或源身份不符时会报错。-pvf-check-catalogs 可在存储/监听之前退出。 |
| 外部 channel_probe.py | launch_local.py 将 profile 环境和所选 binary 交给 server/work/dfo_probe_tools/channel_probe.py。探针构造历史兼容 flags/env、按 binary capability 剪枝、最后以 DFO_SERVER_BINARY 选网关（channel_probe.py:172-200,225-480,481-510,618-620）。 | 探针会条件注入或覆盖配置路径，不是透明 shell。PVF 目录准备后 pvfCoreCatalogs.load* 对已准备投影优先返回原生对象；未覆盖/未选域仍可能走 JSON。应按 profile 与最终剪枝结果逐项检查，不能仅凭字面引用断言无消费者。 |
| admin 与修复命令 | cmd/admin、cmd/initialrepair、cmd/questrepair 都注册 managementdata flags；source 默认 JSON（internal/managementdata/source.go:26-52）。admin 发放在 cmd/admin/main.go:62-104 准备 Awarder；initialrepair/main.go:31-82 依据 source mode 选 characters/equipment reader，但 wear rules 和 character creation rules 始终单独读 JSON；questrepair/main.go:18-37 在 PVF/JSON 间读取 quests。 | admin 默认读 loot.next25.json、inventory.next29.json、equipment.current37.json（cmd/admin/main.go:71-75）。initialrepair JSON defaults 为 characters.skycastle-release.json、equipment.current37.json、equipment-wear.current35.json、character-rules.odyssey-release.json；后两类是槽位/创建行为策略，不是 PVF 内容清单。questrepair 默认 quests.generated.json。三者显式 PVF 均需 archive/checksum；manager flags 不代表默认转为 PVF。 |
| GM Web | gm-tool/scripts/gmweb.py 默认 --catalog-source json；PVF 需要显式 archive 和精确 checksum，并传 drop policy、bag rules、names display overlay（gmweb.py:35-78,118-125）。Go GM 入口在 cmd/gmtool，目录准备见 cmd/gmtool/pvf.go:14-61 与 main.go:153-168,244-269。 | 发布包默认 JSON。PVF 模式从内层 PVF 建物品/装备来源目录；名称文件只作显示，不能决定来源 ID、属性、价格或发放资格。 |
| 离线 profile 检查 | wireprobe -pvf-check-catalogs 或 gmweb.py --check --catalog-source pvf。 | 本次只读审计，未运行这些检查，也未连接存储。 |

## 现存来源管理与绕过点

当前 internal/gamedata.Source 统一拥有只读 PVF archive、source identity 与多种 typed import method；但仅 Characters、World、Quests、Progression、ItemIndex 五个方法内部按 Source.Mode 在 PVF import 与 JSON loader 间双源分派（source.go:267-300）。其他 catalog 方法是 PVF archive import API，不是统一的 JSON/PVF 目录装配入口。因此不可概括为 Source 已统一管理所有双源 catalog。cmd/wireprobe 另构造 54-domain 的 pvfCoreCatalogs，并在 main.go:184 调 preparePVFCoreCatalogs（pvf_catalogs.go:160-214）。多数已准备域再由逐域 load* wrapper 返回投影，否则直接走 JSON loader；部分域有业务代码直调 Source 或 Load*。

审计快照 b7a5692 之后，Phase 01 的源码整合与离线验证已完成：wireprobe 内 `pvf_*.go` 现为 0 个（含测试文件），32 个生产读取文件的职责收拢到 `internal/gamedata/catalogs.go`、`catalogs_items.go`、`catalogs_equipment.go`、`catalogs_scenes.go`、`catalogs_content.go`；目录经 typed `Catalogs` / `PrepareCatalogs` 暴露，由 `main` 装配，运行期适配在 `catalog_runtime.go`。两个预热文件改名归入 `role_detail_prewarm*`。46 个原测试名称全部保留，聚合到 4 个 `gamedata` 测试文件和 `cmd/wireprobe/catalog_runtime_test.go`。PVF 底层文件读取、LIST/COS/脚本解析仍由 `internal/catalog`、`internal/inventory`、`internal/loot` 中的导入器完成；`gamedata` 作为 L4 组合层将按域结果组装成 typed 目录、执行跨目录校验并附接运行期适配，不成为玩法规则唯一实现。该分层符合[服务端依赖契约 §1、§2/R6、§4](../../../server/work/dfo-lan/docs/architecture-contract.md#1-分层)：L4 承担组合与只读投影，玩法事实归领域层。集中读取便于替换实现而不改业务逻辑，不新增运行时来源开关，所选读取失败不隐式切换实现。

Go 1.26.5 候选 build 与 `go vet ./...` 通过；Python 3.11 启动器测试 10/10 通过。全量 `go test ./...` 仍有 4 项失败，均已在原 HEAD 复现：gamedata 中 3 个 warning-only 审计断言（与迁移前 cmd 中同名断言相同）及 cashshop `TestShopPilotPVFCurrentCatalog`。当前 8b2a9f… 内层归档的完整 54 域离线 prepared-check 通过；忽略 memory 统计后报告内容与同一 HEAD b7a5692 源码 overlay 构建完全一致，`storage_accessed=false`、`runtime_started=false`。`NativeLotteryDiscovery` / startup 使用实际归档通过：2,753 个奖池、118 项拒绝，271,492 个权重边界（详运行日志）。`NativeRolePrewarm` 使用实际归档通过：关闭父 Source 后详情仍可读、未知历史模板保留、State 字节不变，且已关闭的 known-view 错误向上传递。未操作玩家数据库或客户端，未替换正式/源码 bin，未实机确认；本批仅收口源码提交。此项完成仅指源码整合和上述离线证据；不代表 configs 剩余内容依赖已清除，也不代表内容范围、数值、客户端协议或存档已验收。`DFO_PVF_VERIFY_BASELINES` 仍只控制历史对照。

集中读取时仍需逐项盘点：旧快照中的 wireprobe `pvfCoreCatalogs` preload / `load*` wrappers、main 中直接 `Source`/`Load*`、通过 `os.Stat` 决定是否装载的可选文件、`channel_probe.py` 对 env/flags 的覆盖，以及 admin/initialrepair/questrepair 和 GM 各自装配入口。已观察的消费者不能据字面引用清单零命中判无；策略/运维文件由目录装配或独立运行策略读取时，需明确各自所有权。

## 规则族台账与字段分类

内容清单决定有哪些 ID/路径/职业/地图；内容值包括源内属性、价格、奖励、权重等；客户端布局包括背包/快捷栏/实例行/请求应答形状；调服策略是运营者选择的范围、概率、禁用或上线策略；运维是路径、缓存、监听；历史对照只校验旧投影，不代表当前运行事实。

### 默认 profile 的策略文件逐字段分类

以下是 pvf-default.json 当前列入 profile 的策略文件及字段级归属。对应数据文件被 profile 当作 required path；“候选源”不等于这些策略字段已经能从 PVF 推导。未列入当前默认 profile 的同名历史/候选文件不据此宣称无消费者。

| 文件和字段 | 分类与当前消费者 | PVF 候选或缺口 |
|---|---|---|
| pvf-character-policy.json：source_checksum | 来源身份 / 存档兼容门禁 | 当前 profile 留空，让源身份按实际 PVF 建立；跨源存档迁移仍未知。 |
| pvf-character-policy.json：initial_skill_slots | 职业到默认快捷栏槽位的客户端布局/策略内容 | 职业/技能源有候选读取，但槽位布局是否由当前客户端定义待 IDB/live 证据。 |
| pvf-character-policy.json：enable_advancement_shortcuts、enable_source_commands | 角色创建/快捷指令上线策略 | 是否为原生开放范围未知；不得仅凭 PVF 职业表自动开启。 |
| pvf-drop-policy.json：basic_equipment_ids | 装备掉落内容白名单（当前 1536 ID） | PVF 装备/任务候选已有 reader，集合与真实可发放范围需逐项匹配。 |
| pvf-drop-policy.json：excluded_loot_ids | 掉落内容排除清单（当前含 6013） | 排除原因是本服边界；需单独保存深渊特殊池和普通掉落的区别。 |
| pvf-drop-policy.json：maximum_loot_grade | 掉落等级范围策略，当前 150 | PVF 有 item/dungeon 数据，但这个运行上限是否应由源自动得到未知。 |
| pvf-enhancement-policy.json：pure_templates、gold.materials、amplify.official.protection_ticket 等 | 强化/增幅允许对象与本服策略值；文档 provenance 内还含待核材料路径 | 成本、材料和强化表有 PVF candidate；成功率/失败惩罚与安全路径混合，需要按字段确认源或运营决策。 |
| pvf-vault-policy.json：source_sha256 | 来源身份保护 | 必须与源目录匹配；不能作为普通内容覆盖。 |
| pvf-vault-policy.json：initial_slots、initial_secondary_slots、verified_slots | 金库容量和客户端布局/准入策略 | verified slots 是人工验证范围；是否来自客户端 layout 或存档能力需 IDB/live 核实。 |
| pvf-content-policy.json：version | 空的旧 schema 壳 | 当前默认 profile 实际把 DFO_PVF_CONTENT_POLICY 指向 pvf-mine-policy.json；只看原默认路径会漏掉真实消费文件。 |
| pvf-mine-policy.json：odyssey_supplemental_items、odyssey_chapter_drops、odyssey_currency.* | 内容 ID 清单、奖励关联、rank 汇率/概率的混合源与运营策略 | 部分源路线/奖励有读者；币值/章节选择是否在 PVF 中完整表达需逐字段对应。 |
| pvf-mine-policy.json：black_purgatory.*、bleeding_mine.no_drop_items | 副本奖励策略、概率和排除内容 | 副本/源 item 可读；目前本服特定的掉率和排除项不能视为 PVF 源真值。 |
| pvf-scene-policy.json：town、area、training_dungeons、disabled_full_dungeons、maze_rates | 入口点、训练地图、禁用范围、迷宫概率；属于内容选择与调服策略 | town/world/dungeon 可从 PVF 读；选择和概率字段需当前客户端条件、实机证据，暂不更改。 |
| pvf-selection-policy.json：templates | 自选盒启用内容清单（2,978 ID） | 自动发现存在结构缺口，当前仍限制列表，未证明完整消费闭环。 |
| pvf-item-shop-policy.json：routes | 527 条 server_shop_id/native_shop_id/script_path 内容路由清单 | SHP 提供商品候选；路由到 NPC/服务 shop 的映射仍是手工维护。 |
| pvf-item-shop-policy.json：purchase_limit_mode | 限购运营策略（当前 disabled） | 当前 PVF 源不自动决定本服购买限制。 |
| pvf-box-policy.json：templates、cos_paths | 2 个礼盒与 COS 源脚本内容清单 | COS material binding 有 PVF reader；这两项仍人工限定范围。 |
| pvf-box-policy.json：missing_stack_limit | 缺少源 stack limit 时的兼容缺省值（1000） | 客户端缺省语义无闭环，不应套用到全部物品。 |
| pvf-box-policy.json：slots | 背包类别与格号范围映射 | 客户端存储/封包布局策略；不是 PVF 内容表，需 IDB/实机支持。 |
| pvf-layer-revisit-policy.json：scenes | 可重复访问场景、保存/恢复/缓存策略 | 地图落点有 PVF candidate；启用场景和状态恢复策略为受验证范围。 |
| pvf-script-warp-policy.json：routes | 12 条经目击转换记录限定的内容路由 | 源脚本可读但服务端转换路径只支持已观察路由；路由表不是完整 PVF 内容清单。 |
| DFO_PVF_LOTTERY_POLICY / pvf-lottery-policy.json：item_pools、equipment_pools | 旧 276/2477 奖池清单；main.go 参数说明该 compatibility flag 当前被忽略 | PVF direct lottery 已从源类型自动发现，不读该文件作为直读清单；旧 JSON fallback / 对照路径仍须审阅。 |

当前默认 profile 列入上表各策略文件，但不列入已弃用的 lottery policy；lottery policy 行说明的是兼容 flag/旧路径。上述文件有内容清单、调服数值和协议布局的混合项。第 0 批只作分类，不删除文件、不迁移字段、不改变任何 ID/范围/数值。

| 规则族 | 消费者、配置字段分类 | PVF 候选 / 已实现与回退 | 缺口及下一批 |
|---|---|---|---|
| 职业、起始技能、成长、技能学习、教程（characters/progression/skills/tutorial） | pvf_characters.go:13-51; pvf_catalogs.go:225-238,310-326。角色/技能 IDs、职业和源成长为内容清单/值。pvf-character-policy.json.initial_skill_slots 是职业→槽位/技能映射（客户端布局与策略）；source_checksum 是存档来源身份。 | ImportCharacters: internal/catalog/characters.go:157，从 list/character.lst 索引的职业 .chr 与 skill/*.lst 投影；ImportProgression: catalog/progression.go:55，读取 character/exptable.tbl、monster/monsterexp.tbl、etc/sptable.etc、n_quest/questparameter.etc、etc/serverparameter.etc、etc/ranksysteminfo.etc；ImportLearningCatalog: internal/character/learning_import.go:112；ImportTutorials: catalog/tutorial.go:152，从 PVF flow/index 投影。Gateway preload 调用见 pvf_catalogs.go:225-238,310-326。 | 初始快捷栏字段是否由当前客户端 reader 决定、全部职业是否匹配未知；角色创建与存档 source migration 仍需专项核对。不可自动改布局或角色身份。 |
| 世界、城镇、副本、地图、特殊地图（world/town/dungeons/training/tutorial/tower/hell/maze/terminal/tournament） | pvf_scenes.go:15-33,53+; pvf_catalogs.go:266-320,439-500。PVF 地图/副本/房间是内容清单和值；pvf-scene-policy.json 的 town/area、training/disabled IDs、maze rates 是入口/运营选择；spawn/world probe flags 是兼容或布局。 | ImportWorldRuntime: internal/catalog/world.go:412，入口 list/town.lst 与 list/dungeon.lst 并跟随 area/map 源路径；ImportTownArea: town.go:26，具体 town/area 由策略传入；ImportRuntimeFullDungeons: dungeon_import.go:15，使用 list/dungeon.lst、list/map.lst 并沿索引定义解析副本；塔/深渊/终场/比赛 overlays 分别由 tower_grief_maps.go:32、hell_party_maps.go:18、terminal_scenes_native.go:20、tournament_maps_native.go:12 import。Gateway 合并见 pvf_scenes.go:53+、pvf_catalogs.go:439-500。 | training、disabled、迷宫权重与客户端条件的等价性未知；移动/出生及特殊地图需 IDB 与 live 闭环。不能扩展范围或改率。 |
| 任务和奖励（quests） | pvf_catalogs.go:275-295; 任务服务在 main.go 按目录装配。Quest IDs、前置、objective、奖励定义是内容清单/值；事件触发、发奖事务和 wire ack 是业务状态/协议。 | ImportQuests: internal/catalog/quests.go:36，从 list/quest.lst 解析每一行引用的任务脚本；任务导入由 Source.Quests 调用。当前 profile 设 DFO_PVF_VERIFY_BASELINES=0；JSON mode 保留旧目录。 | 每种 objective 与奖励投影和客户端完成 reader 的闭环未知；PVF 有字段不等于协议已接线。 |
| 物品/装备/期限/穿戴/随机属性/装备选择（items/equipment/periods/skins/journal/create-cost/equipment-selection/shields/random-options/oath-grades） | pvf_equipment_rules.go:13+; pvf_catalogs.go:210-214,327-408; inventory handlers/flows。LIST 条目、属性、价格、期限、option 是内容清单/值；槽位、装备行、实例字节与 ACK 是客户端布局；equipment-wear*.json 是兼容规则。 | ImportItemIndex: internal/catalog/item_index.go:53，精确读取 list/equipment.lst 与 list/stackable.lst 后解析条目引用；ItemBasics: item_basics.go:43 同样按 LIST 类型构建 item properties/price/material/booster projection；ImportItemPeriods: item_periods.go:39；完整装备投影由 gamedata.Source.Equipment（internal/gamedata/source.go:362）→ inventory.ProjectPVFEquipment（internal/inventory/equipment_full.go:109）构建并校验；ImportRandomOptionData: internal/inventory/randomoption_import.go:115，逐项读取 etc/randomoption 下的 optionnumbering/optionquantity/optiongrouping/optiongroupselection/randomizedoptionoverall{1,2}.etc；其它导入入口在 pvf_equipment_rules.go:13+。 | 全品类穿戴、背包格与实例偏移不得从 PVF 同名字段猜出；本轮没有审读 IDA，因此任何强化/装备实例字节位置均未核实。 |
| 怪物掉落/普通掉落/装备池（loot） | pvf_drop.go:13-47; internal/loot。PVF 掉落组与候选为内容清单/值。pvf-drop-policy.json.basic_equipment_ids（1536 IDs）是内容白名单，excluded_loot_ids 是内容排除清单，maximum_loot_grade=150 是范围策略；drop.compat90.json 是历史公式兼容，不是 115 PVF 源。 | ImportLoot: internal/catalog/loot.go:63，读取 etc/itemdropinfo_monseter.etc、itemdropinfo_common.etc、itemdropinfo_control.etc、dungeonbossdrop.etc、itemdropinfo_monster_hell.etc、etc/dungeondroptablebygroup.etc、etc/dungeondropinfo.cos、list/equipment.lst、list/stackable.lst 与 etc/itemdictionary/itemdictionary.etc；装备来源筛选 ImportEquipmentSelection: internal/inventory/equipment_selection_import.go:56。Gateway 由 pvf_drop.go:13-47 装配。 | 从 PVF 自动发现不得暗中扩大普通掉落范围；6013 排除与深渊专属池要单独保留边界。先证源成员资格再谈迁移。 |
| 强化/增幅/魔法封印/调适/秘宝（enhancements/random-options/fame） | pvf_catalogs.go:210-214,398-408; pvf_awakening.go、pvf_sole.go、equipment handlers。PVF 成本/材料/规则值为内容；pvf-enhancement-policy.json 中模板清单、成功率/失败惩罚、安全路径是策略或未闭环值。强化状态在客户端行/存档中的精确 byte/offset 本轮未读权威证据，记为未知。 | ImportEquipmentAwakeningRules: internal/catalog/equipment_awakening.go:250，原生 COS etc/115lvability/equipmentawakeningoptionsystem.cos；ImportEquipmentAwakeningOptions: 同文件:535，etc/115lvability/equipmentawakeningoption.lst；ImportSoleEquipmentRules: internal/catalog/sole_equipment.go:130，etc/115lvability/soleequipmentsystem.cos；ImportEnhancements: internal/inventory/enhancement_import.go:129；ImportRandomOptionData: internal/inventory/randomoption_import.go:115；gateway 入口见 pvf_awakening.go/pvf_sole.go/pvf_equipment_rules.go。 | 安全材料方向、成功率、失败结果及各请求分支需逐字段 IDB/live 证据；特别是装备实例字段偏移目前未定位。保留已确认的用户数值，不以本轮审计更动。 |
| NPC 商城/现金商城/价格/购买路由（prices/cashshop/item-shops） | pvf_commerce.go:79+; pvf_item_shops.go:13+; main.go commerce init。原生商品/价格/货币为内容清单/值；pvf-item-shop-policy.json.routes 的 527 条映射为路由清单，purchase_limit_mode 为策略；shop release env 是发布选择。 | ImportItemShops: internal/catalog/item_shop_native.go:49，先读 list/itemshop.lst，再解析行引用的 itemshop/*.shp；ImportShopPrices/ImportItemMaterials: catalog/commerce_import.go:22,78；Cera Shop ImportPilot: internal/cashshop/pvf_catalog.go:26，入口含 etc/(r)cerashop.etc 与 list/stackable.lst。 | routes 是否可全量从当前源关联 NPC/shop、替代货币和限购语义仍未知；不得扩大购买范围。 |
| 礼盒/抽奖/自选礼盒（boosters/boxes/lottery/selection-boxes） | pvf_boxes.go:14-75; pvf_lottery.go:23-87; pvf_selection_boxes.go:11-47。pvf-box-policy 当前指向的 COS 路径为 live/else/univ/2024/0514_radianttreasurebox/radianttreasurebox.cos 与 live/else/univ/2025/0318_newrandombox/radianttreasurebox.cos；templates 是礼盒内容清单；missing_stack_limit/slots 是缺源兼容或客户端布局。pvf-selection-policy.templates 是 2,978 ID 内容白名单；抽奖 policy 池清单不作为 PVF 直读清单。 | ImportBoxes: internal/inventory/box_native.go:25；DiscoverLotteryTables: internal/catalog/lottery_native.go:90 及 catalog/lottery_discovery.go；ImportSelectionBoxes: internal/catalog/selection_boxes_native.go:48，使用原生物品索引与策略 ID list。其主要 item entry 来源为 list/stackable.lst。 | 自选类型存在重复分类、未建模、拒绝项，详主计划；缺口未清前不可扩清单。堆叠上限/槽位要客户端证据。 |
| 冒险团/赛季/推荐/背景券/奥德赛及特殊奖励（adventure/season/roster/odyssey/attunement/black-purgatory/mine/apocalypse） | pvf_catalogs.go:439-509; pvf_adventure.go、pvf_recommended.go、pvf_season.go、pvf_roster_backgrounds.go、pvf_odyssey_routes.go、pvf_special.go。 | ImportAttunementRewards: internal/loot/attunement_source.go:14，自动扫描 etc/rewardboostinfo/**/*.ctp；ImportApocalypse: internal/catalog/apocalypse_import.go:10；ImportOdysseyGrowth: catalog/odyssey_native.go:15，contents/2026/aradodyssey/etc/aradodyssey.etc；ImportOdysseyChapters/JournalRoutes: odyssey_journal_parse.go:13 与 odyssey_routes.go:51，contents/2026/aradodyssey/etc/aradodysseyjournal.cos；ImportBleedingMineRewards: internal/loot/bleeding_mine_native.go:17；ImportBlackPurgatory: black_purgatory_native.go:26。 | 特殊奖励与 rank 概率/附加物品仍可能由 pvf-mine-policy.json 供给；文件名/入口只提供 importer 实际调用，不足以证明每个业务范围正确。概率、黑名单、章节范围须分字段保留 unknown 或运营决策。 |
| 背包容量/堆叠/强化费用等兼容和运营策略 | main.go flags :54-72,86-151; inventory/workflow handlers。容量、槽位、ACK shape 是布局；免费疲劳/掉率、release flags 是运营决定；cache/archive path 是运维。 | pvf-vault-policy.json 提供 verified slots/初始容量；pvf-box-policy 提供 slot maps/默认 stack limit；profile 保留 DFO_FATIGUE_FREE、DFO_ATTUNEMENT_REBALANCE 等数值开关。 | slots/初始容量是否为客户端版本事实需逐字段 IDB；不能归成 PVF 内容而自动删除。 |
| 存档源身份与版本 | pvf_characters.go:13-51; main.go source selection; internal storage/character identity checks。source_checksum 是数据身份，不是普通策略。 | 运行选 PVF 时检查源与角色 catalog/存档契约；profile checksum 为空时从实际源建立身份。 | 跨源角色/物品兼容、角色 ConfigVersion 更新和迁移边界未知；禁止改 alias 来掩盖源差异。 |
| 内层归档、缓存、baseline | launch_local.py:115-144,219-220; main.go:66-72; pvf_catalogs.go:160-214。archive/cache paths 为运维；DFO_PVF_VERIFY_BASELINES 只选历史对照。 | Direct mode 从 PVF 投影；verify-baselines 为 0 时不打开所选域历史 JSON。设置非 0 后部分域会打开 JSON 比较；结果存在 warn 与 error 两类。 | baseline 审计逻辑与运行输入需保持分离；JSON mode、baseline、策略文件不能统称“无消费者”。缓存/发布策略不在本批改。 |
| 登录响应、频道配置、账号选项、手工 spawn、探针样本 | channel_probe.py:225-480,481-620; cmd/wireprobe/main.go flags/loaders。按字段归客户端协议/状态布局、运营策略或运维；配置路径本身不能证明是游戏内容。 | 一些世界/频道/疲劳规则可选 PVF 源；登录 response bytes、账号选项模板及运维连接值仍为独立本地输入。 | 当前 IDB 对各固定 payload 与 probe-only paths 的闭环范围未知；probe 会动态拼路径，须跟运行条件审计。 |

## 下一批建议与证据缺口

1. **Phase 01 源码整合与离线验证完成**：最终结构、底层解析职责、架构契约、build/vet/test/Python 状态、54 域 prepared-check、Lottery 与 RolePrewarm 真实归档结果及未实机/未换 bin 的限制，见本节前述记录。读取错误不得隐式切实现；Baseline compare 与运维/玩法策略不混入目录 facade。本结论不表示 configs 剩余内容依赖已清除。
2. 审计高优先级手工内容清单：pvf-drop-policy.basic_equipment_ids、pvf-box-policy.templates/cos_paths、pvf-selection-policy.templates、pvf-item-shop-policy.routes。先比较源字段及客户端消费证据，不增删 ID、不改变数值；证据不足继续保留。
3. 逐字段拆分成功率、掉落率、迷宫选择、stack limit、背包槽位与快捷栏：标注 PVF source field、IDA reader、运营决定或 unknown。只有闭环字段才能进入后续迁移。
4. 复核探针实际能力剪枝后的 flags/env 与 PVF mode，单独核验 admin/GM 默认 JSON 路径及其显式 PVF 输入。
5. 运行依赖解除后再决定历史 JSON 归档；绝不按字面引用零命中删除文件。

未取得的证据：当前客户端 IDB 对所有背包格/快捷栏/礼盒缺省堆叠上限/地图选择/购买与奖励范围的逐字段对照；PVF 原文与每份人工清单的完整关联；当前 profile 下用户手动 live 对每个规则族的覆盖。未知保持未知；不从旧实现、文件名或注释推断。

## 手动验收步骤（后续批次执行；本轮未执行）

1. 用户手动运行 启动服务端.cmd --pvf-mode --source-build，核对最终二进制、profile、PVF 域、archive SHA、准备日志。根入口单独运行时默认仍是 JSON 路径。
2. 用户自行打开客户端，仅验证已指定的一个规则族；记录 run dir、网关日志、PVF source SHA、准备/拒绝信息和对应请求。不让 agent 无人值守操作或跑图。
3. 每批只验证一个假设，并在账号/角色/物品数据上对照；有角色存档字段时覆盖旧角色重登/重选，确认未丢未知字段。
4. JSON 对照须单独使用 --json-mode 并记录实际 paths；baseline warning 不能替代 PVF 源和客户端运行证据。
5. 若 GM 发放列为该批目标，可先用 admin -catalog-source pvf -pvf-archive ... -pvf-source-checksum ... -check-catalogs 做不访问存储的准备检查；实际发放另行由用户手动指定物品与角色。本轮未连接玩家数据库。
