# PVF 直读实施计划

更新日期：2026-10-01。目标是在 Go 服务端尽可能将所有 PVF 有真源的 JSON 数据改为从 PVF 直接读取，同时保留当前玩法行为、玩家存档和回退能力。范围包括运行配置、嵌入程序的导出 JSON、旁路补充目录和管理工具；四个首批目录只是起点。只有 PVF 未提供的规则、服务端策略和运维配置保留独立配置。实施逐领域切换，不整文件覆盖参考合并包。

## 数据和兼容边界

PVF 提供物品、职业、技能、任务、地图和源规则表；服主倍率、白名单、功能开关及兼容公式作为独立策略配置保存；PostgreSQL 中的角色、任务进度、物品、金库和事件回执继续使用现有结构。存档中的 JSONB 与测试样本不属于待删除的 PVF 导出物。

分类以字段来源为准：一个 JSON 同时包含 PVF 表和本服策略时，迁移其中的 PVF 表，只保留独立策略。缺少 Go 导入器意味着需要补实现，不能据此归类为“PVF 没有”。保留策略不要求顺便将 JSON 全部转换为 TOML；配置格式调整与资源直读分开实施。

只修改服务端与准备工具，不修改客户端资源、DLL、协议布局或玩家数据库。保持已实现的冒险团、装备图鉴、骑士盾牌、黑暗武士组合技能和觉醒行为。实机操作由用户执行。

## 已核实的起点

- Go 已支持当前 inner PVF、Token、CTP 与角色、世界、任务、经验等导入器。
- 当前 `server/work/client-build/Script.inner.pvf` 为 760530763 字节，SHA256 为 `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`，与 `characters.skycastle-release.json` 的源一致。
- 客户端外层 `client/Script.pvf` 为 760531281 字节，SHA256 为 `5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`。按现有解包链生成的实际 inner PVF 为 760531281 字节，SHA256 为 `be95d64ee120248ae503194d2f61743ef74409a8ff999a4a986ca2e0bccf69b0`。其来源与历史 `7ef2db59…`、合并包 `b2b503…` 均不同，不能自动绑定同一存档版本。
- 参考合并包要求 `b2b503…`，缺少多个被调用的导入器和 TOML 依赖；包内主入口也缺少当前主干的部分接线。
- 包内清空存档后新建角色的记录不能证明已有存档兼容。

## 目标结构

统一目录加载模块管理资源来源、归档生命周期和领域目录。PVF 的文件索引、解密解压与 Token 层复用现有实现；领域导入器输出现有目录类型，业务服务不依赖磁盘格式。游戏入口与管理工具逐步共用该模块。

迁移期间显式选择 JSON 或 PVF。PVF 读取失败或来源不符时拒绝加载，禁止静默回落。切换前对现用 JSON 与直接导入结果按 ID、字段、引用和原始脚本指纹比较，报告所有差异。元数据路径、加载时间、缓存计数等不作为玩法差异；列表顺序与 Token 值保留。

最终 PVF 模式不以导出 JSON 作为启动前置条件；当前候选的 JSON 对比门禁只在过渡期使用。源表通过审计后，JSON 可以保留为离线回归样本或历史归档，不继续充当运行真源。

## 全量 JSON 迁移清单（2026-10-01 复核）

本次逐项检查 `cmd/wireprobe/main.go` 的显式加载、相邻文件自动加载、环境变量旁路，以及 `internal/` 的文件读取和嵌入资源；同时检查 `cmd/admin`、`cmd/gmtool`、`cmd/initialrepair`、`cmd/questrepair` 和 `gm-tool` 启动/代理的目录依赖。下表按数据族统计；同族的 `next*`、`candidate`、`release`、`generated` 等版本文件不作为多个独立系统计数。`configs/` 的文件存在不等于当前运行已启用。

状态“有库导入器”只说明存在可复用解析代码，仍需有效目录核对和入口接线；“需提取/补齐”包括从现有 Go 导出命令或 Python 导出脚本抽取规则，不能直接复制参考包缺失的函数名。

