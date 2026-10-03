# Hell Party 柱子与主怪身份缺口（2026-10-03）

前两轮仅取证；第三轮在用户明确批准 S4 兼容规则后接入服务端候选。默认程序、PVF、客户端资源和玩家存档未修改；普通副本已确认的掉落、Odyssey 与调律 Abyss 保持各自路径。第三轮的实机状态与回滚入口见文末。

## 实机证据

用户手动日志目录：`server/work/dfo-lan/runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_005518_681718_next37`。以下时间采用北京时间；`events.jsonl` 时间是 UTC，`client_trace.txt` 也是 UTC 格式的时分秒。

- 01:07:57：角色10发送 CMD16，副本87、难度2、Mode1。
- 01:08:31：进入封印图60051。服务端 NOTI29 只有4条固定怪物记录：4126/61187、4127/61187、4128/56137、4129/61187，等级均62，SourceIndex0..3。没有专属主怪/APC记录。
- 01:08:34：上述4只各有 CMD39 和 NOTI38。4127产生金币1110，其余没有掉落；这些是封印房固定怪物，不能因此把它们当作最终Hell Party奖励主体。
- 01:08:34之后至日志结束，没有新的 CMD39。01:08:44、01:09:51、01:10:31、01:10:39及01:10:49～50，客户端有模板1006的本地 `sendDieMonsterPacket start` 日志，uniqueId依次170、458、630、677、721、723、722、724；没有与它们对应的网络 CMD39。出现函数入口日志不等于报文已经发送。
- 01:09:30的 CMD48，明文 `0c00000000000000`，原生名称是 `ENUM_CMDPACKET_DECREASE_DURABILITY`。它是耐久消耗，不是柱子刷怪触发指令。
- 运行目录中没有发现 Hell Party 主怪的服务端身份登记或 NOTI666发送记录。单独提高掉落概率不能补上缺失的死亡事件。

## 当前PVF原始数据

通过 `catalog.ResolveScript` 直读 `server/work/client-build/Script.inner.pvf`，没有使用导出JSON作内容来源。

- 60051对应 `map/seatrainretaking_r/hell_(6,0)named.map`。
- 柱子 `[special passive object]`：30528，坐标681/303。其 `[hellparty]` 有18条 `(group, weight, order)`：
  - order1：125/25、126/25、127/25、128/25，以及31、32、34、36、37、38、39、40、41各20。
  - order2：33、35、42、43、44各20。
- `etc/hellparty.etc` 中125～128为A组，分别包含1050、1051、1052、1053，entity type0；33、35、42、43、44为A组，包含APC（type1），44还包含怪物56717。
- 1050～1053分别是 `monster/cosmofiend/{braze,vapor,overgrowth,darksteel}.mob`，都有 `[hell monster] 1`。
- 日志中的1006实际是 `monster/skeleton/skeletonhellgatekeeper.mob`，属于守门骷髅，不是上述候选主怪。不能依据1006的本地死亡日志认定主怪已完成有效网络生成或击杀。
- `[difficulty]` 原始数据：`4 A 8 5 100 100 0 B 6 4 0 0 0 C 4 3 0 0 0 D 0 3 0 0 1`。本轮没有把未证明的列名、分母、奖励次数写进运行代码。

## 115权威IDB调用链

复用托管会话 `2e6257e3`，没有直接打开/覆盖IDB。第二轮仅为已确认函数补充分析命名，未修改客户端代码。

1. NOTI29 handler `1452B7100`，每条怪物记录的reader：
   `u16, u32, u16(uniqueId), u32(template), u8(level), u8(rank), u8, u8, u8, u32, u8`。
   `1452B7A6C..1452B7AEE`完整消费22字节。首u16经 `145B0D910` 写record+36；两个延迟字段经构造器写record+24、+28。
