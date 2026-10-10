# Boost 662 治疗教学 APC 准备缺口

状态：attempt 1/3、2/3实机未通过；attempt 3/3修正1754特殊APC索引后，业主确认训练APC出现且第二关通过。本功能已收口，确认身份及范围见[confirmed baseline](boostup662-teaching-apc-confirmed-baseline-20261010.md)。没有修改客户端、DLL、权威IDB、数据库结构或活动存档。下文“尚未验收/下一次实机”为当时候选记录，不代表当前状态。

## 已确认的故障证据

会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_013320_802755_next37/`：

- `events.jsonl:149`：角色3在教学城镇发送 CMD2333，明文 `01000000000000000000000000000000`，当时被标记 `unimplemented_sample`。
- 同会话进入地下城100004546、地图100015495。过场实体APC2504被销毁并收到NOTI38，随后技能32使用请求正常到达；客户端没有再发送CMD45。用户观察：APC只出现在过场里，实际地图没有治疗目标。
- `briefingdrone_buff_01.act`的目标是 `[PARTY TARGET] [INDEX] 1 [APC CHECK]`。脚本先扣除目标20%HP，随后检查HP恢复到90%以上，再销毁训练假人。技能使用通知本身不是进度条件。

## 当前源链

同一运行归档 `server/work/client-build/Script.inner.pvf`，SHA256 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`：

1. `live/event/kor/2026/0326_boostup/boostup.evt` 定义活动、职业与训练副本路线。
2. `etc/mycharactersapc/mycharacters_special_apc.etc` 的 `[character]` 行：`[index] 1`、`[apc index] 2504`、`[usable contents] boost up event`、`[type] event 662`。其余占位行是 `none -1` 或空条目，不是本活动教学绑定。
3. 原生 `list/aicharacter.lst` 将2504关联到 `AICharacter/mycharacters_apc_custom/2025_BoostUp_forBuffer/swordman/customMyApcSwordman.aic`。
4. 副本 `.../100004546_boostup_dragonsoldier_buffer.dgn` 含 `[init apc system dungeon]`；地图 `.../100015495_boostup_dragonsoldier.map` 的2504带 `[cinematic]`，用于演出，不能代替队伍APC。
5. 原生被动对象109131695引用 `live/event/kor/2025/0109_boostup/passiveobject/briefingdrone/briefingdrone_buff.obj`，其ACT为 `briefingdrone_buff_01.act`，负责扣血、治疗判断和消灭假人。

## IDA MCP 闭环

取证最初采用权威 `client/DFO.exe.i64` 会话。宽范围文本扫描耗时过长后关闭会话且不保存，后续在其相同内容的私有副本 `.tmp/boost-skill-ida/DFO-copy.i64` 上继续 MCP；无数据库写入。

| 地址 | 确认行为 |
| --- | --- |
| 14074D860 / 140C13B00 | Boost城镇初始化发现模式3选择为空时发送CMD2333，正文仅u8=1。 |
| 140C133DC | CMD2333应答注册到nullsub_1；单纯补ACK不能创建治疗目标。 |
| 142E5EDA0 / 142E60CF0 | 原生活动分支选择mode3并允许APC系统。 |
| 142E62D70 | 从上述special_apc.etc读取NPC定义并加载其AIC。 |
| 142E5A4C0 | NOTI1754读取u8行数，每行531字节；模式u16在+0x0D。 |
| 142E5C590 | NPC标志+0x10..12、特殊APC的PVF `[index]` u32+0x17..1F；比较源节点+52，再映射到本机虚拟选角索引+0x27..32。此前误写为AIC模板，已由动态内存订正。1754后自动发送CMD1811(u16mode)。 |
| 1444FCF90 | NOTI1382在活动启用时选择类型2容器。+FD158明确写r8b=1，再调用145F01580创建容器；读取u8玩家数、u16所属wireID、u8额外角色数。额外数0直接跳过角色快照循环。 |
| 142E5B060 | NOTI1879读取u16mode。+B236明确xor r8d,r8d，以只查找方式取得类型2容器；因此需要先1382。 |
| 1444FABF0 / 1459628F0 | 1879找不到虚拟槽位资料时从已加载PVF AIC创建NPC；不需要伪造账号角色快照。 |
| 142E650D0 / 142E5CD00 | 1879将NPC绑定到APC槽位；后三组技能字段从531字节结构+0x33读取。 |
| 14745250E / 145B22F50 | DGN标签设置+0x1878；进图时读取该标志，将已准备的APC加入场景并启用APC管理器+576。 |
| 141EB5C52..141EB5CA7 / 142E5FFA0 | ACT的APC CHECK开启且管理器已启用时，PARTY TARGET index1解析为APC槽0。 |