| 数据族 / 现有 JSON | PVF 真源或导出依据 | 迁移范围与当前工作 |
| --- | --- | --- |
| 职业 `characters.*` | 职业脚本及关联技能表；`catalog.ImportCharacters` | 有库导入器；先解释默认快捷栏、命令和成长投影差异；创建策略独立 |
| 世界、城镇 `world.*`、`town.*` | 世界/城镇/NPC/地图列表；`ImportWorldRuntime`、`ImportTownArea` | 世界已确认；城镇已接`town`离线候选，38/0及7个可走矩形完整一致，出生点独立策略保留 |
| NPC 传送 `npc-teleport.generated` | `ImportNPCMoves` 读取原版移动表 | 已随世界候选直读；启动基线仍读取该 JSON 供迁移审计，业务使用 PVF 结果 |
| 任务 `quests.*` | 原版任务列表及脚本；`ImportQuests` | 已接同源候选；还需取消过渡期 JSON 门禁依赖 |
| 经验 `progression.*` | 原版经验阈值、怪物经验及源倍率表；`ImportProgression` | 已接同源候选；`experience.compat90` 的兼容计算策略单独保留 |
| 技能 `skills.*` | `skill/<职业>skill.lst`，部分职业回退 `<职业>.lst`，关联 `.skl` | 已提取共享学习导入器并接入 `skills` 候选；3224条与生效next27完整一致；组合/预设/保存业务保持；不存在 `list/skill.lst` |
| 物品索引 `items.index`、材料 `item-materials` | `list/equipment.lst`、`list/stackable.lst` 及 `.stk` | 共享索引已接 `items` 同源候选，599771条完整比对零差异；背包补充、箱子奖励分类、商城分类复用；材料规则和管理工具消费者待迁移 |
| 时限和外观 `item-period-tags`、`skin-storage-items` | 物品期限、外观登记与 skin 列表/脚本 | 已接 `periods`、`skins` 同源候选，完整目录比对零差异；保留现有永不过期策略及127条缺失skin的拒绝边界 |
| 装备 `equipment.*`、`equipment-full.index` + `.data` | `list/equipment.lst` 及 `.equ` | 全量424216条已确认；普通/任务选择已接`equipment-selection`候选，3174行/2794行掉落池完整一致；基础1536个ID白名单为独立策略，1638项任务新增装备由PVF推导 |
| 骑士盾牌 `equipment-knight-shield.*` | `etc/character/knight/shieldwindownewdata.etc`、盾牌 `.equ` | 已接`shields`离线候选，25面盾及窗口行完整一致；职业、槽位和6面任务盾拒绝边界保持 |
| 随机词条 `randomoption.*` | `etc/randomoption/` 的编号、数量、分组、组选与 overall 表 | 已接`random-options`离线候选，4档、246数量行、17组、438选择及246成本行完整一致，6份源脚本哈希也参与核对 |
| 装备图鉴与生成成本 `equipment-journal.generated`、`equipment-create-cost.generated` | `contents/2025/equipmentsetjournal/etc/equipmentsetjournal.cos` | 已接 `journal`、`create-cost` 同源候选，5个分类/9组成本完整比对零差异；执行开关保持独立 |
| 誓约档位 `oath-grades` | `.equ` 中的誓约/引子等级与稀有度 | 已接`oath-grades`离线候选，189件完整一致；诊断启用开关和指定档位不随迁移改变 |
| 普通掉落 `loot.*` | `etc/itemdropinfo_monseter.etc`、`etc/itemdropinfo_common.etc`、物品源脚本 | 已接`loot`离线候选，1022种物品、1221组和281个副本索引完整一致；等级上限及排除6013为原策略，兼容公式不改变 |
| NPC 价格 `shop-prices`、材料商店 `itemshop-*` | 物品价格字段、`list/itemshop.lst` / `itemshop/**/*.shp` 与 `[need material]` | `prices` 599682条、`materials` 14211条已接候选并完整一致；商店绑定遇到同ID多`.shp`冲突，保留JSON，须闭环NPC/客户端开店引用 |
| 商城 `cerashop`、`shop-purchase-pilot`、`shop-vault-release` 等 | `etc/(r)cerashop.etc`、商品关联 `.stk` / `.equ` | `cashshop.ImportPilot` 已存在；源商品、价格、条件直读；已开放商品、特殊交付与购买试验开关独立 |
| Booster、抽奖、选择箱 `booster-catalog`、`lottery-item-pools`、`lottery-equipment-pools`、`selection-boxes-*` | `.stk` 的 booster/select/lottery 字段，`etc/dungeondroptablebygroup.etc` 及装备组 | `boosters`已提取共享解析并接候选，42504条奖励池完整一致，未知项不扩大执行；抽奖和选择箱仍待提取与核对 |
| 独立礼盒 `boxes` | `radianttreasurebox.cos` 等 COS 表 | 可迁移但待绑定完整源路径：当前 JSON 只有 `.cos.txt` 文件名，当前 PVF 有两个同名版本；须按物品脚本引用/内容指纹确认，不按 basename 任选 |
| 强化/增幅券、增幅书、附魔 `reinforcement-tickets`、`amplify-tickets`、`amplify-grimoire`、`enchant-beads` | 对应 `.stk` 类型、作用条件、等级、成功率或附魔能力字段 | 第三批`enhancements`已由用户确认，1196/1629/433/4846种；普通券保留源补充的926个期限头，原实例期限边界保持 |
| 强化费用 `reinforcement-gold` | `etc/upgrade.etc` | 第三批已确认，255级源费用直读；成功率、失败、安全补正和材料选择独立策略保留 |
| 增幅费用 `amplify-upgrade` | `etc/amplifyupgrade.etc` | 第三批已确认，255级普通/安全费用直读，概率和失败策略保留 |
| 金库 `vault.generated` | 账号金库 `etc/accountcargo.etc`；角色金库容量来自客户端分析 | 已接`vault`离线候选，账号60级门槛和40档费用完整一致；角色金库容量与客户端存档来源保留在独立策略 |
| 副本主目录 `dungeons.*` | `list/dungeon.lst`、`list/map.lst` 及 `.dgn` / `.map` | 已接`dungeons`离线候选，3200副本/18387地图完整一致；10个当前解析器新增接受的副本按策略禁用，4个训练场独立合并 |
| 教程 `tutorial-routes.*`、`tutorial-dungeons.*`、训练场 | 职业起始路线及源副本/地图 | 16条起始路线已确认；`tutorial-dungeons`15副本/65地图、`training-dungeons`4副本/7地图离线完整一致 |
| 副本覆盖 `terminal-scenes`、`layer-revisits`、`tournament-quest-maps`、两座塔、`hell-party-maps` | 原 `.dgn` / `.map` 引用、`list/cinematic.lst` 与源场景 | `dungeon-towers`悲叹100层/100地图、眩惑33副本/56地图及`dungeon-hell`55张地图已接候选并完整一致；1个缺失深渊源引用仍拒绝。终场/分层重访/武斗大会仍待迁移 |
| 迷宫概率 `dungeons.maze-chance-rates` | `.dgn` 的 `[maze chance rate]` | 已接`dungeon-maze`离线候选；源概率和脚本哈希直读，原`[992857,7143] → [980000,20000]`覆盖写入独立场景策略 |
| Odyssey 成长、章节、武器箱 `odyssey-growth-*`、`odyssey-chapters-*`、`odyssey-weapon-box-*` | `contents/2026/aradodyssey/etc/aradodyssey.etc`、`aradodysseyjournal.cos`、关联礼盒脚本 | 已接5项奥德赛离线候选中的成长/章节/武器项，50副本、3赠品及毕业礼盒、7章15奖励模板、85组武器选项及运行派生索引完整一致；毕业主线与源顺序保持 |
| Odyssey 掉落 `odyssey-chapter-drop-*`、`odyssey-currency` | 章节最后副本、奖励模板与货币 `.stk` | 已接odyssey-drop与odyssey-currency候选，7行章节掉落及2种币完整一致；章节2/7停用、概率及rank选币为独立策略，未恢复为另一套概率 |
| 天启与军团 `apocalypse.generated`、`legion-contents.generated` | `contents/2026/apocalypse/etc/apocalypse.ctp`、`dungeonskillinfo.ctp`、`contents/system/legionsystem/legionsystem.cos` | 天启apocalypse候选已接，65主记录/14职责/6阶段2190秒完整一致；军团ImportLegionContents已存在，核查无生产消费入口，不随迁移新增玩法启用 |
| 调律 `attunement-rewards.generated` | `etc/rewardboostinfo/skyofathousandseasofborder/{unique,legendary,epic}.ctp` 等 | attunement候选已接，4副本/126奖励模板/5优惠券行完整一致；源CTP按dungeon声明绑定，调参深复制，未知隐藏中间字段与Omen边界保持 |
| 赤红铁矿 `bleeding-mine-rewards` | `contents/2025/bleedingmine/etc/bleedingmine*.ctp`，奖励袋/智能掉落组/合成字段 | bleeding-mine候选已接，12阶段/12领主/3难度、117容器及1782物品完整一致；35负数空奖签、合成机会与失败dummy排除保持，CTP trailer池边界回归已验证 |
| 黑鸦 `black-purgatory-rewards` | `etc/dungeonspecialreward.etc`、`etc/itemdictionary/customroutingwaygroup.cos`、`customroutingway.etc` | black-purgatory候选已接，5普通/1仅记录VIP分支、208史诗/35神话/135腐蚀产物完整一致；10%/0.1%/1%本服策略独立，来源元数据边界见第五批文档 |
| 旁路无色小晶块及单物品 JSON | `DFO_CLEAR_CUBE_SOURCE` 的物品3037和其他已引用 `.stk` | clear-cube候选已接，3037完整源Token/哈希及原存储零值投影一致，无所选导出JSON依赖；其它单物品继续随对应源领域审计 |
| GM 名称与筛选 `gm-tool/configs/names.client`、`equipment.slots`、重复职业/经验/物品目录 | `string/*.uv.str` 的名称/品级/职业文本，`.equ` 的部位和最低等级 | 提取 `cmd/gmtool/names.go` 的文本读取及现有装备解析；GM 与游戏共用源目录，不再复制一套导出 JSON；固定界面属性键的中文对照仍是工具映射 |

