# 任务临时 NPC 的统一判定调研（2026-09-29）

结论：可以系统性减少逐任务修补。需要把“角色当前城镇阶段地图”和“这个角色的 NPC 显隐状态”统一解析，再由点击 Quest、靠近 NPC 等入口共同使用。只扩大 NPC 白名单、延长前置链或合并全部阶段地图，都不足以解决这类问题。

目前已从静态调研推进到 Go 解析核心、离线重放入口及可选择开启的对照日志，详见§8。候选程序已编译；尚未切换现有任务门禁，没有修改数据库结构或角色状态，也没有启动或操作客户端。A Lull 的已确认修复保留在 `eb0a18a`；下述其它任务是源覆盖候选，不是实机确认故障。

## 1. 本次问题的完整来源

任务12911（A Lull）要求会见100000670，地点80/0。它自己的源脚本没有 `[npc visibility]`、没有 `[go guide]`；普通 Chest Town 地图没有该 NPC。

`town/chesttown.twn` 明确提供两条阶段条件：

| 条件 | phase Index | 80/0 阶段地图 |
| --- | ---: | --- |
| 完成12384 | 0 | Destroyed_ChestTown_Main.map |
| 完成12394 | 1 | Destroyed_ChestTown_Main.map |

两个阶段引用同一个地图路径，不能据路径去重后丢掉阶段序号。该地图含100000670，坐标495/182。

NPC100000670同时属于 `etc/hiddennpc.etc` 的默认隐藏名单。它的显隐来自跨章节的任务规则：

| 任务 | 条件 | 对100000670的效果 |
| --- | --- | --- |
| 12384 | clear | show |
| 12394 | clear | hide |
| 12398 | clear | show |
| 12401 | clear | show |

12911的完整源前置图包含20个祖先任务，但不包含上述4个任务。因此，沿当前任务的前置链查显隐，即使不限制深度，也会遗漏这个来源。

首轮只读 PostgreSQL 核对角色11：12384、12911、12930均为completed/progress0；12394/12398/12401没有返回存档行。12384的完成状态与源文件中的地图阶段及show规则一致。续查已定位完成列表和触发列表的静态重放及批量结算入口；完整登录时序尚无新增动态证据，不能仅据缺失存档行重建所有历史事件。

上一轮实机已确认：CMD33推进12911目标，CMD34完成任务、结算奖励并刷新列表。这个确认仅证明此前的窄修复有效，不证明通用显隐解析器已实现。

## 2. 当前服务端的边界

- 当前运行的旧world导出含各阶段的NPC、地图路径及哈希，但 `PhaseNPC` 没有阶段序号及导入地图到根阶段的归属；本次新可选`PhaseMaps`见§8。原始town脚本和area.definition仍保留，信息可以重新解析，不能声称整个配置已丢失阶段定义。
- `internal/world/npc.go` 的 `HasNPC/NPCPosition` 只查基础地图及其imports；`HasPhaseNPC` 查全部候选阶段；`PhaseNPCPosition` 要求所有候选阶段坐标一致。这些函数都不能确定当前阶段是否生效。
- `cmd/wireprobe/quest_flow.go` 使用任务自身/前置链显隐、引导和少量已确认特例。现有递归已经遍历每个前置ID，但限制深度16，且没有读取角色完整显隐状态。不能把问题归因于“只遍历第一个前置ID”。
- `internal/quest/proximity.go` 中普通SingleMeetNPC使用基础地图定位；ReachNPC才有阶段位置备用路径。点击与靠近的证据来源不完全一致。
- subtype1远程会见、communication和特殊目标有独立语义；统一地图解析器不能把这些入口重新强制成当前地图会见。

## 3. 当前115客户端静态证据

在既有权威IDB会话6c446c4b中读取，不直接重新打开数据库。以下地址均重新核对当前反编译结果；已在IDB命名，并同步 `analysis/dumps/idb_funcs.tsv`。旧文档中的其它地址不作为当前闭环依据。

全局实例另已确认、命名并保存：14E634408为g_QuestCatalog115，14E683D40为g_QuestManager115，同步 `analysis/dumps/idb_globals.tsv`。证据页保留读取当时的名称，旧qword名称与这两个地址对应。

| 地址 / 名称 | 已确认行为 |
| --- | --- |
| 1453281A0 / NPCVisibility_ReadHiddenNPCConfig | 解析hidden NPC整数列表和visibility quest sequence三整数记录 |
| 146D0F980 / NPCVisibility_LoadHiddenNPCConfig | 加载Etc/hiddenNpc.etc，调用隐藏入口并向任务管理器注册sequence |
| 1479FF000 / TownPhaseCondition_KindFromTag | 条件类型0近期通关副本、1区域移动、2完成任务、3事件、4城镇区域事件、5事件活动中；6未知 |
| 1479FF2C0 / TownScript_ReadDefinition | 读取Town/资源；phase Index写入条件树节点+56；multi condition构建8字节条件记录 |
| 146CF8C80 | 验证阶段条件组合，写入manager+560按town保存的条件向量及+576当前向量 |
| 146D06B80 | type2检查144F54990的任务集合成员，并要求当前phaseIndex小于目标phaseIndex；不是简单任选一条已完成任务 |
| 146D02440 / TownMap_ResolvePhaseResource | 用条件向量查definition+424得到阶段序号，再查definition+408的(phaseIndex<<16)\|area资源；失败退回definition+360基础资源 |
| 146D03A30 | 按town/area/phase缓存并实例化城镇地图；诊断字符串明确区分CurrentPhaseShiftKey和lastPhaseShiftKey |
| 144F38B60 | 对任务NPC显隐块执行show/hide，并调用146CF6CD0记录该任务的NPC效果 |
| 146CF6CD0 / NPCVisibility_AppendQuestEffect | manager+480以NPC为键，保留NPC/quest/showFlag逻辑记录；不是只保存当前任务的布尔值 |
| 146D04EF0 / 146D14000 | 分别请求hide/show，写逻辑show状态0/1；hide在manager+1064存在非空任务显示覆盖时仍显示实体；极性经隐藏名单加载及源show1消费双向确认 |
| 146D00BD0 / NPCVisibility_ResolveQuestEffectBatch | 按任务单元parent key及单元内sequence比较效果；严格大于才替换；应用后清空批次 |
| 144F4A870 / NPCVisibility_FindSequenceFallback | 查询questManager+1528备用顺序记录；只在原生任务单元不存在时用于效果比较 |
| 147661070 / QuestUnit_SequenceForQuest | 遍历unit+112树，匹配node+40任务ID并返回node+32顺序；不存在返回-1 |
| 14519AE00 / QuestCatalog_FindConnectUnit | 在questCatalog+72按parent key查任务单元 |
| 147663660 / QuestUnit_ReadConnectQuestList | 读取n_Quest/connectQuestList.etc；parent key写unit+4；普通sequence从1递增 |
| 144F3A780 / QuestNPCVisibility_ApplyCompletedQuest | 插入已完成任务集合，应用condition=1显隐块并登记效果 |
| 1452C9B50 / Noti_ClearQuestList_ApplyNPCVisibility | 当前NOTI342完成列表重建和效果结算；不是新构造的封包 |
| 1452DE670 / Noti_QuestTriggerList_ApplyNPCVisibility | 当前NOTI291任务触发列表重建和效果结算；不是新构造的封包 |

