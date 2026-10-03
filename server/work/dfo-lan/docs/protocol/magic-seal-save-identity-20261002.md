# 魔法封印解除的存档身份回归（2026-10-02，已确认）

## 状态与假设

attempt 1/3：仅修正 CMD393 的角色事件身份传递，不调整 codec、字段、应答或等待态。用户确认普通装备已正常解除封印；未操作玩家数据库，独立 PostgreSQL 16.4 回归通过。

## 证据

- 当前 HEAD `c21484f`；legacy `da3d36f` 的 `unseal_flow.go`、`internal/inventory/unseal.go` 与当前一致，解封业务没有删除。
- `07e1551` 将角色 `config_version` 等存档身份归一到服务端契约。CMD393 仍把 `lootService.Catalog.Source.Checksum` 当作角色事件身份传入 `CommitCharacterEvent`，与已归一的角色身份不同。
- 手动会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_234806_594272_next37`：启动日志确认解封服务已启用、17 个选项组；2026-10-02 00:25:41～00:25:44（北京时间）角色21四次 CMD393 校验成功，body=`2500ffff00000000`（槽37、无卷轴），均被拒绝为 `character event source mismatch`（events.jsonl:7111/7113/7115/7117）。拒绝发生在事务读取角色后、随机属性修改前。
- 原生请求、成功 ACK `01` 与 NOTI14 181字节装备行仍按此前115级 IDB/实机证据，见 `next46-magic-seal-unseal.md`；本次无需新增包布局假设。

## 修复

`worldSession.unsealRandomOption` 使用 `w.role.ConfigVersion` 提交解封，不再接收目录哈希参数；分发调用相应移除该参数。装备与随机属性目录仍使用当前原生 PVF 校验和，不削弱数据库来源相等门禁、账号归属、原子提交或防重放。不改 schema、玩家存档、PVF资源、profile或JSON输入链。

## 验证与程序

`TestUnsealNativeSaveIdentity` 直接读取当前内层 PVF，使用显式指定的独立数据库临时 schema：

- 契约身份角色复现旧调用的同一拒绝，修复调用返回成功 ACK393 和单行 NOTI14；历史归档身份角色同样成功。
- 随机属性完整记录提交后重新读库保持；金币、另一件装备及无关存档字段保持。
- 旧会话以相同封印前像重试时复用原随机结果；已解封装备再次请求拒绝；每个角色仅一条事件。

独立实例端口25461/26461，任务产物仅本地留存，两实例已关闭。专项回归通过（两种身份），`go vet ./...` 通过，Go1.26.0 `-trimpath` 候选构建成功，`git diff --check` 通过。

`go test ./...` 已完整执行，5项失败仍在：`TestAdventureAuditProvenanceAllowanceIsNarrow`、`TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback`、`TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader`、`TestOdysseyChapterFinalLordDrop`、`TestOdysseyCurrencySceneRetryAndPoolIsolation`。使用 HEAD 的 main/unseal_flow 与空的新测试文件做 Go overlay 对照，5项均以完全相同原因复现；没有本次新增失败。全量日志 `go-test.log`、对照 `baseline-tests.log` 留在本地任务目录。不把专项通过扩大为全量通过。

用户确认后核对源码入口 `bin/wireprobe-handoff-source.exe` 与默认 PVF 入口 `bin/wireprobe-pvf.exe` SHA256 均为 `2e00530babeb9b6ed4e357efce6a663fefc6c1d9b7da31843e383c0f945e9c7d`，纳入 confirmed baseline。原源码入口为此前确认的 `9594b7440046e106337bc5de66277d5931bf3a08202f40bb9d3b6671148cb75b`，备份在根 `.tmp/unseal-identity-20261002/wireprobe-handoff-source.before.exe`。

## 手动验收

用户已手动验证普通装备可正常解封。此次确认范围不扩大为所有装备、卷轴解封或其他随机词条功能逐项验收；未产生数据库迁移。若需回退，关闭会话后将上述备份复制回源码入口。
