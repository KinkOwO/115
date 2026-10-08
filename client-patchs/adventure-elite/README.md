# 精锐小队资格与普通/剧情单人助战（DLL v0.3.9，剧情APC出现确认）

普通五房通关、多次入场、再次挑战及结算重新选图沿此前确认；本轮新增普通剧情ID5/Quest3146的APC出现确认。两名原生登记成功，日志有五房及任务完成/回城，但死亡函数调用/实际包存在一项未闭合统计，详情与范围见末尾本轮确认段。其它剧情层、NonCombat保护、奥德赛、特殊队伍及对象释放仍待证。环境开关按业主要求保留。

## 开关与启动

`DFO_ADVENTURE_ELITE`仅精确字符串`1`开启，服务端和Go启动器读取同一环境；启用即选择隔离候选并向本次Job创建的正确路径x64客户端注入本DLL。旧INI忽略。初始化失败拒绝启动，不静默回退。日志/诊断均解析自身模块目录，不依赖客户端cwd。

用户手动运行`scripts\启动游戏-SQLite.cmd --source-build`；保留现有名单，不重复保存。关闭时退出旧会话，然后使用以下路线，避开本机CMD入口已有的固定set（该用户改动保留）：

```powershell
$env:DFO_ADVENTURE_ELITE = '0'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\storage-route.ps1 game-sqlite
```

关闭恢复原生资格，存档中的低等级名单保留；真实角色等级、装备、技能及PVF不变。

## 已完成的本次手动流程

一次启动进入原角色，城镇短距离移动恢复已有精锐，进入一次基础普通单人非剧情副本。让APC击杀；顺手拾取掉落，按正常路线过门，若可行打Boss、结算/翻牌、回城，最后正常退出客户端。一次流程同时收集攻击归属、去重、拾取、换房、结果和离场信息。

真人攻击对照可选，没有掉落或某步骤被阻断就直接回城/退出，不为补单项重复跑。回城后如需要看精锐页，可自然打开一次补引用观察，不要保存。本段是已完成的首次通关记录；当前重复入场候选见下节。剧情、奥德赛及特殊频道不要作为本轮验收路线。本轮五房APC跟随过门已由业主确认。

## 客户端与服务端边界

资格补丁十个立即数100/2→0/0；城镇准备绑定真实CMD1811事务、线程、主人、频道和通知消费；名单仍存稳定角色ID。首房登记沿原生145B22F50块，十项身份/CRef/场景门禁全部成立才局部放行，复用已安装predicate钩子。

v0.3.8仅在当前精锐来源赋值TLS、同一怪物、精确setter caller5DD022B且稳定唯一弱引用/绑定/主人匹配时，把原生killer65535传为当前玩家ID。其它65535和调用者透传；不写APC控制器或owner。日志预算耗尽仍执行同样归属校验。三处原生函数保持返回值、异常和last-error；安装前核对当前代码字节，包括14014CC20 self getter。

服务端复用现有CMD39/43/45/46/69~72/117的死亡、拾取、换房、经验、结算、翻牌与离场处理；仍校验普通已加载run及冻结名单/主人/频道/设置。2015/2062特殊入口和剧情/奥德赛、军团/攻坚、教学/塔/深渊暂不扩展。未知killer没有服务端回退，CMD39正文布局不变。

普通范围继续消费同一只读PVF频道/副本源；奖励和数量来自现有reader及执行器，不增设玩法表。已有Type22/发布ID10兼容覆盖沿原台账暂留，本次未新增。SQLite schema及存档布局不变，默认程序不发布。

## 日志与构建

DLL目录保留status/log和按PID/runTick命名的native/lifecycle/entry/combat JSONL。生命周期1024、入场512、战斗4096记录预算，额外一条上限标记；丢弃/异常/不稳定引用明确报告。战斗增加原/实际killer、改写标记、来源唯一性/绑定/self getter及roomSerial；服务端记录同请求前后run/地图/等级/死亡/结果、pending地图和计划包。

```powershell
.\scripts\build-adventure-elite.ps1 -WithServer
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\collect-adventure-elite.ps1 -CheckOnly
```