阶段资源缓存键为 `(town<<16) | uint8(area) | ((phaseIndex+1)<<8)`；基础资源用 `(town<<16)|area`。这与简单合并phase_npcs不同。

全局sequence的源记录为 `(6356,5962,1)`、`(12372,12361,1)`、`(12373,12361,2)`、`(12374,12361,3)`。三列是目标任务、参考任务、顺序偏移；第一列不是NPC ID。144F542A0查参考任务的原生单元及sequence，叠加偏移后登记备用记录。146D00BD0只在目标任务的原生单元查询失败时使用备用记录；不能把它当成无条件覆盖，也不能用最大的任务ID作为优先级。

已保存反编译证据页于 `runtime/npc-presence-research-20260929/ida/`。部分大函数仅保存本轮实际读取的分页，不宣称保存了完整反编译。

### 3.1 续查：显隐冲突和重放已确认的边界

146D00BD0对每个NPC按原有插入顺序扫描本批效果：

1. 用效果里的quest ID查原生任务metadata；从metadata+64取任务单元键。metadata缺失则跳过该候选。
2. 原生单元存在，优先级为 `(unit.parent_key, unit.sequence_for_quest)`。单元缺失才查询全局备用表；两者都没有则跳过候选。
3. 从 `(0,0)` 开始，按有符号整数进行严格字典序比较。相同优先级保留先插入的效果；不能先去重成“最后一条覆盖”。sequence查不到返回-1，不能擅自转换成无符号最大值。
4. 有候选胜出才应用show/hide；扫描完全部NPC后清空manager+480效果树。这里保存的是临时批次，而非完整历史。

1452F9420的当前注册表把1452C9B50关联到NOTI342（CLEAR_QUEST_LIST），把1452DE670关联到NOTI291（QUEST_TRIGGER_LIST），与当前opcode dump一致。342先清manager+1176保护集合，再重建完成状态，在0..39999任务索引范围重放完成效果；同一循环还按当前已接任务的进度重新应用accept/clearable效果，最后统一结算。291调用144F38B60应用收到的触发列表，再结算。145261B40和14527AA60已由1452A1F90注册表确认分别为CMD31接取、CMD34完成返回处理；二者末尾也调用结算。

144F38B60还包含进度非零时处理condition=0、为零时处理condition=2，以及按块+32标志登记相反显隐效果的路径。条件2已在原生tag解析器1476608F0确认是clearable（目标达成可提交）；此前的exposed假设排除。特殊任务组路径可能改写进度或替换任务ID，不能把普通任务规则无条件套用于所有分支。执行过程中存在先直接show/hide、保护集合及其它NPC服务副作用，所以**只取所有已完成任务的最大优先级，不足以重建全部显隐状态**。

顺序源由14519F130加载，XOR字符串14A777B20对应 `n_Quest/connectQuestList.etc`。147663660的普通 `[sequence]` 分支（14766550f）对每个两整数行从1分配递增序号，第一整数为quest ID；第二整数另行保存，不能当作该优先级。`[appoint sequence]` 是独立显式顺序分支，本次源文件未使用，审计工具遇到它保留unresolved。

NPC100000670的源顺序候选为：12384→(37,3)，12394→(37,9)，12398→(37,13)，12401→(37,15)。12911自身是(46,29)，但没有该NPC的显隐效果，不能因为它的单元更大就推导其会显示NPC。

本轮补齐metadata归属构造：14519E760先加载源与任务索引，再调用1451A0A30遍历catalog+72单元表；1451A0710逐单元遍历sequence树，以其中的quest ID查已有metadata，在1451A0839将unit+4的parent key写入metadata+64。源序列到显隐优先级消费的连接已确认。此路径没有为缺失metadata创建新记录；同一任务若属于多个单元，最终归属还受原生遍历顺序影响。当前111单元/1898任务没有重复归属；审计仍保留候选数组，不据此输出角色当前可见或允许交互。

### 3.2 续查：事件和阶段恢复仍有输入缺口

146CF9820分派阶段条件检查：type0/2/3/5走通用条件扫描，type4另查当前town/area事件键；type1不能简单复用这一入口。146CFA2B0重置当前条件向量为(value=-1,kind=6)，清按town阶段集合及+1248/+1264事件表。146D04480从+1248查询town事件值，缺项返回0；这仅证明getter行为，不证明服务端事件输入可以默认填写0。

1456927F0在活动状态更新、当前场景为城镇时，以type5调用146CF9820，阶段变动后重新解析资源并更新地图。近期通关、区域移动、事件表写入和重登恢复的完整来源尚未闭环。对这些条件应输出unknown及缺少的输入，不合并所有阶段NPC作为当前可交互实体。

另定位146D09203..146D09215：146D08B60调用144CEFE60读取全局14E66C090对象的+696字段，仅值不为-1时，以该值及type0调用146CF9820。144CEFE60自身只是四行偏移getter，已保守命名Object_GetUInt32At696；不能仅凭这个getter给所有调用对象赋予同一类型。

续查找到一个写入候选：144CE1670由144CD5238..144CD5244通过通知注册链登记为NOTI571（RAID_DUNGEON_PARTICIPATION_INFO）。它读取u8行数，每行u32键、u8状态、u8附属列表数及该数量的u16项，状态3在144CE186D将该键写入绑定对象+696。注册helper14599D650→1459A4090使用原生NOTI名称表14EF334B0，已核对方向。同一getter另有141EB0E10向它传14E683C30的调用。因此，**NOTI571的绑定对象与地图阶段入口传入的14E66C090是否同一实例，尚未闭环**；该候选不等于“普通副本通关就是阶段输入”，也没有新增571封包。本轮读过该通知handler全部822行，未将其它大函数视为已完整读取。

