# 巴卡尔地图恢复后：房间、立即死亡、回跳与时钟取证

用户11:16会话`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261006_111654_459138_next37`，对照官服`captures/20261005-015111/frames.jsonl`。本轮只读取证，未修改运行代码、客户端或玩家存档。

## 已闭合的遗漏

1. 房间位置未刷新：8次2062有N2285，而11次普通CMD45全部只有ACK45/N29，没有N2285。普通`moveDungeonRoomDecoded`仍只返回通用两包；仅原生2070路径调用BakalOpening.StageWarp。必须通过同源SlotForMap确定新房间位置，所有合法换房同步攻坚队位置，不能沿用入图location。
2. 时钟缺乏同步：当前11:18:51仅一次N584 `000f27000000000000`（9999）。官服01:55:40发9999，01:55:50发9989，01:56:00发9979，随后约10秒一次递减。当前Tick不发送新的剩余时间，只检查总时限；不能把服务端有失败计时器视为客户端计时已正确显示。
3. 脚本移动门禁不完整：11:19:57和11:21:07的CMD2070被`target is not adjacent`拒绝。当前非巴卡尔二阶段分支套用普通Move邻接限制；原生脚本指定房间的移动不一定是普通相邻门。需要对照同源COS/MAP与成功2070，保留归属和源目标门禁，不能简单取消邻接或直接相信客户端坐标。

## 进图立即死亡：已定位差异，因果待确认

首次100003160加载11:19:14.771完成，entity4096第一条CMD39在11:19:14.776到达，其余7条在.791–.830到达。后续地图也有大量加载完成后数十毫秒内的死亡，109次CMD39已有109次ACK，本轮不再是前一版经验拒绝的尸体问题。

对照同一地图100007498（0x05F5FE4A），官服01:55:54的N29与当前11:19:22的怪物类型、等级140、rank0、来源索引及阵营分组一致，但每行Level/Rank/CreateTrigger/Hidden之后的第五byte：官服00，当前FF。当前`protocol.StartMap`写死255。不能把team100解释成HP。

当前客户端1452B7100从该行读有符号byte，在1452B7BE9进入EDI，经1452B7C20参数传入145B0D910，保存到记录+0x24。因此它并非已证明无消费者的padding。还需追踪+0x24到怪物对象和死亡条件，才能认定FF是立即死亡原因，不能只换值试包。现存脚本/客户端装备或角色效果是否在加载阶段杀怪，也需结合CMD38战斗时序；本轮未操作客户端取动态命中。

## Boss回跳：先排除错误归因

本会话没有CMD2074、没有`bakal_retreat_to_camp`，总时限也未到；Boss回跳不能依据当前日志归为服务端撤退或600秒踢回。11:20:17进入100003151，11:20:23客户端再次2062进入100003153；11:21:13再次151，11:21:19再次153。实际发生了新的客户端直移请求，而不是服务端单独回城通知。

当前2062丢弃TargetGrid/SpawnWindow并从源起始地图重新Select，可能对原生脚本跨图移动采用了错误的落点策略；目标字段语义不能只凭名称当可信房间坐标。2070非邻接拒绝与2062重选起始地图需要联合核对，仍需源房间、客户端sender/reader和成功实录绑定。

## 其他已知边界

- 37和2073均触发完成加载，最新每次2062会产生两次bakal_loaded；应核对重复加载是否重复触发地图动作/刷新，AddRaidBoss的模板去重不代表整个加载序列幂等。
- N2286的Buff关联尚无完整来源选择和授予链，定时移动怪波次仍未执行。图标可见不代表这些内容已完成。
- 本会话没有战斗超时撤退，先前N584只发一次的缺陷和这次回跳不能合并为一个假设。

取证输出仍在`.tmp/bakal-ui-20261006/`，新增1452B7100.asm与145B0D910.asm为实际客户端N29消费链。下一轮修复顺序：源绑定的CMD45位置更新、原生时钟同步、加载幂等；立即死亡与脚本跨图移动继续闭合字段/目标语义后再改运行路径。

## 用户授权后修复

