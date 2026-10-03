# 奥德赛章节完成与115级荣誉奖励（2026-10-03候选）

本次按用户给出的官方清单及其明确要求，新增 `configs/odyssey-completion-rewards.json` 保存章节和荣誉奖励的物品ID、数量与英文说明。该配置是人工维护的奖励映射，不是PVF导出器或读取失败回落。章节划分、终点副本、物品类型、堆叠限制及礼包内容仍由当次服务端内层PVF提供；ID缺失或类型错误时启动明确报错。配置在默认PVF路径自动加载，无功能开关。

## 取证与数量

英语客户端来源：`client/Script.pvf`，外层SHA256 `5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`，只读解包内层SHA256 `be95d64ee120248ae503194d2f61743ef74409a8ff999a4a986ca2e0bccf69b0`。名称由原生 `list/stackable.lst` 的ID/路径和 `[name]` 本地化引用对应，不按文件名或汉化名称猜ID。

核查发现PVF实际含有章节 `[reward]` 表和 `[complete reward info]`：

- `contents/2026/aradodyssey/etc/aradodysseyjournal.cos`，SHA256 `d4654fa9a50ddd582077f5f7a0a19835ec4fb6b777f032d1a66be7f288eff67e`。
- `contents/2026/aradodyssey/etc/aradodyssey.etc`，SHA256 `638e71ab8fdc84b4be28db8ca3302fd1dfe689a9b771907514297edee4b8c8e8`。
- 官方清单未列每行数量，沿用原生表：第4章 `10419743×2`、第5章 `10419744×2`，其余行×1。保留原表行序，兼容旧章节收据；配置行序及ID不应被随意重排，以免改变历史收据键。

| 章 | 原生终点副本 | 配置奖励ID及数量 |
| --- | --- | --- |
| 1 | 100004945 | 10417792×1、10419741×1、10419342×1、10419346×1 |
| 2 | 100004953 | 10419742×1、10419745×1、10419346×1 |
| 3 | 100004957 | 10417793×1、10417796×1、10419745×1、10419346×1 |
| 4 | 100004966 | 10417794×1、10419743×2、10419745×1、10419347×1 |
| 5 | 100004972 | 10419744×2、10419745×1、10419347×1 |
| 6 | 100004977 | 10417795×1、10419744×1、10419745×1、10419347×1 |
| 7 | 100004990 | 10419538×1、10419539×1、10419745×1 |

配置每行的 `name` 给出完整英语名称。区分带前缀的章节选择盒 `10419741..10419744` 与同名非章节物品 `10417797..10417800`；不能用后者替换。

荣誉盒 `10420561×1`：`stackable/10420001/10420561.stk`，SHA256 `a3863c9bafb117870859a6a1491dcf42aebf47d8eb92b1a767e8ba689114d0c6`。PVF内容为 `10420562` Party Frame 和 `10420563` Journey Log 各1，复用既有礼包/皮肤使用路径，不把盒内物品再额外发一次。

战斗支援盒 `10419342`：`stackable/10419001/10419342.stk`，SHA256 `efa05f336f6102282091b6240d9569bd60f0791aeb6e943a114fc70f9f769a46`。四项原生内容为 `100991331×1`、`101591087×1`、`10418028×10`、`10418029×5`；原有礼包解析和装备期限处理负责开启，未另建第二套盒内奖励表。

官方文字与本地物品脚本并非所有字段一致：例如战斗支援盒稀有度为3，荣誉盒/章节盒存在 `[attach type] [trade]`。本项实现奖励发放，不修改客户端资源、不猜测物品行绑定字段，也不宣称已把所有物品的显示稀有度和交易行为改成官方清单。此差异保留为资源/物品规则核查边界。

## 发放与兼容

- 章节终点完成后直接入背包。仅使用服务端持久化的 `odyssey_completed_dungeons` 或旧 `dungeon_best_times` 成绩；加锁后的回调再次检查终点，拒绝只在调用者快照存在的完成声明。
- 原始创建包证明奥德赛身份；调试环境强制奥德赛不能使普通角色获奖。已毕业角色仍能补发有真实通关证据的欠奖。
- 保留 `odyssey-chapter-reward:<chapter>:<line>:<template>` 和旧模型，不改已有发放收据。每行独立事务，背包满只留下该行欠奖，下次登录/通关重试。默认发放改用完整原生物品目录及背包规则，不再把选择盒统一伪装为 `[booster]`。
- 荣誉奖励严格按官方条件：原始奥德赛角色达到115级即投递邮箱；末章通关会升至115级并触发。无需等回城毕业标记，也不额外设“所有50个副本逐一完成”的条件。登录和实际回城同时提供重试入口。
- 邮件、`odyssey-honor-mail-v1` 收据和欠奖清零使用同一角色锁、同一事务。背包满不影响投递；邮箱正文/未领取附件255上限仍生效，满箱整体回滚。邮件期限沿用既有系统的15天。
- 尊重历史 `odyssey-graduate-reward-v1` 背包收据：已发盒子只清债，不再投递。旧v2毕业欠奖由新邮件路径处理。毕业事务同时识别新邮件收据，避免先发邮件后毕业又写回欠奖。
- 邮件通知、查看、领取沿用既有NOTI99/CMD95等已取证路径，没有新增opcode、codec、字段布局或DLL。未修改schema、玩家数据库或客户端文件。