## 候选实现与边界

`Source.BoostUp -> boostup.Load -> ParseTeachingAPCs` 读取同一PVF的活动NPC绑定。CMD2333在已激活、未毕业、教学城镇、普通频道且无战斗/转场时生成mode3 NPC选择，线上NPC字段来自 `[index]`，模板字段仅保留源中AIC资源身份；NOTI1754复用既有eliteProfileView，保留其他可见模式的账号角色及技能设置，继续隐藏教学城镇不允许的mode2名单。CMD1811(mode3)须已下发选择，返回空额外角色的NOTI1382，然后NOTI1879(mode3)。客户端自行加载AIC和执行教学ACT。

APC模板号、特殊索引和事件绑定没有新增Go内容表或JSON回退。既有账号精锐保存解码器仍拒绝外部NPC，不把活动临时选择写入账号存档。CMD38与过场死亡处理保持既有行为。数据库/schema无变化，已有SQLite角色和物品存档继续使用。

重复规则检查：既有精锐实现只支持账号角色、mode2，拒绝外部APC模板；此前没有服务端教学APC内容表。本次扩展其原生编码契约，复用角色列表投影，不另建NPC装备、技能或血量表。训练目标血量及过关条件仍由客户端当前PVF的ACT执行。

## 验证与下一次实机

### attempt 3/3：特殊索引与AIC模板混淆

