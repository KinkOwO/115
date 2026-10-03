# 克隆装扮誓约进图修复（2026-10-02）

状态：attempt 2/3 被用户实机否定（进图后 Clone Avatar 裸体）；attempt 3/3 已通过用户实机确认，克隆外观和誓约正常，纳入本功能 confirmed baseline。

## 范围与计次

本次按用户授权，以当前主代码为基线实施参考 README 中尚未合入的克隆装扮进图修复，不整体替换参考源码。沿用当前 `worldSession.store`，保留存档 `SaveIdentity()`、领域 workflow、疲劳计费/失败回城、天平掉落档位和 Buff 登记恢复。

**attempt 1/3**：当前主代码已有的非克隆誓约直发时序。**attempt 2/3**：先克隆重建、再完整穿戴13/14/2839，用户实机反馈裸体，否定该时序。**attempt 3/3**：先完整穿戴刷新，再克隆重建与非装扮恢复，最后2839；用户已确认克隆外观和誓约正常。参考包中把誓约槽塞入 mode-1 投影导致卡图的方案不采用；`detail.go` 和 `clone_reattach.go` 未改。参考源码只作线索。

不修改协议字段、装备实例、数据库 schema、玩家存档、客户端资源或 DLL。

## 当前客户端依据与实施

本轮复核现有权威 `client/DFO.exe.i64` 会话，未另外打开或改写 IDB：

- mode-1 装备 reader `sub_1452C1540`，`0x1452C2D3F..0x1452C2EF7`：按 48 字节出现标记遍历槽位；`0x1452C2D50` 检查缺席标记、`0x1452C2D5A` 跳过槽31、`0x1452C2EE9` 调用角色虚表 `+0x1F30` 清槽，循环上限 `0x30`。当前详细投影不携带誓约槽36..47，因此重挂载后必须恢复这些穿戴行。
- S2C2839 handler `sub_1405757F0` 读取15字节语义前缀，调用 `sub_145EFAFB0` 获取角色，再交给 `sub_14057D140`；后者只在角色非空时应用选项。本轮不改变现有24字节服务端信封。
- 既有实机记录只确认过加载后 NOTI13/14/2839 的选项显示；本次用户进一步确认当前克隆进图路径下外观正确、誓约正常。已知证据范围见 `oath-dungeon-selection-20260929.md` 及本次验收。

### attempt 2 失败与当前IDB复核

