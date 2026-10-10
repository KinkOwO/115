# 合并后巴卡尔兼容修复候选（2026-10-07）

用户授权“开始修改”。本轮保留其它总仓/用户改动，修复实际配置与维护入口问题；用户随后明确“不恢复了，全部按最新的来”，因此使用当前SQLite，旧PG不迁移、不覆盖、不删除。

## 已改动

1. 默认profile的DFO_PVF_CATALOGS补回bakal-raid，保留binary=wireprobe-pvf.exe和其它域/策略。PVF源17个地下城、52个位置、4个Boss重新接到生产bootstrap。
2. Go启动器恢复工具按tasklist CSV第一列检测DFO.exe和所有wireprobe*.exe，覆盖handoff-source、Bakal候选与大小写。不把其它CSV列出现的名称误当进程，解析失败阻止写入。新增进程族/错误CSV/无关进程回归，--help返回0而不打开存储。
3. 配置契约向总仓已合并默认值对齐：VenusFlipGear=configs/venus-flip-gear.generated.json、BoostUpEvent=true，help保留新增参数；新增回归确保用户仍可显式覆盖。没有修改这些功能的运行规则，也没有删除失败测试。
4. 新增纯ASCII、CRLF的scripts/bakal-reset.cmd，使用bin/dfolauncher-bakal-compat-candidate.exe调用同一BakalReset事务，支持--help/--dry-run。不会启动游戏；实际--apply要求游戏/网关先停止。

当前dfolauncher.exe仍在运行，未热替或发布已确认默认启动器。编译了独立启动器修复候选，新的恢复脚本直接使用它；原游戏启动链不变，退出后由用户手动验证。新检出若缺候选，可在模块下执行`go build -trimpath -o bin/dfolauncher-bakal-compat-candidate.exe ./cmd/dfolauncher`。

## 验证结果

- go build ./...、go vet ./...通过。
- 显式实际DFO_PVF_CORE_TEST_ARCHIVE的Bakal/RaidWeekly/RaidRecovery/Reset及配置专项全部通过：catalog、legion、protocol、workflow、dungeon、恢复工具、wireprobe、dfolauncher。
- 默认profile只读PVF准备通过，并输出Bakal dungeons17/locations52/bosses4；报告storage_accessed=false、runtime_started=false。调用补上启动器实际提供的-equipment-wear-rules参数；此前直接调用缺该参数的open空路径是验证参数不完整，没有通过改内容域/猜规则绕过。
- 新启动器check默认/--source-build路径通过，仍为SQLite；launch --dry-run、恢复--dry-run、CMD --help通过。在线恢复调用被进程保护拒绝，未打开存储写入。
- 全量go test ./... -count=1已消除本轮配置/帮助/Bakal门禁失败，只剩合并前两项角色来源基线：TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference。本轮没有改变职业/技能来源校验来强行变绿，CI仍需单独修复这两个来源兼容问题。

原默认网关与handoff-source当前指纹76c54a0b9d02d1e175fbeccf0f9e4e87edad38d4f5daaaf4b064e6693b0b0b9f，网关业务源码未因此改动。修复启动器bin/dfolauncher-bakal-compat-candidate.exe指纹6b44036320b46c9abb00b1376904d00a8f7d40409b8824138f3ffa13be58f088，原在用dfolauncher.exe未覆盖。

日志在.tmp/bakal-merge-fix-{native-tests,all-tests,prepare,launch-plan}-20261007.txt。未提交源码、未操作游戏、未覆盖当前SQLite或历史PG。Final Strike自然结束/竞拍UI仍是既有未完成项，不将本次兼容修复候选记作完整玩法确认。

## 用户验证

先正常停止当前游戏环境，再手动scripts/启动游戏-SQLite.cmd（或--source-build）测试巴卡尔建团、进入、库存/周次数。恢复工具改用scripts/bakal-reset.cmd；`--dry-run`仅预览流程，普通执行恢复probe账号次数并保留其它存档。旧已确认启动器保留，候选实机确认后再走正常发布流程。

## 退出后补检

用户报告已退出游戏后，重新检查未发现 DFO.exe、wireprobe*.exe 或 dfolauncher*.exe 在运行。候选启动器 check --source-build --server-only 及 launch --source-build --server-only --dry-run 均通过，没有启动服务或客户端。

最新会话 roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261007_181233_041780_next37 的 events.jsonl 最后一条为 19:52:10 的 menu_exit_session_closed。未检出 Bakal raid prepared 或巴卡尔业务事件，该会话不能作为修复后的巴卡尔回归确认；它在默认配置修复之前已启动。

追加执行 DFO_PVF_ARCHIVE 指向实际内层归档的 go run ./cmd/dfo-tool charactercheck。工具使用 database.OpenTestFixture 的独立临时 SQLite，不读取玩家存档。模块状态恢复、教程、出生路线、奖励存储、疲劳房间、任务地图目标、成长存储检查通过，但最后以 quest source experience mismatch 退出 1。定位于 internal/toolcmd/charactercheck/quest_reward_check.go 的 receipt.Experience != 1200 断言；尚不能判断是校验基线还是任务奖励执行问题，不将其算作通过，也未修改任务奖励以绕过。该项与既有两项角色测试一起列为全仓兼容门禁未闭环项。

保留默认程序和已确认基线。用户下一次手动运行 scripts/启动游戏-SQLite.cmd --source-build，重点验证巴卡尔建团、进房/换房、三龙及本体入口、NPC 次数和周次数。次数恢复脚本仍使用隔离维护候选，不自动恢复或迁移旧存档。