事件输入不再只有getter线索：146D135C0将town→event写入manager+1248；146D135F0将(town,area)→event写入+1264，146D044D0缺键返回-1。已读141C0D900/141C0E380，前者在对应场景退出/重置路径写0，后者从141C0E730取得模式状态值后写表，并在值改变且虚函数谓词通过时调用146D13970更新地图。141C0E730从该活动对象的原生状态字段生成0..3值；其上游通知和角色重登恢复仍缺证据，不能把这些数值通用于全部活动。

146D135F0当前仅一处普通代码调用144A6EC20；该路径仅对活动键194，将阶段状态分成0/1/2/3并写当前town/area，必要时以(value=-1,kind=6)重置该town条件。该局部规则不代表所有townarea事件。146D02C70、146CB2D90还确认按completed→event→activity的分组扫描与按town恢复向量；不是将所有匹配条件按文件顺序任选最后一条。

本次对type4通用谓词追加原生汇编核对：146D06C96把ebx条件值写入局部键低32位，146D06C9F把常量4写入高32位，再经1416DBCA0访问manager+1264并比较返回节点+36。146CF9820的type4分支还先查当前town/area键。两次访问之间的关系尚未闭环，不能把源条件简化成“当前区域event等于源value”，也不能直接把活动194的writer当作完整输入定义。证据在phase-area-event-predicate-assembly.json；西海岸继续输出unknown。

### 3.3 本轮补齐：取消和重登的静态路径

显隐块的实际条件表为accept=0、clear=1、clearable=2、clearing=3、未知=4。show为(show=1,protection=0)，hide为(0,0)，delete为(0,1)，不能把delete折叠成普通hide。1476549D0将块+32的revert默认设为1；147674E7D..147674F28解析[revert] true/false。

另一个独立开关是完整任务脚本+2793：147657AB3按小端word0x0100将它初始化为1，14766D67B遇到拼写原样的[npc visivility not revert]将它设为0。单块开关和整项任务取消开关不能合并。

| 路径 | 原生行为及已确认边界 |
| --- | --- |
| CMD31接取返回 | 145261B40读取任务/剩余进度，交144F38B60应用，末尾146D00BD0结算 |
| CMD32取消返回 | 14527DD70先从已接集合移除任务；仅当script+2793=1时，以a4=0调用144F3B900反转condition0和2；每块还需revert=1。callback末尾没有直接调用批量结算 |
| CMD34完成返回 | 14527AA60处理完成与阶段更新，末尾146D00BD0结算；后续服务端还会发送291/342刷新 |
| NOTI342完成列表 | 重放clear，同时144F57E20检查已接非零进度后应用accept；144F54BA0检查已接零进度后应用clearable。两者都可能与clear进入同一批效果 |
| CMD4选角色返回 | 14525ED50把14525A120注册为CMD4。14525A770..14525A815读取30个固定任务槽及溢出槽，分别把quest/剩余进度交144F38B60应用 |

当前服务端的保存/重建路径亦已核对：main.go的选角流程通过Service.Active读取accepted任务，写入SelectProbeState.ActiveQuests；entry_flow.go进城后发送342恢复completed任务。普通接取发送31返回，靠近目标推进后发送291；取消当前发送32返回；finishQuest顺序为34返回（实际首次完成）、291、342。不能把这些独立批次任意合并或交换。

已有native_quest_triggers.json提供3145的进度1/0读取向量，既有说明明确game/map/UI调用被stub：这里只用于核对字段及输入值，不能当作NPC显隐或完整重登实机验收。本轮没有操作客户端，选角到地图恢复的完整动态顺序仍待用户手动验证。

### 3.4 续查：条件NPC源行与阶段槽投影

此前5处五字段解析失败均包含[visible on dungeon if quest clear]。原生地图解析器1471C2FD0在1471D38FE识别该字符串14B22E5E0，写逻辑28字节placement的+9=1，再读可选数字或方向；1471D3978识别独立的[visible on dungeon clear]（14B22E630）并写+8=1。方向在+12，x/y/z在+16/+20/+24，1471E4790按28字节步长追加到地图定义+944向量。+4的可选数字及消费者后续已在§3.5闭环；当前导出的5处使用标记加方向形式。

审计新增变长源行解析，保留标记、原行偏移、地图hash及坐标，未知标记或不完整行停止该段并保留raw tail。presence_condition_resolved仍不表示最终可见；后续schema5另记录native_placement的任务门禁投影，不能仅凭标记断定当前受任务限制。

本次解析13个条件placement（11个NPC、5个area/script组合），恢复3个原“无导出位置”会见目标的候选：13600→100001627（140/2，704/194）；21015→100001762（156/2及157/0，1111/138）；21274→100002029（182/0，308/194）。这证明旧固定五字段投影会漏掉有效源行，不证明3项任务实机必然失败。

阶段源另按town.lst绑定town ID，保留[phase Index]及area的每个资源槽，不按路径去重。当前65条均为单条件：22条completed quest、40条by event、3条depend townarea event。3条by event=0的phase Index=-1是原生选择器明确处理的基础地图回退规则，不是非法配置。审计直接根资源匹配输出候选阶段序号数组；例如12911的同一地图同时对应[0,1]，不能选成一个阶段。阶段imports的根归属仍未由现有phase_npcs完整保存，匹配不到时保留unresolved。

### 3.5 续查：地图NPC实例化门禁已静态闭环

146D03A30先经146D02440选择地图资源，144D24C20取得地图定义，再调用145DE8CC0构建场景。该函数在145DE9594调用145DEBA20（已命名MapNPC_CreateFromSourcePlacements），遍历定义+944/+952的28字节行。145DEBA8D..145DEBAAF原生汇编确认以下规则：

| placement输入 | 该项任务门禁 |
| --- | --- |
| +4有符号任务号小于0 | 跳过任务检查，不调用任务集合谓词 |
| +4非负且+9等于1 | 144F54990检查已完成任务集合 |
| +4非负且+9不等于1 | 144F54570检查已接取任务集合；不检查剩余进度 |

