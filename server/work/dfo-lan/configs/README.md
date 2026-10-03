# 配置与 PVF 内容归属

PVF 是游戏内容真源，**对于 Go 服务端是只读资源**。服务端只负责读取、解析和校验，生成各领域使用的内存规则；服务运行与加载过程不得反写 PVF。

物品、技能、任务、副本、奖励以及源中可表达的玩法数值，由服务端之外的内容编辑工具维护在 PVF 中。编辑完成后发布资源，再由客户端和服务端读取。派生索引和导出 JSON 可以自动重建。

本目录当前仍包含运行配置、历史导出数据、兼容公式与手工范围清单。2026-10-01 首批删除 24 个旧导出和已替代 profile，顶层 JSON 由 160 个降为 136 个；后续商城、自选与副本清理降至 126 个。2026-10-03 第一轮删除 17 个 JSON，降至 109 个；第二轮解除 3 个冗余政策文件依赖，降至 106 个；第三轮迁移三个大内容表，降至 103 个；第四轮收口物品索引与全量装备降至 101 个；第五轮收口世界、任务和 NPC 传送降至 98 个；第六轮收口技能、完整副本与六类强化/附魔导出，降至 84 个；第七轮收口经验表、物品材料/期限/皮肤及四种副本地图覆盖，降至 76 个；第八轮移除只供测试使用的 150 级掉落快照，第九轮收口两份任务装备重复导出与两份抽奖奖池，运行目录归位后当前为 **66 个**。不能把文件仍存在理解为它需要继续人工维护，也不能依据命名或零字面引用直接删除。

本次收口删除没有消费者的 `cerashop.json` 及其旧导出脚本、5 个与 release 文件字节相同的 candidate 数据，以及 11 个逐批 PVF candidate profile，共减少 JSON 23,584,750 字节（约 22.49 MiB）。重复数据测试统一读取保留的 release 文件；profile 测试使用当前默认档与临时显式配置，保留解析、策略路径和启动覆盖检查。逐项清单见下方改造计划的“2026-10-03：17 个 JSON 收口”章节。默认 `pvf-default.json`、运行加载逻辑、玩家存档和运行二进制保持；历史阶段配置从 Git 恢复，不再作为当前入口维护。

## 当前目录边界

| 用途 | 位置与处理 |
|---|---|
| 运维启动档、手工政策、客户端容器/槽位规则 | 留在 `configs/`；即使带 probe/compat 名字，仍按真实运行用途判断。 |
| 选角/出生点实验输入、原生登录回包向量 | `cmd/wireprobe/testdata/`，供显式诊断和测试使用；不混在运行配置目录。 |
| 面向操作人员的维修 profile 示例 | `docs/repair-profile.example.json`；真实默认启动档仍是 `configs/pvf-default.json`。 |
| 仅被旧测试读取的完整内容导出 | 逐项改成小型行为输入或真实 PVF 集成测试后删除；不默认整表搬进另一套测试目录。 |
| 可从 PVF 重建、仍有 legacy loader 的内容表 | 暂留并追踪入口，解除运行 fallback 和测试依赖后删除；不是另一套人工内容真源。 |

2026-10-03 本轮原字节归位 `login-normal22.bin`、`select-parser-probe.json`、`select-world-probe.json`、`town-entry-probe.json` 与维修 profile 示例；删除冗余 `character-rules.jobs-release.json`（88 字节）及其旧 fallback。顶层 JSON 71→66，其中 4 个是迁出、1 个是真正删除；不能把迁出当作仓库减重。武器皮肤职业规则测试改为一条 typed 装备输入；完整宽/窄装备选集改由真实 PVF 重建的集成门禁验证，无新增整表快照。角色候选和大内容表仍有测试或运行消费者，未按零引用删除。

- 端口、路径、数据库连接、日志等属于运维配置。
- `pvf-*-policy.json` 中的概率、范围和默认值，需要追踪源字段与当前消费路径；后续逐项解除重复维护。
- `generated`、`next`、`candidate` 等文件可能被旧工具、测试或回退模式使用，移除前检查引用与运行分支。
- 玩家存档、事务回执和协议向量具有各自用途，不作为 PVF 内容表处理。