2. `145B257F0` 在 `145B25C30` 对record+24==1跳过普通即时创建。当前服务端 `protocol.StartMap` 仍把该字段编码为0，首u16也恒0；没有Hell专属延迟记录模型。
3. 柱子关联函数 `145B26960` case5：查找record+24==1且record+28==-1的记录，区分rank0..3与5..8，把模板、等级、uniqueId及record+36挂入柱子队列。`147574AF0`确认rank<=3为怪物；`147574AB0`确认rank5..8为APC。
4. 基类构造器 `146144DE0` 的vtable为 `14AA30C68`。vtable+5072/5088分别是 `146146CA0` / `146146A40`，保存20字节的怪物/APC队列条目。队列中+16承载波次序号。
5. Hell柱子的 `1456E6EE0` 比较柱子+12168与队列条目+16，只生成当前序号条目，之后推进序号并截断到9。它调用 `1456E8F60`（怪物）或 `1456E8C80`（APC），后续分别进入 `145D883C0` / `145D880C0` 原生创建链。
6. 以上静态链证明：正常房间的固定怪物记录与柱子主怪的延迟记录是两套生成路径。不能靠房间全部固定怪物死亡时发最终装备奖励代替主怪登记。
7. NOTI666 handler `1452AFF70`读取 `u32 count + count*(u32,u32)`，经 `145B4BA70`记录正值映射；`145B4BAA0`按首字段创建/准备AIC。参考端把第二字段称为level，但本轮没有闭环该字段的115实际赋值语义；不能把此包当成替代NOTI29的实体ID登记包。
8. 模板1006对应日志字符串VA `14A96F9D0`，唯一xref是 `145DC86D0`。该函数在入口日志之后仍有多条早退及条件分支；CMD39 writer在 `145DC9BDA`，处于后续条件内。因此入口日志与网络日志缺失不矛盾。

## 第二轮：Hell Party 实际交互职责（2026-10-03）

本节继续沿115 IDB调用链调查，不改运行路径。结论是服务端预先分配专属实体，客户端柱子控制器推进阶段并通过原生UDP/world-action层生成实体；有效死亡再进入游戏服务端CMD39。现有TCP会话日志不能代替UDP/world-action观测。

| 阶段 | 方向 / 通道 | 已确认消费与职责 |
| --- | --- | --- |
| 选择Hell副本 | C2S CMD16 | 本次副本87、难度2、Mode1；服务端处理入场与封印路线。 |
| 下发副本信息 | S2C NOTI28 | 客户端记录Hell房间XY，区别于Boss XY；`1452AC840`写dungeon+6468/+6476。 |
| 下发房间与专属名单 | S2C NOTI29 | 普通行立即创建；hidden=1、selector=-1的行挂入Hell柱子的怪物/APC队列，首u16决定队列order，uniqueId来自服务端。当前服务端没有这些专属隐藏行。 |
| 准备APC资源 | S2C NOTI666 | `1452BAD10`在`1452BB69D`注册666→`1452AFF70`；reader为count-u32及(template-u32, level-u32)对，进入AIC加载缓存。此包不携带uniqueId，不是实体登记包。 |
| 打柱子、推进阶段 | 客户端柱子控制器 | 命中累积与动画状态推进；不是已经证明的专属TCP启动CMD。115原生opcode表没有`HELLPARTY_START`名称，本次也没有CMD85/1061。 |
| 创建主怪 / APC | 原生world-action 22 / 41，类型参数2 | 分别从`145D883C0` / `145D880C0`进入UDP系统；保留隐藏名单中的uniqueId。单人自身目标进入本地队列，远端目标进入UDP发送队列。22/41是此层动作号，不能解释成TCP CMD22/41。 |
| 怪物死亡与掉落 | C2S CMD39 → S2C NOTI38（现有掉落路径） | 死亡发送受实体状态及authority条件约束。服务端需要已登记的uniqueId才能核对模板、波次和奖励；入口日志不等于CMD39已发送。Hell专属奖励条件/次数仍需另行闭环。 |
| 完成标记 | S2C NOTI777 | 注册`1452BB6E2`→`1452AFF30`，handler不读body；调用任务事件37及`144998B50(...,1)`设置角色+1512。此包不含掉落物品列表。何时发需要由服务端完成规则决定，不能凭名称补发。 |
| Hell Party数值 | S2C NOTI207 | 注册`1452BB122`→`1452AFEA0`，读取单u32并写入编码全局`14E684DF8/+4`；角色条件不足时归零。不是波次实体列表或奖励物品列表。 |

### 隐藏名单的115字段闭环

相对于每条22字节NOTI29怪物行：

| 偏移 | 类型 | 消费结果 |
| --- | --- | --- |
| +0 | u16 | order，构造后record+36，柱子队列条目+16。 |
| +2 | u32 | SourceIndex。 |
| +6 | u16 | 服务端uniqueId。 |
| +8 | u32 | template。 |
| +12 / +13 | u8 / u8 | level / rank；rank0..3是怪物，5..8是APC。 |
| +14 | u8 | CreateTrigger，经后续赋值写record+76。 |
| +15 | u8 | hidden布尔值，写record+24。 |
| +16 | i8 | selector，`1452B7BE9`明确`movsx`；0xFF转为-1，写record+28。 |
| +17 | u32 | team。 |
| +21 | u8 | 额外布尔字段，具体内容语义不在本轮命名。 |

