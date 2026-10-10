# Boost 662 训练 APC confirmed baseline

用户确认原话：“训练用APC出现了，能通过第二关”。附件截图显示当前玩家与“训练小助手”队伍栏。确认范围为此角色在模拟训练（二）中的实际训练APC出现、第二关恢复正常推进；不扩大为全部职业、其它训练路线、活动奖励或所有精锐玩法。

确认程序是源码入口 `bin/wireprobe-handoff-source.exe`，31,027,712字节、Go1.26.0、SHA256 `6b93bfdc34c8ffdbeee6a91c7ec82595c8068018d064e5f8382cde3840118325`。其本机精确副本留在未跟踪的 `runtime/baselines/boostup662-teaching-apc-confirmed-20261010/`。默认 `bin/wireprobe-pvf.exe` 未替换；日常复验本功能仍走仓库根 `scripts/启动游戏-SQLite.cmd --source-build`，由用户手动操作。

## 实机证据

会话：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_034251_994078_next37/`，以下时间为北京时间2026-10-10。

| events.jsonl行 | 确认事实 |
| --- | --- |
| 158..160 | 03:44:09，角色3发送CMD2333；NOTI1754为532字节、mode3、NPC标志1，线上特殊索引为1。 |
| 176..181 | 同秒自动CMD1811(mode3)，服务端按序下发NOTI1382 `01030000`及NOTI1879 `0300`。 |
| 259 | 03:44:29进入训练第二房100015495、副本100004546。 |
| 293..296 | 03:44:58客户端发送CMD45，移动成功ACK后进入100015496，原第二房出口卡点已通过。 |

APC可见与第二关通过由用户实机反馈及截图确认，服务器日志补充原生准备与过房时序；未将服务器发包记录单独当作NPC创建成功证据。

## 源、协议与存档边界

服务端唯一内容源为 `server/work/client-build/Script.inner.pvf`，SHA256 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`。`etc/mycharactersapc/mycharacters_special_apc.etc` 定义 `[index] 1`、`[apc index] 2504`、`[type] event 662`与Boost可用模式；`Source.BoostUp -> boostup.Load -> ParseTeachingAPCs`读取同一源，网关将特殊索引送入1754，保留其它可见账号模式并在刷新中保持活动选择。1811加载空额外角色的类型2容器后用1879绑定NPC，客户端自行创建AIC、处理DGN的APC初始化标志及ACT治疗/出口条件。

IDA MCP及经用户授权的x64dbg只读内存证明1754的NPC字段比较特殊索引，不能填AIC模板。attempt1遗漏周期刷新保持，attempt2仍误发模板2504，attempt3订正为源特殊索引；完整失败记录、原生地址与回滚副本见[取证记录](boostup662-teaching-apc-20261010.md)。运行客户端目录是 `F:/wip/dof/115US`，其PVF SHA256 `e18cd1c5a91e158138880e29d0c78f425b833f39a2e6f138b4e5c4d000d7a961`，与服务端内层资源身份分别记录；当前内存绑定与本次实机确认支持本功能，不声称两份归档完全相同。

本次不新增硬编码NPC内容表、技能ACK或进度强制包，不新增或启用DLL，不改客户端资源、权威IDB、schema、SQLite角色/物品存档。临时活动选择保存在连接会话中，不写账号精锐设置。重复规则检查及源变化回归已完成。

## 验证与提交

Go1.26.0专项回归通过；`go build ./...`、`go vet ./...`、`go test ./... -count=1`全部退出0，wireprobe全量71.018秒。attempt2与attempt3失败集合逐名比较均为空；`git diff --check`通过。真实PVF教学绑定专项通过；额外奖励全源闭环的590015880缺少[booster]错误已在未修改HEAD复现，保留既有缺口，不扩大本次确认范围。

用户已明确授权“按 gud/main 同步并提交”，用于替代当前不存在的origin/fork配置；`git fetch gud`后HEAD与gud/main差异为0/0，无需合并，不存在合并引入的新源码。仅提交本任务源码、测试、CHANGELOG、交接及确认记录；本地程序、runtime日志、备份、资源、存档和其他工作区改动不提交。
