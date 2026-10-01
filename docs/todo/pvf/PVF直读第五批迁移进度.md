# PVF直读第五批：特殊内容与奖励候选（2026-10-01）

第四批28个选择项/34类源数据已由用户确认正常。confirmed baseline 为 `pvf-migration-candidate.json`，程序SHA256 `5475dbccdf316f4c582cc2742b22e1f66b5512f22e04997ccb23a031f6e36609`。第五批新增内容均为离线候选，下文按各次迁移记录；最新54选择项/63类源投影及管理工具候选迁移已完成，待用户手动实机验收；完整保留边界见迁移清单。

| 选择项 | PVF来源与完整核对 |
| --- | --- |
| `apocalypse` | `contents/2026/apocalypse/etc/apocalypse.ctp` 和 `dungeonskillinfo.ctp`，主表65记录、4操作、6阶段共2190秒、职责14记录；原始字段、奖励位置、源哈希与运行投影一致 |
| `attunement` | 通过 `etc/rewardboostinfo/**/*.ctp` 的 `[dungeon index]` 源字段定位4个已启用副本，126奖励模板、5优惠券行；源路径/哈希/概率/隐藏原始字段完整一致 |

调律副本选择 `[100005066,100005067,100005068,100005014]` 置于 `pvf-content-policy.json`。策略不记录脚本路径、源概率或奖励池；文件绑定由PVF声明决定，重复声明拒绝加载。使用无整库切片复制的归档遍历，避免复制全部约565万条记录。现有调参、隐藏表中间字段及Omen启用边界保持；每次运行装载深复制奖励表，调参不改变准备好的源目录。天启时钟与运行操作继续使用原验证器。两个离线导出命令也复用同一源解析器。