144F54570扫描QuestManager+8/+16的24字节已接取向量；144F54990查询+336的已完成hash。两者的集合身份与既有选角/342恢复、31接取/32取消路径一致。+8独立标记不是上述任务门禁，它在创建后写NPC脚本+4272；本轮没有把它解释成普通城镇“通关就显示”。

解析器还有不能按常规直觉省略的行为：1471C3124清零esi，1471C31CE初始化任务号=-1，1471C31D8/31DF初始化两个标记=0；只在整个地图解析调用的入口初始化一次。完整1471C2FD0函数范围内的局部字段写入检索，加上1471D38C0..1471D3A74原生循环，未见逐行重置。后一行省略任务号或标记时会沿用前面的值；后续[NPC]源段虽清空向量，也沿用这些局部字段。审计保留最近源写入的位置和最后有效段身份。未知源行后不能假装重新得到默认值，后续投影保留unknown。

新增map_placement_gate只读对照函数：已接/已完成集合按需要分别提供，缺失集合返回unknown，显式空集合返回不满足；负数任务号仅表示这项placement任务门禁通过。它不返回“允许交互”，当前阶段、资源创建、全局hidden及任务show/hide仍是独立证据。

全量基础地图源行的这项投影均可解析：13个显式带标记placement的任务号都为-1，实际不会触发该项接取/完成检查。因此“基础地图条件行候选3项”保留为源语法分类，不能称为3项受此门禁限制的任务。后续应主要核对其默认隐藏及跨任务显隐来源。145B5EF80还存在源NPC变体替换，145B5F520需要NPC目录资源且有特殊构造分支；即使placement门禁通过，也不能据此保证实际实例创建和显示。

### 3.6 续查：已完成任务驱动的阶段情景检查

30个仅阶段地图有位置的会见目标集中在6个town。其中5个town的全部源阶段条件都是单一completed quest，可在明确给定初始阶段及已完成任务集合时进行受限投影；另1个包含townarea event，继续保留unknown。

| town | 源文件 | 阶段目标任务数 | 本次情景投影范围 |
| --- | --- | ---: | --- |
| 6 | town/gent.twn | 10 | 单一completed quest |
| 14 | town/slough.twn | 1 | 单一completed quest |
| 22 | town/jelva.twn | 1 | 单一completed quest |
| 54 | town/blackmarket.twn | 3 | 单一completed quest |
| 80 | town/chesttown.twn | 5 | 单一completed quest |
| 40 | town/new_westcoast.twn | 10 | completed quest及3条townarea event，未支持 |

146D06B80的kind2要求任务在已完成集合中且当前phase Index小于目标phase Index；146D02440遍历整个条件树，146CF8C80保存条件向量。因此，仅在上述单条件、无重复条件键且town绑定唯一的范围内，结果等于“指定初始阶段”和“匹配任务的目标阶段”中的最大值。它不按任务ID大小、源文件位置或资源路径去重选择阶段。已知较高阶段不会因完成集合只含较低阶段任务而倒退；缺少初始阶段时不能假装从-1开始。

此前的构造链证据：146CF1CC0初始化阶段/事件容器，并在146CF21DA调用146CFA2B0；已命名TownPhaseManager_Construct。1459A4BD0在1459A4D80创建它并存入owner+280；上一层1459968C0在145998AB2调用该组管理器创建函数。本次读取前两个构造函数的完整反编译，上一层只读取调用附近汇编。146CFA2B0的14D877610数据引用已确认是IPtoStateMap异常处理元数据，不能当作虚表或重登调用。构造链本身不证明选角重置；后续新增的明确选角重置调用见§3.7。schema1单点情景仍要求显式初始状态。

审计schema6新增completed_phase_scope，并提供可选--phase-scenario。输入schema1只接受source、completed_quests、initial_phases；PVF不一致、非法整数/布尔值、未知town键或额外字段拒绝。初始阶段必须显式给定，缺少完成集合与给定空集合不同。事件条件、未知条件、多条件、重复条件归属或town绑定歧义返回unknown。输出只比较目标地图根资源的阶段序号，final_visibility始终为null；区域基础地图强制覆盖、NPC变体/实例创建及显隐重放仍需另证。

例如以下仅是假设输入，不是当前角色存档或重登结论；source应替换成当前审计的PVF checksum：

```json
{"schema_version":1,"source":"<audit.source>","completed_quests":[12384],"initial_phases":{"80":-1}}
```

指定该输入文件后，运行原审计命令并增加--phase-scenario路径，额外生成phase-scenario.json。Chest同一资源对应0/1两个槽时保留全部匹配；输入12384从-1推进到0，输入12394从-1推进到1，但两者都不能单独证明NPC100000670可见。没有给定初始状态时，两者都输出unknown。

### 3.7 续查：成功选角的重置点及完成列表恢复时序

已完整读取CMD4响应14525A120的1567行，成功分支（a2非零）在14525A6CF取得Scene+280的同一管理器，14525A6D7调用146CFA9B0清空+560全部已保存town条件；14525A6EB调用146D04FE0将+576当前条件向量置为单个8字节记录(value=-1,kind=6)。141308BD0的完整4行getter与构造链owner+280一致。失败分支不会执行这两项重置。两项helper本身不清空+1248/+1264事件表，不能由此推出事件初始为0，也不能视作每次换地图都会重置。

重置点之后并非马上可用最终已完成存档计算阶段：同一CMD4成功路径在14525A9C6调用146D104E0，14525A9E9调用146D00A80；后者内部调用146D02440选择当前地图。NOTI342的1452C9B50随后替换已完成集合、重放显隐并结算批次，其完整385行中没有上述两个阶段清空helper。若在新完成集合恢复前已经用未知集合推进过town缓存，后来提供最终完成集合不能证明该缓存自动下降。1459B04B0场景状态转换函数也有146CFA9B0调用，但处于嵌套条件内；本轮不把它泛化成普通地图变化重置。

新增--phase-trace只读输入，投影明确列出的条件缓存操作，不解析封包、不推定当前角色状态。select_phase_reset指14525A6D7/6EB两项helper执行后的瞬间，不能当成整个CMD4结束；它将阶段初始条件置为-1，并使工具中的完成集合知识失效（不是声称helper清空QuestManager）。completed_snapshot显式提供该时刻的完成集合，不清阶段缓存；resolve_town_phase仅对已有单completed范围推算；gap代表尚未建模的调用，清除工具的已知状态。输入仍绑定同一PVF；未知操作、失败ACK、普通换图、非法town/集合拒绝。

