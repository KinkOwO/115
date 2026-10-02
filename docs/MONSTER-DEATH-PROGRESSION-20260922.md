# 怪物死亡后尸体卡住：转职成长门禁阻断死亡确认

日期：2026-09-22。状态：attempt 1/3 的服务端修复与自动化验证已完成；**用户实机验证通过并确认收口**。

## 1. 当前实机证据

会话：`server/work/dfo-lan/runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260922_194901_505941_next37/events.jsonl`。

- 第 963 行：`advancement_job_info`，NOTI718 正文 `01`。
- 第 971 行：CMD1881 `advancement_completed`，正文 `010000`，时间 `2026-09-22T11:58:36.4891026Z`（本地 19:58:36）。即普通转职已成功。
- 第 1052 行：进入 dungeon 11 / maze 0 / map 58597，4 个源怪物。
- 第 1069/1071/1073/1085 行：客户端 checksum-verified CMD39，实体分别为 4098/4096/4099/4097，killer 均为本角色场景 ID 10。
- 第 1070 行起：`dungeon_request_refused id=39 reason="advanced profession growth is not yet supported"`，随后同一批实体持续重报。没有对应 `monster_death_ack` / `monster_death_confirmed`。
- 第 1235/1243 行：换为 advancement 2 也成功，但死亡请求仍然被同一成长门禁拒绝。
- 后续另一角色在 dungeon 17 / map 57964 也发生同一拒绝，不是某张地图独有的尸体动画问题。

四条原生 CMD39 正文已保存为小型回归向量：
`server/work/dfo-lan/cmd/wireprobe/testdata/monster_death_after_advancement_20260922.json`。

## 2. 调用链与根因

```text
CMD39
  -> worldSession.monsterDeath
  -> protocol.DecodeMonsterDeath            已通过（不是解码错误）
  -> dungeon.Session.ConfirmDeath           已通过（不是归属错误）
  -> character.ProgressionService.Monster
  -> storage.CommitCharacterEvent
  -> character.ProgressionService.ApplyGain
       advancement != 0 且没有旧 pilot 标记 -> 拒绝
  -> monsterDeath 返回 nil, error
  -> 整份 ACK39 / NOTI38 / NOTI37 回包计划不发送
```

`ApplyAdvancement` 已按当前职业的 `AdvancementGrowth[advancement]` 验证分支并保存普通转职状态，但 `ApplyGain` 仍停留在旧试点阶段：非零转职只接受 `all_jobs_pilot` 或特定 `swordmaster_pilot` 存档。因此成功转职后，即使本次经验不升级，怪物死亡处理也会失败。

旧成长选择还有相同来源的问题：所有 `advancement == 1` 都先取 `SwordmasterGrowth`，只有带 `all_jobs_pilot` 才切换到本职业的 `AdvancementGrowth`。只删掉报错门禁而不改成长选择，会让其它职业的一转继续失败或选错属性。

当前启动链目录 `characters.skycastle-release.json` 已含全职业有源 `advancement_growth`；无需猜成长数据、重导配置或修改死亡包。

## 3. attempt 1/3 的修复边界

运行逻辑仅修改 `internal/character/progression_state.go`：

1. 未转职继续使用 `BaseGrowth`。
2. 非零转职使用本职业的 `AdvancementGrowth[advancement]`，不再要求旧试点标记。
3. 对旧剑魂试点目录保留严格兼容：仅剑士 job 0 / advancement 1 / `swordmaster_pilot` 且缺通用分支时使用 `SwordmasterGrowth`。
4. 分支无源、版本不匹配仍拒绝，不回退未转职成长，不伪造成功回包。
5. 现有经验/等级/SP 算法、属性增量、事务幂等键、JSON 未知字段保留机制均不变。

本轮没有修改 CMD39、NOTI38/37 的布局/顺序、清房/归属规则或掉落算法，没有新增协议假设。没有启动或操作客户端，没有打开权威 IDB，没有修改 DLL/PVF。

## 4. 验证结果

全部使用项目自带 Go 1.26（`tools/go`），工作目录 `server/work/dfo-lan`。

| 检验 | 结果 |
| --- | --- |
| 修复前定向复现（job12 / adv1，`gain=0/1/192739`） | 全部在 `advanced profession growth is not yet supported` 处失败，与实机拒绝同因 |
| `internal/character/progression_advancement_test.go` | 通过：源目录 86 个基础/转职分支、三类经验增量、SP/属性正确、无 pilot 泄漏、5 类拒绝路径、存档字段保留 |
| `cmd/wireprobe/monster_death_advancement_test.go`（真实 PostgreSQL） | 通过：4 条原生 CMD39 均得到 ACK39 + NOTI38 + NOTI37，12 次重报仅 4 份收据，清房后可换房，经验落盘且背包/未知字段无损 |
| 既有全职业试点与旧剑魂试点成长回归 | 通过 |
| `go test ./...` | 全绿 |
| `go vet ./...` | 全绿 |
| 源码候选版编译 | 通过，`bin/wireprobe-handoff-source.exe`，18,048,000 字节，sha256 `3cee1d2dc2aa13e8bf29cee61a5a2f1589b009d4ee0620973d2917791acf48e7` |
| `launch_local.py --check` | Paths OK；PostgreSQL 已在监听；启动将使用上述候选版 |

过程中的两类非业务失败（均已修正，不计入运行路径尝试）：

1. 首次后台命令在 Windows CMD 下使用了 `export`，Go 测试完全没有启动；改用绝对路径与 `cd /d` 后得到真实复现。
2. 死亡事务测试最初误用 `configs/dungeons.generated.json`（仅 11 个副本，不含实机副本 ID 11），报 `dungeon absent from imported source`；按 `channel_probe.py` 的启动实参改为 `configs/dungeons.full.json` 后通过。仅调整测试夹具，未改运行逻辑。

## 5. 存档与工作区安全

不修改数据库结构或现有玩家存档，不补写 pilot 标志，不重置角色、经验或装备。当前副本会话的死亡状态仅在内存中，重启服务后需重新进入副本。

本轮开始前，`world_flow.go`、`internal/dungeon/session.go` 及其测试、`internal/game/protocol/dungeon.go` 及其测试已有未提交改动（APC 跟随及格式化），另有本地 PostgreSQL 配置与分析文件；本轮不修改、回滚或提交这些内容。

回滚本轮实现时只恢复本轮的 `progression_state.go` 补丁并移除本轮新增测试/记录，不对整个工作区执行 reset/checkout。不覆盖归档 `wireprobe-dungeon39.exe`。

## 6. 实机验收结果

用户实机测试确认：
- 已转职角色进图击杀怪物后，尸体正常消失。
- 经验正常增加，房门正常开启，副本可正常继续推进。
- 服务端正常响应 ACK39、NOTI38、NOTI37，不再出现 `advanced profession growth is not yet supported` 拒绝。
- 存档兼容无损，本次修改正式合入并收口。