`145B26960`case5按hidden=1、selector=-1收取对应记录，通过`146146CA0` / `146146A40`保存在20字节队列中；附着之后经`145B2BF80`移除普通待建记录。队列为模板、等级、uniqueId、类型相关字段、order，不需要每打一次柱子重新向服务端取一批ID。后续取证确认隐藏 APC team 默认为100，仅当 SourceIndex 小于地图 AIC 表长度时才读取该表 team；候选使用已证明的10000哨兵，避免误指向固定 AIC 行，不使用负索引。

### 柱子、守门怪与主怪是不同来源

- `1456E7170`初始化wave=1（柱子+12168），hit stage=0（+12164），命中次数/伤害阈值来自柱子参数8/9（+12264/+12268）。
- `1456E7D40`累积次数（+12156）与伤害（+12160），两者达到阈值才推进stage，stage上限6；相应地选择动画偏移1、2、4、5。该函数内没有游戏服务端TCP writer。
- stage1调用`1456E92B0`，遍历柱子参数生成的模板vector（+12208），创建请求中的uniqueId为0xFFFF，由客户端原生层分配。这是本地守门怪路径，和隐藏主怪行提供的固定uniqueId不同。初始化模板vector来自柱子参数15之后的随机候选段；本轮未直读30528对应OBJ参数，不能将全部vector元素硬编码为1006。
- `1456E8B10`在动画偏移2且NOTI29头部+7写入的`14E6849BD==1`时，以及动画偏移5时，调用`1456E6EE0`。后者只取当前wave的隐藏行，创建后wave++，上限9。头部+7实际取值含义尚未闭环；当前服务端恒0，因此即使补隐藏行，头部条件也是独立缺口。
- `1456E81F0`处理实体移除，检查世界内敌方实体并调整柱子阶段/标记。`1456E8440`也扫描世界中的怪物/APC状态。波次推进包含客户端状态消费，不能简单写成“服务端收齐房间固定怪死亡就发下一波”。

### 原生创建动作的传输闭环

`1456E8F60`把隐藏uniqueId写入怪物请求+44，+49置1；`1456E8C80`把隐藏uniqueId写入APC请求+48，+52置1。正常路径分别调用world-action writer22、41，最终共同进入：

`14665AA60` → `14665F210(qword_14EF418F8, sendSelf=1, target=-1, actor, 3)` → `14707E700`。

- `14707E700`目标槽等于自身槽时，调用`14708A080`复制action缓冲并加入manager+120队列（`14707E772..82D`）。因此单人也可在客户端本地消费创建动作。
- 远端可靠路径经`147081850`→`14708B7D0`，非可靠路径经`147082C10`→`14708C1D0`，进入UDP工作队列；调试字符串`14B200A40`为`UdpClientSystem/UdpManager.cpp`，`14B200B68`为`UdpManager::Send`，`14665F210`也有`makeUdpCheckPacket`字符串`14B0A2B90`。
- 接收大dispatcher`14663EF80`的action41分支`146646E5F`、action22分支`146647BFB`已从jumptable注释确认；分别在`146647883`、`146648881`调用实体绑定`145DE4360`。该大函数Hex-Rays失败，本轮以局部汇编取证，未宣称完整UDP字节布局已闭环。
- 本次只有TCP服务端会话与客户端文本日志，没有可直接核对的UDP创建动作向量。不能把服务端没收到CMD39解释为“柱子完全没有执行创建”，也不能把本地守门怪ID作为已登记的主怪ID。

### NOTI666第二字段的新证据

`145B4BAA0`在`145B4BB54`将映射值`[rbx+20h]`读入EDX，在`145B4BB5E`调用`145C77BA0(actor, EDX)`；后者写actor+101828。原生日志字符串`14A7A3C38`明确为`hell index(%d) level(%d)`。这补齐了上一轮缺失的参数传递，可将第二字段确认为该AIC预加载等级设置；它仍不提供实体ID，也未证明各APC应选哪一级。

### 对参考端的限制

S4参考端用隐藏名单模型及资源预加载，方向与115静态消费吻合；但它把`REQUEST_DISJOINT_ITEM`路由到`HandleSharedDisjointOrHellParty`，无分解商店时调用名为`HELLPARTY_START`的处理函数。这是参考服的兼容分发逻辑，不能据此给115加CMD85柱子触发。它的资源预加载NOTI编号679、完成编号782，也不能直接复制；115对应666、777。

