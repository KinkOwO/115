# 冒险团精锐助战：普通、剧情与奥德赛接入

状态：普通/剧情与奥德赛首场APC出现、奥德赛单次2062快速进入及目标场APC出现已确认；完整死亡采集、连续直进与其它独立机制仍待证。

## 目标与范围

业主授权不改 PVF、允许编写 DLL，取消成为 APC 的 100 级及二次觉醒门槛。保留角色真实等级、觉醒、装备和已学技能。首版支持一名玩家带最多三名本账号角色，复用原生精锐 UI、模式 2 名单和技能使用设置；普通及奥德赛来源角色均可成为 APC。

普通战斗副本及其剧情地图、奥德赛战斗副本及其剧情地图纳入目标。真人的入场资格、单人难度、疲劳、禁用消耗品、复活限制和死亡失败规则继续按对应 PVF 定义执行。收益、任务进度和奥德赛章节奖励只归真人。军团、攻坚战、多人同步、教学/训练/塔以及城镇剧情展示场景不纳入首版。

## 依据与资源归属

- 官方资料仅作线索：[精锐角色介绍](https://www.dfoneople.com/news/updates/1805/Summary)、[黑鸦组队模式](https://www.dfoneople.com/news/updates/2424/Black-Purgatory-Guide-and-Squad-Mode)。当前行为以 `client/DFO.exe.i64` 与当前 PVF 为准。
- 当前内容脚本 `mycharacters_apc_contents.etc` 的 contents key 2 定义 36 个原生内容副本；本次扩展是业主指定的调服差异，不修改源内容树，不另设人工地图 ID 表。
- 已检查源内容表没有等级/觉醒门槛；限制来自客户端 `142E5F7E0`、`142E67D80`、`142E6B1F0` 的十处比较，以及服务端 `cmd/wireprobe/adventure_elite.go` 的两处同义校验。服务端两处校验仅在 `DFO_ADVENTURE_ELITE=1` 时跳过，关闭保留原生门槛，客户端仅调整该资格比较，不修改受保护的等级读取函数或角色状态。
- 名单继续保存稳定角色 ID，发包时按当前账号排序投影为槽位。原有 SQLite schema 和账号 JSON 字段不变。
- 取证文件：`analysis/tasks/adventure-elite-presence-evidence-20261008.json`、`adventure-elite-unrestricted-design-evidence-20261008.json`、`adventure-elite-special-party-compatibility-20261008.json`、`adventure-elite-ordinary-entry-gate-evidence-20261008.json`。

## 实施顺序与启用门槛

| 阶段 | 实施内容 | 交付/启用条件 |
| --- | --- | --- |
| 0 | 隔离工作区，建立 build/vet/test 基线；补齐当前 PVF、IDA 与生命周期证据 | 不改运行路径；保留其他任务存档 |
| 1 | 删除服务端资格限制；制作客户端资格补丁，核对五段完整上下文、十个比较立即数 | 本地字节、补丁机制、低等级保存/加载与真实快照测试通过；仅资格候选 |
| 2 | 普通/奥德赛原生 UI 与模式 2 加载适配 | 精确调用范围、N1382 容器与 N1879 克隆链确认；不能全局伪造频道 |
| 3 | 冻结名单、生成入场上下文并注册原生 type 5 APC | 注册、owner、AI、攻击/击杀/召唤报告链与回收已闭环；未满足就不启用 |
| 4 | 房间、直达下一副本、死亡、退出、掉线与剧情层适配 | HP/MP、冷却和死亡连续性闭环；不改剧情通关条件或 BOSS 门条件 |
| 5 | 手动实机分阶段验收与发布 | 业主操作，记录会话/日志；通过后才更新 confirmed baseline 与默认程序 |

阶段 2–4 的证据缺口不能用强行置位地图字段、猜包、将 type 5 改成 type 3、仅出现模型或者复刻调用列表来替代。

## 原生调用契约

`CMD1719 → NOTI1754 + ACK1719` 保存名单；城镇 `CMD1811(mode=2) → NOTI1382 → NOTI1879(mode=2)` 加载真实装备和技能，由 `142E5B060 → 142E650D0 → 142E5CD00 → 142E653A0` 创建原生 AI 同伴。保留已有布局和顺序，不新增 opcode。

`142E60CF0` 有多个非 UI 消费点，禁止全局返回 true；`142E5EDA0` 的频道模式推导也不能全局改写。N1382 必须只在本扩展加载事务范围内消费到 type 2 容器，其余原生玩法原样执行。

`145B22F50` 的精锐注册段使用 `145DE4360(..., type=5, ...)`，还执行身份分配、位置、原生对象注册和 AI 回调。只允许在完整安全条件满足后适配该段，不能照抄匿名虚表参数另造 APC。

**已订正的地图字段**：`dungeon+6264` 是副本条件完成状态。构造函数清零；通知 312 处理器 `1452FF500` 写入条件号及完成标记，并经 `145B459E0` 设置 6264 和关联的 8056；`145EA0640` 在初始化中清零。现有服务端只有 PVF 指定的 BOSS 入口条件达成后才发送此通知。禁止提前设置 6264/8056 或补发 N312 来触发助战，避免解锁剧情/奥德赛 BOSS 门。

## 服务端接入契约

城镇准备阶段校验所属账号、排除真人、去重、校验已学技能、生成只读装备技能快照，准备完成才允许自动带入。入场前冻结稳定 ID 与当次槽位关系；副本中禁止重建名单或重载同伴。

普通进图入口与 direct move 必须共同校验冻结上下文。direct move 不经过 `prepareDungeonEntry`，需在旧副本正常结束后复用有效准备快照；角色/装备/技能变更或快照失效时要求回城重新准备，不能战斗中补发资料。

攻击者与召唤物必须通过已验证的 native owner 链归于真人，校验当前副本/房间实体后复用现有杀怪推进。不得把未知 actor 无条件接受为玩家，也不得让 APC 获取个人收益。单人副本的人数系数维持一名真人。

普通剧情按当前脚本的战斗层与到达最终地图条件执行。展示角色与 NonCombat 对象不可作为 AI 目标；不抢过场控制，不以击杀展示 BOSS 代替原生到达地图通关。城镇剧情场景、教学和训练继续走各自原生入口。

奥德赛保留真人身份校验及独立内容条件。不能用进程级 Odyssey 开关改变 APC 来源角色身份，不能给 APC 自动成长、觉醒、发奖或推进章节；真人死亡立即按现有规则失败。

## DLL 与发布

DLL 放 `client-patchs/adventure-elite/`，x64 `/MT`，以 schema 2 `file.add` 安装到 `.115us-mods/`，依赖 `qol.client-host`。日志和状态 JSON 均解析自身模块路径，写在 DLL 所在目录。旧 INI 不再参与控制。

资格候选使用 ASLR RVA 和完整上下文逐字节核对，仅将五组资格比较的立即数 `100/2` 调整为 `0/0`；负值继续被拒，合法角色等级仍由服务端存档校验。所有现场字节先验证，全部目标页先取得写权限，再逐字节原子修改；失败回滚已修改字节。服务端和 DLL 共用环境变量 `DFO_ADVENTURE_ELITE`，仅精确值 1 启用，默认关闭。此阶段状态明确标记 ordinaryReady=false。

完整接入必须先初始化全部必需钩子，再允许自动加载/入场。持有 native 弱引用及会话代次，不能跨帧保存裸对象指针。卸载先退出客户端，再卸包；不覆盖 EXE/PVF，重新启动恢复原生内存字节。

Go 构建 `bin/wireprobe-handoff-source.exe` 服务端与 `bin/dfolauncher-adventure-elite.exe` 启动器候选，不覆盖默认程序或 `wireprobe-dungeon39.exe`。每次改动运行 build、vet、全量 test，对照基线逐名检查新增失败。

## 验收顺序

1. 资格补丁现场字节与状态文件正常；低等级、未觉醒角色可选，真实属性/技能不变。
2. 单人城镇名单保存、重登重映射、资料加载顺序和弱引用建立，无循环请求。
3. 一名 APC 普通地图注册、攻击、击杀与房间切换，再扩到三名；单次只验证一个假设。
4. 死亡/复活规则、退出/掉线/再次进图、资源连续性及 direct move。
5. 普通剧情战斗层、过场与最终地图到达；奥德赛重复对应流程并核对死亡失败与真人奖励。

每一步记录角色、地图/模式、服务端会话目录与 DLL 日志。实机仅由业主手动操作。包发出或模型出现均不构成战斗闭环。

## 本轮进展与待证

- 业主截图确认 1 级角色可以选入并保存，实机会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261008_085305_229339_next37` 的 CMD1719 → NOTI1754 → ACK1719 与自身目录 DLL 状态十字节生效一致；当前角色/重复选择仍被拒。没有 CMD1811，本次确认不包含资料加载、重登或进图战斗。详见 confirmed baseline 的精锐资格局部确认。
- 阶段 2 新取证：N1754 的两个 `142E5EDA0` 调用选择当前模式；普通频道为 0，因此保存后不请求模式 2。N1382 reader `1444FCF90` 的三个 `142E60CF0` 调用共同控制 type 2 容器、当前角色合并与槽位对象建立；不能只改一个判断。`146CF32E0` 是城镇更新/入口消费点，不能把它误记为 N1382 reader。作用域和生命周期仍须确认后再启用新运行路径。
- 已实现 v0.2.1 原生通知观察候选：从 `14599D5D0 → 1459A3DD0` 确认 primary map 节点和函数/参数对象，再由 `1459A3BB0` 确认回调 ABI `(opcode, opaque)`。三个原始回调核对后使用对齐指针 CAS 接入；有界链表、全槽拒绝和参数原样转发机制测试通过。只读取 N1754/N1382/N1879 的头部、读取长度和弱引用存活数量，原生处理器原样调用，不改变 C2S/S2C 路径，因此不增加协议尝试次数。完整静态证据与实机资格锚点见 `analysis/tasks/adventure-elite-native-notification-observation-20261008.json`；动态安装/命中尚待玩家操作。
- 远端同步已完成只读核对与主分支 fetch：本机 HEAD `29b48b06`，实际 gud/main `54ee3b56` 领先 61 个提交，涉及当前工作树他人的 `internal/gamedata/catalogs.go`、`scripts/repair_profile.py` 和本任务共享文档/脚本。没有执行覆盖、stash 或合并这些并发改动；资格收口文档已更新，提交待安全整合远端。

- 隔离 stash：`79ad065604a1bb195fc1c2f97d088512c3af1a77`；保留既有 stash，未恢复其他任务源码。IDA 活动数据库和两份大导出留原位，详情在忽略目录 `.tmp/adventure-elite/workspace-isolation.json`。
- 隔离后的 Go 1.26 build、vet、全量 test 基线均通过。
- 6264 的来源/写入链已确认；它不是通用 APC 地图许可位。
- 已读原生注册函数、三槽清理 `142E63BA0`、管理器析构与 NOTI1754 重建路径。弱引用/延迟回收存在，不能以直接 delete 替代。
- 阶段 1 资格候选已构建，补丁机制、当前 EXE 五段现场上下文和 schema 2 包大小/哈希校验通过；Go build、vet、全量 test 均通过，无新增失败。完整取证补充见 `analysis/tasks/adventure-elite-lifecycle-evidence-20261008.json`。本机尚未找到 modkit CLI，因此 install/uninstall 验证及用户客户端安装未执行。
- **尚待闭环**：普通/Odyssey type 2 容器的事务范围；type 5 攻击/击杀/召唤 owner 报告；房间/新副本及死亡时资源与引用连续性；当前候选的实机命中。这些阻断完整接入启用，不影响独立资格改动的源码与机制验证。

## 第一版资格候选构建与验证记录（INI 已被环境变量替换）

| 实际命令/检查 | 结果 |
| --- | --- |
| Go 1.26 `go build ./...` | 退出 0 |
| `go vet ./...` | 退出 0 |
| `go test ./... -count=1 -json` | 退出 0；基线 3504 个通过，本次 3511 个通过；两次均 221 个跳过；失败集合均为空 |
| `go build -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe` | 退出 0；候选 29,627,904 字节，自述版本 dev；Go 1.26.0、windows/amd64，基于 revision `29b48b061d0d576309a3d2fd1583197063d7c58b` 的未提交源码 |
| `scripts/build-adventure-elite.ps1` | 退出 0；MSVC x64 `/O2 /MT /W4 /WX`；机制测试通过 |
| DLL ABI/路径检查 | 在 Python 进程中加载并调用真实 `ModStart`；非游戏现场拒绝修改、默认关闭、自身目录 INI/日志/状态正确；未启动游戏 |
| `dumpbin /imports /exports` | 仅依赖 KERNEL32.dll；导出 ModStart/ModName；无 VC 动态 CRT 依赖 |
| EXE 指令与包检查 | 五组完整上下文与磁盘 DFO.exe 一致；AMD64 DLL、schema 2、宿主依赖、大小及 SHA256 通过 |
| `scripts/check-commit-hygiene.ps1 -All`、`git diff --check` | 通过；没有暂存或提交 |

资格 DLL 版本 `0.1.0`，161,792 字节；SHA256 `eeb36c4cd3f1c029d63ce33414cc7a7a861c4b2b145eb6008b018e1c5ec57c44`。包路径 `client-patchs/adventure-elite/dist/adventure-elite-eligibility-0.1.0.zip`，87,125 字节。状态中的 ordinaryReady=false 是明确边界，不能将本轮交付当作完整副本兼容。

验证记录留在忽略目录 `.tmp/adventure-elite/`。旧服务端候选备份为 `.tmp/adventure-elite/before-wireprobe-handoff-source.exe`。默认服务端、EXE、PVF、SQLite 存档未改；没有启动游戏、安装 mod、更新 CHANGELOG/confirmed baseline 或提交其他任务文件。

验证范围说明：全量测试结束于本轮日志的 03:20:01，服务端候选构建于 03:20:54。之后另一个任务新增 `buffer_rental` 源码并修改 `internal/gamedata/catalogs.go`（03:25:34）；这些改动保留原位，未纳入本轮候选或上述测试结论。重新构建当前共享工作区时，需要与该任务协调并重新执行 Go 门禁。

## 环境变量启用与启动注入

业主本轮明确要求“启用就在服务端启用+启动游戏脚本注入”。统一开关 `DFO_ADVENTURE_ELITE` 来自启动脚本调用进程；只有精确值 1 开启，其余关闭，profile 不承载此开关。服务端读取 `internal/adventureelite.EnvKey`，保存/加载均同步绕过原生资格门槛；关闭后恢复两处原生比较，保留已有名单及真实角色存档。

`storage-route.ps1` 在启用时选择隔离 Go 启动器；Go 的 launch/check 固定选择服务端资格候选，显式 profile 也只保留其内容配置。游戏步骤由 Go 宿主注入 dist 资格 DLL，现有汉化 wrapper 保留。注入只接受本次 Job 的正确路径与原生 x64 客户端，远程地址按实际提供模块与 RVA 计算，限时 30 秒；`ModStartInjected` 校验客户端继承同一开关，初始化失败结束本次启动。旧 INI 忽略，日志仍在 DLL 目录。

UAC 使用 UTF-16 EncodedCommand 携带 Base64 JSON 数据，显式传递精锐开关、奥德赛模式及原始参数，防止提权丢失临时环境变量或 Unicode 路径。奥德赛 CMD 原先引用已删除的 Python/外部启动器，本轮改为同一仓库内 Go 链。关闭时不注入并使用默认启动器；环境变量不是热开关，需退出并重新启动会话。

直接运行期注入是本轮用户指定入口；schema 2 包仍保留插件形态，modkit 安装/卸载未验证。环境开关不解除普通、剧情、奥德赛自动带入的证据门禁，`ordinaryReady=false` 保持。

具体文件、构建命令及手动启闭见 [资格候选 README](../../client-patchs/adventure-elite/README.md)。

### 环境变量候选实际验证结果

| 命令/检查 | 结果 |
| --- | --- |
| Go 1.26 `go build ./...`、`go vet ./...` | 均退出 0 |
| `go test ./... -count=1 -json` | 退出 0；3553 个测试用例通过、203 个用例跳过（含包级事件共 222 次 skip）；基线/本次失败集合均为空，逐名比对无新增失败 |
| `scripts/test-adventure-elite-launch.ps1`（Windows PowerShell 5.1） | 退出 0；精确启闭、模拟提权丢失环境、Unicode/引号参数均通过；只运行临时测试入口 |
| Go Windows 注入测试 | 临时测试进程、无导入测试 DLL：成功返回 0、拒绝返回 13、错误目标路径与缺失 Job 均验证；没有运行游戏 |
| `scripts/build-adventure-elite.ps1 -WithServer` | 退出 0；C++ 资格机制通过，DLL/schema 2 包及两个隔离 Go 候选均构建成功 |
| 实际 DLL ABI/环境检查 | 六种环境值、旧 INI=1 被忽略、重复初始化返回一致、禁用直注入返回 203、启用的非游戏现场拒绝修改；日志/状态仅写各测试 DLL 所在目录 |
| `dumpbin /exports`、包大小/哈希检查 | 导出 ModName/ModStart/ModStartInjected；schema 2、宿主依赖、file.add、DLL 内容与 SHA256 一致，包中没有 INI |
| `scripts/check-commit-hygiene.ps1 -All`、`git diff --check` | 均退出 0；没有暂存或提交 |

最终产物：服务端候选 29571584 字节，启动器候选 16537088 字节；均 Go 1.26.0/windows/amd64，revision `29b48b061d0d576309a3d2fd1583197063d7c58b` 的共享未提交源码（dev）。DLL v0.2.0 161792 字节，SHA256 `7f0b4d3c9049ae24ed8a5eb77ac11fc8c1bb8a1051e0d5e1c2cc90afd55f214e`；包 88136 字节。

源码变化：`internal/adventureelite/env.go` 统一服务端/启动器键名；`cmd/wireprobe/adventure_elite.go` 两处条件资格比较及其 SQLite 回归；`internal/launcher/adventure_elite*` 提供预检/注入与机制测试，`clienthost.go`/`clientrun.go`/`launch.go`/`check.go`/`env.go` 接入路由；资格 DLL/打包器、构建及启动脚本、候选 README/本计划同步更新。

本轮门禁与候选基于当前共享工作树，包含已恢复写入的其他任务源码；没有暂存/回滚/提交其文件。默认服务端、默认启动器、EXE/PVF、SQLite 存档未改。未运行游戏，也未进行实际 UAC 提权、汉化联合实机、modkit 安装/卸载或普通副本带入验收；这些不能从离线测试推导通过。验证详情在忽略目录 `.tmp/adventure-elite/env-delivery.json`、`env-test.jsonl`、`env-dll-smoke.json`。

### 2026-10-08：名单再次进入回显与原生通知观测确认

业主反馈“已保存，重新进入数据也存在”。会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261008_104716_042826_next37` 的 client.log 第 5 行确认注入；events.jsonl 第 198–200 行记录 CMD1719、N1754 同步及保存成功 ACK。DLL v0.2.1 状态 `nativeTraceReady=true`，原生日志第 1–4 行有两次 N1754 的处理前后命中；两次剩余读取长度均为 544 → 10，三个弱引用存活数均为 0。确认名单保存和再次进入的回显，不据此扩展为完整退出登录/重登、资料加载或 APC 创建验收。没有 CMD1811/N1382/N1879。

观测字段订正：v0.2.1 将 N1754 的记录保留字节误标为 `headerMode`；旧日志该字段作废。当前 reader `142E5A753` 复制 531 字节记录，`142E5A7AA` 从记录 `+0x0d` 取模式，因此首行模式位于完整正文 `+14`。v0.2.2 修正这一个诊断偏移并补长度/空名单/原生布局机制测试，不修改发包或原生消费路径，不增加 C2S 尝试。普通、剧情、奥德赛仍为 `ordinaryReady=false`。

## v0.3.0 普通城镇准备候选（未实机）

普通事务适配 attempt 2/3，原生资料包 layout 延续 attempt 1/3。已实现 N1754 原生模式 0 两处指定调用点选择 2；实际 CMD1811 native flush 才绑定线程/原生频道值/主人 wire ID/10 秒事务。N1382 三处判断和 N1879 fallback 仅在匹配事务内选择 type 2，原始函数与克隆/技能应用原样执行。固定 MinHook v1.3.4 源码/许可证，十一段 EXE 上下文匹配，拒绝不符宿主。

PVF 链：etc/channel_info.etc [server] → ImportChannelInfo → gatewayRuntime.channelInfo → 当前 server/发布 ID 普通行；etc/clientchannelinfo.etc 专用队伍标记及 channelslotinfo.etc 面板 → ImportChannelDirectory → 专用频道优先拒绝。内层 checksum 4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f；普通脚本 SHA256 aecc92480ccd469fa2b33bf9c189496a0c4e31c20eb2fef7e1623642c23a687f。真实源逐行测试通过；106 是 isSemiRaid，119 是 isLegion/isSemiRaid，不作普通扩展。此前日志 preparedChannel=121 是原生 getter 值，不能据此命名为服务端奥德赛频道；本轮仅核对其事务连续性。奥德赛角色沿用已有普通频道发布路线。

重复规则台账：channel.local34/35.json 仍手写普通 Type/Area/SourceValues（如发布 ID 10 的本地 Type 22），与 channel_info.etc 存在兼容覆盖。保留既有发布行为，以原生普通 ID 行确定范围，不新增第二份内容表。建议另行按频道兼容证据迁移覆盖，本轮不修改其他任务配置。

服务端下发视图对不支持路线隐藏模式 2，保存名单不变。冻结会话主人/名单/设置；重复加载拒绝，准备后拦普通进图，直到 battle 注册/owner 闭环。清空名单解除保护。SQLite schema、稳定角色 ID、真实等级/装备/技能不变。资格/保存回显已确认，城镇创建仍属候选。

隔离工作树基于 gud/main 54ee3b56，未纳入共享工作区他人未提交源码。Go build/vet/test 均退出 0，3633 例通过、280 例跳过；与整合前基线逐名对照，失败集合均为空。真实 PVF 定向测试另行通过。C++ 四组机制、十一段磁盘上下文、六环境值 ABI、固定源码哈希/schema/许可证通过。修正合成 CALL 夹具相对偏移不变更游戏包布局或增加实机次数。

下一步按 [候选 README](../../client-patchs/adventure-elite/README.md) 退出旧会话后启动，在城镇打开精锐页保存一次、等待数秒、正常退出；核对 CMD1811 → N1382 → N1879 与弱引用。暂不进图；状态 ready/模型不代表普通/剧情/奥德赛战斗、归属报告、切房与死亡回收闭环。证据见 analysis/tasks/adventure-elite-ordinary-preparation-evidence-20261008.json。

提交等待此前门禁二次确认：隔离树 configs/pvf-default.json 的默认 bin/wireprobe-pvf.exe 缺失；不影响隔离候选编译/手动测试。没有暂存、提交、发布默认程序或改 PVF/存档。新城镇阶段未写入 confirmed baseline。

## v0.3.1 先配置采集、再做一次实机

业主指出已第三次重复保存，要求先收齐日志。此前资格/回显不再重复验收。v0.3.1 相对 v0.3.0 只增强观测和采集，不改变包布局、原生判断作用域或角色存档，不增加协议试包次数。

客户端日志按 DLL 自身目录的 processId/runTick 分开命名；N1754 上限 128 次，N1382/N1879 各 64 次，独立预算避免保存刷屏挤掉后续包。每次记录 arrival（身份 getter 前）、before、after、scope-closed；含真实原生返回值/最终返回值、指定 caller 位图、owner/频道、pending 线程/身份/时限/阶段、包头同伴数、三个弱引用强计数、reader 成功或异常 code/ip、丢弃数和上限事件。异常过滤只记录并继续原生异常传播，机制测试验证无吞异常。安装失败记录精确字节校验位置、hook 状态或 reader/通知表步骤。

服务端新增 adventure_elite_transaction_decision：区分接受/拒绝与实际发送，保留期望主人 wire ID、持久角色 ID、server/发布频道/频道类型、奥德赛/源范围/等待态、冻结名单及计划包体长度/SHA/hex（单包 256 KiB 上限，超出明确标记）。原有发送完成事件仍是实际发包证据；不新增第二次 DB 读取或修改包体。

scripts/collect-adventure-elite.ps1 -CheckOnly 只读核对三件配套候选哈希及 EXE/PVF/sk.dat/内层归档身份，不启动游戏、不创建文件。普通采集按 ROOT_PID 与 DLL 状态匹配会话，只写 runtime/adventure-elite-captures/，复制服务端日志、DLL 状态与本次原生日志，生成精锐事件摘录/summary。旧版本/错误 PID、缺阶段、未命中请求、reader 异常、丢弃/上限、资源身份变化分别列出；不能把采集不全归为玩法失败，不能认证战斗。脚本可重复运行而不需要再次保存名单。

下一次仅进入普通城镇、打开精锐页查看已有名单、停留约十秒、正常退出；已有名单恢复会触发候选加载，不要求重复保存。若未触发也先从 arrival/作用域/模式/发送点/服务端拒绝定位，不增加无依据的操作。没有名单时才需选一名其它角色保存一次。暂不进图。采集由 agent 读取本机日志执行，无需玩家搬运日志。

证据及实际结果见 analysis/tasks/adventure-elite-logging-evidence-20261008.json。本阶段没有新的实机确认；之前隔离树默认程序缺失的提交门禁仍待二次确认。

## 已完成日志取证后的装备槽修正候选

实机会话 `20261008_155318_415820_next37` 已命中 v0.3.1/PID73516：普通频道 scoped mode 0→2、CMD1811 发出与服务端接收成立。未创建同伴的具体阻断是 `TagEquipment` 拒绝槽36，尚未发出 N1382/N1879。当前 `list/equipment.lst` 引用 `equipment/character/common/primer/100401592.equ [equipment type]=[primer]`；IDA 1452C1540 的48格标记及清理循环、145962630调用链和当前EXE逐字节验证支持0..47。服务端扩展同一行校验，不删除装备、名单或修改等级。此为普通事务 attempt3/3，原生布局保持 attempt1/3。

当前两名已选角色的源目录/存档副本离线验证通过，完整 N1382 为7449字节；低等级角色仍为1级，另一名115级角色保留36..45槽位。Go build/vet/test通过，3656通过、280跳过、无新增失败。客户端 clone、战斗、剧情/奥德赛及特殊队伍仍未验收；下一次只需正常启动、查看已有名单、停留10秒后正常退出，先不进图。详情与日志行号见 [装备槽证据](../../analysis/tasks/adventure-elite-equipped-slots-evidence-20261008.json)。

## 静止城镇取证与下一次验证前提

实机会话 `20261008_162334_154180_next37` / PID54384 显示三个槽位为空；本次 CMD35=0、CMD1395=2、CMD1719=0、CMD1811=0，没有名单通知或同伴资料通知。代码链为 `world_flow.go::handle(35)` → `notePositionReport` 设置 `adventureReady=true` → 2秒串行轮询 → `refreshAdventure` → N1754；CMD1395 当前只回复冒险团详情。故此前“只打开窗口等待10秒”的步骤不充分，尚未验证装备槽修正或 clone。

SQLite 以 `mode=ro` / `PRAGMA query_only=ON` 核对账号1的 `account_adventures.data.elite_selections`，模式2仍为 `[1,3,0]`。三件候选哈希均与上次交付一致，客户端正常退出，没有溢出 CMD217。采集脚本成功冻结本次原始日志，原生日志缺失与没有对应通知一致，不能据此声称客户端创建失败。

下一次用现有候选正常进入普通城镇，先短距离移动一次，再查看精锐页、停留10秒、正常退出。不重新保存，不进图；由 agent 对照 CMD35 → N1754 → CMD1811 → N1382/N1879 与原生弱引用。普通事务已是 attempt3/3，本轮不增加无依据的第四次运行路径，也不把候选写为确认基线。

后续待办：取消名单恢复对首次移动的依赖；须先核实不移动时的原生主人/频道就绪时机及允许的恢复入口，沿根 AGENTS §0.1 取得明确的新取证范围后再处理。当前证据及命令见 [静止城镇记录](../../analysis/tasks/adventure-elite-static-town-evidence-20261008.json)。本轮只改证据和操作文档，未改 Go、DLL、PVF 或存档；无需重复构建，沿用已通过的3656/280/0候选。

## 名单恢复确认后的选图生命周期候选

业主明确确认页面名单正确显示；会话 `20261008_164902_605424_next37` / PID51160 完整命中 N1382 → N1879，并创建两个原生 AI 弱引用，无 reader 异常。这结束普通资料准备 attempt3/3 的首次编码/克隆取证；并不代表战斗或稳定保留已验收。

随后日志给出独立生命周期缺陷：CMD15进入选图将 `selectingDungeon` 置真，轮询因名单视图复用加载资格而发送空 N1754；CMD132返回后又发送模式2名单。当前原生 N1754 在名单变化时调用142E63BA0清除同伴，再发CMD1811；服务端被冻结准备拒绝重载，最终 weakAlive=0。证据闭合后仅做选图生命周期 attempt1/3，不继续猜普通资料布局。

源码将普通名单显示与城镇加载资格分开：源定义、专用频道、隔离/教学/训练等范围仍保留；普通选图、场景加载/返回不改变模式2名单；在选图/场景中或已有冻结快照时仍拒绝加载。轮询继续仅在实际名单字节变化时下发N1754。临时SQLite回归验证冻结主人/名单不变、各状态下重复轮询不发送销毁通知，实际源变为军团属性时仍隐藏。

原生日志、服务端日志及原始资源哈希已完整采集；汇总新增 latestNativeObservation、cloneReleasedAfterCompletion、cloneAliveAtLastObservation，首次创建后再释放不能继续报告为终态存活。DLL保持0.3.1；不改PVF、SQLite结构、角色存档或默认程序。现有普通 Type22 与发布ID10的兼容覆盖按此前台账保留，本次不新增地图/频道内容表。

下一次仅验证“城镇加载→打开选图→取消返回城镇→查看名单”的对象保留，仍不选择副本开始战斗、不重复保存。需要N1754仅初次一次、CMD1811仅初次一次、N1879后weakAlive=2，选图往返无空名单或重复恢复/加载；手动由业主进行。通过后再推进原生type5注册与owner链，不能直接解除战斗门禁。详情见 [生命周期证据](../../analysis/tasks/adventure-elite-selection-lifecycle-evidence-20261008.json)。

交付：隔离树已快进合并gud/main `48f22d4f`，逐份保留14个任务文件与远端意图，未暂存或提交。合并前3660通过/280跳过，合并后Go build/vet/test再次全部退出0、3717通过/280跳过、失败集合仍为空。候选服务端 `3f773468`（30,486,528字节）已落地；默认程序哈希前后相同，DLL/启动器不变，schema2包更新操作说明且完整校验通过。回退及自述版本见 `.tmp/adventure-elite/transition-delivery.json`。采集只读预检明确只核对磁盘候选和旧DLL版本，不认证旧会话使用新服务端。

## 普通选图取消返回确认与战斗身份缺口

业主反馈步骤2/4正常，步骤3副本显示未变化。`20261008_180054_958631_next37` / PID39224完成实际两次CMD15→CMD132，只有首次一次资料加载，返回后查看名单的4个CMD1395与原生caller采样数量一致；第55个生命周期样本仍有两名type5对象，三字段弱引用绑定完整。全部71个样本一致、无丢弃/上限，确认名单恢复和选图取消返回对象保留；本轮无需重复保存或再做同一实机。

仍有明确下一步缺口：同伴+74020和控制器+112均为65535，当前玩家wireID=2。权威IDA完整读取145C76D90、145EFA8D0、145EFA4B0与145EFD6B0后确认原生控制器默认初始化0xFFFF（145EFA8B5），绑定函数145F04E30按控制器字段回写同伴。不能将该字段直接写成玩家2，也不能把弱引用绑定冒充攻击归属闭环。后续先核对原生type5注册是否会分配独立控制器ID、持有者表收录及攻击/击杀消费，再设计有限适配；普通进图保护、6264条件位、剧情/奥德赛和专用队伍边界继续保持。

此次仅采集日志、读取IDA并更新确认记录；Go/DLL候选与PVF/schema/存档不变，尝试计次不增加。 收口前已快进合并gud/main 55099b2a，并三方保留双方改动；合并后重新Go build/vet/test均退出0：3733通过、279跳过，按测试名比对既有失败集为空、无新增失败。隔离树补冻结默认EXE的忽略副本后门禁通过；本轮实机使用的运行候选未替换。收口提交需先核实远端并过门禁；隔离工作树可补现有冻结默认EXE的忽略副本解决缺路径，不改已跟踪profile、不发布默认程序。详细锚点见 [生命周期控制器证据](../../analysis/tasks/adventure-elite-lifecycle-controller-evidence-20261008.json)。

## 城镇恢复复验与下一步原生身份取证

会话 `20261008_172355_230248_next37`（PID85712）第182行仅一次N1754，第184–189行仅一次CMD1811/N1382/N1879；native第11行weakAlive=2，无异常、日志缺口或重复加载，正常退出。但第192–195行是CMD36/普通城镇区域切换，没有CMD15或CMD132；不扩大为选图往返确认。原有选图保留候选仍待此路径实机。

本轮按权威IDB继续读取完整145B22F50注册、145F04E30绑定、146EA47F0三字段CRef构造、142E63BA0释放以及+74020读者。已闭合字段绑定与控制器查询用途；攻击/击杀报告语义仍缺证，不能把控制器ID视为已验证真人归属。6264条件位与专用队伍保护不变。

交付v0.3.2只读生命周期采样：沿用原有flush钩子，N1879完成后绑定线程/频道/主人，观察后续原生请求，记录对象类型、控制器ID、三字段弱引用并对不一致读数拒绝结论。通过计次数量一致的CMD1395 caller配对，给选图返回后查看名单提供独立对象存活证据。测试覆盖原生返回值/last-error/异常透传、线程边界/身份变化/采集上限、空/死/竞争引用和13种PowerShell采集夹具。没有新增Go内容规则、数据库结构或地图列表。普通资料准备attempt3/3、布局attempt1/3、选图生命周期attempt1/3保持；只读取证不计C2S尝试。

下一次实机合并实际选图取消与准备阶段控制器取证，不重复保存、不开始副本；有这些结果后再实现注册块的有限适配。服务端和启动器沿用上轮3f773468/94296f90，源码Go门禁沿用合并48f22d4f后的3717通过/280跳过/无失败（本轮没有编辑Go）。isolated默认程序缺失的提交门禁仍待二次确认，未暂存/提交。

## 对象身份与主人采样：下一次手动边界

原生 type 5 注册键是 `(kind << 16) | (objectID & 65535)`：145B8C670 读取对象 +356/+360，145B99610 设置同一字段。142E5CD00 克隆阶段已通过 14520C690 分配编号；145B22F50 入场注册段仍会处理与真人相同编号的冲突。控制器 +112 的 65535 是另一字段，本阶段不修改它。

142E5CD00 的 virtual +4864 在当前剑士 vtable 上是 145C77BB0，将当前真人保存在对象 +25264/+25272/+25280 的 CRef；145C6D9D0 读取此引用后还检查原生对象注册表。145DE64C0 的 parent=0 分支将 APC 自己放入攻击数据 owner，不能据此把未知攻击者直接接收为真人。怪物死亡 CMD39 的 killer 来自怪物 +26784，由战斗记录写入；这段 owner → killer 以及召唤链仍待闭环。

v0.3.3 只补充只读采样，记录对象身份、主人 CRef 与当前玩家 CRef 的完全相等、主人字段和场景弱引用，并做同次读取前后稳定性检查。现有 ordinaryReady=false 与服务端进图门禁保留；不修改 6264/8056，不发送 N312，不增加模式或地图内容表。资格与准备事务计次不变，新增采样不改变运行分支。

当前必须取得一次新进程的真实克隆身份/主人样本才能继续适配注册段。业主只需用现有候选入口进入原角色城镇、打开已有精锐页面、正常退出；不重新保存、不重复选图。该采样尚未实机确认，也不作为新的 confirmed baseline。具体链与候选核验见 analysis/tasks/adventure-elite-registration-owner-evidence-20261008.json。

## v0.3.3 城镇身份/主人确认与 v0.3.4 首房只读取证候选

### 精锐对象身份与当前真人主人引用取证确认

业主完成 v0.3.3 操作后的 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261008_184829_613984_next37` / PID17196 / runTick122719062，DLL `8aca1e37`、服务端 `3f773468`、启动器 `94296f90` 及四份原生资源身份均匹配；采集缺口0，client.log第8–9行正常退出。一次CMD1811、12条原生通知记录、42条一致的生命周期样本。两个同伴对象ID为0/1，校验有效；两个主人CRef的control/alias均与当前真人CRef完全相同，主人对象ID和控制器ID为2。双方kind均为5；不能用kind5判为NPC，亦不能把有效对象ID0判为空。

确认范围为这42个城镇观测点的身份/主人引用。APC控制器仍为65535、sceneAttached均为0；这些没有证明场景登记、AI战斗或击杀归属。没有写控制器ID、PVF、SQLite/schema或玩家存档。首房注册/攻击/召唤/切房及剧情、奥德赛、特殊队伍仍待后续；下一候选只读入场观察，不能加入 confirmed 战斗行为。


权威IDA继续闭合：当前剑士vtable14AD21370的+288是145C6D9D0（主人CRef并校验注册表）；+1304/+1312/+1984均为14014CC20返回对象本身。145CE2860只读+74020；145DD0070和145E7B120将该值交给145C3E8A0写怪物+26784，145DC86D0再写CMD39 killer。故当前城镇主人引用正确仍不足以证明攻击自动归真人。后续必须按确切APC对象和已注册场景限制适配，不能在服务端把未知killer或FFFF泛化为真人。

本候选没有实施该适配。原生145B22F50 loader、145F0C980精确caller5B25191和145DE4360精确caller5B2529D只观察；保持8参数ABI、原生结果、last-error、异常传播和TLS恢复。门禁采样读原生调用栈+0x98的scene（原生RSP局部+0x90），记录6264/8056与manager576原值；场景+88/+96/+104向量有界读取并复查首尾，记录CRef匹配数，不能替代原生对象哈希表证明。vtable、+288/+1984函数指针只读、不调用；日志按DLL目录/进程/代次保存，129条上限标记，缺样本显式报缺口。

服务端仅在既有DFO_ADVENTURE_ELITE=1、冻结身份/设置匹配、同一源普通频道、已在选图且本连接首次的单人非剧情普通入口，放行原有进图包。目录仍从同一PVF Source绑定；Tutorial/Odyssey/Tower/HellParty和训练房不纳入本阶段，不建立地图ID表。Domain Select原有等级/源/迷宫校验继续生效。该阶段是一次只读入场假设attempt1/3，不修改N1382/N1879布局，不增加普通准备3/3尝试。进图后拒绝死亡结算、拾取、换房、通关翻牌和连续入口，只允许原生加载和原有回城；只改变隔离候选，默认程序保持。

下一次手动：退出旧进程，用`scripts\启动游戏-SQLite.cmd --source-build`进入原角色；城镇短距离移动以恢复已有名单，不保存、不重复选图取消。选择能正常进入的基础普通副本（普通模式、非剧情、非奥德赛），只进首房、停留5–10秒、用Esc菜单返回城镇并正常退出。不要攻击或走门；没有出现同伴是本候选允许的观测结果。若源入口被拒，先读拒绝理由，不要反复尝试其它包/模式。由agent自动采集同次服务端CMD16/37/42和原生前后/门禁/场景/清理记录，判断下一步局部注册适配所需真实边界，不把日志完整等同战斗确认。

SQLite结构、角色属性和名单保存方式不变；正常进图原有真人疲劳记费仍由当前源/服务执行。现有普通频道Type22/发布ID10兼容覆盖沿原台账保留，不复制PVF内容规则；剧情、奥德赛是后续支持目标，并未取消。候选需在合并实际远端后build/vet/test与C++/采集机制门禁通过才部署。

注入启动竞态：全量测试及重复机制测试发现 Module32First 偶发 ERROR_NO_MORE_FILES。仅将空模块快照纳入既有30秒重试边界，保留路径、Job、x64、字节和返回码门禁；回归覆盖空快照后成功、超时及拒绝权限。依据：https://learn.microsoft.com/en-us/windows/win32/api/tlhelp32/nf-tlhelp32-module32first 。

### 精锐普通首房加载及原生登记条件局部确认

业主完成 v0.3.4 手动首房操作；会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261008_194942_871765_next37` / PID51904 / runTick126375750，DLL b6747908、服务端e6d5e47d、启动器aed177c4及四资源哈希匹配。一次CMD1811、12条原生通知、82条生命周期和3条入场观察；采集缺口0，client.log第8–9行正常退出。原有CMD16→37→42的普通单人入场、首房加载和回城完成；冻结名单[1,3,0]及设置32保持。

首房loader前后和精确caller5B25191同线程110720：145F0C980返回1，但dungeon+6264=0；+8056=128，manager+576=0，场景向量对两名克隆的精确CRef匹配为0。对象ID0/1、主人引用仍匹配当前控制器玩家，控制器65535。原生145DE4360登记未调用；只确认加载、条件值及观测点保留，未确认APC参战、AI、攻击/召唤归属或清理。

实际副本3/地图58548来自dungeon/act1/mirkwood.dgn，SHA256 a1be702502eff219e933eb81df13dbb1c7b18a0ebf7cbcd66a7beda5e37e1e52。源含storymode标签，实际Quest=0/Mode=0是普通入口，不能据标签扩展为剧情验收。PVF、SQLite结构/存档、默认程序不修改。

## v0.3.5 首房原生登记候选：下一次实机

v0.3.4首房证据已闭合加载和退出，当前阻断为原生CMP dungeon+6264,0。权威145B22F50登记块随后检查142E60CF0频道/模式和manager576，再按三个弱引用执行冲突编号处理、145DE4360→145DE4AF0原生登记、位置、AI和技能初始化。这里沿用原生块，不手工重放其内部虚函数或未知参数。

仅替换145B251A9的7字节CMP为CALL近址中继+NOP，返回原始后续JZ；汇编中继保留volatile GPR/XMM、非算术flags、栈对齐及SEH展开，最后CMP有效字节恢复原生分支flags。原生6264非零仍取原值；普通0仅在本次loader线程/频道/主人冻结身份、world当前dungeon、场景有界且稳定、1..3个一致type5克隆及控制器绑定、克隆主人和原生145EFAFB0 primary/fallback玩家CRef完全匹配、无重复编号或已有登记、manager576=0全部满足时取1。当前条件6264/8056不写；controller65535、owner、装备/等级不写。

142E60CF0始终先调用原函数，只在精确caller5B251C3和同次TLS许可/身份/副本匹配时放行。原生总门禁145F0C980不覆盖；其它调用保持原值。安装前现场字节匹配，近址分配和rel32范围验证；暂停并检查其它现存线程后才写7字节，现场占用或失败拒绝安装；错误时恢复保护/释放未用钩子，已安装但线程恢复报错则保留有效中继并拒绝启动，由启动器结束本次进程。原始条件从未改写；重启还原进程内补丁。

本版只走现有候选启动器的Job注入链，安装/卸载第三方mod host未联合实机验证。ordinaryRegistrationReady=true只说明安装完成，ordinaryReady/battleVerified仍false；native AI块可能使APC自行行动，不能称暂停AI。本阶段服务端继续拒绝击杀结算、拾取、切房和通关，不将未知killer/FFFF视为真人。单次入场原有真人疲劳按源计费。

entry日志新增registration-scope-allowed/rejected/native、registration-channel-native、native-registration-result、11项registrationChecks和两套原始真人CRef，仅在当前loader取样，日志仍在DLL目录。检查实际原生玩家引用解决城镇控制器引用与战斗引用是否一致的最后动态缺口；不匹配直接拒绝而不是改owner。loader-after补真实scene以观察登记结果；scene向量匹配不单独等价完整对象表/AI/攻击确认。采集核对同PID/代次/线程、全部放行条件、频道门禁及原生结果，拒绝理由完整也算有效取证。

沿用DFO_ADVENTURE_ELITE=1和`scripts\启动游戏-SQLite.cmd --source-build`，新进程进入原角色，短距离移动恢复已有名单，无需保存；同一基础普通副本首房停留5–10秒，观察同伴是否出现，随后Esc回城并正常退出。玩家不要攻击或走门；APC可能自行攻击。一次操作收齐登记/场景/回城记录，若被拒保留日志而不反复尝试。剧情、奥德赛仍是后续目标，军团/攻坚等专用队伍未支持。

源和重复规则复查：同一gamedata.Source→频道/副本reader→领域校验→原有入场协议/存档；无新地图列表、奖励/等级表或JSON回退。普通Type22与发布ID10的既有兼容策略仍按先前台账暂留，原生etc/channel_info.etc [server]及专用机制属性继续为范围证据，本次没有扩展这项兼容覆盖。登记门禁属于用户明确要求普通副本适配的客户端执行策略，不伪称PVF原生内容。

回退用本机.tmp/adventure-elite/registration-delivery-backup同组DLL、候选服务端、启动器和build清单；原始v0.3.4首房回退目录继续保留。默认程序/PVF/数据库/schema不发布或修改。

## v0.3.6 启动安装冲突修正候选

业主报告v0.3.5无法启动。PID63616 / runTick128937343的status显示资格、资料准备、生命周期、首房观察均成功，仅登记site-mismatch，ModStartInjected返回13；会话20261008_203224_202175_next37未进入游戏。根因是142E60CF0早已由EliteStartPreparationAdapter校验原字节并挂到ElitePreparedPredicate，登记模块再按原字节比较并试图重复MH_CreateHook，误拒绝自己的已安装钩子。旧机制只覆盖分散函数，没有覆盖完整安装链，不能宣称v0.3.5已通过实际启动。

登记阶段改为复用唯一准备predicate。原函数只由既有trampoline调用一次，准备事务先处理原有精确caller，登记callback只消费传入的caller和原始result；其5B251C3/线程/身份/副本/TLS条件保持。登记安装要求准备和首房模块已ready且shared callback空，保留自身CMP现场匹配及暂停线程/保护/回退，不二次比较已接管predicate入口，不新增CF0钩子，不放宽原字节门禁。日志增加entryReady/registrationReady/具体failure。

新增生产串联安装夹具：从EXE只读复制16个代码现场到本测试进程虚拟内存，不启动客户端，不执行客户端原函数；执行实际准备→首房→登记安装，验证共用钩子的字节未被登记修改、原生结果/last-error/SEH转发、缺依赖/错误CMP拒绝和还原。仓库权威EXE及实际F:\wip\dof\115US\DFO.exe均通过。后者整文件SHA为eb3e04a2f07f2eeb69919c3830e9b631c017b3110049ae109efcb9d3d979d552，与权威原文件不同；这里只证明本安装链现场相符，不宣称两份资源整体相同。

v0.3.6候选补足10组C++机制及35种采集夹具；Go门禁仍执行全量并按名比对。候选尚需业主启动及原定首房操作；本次启动失败发生在C2S前，不增加准备3/3、布局1/3或登记1/3尝试，不写新confirmed baseline。SQLite/PVF/磁盘EXE及完整战斗边界不变。回退备份为.tmp/adventure-elite/startup-delivery-backup。

## v0.3.6 本轮实机收口与后续边界

业主反馈“成功了”，加载界面和副本HUD截图显示真人 nene Lv6、APC test Lv1、glow Lv115，副本中两名同伴带 [AI]；随后明确反馈“APC会动，会打怪，没怪会跟随玩家”。本轮确认低等级同伴在该普通首房完成登记、显示及上述可见AI行为；这不是服务端伤害归属、击杀奖励或全流程副本验收。

会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261008_205959_164276_next37` / PID40540 / runTick130600046，DLL v0.3.6 SHA256 5e80ee14、候选服务端5af6d212、启动器aed177c4与构建清单匹配；采集缺口0。客户端启动初始化全部ready，client.log第8–9行exit=0；一次CMD1811、12条原生通知、63条生命周期、7条入场记录，无丢弃/上限。events第226–227行首次CMD16接受，第237–238行CMD37加载确认，第304–305行CMD42回城确认；普通副本3/地图58548，源dungeon/act1/mirkwood.dgn SHA256 a1be702502eff219e933eb81df13dbb1c7b18a0ebf7cbcd66a7beda5e37e1e52，Quest=0/Mode=0。

entry第1–7行同线程27372：原生总门禁返回1，登记作用域十个布尔条件均为1、有效同伴数2，原生玩家CRef与控制器玩家CRef精确相同。频道判定原始返回0，仅本次作用域放行；两次145DE4360原生登记均返回1，场景向量精确CRef匹配从[0,0,-1]变为[1,0,-1]再到[1,1,-1]，loader-after manager576=1。dungeon6264=0/8056=128未写，两个对象ID0/1、owner匹配及controller65535保持。玩家可见AI行为由业主报告证明，不将sceneAttached=0或battleVerified=false解读为AI暂停。

服务端仍限制首次普通单人非剧情入口，拒绝击杀结算、拾取、换房及通关。该会话CMD39记录均被拒；回城后额外CMD39因无活动run拒绝，events第335/338行第二次入口因“不是本连接首次普通选图”拒绝，这是现有阶段边界。ordinaryReady=false和采集battleVerified=false保留，表示完整服务端战斗验收未完成，不否认已确认的客户端AI。生命周期最后样本仍保留两名克隆，不能据正常退出宣布场景解绑/弱引用回收完成。

伤害/击杀/召唤归属、奖励与存档幂等、换房/重复入场/回收、剧情、奥德赛及特殊队伍继续待闭环。原始日志只留本机，版本与证据见analysis/tasks/adventure-elite-registration-confirmed-evidence-20261008.json。本轮仅更新文档/证据；不修改PVF、数据库/schema、控制器、DLL或默认程序，不发布全流程默认版。

下一步先利用本次既有CMD39和权威IDA检查发起对象、攻击/召唤owner到服务端归属及去重执行链；不把65535强制映射真人，不凭HUD猜包，不重复要求名单保存或首房出现测试。换房、重复进图与场景清理需分别确认原生消费/释放路径，再准备能一次收齐所需日志的候选。现有首房限制在证据闭环前保持；剧情与奥德赛目标保留，军团/攻坚特殊队伍独立评估。本轮收口不增加普通准备3/3、布局1/3或登记1/3尝试。

## 下一候选：v0.3.7 战斗来源归属观察（待实机）

静态链：来源对象虚函数 +1312/+1304 → `145DD0070` 写怪物 +29488 来源 CRef 与 +35076 来源 ID → 来源虚函数 +1984 → `145CE2860` 读战斗对象 +74020 控制器 → `145C3E8A0` 写怪物 +26784 → `145DC86D0` 取该值生成 CMD39 killer@4。已有日志 killer=65535，不能据此解除拦截或做全局玩家回退。

只增加上述三函数的只读观察，精确匹配当前精锐弱引用、身份与主人，setter 父事务、调用点 `145DD022B` 和死亡发送逐对象核对原生 CMD39。原生参数/字段/控制器 ID 与服务端结算门禁保持既有行为。预算、丢弃、错进程/启动代次、字段缺失、未正常返回和引用不稳定明确报告。源码离线夹具不表示实机确认。

实机检查点：一次普通单人首房，先让 APC 攻击击杀；若仍有怪则真人攻击一次作可选对照，退出副本并正常结束客户端。不要为真人对照重复实机。仍沿用已保存精锐，日志在 DLL 同目录；不再要求重复保存操作。待动态链闭环后设计最小范围归属适配及服务端结算闭环，再推进多房/释放、剧情和奥德赛；军团/攻坚战独立队伍另行取证。

这轮无新 PVF 玩法表、ID 清单或规则回退；原生的特殊怪物击杀者例外路径 `1451C6830` 仅作取证线索，未复制或覆盖。无 schema/存档修改，无新 C2S 布局假设。分析记录：`analysis/tasks/adventure-elite-combat-ownership-evidence-20261008.json`。

### 精锐原生来源到死亡请求的观察链确认

业主完成 v0.3.7 操作。会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261008_215047_744185_next37` / PID106676 / runTick133649765：12条通知、84条生命周期、43条战斗记录（8条来源赋值、8条击杀者写入、27条死亡发送）；采集缺口0、正常退出。两个精锐对象0/1在精确caller5DD022B将自身控制器65535写入怪物killer；主人ID/控制器2和当前玩家CRef一致。4个怪物4096/4098/4097/4099的27次死亡发送（含重发），逐顺序与服务端CMD39的entity/killer对应。原生self getter14014CC20已由当前IDA和现场字节确认。

本轮只确认这条动态来源链，服务端仍拒绝结算，不能称掉落/经验/通关或多房验收。最后城镇样本保留两名克隆，没有证明弱引用释放。副本3/地图58548来自dungeon/act1/mirkwood.dgn（a1be7025），实际Quest=0/Mode=0；不作剧情确认。没有PVF/schema/存档修改。压缩证据见analysis/tasks/adventure-elite-owned-combat-candidate-20261008.json；原始日志仅留本机。

## v0.3.8 普通单人战斗归属候选：一次完整流程采集

依据v0.3.7动态链，owned-combat attempt 1/3：只在145DD0070来源赋值TLS中、同一怪物、精确setter返回地址145DD022B，当前线程/频道/主人冻结身份、稳定精锐弱引用唯一匹配、controller绑定65535、原生self getter及玩家主人CRef全部成立时，将传给原生145C3E8A0的killer65535改为当前主人。其它调用和未知65535透传；不改APC控制器/owner，不全局推断玩家。适配不依赖日志预算；真实已安装setter夹具覆盖预算耗尽仍生效及未知调用不改。安装新增self getter现场字节门禁。

服务端只为本连接首次已准备的普通单人非剧情run解除39/43/45/46/69~72/117阶段拦截，复用现有死亡确认、拾取、换房、经验、结果/翻牌/离场处理。仍校验冻结主人/频道/名单/设置、有效wire和已加载run；不改变CMD39布局、FFFF原生语义或去重执行，不新增奖励/等级/地图表。特殊2015/2062入口仍拒绝；剧情、奥德赛、军团/攻坚、教学/塔/深渊等后续独立取证。ordinaryReady=false、battleVerified=false保留，表示候选尚待完整实机接受。

扩大只读预算：来源/死亡4096、生命周期1024、入场512，均有明确上限标记；新增原参数/实际参数/改写标记、唯一引用/绑定/self getter、roomSerial，以及服务端请求前后run/地图/死亡/未归属/等级/结果状态、pending地图、实际计划包。采集同时检查原生死亡与CMD39一致、新归属写入与服务端首次有效死亡对应，并汇总拾取/换房/结算/翻牌/退出覆盖。发送前快照不等于成功送达或客户端接受；最终仍由原有发送日志和用户操作确认。

下一次只需一次启动、一次普通副本流程：保留现有名单，短距离移动恢复，选能进入的基础普通单人副本；让APC击杀，顺手拾取已有掉落，按正常路线过门，若可行打Boss、结算/翻牌并回城，随后正常退出。真人攻击对照可选；没有掉落或步骤被阻断就保留现象、直接回城/退出，不为补单项重复跑。原生换房是否携带精锐尚待验证，不能预称兼容。回城后如本来要看精锐页，可自然打开一次以补引用观测；不用重复保存。日志准备完成后再通知用户；不自动启动游戏。

内容链继续为同一gamedata.Source→PVF频道/副本reader→既有领域规则→现有结算/事务/存档；本次无新内容定义。已有普通Type22/发布ID10兼容策略仍暂留于原台账，与etc/channel_info.etc [server]及专用机制属性对照，本次未扩展覆盖；不将它伪称原生规则。数据库/schema、角色等级/装备/技能、PVF/sk.dat及默认程序不修改。

构建/测试产物仅留本机。部署回退为.tmp/adventure-elite/owned-battle-delivery-backup中同组DLL、候选服务端、启动器和清单；先退出旧会话再更换。modkit安装/卸载未联合验证；继续走已授权Go Job注入路线。准备3/3、布局1/3、登记1/3历史计数保持，本次仅新增有当前证据的归属策略1/3。

离线交付门禁：Go1.26 build/vet/test全量退出0，3789通过/279跳过、基线及新增失败均空；12组C++机制、冻结及实际EXE安装/精确setter/异常转发夹具、6环境值冒烟、4097行战斗预算、55种日志采集夹具通过。DLL v0.3.8为223744字节；产物/日志仅本机。无客户端启动，完整副本仍待用户实机。

### 精锐普通单人五房通关流程确认（v0.3.8）

业主明确反馈“已完成通关流程，APC能跟随过门，能拾取，能打boss、能结算”。会话`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261008_223704_525989_next37`，PID93772 / runTick136444046；DLL772bd1e0、候选服务端3449b055、启动器aed177c4及四份源资源哈希匹配，采集缺口0、正常退出。一次准备、12通知、158生命周期、23入场和137战斗记录；54击杀者写入、56来源赋值、27原生死亡。

同一普通单人run经过58548→58549→58551→58552→58553五房，4次CMD45过门及5次CMD37加载全部接受。27个死亡均首次从dead=false变true、unowned=false、killer=主人2；按房间代次/精确来源关联54次setter适配，27个原生CMD39与服务端逐序对应，无死亡重发或阶段拒绝。实际发送27次ACK39/N38/经验N37，保留原有领域去重执行。Boss CMD117确认、CMD46结果/通关经验/奖励、CMD69/70/71翻牌及背包入库、CMD43拾取/场景移除/背包更新、CMD72回城均发送。events锚点：入口224，加载236/291/353/413/473，过门286/348/408/468，Boss549，结果557，翻牌570/573/576，拾取583，离场589；原始日志仅本机，摘要含日志哈希。

确认范围为该普通单人五房精锐参战流程，以及用户可见APC跨门跟随、打Boss、拾取和结算可用。拾取日志是当前玩家CMD43，不单独宣称APC自动捡物；经验更新已下发，不推断重登后数值验收。后续房间entry场景向量匹配值为0，但用户观察和逐房APC来源/击杀证明实际携带；不以该单一字段否定行为或宣布对象表全部闭环。最后城镇弱引用仍保留2个，清理/释放未证实。

本连接第二次入场仍保留限制；其它普通副本、召唤归属、剧情、奥德赛、军团/攻坚等特殊队伍，以及modkit安装/卸载继续未验收。ordinaryReady=false/battleVerified=false是已有候选的全局完整性标记，本次不改原始采集或扩大为所有模式完成。本轮仅文档/证据收口，不再变更运行路径、不增加attempt；默认服务端不发布，DFO_ADVENTURE_ELITE环境开关按业主要求保留，PVF/SQLite/schema保持。

证据：analysis/tasks/adventure-elite-ordinary-combat-confirmed-20261008.json。当前无需重复保存名单或重复该通关实机；后续先分析现有多房/回城证据，再为重复入场、剧情及奥德赛准备同轮可收齐的诊断。

## 普通回城后重复入场候选（reentry attempt 1/3）

本轮只取消服务端 `adventureEliteEntryProbeUsed` 的首次入场拒绝条件，保留其诊断含义。回城后重新打开普通选图才允许再次进入，原账号/角色/频道/名单/技能设置保持冻结；活动副本、待加载城镇剧情、非普通/非单人/剧情请求和源定义教学/塔/深渊/奥德赛仍拒绝。每次成功创建领域会话后增加内存入场序号，失败不计次，重新载入角色归零。每场继续通过同一Source→副本reader→原有Select/入场计划→现有死亡、掉落、疲劳、结算与SQLite事务执行；不新增包、不改变PVF或存档schema。

保持已验收v0.3.8 DLL及其精确归属判断。现有 `145B22F50` loader钩子每房递增roomSerial，TLS登记许可在前后边界复位；已有日志记录manager576、登记条件、精锐弱引用、主人与场景匹配。IDA确认原生 `145B25393→142E64210(manager,1)` 标记登记，`145B38A7D→142E64210(manager,0)` 的原生清理受频道/地图6264/active条件保护；普通地图6264=0时不证明会进入该清理。因此本轮不调用/复制该清理、不写576或6264、不释放CRef、不重载同伴。第二次能否依靠原生对象延续正常战斗是实机缺口。

服务器日志新增run_id、entry_serial以及 `adventure_elite_dispatch_committed`：仅在原有计划包全部发送且dispatch状态切换完成后写入，记录是否仍有活动副本、待城镇场景、死亡确认表、掉落、翻牌及结算标记。CMD16首次也采样；回城按退出请求原run关联。采集分别汇总每场死亡、精锐归属、过门、拾取、结果、翻牌和回城，检查独立RunID/递增序号/干净入场状态，精锐击杀继续按source CRef和DLL roomSerial配对，不能用上一场同entity代替第二场证据。登记频道/结果样本只接受最近同线程loader结束之前的数据，防止后场补齐前场缺口。

已有普通Type22/发布ID10兼容覆盖仍是暂留运维策略，与 `etc/channel_info.etc [server]` 的源目录和专用机制属性对照；本轮不扩展这处重复定义，其收敛条件继续沿原台账。副本地图、资格、收益和数量均未新建Go/JSON平行表。DLL日志预算保持1024/512/4096，达到上限明确报缺口，不能宣称完整。

下一次尽量一次启动完成三次入场：同一普通副本通关/翻牌回城；再次进入同一普通副本，观察APC并击杀首房怪，然后中途退出；第三次再次进普通副本，观察APC能攻击跟随并再退出，最后正常结束客户端。第三次可选择另一可进入普通副本，以补地图覆盖；不要切剧情/奥德赛或特殊频道，不用重新保存名单。某步异常即可保留现象并结束，后续缺项由日志说明，不为补单项重复跑。此批一次同时覆盖通关返回、重复entity、主动退出后的再进和对象延续。

候选以 sidecar `serverCandidateStage=ordinary-reentry` 和服务端事件stage区分，DLL版本仍0.3.8；三件构建产物hash各自核对。默认wireprobe-pvf.exe保持，不发布为全模式完成。回退使用 `.tmp/adventure-elite/reentry-delivery-backup` 里原同组候选服务端、DLL、启动器、清单和源码；仅在用户退出旧会话后切换。原通关confirmed记录不改写为重复入场成功。证据：`analysis/tasks/adventure-elite-ordinary-reentry-candidate-20261008.json`。

## 第二次只剩血条：定位与 v0.3.9 修复候选（reentry attempt 2/3）

业主报告“第二次进入副本只有血条，APC不见了”。失败会话20261008_231554_017721_next37（PID32220 / runTick138774906）首次普通run完成27死亡（其中13次有APC归属）、4换房、拾取/结果/翻牌/CMD72回城；第二次CMD16已成功创建独立RunID和干净状态，但零战斗，最终CMD42回城。采集缺口0和正常退出仅表示日志完整，不表示第二次成功。

entry第24–27行第二次loader来自146D2A1E7，仍有2个稳定精锐对象ID0/1、主人CRef及绑定不变；登记checks=[1,1,1,1,1,1,1,1,1,0,2]，唯一失败项是manager576=1，没有第二次原生登记。权威IDA确认142E60CE0只读该byte，145B251D3调用它（返回地址145B251D8），非零就跳过登记。原生setter145B25393置1，普通地图6264=0不能证明会走145B38A7D清理置0。四次已确认过门走146CE14C1且也保持active1，不能全局清零或每房强制登记。

v0.3.9增加精确新副本loader caller146D2A1E7与全部原生8参数范围。九项稳定主人/原生玩家CRef/唯一精锐type5/绑定/新场景未登记校验全部成立后，允许active1的重复入场范围；仅当前登记块getter消费者145B251D8、同一管理器和副本、冻结身份复核时将原生返回1局部视为0。仍沿145DE4360原生登记、位置/AI回调和原生setter收尾；不写manager576/6264、不释放/重建CRef、不修改APC控制器/主人。其它消费者、房间过门、未知active值和身份漂移完整透传。入口与active getter新增原字节门禁，安装失败拒绝启动。

客户端日志新增loaderCallerRva/freshDungeonEntry和registration-active-reentry阶段（原始active1，物理manager576仍1）；采集在同线程同一次loader内核对频道→active消费→逐同伴成功登记→loader结束新场景成员，并按顺序与服务端入场次数关联。日志预算耗尽不影响适配，但采集报告缺口，不能用上一场样本补第二场。服务端候选仍为已部署ordinary-reentry attempt1/3的5ca6128a，启动器aed177c4不变；本次重入客户端修复attempt2/3，构建清单分开标识，未改服务器包/玩法/schema。

下一次一次启动、保留名单和同角色频道：首次进原普通副本，允许APC击杀一个怪即可主动回城；第二次进同一副本观察身体/攻击，过一道门观察跟随，然后回城；第三次快速进图观察APC并击杀后退出，最后正常关闭客户端。无需每次打完整Boss、翻牌或重新保存名单。任何异常就记录所处次数并结束，后续日志自动区分首次、第二次和第三次。当前为离线候选，原五房confirmed baseline保持，不声明第二次已修好。回退同组备份.tmp/adventure-elite/reentry-registration-delivery-backup；更早备份保留。

现有Type22/发布ID10兼容policy仍暂留原台账，来源对照etc/channel_info.etc [server]；本次只改客户端登记消费，无新地图/等级/奖励Go或JSON定义。证据analysis/tasks/adventure-elite-reentry-registration-candidate-20261008.json。剧情、奥德赛与特殊队伍继续待独立闭环。

### 普通剧情入口候选（story attempt 1/3，待一次实机）

现有普通战斗入口拒绝非零Quest。当前PVF允许用既有DungeonSelection、任务接取状态和`dungeon.Select`解析剧情迷宫；本候选移除精锐入口处额外的`Quest != 0`拦截，不放宽玩家等级/已接任务、PVF maze/scene/战斗/奖励流程、difficulty/fatigue或原生boss门条件。非单人、奥德赛、教学/训练、塔/深渊和特殊频道继续走现有拒绝路径。没有新增Quest或地图手工表。

目标取自实机原请求`ID5/Quest3146/Mode0/Party65535`。当前Script.inner.pvf经原生索引确认`list/dungeon.lst → 5 → dungeon/act1/sunderland.dgn`，minimum required level为5；对应maze0的quest3146、start76131、boss坐标(3,1)。源任务`grandflores_03.qst`保留前置4873并连接后续3147，服务端仍只认可玩家真实任务状态。当前PVF source test证明任务未接取、低于源最低等级及不存在的quest maze均被原reader拒绝。

为一次实机，服务端新增入口请求/Quest/模式/party、source maze/boss/剧情层/NonCombat计数，及入场、过场、房间、精锐登记、归属击杀、通关和结算关联日志；采集器覆盖剧情拒绝场景。DLL沿用已验收v0.3.9；环境开关仍是DFO_ADVENTURE_ELITE=1。候选仅证明该一个source剧情入口的请求被允许，不证明APC正确参与过场、不会攻击NonCombat演员或所有剧情门可通过。详细步骤与止损点见analysis/tasks/adventure-elite-story-candidate-20261009.json。

### 普通回城后多次入场确认（v0.3.9，reentry attempt2/3）

业主明确确认“确认通过，多次进本APC均出现”。会话20261008_234621_294907_next37 / PID100828，DLL94165690、候选服务端5ca6128a、启动器aed177c4及四份资源hash匹配。三次独立RunID与入场序号1/2/3，原生同线程loader每场两名精锐登记均返回1，结束新场景CRef成员均为1；第二/三场active原始及物理manager576仍1，精确145B251D8消费者各适配一次，首场不适配。三场分别有4/7/9次APC归属死亡，第二/三场各过一道门；三次主动退出均提交清空run死亡/掉落/翻牌/结果状态，无阶段拒绝。

确认该普通单人非剧情副本在同连接主动回城后可重复登记与战斗，不扩大为直达下一副本、HP/MP/死亡/冷却连续性或所有地图/模式。采集时客户端仍在运行，run.json和正常退出汇总未落盘；采集明确保留该1项缺口，fullCaptureCertified=false，不为补收尾重复跑图，不宣称正常退出或完整归档。原capture不改写。证据analysis/tasks/adventure-elite-reentry-confirmed-20261008.json。

本轮文档收口，不改运行路径、不增加attempt、不改PVF/SQLite/schema/default。DFO_ADVENTURE_ELITE按业主要求保留。下一项按计划处理普通结算再次挑战/非回城直达新副本；剧情、奥德赛、特殊队伍和对象释放继续待闭环。

### 再次挑战与结算重新选图确认

业主补充确认“再次挑战和选择另一个副本也是正常的”。同一会话现已正常退出，run.json已落盘。CMD72实际向量010001...（state1/option0），旧run9ad51701在1229行创建原有入场计划，1242行提交全新a1644789，清空死亡/掉落/翻牌/结算；对应第五次新副本loader ordinal56，两名APC登记成功，随后5次精锐归属新死亡。CMD72向量010101...（state1/option1）在1829行清空旧run并保留选图状态，1831行CMD16接受新runf55462a5，两名APC正常登记。

只读采集器原来仅数CMD16，漏掉无需CMD16的再次挑战；现在凭原请求、ACK16入场计划、成功提交与干净新RunID统计，保持实际服务端序号4，不伪造新序号。六次CMD16加一次CMD72共七次新副本加载，全部登记成功；正常退出、采集缺口0。78种离线夹具覆盖缺请求/脏状态/同RunID/缺计划/重复/缺loader，不改游戏运行路径。fresh build退出0；同658f3ffa Go源码的完整vet/test退出0结论保持。

本次日志中接受的副本ID均为3，因此确认结算重新选图入口及用户可见行为，不据此扩为所有不同地图。CMD2062和无缝option5没有本轮样本，继续待证。原三次回城确认的采集时缺口作为历史快照保留，本次完整收尾补齐。证据analysis/tasks/adventure-elite-retry-confirmed-20261009.json。下一项普通剧情战斗层，奥德赛随后单独处理；PVF、SQLite、DLL及服务器运行产物不改，默认程序不发布。

### 普通剧情首次入场 APC 出现实机确认（ID5 / Quest3146）

业主确认“成功，剧情副本也有APC出现”。会话20261009_004730_456710_next37 / PID69388，源链为同一内层PVF→list/dungeon.lst→dungeon/act1/sunderland.dgn（f2ad25fd）→既有DungeonSelection/已接任务校验→maze0 Quest3146→普通副本状态机与既有SQLite事务。角色2、频道22、等级8，源最低等级5；入口map76131、NonCombat计数1、source_story_layers=0。两名精锐原生登记均result1，首房场景成员和主人匹配，用户可见APC出现已确认；不把这次无额外maze layer的流程称为全部剧情层验收。

本会话日志还记录五次房间加载（76131→76132→76134→76135→76136）、4次过门、2次拾取、23次怪物经验更新、结果页/翻牌ACK、Quest3146完成与物品/经验事务、后继Quest3147保存发送及结算回城。23组普通怪物CMD39与原生death-send按对象/击杀者精确配对；其中17组同时具备精锐source CRef、同线程原生来源父调用/精确setter、controller绑定和当前主人证据，其余6组是真人来源。日志流程观察与用户“APC出现”的直接确认分别记录，不据此宣称全部剧情Boss均由APC击杀。

完整死亡采集仍有一项缺口：26次原生death函数调用与25个服务端CMD39不等；起始对象13099有CMD39但未经过当前hook记录，终场对象4123有3次函数调用但仅1个CMD39。函数正常返回不等于每次发送一个包；保持原采集器缺口与全局coverage=false，不凭65535猜主人、不过滤未知剧情调用以凑齐计数。后续房间loader-after即时采样sceneVectorMatches为0也不冒充最终挂场确认，后续精锐攻击来源只按本轮精确样本确认。NonCombat演员不被攻击、其它剧情/多层、APC状态连续性及原生引用释放仍未闭环；无需为本次出现确认重复实机。

本轮实际部署服务端为共享工作树29b48b06基线加工作区改动构建的1c9f5842（29,571,584B），与隔离候选8f2e187a区分；策略源码提交08613763、DLL v0.3.9 / 94165690、启动器aed177c4。采集候选/PID匹配、正常退出；摘要中的allFreshEntriesRegistered=false由整体战斗缺口传播，单次freshCycle的两名登记本身均成功。沿用上轮完整build/vet/test、真实PVF source tests与78项采集夹具通过结论；本轮仅文档/证据收口，不改Go/DLL运行路径、不增加attempt、不改PVF/schema/玩家存档或默认程序。DFO_ADVENTURE_ELITE按业主要求保留。证据analysis/tasks/adventure-elite-story-confirmed-20261009.json。

下一项奥德赛仍须先闭合模式、频道与原生登记消费路径；CMD2062/无缝option5、军团/攻坚等特殊队伍另行取证。已有Type22/发布ID10兼容策略与etc/channel_info.etc [server]重复仍保留原台账，本次没有扩展该策略或新增内容清单。

### 奥德赛普通单人入口候选（odyssey attempt 1/3，待实机）

移除精锐接入层额外的奥德赛角色/副本拒绝。奥德赛源副本仍要求本会话与既有character.OdysseyRole一致；普通源副本不改变原角色范围。仍只接受普通单人Mode0、冻结名单/主人/频道和源普通频道，教学/训练、塔/深渊、军团/攻坚与专用传送拒绝不变。DFO_ADVENTURE_ELITE=1按业主要求保留，DLL沿用已确认v0.3.9，不新增hook、不改PVF或SQLite结构。

当前内层PVF由`contents/2026/aradodyssey/etc/aradodysseyjournal.cos`的原生节点发现50个副本，再经`list/dungeon.lst`和各DGN的`[dungeon mode script] arad odyssey`、`[designate dungeon difficulty]`、玩家最低等级、maze/门/通关条件进入既有dungeon.Select与原模式奖励/日志存档。50个源副本的候选准入与原Select在源最低玩家等级、源指定难度下均通过；没有新增运行地图清单或平行等级/难度/奖励表。小型输入改变源等级/难度后原Select对应接受/拒绝，陌生角色、冻结身份漂移、专用频道/模式仍拒绝。

权威IDB中145B22F50的精锐登记块仍是源byte6264→142E60CF0→142E60CE0→原CRef/145DE4360→位置/AI回调。已验收DLL只在既有精确caller、稳定主人/原生玩家引用、唯一绑定精锐和新副本场景范围适配；N1754中仅原生mode0视为2。142E5EDA0的原生mode3来自feature662与独立状态分支，本轮未命名为奥德赛，也不覆盖它。奥德赛实际native mode/channel/loader、精锐登记与AI仍须实机验证，离线源测试不作实机确认。

服务端候选0.3.11补齐角色模式、原生创建标记、发布频道、请求难度、源奥德赛标记/指定难度、Boss和门条件的诊断；采集分别汇总奥德赛入口、原生登记/主人/战斗、结算重入及NOTI2856日志更新。78个既有夹具与新增奥德赛阶段/版本/日志/无伪确认夹具通过。隔离与共享源码全量build/vet/test均退出0，逐名失败集合无新增；共享源码构建单独标识，保护其它工作区改动。默认程序、DLL、启动器及资源保持；候选测试不是默认发布。

下一次只需一次游戏会话：已有真正奥德赛角色保留名单，进入符合源进度的奥德赛普通单人副本，观察APC出现/攻击，拾取并过门，正常通关/结算回城查看进度，再重进一次确认APC与攻击后正常退出。首次异常即结束这次会话，不反复保存/试进。候选与回滚清单见`analysis/tasks/adventure-elite-odyssey-candidate-20261009.json`；原普通/剧情confirmed baseline不扩大。

现有Type22/发布ID10兼容策略与`etc/channel_info.etc [server]`重复继续记入原台账，本轮未扩展；直进2062/无缝option5、特殊队伍、对象释放与完整剧情死亡采集仍独立待证。

### 奥德赛名单包含当前角色：加载前失败与投影修复（attempt2/3，待实机）

用户报告奥德赛没有APC及队伍信息。会话20261009_011918_377182_next37 / PID14344：N1754在频道22/主人1成功消费，native mode[0,0]均局部适配2并发出CMD1811 mode2；服务端随即以“精锐角色归属、槽位或选择已变化”拒绝，无1382/1879或克隆，更没有进入精锐登记。只读SQLite确认出战test ID1，账号保存mode2=[1,3,0]，另一同伴glow ID3属于同账号；当前自己导致整份加载提前失败，不是奥德赛模式判断或原生登记故障。六项APC生命周期/战斗采样缺失按失败原样保留，正常退出不冒充成功。

服务端0.3.12仅在已授权普通频道扩展的mode2临时视图，将当前角色原槽位置0；1754投影、1382加载和冻结Selected使用同一有效视图。test出战时临时[0,3,0]，只加载glow；账号仍保存[1,3,0]，切回其它角色仍可加载test。其它槽位/技能/模式、disabled/native路线不改，外账号/重复/未学技能校验不放宽。DLL继续已确认0.3.9，没有新增包或改layout，也无需用户重新保存名单。

新增真实SQLite回归用角色创建option10=2判断奥德赛、明确不依赖DFO_ODYSSEY_MODE覆盖，验证换角色、空自己槽、单名APC原生资料和冻结hash、技能及持久名单不变；其它三个槽位置、全自选空列表、原生/off/特殊频道隔离均覆盖。已合并实际远端gud/main 76115fcd后重跑完整build/vet/test；共享源码也全量验证，失败集合无新增。79项采集夹具通过，默认/DLL/PVF/存档/schema保持。Type22/发布ID10与etc/channel_info.etc [server]重复策略仍在原台账，本次不扩展。

一次实机继续使用相同奥德赛test角色和现有名单：首进应只出现glow一名APC及队伍信息，出现后在同一会话观察攻击、过门、正常通关结算及重进一次，最后退出。首次无APC就结束该会话，保留日志，不重复保存/试进。实际奥德赛战斗仍未确认，原普通/剧情baseline不扩大。证据analysis/tasks/adventure-elite-odyssey-self-roster-candidate-20261009.json；回滚.tmp/adventure-elite/ordinary-odyssey-self-delivery-backup。

### 奥德赛单名 APC 出现确认；快速进入失败定位

业主确认奥德赛APC出现，但明确报告Boss后快速进入失败。会话20261009_013718_578732_next37 / PID64800 / runTick147257500，实际服务端0.3.12 b1fa9d74、DLL0.3.9 94165690：出战test ID1，冻结临时名单[0,3,0]，glow原生单名登记result1，用户可见出现确认。日志还有8组完整来源的精锐归属新死亡，以及过门、Boss/结果/翻牌、等级升至15与回城观察；这不等于全部奥德赛或完整死亡采集通过。原生死亡调用与CMD39数量不一致的一项缺口原样保留。

events619行CMD2062实际目标100004935/难度2，620/621行被精锐候选“尚未接入专用传送/直进副本”门禁拒绝；没有创建新run/发送下一副本进图计划，故转场停在原地。确认范围仅首场APC出现；快速进入保持失败记录。证据analysis/tasks/adventure-elite-odyssey-quick-next-candidate-20261009.json。

### 奥德赛清关后快速进入候选（奥德赛attempt3/3；2062入口attempt1/3）

服务端0.3.13允许当前已加载、通关并发出结算的奥德赛普通单人run，经冻结主人/频道/名单设置一致性与PVF奥德赛目标检查后进入既有CMD2062路径；在特殊阶段处理之前校验，2015及特殊副本继续拒绝。DecodeDungeonDirectMove→疲劳/已接任务→dungeon.Select→原15/27/16/28/29计划→成功dispatch的新run/清空死亡掉落翻牌结算职责沿用。只在目标Select与计划成功后递增精锐入場序号，不改包布局、不补2062应答、不重发精锐名单或克隆资料，不改DLL。

当前同一内层PVF及journal.cos确认请求目标为01_grakqarak/grakqarak.dgn ddeafda7，源最低玩家等级15、指定难度2、首图100016503；真实PVF离线原入口测试通过，角色模式来自创建option10=2且不依赖全局覆盖。权威IDB146D29700在146D2A1E2以0/1/0/0/1参数调用145B22F50，return146D2A1E7为现有fresh-entry适配位置；实际2062下一场登记仍须用户实机，静态证据不冒充成功。源定义/reader/原执行器保持，没有新增平行玩法规则。

完整build/vet/test在已合并实际远端后的隔离与共享源码均通过，失败集合无新增；85项采集夹具覆盖2062跨代次、目标错配、脏新run、缺请求/计划、重复提交，原有死亡缺口保留。CH/confirmed baseline只收口0.3.12首场APC出现，0.3.13快速进入仍待证。共享dungeon_flow与隔离原基线不同，仅追加直进两个精确hunk，保留其它改动；默认、PVF、DLL、SQLite/schema不改。Type22/发布ID10与etc/channel_info.etc [server]重复兼容策略仍在台账，本次没有扩展。

下一次用相同角色和现有名单在一次会话清关→快速进入→观察新地图/APC攻击/过门；正常再清关快速进入一次，最后回城退出。首个转场失败就停止，不重复保存或连续试进。完整证据analysis/tasks/adventure-elite-odyssey-quick-next-candidate-20261009.json；回滚.tmp/adventure-elite/ordinary-odyssey-quick-next-delivery-backup。三次上限按根规则继续执行，若本次失败先只读取证，不盲目第四次改路径。

### 奥德赛快速进入下一副本及 APC 出现确认（单次2062转场）

业主确认“能够快速进入下一个副本，APC也出现”。会话20261009_015823_668706_next37 / PID85832 / runTick148524671，实际共享服务端0.3.13 / 85a3ff4b（29,683,200B），DLL0.3.9 / 94165690、启动器aed177c4，源归档4d8c0c82及冻结投影名单[0,3,0]一致。首场源100004935格拉卡→CMD2062→目标100004936雷鸣废墟，请求/源目标/原15/27/16/28/29计划及成功提交匹配，RunID14c7cbbc→1663e92e，入场序号1→2，死亡/掉落/翻牌/结果/通关初始状态已清空。现有fresh loader第二次managerActive=1，经已确认的局部适配后单名glow原生登记result1，用户可见下一场APC出现确认。

首场与目标场各有4次过门、结果/翻牌，目标场6次拾取并结算回城，正常退出。日志观察不替代用户确认范围：本轮只有一次2062快速转场，目标场没有完整来源的APC归属新死亡样本，不能据此宣称目标场APC击杀或连续多次快速转场全部通过。服务端记录70个CMD39，原生死亡函数正常返回记录97次，调用/发包计数仍有1项缺口；保留overall coverage与allFreshEntriesRegistered=false，两个独立freshCycle的登记均成功，不过滤未知调用凑计数。

CHANGELOG及confirmed baseline按本次单次快速进入/APC出现收口；原0.3.12拒绝会话与0.3.13候选证据保留。六个任务源码/测试/采集脚本hash均未改变，无新远端合并；本轮fresh build两套源码退出0，沿用0.3.13完整vet/test（隔离3824通过/282跳过；共享3676通过/207跳过，失败及新增失败均空）、真实PVF原目标准入及85项采集夹具通过结论。本轮仅文档/证据收口，不改Go/DLL/PVF/schema/玩家存档或默认程序，不增加attempt，不要求重复实机。mode3、option5、特殊队伍、APC释放/状态连续性及完整死亡采集仍独立待证。

证据analysis/tasks/adventure-elite-odyssey-quick-next-confirmed-20261009.json。Type22/发布ID10与etc/channel_info.etc [server]重复策略仍在原台账，本次不扩展。

### 新角色精锐加载拒绝：宠物重复实例 key 修正候选（0.3.14）

新角色4的20261009_025432_546975_next37会话events158/159已收到CMD1811模式2，但全队编码因队友glow/角色3穿戴宠物500991107的实例字段22拒绝，未发送N1382/N1879，冻结准备为nil；events214随后普通剧情3/Quest3145入场。这不是新的等级门槛。只读SQLite确认账号名单[1,3,2]及宠物槽26记录：+6与+24均为key1，22..55唯独+24非零。

当前PVF list/equipment.lst→equipment/creature/500991107.equ [equipment type] [creature]/[minimum level]1，脚本SHA256f2abad28de698d28ba9d17403f55cececc865b57d5c1efdf2b5a6f0af02f8f08，归档4d8c0c82；玩法条件不变。权威1452C1540在1452C1682读key→1452C1EB7保存原实例+6，14576D8B0/14576D9EA消费+6，原紧凑装备格式不读重复+24。0.3.14仅允许槽26/32的+24为零或等于+6，其余未支持数据、不同key及普通装备仍拒绝。紧凑包布局和key6保留，原名单、宠物、成长数据不删除、不重写；不改PVF/DLL/schema/默认程序。宠物成长/饱食度/名字的独立N2189路径尚未加入，本候选不声称完整宠物功能。

实际四角色穿戴只读编码通过，glow保留25件/4807B；槽26/32、多字节key、所有未支持区间、不同key/普通装备边界及原1382/1879加载不改存档回归通过。隔离与共享全量build/vet/test通过，隔离3853通过/282跳过、共享3910通过/269跳过，基线及新增失败为空。候选30656000B/SHA256 8eae14a07d2f280bf846e81e25575cc338774e4a6b38c730bb25959b31afd19d。已合并实际gud/main e4cc3ba1并保留双方历史；该远端catalogs.go引用的两份buffer-rental实现仍是另一任务未跟踪源码，隔离仅复制本地构建依赖、绝不提交它们，不能声称纯提交树独立完整。

采集器识别0.3.14同一原生schema并新增离线版本夹具；旧DLL/启动器继续使用。失败采集发现共享候选服务端已变为17bce99b，与旧清单85a3ff4b不符，保留原失败身份，不把旧基线重新绑定到新程序。部署前备份当前候选及清单，部署后只读预检核对全部实际哈希。完整证据analysis/tasks/adventure-elite-new-role-pet-key-candidate-20261009.json；源码回退.tmp/adventure-elite/new-role-source-backup，候选回退.tmp/adventure-elite/new-role-delivery-backup。

待一次手动会话：用现有新角色bash及现有名单直接进入此前副本；先确认队伍栏/APC，正常则过门、清关、回城并再进一次，退出后统一采集。首次缺队伍/APC就停止，不重复创建/保存/尝试。候选不写CHANGELOG或confirmed baseline为新验收。

Type22/频道ID10与etc/channel_info.etc [server]的重复兼容策略保留原台账。此次取证还发现inventory/creature_list.go的CreatureDefaultNames/creatureExperienceThresholds仍手工对应宠物脚本及creature/exptable.tbl；该路径未参与本次key投影、不在本次扩大修改范围，源迁移留作独立事项，不能静默称已收敛。

### 三名精锐的加载完成登记候选（DLL0.3.10 / 服务端0.3.14）

业主再次报告新角色没有APC/队伍。032359_422014_next37会话已证实0.3.14实际运行，CMD1811成功冻结角色4名单[1,3,2]并发送原N1382/1879；N1382原生reader完整消费9920B，N1879成功返回并得到3个strong1引用，无异常/丢样。宠物字段拒绝已消失，但当前角色不在名单中，实际填满三名精锐。旧DLL要求N1879必须命中1444FAC6B谓词（calls8），本次calls0因此未arm，生命周期/入场/战斗日志全缺，不能据此说登记成功。

权威142E5B060在142E5B2C1查询已有角色，142E5B2CC有值进入复用分支；只有无对象分支142E5B5E2调用1444FABF0→1444FAC66谓词。三名全占用没有空位也能原生完成，分支调用次数不是完成必需条件。DLL0.3.10仅将lifecycle arm改为原reader成功返回且匹配活动事务；1754→1382（谓词7）→mode2done的线程/主人/频道/顺序/期限检查、实际入场时当前玩家/唯一type5/主人CRef/绑定/新场景/管理器门禁全部保留。没有新增包、hook、角色指针写入、内容规则或存档变更，服务端0.3.14不重建。新角色兼容attempt2/3，前次宠物投影1/3记录保留。

新增本进程原回调夹具：旧代码exit5复现，修正后零/八谓词均arm；缺资料、过期、错模式均拒绝，五次回调只执行五次。MSVC /W4 /WX实际DLL构建及13组机制/原EXE字节安装夹具通过，225792B/SHA256 a46493c61f54fef1ec52852cd8e02383722a10d53587db343a96fc09b6f89099；采集兼容0.3.10既有schema，87项日志夹具通过。本次不改Go，沿用未改源码的上一轮全量build/vet/test（隔离3853/282，共享3910/269，失败/新增失败为空）。原Type22/频道ID10与PVF [server]、宠物名称/经验的重复策略仍在既有台账，未在本次收敛或扩展。

证据analysis/tasks/adventure-elite-full-roster-candidate-20261009.json。只部署DLL和同步清单，备份见.tmp/adventure-elite/full-roster-delivery-backup；保留原验收对象的旧hash绑定。下一次单一手动会话：用现有bash直接进此前副本，正常则回城再进一次；第一次仍无APC就停止，不重建角色、不重保存名单。统一收集原生加载、lifecycle、入场登记结果/场景成员及战斗日志。实机待确认，CHANGELOG/confirmed baseline不写成新成功。

### 三名精锐首次及再次入场确认；缩减名单仍有独立故障

业主确认“带3名修复了，但是只带1名无法进入副本”。034552_507908_next37 / PID109356、DLL0.3.10 a46493c6与服务端0.3.14 8eae14a0匹配。角色bash(4)名单[1,3,2]首次进入普通剧情副本3/任务3145，回城后进入副本5/任务3146，两场均获得三个原生登记result1及新场景成员[1,1,1]；服务端两次进入加载完成。此处仅收口三名首次/再次出现，与用户报告一致。首场Boss结算作日志观察，完整死亡采集仍有缺口，不扩展为全部模式或全部战斗行为验收。

本会话缩减为一名的CMD1719成功返回N1754，但保留旧冻结名单[1,3,2]；随后CMD1811以“已经准备”拒绝，CMD16以“准备身份或源频道不一致”拒绝（events1286~1307）。清空后选两名能重新准备，再次缩减一名重复故障（events1539~1552）。这是尚待修复的保存/准备生命周期问题，不撤销三名登记修复的限定确认。证据analysis/tasks/adventure-elite-three-roster-confirmed-20261009.json。原死亡覆盖、特殊队伍、宠物成长及Type22/ID10重复策略台账保留。

本次仅文档收口，运行源码与两项本地构建依赖hash未变，沿用已取得的全量build/vet/test：隔离3853通过/282跳过，共享3910通过/269跳过，无失败/新增失败；13组DLL机制、87项采集夹具通过。PVF、schema、存档与默认程序未变。后续名单重置候选单独记录，不将一名改为已确认。

### 保存人数变化后的准备状态候选（服务端0.3.15 / DLL保持0.3.10）

034552_507908_next37同会话两次复现：从三/两名改为一名，CMD1719保存成功但冻结名单不变；N1754释放旧APC后发原CMD1811，被“已经准备”拒绝；随后CMD16被设置hash不一致拒绝。空名单能清旧状态，此后两名的原生加载及登记成功。故障不在一名资料宽度或新角色等级，不改包和DLL。三名首次/再次入场的限定确认已单独收口。

权威142E5A4C0 /142E5ABFC比较旧新三个槽位：变化经142E5AE64释放旧角色并在142E5AED6请求1811；不变且原生角色仍有效时保留引用，只经142E653A0应用技能，无1811。服务端仅在成功保存、产生原1754载荷后按有效mode2身份同步会话：人数/槽位身份变化或清空则清旧准备，让原1811重建；相同名单保留对象并更新设置hash。进本/选图/未完成回城或特殊转场期间在事务前拒绝修改。原重复加载门禁、包顺序、主人/频道/身份及入场验证保留。新角色兼容attempt3/3；若仍失败只取证，不增加盲包第4次。协议定义及PVF内容规则没有变化。

真实SQLite回归旧代码10个子例失败，修正后13子例通过，覆盖3→1、2→1中间槽、1→2、2→3、顺序变化、清空、重复保存、技能变化以及运行态/无效技能拒绝且存档和冻结状态不变。两工作树全量go build ./...、go vet ./...及go test ./... -count=1真实通过：隔离3869通过/282跳过，共享3926通过/269跳过；基线与新失败集合均空。采集脚本支持0.3.15，88项夹具通过。另一任务两项buffer-rental只读构建依赖仍未提交；当前源中的Type22/ID10及宠物名称/经验重复规则沿用既有台账，不在本轮扩展。

候选服务端30657536B/SHA256 12b064f76120a828e5c75fd7d883a5af158e78eb19da176b8fd7d4be1c61bbd9；DLL0.3.10 a46493c6保持原文件。证据analysis/tasks/adventure-elite-roster-reload-candidate-20261009.json。只替换隔离wireprobe-handoff-source.exe并更新ignored manifest，默认程序、PVF、schema和玩家存档不改。保留所有旧验收的原hash绑定。本次一名切换仍为候选，不写入confirmed baseline。

下一次只启动一次scripts/启动游戏-SQLite.cmd --source-build：现有角色按1→2→3→1，每次城镇保存后进同一已解锁副本，检查队伍和APC后回城，无需Boss/新角色；首次失败立即停止。正常退出后一次统一采集保存→1754→1811→1382→1879、生命周期、原生登记result/场景引用与入场/加载/回城日志。相同名单原生对象被外部销毁或同连接角色索引改变仍缺新的实机样本，不扩大支持范围。
## 精锐名单人数变更后重新加载确认（服务端0.3.15）

**已修复 / 已完成**：业主确认“确认已修复”。服务端候选0.3.15 / 12b064f7；在城镇保存后，有效精锐名单变化时清除旧准备状态，让客户端原N1754→1811→1382/1879路径重新加载；名单相同时保留已加载角色并同步技能设置身份。DLL仍为0.3.10 / a46493c6。精锐原生登记DLL实现源码已提交于87b6760546c23fa4eb04ddfcb0db35affea00e35，包括src/native-trace.h、src/native-completion-test.cpp、src/adventure-elite.cpp及build-mod.py。

**未修复 / 未闭环**：确认仅覆盖名单变化后重新加载/入场；攻坚/军团特殊队伍、完整死亡包采集、宠物成长等仍单独待证。工作区没有本次确认后的新实机会话日志，按业主明确验收记录，不填写未观察的进本次数或原生事件数据。

**bug 测试取证**：analysis/tasks/adventure-elite-roster-reload-candidate-20261009.json。旧实现回归10例失败，修正后13子例通过；0.3.15候选两工作树完整build/vet/test与88采集夹具通过。手动确认对应源码提交52b778b94bc69083630294b04d499efacf962c2c。未修改PVF、schema、玩家存档或默认发布程序。


### 投影名单隐藏后的原生重新加载（0.3.16，待实机）

业主报告奥德赛“暗精灵的英雄”没有APC/队伍栏。045100_675129_next37 / PID65932日志确认注入0.3.10成功，服务端0.3.15已准备[2,3,4]；城镇传送specialWarpPending期间轮询发空N1754，恢复后发同一名单，原生reader释放三名对象并发CMD1811，服务端却保留旧冻结准备，以“已经准备”拒绝。两次目标入场均weakAlive=0。此故障属于投影视图与准备身份不同步，不是副本等级或剧情内容门槛。

只读当前PVF确认contents/2026/aradodyssey/dungeon/05_darkelf/darkelf.dgn，ID100004939/首图100016019/最低玩家38级/指定难度2，脚本ec5d4a37，归档4d8c0c82。源脚本→journal.cos引用→现有Source.Dungeons/Select→原入场/SQLite执行器保持。权威IDB142E5A4C0在142E5A6DD清设置map；槽位恢复经142E5ABFC比较、142E5AE64释放、142E5AED6请求1811；对应原生日志sequence5的weakAlive3→0、requestObserved=1。

服务端0.3.16将保存后的准备身份同步复用于实际发生变化的N1754轮询刷新：空或不同有效名单清旧准备，让原请求重新走1382/1879；相同槽位保留对象并同步设置hash。转场/特殊频道/重复加载门禁保持，不改DLL、PVF、协议布局、schema、存档名单及默认服务端。这是投影恢复生命周期attempt1/3，已有奥德赛准入3/3不继续盲目试包。

旧代码1/2/3名回归均失败，修正后各两轮传送、恢复资料、拒绝重复请求、存档/序号保持及特殊范围测试通过。全量go build ./...、go vet ./...、go test ./... -count=1退出0；3940通过/269跳过，逐名基线/新增失败均空。真实PVF journal全引用准入（含本副本）及采集夹具通过。隔离候选72c81168（30555648B）已部署并通过只读预检；DLL仍a46493c6、默认服务端仍a6f26960。失败日志已原样另存；原失败现场缺build清单，不将当前清单回绑历史会话为完整认证。证据见analysis/tasks/adventure-elite-projection-reload-candidate-20261009.json；回退.tmp/adventure-elite/projection-reload-delivery-backup。

下一次仅一场手动会话：原角色/原名单，经相同传送入口进入暗精灵的英雄，检查队伍栏/APC；正常则回城再次传送入场检查，可过一道门，最后退出。首次失败就停止，不重新保存名单/创建角色/反复试进。一次收齐城镇传送与N1754隐藏恢复、原1811/1382/1879、原生引用/登记/场景及入场序号。候选实机待确认，不更新CHANGELOG或confirmed baseline为成功。

现有Type22/ID10与PVF [server]、宠物名称/经验重复策略沿用既有台账，未在本次扩展或收敛；特殊队伍、全部奥德赛图、对象释放及完整死亡覆盖仍独立待证。

资源身份补记：实际launcher.local.json启动目录为F:/wip/dof/115US，其DFO.exe为eb3e04a2、外层PVF为e18cd1c5，与仓库client/参考副本的1d394878/5dd03873不同；sk.dat同为59d78371。当前候选记录两套身份，不将参考副本预检误称实机资源一致。既有DLL逐点原生字节门禁及PID65932动态日志属于实际启动目录；本轮未修改两套资源，差异来源未闭环且不据此改变协议或PVF。后续会话统一保留配置目录身份和原始注入/登记日志。


### 城镇传送保持名单稳定（0.3.17，待实机）

业主反馈0.3.16仍失败，并补充“前面有，后续传送或重进后消失”。051649_922170_next37 / PID21152日志确认0.3.16实际运行，传送后原1811重新加载已成功；前两次入场各三名原生登记result1/新场景成员[1,1,1]，第二房存在APC来源攻击记录。之后events491空N1754→CMD15先进入选图→499恢复N1754→501原1811被selectingDungeon拒绝；native sequence9释放至weakAlive0，第三次loader拒绝。原候选只解决重复准备拒绝，未解决投影恢复与选图的时序竞争，保持失败记录。

0.3.17仅拆分名单可见性与加载/战斗资格：普通源城镇传送不再从账号视图移除mode2，故两种轮询/CMD15顺序都不产生破坏性重载；原ordinaryEliteSelectionVisible和PreparationAllowed仍拒绝specialWarpPending及未完成场景，军团/攻坚/guide/tutorial/mine等实际范围仍拒绝。0.3.16的空/变化视图失效处理保留用于实际源范围变化。未改DLL、包布局、PVF、SQLite/schema或默认程序。生命周期attempt2/3，原奥德赛准入3/3不继续猜包。

旧0.3.16六个顺序/人数子例均失败，新版各两轮传送回城、准备身份/技能/存档保持及入场门禁回归通过。完整build/vet/test退出0：3943通过/269跳过，逐名基线及新增失败均空；真实PVF journal准入含本图，采集既有及0.3.17夹具通过。隔离候选1ee81eaf（30555136B）已部署；DLL仍a46493c6，默认服务端仍a6f26960。0.3.16失败会话完整归档，原采集两项缺口保留，不以局部成功冒充整轮通过。源码回退.tmp/adventure-elite/stable-warp-source-backup，运行回退.tmp/adventure-elite/stable-warp-delivery-backup；证据analysis/tasks/adventure-elite-stable-warp-candidate-20261009.json。

下一次一场手动会话：原角色/名单，以DFO_ADVENTURE_ELITE=1和--source-build启动；经相同普通传送到暗精灵的英雄，正常速度直接选图进本，检查APC/队伍，正常则回城再传送进入检查，可过一道门，最后退出。无须为了绕过时序而等待或重存名单，首次异常停止。日志准备收齐原生名单/资料、传送/选图时间序列及每场登记/来源，实机待确认，不把候选写进CHANGELOG或confirmed baseline。

Type22/发布ID10与PVF [server]、宠物名称/经验重复策略仍沿用既有台账，未在本轮扩展；真实运行客户端F:/wip/dof/115US与仓库参考副本的资源身份分别记录，未修改任何一套资源。全部奥德赛/特殊队伍、引用释放及完整死亡覆盖仍独立待证。

## 奥德赛转场后精锐名单与 APC 保持确认（服务端0.3.17）

**已修复 / 已完成**：业主确认“确认修复，可以提交”。将普通精锐账号名单可见性与加载/入场资格分开，城镇传送等待不再发送临时空名单，避免客户端释放 APC 后与 CMD15 选图抢先发生的 CMD1811 加载拒绝；实际名单变化仍清除旧准备，相同名单保留角色并同步设置身份。候选0.3.17 / 1ee81eaf，DLL仍0.3.10 / a46493c6。本次一并补跟踪远端遗漏的精锐 Go 源码、测试和采集/取证文件；DLL源码已在37774428入库。

**未修复 / 未闭环**：确认范围为业主报告的转场/重进消失问题。本次日志观察两场（100004939→CMD2062→100004940），不扩大为全部奥德赛、军团/攻坚机制或完整死亡采集通过。会话缺run.json，采集时未观察正常退出；原生死亡发送与服务端CMD39计数差异保留，汇总overall combat及allFreshEntriesRegistered=false没有改成通过。

**bug 测试取证**：053059_378049_next37 / PID100452 / runTick161280828，原CMD1811仅一次，两次入场冻结[2,3,4]一致；events271/1131准入成功，entry5~7/58~60各三个原生登记result1，fresh loader8/61场景成员均[1,1,1]。原0.3.16失败和旧回归六子例失败保留；0.3.17完整go build ./...、go vet ./...、go test ./... -count=1通过（3943通过/269跳过，无失败及新增失败），真实PVF50条奥德赛journal来源与采集夹具通过。详见analysis/tasks/adventure-elite-stable-warp-confirmed-20261009.json；没有修改PVF、SQLite/schema/玩家存档或发布默认程序，二进制与运行日志不入库。