示例是假设操作序列，不是当前实机抓取。增加--phase-trace路径生成phase-trace.json：

```json
{"schema_version":1,"source":"<audit.source>","initial_phases":{"80":1},"completed_quests":[12394],"events":[{"kind":"select_phase_reset"},{"kind":"completed_snapshot","quests":[12384]},{"kind":"resolve_town_phase","town":80}]}
```

此序列清除旧角色阶段1，再以新快照推进到0。若删除reset，阶段保持1；若reset后先resolve、再补快照，阶段保持unknown。这些是纯completed范围的离线状态对照，完整原生调用顺序仍需动态验证；final_visibility保持null。

### 3.8 续查：事件查询副作用及任务显示覆盖边界

1416DBCA0的完整130行确认是有符号两整数键查找或插入：节点+28/+32保存键，+36保存值，缺失键会插入值0。146D06B80的kind4构造键(value,4)，在manager+1264调用此helper，然后比较节点值与源value；147A00850..147A00AAC的条件解析分支确认源整数直接进入value。146CF9820的kind4分组还检查实际(town,area)键存在。该路径不能简化成“当前area事件值等于源整数”，也不能忽略查询本身的插入副作用；与146D135F0写入实际(town,area)键的完整关系仍待补证，kind4继续unsupported。

本轮复核发现前轮将146D04EF0/146D14000及实体层两个helper的极性解释反了，前轮“显示抑制”结论撤回。完整hidden-loader在146D0FA55对[hidden npc]调用146D04EF0；完整completed-quest-apply在144F3A877对源show=1调用146D14000；批次结算146D00EBF..F17在胜出show=1时写+1080=1并调用145982020。145978490/145982020的完整函数及所用set插入/删除helper分别确认在实体manager+112插入/删除隐藏NPC号。三条相互独立的原生消费路径一致证明：146D04EF0请求hide，146D14000请求show，+1080保存逻辑show标志，+624为隐藏集合。

因此manager+1064实际保存任务显示覆盖：hide请求先写逻辑show=0，无覆盖时隐藏，有非空覆盖时实体维持显示。146CF6650按NPC键建立唯一任务号向量，节点+32为NPC号、+40/+48/+56为4字节任务号向量，64字节节点。144F38B60在144F394FD以脚本+2320和脚本原生任务号注册，紧接着调用146D14060显示实体；后者不改+1080逻辑show。144F65B10移除任务路径在144F65C6A调用146CFC070释放对应任务号；最后一项释放后按已有+1080记录恢复show/hide并删除覆盖节点。若没有逻辑状态条目，该释放函数不自行补造show。

源字段已闭环：1476577D9初始化+2320=-1；14766927E的case585与XOR字符串14B2C1938对应[visible npc]，14767743A读取整数并在147677458写+2320。原生消费是上述任务显示覆盖注册。当前2844个任务仅5项有此标签：7957/7959/7961/7963→469，15459→400001354；全部为单整数可解析，关联15个会见目标候选，没有影响本轮30个仅阶段地图有位置的目标。15是源关联覆盖数，不能称为15个故障。

schema8将前轮错误名称更正为npc_show_override_sources及每个会见目标的show_override_sources，保留标签、原值、脚本hash、偏移和消费地址；缺标签不添加源记录，负数不注册，0仍满足原生非负检查，异常源值及重复源段保留unknown。当前已接集合不能直接当作覆盖表快照，特殊任务组的原生任务ID改写、恢复/取消的完整执行顺序仍需核对。schema7报告中的suppression命名和相关解释作废，必须重新生成。

146D00A80读取地图定义+7184的NPC集合，并根据1459AB5E0选择show或hide。1471C6BB9的分支与XOR源标签14B22FB38对应[pvp channel limit npc]；1471CEA67..CEAaf把源整数插入+7184集合。1459AB5E0读取Scene+4528，经14799B0B0/147AB9310检查频道属性3。源身份已闭环为频道限制NPC集合，频道属性3的完整业务含义不自行命名为某种频道。它是地图与频道层的显隐覆盖，不能归入任务完成规则。

## 4. 可重复的全量配置审计

新增 `analysis/tasks/quest-npc-presence-audit.py`，只读生成配置，不连接数据库、不改变游戏状态。以同一PVF哈希绑定world、quest及可选hiddenNpc导出，保留每条源规则、所有前置分支和地图证据；不把候选位置当作允许交互。

本次覆盖2844任务、694城镇区域，其中921个 `[meet npc]`：

| 静态分类 | 数量 | 解释 |
| --- | ---: | --- |
| 基础地图有候选位置 | 579 | 不等于NPC当前可见 |
| 仅阶段地图有候选位置 | 30 | 全部目标都在默认隐藏名单；其中3个属于现有远程会见策略候选 |
| 基础地图标记行有候选位置 | 3 | 13600/21015/21274；已确认placement任务号为-1，仅保留源语法分类 |
| 当前导出配置没有候选位置 | 293 | 包含6个远程策略候选，另需排查动态、替代NPC、资源覆盖等，不是293个已确认故障 |
| 非单一正数目标 | 16 | 需按特殊目标形式单独分析 |

30个阶段目标中，26个没有“针对该目标NPC”的自身显隐规则；6个在自身和完整前置图中均无规则，其中12482为subtype1。剩余5个普通会见任务优先核对：

| 任务 | NPC | 区域 | 已找到的外部显隐来源 |
| --- | --- | --- | --- |
| 12911 | 100000670 | 80/0 | 12384/12394/12398/12401；窄修复已实机确认 |
| 13111 | 100000308 | 6/2 | 13595 clear/show |
| 13116 | 100000308 | 6/2 | 13595 clear/show |
| 13432 | 100000308 | 6/2 | 13595 clear/show |
| 21091 | 100000308 | 6/2 | 13595 clear/show |

其它结构线索：62个area有阶段地图；15组area/path在多个阶段重复；42组area/NPC有不同阶段坐标。旧五字段检查有5处缺口，续查已按原生可选标记语法解析，本次基础NPC源段语法未解析项为0。placement门禁投影已闭环，最终角色可见性仍需阶段和显隐重放。

