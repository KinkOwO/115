# Cera 商城购买的存档身份回归（2026-10-01，候选）

## 状态与唯一假设

attempt 1/3：修正订单与角色存档比较时的身份取值；不改变 CMD64 reader、codec、包字段或等待态。尚待用户手动实机购买确认，confirmed baseline 不升级。本次未启动客户端，也未连接玩家 PostgreSQL/Redis；独立测试实例已关闭。

## 证据闭环

- 当前代码 HEAD：`577ebf4f854495c0d46f96c2cef6d826ef2f2696`；对照 legacy：`da3d36f181bed240c14f4ca70ecfa7a33d60bb2c`。
- 20:23 会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_202304_920720_next37`：商城加载 17,245 行、17,154 个启用商品，余额 921,790。
- `events.jsonl:207–209`：CMD64 校验通过，body=`000001000028e2330001000000000000`，SKU 3400232、数量 1；拒绝原因为 `cash order catalog does not match character source`，没有扣款。
- legacy 的订单和角色均使用内层归档哈希。当前重构已将角色 `config_version` 归一为 `ArchiveSnapshot.SaveIdentity()`，但商城仍以 `Config.Source.Checksum` 填写订单 `Source`。
- `storage/cash_purchase.go` 在事务内继续检查 `characters.config_version == order.Source`。因此当前商城所有这类订单都会误拒绝，与客户端包解析无关。

## 修正范围

普通物品/礼包与混合契约订单、背包扩容券、金库扩容订单统一采用 `Config.Source.SaveIdentity()`。账号金库在交易内借用角色版本，故对应校验也使用该契约；角色金库 1/2 的独立 `VaultRules.SourceSHA256` 保留。

保留原生 PVF 的价格、商品内容和目录来源校验，保留交易的来源相等校验、账号归属、余额、档位、整单原子性和重复请求保护。不修改存档 schema、已有订单/角色行、默认 profile、导出 JSON 或客户端资源；没有新增 JSON 运行期回落。

## 离线验证

- `TestCashPurchaseSaveIdentity`：普通、混合契约、背包和账号金库订单使用契约身份；容量生效。
- `TestVaultSourcePurchase`：16 档角色金库订单使用契约身份，同时保留金库自身版本、价格、档位及内容。
- `TestShopPilotDatabasePurchase`：独立 PostgreSQL 16.4 / Redis 5.0.14 中，普通与混合购物车扣款发货、ACK 加密、失败回滚、重复请求通过。测试支持通过 `DFO_TEST_STORAGE_CONFIG` 指向独立实例。
- `TestShopPilotNativeSaveIdentityPurchase`：直接读取当前 `Script.inner.pvf`，不读取历史商城 JSON；重放实机 body，商品 3400232 以 7,900 Cera 交付模板 590712474 ×100。旧哈希订单仍被未改变的存储门禁拒绝；响应编码失败不扣款；修复订单扣款、背包和审计原子提交，原有物品/无关字段保留；重试不扣款并返回更新后的存档；没有可再次领取的未交付物品。
- 测试实例位于根 `.tmp/cera-identity-20261001/`，端口 25459/26459；已正常关闭。不入 Git。

`go test ./...` 已执行：商城、存储和其它通过的包均记录成功；全量有 5 项既有失败，不能标记全绿。已从未修改 HEAD 导出源码到根 `.tmp/cera-identity-20261001/baseline/`，仅提取测试所需文件，复跑得到完全相同的失败：

1. `TestAdventureAuditProvenanceAllowanceIsNarrow`（来源哈希审计期望拒绝，但现有实现仅警告）。
2. `TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback`（语义差异审计期望拒绝，但现有实现仅警告）。
3. `TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader`（已有期限变更审计期望拒绝，但现有实现仅警告）。
4. `TestOdysseyChapterFinalLordDrop`（`Odyssey currency source mismatch`）。
5. `TestOdysseyCurrencySceneRetryAndPoolIsolation`（同上）。

上述均不在本次商城改动范围，保持原状并明确记录。`go vet ./...` 单独执行通过，`git diff --check` 通过；Go 1.26.0 `-trimpath` 构建候选成功。

候选 `bin/wireprobe-handoff-source.exe` SHA256：`ffda6686159700d293a1e9395190f218ce8f094a25208ed84fc7220b94e72263`。
本次开始时默认程序实际 SHA256：`bc9ce99272694d81faaa20822acf1110f834a5f7a5e1d1e1d694ecc1ad9262cf`；复制源码候选后再次核对默认程序哈希相同，不将此前历史 confirmed 身份误写为当前文件。
备份旧源码入口 SHA256：`ad9e13e5c2bf7ce55b13494db4dd9ba4fecfbd0ae5387aba8410349d545afeca`。候选未实机确认、不提交收口、不升级默认程序；用户确认后按根规则更新 CHANGELOG 与 confirmed baseline 并只提交本任务文件。

## 手动验收与回退

关闭现有会话后，从根目录手动运行 `启动游戏.cmd --source-build --pvf-mode`。购买原失败商品，确认扣款一次、物品数量正确、界面结束等待；重选角色确认物品和余额保留。必要时再检查契约、背包扩容与账号金库扩容，不将普通商品通过扩大为全商城逐项验收。

候选仅发布到 `bin/wireprobe-handoff-source.exe`，默认 `bin/wireprobe-pvf.exe` 保留。本次候选旧文件备份位于根 `.tmp/cera-identity-20261001/wireprobe-handoff-source.before.exe`；关闭会话后可复制回源码入口。代码撤回仅逐项撤回本任务的身份取值改动，不能 reset 工作区或恢复整份用户文件。协议布局未变，无数据库迁移需要撤回。
## 20:53 用户确认与收口

用户反馈“用cera点的能买，但是用金币的买不了”。确认边界为普通Cera购买：最新20:51会话的events.jsonl:183–187记录SKU3000127，角色13，扣10点（921790→921780），模板10000540×1落袋及CMD64成功回执。源码入口ffda6686…纳入该范围的confirmed baseline；默认程序未替换，不扩大为全商城/扩容逐项验收。本次身份修复按根规则更新CHANGELOG和交接记录并独立提交。

金币SKU3400315的两次请求（179–181、188–190）解码通过，但`product 3400315 not enabled for cash delivery`拒绝，未扣点。该商品历史源行cell3=100、cell5=0，需另行确认当前原生金币价格与扣款、刷新路径；不能把金币价当Cera价，也不把本次Cera确认扩大为金币购买恢复。上文“候选待确认”叙述为当时状态。