- 普通CMD45成功换房返回N2285，位置使用同源SlotForMap查询，与N29目标一致；原生2070共用该投影，不重复发送两份位置通知。
- N584每10秒按源PHASE TIME OVER减去真实已过秒数，负值钳为0；10秒属于已观察的协议刷新节奏，未新增玩法时限常量。源持续时间1234的回归验证10/20秒后的1224/1214。
- 同一map的37/2073重复加载仅发对应ACK，不再重复全量角色/背包恢复、动态boss注册或源初始化事件。每次新map的Loaded仍由Session换房重置。
- 取证发现交接DungeonRoomTransition还有此前遗漏的server-only RaidReturn字段（DWARF offset0），MoveRaidReturn位于raid_stage.go:10..19。它要求已载入、已清怪、目标在本run源maze，并选择已访问layer；不是普通邻接Move。现恢复该执行分支，非巴卡尔二阶段的原生2070只允许返回当前源grid，不能凭任意客户端grid跨maze传送。保留源room与未清boss门禁，N29使用原生layer-return标记与请求过场记录。当前实际失败2070样本在未清boss时仍拒绝，清怪后返回ACK2070/N29/N2285通过回归。
- 补回的RaidReturn没有从普通客户端CMD45解码，不向其它副本扩大权限。原先NPC/奥德赛换房路径保留。

立即死亡仍未修复：N29差异字节从reader传入构造record+24，但尚未证明该字段导致当前死亡，不能擅自把FF改00试包。进一步追踪145B219D0中该字段目前只出现记录复制，不能把这点当成直接HP赋值。依根AGENTS §0硬约束“字段、顺序、等待态或客户端消费路径未闭环时，停止服务端叠包试探”，本轮明确保留该问题，需继续原生创建/死亡触发链取证。

真实源专项与新增换房位置、重复加载、清怪门禁/脚本返回、时钟刷新回归通过。全仓go vet通过，全仓测试仅两项既有character失败；最后收紧同grid门禁后重新运行对应专项并构建新文件。

默认与隔离profile更新为`bin/wireprobe-bakal-room-final-candidate.exe`，SHA256 `aeb82307b17f49ceac468ba6e3c5dd6abeb9ec9b9586459311fd1b3f8871b084`。之前程序保留，没有启动客户端或修改玩家存档。原启动游戏.cmd仍启动D:\115us\DFO。

最终入口验证完成：25项Python默认启动/profile测试与实际launch_local.py --check通过；最终修改的4个相关Go包vet再次通过。全量日志room-fix-tests.txt、room-fix-vet.txt和交接room-warp/raid-return反汇编保存在本轮.tmp目录。

## 11:54新会话：非零列portal被拒与双箭头

会话`20261006_115442_173998_next37`：普通45成功移动至100007498/499/500，位置同步已经发送；11:57:50开始的2062请求100003163(1,1)、100003160(3,0)等反复记录`unsupported legion direct-move fields`。`DecodeLegionPortal115`把TargetGrid[0]限定0，是普通军团证据边界被错误复用到巴卡尔。官服指定抓包也有(1,1)/(3,0)及更多真实二维格点。

修复新增DecodeBakalPortal115，复用同一个46B/padding/ID/difficulty解析器，但两轴接受byte范围；普通DecodeLegionPortal115保留原column0限制。网关将请求grid传入已有bakalDungeonEntry，再由同源DGN maze/MAP和COS slot确认目标，拒绝不存在的格点。此前忽略grid总是从DGN初始房开始的遗漏同时修正，不直接信任SpawnWindow作玩家落点。已用本次实际两个失败请求回归验证目标房与N29格点一致，并验证255不存在的源房仍拒绝。

用户澄清双图标是“两套黄色箭头重叠”。本次日志每次实际map加载只有一次N30、一次bakal_loaded，2073第二次仅ACK，不能归因于重复37/2073初始化。原生PathGate.img引用已定位：144A3A6C0、144A37F20与142024F30；相关资源载入会释放旧引用，但两个导航渲染分支何时同时启用尚未闭合。官服N27/N28确有多处当前简化帧未提供的字段，不能直接抄固定模板或删N27/N30来“去重”。因此双黄色箭头本轮未修复，依根AGENTS“禁止猜包”继续取证；当前没有变更未知导航模式字段。