GM 包内另有 `set_items*.json`、`avatar_sets.json`、`set_display_names.json`、`equip_whitelist.json`，列入补充审计：套装/装扮成员如来自 PVF，应合并到装备与分组导入器；精选套装范围、搜索别名、显示名覆盖和发放白名单属于工具选择。现有文件缺少统一源路径/指纹，本次不将其整体宣称为已确认可直读，也不整体认定为“PVF 没有”。需逐字段核对前端消费入口与原脚本。`gm-tool/backups/` 是玩家操作备份，应保留，不作源规则迁移。

### 嵌入 Go 程序的源 JSON 也必须迁移

当前生产代码有以下 8 个嵌入 JSON；它们不会随 `configs/` 入口切换而自动消失。

| 文件 | PVF 来源与实施边界 |
| --- | --- |
| `internal/adventure/rules.json` | `etc/adventurersystem/adventurersystem2018.etc` 及脚本中引用的材料/物品；移植 `export_adventure_rules.py` |
| `internal/adventure/recommended_rules.json` | `event/conditioneventchkdungeon.evt`、`list/worldmap.lst` 和关联 worldmap；保留源缺失旧区域的明确排除 |
| `internal/adventure/season_rules.json` | `contents/system/seasonlevel/main.cos`、`etc/costs.ctp` 及奖励物品；移植 `export_season_rules.py` |
| `internal/character/fame_rules.json` | `etc/famevalueinfo.etc`、套装分数/分组表及关联装备；移植 `export_fame_rules.py` |
| `internal/character/odyssey_journal_routes.json` | `aradodysseyjournal.cos` 的章节回城目的地；复用章节导入器 |
| `internal/rosterbg/tickets.json` | 背景券 `.stk` 与 `etc/selectcharacterver2/selectcharacterver2.etc`；定义和期限迁移，实际授权状态仍保存数据库 |
| `internal/dungeon/script_warp_routes.json` | CMT → passive-object action → dungeon warp 源链；分离源引用/坐标与客户端实测 transition record |
| `internal/dungeon/forced_script_warp_routes.json` | 同上；保持已确认的非 Odyssey 强制跳转条件，未闭环路线不自动启用 |

`internal/dungeon/skycastle_scene_routes.json` 也列入后续文件引用清理；本次未发现其生产代码加载入口，不能因为它位于 `internal/` 就将其算为已启用嵌入数据。测试目录中的 Token 和原生协议向量继续作为验证样本。

### 应保留或继续取证的字段

- 数据库连接、频道地址/端口、运行输出、诊断 response/fixture、客户端 option 模板及覆盖属于运维或协议数据，不是 PVF 规则表。
- `character-rules.*`、`world-probe`、`town-entry-probe` 的创建/入口选择，`fatigue-probe` 的每日上限/时区/重置小时，`inventory.*` 的客户端容器布局和缺省堆叠策略、`equipment-wear.*` 的客户端槽映射，现有依据不能以 PVF 表替换。字段若后续找到真源，再逐项迁移；物品自己的 stack limit 仍直读 PVF。
- `experience.compat90`、`drop.compat90`、`cards.compat90` 保留兼容公式；其中业务使用的原版经验/金币/物品池通过共享源目录供给，不重复留在策略配置。
- 普通强化/增幅成功率、失败惩罚及 `refine` 的现有服务端公式属于现有外部/服主依据。当前归档按 `refine` 文件名检索 `.etc/.cos/.ctp` 没有命中，只能说明尚未定位同名源表，不能证明所有表中都不存在此规则。锻造材料自身仍从物品索引读取。
- 特殊副本的执行开关、本服概率、白名单与实机确认的协议记录保留。源权重、物品候选、地图条件和费用不得以“特殊规则”名义一并排除。
- 玩家状态 JSONB、历史事务回执和原生向量保持现有存储/验证用途。本次不调整其格式。