抽奖直读从 PVF 自动发现类型，并按现有发放能力校验整个奖池。普通和装备奖池的奖励、数量、顺序及权重均来自 PVF。不能发放的奖池记录源路径、哈希与原因，不启用部分奖励。`pvf-lottery-policy.json` 已删除；两份历史 JSON 已删除，完整历史对照仅从校验原始 SHA256 的 gzip 测试夹具取清单；运行 API 只接受准备好的 PVF 表，缺少原生域明确拒绝。

空白名单的 `pvf-selection-policy.json` 和只有版本号的 `pvf-content-policy.json` 已删除。源码默认不要求这两个路径：自选自动发现，空 content 策略保持原语义。显式指定政策仍严格读取和校验，缺失或损坏不静默回退；当前默认档继续读取承载实际数值的 `pvf-mine-policy.json`，不得以空策略替代该档。第二轮需使用更新后的源码构建；旧程序可能仍默认读取已删除路径。独立候选及验证见改造计划“3 个冗余政策 JSON”章节。

booster、自选和 NPC 价格已成为 PVF 唯一运行内容源，删除三份大 JSON 共 113.85 MiB。运行时不再探测、回退或用它们作 baseline；旧路径参数仅保留兼容，缺少原生域时在存储访问前拒绝。流程测试改用约 0.34 MiB 的 testdata 夹具，完整历史装备发放与当前原生内容指纹另行校验；夹具不能用于运行。相关诊断导出必须显式指定输出，索引和内容均从 PVF 解析。独立源码候选及用户手动验证入口位于 `.tmp/native-commerce-cleanup/`，旧二进制不适用于删除后的配置目录。后续物品/full 装备收口见下一段。

物品索引和全量装备也已只读 PVF，`items.index.json` 与 `equipment-full.index.json/.data` 删除共 428.51 MiB。admin/GM 默认仅 PVF；代理不再回退本地游戏 JSON，测试保留 4.57 MiB 历史夹具。独立源码候选与游戏/GM手动入口在 `.tmp/item-equipment-cleanup/`；旧发布程序保持，当前目录要配新源码构建。其它域的历史导出、政策与回退继续逐项迁移。

世界、任务及 NPC 传送也已只读同一 PVF，删除三个运行导出共 56.63 MiB。全图测试保留 2.04 MiB 压缩历史快照，逐字节校验原哈希；它们不被运行命令加载。相关诊断工具改接原生源，历史比较需要显式 baseline，导出需要显式输出；候选与手动入口在 `.tmp/world-quest-cleanup/`。

技能、完整副本与强化/增幅/附魔也已成为 PVF 唯一运行内容源，删除 14 个导出共 61.40 MiB；完整历史测试快照 2.81 MiB，仅由测试读取，内容净减 58.59 MiB。训练/教程与附加场景、强化政策及独立兼容公式继续保留。旧路径不能恢复 JSON 运行模式，使用当前源码候选 `.tmp/pvf-parallel-cleanup/` 手动回归。

PVF 读取统一复用既有入口：运行装配用 `gamedata.PrepareCatalogs/Catalogs`；诊断与导出用 `gamedata.Open/Source` 的领域方法，归档由一个 Source 管理和关闭；`catalog/pvf` 负责底层只读解析。需要遍历源脚本的诊断用 `Source.Files/Script/ResolveScript`，避免重复打开归档或另建内容 loader。其它未迁移工具仍需逐项接入，详见 [PVF 运行依赖台账](../../../../docs/todo/pvf/PVF运行依赖台账.md)。

第七轮删除 8 个已由 PVF 原生域覆盖的导出，共 10,356,097 字节（约 9.88 MiB）：progression、item materials、period tags、skin storage，以及 Hell Party、Tournament Quest、Tower of Grief、Tower of Dazzlement 地图覆盖。完整历史数据改由校验原始 SHA256 的测试夹具保存，压缩后 563,393 字节；运行路径只使用准备好的 PVF 投影，材料目录缺失时在访问存储前拒绝启动。旧经验 JSON 仅留作离线解析测试夹具；塔导出命令要求显式指定输出路径。完整逐域对照及验证记录见改造计划中的第七轮。