候选`bin/wireprobe-bakal-portal-grid-candidate.exe` SHA256 `e0ef977063db5f4ba9e41ce02586b423785120cd8cef589ce1067ffa126f7923`，默认/隔离profile切换，原程序保留。专项与全仓vet通过；全仓test仍仅两项既有character失败。进图立即死亡与双箭头仍未完成，不能把本次portal格点修复宣称为全部副本机制完成。

## 12:12会话：清怪后出口丢失与撤退无效

会话`20261006_121257_326177_next37`，12:15:28进入100003157 source location23，12:15:37发送N2286 type0/location23/state1/HP0，已经投影怪物清除。12:15:41客户端发送2070，目标grid(0,0)、原生过场记录`01000000ffff6f012c01ffff640064000000`，服务端报`Bakal return grid does not match owned source room`，没有成功ACK2070与目标N29。上一轮额外收紧same-grid限制是错误：source maze内的合法返回目标可能不同于当前grid，不能以同格假设替代交接MoveRaidReturn的源目标门禁。本轮用户只请求定位具体包，未再次改运行程序。

点击退回的实机请求是CMD72，明文`01020100000000000000000000000000`，12:16:13、12:16:17、12:17:22、12:17:50共4次，全部被普通settlementExit报告`cards before owned settlement`拒绝。没有CMD2074。当前dispatchBakal仅接2074返回，不接72的本次原生撤退UI路径，因此72落入通用翻牌结算分发。这是返回功能路由缺口，不是网络没有收到按钮操作。

指定官服抓包同形CMD72的原生应答成功前缀为`01 01 02`（其余填充不可当固定字段），说明该请求的state/option应保留；但该官服时段的其它玩家/副本数据不能直接当巴卡尔回营整个包序列。后续修复应按当前已确认的owned loaded Bakal状态识别72撤退，仅复用巴卡尔camp restore，避免绕过普通副本结算门禁或对未拥有的run放行。

出口恢复需要正确处理2070源maze返回及其目标层图/room record，不应仅重发572开放表或再发一组黄色箭头；死亡确认已收到不能证明客户端脚本返回已完成。

## 用户指定20261004 output抓包检索

检查`D:\115us\analysis-tools\output`的official_20261004-211348_live、215907_live、225825_live、234626_live，共7组双向session文本（14文件）。按真正id=字段而非frame编号筛选：全部没有C2S656/2089/2069/2070/2073，也没有S2C2286；不能把文本中的frame2285/2069等序号或反方向同号通知当成巴卡尔证据。

- 211348/session_s6有19次C2S2062，地下城明文96f4f505=100005014，属于其它副本，不是100003149..165巴卡尔。
- 225825/session_s4有3次C2S2062，52f0f505/53f0f505=100003922/923，为维纳斯阶段地图。C2S2290/2285也属于该军团交互，方向与巴卡尔通知含义不同。
- 215907及234626的其余session没有巴卡尔特征链。
- 原始official_20261004-193513.pcapng没有对应_live解密输出，额外用tshark只读检查：两个游戏连接10021/10020各约1.5秒，只见CHANNELINFO1、LOGIN_PRECHECK1554、LOGIN1及183等登录应答，未进入角色战斗会话，不构成巴卡尔进图/返回抓包。

结论：20261004这些现有输出未找到巴卡尔战斗日志，可继续使用用户指定的20261005-015111以及交接200137成功会话作为巴卡尔双向证据。本轮仅检索，不据其它副本同号数据修改巴卡尔协议。

## 清怪后源返回与实际72撤退修复候选

用户授权继续修复后：

- 删除网关额外的same-grid限制，保留已恢复MoveRaidReturn的RaidManaged/Loaded/RoomCleared与source maze目标存在校验，不能由任意grid构造源外房间。实际100003157 boss grid(0,1)清怪后请求(0,0)回归通过，N29目标grid、N2285源位置及ACK2070完整；未清怪仍拒绝。此前same-grid门禁确属我的过度收紧，已撤销。
- dispatchBakal接CMD72 state1/option2（实际16B010201…），在拥有已载入的同一RaidManaged会话中复用`bakalRetreatPlan`，发送回城状态/N23/N24、N2285camp与ACK72 `{01,01,02}`，不走普通cards/settlementExit。其它option和无巴卡尔所有者继续原有分发；错误body、无已载入战斗或不同run不能借撤退逃过门禁。CMD2074与72共享相同领域返回，不制造2074请求。
- 回归断言撤退不结束整场攻坚、不发未经请求的ACK42、成功包含town与camp恢复、重复无战斗撤退被拒、普通settlement72不被raid处理吞掉。