### 本次目录取证与下一批顺序

对当前实际客户端 inner PVF `be95d64e…` 做只读目录检查，确认上述装备、商城、升级、骑士盾牌、图鉴、冒险团、赛季、Odyssey、军团、天启、调律和黑鸦关键源路径存在。已读出 `etc/accountcargo.etc` 的等级60和40行升级费用，与当前 JSON 的账号金库字段对应。路径存在只证明有迁移来源，不代表所有领域字段、存档版本和运行行为已通过核对。

礼盒检索结果为 `live/else/univ/2024/0514_radianttreasurebox/radianttreasurebox.cos` 与 `live/else/univ/2025/0318_newrandombox/radianttreasurebox.cos` 两条；下一步必须定位物品引用，不能选择更新时间更晚者作为推定真源。技能已确认 `skill/swordmanskill.lst` 存在，而 `list/skill.lst` 不存在，应遵循现有职业索引解析。

实施顺序调整为：①补齐职业/世界有效目录；②建立共享物品索引和按 ID 装备读取；③接入 NPC 价格、商店、礼盒、强化/增幅材料与费用；④副本覆盖和特殊奖励；⑤迁移上述嵌入 JSON，并接入管理/修复工具；⑥各已迁移领域去掉 JSON 启动依赖，最后在存档与用户实机验收后调整默认值。每批仍以来源、字段、业务行为和存档兼容为门禁。

## 实施阶段

| 阶段 | 交付内容 | 完成门禁 |
| --- | --- | --- |
| 一 资源与目录审计 | 实施计划、统一来源入口、只读对比命令、领域差异报告 | 工具不连接玩家库；来源不符有明确错误；记录实际资源和差异 |
| 二 首批目录切换 | 职业、世界、任务、经验逐领域接入；保留策略和源授权条件 | 字段差异全部解释；不丢 NPC 移动、阶段 NPC、默认快捷栏和觉醒数据；旧版本标识兼容 |
| 三 物品与技能 | 物品索引、装备定义、学习目录、价格、礼盒、选择箱、抽奖和强化导入器 | ID 路径使用源列表；同源字段核对；缓存有界；掉落选择策略不扩大 |
| 四 副本及其他规则 | 主副本、教程子集、覆盖地图、剧情与特殊奖励表 | 所有现用覆盖表分类；既有修复用例保留；普通副本不被教程路线限制 |
| 五 工具和启动 | admin、启动编排及其他消费者统一加载；策略独立 | PVF 模式不依赖已迁移导出 JSON；记录模式、资源、耗时和内存 |
| 六 默认切换与清理 | 用户实机验收后默认 PVF；清理弃用文件和历史参数 | 旧角色回归通过；回退实测；更新 CHANGELOG 和 confirmed baseline 后仅提交本任务文件 |

## 资源与内存方案

首先支持明确指定的 inner PVF，显式设置容量上限，不使用 reader 的 512 MiB 默认上限读取当前约 725 MiB 的归档。资源准备流程使用现有解包链的证据，写入服务端生成目录，绝不覆盖客户端。

首批只读审计使用单个归档实例、逐领域处理与临时缓存释放，避免同时装配所有 JSON 和 PVF 目录。全量装备后续按 ID 解析并使用有界缓存。记录启动耗时、峰值和稳定内存后，确定是否进一步采用磁盘分块读取。不能以加载顺序或增加 GOMEMLIMIT 代替缓存设计。

## 存档与回退方案

仅替换相同源资源的读取方式时保持现有 `config_version`。若实际资源或领域语义变更，先列出角色、任务、金库、账号容器和事务校验的版本字段，再设计有审计记录的事务迁移及逆向回退。历史事件来源不得批量篡改，不清空数据库或重建角色。

首批保持默认 JSON，PVF 作为明确的候选路径。未通过语义门禁的领域不得装入运行服务。回退只切换本任务的加载方式；不得覆盖其他改动，不替换39版归档基线或运行中的服务。

## 验证安排

普通测试使用小样本覆盖模式选择、来源错配、解析失败和字段差异。真实 PVF 对比由独立命令串行执行，输出结构化报告。代码变更后运行 `go test ./...` 与 `go vet ./...`；全量 PVF 测试单独串行，避免并发复制归档。

存档验证使用 PostgreSQL 临时库或临时 schema，核对旧角色、任务、背包、穿戴、金库和回执保持不变。实机清单包括旧角色入场、任务继续、技能保存、装备穿脱、金库存取、商城购买、礼盒使用、普通副本与教程路线。测试报告不替代用户实机确认。

## 当前进度