第八轮删除 `loot.level150.json`（3,278,355 字节）。它不供生产路径读取，仅供完整掉落行为测试；测试使用 184,835 字节 gzip 快照并验证原始 SHA256，净减 3,093,520 字节（约 2.95 MiB）。Odyssey 源审计器改用 `gamedata.Open/Source`，且必须显式指定 `-output-dir`，不会再写入 configs。`loot.next25.json` 仍有 GM/JSON 模式消费者，本轮保留；掉落上限仍由现有策略定义。

第九轮任务装备与抽奖收口：

删除 `quest-equipment.current37.json`、`quest-equipment.next29.json`、`lottery-item-pools.json`、`lottery-equipment-pools.json`，共 7,111,136 字节；顶层 JSON 75→71。抽奖完整历史快照保留为 476,221 字节 gzip，仅供测试读取并核验原始 SHA256；本轮内容净减少 6,634,915 字节（约 6.33 MiB）。

- 抽奖运行 API 只接受准备好的原生 PVF 表，不读取旧路径或历史 baseline；启动不再构造退役奖池路径，启用抽奖而缺少原生域时明确拒绝。奖励、数量、权重、发现范围和不可发放奖池拒绝规则保持。
- equipfields、questequipmentimport、equipmentwearimport 统一使用 `gamedata.Open/Source`；两个导出器要求显式输出，旧 JSON seed 参数明确拒绝。基础装备选集继续来自既有 policy，任务装备选集由同源物品与任务构建。
- charactercheck 从 PVF 构建任务装备，内容哈希与原生任务目录核对，角色 ConfigVersion 单独与 SaveIdentity 存档契约核对；不把存档版本当作 PVF 哈希。旧 next29–34 探针不再自动注入退休 JSON，较新候选显式保留当前装备表。`equipment.current35/37.json` 仍有独立消费者，本轮保留。
- Go 1.26.5 全量 `go test -count=1 ./...` 和 `go vet ./...` 通过；Python 3.11.9 启动检查 10/10 与探针语法/参数设置检查通过。当前环境未挂载 PVF，真实归档输出/完整原生对照测试未执行；归档门禁仍保留。未启动客户端、运行服务或玩家库，没有改 PVF、schema、存档或用户 `.gitignore`，不扩展实机 confirmed baseline。

## 2026-10-03：剩余装备 / 掉落运行读取收口（源码候选）

loot 与 equipment-selection 的运行 JSON 回退、隐式 baseline 已移除，加载 API 必须有已选择且准备好的原生目录。wireprobe 启用掉落或任务装备时，在访问存储前检查所需领域；原有等级上限、排除项、概率和发放规则继续使用既有政策。

- charactercheck / audit36 统一使用 `gamedata.Open/Source` 及既有 policy；基础装备选集使用同源空任务目录保持政策基础范围，任务扩展另走原生任务。BagRules 使用实际内容 checksum；角色 ConfigVersion 仍与 SaveIdentity 核对，不修改角色存档或别名化来源哈希。
- GM 正常运行继续用原生 PVF，退休无调用的 JSON fallback 构建器；离线装备部位导出直接使用原生 ItemDisplay，读取失败不覆盖现有产物。initialrepair 删除不可达 JSON 分支，lootimport 使用 Source 并要求显式输出。当前源码探针用 PVF 领域激活标记，历史二进制需要显式提供匹配的历史装备输入。
- 本轮不增加整表 gzip 快照。`loot.next25.json`、`equipment.current35.json`、`equipment.current37.json` 暂时保留为旧测试输入，顶层 JSON 仍为 71 个，不能把运行退役误报为文件已删除。后续普通行为测试优先小型输入，当前内容校验用真实 PVF；完整历史快照仅在确有版本比较需求时保留。
- Go 1.26.5 全量 `go test -count=1 ./...` 与 `go vet ./...` 通过；Python 3.11.9 启动/profile/身份接线检查 26/26 及探针语法检查通过。当前没有挂载 PVF；真实归档专项仍待执行，未运行客户端、服务实例或玩家库，没有修改 PVF、schema 或存档，不扩展 confirmed baseline。

完整审计与迁移顺序见根目录 [PVF 单一内容真源改造计划](../../../../docs/todo/pvf/PVF单一内容真源改造计划.md)，逐文件源码线索见 [configs 字面引用清单](../../../../docs/todo/pvf/configs字面引用清单.md)。其他尚未解除的运行依赖继续逐项迁移。