正常退出后由agent运行`collect-adventure-elite.ps1`，汇总本次进程/资源/版本、通知/来源父子调用、原生死亡和CMD39、归属写入和服务端首次死亡、拾取/换房/结算/回城覆盖。`ownedCombatProcessedObserved`表示日志中的处理对应，不能代替客户端接受；`ordinaryReady=false`和`battleVerified=false`作为所有模式完整性标记保留，本轮通关确认依据业主反馈及对应日志。

MSVC x64 /Go1.26构建产物仅在忽略目录dist和bin；schema2包依赖qol.client-host，含MinHook v1.3.4许可证。继续使用已验证Job直接注入链；modkit安装/卸载未联合实机，不要并行自动加载同名DLL。回退须同组恢复DLL、候选服务端、启动器和build清单，备份在`.tmp/adventure-elite/owned-battle-delivery-backup`，更早备份保留。

实施范围、历史阶段及未闭环项见[计划](../../docs/todo/adventure-elite-ordinary-plan.md)。动态来源与本候选策略见[证据](../../analysis/tasks/adventure-elite-owned-combat-candidate-20261008.json)，首房AI确认见[登记证据](../../analysis/tasks/adventure-elite-registration-confirmed-evidence-20261008.json)。原始日志只留本机；程序及客户端均未由agent启动。

### 精锐普通单人五房通关流程确认（v0.3.8）

业主明确反馈“已完成通关流程，APC能跟随过门，能拾取，能打boss、能结算”。会话`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261008_223704_525989_next37`，PID93772 / runTick136444046；DLL772bd1e0、候选服务端3449b055、启动器aed177c4及四份源资源哈希匹配，采集缺口0、正常退出。一次准备、12通知、158生命周期、23入场和137战斗记录；54击杀者写入、56来源赋值、27原生死亡。

同一普通单人run经过58548→58549→58551→58552→58553五房，4次CMD45过门及5次CMD37加载全部接受。27个死亡均首次从dead=false变true、unowned=false、killer=主人2；按房间代次/精确来源关联54次setter适配，27个原生CMD39与服务端逐序对应，无死亡重发或阶段拒绝。实际发送27次ACK39/N38/经验N37，保留原有领域去重执行。Boss CMD117确认、CMD46结果/通关经验/奖励、CMD69/70/71翻牌及背包入库、CMD43拾取/场景移除/背包更新、CMD72回城均发送。events锚点：入口224，加载236/291/353/413/473，过门286/348/408/468，Boss549，结果557，翻牌570/573/576，拾取583，离场589；原始日志仅本机，摘要含日志哈希。

确认范围为该普通单人五房精锐参战流程，以及用户可见APC跨门跟随、打Boss、拾取和结算可用。拾取日志是当前玩家CMD43，不单独宣称APC自动捡物；经验更新已下发，不推断重登后数值验收。后续房间entry场景向量匹配值为0，但用户观察和逐房APC来源/击杀证明实际携带；不以该单一字段否定行为或宣布对象表全部闭环。最后城镇弱引用仍保留2个，清理/释放未证实。

本连接第二次入场仍保留限制；其它普通副本、召唤归属、剧情、奥德赛、军团/攻坚等特殊队伍，以及modkit安装/卸载继续未验收。ordinaryReady=false/battleVerified=false是已有候选的全局完整性标记，本次不改原始采集或扩大为所有模式完成。本轮仅文档/证据收口，不再变更运行路径、不增加attempt；默认服务端不发布，DFO_ADVENTURE_ELITE环境开关按业主要求保留，PVF/SQLite/schema保持。

证据：analysis/tasks/adventure-elite-ordinary-combat-confirmed-20261008.json。当前无需重复保存名单或重复该通关实机；后续先分析现有多房/回城证据，再为重复入场、剧情及奥德赛准备同轮可收齐的诊断。

## 已测重复入场候选：v0.3.8 第二次只有血条（未通过）

服务端stage为ordinary-reentry（attempt 1/3）。取消同连接首次入场限制，回城后重新打开普通选图可复用已准备同伴；仍要求同一主人、频道、名单和技能设置，排除活动副本/待城镇剧情和未支持模式。每次创建独立RunID及内存序号，原死亡/掉落/翻牌/结算处理继续负责重置与保存。没有数据库结构或客户端资源修改。

