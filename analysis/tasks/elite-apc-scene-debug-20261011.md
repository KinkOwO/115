# APC 艾尔文换层 x64dbg 取证与 v0.3.13 候选

用户明确指定 x64dbg，并手动停在下一层。补救 attempt 3/3；v0.3.12 与 v0.3.13 均为失败 checkpoint。v0.3.13 在 PID8232 中因锚点使用加载器上下文、采样使用全局世界对象而持续 unanchored-scene；详细失败与业主明确授权的 v0.3.14 来源修正见 elite-apc-anchor-source-fix-20261011.{md,json}。以下保留 v0.3.13 构建与交付时的历史记录，不作为实机验收。没有自动启动/操作游戏、修改网络包/Go/PVF/SQLite/schema、保存权威IDB或提交候选。

## 当前实机证据

- DFO.exe 基址140000000，PID7996/runTick5878656，原生主人线程12656。AdventureElite.dll v0.3.12已加载、各hook ready。旧catchup JSONL先started/入口，随后持续world-or-player-ineligible，无A*或传送记录。
- 用户确认停在下一层后，world=04AB2B00，world+260/264为1F2A025B/1F2A0322；原生146E920A0解码(encoded XOR 1F2A025C)-4=3，checksum=encoded+3+196。此前用户回城样本为1F2A0259/1F2A031E，解码1。1459A90F0的override虚表+96仍为1402B4780返回FFFFFFFF，因此走该解码分支。旧DLL把副本状态要求为8，两处错误门禁阻止所有寻路。
- 当前global14E6839F0=137E1DF80（回城时的旧指针137EC4100不能复用）。当前world+288对象138050D00的+1764为FFFFFFFF，native scene getter退回当前override的virtual216=144ED9C60，返回+144对象137BFF040；该对象+264/+272 CRef=(17AE10AC0,195C52B00)。scene与本次首房loader-after一致：6807694080=195C52B00。
- scene+88/+96/+104=(1866201E0,186624470,186625FA0)，读取17040B即710个24B CRef。玩家(control12C6C3CA0,alias106730EF0)恰好1条，index519；三个APC精确CRef均0条。不是凭actor+400为空判断缺登记。
- manager=15FA790C0，+72/+96/+120三槽为(16C183CE0,109754970)、(16E7C5020,1067BABF0)、(16FA89140,10973A230)。三control+8 strong=1，actor=alias-48，三actor+70914 dead=0。管理器持有活对象，但当前场景引用已经缺失。
- 玩家actor106730EC0的dead getter虚表+8464=145CF6C20，dead=0，位置组件12C9FAB00可读。位置core12F2D3A00按145E08B90读取+105/113/121原生编码float，X约485.316/Y300/Z0；第一APC位置core16B631000约856.371/347/0。当前native grid宽84/高40、tile18255B040，两方坐标均在16×12网格范围内；这本身不证明有路或无路。
- 服务端会话roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261011_020233_195461_next37：本次entry_serial2，首房三名登记result1、sceneVectorMatches[1,1,1]；events.jsonl 401行左右CMD2015艾尔文传送提交，后续没有新的首房登记日志。源脚本contents/2022/elvenmere/dungeon/elvenmere.dgn、SHA6a27fc89a79ff2095f35d78712dc7f50979324ba170d489730888fd52763c637。保留原生地图/PVF规则，不新增副本表或玩法数据。

## 原生登记闭环及候选范围

145B25278..145B25298汇编明确八参数(scene,0,5,id,actor,0,0,1)调用145DE4360。包装仅kind>2 tail jump145DE4AF0；反编译错误推断十二参数/varargs，不能据其改ABI。145DE4AF0完整1058行已读，复制actor+48原生CRef入scene+88主向量和类型/身份索引，并执行原生初始化。virtual304的APC thunk145CE24B0→145B8B120读取actor+400/+408 scene弱引用；零值不能替代成员向量检查。145DE66F0是父对象相关处理，不是通用旧场景离场函数；parent=0早退，不能冒充跨场景清理证据。

候选修正状态3，拆分拒绝日志、记录worldMode。原生精锐首房登记实际返回1后保存world/scene/主人CRef/精锐CRef/objectId锚点；只允许同一场景、同一对象代次缺登记持续3秒恢复。检查kind5编号冲突及不稳定向量，调用已校验的原生登记trampoline，复查成员=1和主人/控制器/存活身份后用原生落点检查+立即位置setter。没有直接写CRef/向量/索引。已登记对象仍走原生A*确认无路3秒；缺登记是单独的实机信号，不能标为原生A*结果，两套计时独立。

陌生scene、未观察成功登记、重复成员、外部主人、死亡、未知绑定拒绝。场景对象指针变化的剧情演出离场/重挂未闭环，候选跳过并记录unanchored-scene。登记原生初始化对APC战斗状态连续性的影响待实机，不能宣称全部剧情换场已修好。若attempt3仍失败，保留日志并停止增加无依据运行假设。

## 验证与交付

实际执行scripts/build-adventure-elite.ps1 -CandidateDirectory .../dist/scene-candidate-0313，退出0；MSVC x64 /MT /W4 /WX，14组机制测试全部通过。新增直接执行当前EXE的世界getter/decoder，验证城镇1/副本3；真实A*向量沿用并通过。恢复夹具验证首房登记锚点、缺成员不足3秒不动、恰好3秒原生八参数调用、恢复后不重复、其它场景拒绝、kind5身份冲突检查和日志预算耗尽下行为。登记原函数在夹具中以ABI stub代替，仅实机才能确认完整原生恢复/显示/AI效果，不把它写成已验收。

随后MSVC重编译最终DLL以纳入状态说明，退出0。采集脚本测试、最终产物哈希与只读CheckOnly结果记录在相邻JSON；Go未改，本轮未执行Go build/vet/test。-CandidateDirectory和打包--dist限制在补丁ignored dist中，避免覆盖正在加载的旧DLL。用户手动退出后才替换默认注入路径。modkit工具仍未找到，verify/plan/install/uninstall未执行。

回退为.tmp/adventure-elite/scene-registration-backup下v0.3.12 DLL/构建清单；此前v0.3.11、v0.3.10备份仍保留。日志只落DLL目录；采集文件留runtime/adventure-elite-captures，不入库。没有更新CHANGELOG/confirmed baseline、没有暂存/提交，未触碰其它作者Go文件。

由用户手动运行scripts/启动游戏-SQLite.cmd --source-build，保留名单，进入艾尔文换一层等待3～5秒。日志应出现registration-anchored→registration-missing→registration-restored→teleport；有无效果均保留本代次日志，无需重复保存名单。当前候选待该次实机验收。