第二轮收敛：可确认“入场/名单 → 客户端柱子状态与UDP创建 → 有效CMD39 → 服务端掉落/完成”的职责链。当前最早缺口是NOTI29专属隐藏行、服务端身份登记及头部模式消费，不是单纯专属爆率。待补齐115模式、PVF组选择/等级及奖励条件后，才能制作一次可验证候选；本轮不要求用户重复实机，不消耗C2S运行尝试。

## 参考端与实现边界

只读参考 `../ServerS4A21/Server/DfoServer/Network/Handlers/Dungeon/DungeonMapHandler.cs` 的 `AppendHellPartyTemplateRows`，它给各波条目首u16=order、延迟flag=1、selector=0xFF；并预先登记这些实体。它的 `BuildHellPartyWaves` 从地图各order内按权重选择兼容difficulty的组。该实现提供查找线索，不能据此认定115的难度映射、等级、奖励次数与表分母完全相同。

本轮根因定位：封印入口已通，但主怪波次的源数据投影、延迟身份下发、owned死亡与专属奖池消费者尚未接入。资源中有主怪与掉落规则，不是“PVF没有内容”。当前只有守门骷髅的本地死亡可见，尚未得到主怪有效CMD39的动态闭环。

前两轮的实施门禁是：补齐115模式/组选择消费条件、等级来源、延迟身份绑定，以及专属奖励主体/次数/区域映射。第三轮已补齐协议消费证据，并获得用户对服务端兼容策略的明确授权；兼容公式不升格为115官方事实。

## 第三轮：兼容规则候选（attempt 1/3，待用户手动实机）

用户明确答复“采用参考端兼容规则，倍率默认1倍”。该授权包括地图波次内按权重选择 A/B 组、怪物使用副本 basis level、APC 使用自身脚本等级、奖励组最后一只实体死亡时 A/B 分别按当前 PVF 的首列抽8/6次，独立倍率默认1倍。

新增115取证：

- `147339B10`（已命名 `HellParty_ImportRulesScript`）由 `145A239BF` 调用，路径字符串 `14A911CA8` 为 `Etc/HellParty.etc`。difficulty 的五列宽度为u8/u16/u8/u8/u8，分别落规则对象+121/+128/+139/+145/+151；组字母 A..E 在 `14733AFxx..B0xx` 导入为 native mode1..5，组节点+116保存mode，模板16×u32、类型16×u8。这证明存储和字母序号，不证明服务端概率公式。
- `1471E3A90` 读取地图 `[hellparty]` 字符串内的 group/weight/order 三元组，order范围1..9；隐藏队列与柱子依此匹配。源波次可以有空档，候选保留原order，不将剩余波次重新编号。
- START_MAP header+7 确由 native reader写 `14E6849BD`；动画offset2在其值1时启动当前波次、offset5启动下一次调用。`14692CA30` 的1/2/3分支使用 `interface/mission/missiontitle.img` 不同帧；缺少对应 NPK 图像，未声称恢复官方UI难度文案。A=1/B=2是获批S4兼容映射；header+8保持0。
- APC `[minimum info]` 的name后第5个整数为脚本等级，与S4 `DungeonActorTemplateProjector` 的投影一致。10627当前值65，怪物1050～1053按副本基准62。所有 `[hell monster]` 标记直接从绑定的 MOB/AIC 读取；1050～1053为1，不作为最终装备奖励组。
- OBJ30528绑定 `passiveobject/monster/cosmofiend/sealedgateofhelldungeon4.obj`，SHA256 `5844737f498edf49e79767068cdb3d2d1a79ea500eccb4172abef890ccd9f154`，原始 `[int data]` 为 `1 1 1 0 0 30 1 80 60 40 20 500 20 20 4 1053`。该段与已有1006入口日志不足以宣称所有守门怪源模板相同；候选不改OBJ，不硬编码1006，不为客户端本地守门怪伪造owned身份。

服务端闭环：