业主已完成首次通关及第二次入场，第二次只有血条、没有APC。原始日志保留，无需补跑旧步骤；第二次登记被manager576=1拦截。当前修复与新的短流程如下。

## 第二次只剩血条：定位与 v0.3.9 修复候选（reentry attempt 2/3）

业主报告“第二次进入副本只有血条，APC不见了”。失败会话20261008_231554_017721_next37（PID32220 / runTick138774906）首次普通run完成27死亡（其中13次有APC归属）、4换房、拾取/结果/翻牌/CMD72回城；第二次CMD16已成功创建独立RunID和干净状态，但零战斗，最终CMD42回城。采集缺口0和正常退出仅表示日志完整，不表示第二次成功。

entry第24–27行第二次loader来自146D2A1E7，仍有2个稳定精锐对象ID0/1、主人CRef及绑定不变；登记checks=[1,1,1,1,1,1,1,1,1,0,2]，唯一失败项是manager576=1，没有第二次原生登记。权威IDA确认142E60CE0只读该byte，145B251D3调用它（返回地址145B251D8），非零就跳过登记。原生setter145B25393置1，普通地图6264=0不能证明会走145B38A7D清理置0。四次已确认过门走146CE14C1且也保持active1，不能全局清零或每房强制登记。

v0.3.9增加精确新副本loader caller146D2A1E7与全部原生8参数范围。九项稳定主人/原生玩家CRef/唯一精锐type5/绑定/新场景未登记校验全部成立后，允许active1的重复入场范围；仅当前登记块getter消费者145B251D8、同一管理器和副本、冻结身份复核时将原生返回1局部视为0。仍沿145DE4360原生登记、位置/AI回调和原生setter收尾；不写manager576/6264、不释放/重建CRef、不修改APC控制器/主人。其它消费者、房间过门、未知active值和身份漂移完整透传。入口与active getter新增原字节门禁，安装失败拒绝启动。

客户端日志新增loaderCallerRva/freshDungeonEntry和registration-active-reentry阶段（原始active1，物理manager576仍1）；采集在同线程同一次loader内核对频道→active消费→逐同伴成功登记→loader结束新场景成员，并按顺序与服务端入场次数关联。日志预算耗尽不影响适配，但采集报告缺口，不能用上一场样本补第二场。服务端候选仍为已部署ordinary-reentry attempt1/3的5ca6128a，启动器aed177c4不变；本次重入客户端修复attempt2/3，构建清单分开标识，未改服务器包/玩法/schema。

下一次一次启动、保留名单和同角色频道：首次进原普通副本，允许APC击杀一个怪即可主动回城；第二次进同一副本观察身体/攻击，过一道门观察跟随，然后回城；第三次快速进图观察APC并击杀后退出，最后正常关闭客户端。无需每次打完整Boss、翻牌或重新保存名单。任何异常就记录所处次数并结束，后续日志自动区分首次、第二次和第三次。当前为离线候选，原五房confirmed baseline保持，不声明第二次已修好。回退同组备份.tmp/adventure-elite/reentry-registration-delivery-backup；更早备份保留。

现有Type22/发布ID10兼容policy仍暂留原台账，来源对照etc/channel_info.etc [server]；本次只改客户端登记消费，无新地图/等级/奖励Go或JSON定义。证据analysis/tasks/adventure-elite-reentry-registration-candidate-20261008.json。剧情、奥德赛与特殊队伍继续待独立闭环。

### 普通回城后多次入场确认（v0.3.9，reentry attempt2/3）

业主明确确认“确认通过，多次进本APC均出现”。会话20261008_234621_294907_next37 / PID100828，DLL94165690、候选服务端5ca6128a、启动器aed177c4及四份资源hash匹配。三次独立RunID与入场序号1/2/3，原生同线程loader每场两名精锐登记均返回1，结束新场景CRef成员均为1；第二/三场active原始及物理manager576仍1，精确145B251D8消费者各适配一次，首场不适配。三场分别有4/7/9次APC归属死亡，第二/三场各过一道门；三次主动退出均提交清空run死亡/掉落/翻牌/结果状态，无阶段拒绝。

