# APC换层漏一名：原生virtual632派生实现兼容候选

用户报告“进入下一层时三个APC只剩两个，还有一个没传送过来”。不扩大上一轮确认范围；v0.3.15的PID21376确认保留为历史实机基线，本次PID23648/runTick28766359是新失败样本。沿已授权DLL任务只读取证，未暂停/设置断点/写游戏字段，未代替玩家操作。

## 实机原因

catchup日志slot0自第26行/tick28942859持续dead-or-unsupported-actor；slot1第46/130行和slot2第49/133行各有两次原生恢复与传送。生命周期ordinal118/tick29058890显示sceneAttached=[0,1,1]，三者strong、主人与controller绑定均1。失败精锐已从场景移除，却仍有管理器引用。

x64dbg确认PID23648、EXE基址140000000、manager16ACD2080、world04A42B00。三个actor10A4DEFC0/10AE14300/10AE2E940，死亡原始标志70914均0，坐标组件均有效；启用virtual320、读取328、坐标setter432、死亡getter8464全部匹配旧门禁。唯一不匹配项为slot0的virtual632=1463DE5D0，而另外两者为145C70EE0；旧代码只接受后者，因此在原生恢复前拒绝slot0。不能把合并日志理由当作APC已经死亡。

IDA会话8bcb56cd为权威client/DFO.exe.i64。槽0虚表14AD105F0+632=14AD10868确实引用1463DE5D0；原生函数361字节与运行时逐字节一致。派生实现依据virtual328启用值处理原生附属状态/对象，检查virtual5432(actor,16)，相应分支按virtual536返回状态5许可，否则把原四参数传给145C70EE0。通用实现先145C70E00判断，再145CF6910检查；不能用通用实现替代派生状态规则。

## 单一修正与验证边界

v0.3.16仅接受两个已经核实的原生virtual632地址，逐字节核对完整派生函数，并在调用时重新读取实际虚表槽分派。未知槽继续拒绝，保留死亡、身份、主人、坐标、锚点、场景与3秒门禁；恢复启用可能改变派生许可，所以传送前再次原生检查。仍不实现异scene剧情重挂，不直接改字段/队列或禁清理，不改PVF/Go/数据库/schema。接续原记录attempt3/3，不重置历史计次。

回归先仅加入派生槽兼容断言，旧生产代码编译退出0、运行catchup fixture28/exit33。候选进一步在本进程执行当前EXE完整派生函数及真实通用fallback，覆盖原生许可/拒绝、原参数转发、未知槽拒绝、同场景缺登记补救，以及启用后原生状态拒绝。复杂附属对象与通用地图依赖在离线测试中是ABI stub，不能冒充完整AI/战斗验证。隔离构建输出dist/landing-candidate-0316，旧默认DLL与包保留；结果及产物身份见相邻JSON。

采集223839_809929_next37退出0，保留七项战斗采集缺口。新候选尚待手动实机，未更新CHANGELOG/confirmed baseline或提交。DLL日志仍写自身目录；替换必须等待用户退出当前客户端，备份并核对旧v0.3.15后再复制候选。

## 候选交付状态

用户回复“已退出”后核对无DFO进程，已将默认v0.3.15 DLL、包与构建清单备份到`.tmp/adventure-elite/derived-landing-backup-0315`，核对旧DLL哈希后复制v0.3.16三项产物，逐项校验复制前后哈希相同。默认DLL现为245248字节，SHA256为549aee68f70392a65bb4bd97f296e758279e3a7d88ce262821d813f405107d0c。

隔离构建退出0，14组机制测试通过；采集兼容测试退出0，候选预检与替换后默认预检均退出0。门禁`check-commit-hygiene.ps1 -All`退出0，`git diff --check`通过。没有修改Go，因此未运行Go build/vet/test。modkit verify/plan/install/uninstall未执行。实际日志路径与产物身份见相邻JSON。

未启动或代玩客户端，状态文件仍保留真实旧版v0.3.15/PID23648记录，不能代表新候选已经运行。请用户手动通过`启动游戏-SQLite.cmd --source-build`重启、换层后等3～5秒，确认三名APC是否都出现；待新实机结果后再决定收口。跨scene剧情演出、完整AI/战斗仍未闭环，未更新confirmed baseline或提交。

## 本次实机确认

业主确认“确认修复，可以提交”。新会话225911_434322_next37、DLL v0.3.16/PID20908/runTick29997781：slot0/1/2各两次恢复、原生启用0→1及传送，6次目标与即时坐标全部相同；末次全员登记样本ordinal136/tick30526234为sceneAttached、controllerBound、ownerMatchesCurrentPlayer均[1,1,1]。其后的全员离场样本保留，不把离场当成全场景恢复或完整AI验收。新确认基线见相邻elite-apc-derived-landing-confirmed-20261011.json，以上候选待验收文字保留为历史步骤。提交前已合并gud/main，合并后门禁结果继续记录于确认基线。

收口采集退出0、candidateValid=true，保留六项缺口（包括run.json缺失及死亡/击杀归属样本），battleVerified=false。合并后DLL隔离重建与14组机制退出0，产物245248B、SHA256 6eab970b7bfbca955416e37ddd359087c0955d91c8f82edeba65252ae3b57421；这是离线重建产物，默认已实机DLL仍549aee68，未替换。构建元数据哈希分别记录，不混同实机身份。

合并后Go1.26完整工作树go build ./...、go vet ./...、go test ./... -count=1 -json均退出0，4279通过/278跳过；逐名与upstream-elite-close-gates.json零失败基线比较，无既有或新增失败。采集兼容测试退出0。未构建或替换现用服务端/启动器。