attempt2会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_032157_333342_next37/`，业主仍报告实际地图没有APC：

- `events.jsonl:142..144`：03:30:39收到2333，1754的NPC字段仍为2504；`:160..165` 自动1811后返回1382 `01030000`、1879 `0300`。
- 后续没有覆盖名单的空1754；`:204`进入100004546，`:259`进入100015495。因此attempt2解决了刷新覆盖，但没有闭环NPC创建。
- 该会话启动命令确认实际客户端目录为 `F:/wip/dof/115US`。该目录Script.pvf为771,470,242字节，SHA256 `e18cd1c5a91e158138880e29d0c78f425b833f39a2e6f138b4e5c4d000d7a961`，与服务端内层归档哈希不同。不能把仓库client/外层资源身份当作实际运行资源身份；本轮以实际客户端内存确认相关绑定，未修改任何资源。

经业主明确授权，使用x64dbg MCP对运行中的PID2220只读取内存；DFO基址140000000，无暂停、断点、写内存或补丁操作：

| 内存位置 | 实际值及含义 |
| --- | --- |
| `[14E638EF8]` | APC管理器16A06A080；+0x40起三个对象槽全部为0，+0x240启用标志为0。 |
| 8AE1F120 | 模式3的531字节记录：+0x10=1，+0x17=2504；+0x27、+0x2B、+0x2F全为FFFFFFFF。1754已被原生reader消费，但NPC没有映射成功。 |
| 169FC3580 | 当前客户端源节点：+32=2504（map key），+40=1（event类型），+44=2504（AIC），+48=662（事件），+52=1（特殊索引）。 |
| 8BA09AE0 | 已生成的虚拟映射节点：+28=4，+32=2504。源NPC已可用，实际虚拟选角索引为4。 |
| 169FC3CC8 | 源节点+1864的AIC对象指针16A085610非空；不存在AIC缺失。 |
| 151090160 / 8AB0CB20 | 当前玩家的类型2资料容器存在，记录列表为空；1382创建容器成功，1879传入未解析的FFFFFFFF无法创建NPC。 |

IDA链订正：`1473E0FA0` 的 `[index]`（加密串1491A8940）读入v110，`[apc index]`（14A9329D8）读入v111低位，`[type]`（1491EDFB0）写v111高位并将事件号读入v112。`142E62D70` 在62F1A..62F2C分别保存AIC、事件、特殊索引及类型，在6342E..634D6以AIC为内部map key；`142E5C6B6`匹配的却是节点+52特殊索引。找到以后，5C71A取AIC map key，5C743匹配虚拟映射，5C7AA写入槽位。因此线上应发送PVF特殊索引1，而不是AIC2504；随后客户端可映射为虚拟索引4。此前静态分析把两种索引混为一谈，第一次候选的该字段结论无效。

实现将协议字段命名由APCTemplates订正为APCIndices，选择绑定读取 `TeachingAPC.Index`，不改包布局、时序、技能或过关条件。源中已有 `[index]` 继续直接驱动服务端，不新增常量或JSON表。修改前回归日志 `runtime/boost-apc-attempt3-regression-before.txt` 明确报两项失败：刷新字段期待1而实际2504；改变源索引为7、AIC为9997时线上错误发9997。修复后两项通过，证明线上值随源特殊索引变化。

可回滚文件保存在 `runtime/boost-apc-attempt3-rollback/`：attempt2候选程序及本轮五个源码文件的.go.bak快照。此次仅使用最后一次实现假设；若仍失败，停止继续调整运行路径，保留当前缺口并从客户端原生命中继续取证。

Go1.26.0专项回归通过（wireprobe4.230秒、protocol0.866秒、boostup0.882秒），完整 `go build ./...`、`go vet ./...`、`go test ./... -count=1` 均退出0；wireprobe全量测试71.018秒。与attempt2逐名比较失败集合均为空，`git diff --check`通过。日志为 `runtime/boost-apc-attempt3-{targeted,build,vet,test}.txt`，只读内存取证为 `runtime/boost-apc-attempt3-memory.json`。

候选 `go build -trimpath -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe` 退出0；31,027,712字节，SHA256 `6b93bfdc34c8ffdbeee6a91c7ec82595c8068018d064e5f8382cde3840118325`，Go1.26.0，模块入口dfolan/cmd/wireprobe。默认程序仍为10月7日的29,897,216字节版本，未覆盖。此次修改 `cmd/wireprobe/boostup_apc.go`、`boostup_apc_test.go`、`internal/game/protocol/adventure.go`、`boostup_apc_test.go`、`internal/boostup/teaching_apc.go` 及本记录；未暂存或提交，仍需业主手动重新启动候选验收实际APC及治疗推进。

### attempt 2/3：冒险团刷新覆盖活动名单

业主再次报告没有APC。新会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_030159_502750_next37/` 证明首次加载路径已命中：

- `events.jsonl:130..132`，北京时间03:09:45：CMD2333后下发NOTI1754，532字节，mode3、NPC标志1、模板2504。
- `events.jsonl:148..153`，03:09:47：客户端自动CMD1811 `0300000000000000`；服务端下发NOTI1382 `01030000`、NOTI1879 `0300`。
- **`events.jsonl:175`，03:09:49：冒险团周期刷新发送NOTI1754 `00`，清空整个选择名单。** 03:09:52进入训练副本，03:10:18使用治疗技能仍不能推进。

IDA MCP复核142E5A4C0：新记录替换整个选择map；当前模式由142E5EDA0取得。选择由非空变为空走142E5AE21..142E5AE64，逐个调用142E63BA0释放APC。因此第一次候选的缺口是遗漏了后续周期刷新，不能把已发出加载包当作APC存活的证据。

复现测试 `TestBoostAPCAccountRefreshKeepsPreparedCompanion` 在修复前明确失败：`town refresh overwrote the loaded event NPC ... Payload:[0]`，日志 `runtime/boost-apc-attempt2-regression-before.txt`。修复把活动行合并接入所有 `worldSession.adventureElitePayload` 投影，2333和周期刷新使用同一个合并函数；教学城镇、选图和战斗期间保持同一模式3记录及签名，未变化时不发1754。其他账号模式发生变化时仍携带活动行，离开教学区域或毕业才移除会话临时行，不写账号存档。

