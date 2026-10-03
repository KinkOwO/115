# 怪物掉落与爆率调查（2026-10-02）

当前范围（用户最新定调）：只处理普通副本的怪物掉落与通关翻牌，传统 Hell Party、奥德赛均暂缓；已有 Abyss/调律行为保留。用户已确认普通装备掉落和免费装备翻牌，并要求先提交，接下来处理材料与消耗品。完整115官方爆率、怪物自身独立池、control/Smart控制分支和付费翻牌不在已完成声明内。后文“待复测/未提交”及“等待 Hell 实机”的段落为历史记录，以本节为准。没有启动客户端、操作玩家数据库/schema或改动客户端运行资源，没有覆盖默认二进制。

## 本阶段收口：普通装备掉落与免费翻牌已确认

用户确认原文：“能掉装备了，翻牌也能出装备了，先提交这里；接下来解决材料和消耗品的缺口”。手动会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261002_191351_952423_next37` 的启动命令明确使用scoped候选。角色21于19:21:35.146收到Boss确认、19:21:35.187收到NOTI35；19:21:38.281装备拾取成功并刷新背包，19:21:41.974完成翻牌入袋与CMD71成功回执。卡片模板408030093在刷新背包槽14中可见。确认范围限于上述普通装备玩法，不外推为材料/消耗品、存档全部内容、全地图或官方完整概率。

独立候选SHA256 `ce476c14359b012b24aedcbf8dfead763ddccb71a3d1c40ddafe97c30c790a9e` 纳入本项confirmed baseline，同目录 `启动验证.cmd` 保留。源码/验证记录与根CHANGELOG、server/AGENTS及开发交接同步收口，用户其它文件和默认程序保持。全量测试保留经HEAD对照的4项既有失败，相关领域及当前源回归、vet通过；不把全量写成通过。Hell表解析只是只读源投影，不启用Hell发奖。

## 下一阶段：材料/消耗品源边界（提交aff2337之后）

用户要求继续补材料与消耗品。进一步原生取证否定了“STK缺省创建率按1”这一方案：common reset `1470F42E0` 在 `1470F432F` 清空对象qword+0x10；STK parser `147110790` 的case21039由 `[creation rate]` 字符串 `14B210238` 注册，在 `14712BD1B` 写入DWORD+0x10。因此普通STK未声明创建率时原生字段为0，不能把所有材料、现金道具或食物均匀塞入通用掉落池。装备阶段未声明创建率的兼容策略仍按该阶段文档记录，不能倒推成STK原生默认。

原生MOB reader `147444B30` 的 `[item]` case2757在 `147467A99` 将vector end+0x6C0重置为begin+0x6B8，随后两次int读取形成8字节记录，通过 `1403FAE90` 原样追加。重复[item]替换前段，不能参考正则只读首段，也不能简单拼接。此链只证明记录读取，未证明总触发率或第二格概率分母。

当前PVF哥布林1/2、龙人70/71确实有这条独立物品表；70的原始对为1063/50、1002/100、1047/200。普通材料3012和这些消耗品在STK中都未声明创建率。新增独立只读 `catalog.ParseMonsterItemTable` / `ImportMonsterItemTables`，保留路径/完整脚本SHA、原始有符号数值、重复段替换与明确空段；未知LIST模板、截断或非整数对拒绝。没有接入死亡或卡片生成，未变更已确认二进制、爆率或存档。

参考端存在实质冲突：S4 `DropGenerator.cs` 为MOB池额外固定1000/10000触发；usdof `mob_item_pool.py` 为每次必选一件。S4/usdof的区域材料表还硬编码副本区间和25%，不能作为当前115真源。`independent_drop.etc` 和 `worlddrop.etc` 是另外的路径，已定位原始源，但表结构、条件、分母与触发未作为本阶段已实现声明。根据根AGENTS的证据门禁，已询问用户是否将S4 10%明确作为本服兼容策略；回复之前不启用该率，也不以实机等待冒充能从客户端证明官方服务端公式。

### 用户指定策略后：MOB材料/消耗品候选

用户回复：“用环境变量定义吧，默认10%，等以后找到了再改”。据此新增 `DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT`，整数0..100，空值/未设置默认10，0关闭，非法值在打开存储前拒绝。只作用于普通副本MOB[item]专属池，独立于全局/地图普通生成与翻牌，不将其冒称为115官方率。池内正值按S4兼容选择权重，每次命中选一件；不再以通用创建率/等级窗口删除这类明确声明的物品。

原生LIST/MOB由独立只读视图按需读取，16MiB/256条脚本缓存，源父归档关闭后仍有效。重复[item]最后一段覆盖，未知/负值/零权重/非堆叠/任务/策略排除模板不加入有效池，不可读源产生诊断、不替换源也不阻断既有Boss结算。没有有效池或率0不推进新分支随机状态。只在归属本场的普通战斗死亡启用，遵守随机掉落排除；Hell、奥德赛、Abyss/调律不消费该路径。奖励追加于地图奖励之后，不删除既有装备/金币；领取沿用现有Awarder/角色事件，未改包布局或schema。

真实当前PVF回归验证怪物1/70的草莓1000、朗姆酒1002、恢复食物1063及65005声明的魔力溶解剂3227（[material expert job]）能生成地面对象并入袋，存档future_field与期限保持；源视图生命周期、死亡重放与暂缓模式/排除/归属/关闭隔离通过。默认10%和1:9选型权重的固定种子统计通过；100%仅用于离线强制覆盖，不修改候选默认。当前准备检查27.438秒通过，普通装备6905/旧池2794保持，并验证MOB70视图与6013排除策略。新测试夹具的slice/slot访问、期限字段名、MaximumGrade及vet要求的外包具名字面量错误已修正，不把中间检查写成通过。

最终 `go test ./...`（`go-test-monster-material-final-keyed.txt`）中catalog、loot、dungeon、inventory、workflow、savecontract与协议包通过；仅保留前阶段HEAD对照已复现的4项失败：wireprobe的 `TestAdventureAuditProvenanceAllowanceIsNarrow`、`TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback`、`TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader`，以及cashshop的 `TestShopPilotPVFCurrentCatalog`（SKU3400013空发放）。全量退出1，不宣称全通过。最终 `go vet ./...`（`go-vet-monster-material-final-keyed.txt`）退出0，`git diff --check`通过，构建成功。

独立程序 `.tmp/drop-audit-20261002/wireprobe-drop-materials.exe`，SHA256 `1584a73a30b896d1e493d55702832bfb441351cffb89b6b2c89a396b4583f918`；独立profile `pvf-drop-materials.json` 只替换binary，入口 `启动材料验证.cmd`。scoped装备confirmed候选保持ce476c14，默认EXE不覆盖。按同一普通掉落功能保守记录累计 **attempt 3/3**；这是用户明确选择的兼容策略，不新增C2S或S2C字段。若失败先据源与实机日志取证，不盲目换包或编造新率。MOB未声明的其它材料来源、独立/区域/worlddrop尚未全面实现，付费/材料翻牌不在本候选声明内。此候选待用户手动验证，尚未确认/提交。

## 20:13实机追查：剧情怪无专属池与全局材料缺口

用户反馈几次普通副本基本没有材料/消耗品，并指定核对全局表与实际怪物，而非直接调高概率。手动会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261002_200452_104132_next37` 启动日志明确记录当前4d8c源、MOB兼容率1000/10000以及1022个通用STK候选。NOTI33/后续房间spawn与NOTI38死亡对象逐项关联：副本12/13/14/15分别28/55/61/36条死亡确认，共180条；34种模板（包含不发随机奖励的剧情实体）均未声明MOB[item]。NOTI38实际地面对象为金币11个、装备4件（104030492、116010066、117010025、28313），没有STK物品；不把翻牌计入地面掉落。