- [x] 分析当前源码与参考合并包，确认资源哈希和缺失实现。
- [x] 创建本实施计划。
- [x] 复核全量 JSON 数据族、8个嵌入 JSON、旁路物品和 GM 目录依赖；按字段区分 PVF 表与策略，确认当前账号金库源表。
- [x] 实现 `internal/gamedata` 来源入口、字段对比与 `cmd/pvfaudit` 只读审计。
- [x] 对历史 inner PVF 和当前客户端实际 inner PVF 分别执行首批四个领域审计。
- [x] 接入同源任务、经验的显式候选入口，打开数据库前执行来源和字段门禁。
- [x] 完成全量 Go 测试、vet、准备工具测试和不连接数据库的真实归档候选入口测试。
- [x] 补齐世界运行依赖、稳定 NPC 地点索引顺序；单独核验新增阶段投影并接入 `world` 同源候选。
- [x] 共享物品索引、按ID装备直读、时限/外观/图鉴/成本候选；599771个索引与424216个完整装备定义全量同源核对通过。
- [x] 准备隔离的9领域实机候选及启动profile，不替换日常程序和默认启动入口。
- [x] 用户确认九领域实机正常，核对实际启动/业务日志并更新CHANGELOG及confirmed baseline；本批按任务范围提交收口。
- [ ] 处理职业投影差异；区分本服默认技能栏与源技能定义后开放。
- [ ] 将实际客户端资源来源与旧存档版本安全对接，再开展存档和实机回归。

此文档记录实施候选进度，不将尚未实机确认的行为标为 confirmed baseline。

## 首轮审计结果（世界补齐前）

| 领域 | 历史源与有效 JSON 的差异数 | 当前客户端源与有效 JSON 的差异数 | 当前运行范围 |
| --- | --- | --- | --- |
| 职业 | 341 | 341 | 保留 JSON；默认快捷栏、转职快捷栏、命令、成长和预设技能投影需分类 |
| 世界 | 1225 | 1225 | 保留 JSON；包含阶段地图、旧字段、NPC 移动及运行索引差异 |
| 任务 | 0 | 0 | 仅同源历史归档候选开放，2844 个任务定义 |
| 经验 | 0 | 0 | 仅同源历史归档候选开放，150 个累计经验阈值 |

差异数指对比器报告的值变化、缺失键或数组长度变化，不代表角色或资源文件数量。运行索引虽然标注 `json:"-"`，仍参与核对，防止只看 JSON 字段而遗漏 NPC 传送功能。列表顺序保持有意义，不将不同顺序静默视为相同。

当前客户端的任务和经验字段一致，但归档来源不同，所以报告仍为未通过来源门禁；不能凭字段一致扩大为全资源或旧存档已兼容。职业差异主要包括技能命令175项、转职快捷栏61项、初始快捷栏54项及成长、预设、转职技能集合各17项；首先区分私服默认策略与导入器新投影，不直接覆盖已确认布局。

审计时测得领域处理后的 Go heap 约3.2至3.4 GiB，该采样不等于系统峰值或 RSS。任务和经验候选入口约12秒准备完成，归档释放及 GC 后此独立测试保留 heap 为22197816字节，约21.2 MiB；完整游戏服务仍需另测稳定内存。

生成的完整差异报告与 inner PVF 位于 `server/work/dfo-lan/.tmp/`，不进入 Git。关键命令和结论保存在本计划，临时资产可以重新生成。

## 第二批：世界与 NPC 移动候选

2026-10-01 新增 `catalog.ImportWorldRuntime`，在同一个归档中导入世界和 `list/npc.lst` 所引用的移动规则，并建立 NPCPlaces、EpisodeReturns。`gamedata.Source.World` 与游戏候选入口共用此装配。NPCPlaces 按 town/area 排序，消除 Go map 遍历导致的重复加载顺序差异；现有消费者检查成员或唯一目的地，不依赖多地点的优先顺序。

旧 `odyssey_enter_level` 字段与新 `odyssey_minimum_level` 都由同一个 `[odyssey enter level]` 源数值填充。未覆盖 JSON 中的自定义门槛；字段差异仍会被拒绝。

历史同源归档 `7ef2db59…` 的复核结果：694个区域、236条NPC移动、4个剧情回城索引。补齐后完整字段差异由1225降至62；剩余全部是旧 JSON 未保存的 PhaseMaps，覆盖62个区域、171个槽位。迁移比较为零个既有字段差异，并非完整目录逐字段相同。

新增投影单独检查源 `[phase]` 引用顺序、槽位编号、地图路径、脚本身份、解析 pending 和展开后的 PhaseNPCs。门槛、门户、坐标、NPC移动/任务条件、原始Token、已有投影的任一变化仍拒绝。报告同时保留完整 `difference_count`、`migration_comparison`、`world_projection_additions` 和 `migration_compatible`，不会把新增投影伪装成完整字段零差异。新增阶段图用于源资料保留及已存在的只读 NPC 诊断，不扩大 NPC 交互授权或协议行为。

游戏入口允许 `-pvf-catalogs world,quests,progression` 或单选 `world`。数据库打开前核对源版本、世界有效字段和新增投影；业务复用准备好的 PVF 目录，后续不重读世界或 NPC 导出 JSON。过渡期仍需要 `-world-catalog` 基线及同目录的 NPC JSON 进行对照。PVF 世界禁止 `DFO_NPC_PRESENCE_WORLD` 的额外 JSON 覆盖，需清除此环境变量；不选择 PVF 世界时原诊断方式保留。

不连接数据库的真实归档入口测试验证：三目录约12.76秒准备完成；释放归档和GC后独立测试保留 heap 45294480字节，约43.2 MiB。该数字不是全游戏服务的 RSS 或启动峰值。完整历史源报告为 `.tmp/pvfaudit-world-runtime-20261001.json`，当前客户端报告为 `.tmp/pvfaudit-world-client-20261001.json`。当前源世界也只有62项投影新增、迁移比较零差异，任务/经验完整字段零差异，但三领域均因来源不一致而 `migration_compatible=false`。新增阶段图导致严格审计仍返回2，即使同源 `migration_compatible=true`；跨源结果始终不允许候选接入。

本批完成 `go test ./...`、`go vet ./...` 和 `git diff --check`。测试覆盖新增投影身份/顺序/未解析项拒绝、既有门槛与已有阶段图变化拒绝、NPC索引稳定去重、候选默认关闭、JSON诊断覆盖拒绝及真实归档准备后目录复用；不启动客户端或连接玩家数据库。

