# 巴卡尔NPC库存同步候选与终幕取证（2026-10-07）

用户授权开始修复已知问题。本轮运行改动为NPC库存恢复，attempt1/3；最终动画自然结束的原生条件尚未闭合，保留源180秒兜底，不修改客户端、PVF或猜测动画通知。

## NPC同步改动

当前13B N2288 reader与库存消费布局已闭合，源库存实际2～4而截图0。进一步只读当前客户端发现`1425470c0`重置五个库存项（vtable1499e1c80+10），HUD初始化`142555ee0`明确将五个显示标签设为0。静态证据证明存在独立缓存/显示初始化，不单独证明本次画面具体在哪一次调用清零。

现在房间第一次加载完成、原有角色/装备/房间初始化计划之后，发`BakalOpening.BuffInventorySnapshot`，复用现有N2288、当前owned库存、used kind25及targetffffffff。既有reader对此只赋库存，不触发技能；不会补充次数、重抽奖励或消费库存。重复37/2073加载只ACK，不重复同步。保留三龙、本体、门将及源规则。

这是完整合法布局的状态恢复候选，不声称已实机修复全0根因。回归先改变真实运行库存，再走真实房间加载，验证恢复当前库存、非开局固定2，且两个尾u32均为无效果/无目标值；重复加载不再发。既有出入口/恢复/阶段专项同时通过。

进一步将已实现的C2072登记完整日志保留，超过8次仍保存正文。前轮专属巴卡尔命令诊断已一并包含。本轮不增加同类新codec或未知包。

## 最终动画限制与新增证据

本轮继续追踪源MovieEndTimes及当前客户端parser：1477398f0把4个字段分别写入source+b0/b4/b8/bc，14774aa00同样按4项读取。还未闭合这4项与用户Final Strike场景的对应消费分支，不能任取16/21/40/24秒替换结尾期限。当前Final Strike本地对象manager300000ms、raid源SET TIMER28/56/180，以及Skip产生最终演员C39均保留。

新版第一次加载100003165时新增`bakal_final_scene_started`事件，写run、DGN、map、源deadline、源timeout及原始4个movie times。NPC恢复包事件名为`bakal_commander_inventory_restored`。下次用户手动测试可据此区分自然结束、Skip后的演员死亡与源期限；若到期奖励冻结失败，原有`bakal_settlement_pending`会记录具体错误。

最终动画自然结束问题本轮仍未修复。依据根AGENTS.md“禁止猜包：字段、顺序、等待态或客户端消费路径未闭环时，停止服务端叠包试探”，不以强制杀死演员、缩短源玩法时间或未知终幕包作为修复。

## 候选与验证

- `bin/wireprobe-bakal-inventory-restore-candidate.exe`，SHA256 `8d9f529317c2cca396f7dd9b117bc9b8c7418e34b76ca95b6835583923d01cd5`。
- 默认`configs/pvf-default.json`和既有隔离profile已接线；保留map-reset旧候选，未替换/重启运行中的服务或游戏。
- 显式真实归档Bakal/RaidRecovery专项、全仓vet、Python25项启动检查和`launch_local.py --check`通过；check确认D:\115us\DFO和PostgreSQL原路线，未启动客户端/服务或访问玩家记录。
- 全量仍仅TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference两个既有character兼容失败，无新增失败。日志在`.tmp/bakal-ui-20261006/inventory-restore-native-tests.txt`及`inventory-restore-all-tests.txt`。
- 显示验收仍由用户运行“启动游戏.cmd”。查看战斗/换房后的五个NPC次数；进入Final Strike后不点Skip，观察源180秒兜底是否回营。此步骤用于取证，不称为全部自然剧情已修好。

如显示候选无效，回滚仅把默认及隔离profile的binary改回`bin/wireprobe-bakal-map-reset-candidate.exe`，不动玩家数据。本輪未提交其它工作区改动，提交卫生脚本仍缺失。