新候选`bin/wireprobe-bakal-exit-candidate.exe` SHA256 `863c67700bfd02fee6d75e3b47e5948125f8946a2b0d6d4246d1a92ca789368a`。默认/隔离profile切换，原程序保留。专项与全仓vet通过，全量结果和实际入口测试待最后确认；本轮没有自动启动客户端、改客户端文件或玩家存档。双黄色箭头与进图立即死亡仍为尚未闭合的独立问题，本候选不宣称修复这两项。

最终确认：全仓test仅两项已记载character失败；默认profile一度仍指向portal-grid版导致配置测试失败，单独重新接线后读回确认exit版，25项Python配置测试与launch_local.py --check全部通过，check之后再次读回仍为exit版。没有将配置失败误报为通过，未发现后续再次覆盖。输出exit-fix-tests.txt/exit-fix-vet.txt保存在本轮.tmp目录。

## 12:42会话：区域首领击杀后重复生成

会话`20261006_124225_536108_next37`：12:45:29.420 N2194在grid(0,1)生成basilisk模板109014480、entity4096；12:45:37.712 N2286 location23/type0/state1/HP0确认区域怪物清除；12:45:41.155 CMD2070返回源grid(0,0)，随后12:45:41.435又发N2194同模板、entity4099。不是死亡请求重复或客户端残影，是服务端在另一个房间再次生成。

旧逻辑仅主龙DGN记录cleared，非主龙区域首领仅发移除通知而没有保存本次raid的区域击杀状态；AddRaidBoss的模板去重只覆盖当前房间Monsters，换房后列表不同，无法阻止重复。

修复BakalOpening增加本次会话defeatedLocations：确认源实体死亡且RoomCleared后DefeatMonster记录location，重复确认不再发第二次移除/奖励投影；bakalLoadedBoss在加载当前source slot前检查location击杀状态，已击杀区域不再发2194。未改变源InitMonsters、地图目录、模板或玩家库schema，也没有把所有非主龙DGN封闭成不可通行地图。

巴卡尔第一阶段死亡同样记录当前location，只有源二阶段条件通过后BeginSecondPhase清除该位置的第一阶段抑制，让第二模板合法生成。新建BakalOpening重新初始化击杀状态，不把上一场状态带入新raid；未来源脚本明确再创建怪物时应由该源事件改变位置状态，现有未实现波次不靠自动房间加载复活。

新增真实PVF回归复现100003157 grid(0,1)→原生CMD39→2070回(0,0)→加载、同raid再portal进入、新raid再生成，断言前两条路径无2194而新raid有2194；巴卡尔二阶段回归通过。候选`bin/wireprobe-bakal-boss-once-candidate.exe` SHA256 `19ee6a4a73fe0e6137f0013b825527477c74acfd86e99ad8932c9ae27e95a414`，旧程序保留。本次击杀状态是源slot的执行状态，不是新增玩法常量表。

最终验证：全仓vet通过，全仓test仍仅两项既有character失败；25项启动/profile测试及launch_local.py --check通过，check后读回默认profile仍指向boss-once候选，客户端D:\115us\DFO与活动PostgreSQL路线保留。输出boss-repeat-tests.txt/boss-repeat-vet.txt在本轮.tmp目录。未替用户重启游戏，实机重复Boss修复待用户手动验收。

## 15:13 整合交付对照与 14:36 实机会话（只借鉴）

用户明确：`D:\115us\115US-整合导入-改动交付-20261005` 自身也有重大问题，只作参考，不整体导入。只读比较该目录 `server-work-dfo-lan-changes`，未覆盖当前源码、客户端、PVF 或玩家库。

确认其中包含 `raid_bakal_flow.go`、`raid_bakal_script.go`、`raid_bakal_retreat.go`、`internal/raid/bakal_script.go` 等。交付说明验证的是建队、进本、多房间加载与换场景；不能据此认定小怪和全部机制通过实机验收。

### 本轮闭合修复

