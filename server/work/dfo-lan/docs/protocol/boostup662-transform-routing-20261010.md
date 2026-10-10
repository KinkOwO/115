# Boost 662 教程装备变换分派纠错（已确认）

## 证据与根因

用户报告第十步“变换为添加到装备库的装备”确认后无法完成。当前会话为
`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_034251_994078_next37/`：

- `events.jsonl:958..963`：角色 3 进入第十步，领取后 N2638 为 `010a020001`，即 step=10/phase=2。
- `events.jsonl:966,974`：两次原生 CMD2259 请求，u32@0=46，u8@12=0，付款方式 1，槽 14/模板 100051289。
- `gateway.err:329..332`、`events.jsonl:967,975`：两次误走生成，装备分别落背包槽 15、16；没有执行替换穿戴装备的变换事务。

旧 `boostJournalSwap` 把 u32@0=36 当教学窗口身份。此前采样 36 无法覆盖本次 46，故 action=0 退入生成。这次确认并非材料不足。

## 真源链

本次只读打开的内层 PVF SHA256 为 `4d8c0c82192eb72806d4bec803f5ab638e91e24401d38110cb78fcb09f69e37f`，与该会话源身份一致。离线提取位于 `.tmp/boost-transform-source/`，不作为运行输入：

`live/event/kor/2026/0326_boostup/boostup.evt` 第十条 `[step info]` → `[mission][type] transform equip journal or equip item`、`[equip grouping]`/`[count] 1` → 现有 `boostup.Load/Parse` 的 `Catalog.Steps` → 网关已激活、未毕业、已领取且 phase=2 的源任务分派 → 现有 `workflow.ItemService.TransformEquipment` → 角色/账号材料原子事务 → 穿戴 N14、背包 N13、图鉴 N2610 与 `reconcileBoostEquipment` 的源条件复核/N2638。

不以 step=10、模板号或装备分组清单作为新增运行常量；测试将任务移到 step=1，再改变源任务，证明分派跟随规则。

## IDA 边界

权威 `client/DFO.exe.i64` 与已打开 `.tmp/boost-skill-ida/DFO-copy.i64` SHA256 同为 `705d3525d2e098ea133fdd287c979ac42068f6b400f0b04e1280272429b1cb86`。IDA 单 worker 已占用，复用这个逐字节一致的副本只读取证，未修改/保存 IDB：

- `1414F4480` 初始化请求 +13、14 条 +17 起的 7 字节装备记录及 +115..120 尾部，没有初始化前 13 字节。
- `14150C4D0` 的 a2=0 分支调用该构造器，从选择控件取装备记录，写 +13 的付款字段，`14150CEDC/14150CEF6` 用 CMD2259 发送 121 字节原生结构。
- `145277D00` 仍读成功前缀后的 6 字节，按字节 @4/@5 选择窗口/方法。此次保持既有应答布局和刷新序列。

上述证据推翻“u32@0=36 是教学窗口固定身份”；不声称已完全定义前 13 字节的所有语义，也不把普通生成/变换的历史 header[12] 分派扩写成新的原生协议结论。

## 本轮改动与验证

本轮分派纠错 attempt 1/3：只把错误窗口常量改为当前源任务和领取事实的门禁；没有新增出站包或客户端补丁。测试包含原始 128 字节实机向量的实际网关分支回归，以及源任务改变/步骤迁移、未激活、未领取、越界、毕业和活动缺失的隔离边界。

正式/default 和 39 归档保持；候选编译到 `bin/wireprobe-handoff-source.exe`，旧候选备份 `.tmp/boost-transform-routing/wireprobe-handoff-source.before.exe`。没有修改存档/schema/PVF，兼容现有角色、物品和回执；本次误生成的两件装备不自动删除。

Go 1.26.0，`GOPROXY=https://goproxy.cn,direct`、`GOPATH=C:\Game\dof\115us\tools\gopath`、`GOCACHE=C:\Game\dof\115us\tools\gocache`、`GOTOOLCHAIN=local`。实际执行结果：

- `go test ./cmd/wireprobe -run TestBoostJournal -count=1`：通过。
- `go build ./...`、`go vet ./...`：均退出 0。
- `go test ./... -count=1`：退出 0，无失败集合；wireprobe 67.091 秒、catalog 78.885 秒。
- `go build -trimpath -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe`：退出 0，31,027,200 字节，SHA256 `8d3b2d50f57d5ac8f43501b26dce978113d84c8ffd88405e4b922a430befc65b`。`go version -m` 自述 Go1.26.0、git 8081329022a400d5a29749642aaea4c995ff4ef8、vcs.modified=true。
- 旧候选 SHA256 `6b93bfdc34c8ffdbeee6a91c7ec82595c8068018d064e5f8382cde3840118325`；`git diff --check` 通过。

用户已手动复测并确认：“已确认教程推进，可提交”。040737_176050_next37会话 `events.jsonl:229..235` 记录同形CMD2259、槽14从100051288变为100051289、applied=true、gold=0及N14/N13/N2610/N2638；该程序纳入本功能 [confirmed baseline](boostup662-transform-confirmed-baseline-20261010.md)。默认程序保持，复验入口为 `scripts\启动游戏-SQLite.cmd --source-build`；AI 未启动或操作客户端。提交前检查origin/fork均未配置，沿用当前gud/main同步：fetch后HEAD与gud/main为0/0，无需合并；仅提交本任务文件。

## 已有关联规则未收敛

源 `boostup.evt [discount cost]` 已定义 step/state/condition/gold/material；`boostup.JournalTransformDiscount` 已解析，但 `inventory/equipment_journal_operations.go:PrepareEquipmentTransform` 仍使用 `TutorialMode && unpriced` 的教学免单特例。该代码注明此前业主授权偏离源，当前沿用以隔离分派纠错；本轮没有修改费用、复制新的玩法表或新增 JSON 回退。后续须在客户端折扣语义与旧特例适用范围闭环后，决定接通既有折扣 reader 或明确保留授权差异，不能把这次路由修复记为费用规则迁移完成。

网络对照仅作线索：[当前 Boost 活动](https://www.dfoneople.com/news/events/4871/Sky-of-a-Thousand-Seas-Boost-Up)、[官方装备库/变换说明](https://www.dfoneople.com/gameinfo/guide/Advanced-Game-Information/Equipment-System/Armory%2C-Equipment-Conversion-System)。执行判据以本地同源 PVF、当前客户端和本次原生实机帧为准。