用户报告“进入副本后，角色裸体，和clone avatar冲突”。会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261002_220745_148023_next37/events.jsonl`，角色11，UTC14:23:34.938及14:23:57.633的开图序列均为克隆重建→非装扮14→完整13→完整14→2839→29；前一次完整13/14各36行，非装扮14仅25行，14:23:35.425加载后没有再次克隆重建。日志证明走到候选路径，但不能单独证明哪条客户端调用使图层丢失。

当前IDB复核：NOTI13 `sub_1452D5A80` 在 `0x1452D5EB4` 调用 `sub_145ADC2A0`，后者在 `0x145ADC5D2` 进入 `sub_145AF4360` 清理装备管理器；循环在 `0x145AF45CD..0x145AF4619` 清理现有条目，随后NOTI13按行创建/更新实例并挂载。NOTI14 `sub_1452E9810` 的穿戴分支在 `0x1452E9C23` 调用对象虚表+0x430更新记录、`0x1452E9C85`调用+0x358绑定角色，随后处理挂载。因此完整13/14不能被当作仅刷新UI而放在克隆继承外观重建之后。已有当前客户端实机确认的加载/换装流程也是先完整刷新、后克隆重建、最后只恢复非装扮14，参见 `avatar-clone-coexistence-candidate.md` 的2026-09-24/26记录。本轮移用该相对顺序；尚未证明NOTI29跨场景时仍保持外观，需手动验证。

本轮没有修改PVF、sk.dat或任何运行配置资源；前一候选和当前候选使用相同默认PVF环境。没有资源查找失败日志证据，不将此次顺序回归认定为资源数据缺失。

当前 `cmd/wireprobe/dungeon_flow.go` 的行为：

1. 取消 `oathDirectEntryActive()` 的克隆排除条件；缺失 `w.store` 时返回未命中，避免测试/无存储角色访问空存储。真实誓约核心与选项仍从存储读取。实机确认后移除临时诊断开关。
2. 命中直发时先发 NOTI13 完整穿戴和 NOTI14 穿戴更新；存在克隆时再发 `dungeon_clone_detached_pre_direct`、`dungeon_clone_reattached_pre_direct`、`dungeon_nonavatar_worn_restored_pre_direct`（只包含非装扮行，恢复誓约槽），之后发 S2C2839，最后 NOTI28/29。克隆重建之后不再发包含装扮槽的完整13/14。
3. 加载后直发角色跳过克隆重挂载和重复13/14/2839；非直发角色保留原来的重挂载、非装扮恢复，之后补发2839。保留末尾 NOTI1361 Buff 登记恢复。
4. 不保留实验性运行时开关；本修复默认作用于穿槽47誓约核心的角色。

## 回归

Go 1.26，PostgreSQL16.4。独立测试集群位于仓库根 `.tmp/oath-entry-20261002/pgdata`，仅监听回环25442；专项测试使用临时 schema，与玩家25438存储隔离，测试结束后删除测试 schema 并停止集群。

- `TestOathDirectEntryWithoutWorldStore` 通过：角色领域 Store 不替代世界 Store，无世界存储时不访问数据库。
- `TestOathDirectEntryIntegration` 通过：克隆/非克隆、有/无誓约、关闭直发五类状态，各覆盖普通副本22和修炼场5000（10组）。验证 NOTI29 前的顺序、存储选项3、无加载后重复重建、非装扮实例原始字节和未知存档字段保持，以及1361仍在加载恢复末尾。
- 原有克隆装备恢复、修炼场入口与 Buff 登记重建专项通过。
- 通过 Go overlay 将新专项套到未修改 HEAD 的 `dungeon_flow.go`：克隆+誓约在副本22、5000均因没有开图前克隆恢复帧而失败；没有回滚工作区。
- attempt 3新顺序测试通过；用overlay套到保存的attempt 2源码，在副本22和5000均因克隆重建位于完整刷新之前而失败，证明覆盖本次实机否定的时序。临时集群恢复在沙箱内受跨进程信号权限限制，转到沙箱外执行同一临时集群测试并正常停止；没有使用玩家库。
- `go vet ./...` 与 `go build -trimpath` 通过。
- 修改前后 `go test ./...` 均有相同4项已有失败：`TestAdventureAuditProvenanceAllowanceIsNarrow`、`TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback`、`TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader`、`TestShopPilotPVFCurrentCatalog`（SKU3400013空发放）。本轮没有修正这些其它范围的失败。

完整输出在仓库根 `.tmp/oath-entry-20261002/`：attempt 2为 `baseline-test.txt`、`targeted-test.txt`、`regression-before.txt`、`full-test.txt`、`vet.txt`；当前attempt 3为 `targeted-attempt3.txt`、`regression-attempt2.txt`、`full-attempt3.txt`、`vet-attempt3.txt`。当前全量测试仍为上述相同4项已有失败。`verification.attempt2.json`和`live-attempt2-rejected.json`记录失败，`verification.json`记录当前检查。候选 profile 的环境与当前 `pvf-default.json` 完全一致，19项文件依赖只读检查通过。

## 确认基线

用户已确认：“外观显示正确，誓约正常，可以提交”。attempt 3/3 纳入确认基线。独立程序：仓库根 `.tmp/oath-entry-20261002/wireprobe-oath-entry-confirmed.exe`。attempt 2程序 `wireprobe-oath-entry.exe` 与 `dungeon_flow.attempt2.go` 保留在本机临时目录作失败对照，不纳入提交。

确认程序SHA256：`1127299544defa56908cb12887ea4bbecaf6416ebbef1cf27ef4abb51484efae`。移除诊断开关后重新构建；先前attempt 3验收程序SHA256：`a277edd1360e156230eb3e4c3257f8c59eab75da42083288c78bc1bdf8d321cb`；失败attempt 2 SHA256：`f6bb4b044f39008ee9f5d9ecf2bdef1ddc21709142f12e7b95062bad06f1c359`。

确认程序仍通过同目录 `profile.json` 隔离选择；源码入口 `wireprobe-handoff-source.exe` 和默认入口 `wireprobe-pvf.exe` 均未覆盖。

实机验收：用户确认进图后外观显示正确、誓约效果正常。

日志应显示开图前 `dungeon_worn_equipment_direct -> dungeon_worn_visuals_direct -> *_pre_direct -> dungeon_oath_selection_restored -> dungeon_start_map_sent`，命中直发的加载阶段不再出现 `dungeon_clone_detached/reattached`。本次提交仅包含源码与文档，不含临时二进制或玩家数据。

运行回退：关闭候选会话，使用原根 `启动游戏.cmd` 即恢复保留的默认程序。源码回滚依据 `.tmp/oath-entry-20261002/dungeon_flow.before.go`（HEAD原文件）及本次 diff，不能整份替换过时参考源码。
