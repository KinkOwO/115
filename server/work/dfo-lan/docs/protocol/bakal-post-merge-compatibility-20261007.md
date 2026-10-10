# 合并总仓后的巴卡尔兼容检查（2026-10-07）

检查对象：合并提交cc03724e（merge: origin/main 合并总仓（巴卡尔副本数据以本地为准））及当前未提交工作区。结论：核心修复大部分完整保留，但当前默认运行环境尚不能判定完全兼容，存在实际配置/启动链问题及验收缺口。

本轮仅检查、运行测试、编译源码候选及写此报告；未修改用户现有配置/源码、未暂存提交、未启动/操作游戏、未重置玩家次数。编译输出为规范的bin/wireprobe-handoff-source.exe，未覆盖默认wireprobe-pvf.exe或39版归档。

## 需要先处理的问题

### 1. 当前默认profile没有启用巴卡尔目录

configs/pvf-default.json第4行的DFO_PVF_CATALOGS缺少bakal-raid。合并提交HEAD里的profile仍有该域，但当前工作区修改将其去掉，并将默认程序改为wireprobe-pvf.exe；不能把这一问题误归因于专用代码被合并覆盖。

生产链为profile env → gamedata.PrepareCatalogs selected["bakal-raid"] → Catalogs.Bakal → bootstrap.go1686/1687 → worldSession.bakalRules/bakalRewards。域未选中时两个依赖为nil，建团在bakal_flow.go224附近拒绝。Go启动器的路径check能通过，并不证明内容域已加载。

证据：显式原生专项唯一巴卡尔失败为TestBakalDefaultProfileAndNoticeKind：“default profile omits native raid”。最新174403会话gateway.err有各域准备记录，但没有PVF bakal raid prepared。先恢复当前profile的bakal-raid内容选择，并保持其它已合并域/用户策略；不靠删除门禁测试或伪造raid内容解决。

### 2. 次数恢复的新源码与当前启动器exe不一致

工作区cmd/dfolauncher/main.go已新增bakal-reset，工具RunFromConfig也已接入，但bin/dfolauncher.exe --help没有此命令；实际以--dry-run调用返回unknown subcommand。原根恢复CMD已被用户删除，scripts中也没有巴卡尔恢复替代CMD。

当前EXE指纹824ffe244d7717b35fa1d7eac5547712e3a1fec10b15efa6e15aa345957793e4。源码能build，但维护入口当前不可用。需要按现有Go发布流程构建并接入启动器，或提供scripts下入口；本轮未替用户发布默认启动器。

### 3. 新恢复入口的在线进程检测漏掉合法程序名

cmd/dfolauncher/main.go24的bakalResetImages只有dfo.exe、wireprobe.exe、wireprobe-pvf.exe；sessionProcessesRunning按这些固定子串检测。因此wireprobe-handoff-source.exe及wireprobe-bakal-*-candidate.exe不匹配。仅启动源网关/候选或远端玩家连接时，可能允许在线重置，随后被会话旧值覆盖。

ApplyGrant通过数据库事务安全修改JSON，但不会自动使所有在线worldSession.role缓存重新读取。新增入口注释宣称覆盖wireprobe在线场景，当前名称集合不足。应识别wireprobe系列并覆盖source-build/候选名称，再补相应回归；不能通过绕过在线保护让工具可用。

### 4. 全量检查未全绿，配置契约需要对齐

本轮go test ./...失败项：

- TestBakalDefaultProfileAndNoticeKind：上述内容域遗漏。
- TestWireprobeConfigLegacyContract（7个子用例）及TestWireprobeConfigHelpAndCLIRejection/help...：源默认VenusFlipGear为configs/venus-flip-gear.generated.json，而快照期望空；BoostUpEvent默认true，而快照期望false，help也多出两类相关参数。
- TestEntrySkillsPreservePayloadAcrossProfessionHashChange、TestEntrySkillsRejectDifferentProfessionReference：此前已有的角色来源兼容失败。

