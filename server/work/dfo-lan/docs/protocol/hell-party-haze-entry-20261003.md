# 雾都赫伊斯 Hell Party 重复声明修复（2026-10-03，已确认）

## 真实拒绝与资源证据

- 用户手动会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_174608_645705_next37/events.jsonl`，北京时间18:16:28：角色10发送CMD16 `5c0000000200000100ffff000000000000000000000000000000000000000000`（副本92、难度2、Mode1、Quest0），服务端拒绝 `Hell Party is absent from this dungeon source`。随后七次重试均为相同请求和拒绝。
- 当前内层PVF SHA256 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`；`list/dungeon.lst` 将92绑定到 `dungeon/cataclysm/act15/92_haze.dgn`。
- DGN的普通迷宫0/1各声明 `[hell dungeon] 1`、`[seal door map index] 60056`、`[seal door pos] 1 2`。两份完全相同；起点候选为(1,4)/(3,4)，迷宫0 Boss为(0,1)，普通封印位置原房间95155。DGN原始4170字节SHA256为 `774981907140a68d5f6c5c8327615f4789ab3847fe32a4be9b81c303ed5cb116`。
- 原 `sectionCells` 将同名段内容拼接；原Hell解析器要求标量长度1、坐标长度2，于是两份合法声明拼成长度2/4后被静默丢弃。源有定义且地图可读，问题来自服务端解析。

## 入场修复 attempt 3/3

本轮是全副本入场 attempt 2/3 后的重复声明修复。只修复已知字段的读取，继续使用已取证的CMD16、NOTI28及NOTI29路径，未增加报文或猜测客户端分支。既有115 IDB闭环见 `hell-party-all-entry-20261003.md` 和 `../../../../../analysis/tasks/hell-party-owned-waves-20261003.md`。本轮IDA MCP 127.0.0.1:8745不可连接，未直接打开/覆盖权威IDB；没有据此扩展新协议语义。

Hell相关字段逐个按段读取；所有出现的该字段必须逐token一致，才保留一份。不同地图、坐标、启用值或空/损坏的重复段保持拒绝，不选择第一份、最后一份或任意迷宫值。通用 `sectionCells` 不改动，原始DGN cells保留。

当前源扫描恢复6个同值重复定义：70、92、100000522、100000523、100000524、100000525。Hell定义数60→66，基础入场离线覆盖59→65；100005110缺少源地图100016811仍拒绝。此数字只证明目录、迷宫和入口基础检查，专属波次支持仍受已有柱子/actor源规则限制，不表示65张图的完整玩法逐项实测。86及其他存在不同封印地图/坐标的副本仍需按迷宫建模和取证，本轮不猜测该类规则。

## 验证与手动回归

- 重复同值、不同值、空段、损坏段、原始cells保持测试通过。
- 当前PVF专项覆盖66项定义：65项基础入场通过，1项缺图拒绝。副本92真实CMD16向量通过，封印坐标(1,2)、地图60056及owned专属怪物名单均有效；Mode0仍使用95155原普通房间。
- `go test ./...` 与 `go vet ./...` 全部通过（Go1.26）。之前记录的4项既有失败在当前合并HEAD已不复现。初次测试因默认Go缓存目录不在可写范围失败，改用工作区内GOPATH/GOCACHE后通过，没有改变代码或运行路径。
- 独立候选 `wireprobe-hellparty-heiz.exe` SHA256 `f31c611c33956867beaf091ad8baccf87e08b11f0ee69d1057fa10d7a1dd503e`，配套 `profile.json` 继承当前默认54域PVF配置；启动器 `--check --server-only --repair-profile <绝对路径>` 通过。启动验证入口为根 `.tmp/hellparty-heiz-20261003/启动验证.cmd`，调用根游戏启动器并显式选择该profile。未覆盖默认/源码二进制。
- 同候选的54域PVF只读准备检查退出0，准备49.528秒、GC后堆642.1MiB；没有打开玩家存储或监听器。用户确认副本92 Hell Party 可以进入；手动会话记录18:36:43的Mode1请求和成功ACK，18:39:19在地图60056开始副本会话。候选纳入本项confirmed baseline。确认范围限本次副本92入场，不宣称封印柱破坏、后续波次/掉落或所有副本已实测。
- 回滚仅撤回 `internal/catalog/dungeons.go` 本次Hell字段读取及新增 `dungeon_hell_party.go`。不回滚其他功能或用户工作区改动。

本轮不改数据库schema、玩家存档、客户端资源或DLL。独立候选/profile/测试输出位于根 `.tmp/hellparty-heiz-20261003/`，不入Git。