确认该普通单人非剧情副本在同连接主动回城后可重复登记与战斗，不扩大为直达下一副本、HP/MP/死亡/冷却连续性或所有地图/模式。采集时客户端仍在运行，run.json和正常退出汇总未落盘；采集明确保留该1项缺口，fullCaptureCertified=false，不为补收尾重复跑图，不宣称正常退出或完整归档。原capture不改写。证据analysis/tasks/adventure-elite-reentry-confirmed-20261008.json。

本轮文档收口，不改运行路径、不增加attempt、不改PVF/SQLite/schema/default。DFO_ADVENTURE_ELITE按业主要求保留。下一项按计划处理普通结算再次挑战/非回城直达新副本；剧情、奥德赛、特殊队伍和对象释放继续待闭环。

### 再次挑战与结算重新选图确认

业主补充确认“再次挑战和选择另一个副本也是正常的”。同一会话现已正常退出，run.json已落盘。CMD72实际向量010001...（state1/option0），旧run9ad51701在1229行创建原有入场计划，1242行提交全新a1644789，清空死亡/掉落/翻牌/结算；对应第五次新副本loader ordinal56，两名APC登记成功，随后5次精锐归属新死亡。CMD72向量010101...（state1/option1）在1829行清空旧run并保留选图状态，1831行CMD16接受新runf55462a5，两名APC正常登记。

只读采集器原来仅数CMD16，漏掉无需CMD16的再次挑战；现在凭原请求、ACK16入场计划、成功提交与干净新RunID统计，保持实际服务端序号4，不伪造新序号。六次CMD16加一次CMD72共七次新副本加载，全部登记成功；正常退出、采集缺口0。78种离线夹具覆盖缺请求/脏状态/同RunID/缺计划/重复/缺loader，不改游戏运行路径。fresh build退出0；同658f3ffa Go源码的完整vet/test退出0结论保持。

本次日志中接受的副本ID均为3，因此确认结算重新选图入口及用户可见行为，不据此扩为所有不同地图。CMD2062和无缝option5没有本轮样本，继续待证。原三次回城确认的采集时缺口作为历史快照保留，本次完整收尾补齐。证据analysis/tasks/adventure-elite-retry-confirmed-20261009.json。下一项普通剧情战斗层，奥德赛随后单独处理；PVF、SQLite、DLL及服务器运行产物不改，默认程序不发布。

### 普通剧情入口候选（story attempt 1/3，待一次实机）

现有普通战斗入口拒绝非零Quest。当前PVF允许用既有DungeonSelection、任务接取状态和`dungeon.Select`解析剧情迷宫；本候选移除精锐入口处额外的`Quest != 0`拦截，不放宽玩家等级/已接任务、PVF maze/scene/战斗/奖励流程、difficulty/fatigue或原生boss门条件。非单人、奥德赛、教学/训练、塔/深渊和特殊频道继续走现有拒绝路径。没有新增Quest或地图手工表。

目标取自实机原请求`ID5/Quest3146/Mode0/Party65535`。当前Script.inner.pvf经原生索引确认`list/dungeon.lst → 5 → dungeon/act1/sunderland.dgn`，minimum required level为5；对应maze0的quest3146、start76131、boss坐标(3,1)。源任务`grandflores_03.qst`保留前置4873并连接后续3147，服务端仍只认可玩家真实任务状态。当前PVF source test证明任务未接取、低于源最低等级及不存在的quest maze均被原reader拒绝。

为一次实机，服务端新增入口请求/Quest/模式/party、source maze/boss/剧情层/NonCombat计数，及入场、过场、房间、精锐登记、归属击杀、通关和结算关联日志；采集器覆盖剧情拒绝场景。DLL沿用已验收v0.3.9；环境开关仍是DFO_ADVENTURE_ELITE=1。候选仅证明该一个source剧情入口的请求被允许，不证明APC正确参与过场、不会攻击NonCombat演员或所有剧情门可通过。详细步骤与止损点见analysis/tasks/adventure-elite-story-candidate-20261009.json。

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