配置/help差异不直接证明巴卡尔协议坏了，但会使仓库完整CI失败。需审核新增默认是否符合上游意图再更新契约，不能只删测试。用户正在修改这些fixture，本轮未覆盖。

## 已通过与源码保留情况

对合并前a314ae96清单的58份专用源码/测试/向量逐一核对：全部存在，57份Git blob未变；唯一变化是protocol/raid_entrance115.go追加通用辅助函数，原有部分未改。当前用户对bakalreset的工作区改动另外检查，仍保留只改clears/rewards及原账本审计的实现。

分发仍在beforeClientTypeDispatch的Bakal专属阶段；新raid_channels仅记录未被处理的请求，新通用协议辅助尚未直接取代旧路径。二阶段、同格点传送、空arena回普通房、重开图标清空、怪物实体去重、复活/药剂预算、死亡回营与恢复时间、库存恢复、周次数与事务回放等服务器回归没有新增失败。

明确设置DFO_PVF_CORE_TEST_ARCHIVE为实际server/work/client-build/Script.inner.pvf，运行catalog/legion/protocol/workflow/dungeon/bakalreset/wireprobe/launcher的Bakal/RaidWeekly/RaidRecovery/Reset专项：除上述默认profile门禁，相关包和功能用例通过。包含总仓新奥德赛巴卡尔最后房间用例；这不是对攻坚战的实机逐房验收。

go build ./...与go vet ./...通过。新源码候选handoff-source指纹76c54a0b9d02d1e175fbeccf0f9e4e87edad38d4f5daaaf4b064e6693b0b0b9f。Go启动器check默认和--source-build均路径通过，均指向SQLite；仅做check，未launch。

检查末尾复查发现默认wireprobe-pvf.exe也已更新为76c54a0b...（检查早期为1293028f...）。本轮的编译命令只指定handoff-source，没有Copy-Item/发布默认操作；据最终文件状态，两个网关程序当前指纹相同。当前profile仍缺bakal-raid、启动器仍为旧824ffe24且无bakal-reset，所以上述配置/维护入口问题仍存在。结论以末尾状态为准，不能仅用较早的默认程序指纹判定网关仍旧。

另有协议收敛注意项：旧legion.DecodeBakalStartRaid要求至少32B及feffffffff标记，新protocol.DecodeRaidStartRequest接受0/8/16零体，两者不是同一输入契约；新通用函数目前只有协议用例消费者，Bakal仍走旧已回归路径。本轮没有无新实机向量就替换解码器，不宣称所有客户端版本都已统一兼容。

## 实际存档检查与历史边界

最新gateway.err明确打开SQLite：runtime/storage/dfolan.sqlite3，活动local.json也是driver=sqlite。按Python sqlite3 URI mode=ro只读检查，当前账号probe、1个角色、角色JSON均有效，0份bakal_raid_rewards账本。没有写入玩家库。

这只能证明当前SQLite数据可读，不能证明此前PostgreSQL角色7的装备、周次数及回执已迁入。历史pgdata目录仍存在；若用户需要那份旧存档，须按当前根AGENTS §0.6的旧版本sqliteconvert恢复路径单独迁入独立SQLite，不直接覆盖当前库。临时SQLite的奖励/预算/恢复/管理事务回归通过，也不能代替旧玩家档迁移验收。

Final Strike自然结束、通关竞拍UI仍沿用合并前未完成状态；NPC库存显示与周次数窗口此前是源码候选，缺本轮巴卡尔实机画面。当前174403实录没有巴卡尔新一轮进图/通关，不能把普通副本日志作为巴卡尔实机确认。

## 本机检查产物

`.tmp/bakal-merged-native-tests-20261007.txt`、`bakal-merged-all-tests-20261007.txt`、`bakal-merge-preservation-audit-20261007.json`及只读检查脚本。无需提交临时产物。审核建议先处理profile与恢复入口/在线保护，再通过CI，最后由用户手动新开Bakal实机回归。
