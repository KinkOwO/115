# MR !177 合并冲突修复（2026-10-07）

## 原因

来源 fork/main=382f4452（sync）只有单亲 429a283c。该提交把总仓文件复制进工作树，实际共同祖先仍是 5c6fb951，未把已复制的 ee290b2c 及其后总仓历史作为 merge parent。GitGud 因此显示来源落后 193 个提交，真实三方合并产生 57 个冲突文件。不是截图中的 CI 失败：该页 Pipelines 为 0，阻断原因是 merge conflicts。

## 合并原则

独立工作区 D:/115us/bakal-mr177-merge，分支 codex/bakal-mr177-merge，从已推送的来源 382f4452 合入 origin/main=2625f031。保留双方历史，不 rebase、不 force push。原工作区 D:/115us/115-server 的两项已暂存 protocol/raid_bakal 文件删除保持不动，不夹带进独立工作区。

用户明确要求巴卡尔只用本地版本、远端重复实现删除。合并结果保留 cmd/wireprobe/bakal_*.go、internal/legion/bakal*.go、internal/catalog/bakal*.go、workflow/bakal*.go 和本地 native 115 包向量；删除远端 cmd/wireprobe/raid_bakal*.go、internal/raid、catalog/raid_bakal*.go、workflow/raid_bakal_rewards*、protocol/raid_bakal* 及仅用于该旧实现的 raid_team/raid_vote/raid_symbol/raid_assignment/raid_owned_details 测试。重复 raid_entrance codec 使用本地 raid_entrance115.go，入口 reader 调用本地 ImportBakalRaid。没有用删除现行本地测试来掩盖失败。

共享挂点逐处保留：dispatchBakal + dispatchForest + dispatchBoostEvent；本地 tickBakalOpening 和总仓 forestStageTimeout；本地 phase/room/coin/potion/weekly/recovery 与总仓 forest/venus/boost/oath/clone 行为。目录 parser 保留本地 raid header level 解析与总仓 Odyssey boss entrance condition 解析；源码域及 legacy repair whitelist 均保留 bakal-raid。

启动器采用总仓 SQLite-only 基础，再接入本地 bakal-reset 与所有 wireprobe*.exe 在线保护，移除误带回的 PG initstorage/storage 文件；没有迁移、恢复或写入玩家存档。源分支已去掉的大工具链归档不重新添加。前端发布 exe、IDA 索引、非巴卡尔文档采用总仓版本；默认网关与39归档不在本轮发布或覆盖。帮助 fixture 对齐实际合并后的声明。

## 验证与边界

全仓 go build ./...、go vet ./... 通过。实际内层 PVF 的 Bakal/RaidWeekly/RaidRecovery/Reset、配置契约、Forest/Venus 及白名单专项通过。全量 go test ./... -count=1 只有既有 TestEntrySkillsPreservePayloadAcrossProfessionHashChange 和 TestEntrySkillsRejectDifferentProfessionReference 两项角色来源失败，未改职业/技能来源校验绕过。

check-commit-hygiene.ps1 -All 门禁通过，产物和大工具链归档不入库。日志保留独立工作区 .tmp/merge-{all-tests,native-tests,hygiene}.txt/json（不提交）。未启动游戏，未实机验收，Final Strike 自然结束和竞拍 UI 仍未完成；charactercheck 的 quest source experience mismatch 是上一轮独立记录的未闭环项。

合并提交必须让 2625f031 成为来源分支祖先，普通推送到 fork/main 后 MR !177 才会更新冲突状态；若目标分支期间有新提交，需要再次 fetch/merge 及验证。

额外尝试对全仓设置 DFO_PVF_CORE_TEST_ARCHIVE 后，历史真实归档门禁另有 11 项失败：多数绑定旧 8b2a9f83/7ef2db59 指纹，当前归档为 b814d76e；还有 scene selection changed、缺少 items.index.json、要求 DFO_PVF_CORE_TEST_SHA256 的迁移见证配置。该额外完整运行日志保存在 .tmp/merge-native-all-tests.txt，不能宣称全仓原生门禁通过。巴卡尔专项使用同一当前归档已通过。本轮不改这些无关校验来绕过来源约束。