## 检查与手动验收

通过：章节配置/资格/锁内证据回归；满包部分发放、毕业后补发和重放；荣誉条件/未知存档字段保留；隔离PostgreSQL并发投递、删除后重试、旧已付收据、满箱回滚与重试；当前服务端 `4d8c0c82…` 内层PVF直读启动及全章ID/数量对照；`go vet ./...`；构建；候选入口 `launch_local.py --check`。隔离数据库位于本任务 `.tmp`，测试后已停止，未接入玩家库。

`go test ./...` 已执行，保留4项失败，逐项在仅撤销本任务代码的HEAD overlay中复现：`TestAdventureAuditProvenanceAllowanceIsNarrow`、`TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback`、`TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader`、`TestShopPilotPVFCurrentCatalog`（SKU3400013空发放）。旧 `TestPVFOdysseyLocalArchive` 使用历史角色锚/已淘汰策略，不能作为当前源启动检查；本次新增独立 `TestOdysseyCompletionNativeStartup` 使用现行角色及内容策略并通过。

候选程序：`.tmp/odyssey-rewards-20261003/wireprobe-odyssey-rewards.exe`，SHA256 `02f58210857003760f1265140fe0185cc86527be936fe1d32852e72f3aa1176e`。同目录 `启动验证.cmd` 使用独立profile和现有玩家环境；它由当前工作区构建，包含同时存在的其它候选代码，未替换默认程序或更新confirmed baseline。

请用户手动操作：

1. 关闭旧游戏会话，运行候选 `启动验证.cmd`。
2. 完成任一章节终点，核对配置物品及数量入背包；重登不重复发放。
3. 用115级奥德赛角色重选，或通关末章达到115级，核对邮箱出现荣誉盒 `10420561×1`，领取后能走既有礼包路径；再次重登不重复。
4. 背包满时清章，清空槽位后重登确认欠奖补发；邮箱满时清理邮件后重登确认荣誉奖励投递。

以上实机操作尚未执行，候选不写成confirmed。当前源码/配置不提前提交用户其它任务的文件；用户确认后按根规则收口。

## 首个剧情节点验收排查（2026-10-03）

用户反馈完成“第一章”后未收到武器盒和战斗支援盒。会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_011800_247986_next37` 确认运行上述候选程序。只读查询玩家存档：角色19为35级，`odyssey_completed_dungeons` 为 `[100004934,100004935,100004936]`，章节奖励收据为空，四种奖励物品均未入包；通关日志与存档一致。

原生journal的第一个 `[chapter]` 包含12个副本（100004934至100004945）。前三个副本属于该章第一个 `[node]`，100004936播放 `ACT1_GreatMagicCircle_outro.avi`；这段剧情结束不等于奖励章结束。章末为时空之门100004945，成长表目标等级50。当前尚未触发第一章奖励，不按ACT1影片或35级提前发放。

同一排查期间工作区发生Git合并（HEAD `6010ad4`）。当时本任务修改的已跟踪文件暂存在stash，而新增奖励文件仍在；`ProgressionService` 缺少这些新增文件依赖的字段，源码不能重建候选。现存候选二进制不受此源码变化影响，以上检查不代表章末实际发奖已验收。

## 合并后恢复（2026-10-03）

用户告知上一轮代码位于git stash并授权恢复。最新stash（基于626f23e）仅含本任务11个文件；恢复其中8个布局未变的文件，把旧main.go接线迁移至bootstrap.go/client_entry.go，把旧pvf_catalogs.go/pvf_odyssey.go改动迁移至internal/gamedata/catalogs.go/catalogs_content.go。原生启动测试也移至internal/gamedata，沿用现行PrepareCatalogs入口。未恢复旧架构文件、未删除stash、未修改合并基线的其它功能。

恢复后 `go test ./...` 和 `go vet ./...` 全部通过；当前服务端PVF `TestOdysseyCompletionNativeStartup` 与隔离PostgreSQL `TestOdysseyHonorMailPostgres` 通过。上轮4项失败属于当时基线，本次合并后的全量运行已通过。通用charactercheck在本任务隔离数据库报缺少 `account_unified_options` 表；撤销本任务代码的当前HEAD只读overlay同样失败，未扩大任务修复该检查器。隔离数据库25458已关闭，未访问玩家数据库。

新候选 `.tmp/odyssey-rewards-20261003/wireprobe-odyssey-rewards-restored.exe`，SHA256 `3ca6bb14223ba5023eb12efcea3c82eea4aa8f281c7925c3fbc16022563e365d`。独立profile `pvf-odyssey-rewards-restored.json`，新入口 `启动验证-恢复版.cmd`，保留原验证入口中的用户环境设置和奥德赛启动脚本，原候选/profile/入口不覆盖。`launch_local.py --check` 路径检查通过；不启动客户端、不更新默认程序或confirmed baseline。需用户关闭旧会话后手动运行恢复版入口，继续完成奖励章终点验证。