存档版本保持 `7ef2db59…`，未部署、重启或修改数据库。当前客户端 inner `be95d64e…` 的来源对接仍未放开。下一批为职业投影与共享物品/装备入口。

## 第三批：物品与装备直读（2026-10-01用户实机确认）

新增 `items,equipment,periods,skins,journal,create-cost` 六个候选领域，连同此前的 `world,quests,progression` 共九个。共享索引的 ID 只来自原版两个 list，不根据文件名猜测；男女职业 `avatar` / `at_avatar` 路径保持原来的分类。599771条索引的路径、类型、堆叠类型/上限及列表哈希与历史同源JSON逐字段零差异。

全量装备由 `inventory.OpenPVFEquipmentCatalog` 按ID读取，继续复用原定义解析器、名望段解析及2048条定义缓存。独立真实归档测试将424216条PVF定义与原 `.index.json` + `.data` 的定义逐项 `DeepEqual`，包括脚本路径/哈希、全部字段及私有名望数据，全部通过，约128.76秒。游戏启动门禁核对版本、列表哈希和完整ID集合，不在每次启动重复这一离线全量解压对比。

常驻Archive缩减到424216个装备文件的只读视图，完整目录可回收；视图共享已核验的不可变归档字节及字符串池，不继续读可能被修改的磁盘文件。视图内重新编号的索引不能用于改写归档，`WriteEntryCopy` 和 `Localize` 明确拒绝它。解压块缓存上限64MiB，大于上限的单块不缓存，文本缓存最多2048条。此批并未把所有装备并入怪物掉落或任务奖励池；原3174行投影和部位策略保持当前范围。

其余四目录完整比对零差异：期限124610个模板、外观1733个可用模板及127个源缺失skin、图鉴5个分类、成本9组。已有永不过期策略、外观授权与玩家持有状态保持原逻辑。选中的候选在数据库打开前拒绝导入/来源/字段错误，不因缺失而回退为另一份JSON；业务加载复用已准备数据。

六个新增领域独立启动门禁约30.43秒；九领域合并测试在其他全量测试并行时约45.03秒，GC后heap为1579436784字节（约1.47GiB）。这些是无数据库独立测试数据，并非完整游戏服务RSS或启动峰值。主副本、技能、材料、商店、礼盒、特殊奖励、嵌入JSON及管理工具仍在迁移清单中。当前客户端inner的跨源兼容仍未开放，存档版本继续使用7ef2db59…。

启动profile `server/work/dfo-lan/configs/pvf-direct-candidate.json` 指向 `.tmp/bin/wireprobe-handoff-source.exe` 隔离候选，使用历史同源归档；`DFO_PVF_CATALOGS`、`DFO_PVF_ARCHIVE`、`DFO_PVF_SHA256` 可供显式候选启动使用。PVF profile保留用户已有玩法开关，JSON默认启动不变。等待ready的超时仅在PVF模式扩大到180秒。手动操作、日志验收、回退与剩余边界见 [PVF直读实机验证](PVF直读实机验证.md)。

实机前，本批全量 `go test ./...`、`go vet ./...`、6项Python准备/profile测试和启动脚本语法检查通过；隔离候选已编译，SHA256 `95b009aa41e830a70aa1fc76e150b186b7caef49a02842b8e9ffd07a5255e44e`。代码准备阶段未覆盖日常exe、连接玩家数据库或启动客户端；当时未更新confirmed baseline或提交，后续用户验收见下文。

只读启动依赖检查通过；日常客户端目录实际为`F:\wip\dof\115US`，Script.pvf/sk.dat与仓库资源同哈希。日常与权威EXE同为2.38.2.34，但`.text`有4字节既有差异，运行身份与文件哈希已记录在实机验证文档；不改客户端，也不将该差异推定为PVF加载或协议行为等价。

2026-10-01用户确认“经过确认，都是正常的”，九领域直读已成为本批confirmed baseline。实机会话`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_031044_836440_next37`确实运行隔离程序并通过全部九领域启动校验，准备约37.99秒；角色11/19/20有换装/名望刷新、任务奖励与两次副本结算日志，客户端正常退出。完整运行身份、事件计数与确认边界见实机验证文档。后续继续技能/职业投影、材料/价格/礼盒与其它领域，并移除已验收领域的过渡期JSON启动门禁；默认启动与跨源存档对接暂不改变。

## 当前工具操作

以下命令在 `server/work/dfo-lan/` 执行，使用仓库便携工具链。输出路径须不存在；准备工具也拒绝写入客户端目录。

```powershell
# 当前客户端只读解包，写到服务端临时目录；绝不调用资源修改或重包脚本。
../../../tools/python/python.exe -B scripts/prepare_inner_pvf.py `
  --client-dir ../../../client `
  --output .tmp/Script.client.inner.pvf `
  --manifest .tmp/Script.client.inner.manifest.json

# 历史归档与现用 JSON 核对。
../../../tools/go/bin/go.exe run ./cmd/pvfaudit `
  -pvf-archive ../client-build/Script.inner.pvf `
  -pvf-sha256 7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80 `
  -difference-limit 400 -output .tmp/pvfaudit-history.json

# 实际客户端归档核对。来源错配仍返回2，同时记录字段差异供诊断。
../../../tools/go/bin/go.exe run ./cmd/pvfaudit `
  -pvf-archive .tmp/Script.client.inner.pvf `
  -pvf-sha256 be95d64ee120248ae503194d2f61743ef74409a8ff999a4a986ca2e0bccf69b0 `
  -difference-limit 400 -output .tmp/pvfaudit-client.json