attempt 2/3只改 `cmd/wireprobe/boostup_apc.go`、`adventure_elite.go`、`boostup_apc_test.go` 和本记录。无新协议布局、无技能ACK、无额外推进包。专项Boost/精锐回归通过（4.249秒）；Go1.26.0执行 `go build ./...`、`go vet ./...`、`go test ./... -count=1` 均退出0。与attempt1逐名比较失败集合均为0；wireprobe全量测试72.525秒。日志为 `runtime/boost-apc-attempt2-{build,vet,test,targeted}.txt`，`git diff --check`通过。

attempt2产物：`go build -trimpath -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe` 退出0，31,026,688字节，SHA256 `b611c3499a080ba06752889b5afc6525c6c2ed2b164989f52ab437591bc7e17c`。`go version -m`确认Go1.26.0及模块入口`dfolan/cmd/wireprobe`。默认程序未覆盖，未提交；需关闭旧会话后重新使用 `scripts/启动游戏-SQLite.cmd --source-build` 手动验收。

可回滚快照：`runtime/boost-apc-attempt2-rollback/{wireprobe-handoff-source-attempt1.exe,boostup_apc.go.bak,adventure_elite.go.bak}`，保留第一次候选程序及本轮修改前的两个实现文件。回滚源码最初使用.go后缀，导致build扫描到非独立包并失败；已改为.go.bak排除扫描后重跑门禁。下一轮仍检查2333/1811链，但重点确认1382/1879后不再出现覆盖模式3的空1754，再由业主观察实际APC与治疗推进。

### attempt1历史验证与资源边界

专项协议/领域测试通过；真实归档 `TestBoostUpTeachingAPCSource` 通过（9.80秒），确认Load读到特殊索引1、模板2504、事件662。修改前普通全量 `go test ./... -count=1` 无失败；attempt1普通全量同样无失败。以下为第一次候选的历史构建信息，当前候选以attempt2段为准。

最终门禁使用Go1.26.0：`go build ./...`、`go vet ./...`、`go test ./... -count=1`均退出0，逐名失败集合比较before=0、after=0。`git diff --check`退出0。

候选构建：`go build -trimpath -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe`退出0，产物31,022,592字节；`go version -m`确认Go1.26.0、模块入口`dfolan/cmd/wireprobe`。旧候选可回退文件 `runtime/boost-apc-rollback/wireprobe-handoff-source-before.exe`（31,008,256字节）。默认`bin/wireprobe-pvf.exe`没有覆盖。

候选SHA256：`b3e1d2f5890aa972e2656afd4d8342ffe261d9ee071c440954938a8068d5fbba`。

额外启用真实归档的 `TestBoostUpFullSourceClosure` 有既存失败：奖励根590015880没有 `[booster]`。用HEAD的原始模块源码在 `.tmp/boost-apc-before/` 独立复跑，同一个测试及同一个错误再次失败；与本次APC解析无关。两次日志分别是 `runtime/boost-apc-source-test.txt`、`runtime/boost-apc-source-test-before.txt`。该奖励定义问题不在本次修改范围。

本次文件范围：`cmd/wireprobe/{boostup_apc.go,boostup_apc_test.go,boostup_flow.go,adventure_elite.go,request_scope.go,world_flow.go}`；`internal/boostup/{catalog.go,teaching_apc.go,teaching_apc_test.go}`；`internal/game/protocol/{adventure.go,boostup_apc.go,boostup_apc_test.go}`；`internal/gamedata/boostup_source_test.go`；本协议记录。其他工作区修改不在本次范围，没有暂存/提交。

### 下一次实机

需要业主关闭旧游戏会话后，手动通过 `scripts/启动游戏-SQLite.cmd --source-build` 进入现有角色的“模拟训练（二）”。检查教学目标APC是否出现在实际地图、扣血后用治疗恢复、是否开放下一段。

日志关注：CMD2333不再未实现；NOTI1754带mode3 NPC特殊索引1；自动CMD1811正文mode3；NOTI1382四字节空额外角色头；NOTI1879 mode3。实机应确认客户端将槽位解析为当前虚拟索引并实际创建APC对象，然后检查技能治疗、假人死亡及CMD45。若失败，停止新增运行路径假设，继续收集原生命中；不补技能ACK或盲加推进包。候选不覆盖默认程序，不写confirmed baseline，尚未提交。