142次 `monster_item_source_refused` 揭示本候选的路径错误：LIST已给出归档根 `contents/2022/new_scenario_renewal/...`，读取器仍无条件添加 `monster/`。PVF目录扫描和直接读取确认原始LIST路径的文件存在，并非资源缺失；23种出现拒绝的模板均对应此问题。修正为优先保留存在的精确LIST绑定，只有相对路径才补旧monster目录，两条导入/按需视图路径共用。真实源回归同时验证哥布林1保留旧路径/池，以及109014957、109014964、109015032使用contents根路径且不凭空补池，父归档关闭后视图仍有效。这里只修有明确源证据的路径错误，没有更换包或新增未知发奖公式；累计attempt3/3保持，不追加盲试。正确读取实际这34种模板后仍全部未声明[item]，因此10%或100%都不能使本次怪物从专属池产生物品。旧哥布林/龙人模板的池不能移植给同名新版剧情模板。

当前通用STK候选1022种仅含1007个[material]与15个[throw]；草莓1000、朗姆酒1002、恢复食物1063、普通材料3012都未声明创建率且不在池内。实机怪等级15..18：对应等级/稀有度窗口可命中的候选只有投掷物，15级额外两个rarity4投掷物在当前普通稀有度第一行（rarity3已达1000000/1000000）无法被选中，没有可命中的普通材料/恢复消耗品。两条通用STK分支基础率在1..15级为60/10000与0，16..23级为30/10000与17/10000，再乘rank/难度；这些低概率只影响已有候选，不会补出缺失物品。

地图来源也不能补齐：12带率索引只引用unique11005/legendary11006；12..15的DGN自身均引用11005。源组11005全部194行属于EQU，11006两行也均为EQU，没有STK。13..15无带率全局索引，现行声明组分支会把通用物品预算转交11005，原STK结果也被组装备替代。这是当前服务端行为与源组成的确认，不把该替代语义冒称115官方规则；不能因此给装备组硬塞材料或复制旧同名MOB池。`independent_drop.etc`、`worlddrop.etc`及区域材料来源仍需进一步闭环，未实现部分才是普通材料目标的主要缺口。现有实机证据足够定位，不要求用户通过更多重复跑图证明概率问题。

只读复核工具/结果为 `.tmp/drop-audit-20261002/live_material_audit.go` / `live-material-audit.txt`，不入Git。当前源路径回归 `go-test-monster-root-path.txt` 通过（10.936秒）；最终全量 `go-test-monster-root-final.txt` 中相关包通过，仅保留上述4项已对照的既有失败，退出1；`go-vet-monster-root-final.txt` 为空且退出0，diff检查通过。路径修正程序为 `wireprobe-drop-materials-root.exe`，SHA256 `120fb257c275e400f5c371d8ccf6299bf7fa404829d1b1b995710546810417b3`，原材料独立profile只更新binary指向它，旧1584候选及装备confirmed候选保留。默认程序不覆盖。这不是普通材料掉落完整修复，尚未确认/提交；本次未操作玩家存档、schema、资源或客户端。

## 世界材料/消耗品：用户实机确认

用户继续指定核对全局与实际怪物来源，随后明确回复“采用参考规则，默认1倍”；实机确认“能掉落消耗品和材料了，可以提交”。本轮新的取证范围是独立/世界/区域来源，世界发奖是用户明确指定的兼容策略；不是在旧attempt3/3上继续换包、无依据硬编码或盲试第四次。新的世界来源验证记录为 **attempt1/3**，保持既有协议布局和MOB策略。本项已纳入confirmed baseline；独立主表、区域来源及材料/消耗品翻牌继续留待后续取证。

当前源 `etc/worlddrop.etc` SHA256 `b75a987a68d93e579b56e67c3dd0309b9bc5e5813efda38073c640057975dc71`，46202 cells、200行。逐行闭环等级/保留列、物品权重对与-1边界；保留列当前全部0，不冒称115官方语义。15～18级正权重合计7006，材料3030/3028/3027/3142/3151/3156分别850/830/830/830/830/830；15级药剂1107/1113各1000，16～18级为1108/1114各1000；6种制作配方各1。材料及药剂均有当前原生STK/LIST绑定，因此问题是服务端遗漏世界来源，不只是MOB池触发太低。S4 `WorldDropSystem.cs` 为权重和/100000独立触发、命中再按权重选一件，usdof同分母但查找(r)/(f)文件；当前只读现存的worlddrop精确源，不复制旧区域硬编码或造备用表。

115原生MOB `[exclude world drop]` 在14744CB96注册case11379，1474541BC对MOB+1694执行OR8、不消费数值。已注释IDB并保存；只读MOB投影保留此布尔标记，world发奖必须先读取本场怪物的精确源并遵守它。world运行表和策略标记json:"-"，不进入内容快照；即使命中普通loot派生缓存，EnableRuntimeDetails仍从当前已验证PVF重新解析此表。父归档关闭后世界表及STK/MOB只读视图保持有效。无玩家数据库/schema或资源改动。

新增 `DFO_ORDINARY_WORLD_DROP_PERCENT` 整数0..10000，空/未设置100=1倍，200=2倍，0关闭，非法值拒绝启动；按源权重和乘百分比/100生成阈值，分母100000，阈值封顶100%。这是用户指定的参考兼容公式，不声称取得115官方server公式。第二列非0或权重溢出拒绝该次世界奖励并诊断，不阻断既有死亡/Boss结算。只选源中正权重条目，一件/数量1；选中未知、非STK、任务、策略排除或不可读项则跳过，不重新归一化或重抽其它物品。该分支追加在普通地图生成之后，MOB[item]未声明仍可世界掉落；不补通用创建率、不替换装备金币，不进入翻牌。排除随机、未归属、剧情/APC与Hell/奥德赛/Abyss/调律边界保持。

独立主表另行只读取证：`etc/independent_drop.etc` SHA256 `3e13815db2e4ef8a55b905d86217d4067791fea4aaae3ec6fd6eed3ab8e0f798`，1806条：953直给flag0、844内联flag1、9替换flag5；当前外部 `independentdrop.lst` 为空。115 reader147C74470读取17-int行，flag上限5，flag5有replace list及可选dungeon condition；已命名IndependentDrop_ReadSourceTables并写索引。唯一已见直接调用1450B0840加载的是Independent_Drop_Card.etc，用于MonsterCardDictionary元数据；不是独立怪物发奖，也不证明官方触发分母、计数索引和条件解释。实际34种模板中主表仅59520/59522有匹配，原始五列均2300、内联10149045..10149052各12500；这些8种物品是原生[waste]，不能误报为装备。新版109014...怪物未匹配主表。旧参考端对计数索引、external分母及entry_type处理存在冲突；本候选未接入这条主表，不把它算作已解决范围。审计JSON只保留于.tmp，不是运行内容源。