```

工具和客户端路径以模块根为基准；也可使用绝对路径，避免从其他工作目录运行。审计退出码0代表所选目录来源与字段均一致，2代表审计完成但差异或领域错误未通过，1代表参数、归档或报告写入失败。

游戏服务新增参数为 `-pvf-catalogs world,quests,progression -pvf-archive <历史inner绝对路径> -pvf-sha256 7ef2db59…`，SHA需填完整值。仍要求原 `-character-catalog` 和所选领域的 `-world-catalog`、`-quest-catalog`、`-progression-catalog` 作为启动核对基线；核对完成后业务消费 PVF 导入的内存目录。移除启动时基线 JSON 依赖是后续默认切换阶段的工作，不将本候选称为全量直读已完成。

候选源码没有部署、重启或实机验收；默认启动方式保持原行为。回退本候选只需不传 `-pvf-catalogs`，不修改存档与历史来源。


### 2026-10-01继续实施：十四领域候选与JSON门禁分离

新增`skills,prices,materials,boosters,tutorial`五领域，联合十四领域完成完整对照。技能按职业/技能ID比较以消除旧导出器map遍历造成的职业行顺序差异；Token及全部学习字段保持严格比较，未忽略技能类型差异。生效next27的3224条一致，旧release的12条技能差异没有覆盖进本批。

`-pvf-verify-baselines`/`DFO_PVF_VERIFY_BASELINES`将导出JSON对照限定为可选审计。默认仍开启审计；新`pvf-next-candidate.json`设为0，全部所选领域仅从同源PVF准备。预期SHA、角色来源锚点和存档版本继续强制校验。十四领域缺失JSON路径测试通过；职业锚点、本服策略和未迁移目录仍保留现有文件。

价格599682条、材料14211条、Booster42504条、教程16条完整一致；88条非法价格继续拒绝交易，材料金币模板0保留。材料JSON旧外层来源2429b15a与inner来源7ef2db59仅在该投影全部成本/路径一致后替换元数据，不作为通用来源别名。礼盒有226次智能组替换，6159个既有未解析body和掉落组21469仍明确报告，不猜奖励。

商店读源核对发现`100000375`绑定两份周年`.shp`，旧导出器以覆盖选定一份，源itemshop列表未提供两者引用。尚缺开店引用/当前客户端消费闭环，`shops`保持禁止选择并保留现有JSON；这不是认定商店数据PVF没有。普通物品价格及自身材料成本已先迁移，商店绑定单独补证据。

十四领域完整对照准备41.42秒，GC后heap约1.53GiB；不读所选JSON的准备35.02秒。全量Go测试/vet、7项Python准备/profile测试通过。新增隔离程序`.tmp/pvf-next/bin/wireprobe-handoff-source.exe`，SHA256 `b8668e5ef9e56ed37f3ba313fb9349625aa64e0cf16dd41b67e13192a85f0db9`；九领域已确认程序保留原哈希。详细启动、回退和手动检查见[第二批实机验证](PVF直读第二批实机验证.md)。本批等待用户实机，不升级confirmed baseline，未修改默认启动、客户端资源、数据库结构或玩家存档。

剩余可迁移项继续覆盖：职业源字段/策略拆分、普通装备选择/掉落、盾牌和随机词条、强化/增幅券与费用、附魔、金库PVF字段、主副本和覆盖、Odyssey/军团/调律/矿区/黑鸦源奖励、抽奖/选择箱/COS礼盒、嵌入数据及GM源查询；详见上方清单。下一批优先强化/增幅与附魔等已有导出器的直接规则，商店和同名礼盒并行补只读取证，不因一处绑定缺口停止其它目录迁移。


### 2026-10-01十四领域确认收口

用户确认正常，十四领域及所选JSON启动对照分离升级confirmed baseline。实际会话`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_042311_786667_next37`运行第二批隔离程序，准备32.157秒；有4次材料购买、1次开箱、9次装备移动提交及技能恢复日志，客户端正常退出。完整范围见第二批实机验证文档；后续迁移另立候选，不覆盖本已确认程序。


### 2026-10-01第三批强化、增幅与附魔候选

第二批十四领域已确认并提交为`28c866b`。第三批新增`enhancements`选择项，接入强化券1196种、增幅券1629种、增幅书433种、附魔宝珠4846种，以及强化/增幅各255级费用源表；合计15个选择项、20类源数据。材料路径来自PVF索引，原有纯净书白名单、材料选择/容器、成功率、失败和安全强化补正策略分离到`pvf-enhancement-policy.json`。

原始审计仅有926处普通强化券期限头差异，旧JSON缺少而PVF存在；直读保留源字段，实例期限校验逻辑不变。仅允许该缺失头补充，其余有效字段完整一致，不添加来源别名或改存档版本。缺失全部所选JSON路径的联合准备与现有金币费用锚点通过。第三批使用独立profile和程序，待用户实机；第二批confirmed baseline继续保留。启动、核对与回退见[第三批实机验证](PVF直读第三批实机验证.md)。

下一步继续随机词条、骑士盾牌、普通装备选择/掉落、副本与特殊奖励、嵌入目录及GM源查询；职业策略差异、商店绑定、同名COS礼盒仍按各自证据闭环，不把尚未完成的项归为PVF没有。


### 2026-10-01第三批确认与继续迁移

用户于2026-10-01确认正常并要求提交。第三批15个选择项/20类源数据升级confirmed baseline，程序SHA256继续为`9372b04936353219a0aa1c428cf61c4f30ef463e7d282022f20229fc322005db`，使用`pvf-enhancement-candidate.json`。本次确认依据用户反馈；没有新增会话日志可引用，不扩展为逐职业或全部特殊券验收。

后续六项已完成源码接线和完整离线核对，仍属候选：随机词条17组、骑士盾牌25面、誓约/引子189件、账号金库40档、掉落1022种及装备选择3174行/2794行掉落池。金库客户端容量及存档来源保留，装备基础白名单1536个ID和掉落排除6013保留为原策略；任务新增装备从PVF任务推导。尚未为这六项部署或确认实机。

完整迁移目标继续有效。下一项为城镇/主副本及其源覆盖数据，随后继续特殊玩法、嵌入目录、商品绑定和GM查询。未完成项不得据文件名缺失判定PVF没有。

### 2026-10-01继续迁移：场景与副本源覆盖

第三批确认与后续六类目录源码提交为`17a8c2b`。新增城镇、主副本、训练场、教程副本、两座塔、普通深渊地图和迷宫源概率。合计28个选择项/34类源数据，全部所选导出JSON路径不存在的联合准备通过（并发全量测试时49.03秒）；独立策略及角色来源锚点继续保留。新数据均为离线候选，尚未实机，不覆盖第三批已确认程序。

旧主副本导出有14条`invalid [basis level]`诊断：4个训练场现在由独立源目录接入，另10个当前解析器新增接受的副本维持禁用策略。仅归一化这14条已失效诊断，剩余1699条诊断仍完整核对；3200个副本和18387张地图全部有效字段无差异。源数据未被补猜或重写。

运行入口保留JSON模式回退，PVF主副本按完整目录应用覆盖，不依赖旧导出文件basename启用。候选目录见[第四批进度](PVF直读第四批迁移进度.md)。剩余终场/重访/武斗大会、特殊玩法、职业、商品绑定、抽奖/COS礼盒、嵌入目录和GM查询仍继续实施，不能称全项目迁移完成。


## 2026-10-01：第四批确认与第五批源表候选

第四批28选择项/34类源数据由用户确认正常，确认程序保持。天启与调律已接入完整离线候选，新增2项后合计30项/36类源数据；源绑定、原始字段及运行投影核对通过，调参副本隔离保持。军团导入器已存在，但未发现生产消费入口，不新增玩法启用。第五批身份与验证见[第五批进度](PVF直读第五批迁移进度.md)。其它剩余迁移继续执行，本段不宣称全部完成。


## 2026-10-01：奥德赛五项完成离线迁移

奥德赛五项直读已完成离线候选，合计35选择项/41类有效源投影。成长源50通关/50准入副本、3赠品及毕业礼盒10420561与毕业主线完整一致；章节7章/50副本/15奖励模板、7行章节掉落、2种货币及85组创建武器选项完整一致。章节2/7禁用、章节概率和货币概率 `[1000,10000,10000,10000]` 保留为独立策略，奖励模板与最终副本由PVF推导。35项缺失所选JSON联合准备、完整源对照、全量Go测试/vet及8项Python测试通过。profile为`pvf-odyssey-candidate.json`，隔离程序SHA256 `9c1b9722733ef93db5f57d11fc25fa415dfd01e5f16c36b6bdea89c97ff26f3a`。确认范围仍为第四批28项；所有旧候选程序/策略保持可回退。

赠品与毕业礼盒引用、章节文本、武器选择类别和两种币的源定义均从同源PVF读取；章节启用/概率及rank选币继续作为服务器策略。原30项策略文件保持，以新文件承载35项策略，保留回退程序严格解析兼容。矿区、黑鸦及其它剩余项继续实施。


## 2026-10-01：无色晶块存储源覆盖

新增clear-cube选择项，3037无色小晶块从共享PVF索引及原始脚本直读，完整源Token/哈希对照一致。保留原存储覆盖中Grade/Rarity/Weight为0的最小投影，原源值仍在ScriptRecord中；不进入普通掉落池，不改变分解或技能消耗公式。总计36选择项/42类有效源投影，缺失所有所选导出JSON联合准备约42.10秒通过，混用存储来源拒绝。全量Go测试/vet、8项Python测试及只读依赖检查通过。新profile为`pvf-cube-candidate.json`，隔离程序SHA256 `c6e29f3cc4c8b375ee9ecbc7781553134b005c6b1d03cc94980b83ffddd1261b`；第五批尚未实机，确认范围仍为第四批28项。


## 2026-10-01：黑鸦源奖励范围迁移

新增black-purgatory直读选择项，普通翻牌5分支、仅记录的VIP1分支、三组装备源范围完整一致（史诗208、神话35、腐蚀产物135件）；现有奖励包装展开、装备验证与事务链保持。10%/0.1%/1%本服独立概率迁入`pvf-reward-policy.json`；源脚本路径/哈希、八列奖励及装备ID/等级/稀有度全部从PVF读取。合计37选择项/43类有效源投影；37项缺失所选JSON联合准备约42.10秒、完整黑鸦源审计、全量Go测试/vet、8项Python测试和只读依赖检查通过。最新profile为`pvf-rewards-candidate.json`，隔离程序SHA256 `a044d143c38924931675929bd2bc768fcbcd551c1f002a91ef6176d9520bf5b1`。确认范围仍为第四批28项；第五批候选未实机。

来源身份、保留概率与回退见[第五批进度](PVF直读第五批迁移进度.md)。矿区奖励及其它剩余目录继续推进，不宣称项目已全部无JSON。

## 2026-10-01：赤红铁矿源奖励图迁移

新增bleeding-mine直读选择项，12阶段/12领主入口/3难度奖励、117容器、1782物品及全部合成列表/权重完整一致；35个负数空奖签保留，合成机会[1,3,5]和最大5次保持，源失败占位物10330673继续强制禁止发放。修正共享CTP标签池定位：矿区列索引90024包含0x5b，真实池始于90481，改按头部trailer边界跳过NUL垫字节；已加入索引91含左括号的回归用例，天启/调律再次完整核对通过。合计38选择项/44类有效源投影，缺失所选JSON联合准备约42.11秒、全量Go测试/vet、8项Python测试及只读启动检查通过。最新profile为`pvf-mine-candidate.json`，隔离程序SHA256 `3df13ff8b310f634b09308dcf6e5faa3558e8ccc679b3814a79953a812024571`；确认范围仍为第四批28项，第五批新增候选未实机。

第五批已累计新增10选择项，源范围和回退身份见[第五批进度](PVF直读第五批迁移进度.md)。其它剩余项继续实施，本段不宣称全部完成。
