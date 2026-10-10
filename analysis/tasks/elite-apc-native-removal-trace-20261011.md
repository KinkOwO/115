# APC 原生移除链取证：v0.3.14 实机失败

业主报告“这次也没有成功”，随后明确授权“继续追踪原生移除链”。范围是恢复登记后由谁移除 APC，以及当前渲染/AI 场景关系；接续 attempt 3/3，不重置次数、不新增猜测性运行路径。

## 已取得的实机证据

PID2012/runTick9432796，DLL v0.3.14，主人线程21480。catchup 日志达到1024条诊断上限：registration-anchored=3、path-reachable=3、registration-missing=428、registration-restored=282、teleport=282。全部282条 teleport 的 after 与 target 完全相同；没有登记恢复拒绝或落点拒绝。首次恢复 tick9740265，槽0从857,337更新到653,300；玩家717,300。后续仍反复恢复登记，因此原生登记和即时坐标调用确实执行，但不能证明 APC 留在场景、可见或 AI 正常。

首房锚点与当前 world 均04A72B00，scene均17FBEB900；v0.3.13地址域混用已不再阻止恢复。manager16B8B6080中三名 APC actor为AEF3E780、101E718C0、110A9A200，control/alias分别173039280/AEF3E7B0、174634780/101E718F0、176C58960/110A9A230。原生身份0/1/2、主人4、强引用1、绑定和主人匹配均保留。首次只读采样 actor0+392/+400/+408全零；scene的CRef control17FBC7FA0 strong=1，当前全局场景链仍指向该scene。最后生命周期 tick10174765 仍显示三名 APC 的 sceneAttached=0。

日志在 client-patchs/adventure-elite/dist/adventure-elite-{catchup,entry,lifecycle}-2012-9432796.jsonl。服务器会话 roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261011_030151_053071_next37。collect-adventure-elite.ps1 对该会话实际退出0；六项战斗采集缺口不作为补救验收。

## 权威 IDB 静态链

DFO.exe.i64会话aae19eba，只读，无IDB改名或保存。

- 145DE4AF0：登记时先处理旧scene，再调用actor虚表+296 SetScene，复制完整CRef进入scene+88对象向量及其它索引。
- 145B97400：SetScene构造scene CRef并复制到actor+392的24字节引用；不是直接写指针。
- 145DFFF70：场景移除函数，清理对象向量与索引，并在145E00ECB调用actor虚表+296(actor,0)，清空场景CRef。
- 145DFF4D0：处理scene+544的待清理引用，也遍历scene+88。actor虚表+328返回0时先收集该对象，再按原生分类/状态分支调用145DFFF70。当前APC虚表14A59DDC0的+328指向14449BF20，读取actor+720；+320为141854970→14449C130，写actor+720。静态证明该字节参与原生保留/移除判断，尚未读取失败瞬间的值或命中调用来源，不擅自置1。
- 145E00F10：按调用者提供的flags掩码遍历并移除匹配对象；具体本次掩码未取证。
- 145B960D0：坐标setter只调用三轴位置组件写入及调用来源诊断队列；没有直接场景解绑调用。仍需区分间接状态清理与原生逐帧移除。

## 中断与崩溃边界

x64dbg初始running，原断点列表为空。尝试在145B97400设置软件日志断点，条件rcx==AEF3E780、break_condition=0、silent=true、fast_resume=false，记录scene、返回地址、线程及栈槽。工具set/configure报告成功，但随后get返回404，state转为stopped，memory返回无活动会话；导出日志只有初始化文本，无APC命中。最终断点列表为空。没有调用pause/run/stop/restart，没有写游戏字段，没有代替玩家操作。

Get-Process确认DFO已退出。业主明确回复“自行崩溃”。不能据时间顺序断言断点导致崩溃，也不能排除相关性；本次没有取得异常码、故障地址或调用栈。读取近一小时Windows Application 1000/1001没有DFO/x64dbg事件；client目录没有新的dmp，LocalAppData/CrashDumps只有9月19日旧dump。没有崩溃定位闭环，暂停实机断点并告知用户暂不重进。

## 后续限定范围

必须在新进程重新核对manager、APC、玩家和场景地址，旧PID地址不得复用。先只读比较actor+720及场景成员，核对当前渲染/AI上下文。需要动态断点时限定原生移除入口和这三个APC，验证断点确实存在；保存现场返回地址和栈，并保留异常停止证据。不得直接清待删除队列、强设+720、禁用原生清理或扩大恢复规则。

前一阶段仅分析/记录：未改DLL源码或构建产物，未执行新的build/机制测试；未改Go，因此Go build/vet/test不适用。git diff --check为该阶段记录检查；没有提交或更新CHANGELOG/confirmed baseline。该阶段原生移除调用来源、渲染/AI连续性与崩溃原因均未闭环；下一节是用户重启后的新证据。

## 接续：PID15920 原生移除命中与 v0.3.15 修正候选

业主手动重进并回复“已停住”。新PID15920/runTick10822437，v0.3.14全部hook ready，主人线程22620；没有复用PID2012对象地址。全局world049C2B00，manager1694BE080，玩家control16F8CDE40/alias1066FEF30；原生首房登记scene17C0AD600，玩家和三APC登记首房各1。APC actor117BAA480、1364E6A00、63E601C0；控制块1841B9520、187784BC0、187D9D2E0，alias各actor+48。三APC flags348均01110001，kind5、编号0/1/2、主人4，actor392的场景CRef全零，actor720均0；玩家720=1、scene alias408=17C0AD600。APC0 dead70914=0、control strong=1。三类APC虚表不同，但320均141854970、328均14449BF20。当前scene归属和标志只证明清理条件，完整渲染/AI验收仍待候选实机。