12990、22353、22461能找到目标显隐祖先，但最近分别为38、127、86层，超过当前16层查找上限。这也是候选覆盖缺口，不能单独认定当前角色会被拒绝。

详细输出：`runtime/npc-presence-research-20260929/audit/audit.json`（所有921任务及证据）、`audit.md`（统计和30个阶段目标列表）。分类总数校验为921。源版本不一致会拒绝审计；未提供全局隐藏源时输出unknown而不是false，这两条边界已验证。

续查增加可选 `--quest-order-export`；schema3增加native_block，记录原生condition编号、show/protection、块revert及整项任务取消开关。schema4增加条件NPC源行、town规则到phase Index/area资源槽的投影及根阶段候选数组；schema5增加native_placement任务号、两项标记、门禁谓词、源写入位置及最后有效NPC段身份，并提供只读集合对照函数。当前顺序源有111个单元、1898个独立任务，没有重复归属或appoint sequence。253个任务包含NPC显隐规则，其中214个找到源单元，另39个仍需核对metadata及其它来源；不能认定这39个任务已损坏。每条显隐规则附parent key、单元内顺序、源序号及hash，额外sequence记录附参考任务来源。未提供顺序源时字段为unknown；重复归属会保留全部候选，不选一个覆盖。

当前schema8另投影5项[visible npc]任务显示覆盖源及15个关联会见目标，附source字段与原生消费证据。--phase-trace支持成功选角内明确重置点、完成集合快照、单completed城镇选择和未建模间隙；它是离线操作序列，不是完整客户端重放器。

本轮实际664个显隐块的上述字段均可解析；3个块显式[revert] false（15460/15464/15497），8个任务关闭取消反转（8645..8652）。这些统计仅是源语义覆盖，不是664项实机验证。

从仓库根目录运行：

```powershell
& tools/python/python.exe analysis/tasks/quest-npc-presence-audit.py --hidden-npc-export server/work/dfo-lan/runtime/npc-presence-research-20260929/hidden-npc --quest-order-export server/work/dfo-lan/runtime/npc-presence-research-20260929/quest-order --output server/work/dfo-lan/runtime/npc-presence-research-20260929/audit
```

重新导出全局源时在 `server/work/dfo-lan` 使用现有pvfinspect：

```powershell
$env:GOCACHE = Join-Path (Get-Location) 'runtime/go-cache'
& ../../../tools/go/bin/go.exe run ./cmd/pvfinspect -source ../client-build/Script.inner.pvf -output runtime/npc-presence-research-20260929/hidden-npc -find hiddennpc.etc -files etc/hiddennpc.etc -tokens
& ../../../tools/go/bin/go.exe run ./cmd/pvfinspect -source ../client-build/Script.inner.pvf -output runtime/npc-presence-research-20260929/quest-order -find connectQuestList -files n_quest/connectquestlist.etc -tokens
```

## 5. 推荐的实现次序

1. **补足配置投影**：阶段规则、phase Index、基础/阶段map与imports归属、全局hidden/备用sequence、connectQuestList的parent key及单元内sequence、所有任务的NPC显隐规则统一入索引，保留源hash和原始条件。原始world/quest配置已经保存大部分源cells，可增量派生；新增字段可选，旧配置仍可读。不需更改角色存档或数据库结构。已有只读审计投影可复用；metadata绑定虽已确认，完整重放及当前阶段证据仍是运行放行前提。
2. **建立角色状态解析器**：读取同一份已接/已完成任务及已确认事件状态，按客户端规则确定当前地图和NPC显隐。结果含present/hidden/absent/unknown、地图、阶段、坐标和依据。缺数据、未闭环条件或冲突返回unknown，不自动开放所有NPC。
3. **先运行对照诊断**：旧逻辑继续决策，新解析器记录差异。拒绝日志明确写目标任务/NPC、town/area、所选phase、来源、决定性任务、缺口原因；不止打印NPC不在地图。
4. **证据闭环后统一入口**：普通Quest点击、普通靠近NPC、会见目标与完成NPC检查复用解析结果。任务归属、accepted状态、目标匹配、config版本检查仍由现有领域层执行；subtype1与communication独立保留。
5. **配置更新时自动审计**：把无位置、阶段归属丢失、条件未知、默认隐藏且无已支持show来源、坐标冲突列为覆盖报告。只让已确定的配置结构错误阻断构建；尚未解析的候选不能一律判死。

## 6. 仍需补齐的证据和人工验收

- 批次冲突、备用sequence、metadata+64绑定、342/291/31/32/34及选角任务恢复的主要静态路径已定位。剩余为特殊任务组的进度/ID改写、clearing条件消费者、NPC服务保护副作用、跨批次与完整登录动态顺序。
- 地图阶段的近期通关对象身份、区域移动、事件状态的上游输入；成功选角清空保存town条件及当前条件已闭环，但事件表和重置后、完成集合恢复前的完整动态顺序仍缺证。当前65条源规则主要涉及completed/event，不能声称支持全部原生条件种类。
- 条件NPC行的源语法、placement+4任务号及+9选择已接/已完成集合已静态闭环。剩余为源NPC变体/特殊构造、+8相关副本生命周期，以及293个无导出位置目标的分类来源；这些不能用“有任意候选位置”代替。
- 本轮没有新的客户端动态命中。后续人工验收至少覆盖跨章节show（12911类）、NPC308外部规则、hide后拒绝、同一NPC跨阶段改坐标、取消任务和重登。由用户操作客户端，读取服务端日志；不自动跑图。

检索排除记录：1475E48E0的default map phase/map phase list伴随storybook/archangel/seven sin标签；147CCCF80伴随dungeon skill level info/hidden named条件。两者不能因phase关键词相同而作为普通城镇阶段选择依据。历史14546260e也不作为本轮当前函数定位依据。

## 7. 资源身份

- inner PVF：`7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`（760530763字节）；与当前world/quest source一致。
- client/Script.pvf与Script.required.pvf：`5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`。
- etc/hiddennpc.etc：`a6b2d2d45a6798a145378462fe9469928f505c61cdf50b625b64a832838c1c4a`，341条隐藏记录/335个独立ID。
- n_quest/connectquestlist.etc：`6573dc67bc6045a9809cd611c6ddbb2671fbf0d6f0a58186954ae2bc9a4f1593`（29460字节），111个单元/1898个独立任务；与同一inner PVF绑定。
- town/chesttown.twn：`6545ed184766b852ac971372db208a11ed4f527f3867da70c67d30fa90179283`。
- world.generated.json当前文件：`be4676943376b1286fcc6016bbc1d70d60d992906bdc77a9e7d4e63026a8e0ae`。
- quests.generated.json当前文件：`1c540704ef5afc9f6dc152531a4f1fac815d79326f58705c7a0a2b932982bb3e`。