- 当前 `20261006_143658_409112_next37` 会话中，巴卡尔 109014482 在 grid(2,1) 出场，普通换房后在 grid(2,0) 又生成。旧重建仅按 slot/map 匹配，漏掉原实现格点门禁。交付源码与交接二进制反汇编互证：`InitialMonster` 从同源 `LOCATION INFO / LOCATION XY / SPECIFIC GRID` 绑定出场格点，`bakalLoadedBoss` 比较当前 Room XY；第二阶段改用 `APPEAR GRID2`。已恢复该门禁，保持此前 defeatedLocations 和三龙 Awake 状态修复。不要把 DGN maze.Boss 当成所有区域首领唯一出场格点。
- 15:15:21.861 已发送 N574 `0000`，15:17:37.912 CMD2089 被拒，理由是没有 owned waiting-room member；并非失败阈值没判定。旧 tick 将 w.bakal 清为 nil，却保留客户端编队。现在失败保留归属及编队，成功发送结束计划后标记已通知，避免每秒重复结束；下一次合法 CMD2089 建立全新源状态和运行账本键，旧击杀/觉醒/怒气不带入。CMD13 仍可离队。

### 尚未闭合，禁止猜包

- 该会话共有 428 个 CMD39 在客户端37/2073加载请求后100ms内出现，确认入场死亡的现象；N29早于加载阶段，所以不能用N29到CMD39的时间直接排除。交付版与当前 StartMap 小怪记录编码、create-trigger默认逻辑、普通Session怪物加载相同；并未找到可以安全移植的小怪死亡修复。不能把未知255字段直接改为0，也不能过滤客户端死亡来掩盖根因。
- 此会话没有 CMD2070 的拒绝事件；缺入口不能直接认定是2070拒包。新增 `bakal_room_combat_state` 日志：每次死亡记录当前DGN、Map、格点、reported_entity、confirmed、cleared和剩余战斗实体(template/rank)。下一次用户手动验证可区分脚本未生成入口、存在残余实体、错误房间首领等情况。本轮不宣称三龙击杀后的入口缺失已完全修好。
- 重复规则检查：位置/模板继续使用同源 COS，未新增Boss格点/模板常量表。交付版完整事件执行、随机波次/增益与当前子集差距仍在，不据参考源码生成新的硬编码玩法表。

验证：真实内层PVF Bakal专项（wireprobe/catalog/legion）通过，增加所有巴卡尔第一阶段房间仅源arena生成Boss及失败后保持编队重新开始测试。全仓vet通过；全量test仍仅两项既有character来源兼容失败（TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference），无新增失败。未自动启动游戏或访问玩家库。

交付候选：`bin/wireprobe-bakal-arena-retry-candidate.exe`，SHA256 `92dc2fed6d7a9c02f9a629f40d9e1ff843571f96be81380f715fd95b2fa82889`。默认和隔离profile已接线；25项Python启动/profile测试通过，`launch_local.py --check`确认同候选、客户端`D:\115us\DFO`、PostgreSQL路线。旧二进制保留，本轮实机验证待用户手动运行`启动游戏.cmd`。未将小怪入场死亡和三龙入口缺失写为已修复。

## 小怪入场死亡闭合（后续取证）

此前“编码相同仍未定位”已由出生动作取证闭合：区域首领存在符号缺失导致Incoming_ready/Incoming_Dummy的CHECK RAID SYMBOL==0命中DESTROY。官服抓包和成功交接日志均有对应N570=1，当前会话缺失。已补齐源创建/确认击杀/换房快照和源INIT延迟创建，详细链路、回归范围及剩余机制见[小怪存在符号修复](bakal-minion-presence-20261006.md)。本次候选切为minion-presence-final，实机仍待用户验证；不宣称所有后续波次或三龙入口细节已经完成。

## 活首领入口格点回归修正

15:40 arena-retry会话仅冰龙/部分原生请求直接带战斗格点的首领生成，其余普通0,0入口被送入服务房。上一轮补齐出场格点守卫时遗漏了配套的活首领入口转换，已根据LOCATION/SPECIFIC XY恢复，7组实际2062向量走完整入口/加载/生成链回归通过。详见[入口回归与攻略对照](bakal-live-arena-regression-20261006.md)。默认候选升级live-arena，保留小怪presence修复与以前有效修复；完整BUFF和后续复活波次仍待实现，实机待用户验证。