1. `dungeon-hell` 直读当前规则与各自 monster/AIC LIST，未知实体type3保留原值且拒绝转成普通怪。按获批S4手动权重列选择 A/B，然后在各源order内按group权重选一组。
2. 进本即为选定名单保留本次run的entity ID；进入封印图追加hidden=1/selector=-1/order行。怪物rank0，APC rank5/SourceIndex10000/team100；重复进房不重抽，不重分配ID。APC预加载按115 NOTI666先于NOTI29发送模板/等级对；不照抄旧端679，不接猜测CMD85/1061。
3. CMD39沿既有死亡归属校验。记录每组真正最后死亡实体，早死实体的重发不会变成新的奖励主体；FFFF保持无主、无物品。loot额外按(order,group)去重，奖品与怪物共用run对象ID分配器。选定全部实体确认死亡后发送空体NOTI777一次，只作为完成事件，不承载奖品。
4. 专属装备按 `etc/itemdropinfo_monster_hell.etc` 的7列等级/难度行、两行9列rarity原值抽取。115客户端难度1起算，转换到0起算表列；S4概率roll0..1000且<=倍率×rate，rate0不抽，rarity roll1..1000000。环境变量 `DFO_HELL_PARTY_DROP_PERCENT` 为整数0..10000，默认100=1倍。
5. Hell独立装备池通过当前EQU LIST与原脚本正creation rate选取，可覆盖rarity4史诗；不套普通Basic rarity<=2门禁，不改变普通装备池。候选范围为当前背包已支持的战斗装备部位；S4等级窗口为minimum required level..basis level+7，空稀有度仅按S4尝试rarity0，不任意提升/降低稀有度。保留源耐久，通过既有owned地面拾取和保存路径发放；未新增schema。
6. typed HellEpic区域表未被拍平成共用物品池；现代柱子/奖励区域映射、史诗碎片、史诗增益药额外重抽均不在本候选范围。Abyss调律与Odyssey奖励保持原路径。

离线证据：原生60个声明Hell的副本中55个可产生有效名单与NOTI29行；4个无旧式波次数据的地图100012185/100013838/500000730/500000732拒绝，地图100016811源缺失拒绝。此55项不是逐图实机确认。Trombe87/60051可产生源cosmofiend+APC两波，ID/等级/源组与预加载均离线检查。

测试包括：原PVF规则/所有60051候选绑定、cosmofiend标记、APC65级、原生隐藏队列字段和NOTI666宽度、稀疏order/B模式、外来/跨房身份、最后组死亡/重发/FFFF、源概率/装备权重/耐久、史诗池与普通池隔离。Go全量测试、vet、独立编译通过。`go run ./cmd/charactercheck` 使用独立临时schema并完成清理，但旧自检缺少 `account_unified_options` 依赖而失败；未改其代码或玩家表，未将此失败记为通过。

本候选为 `.tmp/hellparty-20261003/wireprobe-hellparty-owned-waves.exe`，SHA256 `308a7b8434815dc2936557d8cebad41faeee61e9dedc6c1fc6d0c6ebcdddc352`。独立profile `pvf-hellparty-owned-waves.json` 与 `启动验证掉落.cmd` 同目录。关闭已有游戏会话后由用户手动进入同一副本87，攻击柱子、打完主怪/APC并尝试拾取装备，再读取日志确认真实CMD39身份及NOTI38掉落。默认程序与已确认入场候选104965f8均保留，可切回原 `启动验证.cmd` 回滚此候选。禁止无人值守操作客户端。

最终启动准备检查：候选以实际启动器参数执行 `-pvf-check-catalogs`，54个数据域全部成功，报告 `runtime_started=false`、`storage_accessed=false`，源哈希为 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`。独立Hell装备派生池5701项（rarity4共1076项），普通池6905项；这不是每个副本都可抽到史诗，等级窗口60..69仅包含rarity2/3。8/6是抽取次数，不保证对应件数。证据保存在 `.tmp/hellparty-20261003/owned-prepare-report.json`、`owned-prepare.log`、`ready-check.txt`。独立profile能被启动器接受，所有必需文件存在；Python profile/launcher 25项测试通过。

启动入口复核发现已有 `test_pvf_default_launch` 的等待测试未隔离 `_ensure_inner_pvf`，模拟启动仍触及真实派生归档准备逻辑，导致 `server/work/client-build/Script.inner.pvf` 与manifest缺失。已补充mock与调用断言，重跑25项测试不会触及真实资源。先从工作区冻结client重建得到be95源，但实际启动器配置使用 `F:/wip/dof/115US`；随后从该配置目录只读重建，校验恢复上述4d8c源，与54域检查报告一致。be95归档保留在候选临时目录作区别审计，不用作本次实机输入。没有修改两处客户端原始文件、玩家存档或默认服务端程序。