聚焦测试 `go-test-world-focused-final.txt` 通过：默认1倍100000个固定种子接近7.006%，正确保持源选型权重，排除/未知项不重抽；当前PVF的109014957在15～18级离线强制覆盖6种材料及4种HP/MP药剂，地面对象、入袋、期限与future_field保持、死亡重放不重复、暂缓模式/归属/剧情边界通过。初次夹具遗漏NextEntity导致drop identity exhausted，已补回真实会话要求的身份空间；该错误没有改运行逻辑。强制倍率仅用于离线覆盖，候选仍默认1倍。

当前准备检查 `go-test-world-prepare.txt` 通过（40.69秒），使用4d8c源、默认世界100%倍率、MOB1000/10000，世界200行、装备ordinary6905/legacy2794保持。最终 `go test ./...`（go-test-world-all.txt）相关catalog/loot/dungeon/inventory/protocol/workflow/savecontract/storage全部通过，仅保留已由HEAD对照复现的4项既有失败：wireprobe三项来源审计、cashshop SKU3400013 empty delivery；全量退出1，不宣称全绿。`go vet ./...`（go-vet-world-all.txt）退出0，diff检查通过，构建成功。

新独立程序 `wireprobe-drop-world.exe` SHA256 `5e40b294dfd92ab27408b13f0f5d9918c79b30f1bdecb6a6bf9f6a6cea49329f`。新profile `pvf-drop-world.json` 相对材料profile只修改binary，环境集合比对一致；新入口 `启动世界掉落验证.cmd` 调用根启动游戏.cmd --repair-profile。它们位于 `.tmp/drop-audit-20261002/`，不入Git。用户确认材料/消耗品可以掉落；该候选纳入本项confirmed baseline，默认程序哈希仍77ce8513/2e00530b，未替换。详见 `server/work/dfo-lan/docs/ordinary-world-drop.md`。普通材料/消耗品翻牌和其它来源不外推为已确认。

## 18:33 实机回归：难度 0 阻断结算