源仍为 `server/work/client-build/Script.inner.pvf`，SHA256 `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。角色存档来源门禁保持，未修改协议、客户端资源、数据库结构或玩家存档。

联合30选择项/36类源数据测试将全部所选导出JSON路径设为不存在，准备成功，约47.38秒；两个CTP完整对照及调参副本隔离检查通过。全量Go测试、vet和7项Python profile测试通过。时间是离线准备测试结果，不代表实机启动或内存上限。

独立profile：`server/work/dfo-lan/configs/pvf-content-candidate.json`。

隔离程序：`server/work/dfo-lan/.tmp/pvf-content/bin/wireprobe-handoff-source.exe`。

SHA256：`fcefe7737c198e1425a4d87c03affc7122f7828d1615412835751ed1beafcf2d`。

只读 `launch_local.py --repair-profile configs/pvf-content-candidate.json --check` 通过；未启动客户端、服务或连接玩家数据库。第四批及之前回退程序保持。Odyssey、其它副本覆盖、特殊奖励、嵌入目录、商品绑定、职业源/策略拆分和GM查询继续按实施计划推进。


## 后续奥德赛五项（离线候选）

奥德赛五项直读已完成离线候选，合计35选择项/41类有效源投影。成长源50通关/50准入副本、3赠品及毕业礼盒10420561与毕业主线完整一致；章节7章/50副本/15奖励模板、7行章节掉落、2种货币及85组创建武器选项完整一致。章节2/7禁用、章节概率和货币概率 `[1000,10000,10000,10000]` 保留为独立策略，奖励模板与最终副本由PVF推导。35项缺失所选JSON联合准备、完整源对照、全量Go测试/vet及8项Python测试通过。profile为`pvf-odyssey-candidate.json`，隔离程序SHA256 `9c1b9722733ef93db5f57d11fc25fa415dfd01e5f16c36b6bdea89c97ff26f3a`。确认范围仍为第四批28项；所有旧候选程序/策略保持可回退。

| 选择项 | 源字段与核对 |
| --- | --- |
| `odyssey-growth` | `contents/2026/aradodyssey/etc/aradodyssey.etc`：50成长/50准入副本、3级别赠品、毕业礼盒10420561、毕业任务计划及6份关联源脚本全部一致 |
| `odyssey-chapters` | `aradodysseyjournal.cos`：7章、50副本、15奖励模板，原顺序和最终副本一致 |
| `odyssey-drop` | 从源章节奖励及共享物品索引推导首个选择箱和最终副本；7行一致，章节2及7继续停用 |
| `odyssey-currency` | 两种币的 `.stk` 全部原始Token、源哈希、Grade/Rarity/StackLimit一致；权重0，不进入普通随机掉落池 |
| `odyssey-weapons` | 10417789脚本的85组类别及装备选项；共享选择箱源解析器，沿用原武器奖励启用开关 |

新增 `pvf-odyssey-policy.json` 保存章节启用/概率、已确认货币概率与按怪物rank选币策略，以及原成长证据目录的额外2个物品ID（10418028、10418036）。赠品、毕业物品、章节最终副本及选择箱模板不保存在策略中；所有脚本路径/哈希/属性从PVF读取。货币rank选币是现有服务器策略，未据此宣称源官方掉率已恢复。原 `pvf-content-policy.json` 没有加入新字段，因此旧30项程序的严格策略解析仍可使用。

新的35项profile为 `server/work/dfo-lan/configs/pvf-odyssey-candidate.json`，对应程序 `.tmp/pvf-odyssey/bin/wireprobe-handoff-source.exe`；旧30项profile和程序仍保留。35项源装配耗时约49.94秒；完整奥德赛对照约15.63秒。两个离线导出器共享运行源解析。只读启动依赖检查通过，未启动客户端、服务或修改玩家数据库。本批尚未实机确认，剩余特殊奖励、副本覆盖及其它目录继续迁移。


## 无色小晶块源覆盖候选

新增clear-cube选择项，3037无色小晶块从共享PVF索引及原始脚本直读，完整源Token/哈希对照一致。保留原存储覆盖中Grade/Rarity/Weight为0的最小投影，原源值仍在ScriptRecord中；不进入普通掉落池，不改变分解或技能消耗公式。总计36选择项/42类有效源投影，缺失所有所选导出JSON联合准备约42.10秒通过，混用存储来源拒绝。全量Go测试/vet、8项Python测试及只读依赖检查通过。新profile为`pvf-cube-candidate.json`，隔离程序SHA256 `c6e29f3cc4c8b375ee9ecbc7781553134b005c6b1d03cc94980b83ffddd1261b`；第五批尚未实机，确认范围仍为第四批28项。

新增选择项`clear-cube`只替换原`clear-cube-source.json`的来源。源脚本`stackable/material/cubepiece_clear.stk`中Grade=5/Rarity=1完整保留；现有业务投影仍为0/0，不因迁移将其纳入随机候选。源哈希仍由原覆盖验证器强制校验。新程序位于`.tmp/pvf-cube/bin/wireprobe-handoff-source.exe`，所有原profile/程序/策略文件保持。特殊奖励与其它剩余目录继续实施。


## 黑鸦源奖励与装备范围候选

新增black-purgatory直读选择项，普通翻牌5分支、仅记录的VIP1分支、三组装备源范围完整一致（史诗208、神话35、腐蚀产物135件）；现有奖励包装展开、装备验证与事务链保持。10%/0.1%/1%本服独立概率迁入`pvf-reward-policy.json`；源脚本路径/哈希、八列奖励及装备ID/等级/稀有度全部从PVF读取。合计37选择项/43类有效源投影；37项缺失所选JSON联合准备约42.10秒、完整黑鸦源审计、全量Go测试/vet、8项Python测试和只读依赖检查通过。最新profile为`pvf-rewards-candidate.json`，隔离程序SHA256 `a044d143c38924931675929bd2bc768fcbcd551c1f002a91ef6176d9520bf5b1`。确认范围仍为第四批28项；第五批候选未实机。

`etc/dungeonspecialreward.etc` 的八列翻牌表按已取证的记录顺序解析：每个普通/VIP奖励分支的五档条件逐行一致，概率各合计10000；VIP数据只记录，未混入普通奖励池。装备三组由 `etc/itemdictionary/customroutingwaygroup.cos` 的group1010001、1010500、1010007及共享源装备索引推导。各装备脚本的ID/路径/哈希、等级与稀有度核对一致；`customroutingway.etc`整份哈希一并参与审计。包装全图校验在源准备阶段执行，运行再次通过原构造器与装备验证器，未扩大玩法实现。

旧JSON的`client_pvf_sha256`为历史外层`2429b15a…`，角色来源`source`仍为inner `7ef2db59…`。直读记录实际读取归档的inner哈希。审计只在角色inner来源严格一致、翻牌/分组/routing三份原始文件哈希全部一致时，允许归一化这一明确的旧导出来源元数据；未知外层来源或任一源定义变化拒绝。这不是存档版本迁移，也没有为归档checksum建立别名。其它有效字段（含旧运行类型未接收的VIP和两份routing来源哈希）全部参与比较。

新profile为`configs/pvf-rewards-candidate.json`，使用独立`pvf-reward-policy.json`，程序位于`.tmp/pvf-rewards/bin/wireprobe-handoff-source.exe`。旧35/36项策略和程序继续可回退；运行仍使用同源inner `7ef2db59…`，没有启动客户端、重启服务或修改玩家数据库。矿区奖励、剩余副本覆盖、抽奖/选择箱、嵌入目录、商品源绑定、职业源/策略和GM查询继续实施。


## 赤红铁矿源奖励图候选

新增bleeding-mine直读选择项，12阶段/12领主入口/3难度奖励、117容器、1782物品及全部合成列表/权重完整一致；35个负数空奖签保留，合成机会[1,3,5]和最大5次保持，源失败占位物10330673继续强制禁止发放。修正共享CTP标签池定位：矿区列索引90024包含0x5b，真实池始于90481，改按头部trailer边界跳过NUL垫字节；已加入索引91含左括号的回归用例，天启/调律再次完整核对通过。合计38选择项/44类有效源投影，缺失所选JSON联合准备约42.11秒、全量Go测试/vet、8项Python测试及只读启动检查通过。最新profile为`pvf-mine-candidate.json`，隔离程序SHA256 `3df13ff8b310f634b09308dcf6e5faa3558e8ccc679b3814a79953a812024571`；确认范围仍为第四批28项，第五批新增候选未实机。

源为 `contents/2025/bleedingmine/etc/bleedingmine.ctp`（841记录）与 `bleedingminerewardscript.ctp`（21记录），复用已取证的CTP格式，不新增协议包。难度键按源 `easy/medium/hard` 顺序投影；阶段和领主奖励、奖励袋 `[booster info]`、装备罐 `[int data]`、智能掉落组与两类合成表递归从源读取。不存在的源物品、未能展开的智能组、循环、零奖签或无效权重拒绝发布。负数-1/-2保持空奖，禁止取绝对值。运行物品保留原索引投影，未因迁移加入Script/Grade/随机Weight，原Rarity投影一致。规则整图验证复用原构造器，奖励冻结、领取、邮件及合成事务沿用原流程。

原 `inspect_bleeding_mine.py` 已按头部offset28的trailer边界定位pool，而共享Go解析器此前从cellEnd搜索首个 `[`；矿区列索引出现0x5b暴露该差异。只读原始字节确认cellEnd=88648、trailerEnd=90480、错误首括号=90024、实际pool=90481。修正使用已有头部字段，未猜测新的编码格式；新的合法92记录回归样本包含列索引91，以及错误trailer边界拒绝用例。天启和调律完整源核对再次通过。

新增 `pvf-mine-policy.json` 仅在上一批独立策略基础上保留失败占位物排除ID。源机会/概率/合成范围不进入策略；缺失该排除也拒绝启动，避免将Drop Failure Dummy当作奖励发放。最新程序为 `.tmp/pvf-mine/bin/wireprobe-handoff-source.exe`，profile为 `configs/pvf-mine-candidate.json`。其它已确认及候选程序/策略继续保持，未启动客户端、服务或改写玩家数据库。剩余副本覆盖、商品绑定、抽奖/选择箱、嵌入目录、职业源与策略拆分及GM查询继续实施。

## 后续任务终端场景与武斗大会地图（离线候选）

新增dungeon-terminal与dungeon-tournament直读选择项，7条任务终端场景和2张武斗大会任务地图完整源对照一致。终端场景复用导出器的任务目标/末层/ACT/CMT解析链；竞技场依原MAP的[dungeon]所属关系及DGN任务迷宫绑定，重复所属拒绝。末层范围、脚本哈希、剧情销毁目标和地图Token均从PVF读取；现有结算与协议记录保持。运行附加不修改准备好的源副本目录。合计40选择项/46类有效源投影，缺失所有所选JSON联合准备约55.08秒（与全量测试并发）、独立场景审计、全量Go测试/vet、8项Python测试及只读依赖检查通过。profile为`pvf-closing-candidate.json`，隔离程序SHA256 `5be230e05298100700957c8eb9559930be35a04bd0710db2e8afed89f809fb0c`。确认范围仍为第四批28项；第五批新增候选未实机。

独立程序路径为`server/work/dfo-lan/.tmp/pvf-closing/bin/wireprobe-handoff-source.exe`，复用38项的不可变`pvf-mine-policy.json`，无需新策略字段。原38项及所有旧profile/程序保持可回退。场景准备在打开存储之前完成；未启动服务、客户端或连接玩家数据库。JSON附件验证器与原离线导出命令复用同一终端场景投影。

## 后续通用自选箱（离线候选）

新增selection-boxes直读选择项，原2978个加载模板仅以ID保留在独立`pvf-selection-policy.json`；PVF按原索引精确路径解析2975个自选箱、2个固定箱和1个未解析模板。所有类别、数量、推荐项、未建模段标识和脚本哈希与原导出完整一致；逐类别Resolve派生查询核对通过。保持既有未知选择项观察策略及发放事务，不扩大为全PVF自选箱范围。合计41选择项/47类有效源投影，缺失所有所选JSON联合准备、完整选择箱审计、全量Go测试/vet、8项Python测试和只读启动依赖检查通过。profile为`pvf-selection-candidate.json`，隔离程序SHA256 `4e42eac7d702b01fc9acd430862220072ca3e2a72691d74deed6daa4e222c353`；确认范围仍为第四批28项，第五批新增候选未实机。

策略文件严格拒绝源字段、重复模板、零模板和尾随内容。原10307659/490022952固定箱及未解析10358468保持；原解析器同时由自选箱导出命令、奥德赛武器选项和原生自选箱使用。候选程序路径`server/work/dfo-lan/.tmp/pvf-selection/bin/wireprobe-handoff-source.exe`，额外profile路径键为`DFO_PVF_SELECTION_POLICY`。完整源对照及逐类别查找约16.39秒通过；所有旧策略/profile/程序保持可回退。未启动服务或客户端，未更改角色存档、协议或数据库。

## 后续两类抽奖源池（离线候选）

新增lottery直读选择项，276个材料/金币池和2477个装备池按原精确物品索引路径读取PVF的[int data]三元组与原始脚本哈希，全部源池及派生总权重完整一致。额外核对271492个权重区间边界，保持0模板金币和7772/10306598/7213源回归门禁。原启用/可发放池ID范围独立放入`pvf-lottery-policy.json`，不复制奖励、概率或路径；装备池仍随既有穿戴服务门禁启用。每次运行装载重建私有索引/总权重，互不污染；扣物、发放与事务回放保持。合计42选择项/49类有效源投影，缺失所选JSON联合准备约65.30秒（并发全量测试）、独立完整源审计、全量Go测试/vet、8项Python测试及只读依赖检查通过。profile为`pvf-lottery-candidate.json`，隔离程序SHA256 `6a440c87854e1132e647b3562b81c5552dea652944d549d296d74a5ab05afb41`。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径`server/work/dfo-lan/.tmp/pvf-lottery/bin/wireprobe-handoff-source.exe`，新增profile路径键`DFO_PVF_LOTTERY_POLICY`；旧41项及全部旧profile保持。2753个源池共135746行奖签，两端边界共271492次对照；完整审计约16.09秒通过。策略严格拒绝源奖励字段、重复/跨集合重复ID、零ID和尾随内容。未启动客户端、服务或连接玩家数据库，不改协议或存档来源。

## 后续冒险团内嵌规则（离线候选）

新增adventure直读选择项，从etc/adventurersystem/adventurersystem2018.etc及精确物品索引读取60级经验表、3类商店和25个物品的价格、限购、重置、期限及脚本哈希。经验倍率保留float32原位，经验十进制字符串按64位整数读取（50级171361456285、60级369956531543）；最高开放等级仍为50。启动在打开存储前安装独立深拷贝规则，正常直读不解析内嵌rules.json；未知或无效来源拒绝替换。旧内嵌文件记录2429外层身份，仅完整审计中允许该已知身份对7ef内层的元数据对齐，所有源哈希及规则必须完整一致，不改变运行来源或玩家存档。合计43选择项/50类有效源投影；缺失所选JSON联合准备51.06秒、完整源审计、全量Go测试/vet、8项Python测试及只读依赖检查通过。独立profile为pvf-adventure-candidate.json，程序SHA256 acefaba438d6692a9baf1213061de686a7fa163f47f4bbff1b7cf474dc8a07fa。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-adventure/bin/wireprobe-handoff-source.exe。没有新增策略文件；旧42项及全部旧profile保持。关闭内嵌读取后的规则调用、安装失败不污染当前目录、可变切片及映射深拷贝、未知来源/源哈希改变拒绝均通过单元验证。未启动客户端或服务，未连接玩家数据库，不改封包、业务事务、schema或存档。

## 后续推荐地下城内嵌规则（离线候选）

新增adventure-recommended直读选择项，按event/conditioneventchkdungeon.evt及list/worldmap.lst的精确WDM引用展开1446个推荐等级范围、337个排除副本，保留2个歧义副本和7个失效区域引用诊断。WDM按副本/任务条件配对读取，专属范围覆盖区域范围，跨区域冲突不任选一份。51份原始脚本哈希及完整规则一致，全部相关副本在0～255级的444160次运行时资格对比通过；实际推荐计数仍走原业务调用。启动在存储前安装深拷贝及私有排除索引，正常直读不解析内嵌recommended_rules.json。旧2429外层身份只在完整源审计中对齐已知7ef内层，源哈希和诊断不豁免，不改玩家存档身份。合计44选择项/51类有效源投影；缺失所选JSON联合准备46.67秒、完整源审计11.68秒、全量Go测试/vet、8项Python测试和只读依赖检查通过。profile为pvf-recommended-candidate.json，程序SHA256 816a50dd39251cc88f887a2389984fbb9c7327e4fb4b86e0498bffc866b0e1c6。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-recommended/bin/wireprobe-handoff-source.exe。没有新增策略文件；旧43项及全部旧profile保持。额外验证未知选图条件和缺失任务配对拒绝、关闭内嵌读取后仍可查询、规则副本不污染源，以及安装无效规则不覆盖当前有效索引。未启动客户端或服务，未连接玩家数据库，不改协议、schema、存档和业务事务。

## 后续迷雾誓约内嵌规则（离线候选）

新增season直读选择项，从contents/system/seasonlevel/main.cos、精确cost key及etc/costs.ctp、原生物品索引读取120阶经验/名望、59条玩法规则、8组衰减、40个经验道具、4个奖励物品、12件誓约装备及限时奖励。COS原始哈希172bb1119834f8d891be071c7defe4e71deb5a690efeba79e6596a05bd435175，CTP哈希ae67348fad62643a341cd308955544edcc3b5269391749e805efb6783f940bdb；全部有效字段、费用键及40道具原始哈希完整一致。key16的源金币-1仍投影为0，材料10403609数量10；其他分支、分数成本、所有者/引用异常拒绝。360个等级边界及活动起止两端一致。规则在存储前深拷贝安装，正常直读不解析season_rules.json；旧2429外层仅在完整源审计中对齐已知7ef内层。最高显示100级、20次获取、原存档赛季校验、周边界、奖励和交易流程保持。合计45选择项/52类有效源投影；缺失所选JSON联合准备51.06秒、完整源审计17.07秒、全量Go测试/vet、8项Python测试和只读依赖检查通过。profile为pvf-season-candidate.json，程序SHA256 dbad515d612fcf7e10217677ba5f3b8d912ebeea2feb77d5fd714cac8f641349。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-season/bin/wireprobe-handoff-source.exe。旧44项及全部旧profile保持。额外验证关闭内嵌读取、等级和材料切片深拷贝、安装失败不覆盖有效规则；费用引用按父子所有者及唯一key校验，不允许optional或未解释子树。未启动客户端或服务，未连接玩家数据库，不改协议、schema和存档。

## 后续奥德赛日志传送内嵌规则（离线候选）

新增odyssey-routes直读选择项，从contents/2026/aradodyssey/etc/aradodysseyjournal.cos的[node]/[teleport info]/[dungeon]有序块读取29个日志传送节点、50个副本引用和原生目的地区域/坐标。原始SHA256 d4654fa9a50ddd582077f5f7a0a19835ec4fb6b777f032d1a66be7f288eff67e及7ef归档身份必须一致，全部节点完整匹配内嵌目录。准备阶段完成来源验证，运行服务绑定独立深拷贝；正常直读不解析odyssey_journal_routes.json。全部50个进度前缀下2958次传送资格对比通过，保留按先前节点已确认通关解锁、仅比较目的地town/area、允许客户端不同合法落点及尾标志[0,2]、排除地图选择器标志5的现有行为；不改区域变更封包或世界落点校验。合计46选择项/53类有效源投影；缺失所选JSON联合准备48.22秒、完整源审计10.37秒、全量Go测试/vet、8项Python测试和只读依赖检查通过。profile为pvf-odyssey-routes-candidate.json，程序SHA256 d05244375fb6e98af0356259a598bb43f7d8bb6fec1527eba34cef89959dbdef。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-odyssey-routes/bin/wireprobe-handoff-source.exe；旧45项及全部旧profile保持。重放已有魔界(31,2)和天界(12,0)入口单元案例通过；跨来源角色与地图选择器尾标志拒绝。关闭内嵌读取后所有进度前缀仍可查询，绑定切片改动不影响准备源。未启动客户端或服务，未连接玩家数据库，不改schema和存档。

## 后续选角背景券与原生资源（离线候选）

新增roster-backgrounds直读选择项，从原生索引遍历所有stackable脚本，以[action type]的[change bg select character]定位95张选角背景券，读取类别/编号、[action expiration info]的永久/按天/固定日期、源路径和哈希，72永久、2按天、21日期完整一致。63个背景类别/编号由etc/selectcharacterver2/selectcharacterver2.etc的[group]/[image]闭合块读取，原始SHA256 5ef228e9f72658cefd402227e8f6f33972609b61c59ce89c334bdbf0caaaba27，全部16777216个uint8类别/uint16编号组合与原有效性边界一致。图片中的同名[background image]字段按所有者区分。158次授权时间边界一致，物品删除日期与背景授权日期保持独立。规则和资源索引在存储前深拷贝安装，TicketFor及Background.Valid正常直读不解析tickets.json或使用旧编号范围；特殊背景仍需账号拥有，五页选择、原生32位时间和交易流程保持。旧2429外层只作已知完整审计来源，不改7ef运行或存档身份。合计47选择项/55类有效源投影；缺失所选JSON联合准备60.31秒、完整源审计17.46秒、全量Go测试/vet、8项Python测试及只读依赖检查通过。profile为pvf-roster-backgrounds-candidate.json，程序SHA256 b5530fa668bf0a54c3e0c4cc2651e253402d2ac423528c65c168b7c4a7ef2799。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-roster-backgrounds/bin/wireprobe-handoff-source.exe。旧46项及全部旧profile保持，没有新增策略文件或账号自动授予。未启动客户端或服务，未连接玩家数据库，不改封包、schema或账号/角色存档。

## 后续名望内嵌规则（离线候选）

新增fame直读选择项，读取etc/famevalueinfo.etc、equipmentgrouping.etc、equipmentpartset.etc、115lvability/setpointinfo.cos、原生套装阈值列表及全部stackable脚本。9张名望表、8100个物品、13组套装阈值、1054个物品积分、336个觉醒模板及8411个源路径/原始哈希完整一致；保留字段最后出现值、同组最大觉醒值、积分去重、part set index=-1以及源列表中的旧缺失引用处理。规则在存储前深拷贝安装，正常直读名望入口不解析fame_rules.json，原生单精度计算公式、锻造/强化取高、记忆和独立装备惩罚行为保持。验证全部8100物品附魔计算、全部可变规则副本隔离、非法安装不覆盖有效规则。旧2429外层仅作已知7ef内层的审计来源，不改真实7ef运行或存档身份。合计48选择项/56类有效源投影；缺失所选JSON联合准备48.32秒、完整源审计17.84秒、全量Go测试/vet、10项Python测试和只读依赖检查通过。profile为pvf-fame-candidate.json，程序SHA256 836d1f050b4662e19591a3c464f2fb977a720cf32df6b2e4e3dee35bc585cb9d。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-fame/bin/wireprobe-handoff-source.exe。全部旧profile和第四批确认程序保持；未启动客户端或服务，未连接玩家数据库，不改协议、schema、账号/角色存档。

## 后续两类脚本传送内嵌规则（离线候选）

新增script-warps直读选择项，覆盖11条CMT传送和1条怪物动作强制切房。源CMT的[MAP]及原生cinematic列表绑定确定地图身份，[BEHAVIOR]内对象模板/地图对象序号、OBJ自定义动作索引、ACT的[MOVE MAP]或[KICK OUT MAP CHARACTER]确定目标格及落点；DGN/maze决定网格所属与目标地图，已声明的warp条件必须一致，源ACT强制/忽略状态路线允许没有重复DGN声明。怪物路线由原生monster列表、maze内实际怪物及其etc action定位。全部原始DGN/MAP/CMT/ACT/OBJ哈希、12条有效路线和落点字段完整一致。独立pvf-script-warp-policy.json只保留已启用源标识、实测18字节record及关键房准入，不保存导出地图/坐标/哈希。正常模式不解析两份内嵌script warp JSON；关闭内嵌数据后12条移动仍通过，源切片隔离、无效安装和逐字节篡改拒绝均通过。合计49选择项/58类有效源投影；缺失所选JSON联合准备53.76秒、完整源审计33.36秒、全量Go测试/vet、10项Python测试和只读依赖检查通过。profile为pvf-script-warps-candidate.json，程序SHA256 410ed509de6a125e01156e988d5d5cb17f444341c437d6c23622353811cb40d0。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-script-warps/bin/wireprobe-handoff-source.exe。全部旧profile及第四批确认程序保持；未启动客户端或服务，未连接玩家数据库，不改协议、schema、账号/角色存档。

## 后续剧情层回访源目录（离线候选）

新增layer-revisits直读选择项，任务12893末层回访由CMT的[MAP]、MAP basic action至原生cinematic列表的精确引用、唯一[CHANGE MAP]、DGN任务maze及末层/同格base地图读取。剧情地图100004546、恢复地图100004325、任务/网格、原始DGN/MAP/base MAP/ACT/CMT哈希及全部有效字段完整一致；源落点(165,289)与已实测record一致。独立pvf-layer-revisit-policy.json仅保留启用DGN/maze/CMT标识、实测18字节记录和resume_base缓存恢复策略，不保存导出地图、坐标、任务或哈希。正常选择项不读dungeons.layer-revisits.json，应用目录深拷贝，不改准备源副本；无效/异源/base不在同格及每个篡改record字节均拒绝。现有剧情结束回同格原战斗房、12个实体、死亡缓存与NOTI29 flag2/mode0测试通过，玩家仍需实际清怪开门。合计50选择项/59类有效源投影；缺失所选JSON联合准备65.72秒、完整源审计26.86秒、全量Go测试/vet、10项Python测试和只读依赖检查通过。profile为pvf-layer-revisits-candidate.json，程序SHA256 ebdbf6a17b045494c0569bcd1c852a2126f98d776416bd2320977f213b1f01d9。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-layer-revisits/bin/wireprobe-handoff-source.exe。全部旧profile及第四批确认程序保持；未启动客户端或服务，未连接玩家数据库，不改协议、schema、账号/角色存档。

## 后续职业源与运行策略拆分（离线候选）

新增characters直读选择项，17个职业属性、初始/转职/觉醒技能授予、成长及默认装备/外观从原生CHR读取。完整运行字段审计通过，旧JSON与原始导入的341处差异已逐项归因：175处源命令、61处转职快捷栏、54处初始快捷栏和各17处重复成长/预设/栏位视图。独立pvf-character-policy.json只保留已确认源身份、17职业默认快捷栏及源命令/转职快捷栏启用策略；运行保持原有快捷栏和默认命令行为，完整原始成长视图另行保留，不改源导入器。17个CHR哈希不变，存档源仍严格绑定7ef，不加source alias、不迁移或改写账号/角色存档。正常读取连characters.skycastle-release.json源锚点也可缺失；所有51选择项/60类投影缺失JSON联合准备62.34秒，17职业完整审计10.57秒，全量Go测试/vet、11项Python测试和只读依赖检查通过。profile为pvf-characters-candidate.json，程序SHA256 576d41bc830a18aea83f9af0eb322b475540809e7f3abd02f67228ef138438b6。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-characters/bin/wireprobe-handoff-source.exe。第四批已确认程序SHA256仍为5475dbccdf316f4c582cc2742b22e1f66b5512f22e04997ccb23a031f6e36609。未启动客户端或服务、不接入玩家数据库；只读检查只探测已有依赖端口。

## 后续现金商城原生目录（离线候选）

新增cashshop直读选择项，从etc/(r)cerashop.etc、stackable/equipment原生列表及全部关联STK/EQU读取价格、商品与购买策略。17245条商品记录的全部typed cells、索引路径、原始脚本哈希、原有导入拒绝原因及8组源策略完整一致；实际16606项可购买Product投影完整一致。发布模式保留为既有DFO_SHOP_RELEASE独立服务端开关，候选显式为1；不改变扣款、事务、发货、契约、限购或仓库容量行为。NewPilot复用原LoadPilot验证和分类，深拷贝导入行/脚本/策略，ProductSnapshot不暴露购买缓存；正常读取及准备复用不访问shop-vault-release.json。52选择项/61类源投影缺失JSON联合准备50.05秒，完整商城审计15.05秒，全量Go测试/vet、12项Python测试和只读依赖检查通过。profile为pvf-cashshop-candidate.json，程序SHA256 a126d896a55ca75628a28574747f3130484e5cb50205773ce53a7d44db175c45。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-cashshop/bin/wireprobe-handoff-source.exe。第四批确认程序、旧profile和玩家数据库保持，未启动客户端或服务。

## 后续光辉宝箱COS直读（离线候选）

新增boxes直读选择项，两个同名radianttreasurebox.cos由原生[material]唯一关联：2024/0514属于590712474普通箱，2025/0318属于590719043增强箱。两个表的rate、材料数量、普通/特殊池、逐行tier/模板/数量/权重、bonus/section计数器、75次变形点及原postal tag全部一致；54个奖励的原生STK类型/堆叠/槽位完整一致，新增记录58条原始源哈希。旧boxes.json仅有描述来源且未记录原哈希，审计明确校验原描述并比较全部有效字段，不声称旧哈希可比；运行源严格为7ef。独立pvf-box-policy.json仅保留两个启用模板/COS候选路径、原有槽位及缺失stack limit默认1000；不保存奖励池、概率、节点或哈希，不按basename/日期/遍历顺序选择。正常直读不读/探测boxes.json；既有抽取、逐抽行、保底计数、契约和事务不变。53选择项/62类源投影缺失JSON联合准备51.87秒，完整礼盒审计15.79秒，全量Go测试/vet、13项Python测试及只读依赖检查通过。profile为pvf-boxes-candidate.json，程序SHA256 654540902348dc4bb713d283f5b7fa6b36fa26faab11b6e6ac428aaa33270c06。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-boxes/bin/wireprobe-handoff-source.exe。原接口对可变缓存的访问不变，重复reward行保留原顺序，错误/重复material、未闭合块、负数、行数及权重溢出拒绝。未启动客户端或服务、不接入玩家数据库。

## 后续物品商店源与服务端路由拆分（离线候选）

新增item-shops直读选择项，527个现有服务端商店的全部7025条SHP商品、原始路径/哈希、源NPC/type、tab/index、purchase amount和材料支付完整一致；逐商品核对实际Materials/Listed/PurchaseAmount/PurchaseLimit查找也全部一致。独立pvf-item-shop-policy.json只保留既有服务端路由：202个同ID原生列表绑定、30个明确原生列表ID兼容映射、295个既有明确源路径；不把旧服务端ID解释为客户端原生ID，不按文件名或遍历顺序选冲突表，不扩大路由范围，当前295个未列入原生列表的服务端路径未重新证明为原生客户端路由。源价格/材料/商品/哈希不保存在策略内。purchase_limit_mode=disabled保留本服当前不限购行为，未直接将STK源限购自动接入交易。Odruz源100000607第三tab缺少[sell item list]起始标记，原导入器排除其4个item；新读取保持该准入边界，非法材料/未闭合条目拒绝。NewItemShops深拷贝offers/materials并复用首个可支付报价优先；正常读取不访问itemshop-candidate.json，源或运行目录不匹配时拒绝。54选择项/63类源投影缺失JSON联合准备67.68秒，完整商店审计10.92秒，全量Go测试/vet、14项Python测试及只读依赖检查通过。profile为pvf-item-shops-candidate.json，程序SHA256 c29f2d7134988d986b993af4bc33163e9b995f48663ccbdeb2ccf4c6f2f3ec63。确认范围仍为第四批28项，第五批新增候选未实机。

独立程序路径server/work/dfo-lan/.tmp/pvf-item-shops/bin/wireprobe-handoff-source.exe。未修改其它物品导入命令、客户端、数据库schema或存档，未启动客户端/服务；原已确认程序与所有旧候选保持。

## 2026-10-01：管理与修复命令共享源入口

admin、initialrepair、questrepair已接入显式catalog-source=pvf入口，共享internal/managementdata和原生gamedata；管理查询/发放的源目录、建号修复的职业及装备、旧任务修复的任务目录可完全不读取导出JSON。新增check-catalogs在读取存储配置前退出，无需角色ID；只读准备不启动数据库、服务或客户端。来源必须显式提供inner归档路径及精确7ef SHA256，不转换存档来源。袋位、穿戴/建号规则、默认技能栏与装备选取仍用现有独立策略；运行修复的development-only、幂等键、事务、审计和apply门禁保持。真实归档管理审计19.37秒通过，17职业、175554个补充后物品和3174行基础装备完整typed parity，424216完整装备绑定可用；奥德赛币和import-script薄壳装备仍通过原发放验证。全量Go测试/vet通过。三个隔离程序将导出JSON与storage路径均设为不存在仍完成只读准备；未访问玩家库。网关候选维持54选择项/63类源投影，confirmed baseline仍为第四批28项。

候选程序置于server/work/dfo-lan/.tmp/pvf-management/bin/，未覆盖gm-tool发布程序。示例（模块根目录执行，仅目录检查）：

```powershell
./.tmp/pvf-management/bin/admin.exe -catalog-source pvf -pvf-archive ../client-build/Script.inner.pvf -pvf-source-checksum 7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80 -check-catalogs
```

initialrepair/questrepair同样支持这些参数。check-catalogs只准备目录；默认修复预览仍会连接数据库和迁移既有表，当前迁移验证未执行普通预览或apply。GM名称核对发现当前客户端uv为英文、translate/kor主要为韩文，与旧中文译名不同；继续处理可迁移元数据并保留无法在当前源表取得的外部译名。

## 2026-10-01：GM原生管理查询与外部译名边界

GM新增显式PVF候选入口，管理索引、装备部位/最低等级、原生显示文本及发放目录共用gamedata/managementdata；599771个原生LIST绑定的源脚本全部完成读取。旧386230个GM物品ID全部可达，kind/grade/rarity完整比对零差异；重复grade/rarity出现在嵌套条件块时，显示投影沿用旧导入器首字段行为，不用通用单值验证将其置0。实际[name]引用替代name_<ID>拼键，旧ID拼键显示有663处差异，保留原生别名/chn引用与纯文本。旧equipment.slots有392575条，其中6561无当前LIST绑定、12条为原生堆叠物，416个部位和244403个最低等级字段与当前源不同（大量旧装扮最低等级为0，当前源为1）；候选显示使用源值，旧缓存不再参与筛选，未将这些差异宣称等价。当前实际客户端uv主要英文，translate/kor主要韩文，旧中文译名多数无法据当前名称表还原，names.client/names.zh仅作可选外部显示覆盖，不能提供ID、属性、槽位或存档来源。GM发放补充目录与网关一致为175554种物品并挂原生424216完整装备绑定，仍由原发放验证、事务、幂等及审计决定实际可发放，不因索引存在自动放行未知特殊状态。新增认证只读/api/catalog-metadata，Python代理使用后端类型/可堆叠集合，原生模式不再读取重复物品/装备/loot导出JSON；仅旧后端明确404时保留兼容路径，401/5xx及错误响应拒绝。gmweb.py支持显式候选程序、源SHA256和只读check；候选准备在自动存储启动和数据库读取之前结束。源审计34.08秒、全量Go测试/vet、14项原Python测试及5项GM只读测试通过；实际候选check在storage不存在时成功，未启动服务或客户端/数据库。隔离gmweb程序SHA256为6a04bf5e11ee676527fd76b74028f9ace7874b6892e47f964ee114f577cac7d1；未覆盖GM发布程序，网关候选仍54项/63投影，确认范围仍第四批28项。

当前Go后端/代理/前端未消费set_items、avatar_sets、set_display_names、equip_whitelist源文件；这些文件继续保留为未启用资产/工具选择数据，不因历史归档中的引用而新增功能。历史备份和数据导出命令可继续生成JSON作为审计样本，不作为PVF候选运行前提。

## 2026-10-01：全量PVF候选与无数据库准备检查

当前生产入口中已定位PVF真源的数据完成候选接线：网关54个选择项/63类源投影，以及admin、initialrepair、questrepair和GM查询/发放。新增configs/pvf-all-candidate.json汇总之前逐批范围，使用隔离程序.tmp/pvf-all/bin/wireprobe-handoff-source.exe；环境和独立策略与上一商店候选一致，不改变用户客户端路径、存档来源或既有玩法开关。程序SHA256为a3ea388ac9a2966f0368e6ede552f3d8559fc10bfba08f24f5158bb583e2d98c。

新增-pvf-check-catalogs，必须显式选择PVF领域，在源身份及目录准备成功后、安装运行全局/访问存储/创建捕获目录/监听端口之前输出报告并退出。实际全量检查将26个导出JSON参数、存储配置和输出目录设为不存在，53.26秒完成：17职业、2844任务、599771物品、424216完整装备绑定、3174装备选择行、3200副本、16606商城商品、2个COS礼盒和527商店；storage_accessed=false、runtime_started=false，未创建指定输出目录。源仍为精确7ef2db59…，不建立source别名。修正boxes独立选择时漏装原生item索引的依赖，characters,boxes缺失JSON独立准备实测通过。

最后一轮go test ./...与go vet ./...通过，13项profile、2项inner-PVF和5项GM Python检查共20项通过；全量profile的launch_local --check --server-only路径/端口检查通过。该检查未启动服务、数据库或客户端，也未执行普通数据库修复预览/apply或历史charactercheck。

详细完成项及保留理由见docs/todo/pvf/PVF直读迁移清单.md。剩余配置为独立运维/服务端或客户端布局策略、外部中文显示覆盖、历史样本与未启用资产；锻造公式尚缺可靠源定位，不将检索未命中宣称为PVF没有。confirmed baseline保持用户已确认的第四批28项/34类源数据，隔离程序SHA256仍为5475dbccdf316f4c582cc2742b22e1f66b5512f22e04997ccb23a031f6e36609，已重新核对。第五批及GM候选需用户手动实机验收，默认入口未据离线通过自动升级。