验证结果：首轮全量审计及source边界检查通过，`go test ./...`、`go vet ./...`均退出0（测试复用既有缓存）。续查重新生成921任务审计，增加6项检查并全部通过：顺序不依赖任务ID/辅助单元值/文件中的单元位置、缺源保留unknown、重复归属保留歧义、未支持appoint sequence保留unresolved、PVF身份或raw校验失败拒绝、不完整两整数行拒绝。检查入口为 `tools/python/python.exe analysis/tasks/test_quest_npc_presence_audit.py -v`。首轮6个及续查8个确认函数命名均核对并保存到权威IDB，相关字段同步到资料库。续查只改分析工具和资料，没有修改Go运行路径，不重复声称新的实机验收。

本轮新增3项检查，总计9项通过：clearable/clearing区分且未知条件保留unknown、delete保护标志及默认revert、单块revert与整项取消开关独立且非法值保留unknown。全量921任务审计重跑通过；新增8个确认函数命名已查询核对并保存，3个部分结构同步资料库。未改Go运行路径，不重复全量Go测试或要求用户实机。

取证checkpoint：146D08B60完整反编译请求未返回可用结构化代码，未将它计为已读完整函数；改用该函数13KB范围内的原生汇编检索继续取证。1451A1E20与1451A0C40经读取均不是metadata+64的绑定入口，避免后续重复追错。

本次续查：原6c446c4b会话已退出，idb_list确认无剩余会话后，将权威i64复制到忽略目录runtime/npc-presence-research-20260929/idb-work/，通过IDA工具打开工作副本b863f6ef（不运行auto analysis，不直接重开或覆盖权威文件）。新增8项函数命名及3条边界注释已在该副本保存，命名查询核对通过；同步idb_funcs.tsv和2个部分结构。29页加2页证据保存在原runtime/ida目录。共享getter+696对象身份仍保留缺口，名称未强行绑定为副本通关字段。

本次验证：16项源投影检查通过；schema4全量921任务重新生成，分类总和921、条件候选集合恰为13600/21015/21274、5处条件源行全部解析、Chest Town重复路径的0/1槽及独立条件保留、65条阶段规则全部可投影。阶段-1的基础资源规则有专门检查。未改Go运行逻辑、数据库、封包发送或客户端运行文件，不需在本轮要求实机操作。

继续取证checkpoint：新增26页原始取证保存在runtime/ida目录，包括场景构建链、NPC源字段入口初始化/完整函数写入检索、实例化门禁汇编及两个集合谓词。145DE95D0只读取827/2132行，并非本轮完整读过的NPC入口；真正入口为完整154行145DEBA20。1471C1A80是地图定义重置函数，归档名npc-copy只代表初始检索标签，不是复制函数结论。附加6页构造/显隐线索中，144F53FC0根据已接脚本+2048及14519ABB0派生目标判断；14519ABB0存在三参数签名及已完成任务驱动的替换路径，本轮未确认这些参数的源标签或完整行为，不能按归档名protection推导保护语义。未进行C2S尝试。

本次验证：增加6项边界检查，共22项通过；schema5全量921任务审计重跑通过。新检查覆盖已接/已完成集合区别、未提供集合与空集合区别、标记但无任务号及负数任务号、字段跨行继承、重复[NPC]段替换但字段继承、未知前段后不得臆造默认值。13个显式标记行均为负数任务号且placement任务门禁旁路；基础源投影unknown项0，分类总和921。新增4个确认函数命名及2条注释已在b863f6ef工作副本保存并查询核对，更新函数和结构资料；没有覆盖权威i64。本轮未改Go运行路径、数据库或客户端运行文件，暂不需要用户实机。

schema5集成检查另覆盖1786条基础候选源行、全部13个标记行、分类总数921、3个源语法候选集合、Chest重复资源槽、65条阶段规则及缺hidden/order输入的unknown边界；检查通过，摘要保存在runtime/npc-presence-research-20260929/audit/schema5-validation.json。当前源没有可选非负任务号形式，没有“省略标记却继承completed标记”的行；这些两项源语法行为由上述边界用例覆盖，不能声称已在实机命中过。

本次续查验证：增加11项独立情景边界检查，共33项通过；schema6全量921任务审计及CLI情景输入检查通过。集成检查覆盖6个相关town、纯completed范围的20个阶段目标和混合事件范围的10个目标，另验证空完成集合、全匹配完成集合、缺初始阶段及较高缓存4组假设输入。4组输出共88条候选位置，每组final_visibility均为null；缺初始阶段时88条阶段根比较全部unknown。全匹配情景下22条匹配、6条不同、60条因事件输入未知而保留unknown；这些数量是候选位置对照，不是任务成败统计，也没有读取当前角色存档。输入/输出在runtime/npc-presence-research-20260929/audit/scenarios，摘要在schema6-validation.json。

本次生命周期新增构造链及异常元数据证据保存在runtime/ida；146CF1CC0命名及两条范围注释已在b863f6ef工作副本保存，查询核对通过并同步idb_funcs.tsv。没有覆盖权威i64、改动Go运行路径/数据库/封包或替换服务程序，未进行C2S尝试。仍需追查阶段缓存生命周期、type4通用谓词与当前town/area事件键的联系、完整NPC显隐重放；当前不要求用户实机操作。

前轮取证记录（显隐极性解释已由§3.8更正）：成功选角的两个缓存重置helper、type4查询插入副作用、地图频道限制源及[visible npc]注册/释放已取证。新增13项独立边界检查，总计46项通过（0.034秒）；schema7当时重新审计全部921个会见目标；其suppression名称及解释已作废，当前schema8验证替代此结论。6组source绑定的假设trace与CLI入口通过：切角色高阶段清除、未重置保留较高阶段、先resolve后恢复集合保留unknown、显式空集合、未建模gap、5个纯completed town及混合事件town40。全部final_visibility为null；没有读取当前角色存档。输入/输出在runtime/audit/traces，摘要在audit/phase-trace-validation.json（均位于npc-presence-research-20260929下）。