用户反馈“打完boss不触发结算了”。手动会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261002_182316_881531_next37` 的 CMD16 原文为 `0b0000000000000000ffff000000000000000000000000000000000000000000`，即副本11、难度0；18:33:46.162 的 `boss_check_confirmed` 为 `01014610`，死亡/完成确认正常。随后18:33:46.177 的 CMD46 被拒绝，原因精确为 `ordinary clear-reward table/difficulty unavailable`。新 `ordinaryFreeCard` 的1..5门禁与已有 `dungeon.Select` 的0..5准入、死亡掉落的难度0→第一列处理不一致，导致 `FreezeCards` 返回错误，结算通知未发出。

修正普通翻牌共享难度索引：0与1均使用第一列，2..5对应后续列，>5显式拒绝。金币和物品使用同一索引，未修改请求或结算通知结构。此为实机错误驱动的服务端边界修复，不增加猜包尝试；原免费物品生成仍为 attempt 1/3。旧候选保留可回滚，新程序另存 `wireprobe-drop-audit-zero.exe`。补充零难度 `PlanCards` 回归和当前PVF等级15/0对照1的完整计划回归；客户端与玩家数据库没有操作。暂不升级 confirmed baseline，待用户复测。

## 普通副本实现候选（用户授权继续后）

### 本次实机追查：地图类别覆盖过宽与普通材料缺口

同会话用户反馈只掉金币。当前PVF副本11 `dungeon/act2/draconiantower.dgn` 的带率索引只声明 unique→11005、legendary→11006；普通怪率分别为1157/1000000、139/1000000，Boss率分别218750/1000000、26250/1000000。此前普通严格替换错误地清掉所有非金币，等于让这两类低频装备条目删除普通白蓝装备、投掷物及其它通用堆叠物。当前修正按声明类别覆盖：unique/legendary未命中依然不能保留同类别通用结果，但保留不属于它们的类别。

权威IDB会话753d5552：reader `1473D30F0` 调用 `147399CC0` 解码[type]，原生枚举为special0/epic1/legendary2/unique3/rare4/random5/stackable6/primeval7/unknown8；**这不是EQU rarity**。当前PVF11005前三个装备模板117010020/25/30的EQU rarity均3，11006的100322786/7均6。新增普通类别匹配使用EQU rarity rare2/unique3/epic4/legendary6；已确认Abyss/调律及暂缓Hell/奥德赛维持旧分支，旧 `gradeMatchesRarity` 不改。原生解码函数命名 `DungeonDropInfo_DecodeRewardType`，记录仅证明reader分类，不声称客户端模拟了服务端完整发奖公式。

当前普通材料仍有真实缺口：未应用排除策略的源严格STK池共1023条，只有14条rarity0投掷物、2条rarity4投掷物、1007条rarity4材料；正式入口按 `pvf-drop-policy.json` 排除6013后为1022条。未排除的15级窗口只有8条rarity0投掷物与2条rarity4投掷物。源中普通材料3012/3015/3016等有明确grade/rarity，但未写[creation rate]，旧导入直接排除。参考S4的MonsterDropConfig也把该缺省当0，因此不能把参考逻辑当115默认规则；未证实STK原生缺省值/怪物专属池触发条件前，不为这些材料编造生成权重或固定百分比。命中但候选为空时新增 `stackable_grade_window_empty` 诊断，避免继续静默消失。材料与回血等消耗品池不能写成已修好。

原生PVF回归 `go-test-live-map-scope.txt` 与应用6013排除策略后的 `go-test-live-map-scope-policy.txt` 均通过，后者22.09秒：实际Select副本11/难度0/15级普通怪，在原始概率下2000次死亡产生普通装备11、堆叠物14、地图装备1、金币191；普通装备和堆叠物均经过真实Awarder入袋回归。零难度免费卡计划与难度1一致，等级5/15/20/55/100/115各有961/2000次物品牌。这里只是离线确定性样本，不是玩家实机已确认；源概率较低，单次通关没有普通物品依然可能发生。

当前验证候选为 `wireprobe-drop-audit-scoped.exe`，SHA256 `ce476c14359b012b24aedcbf8dfead763ddccb71a3d1c40ddafe97c30c790a9e`；原 `启动验证.cmd` 的独立profile已指向它。旧6650a3ce候选和零难度单修候选2cf36015均保留可回滚，默认二进制仍77ce8513/2e00530b，未覆盖。普通掉落/免费翻牌候选按同一用户功能保守记为 attempt 2/3：据实机错误和当前源分类修正，未修改协议字段或客户端。全量 `go-test-map-scope-final.txt` 中loot/dungeon/inventory/protocol/workflow均通过，仍仅3项已对照复现的wireprobe审计失败和1项商城SKU3400013失败；`go-vet-map-scope-final.txt`为空且退出0，diff检查通过。补充真实native回归应用运行入口相同的6013排除策略；早期新测试夹具的缺import/Bag字段编译错误已纠正，最终检查不将这些中间失败写作通过。权威IDB保存返回ok=true。没有操作玩家数据库/schema。完整普通材料掉落目标仍未完成，不升级confirmed baseline、不提交。

本轮运行路径变更按普通物品翻牌功能记录 **attempt 1/3**：使用当前PVF数据与明确命名的参考兼容公式，沿用已经闭环的NOTI35第一通道布局及CMD71领取，没有新增包字段或其它通道试探。若实机失败，先收集日志与对应原生消费证据，不盲目换布局。

- `loot.Parse`新增普通怪物表的5类×5难度投影。普通Roll使用各类目自己的倍率，与当前全局基础概率和怪物rank倍率组合；金币金额使用金币行。Abyss/调律、实际Hell入场及奥德赛仍走原有策略/随机序列。
- 普通装备池从当前equipment LIST/EQU自动发现，独立于旧`basic_equipment_ids`，并使用已有Basic可发放边界（普通主背包装备、自由附件、rarity<=2、耐久规则）。当前源为6905件；旧特殊模式池仍2794件，不扩大它。源显式创建率<=0排除，正值用于权重；未声明时保留现有均匀兼容权重1，**这是本服兼容策略，不是已逆向出的115缺省值**。排除ID策略仍生效。池投影按实际源/程序/索引/排除策略与已有交易选项绑定缓存。
- 堆叠物保留PVF正创建率作为权重，普通选择使用权重；旧Roll依然均匀选取，避免改特殊模式序列。普通模式取消“抽到空稀有度后自动换邻近品质”的替代。
- 普通成功物品掷骰单独形成地图预算。即使通用池没有该等级/品质的候选，声明地图池仍可接收预算，不会把“没选到通用物品”误当成“概率未中”。地图组未命中仍不回退通用物品。普通装备在生成地面对象前整体校验源耐久，失败不发布部分对象。
- 普通免费翻牌用clearreward的default四列profile、地图探索率、难度加成、四类权重、九档稀有度与有符号等级窗口生成独立物品。只填已有原生第一通道的主背包装备结果；其它类别保持空结果，不转为主装备。当前default辅助值必须0；未识别分支不回落。参考兼容公式不冒称115原生完整公式。
- 物品与金币一起冻结；选牌仍读取同一服务器奖单并通过原角色事件事务发放。新`ItemModel`是兼容可缺省的诊断字段，不改schema，旧奖单仍可解析。黑鸦专属奖单路径未改。付费翻牌未启用。

当前PVF只读证据：完整424216条EQU扫描无读取错误，显式正创建率只有5701件，几乎没有白/蓝。白/蓝各等级条目仍在源内；不能把未写创建率当作源显式零。ItemDictionary样本10061记录`[10061,5,20,11101,1,...]`，对应EQU是rarity3/grade24/创建率500；100050204字典为rarity8/grade85/列4=2051，对应EQU是rarity4/grade91/创建率125。字典列4不能冒充EQU创建权重，未用它生成物品。

验证结果：

- 当前PVF五档等级5/20/55/100/115，5房间探索3房间、王者难度：每档2000个固定种子产生961个主装备翻牌结果，均通过源发放检查，并逐档实际入袋校验未知存档字段与期限保留。该统计是此测试条件下的兼容实现结果，不是所有地图的固定爆率。
- 当前源5级与115级普通地面装备生成、耐久行、死亡重放通过；全局/怪物/难度组合、1:9候选权重、通用空池地图预算及暂缓模式隔离回归通过。
- `go-test-ordinary-ready.txt`相关包均通过；全量仍为此前复现的3项wireprobe审计失败及1项商城SKU3400013空发放失败，无新增失败。`go-vet-ordinary-ready.txt`退出0，diff检查通过。前一轮新预算测试夹具缺少外层源标签已纠正，最终全量通过该测试。
- 原历史本地启动测试固定引用7ef角色JSON，不能验证当前4d源；新增按默认入口显式关闭历史基线对照、使用PVF角色定义的只读准备测试，`go-test-preparation-cold/warm.txt`均通过，冷27.91秒、热9.11秒，普通池6905/旧池2794一致。没有打开存储或启动游戏。

独立候选：`.tmp/drop-audit-20261002/wireprobe-drop-audit.exe`，SHA256 `6650a3ce561888603aa5cd04538b32cca9c5c02cf0790703704e1f8a6a228852`。入口`.tmp/drop-audit-20261002/启动验证.cmd`用独立profile指定此程序。程序下默认资源、存档与功能参数不另行替换。18:10核对现有默认PVF程序为`77ce8513c072f3b61862a69622c566e69fdb480d424c83e085e15e2fbe645683`（期间外部更新，非本任务写入），源码默认入口仍`2e00530b...`；均保留，不将外部程序身份混作本候选。

实机仅需普通副本：观察地面物品及装备拾取，通关数次后选免费牌，核对物品显示、背包到账与重选角色后的保持。每次通关不保证抽到物品。若异常，记录副本名/等级/难度与大致时间，再读对应服务端日志。当前尚无本候选实机确认，不升级confirmed baseline，不提交。

以下“仍只生成Gold”等内容为此前阶段的历史记录，以本节为当前实现状态。

本轮范围隔离：普通组空结果处理和共享随机状态推进，仅在非奥德赛、非实际 Hell Party 入场、非 Abyss/调律时使用；普通翻牌金币难度修改使用同一范围判据。实际 Hell Party 以本次 Session.HellPosition 判断，普通副本仅声明可进入 Hell Party 不会被排除。测试覆盖暂缓模式的旧掉落结果、翻牌金币和死亡重试。

范围隔离验证：`go-test-ordinary-scope.txt` 的 catalog、loot、dungeon、inventory、protocol、workflow 等相关包通过；全量仍有3项此前已对照复现的 wireprobe 失败，另有 `TestShopPilotPVFCurrentCatalog` 的 SKU3400013 empty delivery 失败，使用改动前 HEAD overlay 单独复现（`head-cashshop-scope.txt`），不将全量写成通过。`go vet ./...` 退出0，diff检查通过。独立候选重新构建SHA256 `0c5cd443be01d3d8ea4847ba075363837257debce71048e963456fcffc9bb434`；下文 `f41be...` 是范围调整前的历史候选。没有启动候选或客户端，默认二进制不变；此候选仍缺普通物品翻牌生成，不作为完整验收版本。

普通调查结论：`PlanCards` 只填 Gold，没有普通物品生成调用，故正常翻牌必定没有 Items；普通怪物 Roll 未读取 PVF 的难度25格/组队20格，也未消费 control 与怪物自身 `[item]`。装备候选依赖旧 basic selection，选择均匀且存在邻近稀有度回退。参考端提供独立翻牌生成链的线索，但其类别对应、装备生成权重和当前四列 profile 辅助值尚未由115证据闭环；这些是普通玩法自身的证据缺口，与暂缓的 Hell/奥德赛无关。不能将已修复的组回退问题或冻结领取事务，写成“普通爆率/物品翻牌已实现”。

## 历史：范围调整前的等待实机证据检查点

在01:36候选准备、01:57翻牌直读投影、02:11原生显示链取证之后，同一手动Hell验证缺口仍存在。本轮复核最新会话仍是 `20261002_003945_056917_next37`：`events.jsonl`最后写入00:47:13，`gateway.err`最后写入00:40:41；未发现候选/客户端或本任务Go测试进程。没有把等待用户回复当作正在运行的验证，也没有重启环境。上一轮为有效原生证据进展，本轮未获得能解除缺口的新证据。

完整目标审计：

| 要求 | 当前证据与结果 | 未完成部分 |
| --- | --- | --- |
| 普通怪物按全局、地图和怪物规则掉落 | 已有死亡链回归证明组未命中/空组结果和连续随机状态修改；当前`loot.Parse`仍只读旧通用五段，普通Roll不读control/MOB专属池 | 完整组合算法、创建率/Smart分支和全量候选边界缺当前版本依据，不能靠现有测试宣称完成 |
| 通关翻牌出物品并受独立规则控制 | 四列profile、嵌套倍率直读回归通过；冻结领取可原子授予装备/堆叠物；NOTI35显示链静态闭环 | `FreezeCards`普通路径仍只生成Gold，Items为空；独立物品生成、概率类别/辅助字段消费、各奖励通道和实机显示/领取未完成 |
| 传统Hell专属爆率，不改变Abyss | 当前Hell史诗源7区域/63列表直读；普通修改对Abyss/调律保持历史序列的专项测试通过 | 动态怪归属→CMD39死亡→专属发奖没有当前实机记录，波次/概率/区域映射仍缺证据；不能把固定地图怪或Abyss奖励当作Hell交付 |
| 真源、存档、工作区与验证门禁 | 当前PVF直读；没有新增内容JSON输入、DLL、schema或玩家数据库操作；完整test中相关包通过、3项既有失败，vet退出0，diff检查通过 | 尚无用户实机确认，未提交或更新confirmed baseline；这些门禁通过不替代玩法完成 |

候选与入口重核存在，程序SHA仍为 `f41beefd706e681f134255472601db569f8a48a983eef815623ec0c0f8538429`；两个默认二进制仍为 `2e00530babeb9b6ed4e357efce6a663fefc6c1d9b7da31843e383c0f945e9c7d`。后续必须先取得用户手动Hell运行记录，或用户提供新的权威取证资料/范围；不再重复同一候选准备、测试或猜测运行路径来假装推进。目标保留完整范围并标记blocked，不标记complete；新证据到达后恢复调查。

## 实施检查点与参考端对照（2026-10-02 01:36）

继续取证（02:11，上一轮为有效进展）：当前工作树与候选仍在；最新手动会话仍为00:39启动、00:47结束，没有新的Hell测试。当前客户端 `client/DFO.exe` SHA256核对为 `1d3948784e5e0f77ed744017bf82bf0c50de9421423f9e59f6f70aed609d8ffa`。

NOTI35的当前消费链进一步闭环：`1452A7508` 每行重置320字节临时物品状态；`1452A7519` 读取21字节到临时状态+239..259（栈地址差 `257h-168h=EFh`），不是整个装备实例。`14576EE90` 在+239清8字节、+247清4字节、+251清1字节、+252清8字节，恰好覆盖这21字节。`1452A7869` 将临时状态作为第6参数送入 `146AA0AE0`；后者按模板目录查询二元类型，值2时把数量改为1并保留状态指针，否则传null。显示工厂 `14603E2D0` 调用 `1450133D0`，后者将+239的16字节、+255的4字节和+259的1字节复制到显示对象。此证据证明字段转运/缺省值，**不证明这21字节的业务含义，也不证明clearreward四个概率类别对应关系**。`146AA0AE0` 已命名 `ResultReward_AddTemplateRow`，注释及权威IDB已保存。

同一handler的3个通道结构不同：第一通道8个参与者列表，每行template32/amount32/metadata21，UI channel=0；之后读取一u32并送入 `146AB9190`（仅确认存入UI+3792，未擅自命名金币价格）；第二通道每行只有两个u32，UI channel=1；第三通道又有metadata21，非空列表后额外读取一u8并调用 `146ABBC20`。目前encoder只填第一通道，其余保持已验证空路径。尚不能把三种通道的字段布局统一，或据位置认定付费翻牌已经闭环。反编译保存于 `.tmp/drop-audit-20261002/native-card-consumption.txt`。

历史native oracle使用过期的 `F:/dnfop/DFO/DFO.exe` 路径；本轮仅查看其实现，没有执行该入口或改写原生向量。当前便携Python与系统Python均缺Unicorn，旧向量只能证明其明确声明的cursor/调用参数范围（游戏/UI主体为stub），不能用来声称当前装备卡片已实机显示。当前没有改新协议字段或叠包，因此C2S尝试计次仍未增加。

补充验证记录：加入翻牌直读投影后的全量 `go-test-clear-projection.txt` 仍只有下文3个既有失败；catalog/loot/dungeon/inventory/protocol/storage通过，`go-vet-clear-projection.txt` 为空且退出0。该证据覆盖解析/现有链路回归，不覆盖尚不存在的普通物品奖单生成或Hell奖励。

后续静态推进（01:57）：新增 `catalog.ParseClearRewardTable` 的当前PVF直读投影，接入 `ImportLoot`；7个命名profile各6行，按四列读取并保留未知辅助值。组队倍率保留3种副本类型×4人档，金币难度保留3种副本类型×5档，不再被平铺section读取吞掉；另保留5个地图数量/率对、200个金币翻牌价格、200个等级参考三元组、9列稀有度与有符号控制值。未知profile/越界等级不回落。当前原始PVF测试通过，覆盖缺第四列、缺关闭标签、短嵌套行、重复核心段的拒绝。此投影尚不生成物品奖单，不能把解析完成等同于翻牌修复。

新增115 IDA证据：`1417C4D40` 在 `1417C4DDB` 以对象+40调用 `147426050` 读取 `dungeondroptablebygroup`。该reader读取两个creation整数；`[drop item]`在首值非零时保留原始对，首值为零时在 `147426DE2` 累计第二个字段，分别存入reader对象+32与+64的map；`[smart drop item]`另存+48的map。函数已命名 `DungeonDropGroups_ReadTables` 并保存注释/索引。这否定“原生直接合并Explicit和Smart”的说法，尚未证明三个map的发奖分支和概率分母，故没有据此猜测新的运行选择算法。现有合并选择仍是兼容算法的待替换部分。

另查看 `../dfo115/fake_server/domain/dungeon_boss_drops.py`：使用历史不同PVF的导出JSON，并将测试boss掉落概率设为10000；仅提供组候选线索，不证明当前115普通爆率算法，也未作为运行期数据源。

本轮对照了 `../90dof/go-server/internal/services/dnfbridge/dungeon_drop_formula.go`、`../ServerS4A21/Server/DfoServer/Game/Dungeon/ClearRewardGenerator.cs` 及 `GameWorld/ClearRewardDefinitionCatalog.cs`、`../usdof/server_proto/game/drop/hell_drop_config.py` 和 `game/dungeon/clear_reward.py`。参考端确认掉落与翻牌有独立生成链，但彼此版本也有差异，不能作为115官方算法标准：S4A21翻牌 profile 按三列推进，而当前115 PVF是四列；usdof保留第四列未知语义，Hell概率分母用1001、稀有度七列，而当前115有行数头和九列稀有度。usdof金币将两列读作max/min，当前Go兼容策略读作基数/浮动百分比，两种说法尚不能合并为原生事实。

已实施：

- 普通地图已声明掉落组时，概率未命中、空组或缺组不再返回通用物品结果；保留通用金币与其它独立奖励。组内选择推进本次死亡共享随机流。历史未携带组投影的目录保留兼容结果。
- 从源 `dgn_hell` 分类和 `rewardboostinfo` 奖励表声明自动识别本轮排除的 Abyss/调律范围，维持其组选择、空结果回退、随机序列和翻牌金币行为；没有新增硬编码副本清单或玩法开关。
- 普通翻牌金币按实际副本难度选择既有兼容策略档位；不将其误称为115完整金币公式。
- 冻结奖单领取接入现有 `inventory.Awarder`，能发放装备与堆叠物，仍在同一角色事件事务内提交奖单和存档。纯状态回归覆盖装备耐久、期限、未知存档字段保留及后续奖项失败时无部分结果发布。普通奖单生成路径仍不填 `Items`，所以**尚未修复普通翻牌出物品**；付费翻牌也未启用。
- 当前PVF直读导入新增clearreward和hellparty原始支持表；Hell史诗表单独类型化投影，保留 `(area,list type)` 及逐项权重，不并入普通或调律池。实际当前源核对7区域/63列表，缺失区域不回落通用史诗池。

115权威 IDB `def2fcf1`：`145857050` 从 XORSTR `14A8D1E30` 加载 `Etc/HellDropEpicItemTable.etc`，调用 `1473E9E00`。后者读取 `14B2720A0/C0/E8` 三个标签（drop area、结束area、drop epic item list），以区域和列表类型建键，将template/weight对累计成权重边界树。此函数已命名 `HellEpicTable_ReadAreaWeightedLists`，添加证据注释、保存权威IDB并更新 `analysis/dumps/idb_funcs.tsv`。这一证据证明池结构和加权读取，**不证明副本到区域映射或专属掉落触发公式**。NOTI35卡片行在 `1452A74F0..1452A7519` 消费两个u32和21字节，`14576EE90`只是本地物品对象初始化；`1452A7869`传给结果UI。未因此给未知metadata字段赋业务意义。

验证：新增普通组/地图边界、死亡重试、Abyss范围、翻牌装备发放、翻牌难度、当前原始Hell史诗分区回归通过。旧普通源ownership测试显式限定为通用Roll，不再假定声明空地图池必然保留通用物品；地图语义由新的死亡链测试覆盖。最终 `go test ./...` 中catalog、loot、dungeon、protocol、inventory、storage等通过，只有cmd/wireprobe的3个既有审计测试失败：`TestAdventureAuditProvenanceAllowanceIsNarrow`、`TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback`、`TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader`；用HEAD源码overlay独立复现相同失败。最终 `go vet ./...`、`git diff --check`通过。完整输出与overlay只放在 `.tmp/drop-audit-20261002/`。

独立候选：`.tmp/drop-audit-20261002/wireprobe-drop-audit.exe`，SHA256 `f41beefd706e681f134255472601db569f8a48a983eef815623ec0c0f8538429`。同目录 `pvf-drop-audit.json` 从现行默认profile复制，仅指定候选程序；只读profile校验通过，13个必需路径存在、19项设置。用户可双击同目录 `启动验证.cmd`，它调用根入口和显式repair-profile。默认 `wireprobe-pvf.exe` 与原源码候选仍都是 `2e00530babeb9b6ed4e357efce6a663fefc6c1d9b7da31843e383c0f945e9c7d`，没有覆盖；未提交、未更新confirmed baseline。

下一步需要用户手动进入Trombe副本103的传统Hell Party：进入封印房、先清固定地图怪、击破柱子并杀掉召出的怪，等待几秒后报告完成。读取该轮当前日志，核对动态Hell模板是否登记、实际CMD39实体和拒绝原因；历史日志缺失且之前没有这些死亡报告，不能给固定地图怪套Hell专属率来补偿。普通完整爆率还缺全局control、MOB专属池、地图组生成/Smart等组合条件和装备字典权重；翻牌还缺当前type分支、四列profile辅助值和候选池条件；Hell还缺动态怪归属、波次/奖数、概率分母、区域映射。依照根AGENTS的禁止猜包与用户手动实机规则，未在这些缺口上增加猜测性运行路径。目标保留未完成状态。

## 结论

用户反馈有依据。现行PVF直读保留了`reference90-gold-stack-v1`普通掉落公式，并未完成115版本全局控制、地图池、怪物专属物品与生成权重的联合计算。不能概括为“所有爆率都没用”：怪物等级、rank和地图组率已有部分生效，但链路存在明确缺口和两个可复现的代码缺陷。

## 当前真源与实机证据

- 只读打开当前`server/work/client-build/Script.inner.pvf`，SHA256为`4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`。
- 最近手动会话`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_234806_594272_next37/gateway.err`的23:48:13日志明确为`PVF loot prepared: maximum grade=150 stackable candidates=1022 drop groups=1221 dungeon indexes=281; compatibility formulas unchanged`。
- 同会话日志的普通装备候选为1536个基础白名单+1638个任务追加，共3174行、2794个droppable。全装备索引有424216条，普通掉落池并未覆盖全装备；地图组奖励另走独立路径，不能用2794概括全部掉落。
- 同会话`events.jsonl`在2026-10-02 00:10:07北京时间起已有`drop_rules_pending`，内容为`no rate for kind "normal" (group 0 grade special)`。这只证明当时组分支被执行且该项没有对应normal率，不能单靠它推导官方应掉什么。

## 实际调用链

`cmd/wireprobe/main.go`加载PVF Loot，同时仍用`loot.LoadRules`加载`configs/drop.current36.json`（启动编排设置）或默认`drop.compat90.json`。两份策略的普通掉落模型和难度倍率相同。

怪物确认死亡 → `cmd/wireprobe/dungeon_flow.go:monsterDeath` → `internal/loot/session.go:Death`：

1. 先`RollWithBonus`按通用表抽金币/物品。
2. 若副本在`dungeondropinfo.cos`，调用`RollDungeonGroups`按怪物类别和难度抽组；否则读副本`[normal group index]`，以通用表已实际产出的非金币件数为budget抽组。
3. `replaceItemAwards`用组产出替换通用物品，但组产出为空时保留全部通用产出。
4. 按副本exclude标记过滤，之后才追加奥德赛货币、调律/征兆/天平、黑鸦等独立奖励并展开包装。

这些特殊奖励另有路径，本轮结论主要针对普通怪物随机掉落，不据此变更已确认特殊奖励池。

## 已确认的缺口与缺陷

### 1. 全局与难度控制只使用一部分数据

`internal/loot/rules.go:Parse`只读取通用表的`[drop prob]`、金币参考表、物品等级窗口、怪物类型倍率和稀有度表。`catalog.ImportLoot`虽然载入`itemdropinfo_control.etc`、`dungeonbossdrop.etc`和`itemdropinfo_monster_hell.etc`，普通Roll没有消费它们。

当前普通类目阈值实际是：

`floor(等级段类目基础概率 × rank类目倍率 × JSON难度倍率 × 已接入的任务加成)`，分母10000。

JSON难度倍率固定为`[1,1.2,1.4,1.6,1.8]`；当前PVF的`[dungeon difficulty drop bonusrate]`却有25格，分成5行，每行5格，包括`[1,1,1,1.4,1.1]`与`[1,1.4,1.6,2.2,2]`。源码没有读取该段，也没有读取`[party member drop bonusrate]`。这些是明确的数据未接入，行的完整官方应用顺序仍需IDA验证。

稀有度表解析36格，却始终只使用`t.Rarity[:9]`，其余3行没有参与普通Roll。怪物rank参与的是类目概率，并没有选择其余稀有度行。不能在未核对reader前直接假定4行就等于4种rank。

### 2. 怪物专属物品配置未接线

`RollWithBonus`只收到monster.Level和monster.Rank，没有monster.Template或怪物脚本。`Session.Death`在普通掉落部分同样没有解析monster LIST/MOB。`catalog.ImportLoot.Pending`仍明确列出`independent/world/explicit monster item pools`。

当前PVF`list/monster.lst`有15165条绑定。只读采样确认：

- 怪物1：`monster/goblin/goblin.mob`，`[item] 1000 50 1047 200 1004 100`。
- 怪物2：`monster/goblin/goblinthrower.mob`，同类`[item]`数据。
- 怪物3：`monster/goblin/seagab.mob`，`[item] 1062 50 1065 100 1047 200`。

源码不存在消费这些MOB物品段的普通掉落路径。这里确认“数据存在但没接入”，不宣称数字50/200的分母或组合规则已确立。任务的`[monster reward item]`是另一条已接入的任务发放线，不能混同于本项。

### 3. 地图组率没有完整决定最终掉落

`RollDungeonGroups`确实使用百万分母下的`RateList[kind][difficulty]`。所以地图率不是完全没生效。问题在`replaceItemAwards`：`len(from)==0`就返回base，无论为空是“正常没有命中”“明确0率”“缺少该怪物类别”“空组”还是“不可读”。于是0率不能阻止通用物品落地。

离线最小复现：给通用表一个100%消耗品命中，并给副本99的normal组配置0率，通过真实`loot.Session.Death`得到1个模板10000的场景掉落。地图组没有命中，但通用物品仍然保留。这证明路径缺陷，不依赖随机实机样本。

另外，组索引分支的budget取的是通用池已经成功选出物品的件数，源池候选空时即使类目概率命中也不计budget；`RollDeclaredGroups`按组号顺序最多各抽1件，先列出的组优先；`Rarity:-1`也未传递通用Roll得到的稀有度过滤。

`DropGroup.CreationRate`/`ArmorRate`和`DungeonDropRateEntry.DropWeaponRate`已被解析但普通组选择未消费。当前1221个组中744个有creation rate、1个有armor rate。`pickFromGroup`直接合并Explicit和Smart按物品权重选取，其两类原生区别尚未建立。

### 4. 通用物品选择忽略生成权重

`catalog.ImportLoot`只用stackable的`[creation rate]>0`作为是否进池的条件，然后统一赋`Weight:1`。`rules.go`在等级/稀有度窗口里按候选数量均匀选择。装备同样按既有白名单池均匀选择，装备字典生成列仍在Pending。

因此当前普通掉落的“掉什么”并非按每个物品的原生生成权重计算。装备窗口空时还会寻找邻近稀有度，消耗品窗口空时可能回退rarity0，会改变最终品级分布。

### 5. 组选物品没有推进调用方随机数状态

`pickFromGroup(rng RNG, ...)`按值传入RNG，内部`Next`只改副本。`RollDeclaredGroups`的调用方rng一直保持初始seed；`RollDungeonGroups`则只有率判定推进了外层rng，选物品没有计入外层状态。

离线复现：两个相同候选组各含100/101、权重均1，输入seed42、budget2；实际产出101×2，NextSeed仍为42。这会造成选择相关性，不等同于每次必掉相同物品，但确实没有保持连续随机流。

## 取证边界及建议顺序

先修有独立代码证据的空结果回退与RNG状态传播，并增加真正区分“未声明来源”和“已声明但本次未命中”的结果语义；同时保留特殊奖励池边界。其余115规则先取证再实现，不直接把所有标签当乘数。

需要继续核对：MOB `[item]`/额外掉落段的reader与数值单位；全局control、类目难度倍率、稀有度各行的应用条件；组creation/armor/weapon率、Explicit与Smart、DGN分段的真实语义。权威IDB已存在的会话`def2fcf1`是`client/DFO.exe.i64`，本轮只查询xref，没有修改/保存数据库：`[drop rate]` xorstr地址`0x1491AFC68`引用到`0x14017AF70`、`0x1472ACF70`、`0x1473D30F0`；这仅是下一步定位线索，不能作为公式闭环证据。

离线取证程序和完整结果在`server/work/dfo-lan/.tmp/drop-audit-20261002/main.go`、`result.txt`，原始PVF支持表只读导出在同目录`pvf/`，均是临时取证资产，不入Git。

## 验证

离线取证程序执行通过，直接读取当前PVF并复现上述两个代码缺陷。Go1.26专项回归通过：`go test ./internal/loot -run 'TestSourceDropPlanOwnershipAndRetry|TestRollDungeonGroups|TestDungeonGroupIndices|TestExtractGroupIndices|TestGradeMatchesRarity|TestMonsterKindName' -count=1`。已有测试通过只说明当前实现符合这些既有断言，不证明缺失的115掉落规则已正确。本轮没有业务/协议改动，未执行全量test/vet，未升级confirmed baseline或提交。

## 追加：普通通关翻牌永远没有物品

用户追加反馈后核对普通翻牌链：`settlement_flow.go:dungeonResult` → `loot.Service.FreezeCards` → `CardPlan` → NOTI35展示 → CMD71选牌 → `PickCard`持久化领取。

### 直接原因

`internal/loot/cards.go:130`普通路径只构造`CardPlan{Run, Source, Model, Gold, Level}`，`Items [8]Award`从未赋值。普通路径没有任何物品概率掷骰或候选池选择。`configs/cards.compat90.json`明确写着`reference90-free-gold-v1`，note为“free gold only until current equipment/card item pool is verified”。这是未实现的奖励生成路径，不是低爆率造成的统计现象。

`settlement_flow.go`会把plan里的Items编码成NOTI35行；`pickFrozenCard`也会领取Items，但普通奖单里它们全是零，因此始终只发金币。现有协议编码器支持物品行，不能将本问题直接归因于客户端展示。黑鸦小队明确走`FreezeBlackPurgatoryCards`，会从专用池生成Items，属于已有特殊路径，不在“普通翻牌永远无物品”的结论内。

同一函数`cards.go:126`调用`CardGold(..., level, 0)`，没有使用`d.Difficulty`；所以普通翻牌金币也总取兼容难度表第0档，而非本次实际难度。兼容策略金币倍率`[1.02,1.38,1.6,1.9,2.0]`也不是对当前PVF完整分类表的直读。

### 当前PVF确实提供独立通关奖励数据

当前源中的`etc/itemdropinfo_clearreward.etc`（index5055830、11545字节、原始条目SHA256 `16a6be8c28a7f2c116254536fee92af520153459be053b6bdd403f31402743c8`）包含：

- `[drop prob]`：default/event/event2/pcroom/tayberrs等分支及等级段数据。
- `[drop kind prob]`、`[drop item type prob]`、`[basis of rarity dicision]`。
- `[dungeon difficulty drop bonusrate]`：当前值`0 600 1000 1400 1200`。
- `[party member drop bonusrate]`按dungeon type分组。
- `[dungeon difficulty gold drop bonusrate]`按dungeon type分组，type=-1的一行为`1.02 1.30 1.40 1.60 2.00`。
- `[reward item rate per map max count]`：`1 2000 2 3000 3 4000 4 5000 5 6000`。
- 金牌成本、生成率、空牌补偿物品等其他段。

另有`etc/itemdropinfo_togclearreward.etc`独立表。普通Loot导入清单没有载入这两张表，普通翻牌路径也没有消费者。可以确认通关翻牌有自己的概率/奖励参数，不能直接以怪物Roll代替；这些值的分母、类别、加算/乘算顺序以及是否再受全局控制影响，仍需权威reader和使用逻辑确认。上述段名与数值只作原始取证，不当作已闭环公式。

### 实机日志交叉核对

从最近手动会话的12条`dungeon_clear_reward`按现行NOTI35原生结构解码：第一组偏移131，8个count，奖励行29字节（template32+amount32+metadata21）。12条全部只有group0的一行template0金币，没有非零模板物品，金额依次为`32,32,31,3,5,8,12,12,14,17,20,18`。与普通奖单只生成金币的源码行为一致。

### 后续取证与验证边界

需连同普通掉落一并规划，但分别接入怪物掉落表与clearreward表，保留通关冻结奖单、防重复领取和既有存档。不能为了让翻牌“出物品”临时调用普通怪物Roll或硬编码赠送。

权威IDB只读xref线索：`[result reward prob number]`（`0x14B301A80`）与`[result reward prob]`（`0x14B301AC0`）都引用到`sub_147812210`；`[exclude clearreward drop]`（`0x14B284540`）引用到`sub_147444B30`。本轮尝试反编译前者超时（120秒），没有获得可用公式证据，没有改写或保存IDB。注意result reward段不因名字就被认定为普通翻牌表reader。

专项`go test ./internal/loot ./internal/game/protocol -run 'TestCardGold|Test.*ClearReward|Test.*CardSelection|Test.*CardLayout' -count=1`通过。支持表只读导出位于`server/work/dfo-lan/.tmp/card-audit-20261002/`，不入Git。本次追加仍仅调查，未修改运行逻辑、资源、profile或存档。

## 追加：传统Hell Party专属爆率（不含调律Abyss）

用户明确指传统Hell Party：普通副本选择`CMD16 Mode=1`、进入封印房并击破柱子后战斗的玩法。这里不使用调律之边界的Attunement/征兆/天平奖励来代替它，也不把`dungeondropinfo.cos`里几个`dgn_hell`条目当作传统Hell Party已完成的证据。

### 当前数据与代码结论

1. `catalog.ImportLoot`会载入`etc/itemdropinfo_monster_hell.etc`，但`loot.Parse`仅从`itemdropinfo_monseter.etc`读取普通概率、rank倍率和稀有度。`Session.Death`没有按`HellPosition`/Hell怪身份切换专属表的分支。因此“hell表已导入”不等于专属爆率生效。相关源码和测试名字混用Abyss/Hell，不能根据命名判定覆盖范围。
2. 当前源Hell专属表的`[dungeon difficulty drop prob]`数据为`1 44 0 0 0 0 0 45 200 335 875 1000 1400 1000`。另有`[basis of rarity dicision]`、`[item drop ref table]`、`[item drop rarity control]`。其难度数值与普通表不同，目前完全没有进入普通Roll；分母、触发对象及选取条件尚未闭环，不将335擅自换算为3.35%。原始条目SHA256为`fe57c5e428032fdd0854bb484ea48610959f906d6bebd1fab0ba34d3aaa356da`。
3. 当前源`etc/helldropepicitemtable.etc`存在独立史诗池（index5055610、20220字节、SHA256 `cb9141008854193d526e6aa630a579992cbbec3286bad0a061146e7e97b42514`），原始表含7个`[drop area]`块、63个`[drop epic item list]`块。普通掉落导入与发放路径没有读取该文件。区域映射、列表首格以及权重应用条件还需要核对，不能用全量装备表或调律池代替。
4. `etc/hellparty.etc`（index5055611、SHA256 `83402cb54ba58c6af1239ffb00ed369188881d136b89939add4aa0da76f85b4a`）保存基础值、难度/队伍参数、字母档位和怪物分组。服务端没有将其解析为Hell Party波次/奖励运行模型。

### 缺口不止概率

当前`dungeon.Select`的Mode1路径只允许已验证的副本103（Trombe），把DGN的seal map加入房间路线并记录`HellPosition`，供NOTI28展示；地图被导入并不代表柱子动态召怪已成为服务端拥有的奖励对象。普通死亡链只消费`d.Monsters`中已登记的怪物，未见Hell专属波次/死亡身份登记与奖励分支。

历史`docs/protocol/normal-hell-party-loot-20260927.md`记录用户曾确认柱子和召怪可见，但当时只有固定地图怪的8条CMD39，召出的怪物未形成对应死亡回报。原会话日志当前路径已不存在，本轮未将历史文档提升为最新实机确认；当前仍缺“服务端登记的Hell怪实体/模板 → 用户手动击杀 → CMD39死亡报告 → 专属奖励”的闭环证据。

故修复需要按顺序确认柱子波次与服务端怪物归属、专属表触发及难度/稀有度计算、drop-area史诗候选池映射，再实现服务端发奖。不能只调普通概率，也不能给封印房所有固定怪物套Hell爆率。本轮无新增运行路径尝试，历史入口attempt1/3不增加。

当前源支持表只读导出在`server/work/dfo-lan/.tmp/hell-party-audit-20261002/pvf/`。本次仍只调查，未更改客户端、服务端二进制、profile或玩家存档。

专项`go test ./internal/dungeon ./internal/catalog -run 'TestCapturedTrombeHellSelectionUsesSourceSealRoom|TestAbyssHellDropTableIsImported' -count=1`通过，只验证既有入口选择和Hell表导入，不能证明动态Hell怪物与专属奖励已闭环。未改运行逻辑，未执行全量test/vet或升级confirmed baseline。
