# APC 登记锚点地址来源修正 v0.3.14

实机更新：业主报告 v0.3.14 仍失败。PID2012已执行282次登记恢复与坐标更新，全部即时坐标符合目标，但登记反复消失。该候选不得标为已确认。业主授权继续追踪原生移除链；本次客户端自行崩溃且未取得解绑命中/异常现场，详见 [原生移除链取证](elite-apc-native-removal-trace-20261011.md)。下面的构建及候选交付记录保留为历史事实。

v0.3.13 已由业主手动验证失败。按根 AGENTS §0.1 停止新增运行假设、报告具体缺口后，业主明确答复“继续按这个范围修正”：限定核对加载器参数与全局世界对象的原生关系、修正锚点来源，并补两个地址不同的测试。接续补救 attempt 3/3 的已定位实现错误修正，不重置次数，不宣称新的实机确认。

## 失败与只读动态证据

DFO PID8232/runTick8251828，DLL v0.3.13、主人线程14780、全部hook ready。catchup日志21～23行，tick8575312/8575328，三名 registration-anchored；首房 native-registration-result 全部1、sceneVectorMatches [1,1,1]。34行起 tick8584109，三槽持续 unanchored-scene，没有 registration-missing/restored/teleport。首房只有 path-reachable 样本，不能声称无路已触发或原生恢复调用失败。

x64dbg 正在运行，全程只读，没有暂停/断点/内存写入。AdventureElite.dll 基址7FFA930C0000。按scene/玩家CRef模式扫描自身.data区，恰好3个匹配；读取7FFA93166220连续168B，按3×56B生产 EliteCatchupKey 解出：

| 字段 | 三个锚点 / 当前采样 |
| --- | --- |
| world | 锚点均17AB70080；global14E66C090当前04B62B00，不相等 |
| scene | 均17E554F00，与首房日志一致 |
| player CRef | 均13BA0AE80 / 8B40EF70，与当前14EF2CA70一致 |
| APC CRef | 1761010E0 / 1005F63F0；1786DC780 / 10664FA70；1799192E0 / 106FB6330，与当前manager16C516080三槽一致 |
| objectId / room | 0/0、1/0、2/0，与生命周期当前编号一致 |

world状态word1F2A025B/checksum1F2A0322，原生解码3；当前override1558B1F80→+144对象155683040→+264/+272 CRef13AA840C0/17E554F00。scene+88/+96/+104=1514940E0/151497A40/151499EA0，读取14688B即612条24B引用，玩家精确引用1、三精锐各0。完整场景名单不入库。

生产 EliteCatchupSame 比较的其它字段均相同，仅world地址域不同。EliteCatchupRegistered旧版直接保存首房回调传入g_entryWorld；采样从14E66C090读取world。错误在来源混用，不能删除世界身份门禁来掩盖。既有离线夹具把entryWorld同时塞入全局世界地址，因而漏测。

服务端会话 roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261011_024147_845047_next37。实际执行collect-adventure-elite.ps1 -SessionPath该绝对路径，退出0，日志保留runtime/adventure-elite-captures同名目录；6个战斗采集缺口不作为补救验收。源仍为contents/2022/elvenmere/dungeon/elvenmere.dgn，没有新地图/玩法规则或PVF改动。

## IDA 原生关系与修正

权威DFO.exe.i64会话aae19eba只读反编译：146D29700的146D2A08F调用14520CFB0获取副本上下文v79，146D2A1E2将v79传给145B22F50；其+6792字段是本轮首房登记已核实的副本对象。145B22F50独立读取global14E66C090，在145B235AE等处调用1459A90F0获取世界状态，不能把首参当全局世界对象。加载器选择的注册scene来自自身场景CRef列表（145B23531..573），因此锚点保留原生注册实际传入scene。1459A9260→1459A9180→145E930B0则读取全局当前场景上下文+264/+272，供后续采样核对。

v0.3.14把登记回调首参命名loaderContext，仅用于事件证据；成功原生登记回调内稳定读global14E66C090两次，绑定与采样相同的world身份。仍要求完整scene/玩家/APC CRef/objectId一致；世界缺失/替换、陌生scene、死亡、未知主人、重复或编号冲突仍拒绝。schema2增加world/anchorWorld/loaderContext诊断字段。三秒策略、原生A*、登记八参数、原生位置调用均沿v0.3.13；不新增其它副本或跨场景逻辑。

## 验证与交付状态

先把测试全局世界设为独立catchupWorld、加载器仍传entryWorld，并要求锚点保存全局来源：旧实现编译成功，执行出现catchup fixture16并非零退出；修正后同夹具退出0。进一步将全局世界换成另一份同状态/频道对象，验证恢复仍被拒绝；无全局世界、场景变化、不足3秒、恰好3秒恢复、恢复后不重复及预算耗尽行为均覆盖。原生登记原函数仍是ABI stub，完整显示、AI与战斗连续性只由实机验收。

实际执行build-adventure-elite.ps1 -CandidateDirectory .../dist/scene-candidate-0314，MSVC /W4 /WX /MT，退出0，全14组机制通过；增强“实际替换全局对象”测试后重编并重跑registration-startup-test.exe，退出0。采集兼容测试和只读预检最终结果及产物哈希见相邻JSON。Go未改，本轮未执行Go build/vet/test；modkit尚无工具，verify/plan/install/uninstall未执行。

候选DLL只在ignored dist/scene-candidate-0314构建，不覆盖运行中的v0.3.13。备份.tmp/adventure-elite/anchor-source-backup含v0.3.13 DLL、schema2包和构建清单；最终替换状态见JSON。用户退出后才复制到默认注入路径，机器状态日志不伪造新版本运行。用户手动启动scripts/启动游戏-SQLite.cmd --source-build，换一次艾尔文层等待3～5秒；成功或失败都保存本进程日志，无需重新保存名单。异场景剧情迁移仍未闭环。没有更新CHANGELOG/confirmed baseline、没有暂存/提交；其它作者Go改动保留。
