# 精锐 APC 距离传送候选（DLL v0.3.11）

用户明确授权“能不能在dll实现，发现apc太远就传送”。服务端N138/N22无法按三个65535控制器精确选择APC，见同目录elite-server-teleport-feasibility-20261010.md；因此采用本地原生精锐对象、主人CRef与独立对象编号进行检查。此前换层入口修复已经由用户确认并提交6d5f84cb，本候选未实机验收，不更新CHANGELOG或confirmed baseline。

## 当前客户端证据

只读权威client/DFO.exe.i64，会话a07848af；没有修改、另开或保存IDB。N138处理器1452E8880原生虚表+432立即设置坐标；145B960D0以整数x/y/z调用坐标组件145E0CD40，反汇编确认第三整数参数转换为float传入组件虚表+16。145B8C7D0读取actor+368并以mode0调用145E08940，145B8C890/8C0分别读y/z。145CF6C20为actor+70914死亡标记getter。145C70EE0是三名APC共用的落点检查虚函数。1459A90F0读取当前world状态，1459A9260→1459A9180→145E930B0获得当前场景。原生N22在状态8执行副本位置消费；未在状态1城镇执行补救。

成功艾尔文会话roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_150641_992519_next37，PID22348/runTick7840796：三个原生登记result1，sceneVectorMatches[1,1,1]，主人CRef与玩家一致，objectId[0,1,2]、controllerId[65535,65535,65535]。日志三张虚表14A59DDC0/14AD21370/1495AD718的+432均为145B960D0，+632均为145C70EE0，+8464均为145CF6C20。actor+400场景弱引用为0，但实际成员登记成功，故不得用该弱引用单独拒绝或认定未登记。

## 实现与策略归属

catchup-policy.h定义用户指定的补救策略：距离超过800地图单位持续3秒，每250ms采样，附近横向偏移64单位。没有把该策略写成PVF原生规则，也未增加副本、角色或内容ID清单。已有Type22/原频道ID10等重复策略继续留在docs/todo/adventure-elite-ordinary-plan.md，不在本轮收敛或扩展。

catchup-adapter.h只hook原生玩家x坐标getter；原函数仍执行并保留结果、LastError与SEH。补救只在既有加载完成所记录的主线程、冻结主人和频道一致时执行。当前玩家两种原生引用必须一致；只有唯一type5精锐、校验有效的独立对象编号、正强引用、主人绑定与控制器绑定一致且恰好登记于当前场景才能执行。死亡、悬空、loader执行期间、城镇及虚表不匹配均跳过。世界/场景/玩家/槽位/对象/loader周期变化或采样中断重计时；传送前再次检查对象引用、当前场景和主人。落点检查失败尝试玩家当前点，再失败则等待下一周期。调用原生立即坐标setter，不发送包，不写对象ID、管理器状态或场景CRef。

日志与状态解析自身DLL目录，新增apcCatchupReady及每PID/runTick的catchup日志。最多1024条诊断不影响补救。状态和采集结果只能说明候选初始化/原生调用观察，apcCatchupVerified始终false，直到用户手动确认。

## 验证和交付

scripts/build-adventure-elite.ps1实际退出0，MSVC x64 /MT /W4 /WX构建DLL，14组机制/原EXE字节安装夹具通过。新增距离边界、3秒持续、冷却、身份与场景变化、采样中断/倒退时钟测试；当前EXE八处字节逐项破坏均拒绝，正确现场安装成功；原getter结果/LastError/SEH保持，传送在日志耗尽后仍有效；城镇、死亡、缺登记、重复登记、外部主人、错绑定与未知setter均拒绝。没有Go改动，本轮未执行Go build/vet/test，未覆盖已有服务端/默认程序，也无PVF或SQLite/schema迁移。

DLL为233472B，SHA256 ac794170d723e58314a37b5bb9871e33e7ca0d96d28c9195b0b1d47a562fa7bd；schema2包依赖qol.client-host，dist/adventure-elite-eligibility-0.3.11.zip。DLL和候选清单原文件备份.tmp/adventure-elite/apc-catchup-backup。构建、14组机制测试与只读候选预检通过；日志在.tmp/adventure-elite/apc-catchup-{build,checkonly,capture-test}.log。scripts/test-adventure-elite-capture.ps1全套退出0，既有采集版本与新增0.3.11的日志复制/传送观察/异PID拒绝/不伪造实机确认夹具通过。CheckOnly退出0，正确识别最后旧进程仍为0.3.10，未当成新候选运行。

本机未定位到modkit.exe或独立启动器仓源码，因此未执行modkit verify/plan/install/uninstall，不宣称该链已验证；schema2打包已完成，保留既有Job直接注入的已验证启动路线。没有向实际client目录安装包，也未无人值守启动客户端。modkit安装、卸载逐字节还原仍是交付限制，不能以自编解压脚本冒充原工具门禁。

## 未闭环与手动下一步

没有无人值守启动或操作客户端。由用户手动重开scripts/启动游戏-SQLite.cmd --source-build，确认本次PID状态apcCatchupReady，再进艾尔文换一层并等待数秒。日志区分sample坐标、teleport目标/结果、not-current-scene与ambiguous-scene。原生getter实际轮询频率、艾尔文内部地图坐标与修正效果均待此live确认。若剧情转场后APC未登记在当前场景，当前候选会拒绝；必须取得原生离场和重新登记消费链才能安全扩展，不能宣称已修复所有剧情演出。