前轮37页取证保存在runtime/ida（24页缓存/事件/频道、13页[visible npc]）；原始代码仍有效，旧归档名中的suppression不代表正确语义。完整CMD4为1567行、场景转换为1913行、插入helper为130行、任务覆盖注册/移除为140/83行。新增9个确认函数名和6条边界注释已保存到b863f6ef工作副本，查询核对通过；同步函数及部分结构资料。没有覆盖权威i64、修改Go运行路径或客户端文件、替换服务程序，C2S尝试数0。下一步重点为特殊任务组/ID改写和完整显隐重放；目前还无需用户实机。


## 8. Go 解析核心与首次实机对照边界

当前实现入口为`internal/npcpresence.Index.ResolveNPC`，组件包括：

- 源投影：同一PVF绑定的world/quest、town条件、各阶段根槽及imports、NPC的原生有符号placement字段、NPC显隐块、[visible npc]显示覆盖来源。所有来源按NPC索引查询，不通过当前任务前置图筛掉跨章节来源。
- 阶段缓存：已闭环的单completed条件；明确成功选角重置点、较高阶段保持、缺完成快照及未建模间隙保留unknown。事件与多条件范围仍不支持，不能以完成集合快照重新初始化这些状态。
- 原生显隐重放：逻辑show、实体hidden集合、任务SHOW覆盖、保护集合1176及有顺序的批次效果分开维护。源NPC位置不替代实例证据。
- 最终组合：`present/hidden/absent/unknown`；同时保留phase/root、placement任务门禁、实例、逻辑show/实体显隐及缺口理由。候选根图与最终所选根图分开；只有当前实例生命周期证明和显隐证明足够时才给出确定实体结果。

源导出新增可选`phase_maps`，保留重复路径的独立槽、完整map和imports、源哈希及未解析项。旧world配置仍可读取；缺该字段时不从旧`phase_npcs`猜所选根。实际重新导出62个区域、171个阶段槽，phase资源读取/导入错误0。产物在`runtime/npc-presence-research-20260929/world-phase-source.generated.json`，逐项比较确认694个区域和顶层字段的既有内容全部保留；当前运行配置未覆盖。

原生保护执行顺序已由完整144F3B900（100行）及146D05450/146CF6930/146D0F120确认：apply时，未保护NPC先请求并追加源动作；delete加入1176保护，其它动作先追加临时show并移除保护；然后再次请求并追加源动作。revert仅在块+32==1时反转动作，不改保护集合。同一任务的等rank效果由146D00BD0保留第一项，因此不能去重。如未保护hide的批次顺序为hide/show/hide；预先保护时为show/hide。unknown保护必须保留批次结果的不确定性。

全量Go源投影初查发现`[/npc]`及type7单元使源块被过度判为未知。已读取1470A2910（240行）、1470A1A80（92行）及147C99AC0/147C99090/142C9C6C0，确认整数返回0/1/2、字符串返回3/4，其它kind在读取循环中跳过；修正type7和闭合标签处理，不把其payload当NPC号。当前2844任务的664块、5项显示覆盖及65阶段规则全部投影通过；源投影缺口0。未包含在已闭环范围的数值形式继续unknown，不按本次源统计推定未来资源都合法。

额外复核：完整146D02C70（437行）在146D03196检查146D06E30返回值；后者只查town基础area资源节点+40 area与+80标志，标志1强制基础资源，不读取1264事件表。节点80对应的源标签尚未确认，不能因导出里缺此字段就默认false。145B5EF80（48行）还可能经源变体调用另一NPC构造，原始NPC身份写实例2816、变体选择写2812；NPC1000有场景类型1拒绝。当前解析器保留最终map与实例知识unknown，不凭placement直接判断存在。已确认名称及部分字段在IDA工作副本和dump资料中同步，未覆盖权威i64。

离线入口为`cmd/npcpresenceaudit`：读取显式source绑定trace，无数据库连接和游戏启动；拒绝未知操作、来源错配及未闭环condition消费者。trace中的final_root、instances和rank属于提供的情景假设，不是实机事实。8组source绑定情景覆盖present、hidden、absent、覆盖维持显示、最后释放恢复hide、源clear/show批次、缺rank及gap，全部结果符合预期，产物和断言在`runtime/npc-presence-research-20260929/go-replay/`。

运行对照入口为`cmd/wireprobe/npc_presence_shadow.go`：`DFO_NPC_PRESENCE_DIAGNOSTICS=1`时，在既有CMD33处理后、发送其响应前记录`quest_npc_presence_shadow`，包括旧结果、目标accepted状态、当前area的分根候选、全部外部显隐来源和知识缺口。只读现有任务存档，失败/版本不符保留unknown；不改封包、数据库或交互结果。可用`DFO_NPC_PRESENCE_WORLD`指定上述同源阶段图。默认关闭，不把server已发送封包伪称为client已消费，也不自动把角色完成集合当成完整显隐历史。

首次实机对照目的：核对候选程序的只读日志、真实NPC显示与Quest交互、重登前后时序；只有完成对照取证后才考虑把现有点击/靠近/完成NPC门禁切到统一结果。请优先用现成角色和任务，不为本阶段修改存档。若只有已完成的12911，则该角色只能验证NPC显示及菜单，不能宣称再验证任务目标/奖励。

候选程序为`bin/wireprobe-handoff-source.exe`；对照启动入口为`runtime/npc-presence-research-20260929/start-shadow-server.cmd`，它继承现有服务端启动方式并只增加诊断环境设置，使用`--source-build --server-only`，不启动客户端。已用相同环境设置执行Python编排的`--server-only --source-build`准备候选服务端：2026-09-29，7001端口启动成功；日志目录为`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_080129_288102_next37`。没有启动客户端。用户手动进游戏可用同目录`start-shadow-client.cmd`，它仅调用既有启动游戏入口的`--client-only`，不另起服务端。

本阶段验证收口记录：32项NPC核心检查、4项入口边界检查及46项Python源审计检查通过；修正type7后全量`go test ./...`、`go vet ./...`及候选编译均退出0。8组离线重放及所有664源块检查通过。本阶段新增/复核原始证据在runtime/ida；新增字段及更正的函数名已查询核对并保存工作副本。当前阶段没有客户端动态命中；进入首次人工对照，不将它标为实机已确认基线，也不提交泛化门禁替换。