硬件执行断点145DFFF70实际命中一次：RCX(scene)=17C0AD600，R8(actor)=117BAA480，byte(actor+720)=0，RSP=005FF7F8，return=145DFFCDA，tid585C即22620。调用栈返回链145DFFCDA→145DFD85C→145E9C4F2→146CE04AC→1459A697A→146DE4B9A→148860FA2。与静态145DFF4D0中的145DFFCD5移除调用对应：virtual328=0被收集后，分类111分支和virtual176=0路径移除。这次取到的是原生场景更新清理调用，栈内没有DLL恢复调用；不是仅依据“登记后来消失”推断调用者。

工具边界：set_hardware/configure报告成功，但get实际显示条件/日志/name全为空、silent=false、hit_count=1，客户端处于该硬件断点暂停。立即读格式化参数和调用栈，delete硬件断点，再run；state running、断点列表为空。没有软件断点或游戏字段写入。配置接口报告成功与实际不一致，后续不能据success假设非停顿日志已配置；本次有一次短暂停顿。稍后日志worldMode=1且旧scene被释放，停止读取旧地址。没有证据归因返城原因；不能混用返城后的内存作为失败房间场景状态。

IDA和运行时字节同时确认原生加载器145B253B7..145B253BC：成功登记后的virtual320(actor,1)，机器码B2 01 48 8B CE FF 90 40 01 00 00。141854970的E9 BB 77 C4 02跳转14449C130；后者88 91 D0 02 00 00 C3只写actor720；14449BF20的0F B6 81 D0 02 00 00 C3读取同一字节。此前补救复制了登记八参数，却缺这个原生后续启用步骤。

v0.3.15只在已有完整锚点/身份/存活/主人/场景门禁、缺失持续3秒并原生登记成功后补该调用。验证虚表320/328精确指向已核实原生函数、四处原生字节正确；未知即不补救。已有启用值非0时不调用setter；之后原生getter必须返回1，且上下文和场景成员仍正确才传送。已有场景成员的A*路径不改变；不禁用原生清理、不直接改字段/队列/控制器。schema2新增retentionBefore/retentionAfter和registration-reactivated/reactivation-rejected诊断，日志预算不影响恢复。异scene剧情重挂仍未实现，不声称全部剧情解决。

离线回归使用当前DFO.exe的启用thunk、leaf和读取函数，在本进程执行，验证只改变actor720；恢复前0、恢复后1，已有1保留，未知setter/getter拒绝，原有世界/身份/场景/存活等拒绝仍过。隔离回归副本只去掉本次生产启用步骤，编译退出0、执行catchup fixture26/exit33；修正版本同一夹具通过。登记本身仍为ABI stub，完整清理/渲染/AI未离线模拟。

实际build-adventure-elite.ps1 -CandidateDirectory .../dist/scene-candidate-0315退出0，MSVC /W4 /WX /MT，全14组机制通过；DLL245248字节，SHA256 8180378b06d0a106a95e75835bd4580a4d07d580d6f378a5629556a5f69244a5，包150451字节。构建日志.tmp/adventure-elite/native-retention-build.log；回归日志native-retention-regression-{build,run}.log。采集PID15920会话032455_360956_next37退出0，六项既有战斗缺口仍不代表验收。采集兼容测试、预检与最终替换状态见相邻JSON。Go/PVF/存档/默认服务程序未改；未执行Go build/vet/test，没有新实机确认、CHANGELOG或commit。

最终交付：采集兼容夹具退出0，包含v0.3.15新诊断字段保留和禁止自动验收断言。隔离预检退出0，三项产物/四项资源哈希全匹配。用户回复“已退出”，确认无DFO进程且默认DLL仍为已备份的v0.3.14哈希后，复制候选DLL、schema2包和清单到默认dist目录；逐项哈希匹配，再次默认预检退出0。机器状态文件仍保留真实上次PID15920/v0.3.14，不伪造v0.3.15运行。回滚目录.tmp/adventure-elite/native-retention-backup-0314；源码/脚本/文档变更见JSON。用户手动启动scripts/启动游戏-SQLite.cmd --source-build，换一次艾尔文层等3～5秒；读取新进程registration-reactivated的0→1及后续登记持续性，并由用户确认可见/AI跟随。

## 收口：v0.3.15 艾尔文换层 APC 出现已确认

业主确认“确认APC出现，可以提交了”。新PID21376/runTick11640906实际加载v0.3.15，原生主人线程17096。确认记录为elite-apc-catchup-confirmed-20261011.json，区分用户可见性确认与采集事实。

033840_888022_next37采集退出0，产物/资源匹配。首次三轮换层九次activation均0→1，teleport目标/复读坐标全部匹配；catchup日志启用行34/37/40、95/99/103、130/133/136，传送行35/38/41、96/100/104、131/134/137。后续生命周期ordinal263/tick11984468仍sceneAttached、weakStrong、controllerBound、ownerMatchesCurrentPlayer各[1,1,1]。采集持续期间用户继续操作，完整catchup副本另有后续记录，确认基线保留上述固定窗口，避免用滚动日志扩大验收。

六项战斗采集缺口保留，含缺run.json、战斗身份/预算/运行改写门禁、原生死亡/CMD39、来源归属证据等；登记与坐标数据不代替完整战斗验收。不同scene剧情重挂、完整死亡/AI连续性、PID2012崩溃原因及modkit安装卸载仍未闭环。CHANGELOG、README及计划更新本次确认；前文失败/候选准备是历史记录。远端合并与提交前Go结果见确认基线，不将此前未跑Go描述替换为假通过。
