# PVF 单一内容真源改造计划

## 2026-10-03：规则驱动流程已确认，继续审计消费者

用户确认：ETC / PVF 原生脚本应定义玩法，Go 负责执行，不能维护多套规范。流程唯一真源为根 [AGENTS.md §0.2](../../../AGENTS.md#02-pvf-脚本驱动与单一规则流程强制2026-10-03-用户确认)，本计划按该流程执行，不另定义内容归属标准。

下一阶段检查从「是否直读 PVF」深入到「执行器是否仍重复维护玩法表」：奥德赛三个副本 ID 的槽位解锁 switch，与源成长表/等级动作的关系；创建奖励 `.etc → .stk → 护甲/武器/补给` 引用链，与 Go 固定 ID、数量和清单的关系。普通模式仍按任务 `.qst` 奖励触发，共享动作执行能力时保留模式与条件。上述项处于审计/待迁移状态；本轮仅固化流程，未改运行行为。后续改代码发现与源脚本重复的定义，按根流程当轮主动提示用户，并记录源字段、重复位置及迁移状态。

## 2026-10-03：COS 礼盒清单由 PVF 自动发现（源码候选）

移除 `pvf-box-policy.json` 的 `templates` 和 `cos_paths` 两份内容清单。统一 Source 的盒子导入器遍历当前归档 `.cos`，按现有原生 lot-group 标签选择已支持语法，由 `[material]` 关联原生 stackable 索引，并核验普通/增强光辉宝盒动作。解析错误、重复材料绑定、缺失物品或不支持的动作明确失败；不猜文件名，不新增奖励或概率规则。policy 只保留版本、槽位及缺失堆叠上限兼容值；旧清单字段由严格 JSON 解码拒绝，部署源码候选须同时更新 policy。

实际内层 PVF `8b2a9f83…` 有 1,434 个 COS 文件，发现的两个宝盒保持 54 种奖励、58 份来源记录；完整 Tables/Rewards 与保留历史对照一致。Go 1.26.5 全量测试、vet 及实际 PVF 专项通过。顶层 JSON 仍为 62 个，本轮收敛的是字段和源绑定，不是删除文件。奥德赛补充物品、按难度选择货币及掉落概率仍保留，尚未建立完整原生关系替代证据。

confirmed baseline 保持已有实机范围；没有改存档、数据库或客户端，正式程序未替换。

## 2026-10-03：四份无当前消费者的内容导出删除

本轮实际删除 `black-purgatory-rewards.json`、`bleeding-mine-rewards.json`、`town.generated.json`、`dungeons.terminal-scenes.json`，共 826,529 字节（约 0.79 MiB），顶层 JSON **66→62**。四域现有原生读取保持，隐式历史 baseline 依赖退休；towncatalog 只读统一 Source，诊断导出要求显式输出。没有新增整表压缩快照；城镇条目从两份历史交付清单同步移除，其它旧条目不重生成。

删除后 Go 1.26.5 无缓存全量测试与 vet 通过；未访问玩家库，正式程序未替换。

仍有独立消费者的测试输入、历史版本输入、运行策略与布局规则保留。前一阶段“移除 26 处回退”是运行入口改造，本段才是四份文件的实际删除。按用户继续授权提交本轮源码和文件清理，并更新上游 MR !139；confirmed baseline 保持原实机范围，不将源码确认记为新的实机验收。

## 2026-10-03：剩余内容加载入口 PVF 唯一真源（源码候选）

- 移除 26 个剩余运行入口的 JSON 回退：角色、随机词条、骑士盾牌、誓约档位、仓库、装备图鉴/生成成本、教程、盒子、材料商店、城镇/训练与教程副本、黑鸦/矿区/清晰方块、奥德赛五域、调律、末世录及三类副本叠层。未选择或未准备对应 PVF 域时明确拒绝；旧路径参数仅兼容调用，不提供内容。已有原生解析器和玩法数值保持。
- 删除网关随机词条/商店的 JSON 自动发现，以及矿区/盒子的文件存在性启用逻辑。新增存储前内容依赖门禁；非角色域准备不再读取历史角色 JSON 作来源锚点，仍核验实际 PVF SHA256。Source.Characters/Progression 只接受 PVF；AuditCatalog 显式读取历史对照，保持审计用途。shieldaudit 接统一 Source 和骑士盾牌解析器，要求显式输出并拒绝 JSON seed。
- Go 1.26.5 无缓存全量 `go test -count=1 ./...` 与 `go vet ./...` 通过；后补的只读真源/盾牌诊断门禁专项通过。Python 3.11.9 的 profile/默认启动检查 24/24 通过，默认启动测试使用已有 `.tmp/config-cleanup/pydeps`，未安装依赖。
- 实际内层 PVF `8b2a9f83…` 修改前/后 54 域准备报告的 14 个非 memory 字段相同（物品 599,771、装备绑定 424,216、装备选集 3,174、任务 2,844、副本 3,200）；报告均 `storage_accessed=false`、`runtime_started=false`。原生随机词条/盾牌/誓约/仓库及生产入口缺失历史角色锚点专项通过。报告规模一致不声称等于逐项实机玩法验收，也不据两轮不同缓存状态比较性能。
- 独立候选 `.tmp/pvf-remaining-runtime/wireprobe-native-only.exe`，SHA256 `7447659027474094f15e532009de1ab890fba07c495ff984b6ff89366fff7b72`；同目录 `启动验证.cmd` 供用户关闭现有会话后手动验证。正式/default bin 未替换，confirmed baseline 保持既有实机范围，等待本轮用户确认后收口。
- 顶层 configs JSON 仍为 66 个：历史对照及测试输入继续保留，本轮没有把运行退役计作文件删除。运行策略、容器/槽位和服主锻造数值 JSON 保留；没有改 PVF、SQL/schema、玩家存档或客户端资源，没有启动服务/客户端或访问玩家库。用户 `.gitignore` 和四份 charactercheck 工作区改动不属于本任务，不覆盖或提交。

更新：2026-10-03。用户要求 PVF 作为核心游戏内容文件，消除另一套人工维护的玩法 JSON，并明确 PVF 对服务端是只读资源。内容修改由服务端之外的编辑工具完成。本文记录目标、第一批实际审计和迁移缺口；不表示全量配置依赖已经解除。

## 2026-10-03：configs 目录语义归位与小型测试输入

五份诊断/示例资源原字节归位：登录回包和两份 SELECT 实验、出生点策略转入已有 `cmd/wireprobe/testdata`；维修 profile 示例转入 `docs`。Python 探针、Go 登录回包测试、维修测试及操作文档同步路径。删除被现有 Odyssey 规则覆盖的 `character-rules.jobs-release.json` 和旧探针 fallback。顶层 JSON 71→66（四个迁出、一个删除），实际只减少冗余 JSON 88 字节，不把目录调整计成数据去重收益。

普通武器皮肤职业限制测试使用一条 typed 装备定义；完整宽/窄装备选择测试从真实 PVF + 既有 policy 重建，不保存新整表快照。当前没有挂载真实归档，相关集成门禁未执行；不声称当前内容等价已验收。依赖报告增加 docs JSON 示例扫描，交付清单同步本批路径。其它角色快照、大内容表和默认 probe 规则仍有消费者，后续按实际用途继续处理。

Go 1.26.5 全量 `go test -count=1 ./...` 与 `go vet ./...`、Python 3.11.9 的 26 项启动/profile/身份检查通过；迁移输入内容保持，新路径、旧路径退休和交付清单条目核验通过。没有运行服务实例、客户端或玩家库，没有修改 PVF/schema/存档；用户 `.gitignore` 保持并排除提交。

## 2026-10-03：剩余装备 / 掉落运行读取收口（源码候选）

loot 与 equipment-selection 的运行 JSON 回退、隐式 baseline 已移除，加载 API 必须有已选择且准备好的原生目录。wireprobe 启用掉落或任务装备时，在访问存储前检查所需领域；原有等级上限、排除项、概率和发放规则继续使用既有政策。

- charactercheck / audit36 统一使用 `gamedata.Open/Source` 及既有 policy；基础装备选集使用同源空任务目录保持政策基础范围，任务扩展另走原生任务。BagRules 使用实际内容 checksum；角色 ConfigVersion 仍与 SaveIdentity 核对，不修改角色存档或别名化来源哈希。
- GM 正常运行继续用原生 PVF，退休无调用的 JSON fallback 构建器；离线装备部位导出直接使用原生 ItemDisplay，读取失败不覆盖现有产物。initialrepair 删除不可达 JSON 分支，lootimport 使用 Source 并要求显式输出。当前源码探针用 PVF 领域激活标记，历史二进制需要显式提供匹配的历史装备输入。
- 本轮不增加整表 gzip 快照。`loot.next25.json`、`equipment.current35.json`、`equipment.current37.json` 暂时保留为旧测试输入，顶层 JSON 仍为 71 个，不能把运行退役误报为文件已删除。后续普通行为测试优先小型输入，当前内容校验用真实 PVF；完整历史快照仅在确有版本比较需求时保留。
- Go 1.26.5 全量 `go test -count=1 ./...` 与 `go vet ./...` 通过；Python 3.11.9 启动/profile/身份接线检查 26/26 及探针语法检查通过。当前没有挂载 PVF；真实归档专项仍待执行，未运行客户端、服务实例或玩家库，没有修改 PVF、schema 或存档，不扩展 confirmed baseline。

## 2026-10-03：任务装备重复导出与抽奖唯一真源（源码候选）

删除 `quest-equipment.current37.json`、`quest-equipment.next29.json`、`lottery-item-pools.json`、`lottery-equipment-pools.json`，共 7,111,136 字节；顶层 JSON 75→71。抽奖完整历史快照保留为 476,221 字节 gzip，仅供测试读取并核验原始 SHA256；本轮内容净减少 6,634,915 字节（约 6.33 MiB）。

- 抽奖运行 API 只接受准备好的原生 PVF 表，不读取旧路径或历史 baseline；启动不再构造退役奖池路径，启用抽奖而缺少原生域时明确拒绝。奖励、数量、权重、发现范围和不可发放奖池拒绝规则保持。
- equipfields、questequipmentimport、equipmentwearimport 统一使用 `gamedata.Open/Source`；两个导出器要求显式输出，旧 JSON seed 参数明确拒绝。基础装备选集继续来自既有 policy，任务装备选集由同源物品与任务构建。
- charactercheck 从 PVF 构建任务装备，内容哈希与原生任务目录核对，角色 ConfigVersion 单独与 SaveIdentity 存档契约核对；不把存档版本当作 PVF 哈希。旧 next29–34 探针不再自动注入退休 JSON，较新候选显式保留当前装备表。`equipment.current35/37.json` 仍有独立消费者，本轮保留。
- Go 1.26.5 全量 `go test -count=1 ./...` 和 `go vet ./...` 通过；Python 3.11.9 启动检查 10/10 与探针语法/参数设置检查通过。当前环境未挂载 PVF，真实归档输出/完整原生对照测试未执行；归档门禁仍保留。未启动客户端、运行服务或玩家库，没有改 PVF、schema、存档或用户 `.gitignore`，不扩展实机 confirmed baseline。

## 下一批子代理边界（只读规划，尚未实施）

- 装备诊断组：先处理 `equipment.current35.json` 的 audit36、charactercheck 与测试消费者，再处理 current37。保留装备槽位/行为 policy；基础 1536 选集和任务扩展选集分别验证，不据当前归档扩大或收窄历史测试。
- 运行入口组：移除 gamedata 的装备/loot JSON fallback 与隐式 baseline，配套 wireprobe 在存储前校验准备好的原生域；旧路径参数的功能开关语义须独立保留。与测试组分开拥有文件。
- 完整夹具组：为剩余装备与 `loot.next25.json` 保存 SHA256 校验的历史快照，迁移单元测试。旧快照 7ef2 与当前归档 8b2 分别记录，不把存档 SaveIdentity 当内容哈希，不宣称跨版本等价。
- GM/导出组：退役无生产调用的 GM JSON 索引构建器，离线 build-data 的 slots 改从原生物品显示投影取得；lootimport 输出改为显式指定。需要实际源/版本证据的 charactercheck 整栈迁移后再删除对应内容文件。

## 2026-10-03：退役仅供测试的 150 级掉落导出

删除 `loot.level150.json`（3,278,355 字节）；配置目录顶层 JSON 76→75。完整测试输入压缩为 184,835 字节的 gzip 夹具，解压时核对原始 SHA256 `939c837b9c1b966cf1655dace420361d03613354869c17a607cbe703b7e0b6cf`，净减少 3,093,520 字节（约 2.95 MiB）。

- 全仓生产代码没有读取该文件；它是掉落、深渊、Odyssey 和装备操作测试的完整历史输入。测试现从 `internal/testfixture.LootLevel150Path` 取得临时副本，`loot.next25.json` 与其现有职责未改。
- Odyssey 源审计器从 `gamedata.Open/Source` 读取 PVF，原始字节诊断也经 `Source.ReadRaw`；输出目录必须通过 `-output-dir` 指定，完整 loot audit 输出在该目录，不写入 configs。掉落等级上限仍使用现有 policy，没有删改概率、排除项、费用或玩家数据。
- Go 1.26.5 `go test -count=1` 覆盖 catalog、loot、inventory、wireprobe 与 gamedata；相关包测试、审计器编译和 `go vet ./...` 通过。隔离 PostgreSQL 测试未启用；未启动客户端或访问玩家库。

## 2026-10-03：经验、物品成本与副本地图覆盖（源码候选）

本轮删除 8 个 JSON：`progression.next25.json`、`item-materials.json`、`item-period-tags.json`、`skin-storage-items.json`，以及 Hell Party、Tournament Quest、Tower of Grief、Tower of Dazzlement 四份地图覆盖，共 10,356,097 字节；顶层 JSON 84→76。完整历史输入只供测试使用，压缩快照共 563,393 字节并校验原始 SHA256；净减 9,792,704 字节（约 9.34 MiB）。

- progression、materials、periods、skins 与四类副本 overlay 的运行读取改为只使用已经准备的原生 PVF 投影。启用 loot 时要求 materials 已选择、准备且非 nil，并在 storage 访问前拒绝缺失目录；商店引用材料成本时不允许缺失材料目录退化为金币报价。原有角色/物品存档与费用规则未更改。
- 对当前 8b2a PVF 的对照测试只在临时历史副本中对齐已知旧 source checksum，完整 typed fields 仍作比较；progression 和两类场景覆盖分别保留 gated 真实归档对照。材料 14,211 项、期限 124,610 项、皮肤 1,733 个已解析项及 127 个未解析引用锁定在测试中；材料成本示例 3242 与商店商品 10345008 保持。
- 角色检查工具的 progression 改用 `gamedata.Open/Source` 原生读取。progression importer、期限/皮肤 importer 和两种塔地图 importer 必须显式指定输出；pvfaudit 的 progression baseline 默认留空。旧 JSON 只通过 SHA 校验的 gzip fixture 进入测试，不作为启动回退。
- Go 1.26.5 全量 `go test ./...` 与 `go vet ./...` 通过；未启动客户端、服务端运行实例或玩家数据库，未改 PVF、存档、schema 与用户 `.gitignore`。本轮不增加实机确认范围。



## 2026-10-03：技能 / 副本 / 强化并行唯一真源（源码候选）

本轮删除 14 个运行 JSON：`skills.next27.json`、`skills.release.json`；`dungeons.generated.json`、`dungeons.next28.json`、`dungeons.odyssey-candidate.json`、`dungeons.odyssey-release.json`、`dungeons.odyssey-scenes-release.json`、`dungeons.skycastle-candidate.json`；`reinforcement-tickets.json`、`reinforcement-gold.json`、`amplify-grimoire.json`、`amplify-upgrade.json`、`amplify-tickets.json`、`enchant-beads.json`。共 64,385,633 字节（61.40 MiB），顶层 JSON 98→84。保留完整历史 gzip 快照 2,947,505 字节（2.81 MiB），内容净减 61,438,128 字节（58.59 MiB）。新 helper 仅由测试引用，解压到每个测试临时目录，验证原始未压缩 SHA256；两版技能与六版图保留各自完整数据，未用小样本替换全图覆盖。

- 三域 `Catalogs` 运行入口拒绝 JSON 回退，prepare 阶段不再读取对应 baseline；技能准备去除已死 `LearningPath`，无 character 域时仍由原生角色导入学习绑定。显式角色来源锚定、PVF checksum 及存档身份门禁保持，未对其它域 baseline 改语义。
- 网关显式旧技能 CLI/env、两个副本旧 env 与缺少原生增强的 loot 装配，在 storage 前报错；已准备的原生技能保持来源检查。强化激活仍在原 loot 装配时机，未提前切全局规则。探针移除技能 JSON 注入，旧非原生副本检查直接提示源码/PVF；repair 示例采用默认54域及原政策，保留临时信用额度0。
- skillaudit、dungeonimport、dungeonscenesaudit 统一用 `gamedata.Open/Source`，显式指定导出输出；后者通过 `Source.Files/Script/ResolveScript` 复用同一个已验证归档。charactercheck 学习读原生角色/技能，四个存储检查只导入其用到的原生 dungeon 3。本轮未运行存储检查。删除无生产入口的两份强化旧 Python exporter。
- 原生技能全部 3224 定义及完整详情，父 Source 关闭后 eager/lazy 内容一致；指纹 `352504f912d7621d446affc4c098f18cb026ea88898265c90eb04cbf3fb2ef26`。额外 skills 单域准备以 VerifyBaselines=true 完成，缺失旧技能路径仍复用原生目录，错误 source 拒绝。
- 原生副本全部 3200、地图18387、skipped1699，完整 eager/lazy 地图对照与十四个政策排除图通过；归一化 Source 的加载时间/路径等运维字段后，所有内容字段指纹 `cfef29a48e70f1b13bcc04d4b5bdac8bcf0728f9b9313678d816662149b90427`。训练、教程、附加场景与政策保持。
- 六类增强指纹 `22083b62b1071c074bb6592612309c7dfd94d3070838623477a828fd2c551507` 前后相同，含强化券1196、增幅券1629、增幅书433、宝珠4846、两类费用各255行。历史快照与当前 PVF 的1430处差异全部是增幅券到期年份2025→2099；旧强化券缺少的926个日期头仅在离线审计补齐，其它 typed字段一致。两侧来源差异钉在门禁里，没有修改当前运行内容、费用公式、倍率、政策或资源。
- 删除后 Go 1.26.5 `go test -count=1 ./...` / `go vet ./...` 通过，各域原生专项及相关包检查通过；Python 3.11.9 的24项 profile/启动测试与脚本编译通过，复用原 `.tmp/config-cleanup/pydeps`，没有安装依赖。技能prepare收尾测试/vet另过。
- 网关候选 `77de3398a6362756c2d6837c04a567d2aaef9a0049c1a63e624804ad98d33b6c` 的54域只读报告，与上一提交4d943e5的已验证候选 `56f24bd2f9eb5d38112a598a4836f3d6b62b5dba541328d23251e93ed26c1948` 全部非 memory字段一致；storage_accessed=false、runtime_started=false，不据并行内存或时长推断性能。
- 旧交付清单只移除原有退休条目，其它历史哈希不重写；字面引用清单重新生成。独立程序/profile/手动入口位于 `server/work/dfo-lan/.tmp/pvf-parallel-cleanup/`，原生分域证据分别在 `.tmp/skills-cleanup/`、`.tmp/dungeons-cleanup/`、`.tmp/enhancements-cleanup/`。正式/default bin 未替换、未访问玩家库或启动客户端，PVF、SQL/schema、存档及用户 `.gitignore` 保持；confirmed baseline 不增加实机确认范围。

## 2026-10-03：世界 / 任务及 NPC 传送唯一运行真源（源码候选）

删除 `world.generated.json`（29.80 MiB）、`quests.generated.json`（26.79 MiB）与 `npc-teleport.generated.json`（0.05 MiB），共 59,382,632 字节（56.63 MiB）；顶层 JSON 101→98。完整历史图仅作为 `internal/testfixture/testdata/*.json.gz`，共 2,138,961 字节（2.04 MiB），净减少约 54.59 MiB。保留全图是为了继续覆盖全任务目标/碰撞/前置链、城镇到达白名单、NPC 传送及阶段图；没有把全图改成当前运行内容表。测试 helper 仅由测试引用，解压到各测试私有临时目录，验证原始未压缩 SHA256，缺失或损坏直接失败；不落回 configs、不修改历史来源或字段。

- `gamedata.Source` / `Catalogs` 的世界和任务入口只返回原生结果，移除启动阶段两域的旧 baseline 读取。`VerifyBaselines` 对其余未退休域保持原语义。世界仍从同一 PVF 导入 NPC moves、位置索引和 episode returns，不需要传送 sidecar。
- 网关显式旧 world/quest 路径缺少对应原生域时在存储访问前拒绝；兼容参数不提供内容。启动器不再注入旧文件路径，活动原生域继续自动装配服务。NPC 影子诊断移除外部 JSON 覆盖，旧 `DFO_NPC_PRESENCE_WORLD` 明确拒绝；只读诊断保持原时机与玩家状态取样，不改变互动准入或协议。
- questchain、npcpresenceaudit、audit36 和 charactercheck 的任务/世界读取改为原生；questequipmentimport 从自己的同源归档导入任务奖励。questrepair 去掉已不可达的 JSON 分支，默认派生归档位置，`-check-catalogs` 在数据库之前返回。现有修复/临时 schema 逻辑及存档身份保持，本批不运行数据库流程。
- worldcatalog、questcatalog、npcteleportimport 必须明确指定诊断输出；pvfaudit 默认比较剩余 characters/progression，world/quests 历史比较须显式提供 baseline。纯 `catalog.LoadWorld/LoadQuests` 仅供测试和明确历史审计读取，不是运行源；`AuditCatalog` 直接调用它们，保留旧源匹配与字段差异报告。
- 迁移前后完整原生指纹相同：694 areas、175 towns、2,844 quests、236 NPC moves、1,080 NPC places、4 episode returns，以及三份原生 LIST 和完整世界/任务元数据。当前 8b2a 归档的十项指纹钉在 `TestNativeWorldQuestsCurrentArchiveFingerprint`，也验证 `VerifyBaselines=true` 与缺失旧文件同时成立。
- Go 1.26.5 删除后 `go test -count=1 ./...`、`go vet ./...` 与 Python 3.11.9 的 32 项 profile/启动/GM 检查通过；Python 使用既有 `.tmp/config-cleanup/pydeps`，没有安装或修改工具环境。原生核心准备、完整 2,844 任务与 3,224 技能详情、全部 NPC 可见性图、任务索引和 33 个城镇到达场景回归通过。技能仅作为既有详情测试的联合验证，本批未迁移技能 JSON。
- 最终候选 54 域准备报告与 52962ce 原生 baseline 的全部非 memory 字段一致；`storage_accessed=false`、`runtime_started=false`。questrepair 使用不存在的 storage 配置完成原生只读准备，报告 2,844 quests 和 `storage_accessed=false`。不据并行运行时长或 memory 指标推断性能变化。
- 两份旧交付清单仅移除原有 world/quests 两个条目，其它历史哈希和说明保持；未重生成清单。旧 verification 报告为历史证据，未改写。根本资源、SQL/schema、玩家存档及默认 profile 保持，未启动玩家库/客户端或替换正式程序；confirmed baseline 不增加实机确认范围。用户 `.gitignore` 保持并排除提交。

候选、baseline、指纹和准备报告在 `server/work/dfo-lan/.tmp/world-quest-cleanup/`。游戏候选 SHA256 `56f24bd2f9eb5d38112a598a4836f3d6b62b5dba541328d23251e93ed26c1948`；questrepair 候选 `d2d183e0d39e11a91cf0da6040c01e0a4f117d8942e316da85471b477776aabf`。手动游戏入口为同目录 `启动验证.cmd`，本批没有运行它。其它领域导出与运维/容器政策继续逐项审计。

## 2026-10-03：物品索引 / 全量装备及 GM 原生收口（源码候选）

删除模块 `items.index.json`（65.81 MiB）、`equipment-full.index.json`（51.16 MiB）、`equipment-full.data`（311.55 MiB），共 449,330,028 字节（428.51 MiB），顶层 JSON 103→101。新测试夹具 4.57 MiB，保留 37,887 条旧抽奖/物品元数据及 490 条装备测试锚点与成本组成员。测试记录沿用旧来源和逐条压缩记录哈希，不迁成运行内容表。

- 网关移除物品索引与 full 装备 JSON 回退、自动探测和旧 baseline 审计；显式旧路径缺少原生域时在存储访问前拒绝，已准备时只取原生结果。完整穿戴目录继续独立于掉落/任务选集，不扩大掉落范围。探针不再注入旧 index/full 路径，既有穿戴规则路径选择保持。
- admin/GM 的目录默认且唯一使用 PVF，JSON 模式显式拒绝，原 JSON Awarder 与 GM 运行装配分支移除。余额/历史/纯点券操作保持原存储事务路线；发物品、金币、查询目录在存储前准备原生目录。中文名字保留外部显示覆盖，物品身份、属性、部位、最低等级和发放校验来自 PVF；不修改存档身份或 SQL/schema。
- GM Python 默认 PVF，未显式指定时从 storage 的模块位置派生归档及掉落策略，当前包默认 storage 为相邻游戏目录。归档 SHA256 可显式断言，空值使用实际资源校验哈希。`--check` 只准备目录，不读取 storage 内容、启动 PG/HTTP 或打开浏览器。代理必须使用后端认证的 `/api/catalog-metadata`；401/404/500、无效元数据均拒绝，不再回退游戏 JSON。
- 六个无当前入口调用、依赖退休索引的旧 Python 导出脚本移除（amplify grimoire/tickets、enchant beads、item materials、fame、black purgatory）。Odyssey/lottery/shield/装备名望与 avatar pilot 诊断改用原生物品/装备提供者；equipmentfull 和 indexed exporter 输出必须显式指定，不再默认重建 configs 大目录。诊断工具未执行玩家数据库流程；原 shield 源一致性门禁保持。
- 删除前后原生指纹相同：599,771 条完整物品元数据、424,216 条装备 LIST 绑定（脚本 SHA256 `7ffc480e…`）以及 345 个原生锚点定义。490 条历史装备夹具中 489 条完全一致，109010772 的过期日期由旧 2025 对照当前 2099；确认来源 epoch 的文本差异，两侧值和其它字段分别校验，不改源或导出。完整原生值在迁移前后相同。
- Go 1.26.5 删除后无缓存全量测试、vet、原生 admin/GM/物品及装备回归通过；Python 3.11.9 的 30 项 profile/GM/代理检查通过。54 域网关、admin、GM 的只读准备报告与 HEAD 892b55e 隔离原生 baseline 相同，storage_accessed=false；网关 runtime_started=false。memory 诊断不作为内容等价字段，不据并行运行推断性能收益。
- 独立候选、baseline、指纹、报告、日志和手动入口在 `server/work/dfo-lan/.tmp/item-equipment-cleanup/`，包括游戏 candidate、admin、gmweb。旧正式/源码/GM 发布程序未替换，删除后的配置目录必须配新源码构建；confirmed baseline 仍为原实机范围，没有新增实机确认。服务端/客户端 PVF、玩家库、SQL/schema、存档和用户 `.gitignore` 均保持。

其它域的导出/回退以及 GM 包中的旧显示与历史对照资产继续逐项审计；不表示全部 JSON 已退休。两份交付清单原来没有上述三个数据与六个旧脚本条目，保持不重生成。只读验证不运行数据库集成命令；player SQL 流程未修改。

本批候选 SHA256：网关 `c905fd83debd4aa11553c3b2f2fdcc6374a8301d5b808c82f5bb1f654e2432ce`；admin `0dc2bc27afed975d21ba9313d7fa42fcbd9e34191825424c8a0c2d50708ca432`；gmweb `56021c8260fcdad16e4404f7702bff8cfb8fe049f7014f0653b346ed392dae38`。GM 启动器以默认派生路径调用候选的真实 `--check` 也通过，报告 `storage_accessed=false`；旧 index/full 两条入口的存储前拒绝测试通过。

## 2026-10-03：booster / 自选 / NPC 价格唯一真源（源码候选）

- 删除 `booster-catalog.json`（84.03 MiB）、`selection-boxes-candidate.json`（15.75 MiB）、`shop-prices.json`（14.06 MiB），共 119,375,398 字节（113.85 MiB），顶层 JSON 106→103。三域运行内容只从只读 PVF 准备，移除 JSON 加载、自动探测和旧 JSON baseline 审计；其余域审计保持。
- 旧 CLI/环境变量名保留兼容：原生域已准备时忽略旧路径，未准备且显式给旧路径则在访问存储前报错。未启用的价格域继续拒绝金币交易；物品索引单独服务既有发货分类，不再触发 booster JSON。探针启动器停止传入两个旧导出路径。
- booster/价格诊断导出工具改为从 PVF 枚举索引，自选诊断导出复用原生运行投影；均必须明确指定输出，不再默认重建 configs 内容表。旧 `-index` 与自选 bounded/config 引用参数退休。
- 流程用例使用 359,657 字节的 testdata 夹具（非生产内容）；原生审计保留全部 2,975 个历史盒模板，126,008 次装备发放检查通过。当前源完整 booster/price/selection 与 fixed/unparsed/rejected 六项内容指纹在删除前后完全一致：42,504 个 booster、599,682 条价格、16,749 个自选盒、2 fixed、3 unparsed、3 rejected。
- Go 1.26.5 删除后无缓存全量测试、vet、原生指纹/自选范围/历史装备审计，以及 Python 3.11.9 的 24 项 profile/启动检查通过；收尾变更复跑相关包与 vet。54 域只读准备报告与 HEAD 5f51ee2 的隔离 baseline 相同（14 个非 memory 字段；runtime_started/storage_accessed=false），不据 memory/并行时长推断性能变化。
- 独立候选、内容指纹、54 域报告和手动入口保存在 `server/work/dfo-lan/.tmp/native-commerce-cleanup/`。候选 SHA256 `5cf860a4df2985c2f6b17e6c50ad9fc7a4a9dba6f2eb116692f37f15abbf55a9`。当前工作树须使用更新后源码构建，正式/源码 bin 未替换；confirmed baseline 保持既有实机范围。没有 PVF、SQL/schema 或玩家存档改动，未访问玩家库或启动客户端。用户 `.gitignore` 改动保持且排除提交。

## 2026-10-03：3 个冗余政策 JSON（源码候选）

继续用户授权的配置清理，删除 `pvf-lottery-policy.json`、`pvf-selection-policy.json`、`pvf-content-policy.json`，共 40,829 字节，顶层 JSON 109→106。

- 抽奖历史清单原来有 276 个普通池、2,477 个装备池；删除前逐 ID 核对，与保留的两个奖池 JSON 完全一致。历史原生 importer 对照测试现在从这些奖池取 ID，不再复制独立清单；正常运行继续自动发现。旧 compatibility 参数保持忽略语义。
- 自选政策不提供路径时采用 version=1、空白名单，仍从 PVF 类型自动发现；默认 profile 移除 `DFO_PVF_SELECTION_POLICY`。显式 whitelist/旧 templates 政策仍可读取，路径缺失、版本错误和非法内容仍报错，不静默回退。
- 不提供 content 政策路径时采用与原 `{"version":1}` 文件完全相同的空策略。当前默认档仍明确使用原 `pvf-mine-policy.json`，其数值、补充物品、范围和哈希保持。不是把正常默认档改为空策略。
- 两个 Go 参数的默认路径改为空，帮助与配置向量同步更新；其余参数、环境覆盖、显式空值与路径校验保持。增加缺失/非法显式政策拒绝及空策略等价回归，保留 24 项 Python profile/启动检查。

Go 1.26.5 无缓存全量测试、vet、政策/配置专项、当前 8b2a PVF 的抽奖自动发现与旧范围对照通过。删除后候选的 54 域只读准备通过；与 HEAD 63c783c 的隔离 baseline 构建相比，准备报告的 14 个非 memory 顶层字段全部一致（同源、同域、同目录数量、runtime_started/storage_accessed=false）。memory 为 GC/内存诊断，不作为内容等价字段，也不据本次并行运行推断性能改善。

候选程序为 `server/work/dfo-lan/.tmp/config-policy-cleanup/candidate.exe`，配套 `candidate-profile.json` 和 `启动验证.cmd`；由用户手动运行验证脚本。旧程序可能仍默认读取已删的两个政策文件，删除后的工作树须使用本次源码构建；未替换正式/源码 bin，未启动网关、玩家库或客户端。confirmed baseline 保持原实机确认范围，不将本次离线检查计为玩法实机确认。候选、baseline overlay、报告、日志与删除前哈希均保留在该临时目录且不提交；当前客户端资源、服务端内层 PVF、玩家存档与 SQL/schema 无改动。

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
