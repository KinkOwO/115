# Hell Party 全副本入场支持（2026-10-03，已确认）

## 根因与证据

用户报告 Hell Party 无法进入，并要求覆盖所有支持副本。本轮只修改 Go 服务端。

- 手动会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261002_225254_940143_next37/events.jsonl` 的北京时间 2026-10-03 00:23:15 / 00:23:16：角色10发送 CMD16，明文 `570000000200000100ffff000000000000000000000000000000000000000000`，即副本87、难度2、Mode1、Quest0。服务端拒绝原因为 `Hell Party entry is not yet verified for this dungeon`。
- `dungeon.Select` 保留了9月27日 attempt 1/3 的 ID103 单图验证限制。旧 Trombe 用户验证只确认进入、柱子和刷怪，未确认 Hell 专属奖励；见 `normal-hell-party-20260927.md` 及 `normal-hell-party-loot-20260927.md`。
- 本轮复用当前已打开的权威 IDB 会话 `2e6257e3`，未直接打开或覆盖 IDB。原生 sender `sub_146D495E0` 的 `0x146D4A2D6..0x146D4A2E6` 将选择状态 `a1+1444` 写成 CMD16 Mode 字节，没有服务端 ID103 门禁。NOTI28 handler `sub_1452AC840` 在 `0x1452ACF01/0x1452ACF10` 连续读两个 u8，写入副本对象的 Hell XY (`+6468/+6476`)，普通 Boss XY 在此前的另一组字段。`sub_145B27520` 在 `0x145B2761D..0x145B276E6` 按当前格与 Hell XY 比较；没有按本轮副本号另选布局。
- 当前内层 PVF SHA256 为 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`。直接解析 DGN `[hell dungeon] 1`、`[seal door map index]` 和 `[seal door pos]`：60 个副本声明，59 个封印地图可读。活动副本 `100005110` 引用的 `100016811` 不在当前地图索引，继续明确拒绝，不能凭空补资源。

## Attempt 2/3：扩展已验证入场路径到源声明副本

- 删除 ID103 硬编码准入；范围仍由当前 PVF 的 Hell 声明与可用封印地图决定。没有新增 JSON 内容清单、玩法开关或客户端补丁。
- 封印房间仅覆盖本次会话的迷宫副本。保留起点、Boss坐标和源目录，普通 Mode0 路线不变。
- 源自身将封印地图放在起点/Boss格的专用 Hell 副本允许入场，并保留 Boss 标记；剧情路线中不同地图的起点/Boss不被替换。
- 检查封印房间与源路线连通，拒绝不连通的剧情路线，不虚构连接房间；已有 Trombe Quest6399 向迷宫边界外增加源封印格的行为保持。封印格不继承被替换的剧情图层，避免重访跳回原地图。
- 等级、难度、任务所有权、入口许可及疲劳仍走原有检查。特殊模式、季节模式、邀请函扣费和 Hell 专属掉落没有新实现；旧奖励缺口未因准入放开而宣称解决。

## 实机确认

用户于2026-10-03确认：Hell Party 正常进入，封印柱出现，攻击可破坏，随后刷出 Hell Party 专属怪物。此结果将候选 `104965f85e1721547dc646b65c5a24ed8050529c5867e55b22a712842b93b7b8` 纳入本项 confirmed baseline。确认覆盖用户实际测试的副本路径，不等于逐一实测全部59个资源完整副本；Hell 专属怪物掉落和奖励完整性仍未确认。

回滚：撤回 `internal/dungeon/session.go` 本轮变更及新增 `hell_party.go`，或继续使用原默认入口。本轮独立二进制/profile未覆盖默认或其他任务候选。未访问玩家数据库、改 schema、存档身份或客户端资源。

## 验证与手动入口

- 当前 PVF 专项 `TestHellPartyCurrentPVFEntryCoverage`：60项声明、59项成功选择、1项缺图拒绝；实际 CMD16 副本87原生向量成功。Trombe旧原生向量及专用单房间/Boss标记、冲突/缺图/断路和源迷宫不变回归通过。
- `go test ./...` 已执行：dungeon、catalog、gamedata、loot等通过；三项 wireprobe来源审计和商城SKU3400013空发放失败。以 HEAD 的 `session.go` overlay复核四项，全部同样失败，无本轮新增失败。
- `go vet ./...` 通过，独立候选构建通过。启动器 `--check --server-only --repair-profile ...` 通过（不启动客户端）。按真实启动器补齐穿戴策略和目录定位参数后，默认54项全量 PVF 的 `-pvf-check-catalogs` 检查退出0，准备49.066秒、GC后堆528.2MiB，未打开数据库/监听器；见本轮 `.tmp/prepare-report.json` 与 `prepare.log`。最初单独调用缺少穿戴策略参数的失败仅属只读检查参数问题，没有修改运行路径。
- 候选 `.tmp/hellparty-20261003/wireprobe-hellparty.exe` SHA256 `104965f85e1721547dc646b65c5a24ed8050529c5867e55b22a712842b93b7b8`。
- 手动候选入口为 `.tmp/hellparty-20261003/启动验证.cmd`。用户已确认封印房及专属怪物刷出；先前被拒绝的原生副本87请求也通过源码和当前PVF专项回归。

59项为服务端当前资源离线覆盖，不代表每张图已逐一手动跑通或 Hell 奖励完整。

候选已于00:37构建并隔离。00:41以后检测到同期其他工作修改 `cmd/wireprobe/main.go`、`pvf_catalogs.go`、`pvf_odyssey.go` 和 `internal/character/progression.go`；这些文件不属于本任务，保持其工作区改动，不重建覆盖本轮独立候选，也不将其纳入本任务提交。
