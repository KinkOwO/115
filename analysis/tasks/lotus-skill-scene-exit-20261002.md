# 任务3215：技能请求误触发末层出口与崩溃候选修复

状态：attempt 3/3，用户已手动确认可以触发结算并完成任务，客户端不再崩溃；本修复纳入confirmed baseline。用户未单独报告黑屏画面是否仍出现，当前确认范围以结算、任务完成和不崩溃为准。此前attempt 1/3、2/3及旧验收边界见 `lotus-terminal-layer-20260923.md`。

## 本次实际证据

会话为 `server/work/dfo-lan/runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261002_212301_725243_next37`，角色10。以下日志时间转为北京时间。

- 21:44:17.941，客户端原生CMD45进入副本26、迷宫3、末层100008697，收到NOTI29；21:44:18.461发CMD37加载完成。
- `events.jsonl:4336` 的末层演员包含70160、109015585、109015474、75099及动态同行APC。原生PVF直读表明70160、75099均有`[displayhuntdummy] [boss]`，另外两个演员team0，因而服务端将此房识别为无战斗演员的演出房。
- 21:44:23.080，`events.jsonl:4371` 为CMD38，明文 `000500a00f9045000c000089ee307800`。opcode真源及客户端TRACE同时确认它是`ENUM_CMDPACKET_USE_SKILL`，技能5；不是客户端CMD45演出结束请求。
- 服务端却紧接着发送ACK38、ACK45和NOTI29；`events.jsonl:4376`明确记录 `scene exit: [3 0] layer=100008697 -> base=53543`。NOTI29的layer flag=1、mode=1、map=53543。客户端TRACE确认接收后切到了53543。此时没有末次原生CMD45，不能将技能使用解释为结尾过场完成。
- 21:44:24客户端生成崩溃报告，`client.log`最终退出码`0xC0000005`。TRACE CallStack含 `0x145c34e0c -> 0x145dc86bf -> 0x145c3beee ...`；权威IDB中对应`sub_145C34C60`，在Boss死亡/确认路径取得演员相关对象后解引用`*v15`。不是静态猜测NOTI29空descriptor即为本次实际崩溃点。

以上证明服务端确实在技能请求后错误改变了演出房及演员生命周期；这条切图与紧随其后的Boss清理崩溃一致。仍需实机确认阻断该路径是否消除本次完整症状。

## 资源与分支边界

实际启动器`server/launcher.local.json`指向`F:/wip/dof/115US`。实际EXE、sk.dat、外层PVF及服务端内层PVF均通过现有manifest哈希核对；本次没有修改任何客户端资源或DLL。

内层PVF SHA256为 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`。副本26迷宫3的boss位置仍为(3,0)，层序列仍为[100008786,100008697]。53543、100008786、100008697三张地图的原始脚本SHA256与历史`configs/dungeons.full.json`相等（历史JSON只作审计对照，不接入运行期）。末图ACT14948在70160不再存在时播放14949；CMT14949末场景是落点X=701..704、Y=229..231的`[CHANGE MAP]`。没有缺层图证据。

当前运行EXE与仓库client/DFO.exe的完整大小不同，因此额外逐字节核对了运行EXE的`0x145c34e06`和`0x1452b7793`各24字节：均与权威IDB完全一致，分别覆盖崩溃解引用附近及NOTI29 layer分支。IDB沿用现有session `2e6257e3`查询，没有重新打开或修改数据库。

本地legacy指向`da3d36f`，本次源码HEAD为`26317fd`。legacy与HEAD的`internal/dungeon/scene_transition.go`相同，`interactDoor/sceneExitRequest`关键路由也相同；不能仅凭分支名称认定是哪次重构新增该问题。用户初述legacy只黑屏且任务完成，后续补充两套客户端均崩溃；本轮以当前实机日志确定的错误路径为修复对象。

## 候选改动与验证

仅在`internal/dungeon/scene_transition.go:ExitSceneRoom`拒绝副本26/迷宫3/末图100008697的服务端合成出口。已有调用方对拒绝保留ACK38；不发ACK45或NOTI29、不改变当前房间。ACT/CMT自主发送的原生CMD45仍走已有精确记录校验、末层mode0缓存复用和加载后完成链。没有增加封包布局、尝试新codec、改变通关时点或提前伪造Boss死亡。

`cmd/wireprobe/lotus_skill_exit_test.go`使用本次真实技能明文及原生地图演员形态：改动前复现返回53543；改动后只ACK38并保留末层，随后准确CMT记录仍返回layer flag1/mode0并在加载后完成。相关Lotus、场景出口及目标位置回归通过。完整候选测试与vet结果记录于任务临时目录；全量测试保留经改动前overlay复现的3项PVF审计失败及商城`empty delivery 3400013`，不称为全量通过。

工作区原有誓约改动`dungeon_flow.go`和后来出现的`oath_direct_entry_test.go`未编辑；独立候选通过Go overlay使用本轮HEAD的flow及空的其它任务测试文件。正常工作区测试通过此项守卫；隔离候选测试不将另一项未提交工作混入本轮。初次隔离遇到其它任务新测试与HEAD flow不匹配的panic，随后将那份未提交测试一并隔离；未改那项任务代码。

独立候选：`.tmp/lotus3215-20261002/wireprobe-lotus3215.exe`，SHA256 `d000ea82f278a48b9b89a6cd70fa0cfaa9e6ac12609883ffc86781c9540adbd7`。同目录`pvf-lotus3215.json`沿用默认PVF策略，仅指向该程序；`启动验证.cmd`通过根标准启动器启动。准备检查`--check --server-only --repair-profile <候选profile>`通过，没有启动客户端。原默认PVF和源码入口未替换。无schema、SQL、玩家存档或数据库操作。

## 手动验收及回退

用户于2026-10-02确认：可以触发结算并完成任务，客户端不崩溃。验收对应独立候选`.tmp/lotus3215-20261002/wireprobe-lotus3215.exe`（SHA256 `d000ea82f278a48b9b89a6cd70fa0cfaa9e6ac12609883ffc86781c9540adbd7`）。

回退只需关闭候选会话，使用原根入口；默认二进制未覆盖。源码单点回退备份在`.tmp/lotus3215-20261002/before/scene_transition.go`。本轮是该功能第三次运行路径候选；若仍未闭环，停止试包，保留新日志并请用户指定新的取证范围，不进行第四次无依据运行路径调整。
