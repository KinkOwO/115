# 巴卡尔重建实现审查（2026-10-06）

> 本报告保留修复前的审查证据及当时行号。核心问题已在同日修复，当前结果、候选入口和仍有缺口见 [修复记录](bakal-fix-20261006.md)。

审查对象：当前工作区 `next151-bakal-raid-reconstruction.md` 及 catalog、legion、workflow、wireprobe 接线。结论：现有实现尚不能按正常客户端流程完成进图、通关和奖励同步。解析器和部分帧构造已完成，但回放测试绕过了关键网关路径，不能作为副本完成的验收依据。

本次仅审查与离线复现；没有修改运行源码、默认配置、可执行文件、玩家存档，没有启动客户端或服务监听，没有提交用户工作区。

## 验证结果

- Go 1.26.0：`go test ./internal/legion ./internal/catalog ./internal/workflow ./cmd/wireprobe -run Bakal -count=1` 通过；wireprobe 明确返回 `[no tests to run]`，没有巴卡尔网关专项测试。
- 上述 catalog 测试最初因未设置归档环境变量跳过。随后设置 `DFO_PVF_CORE_TEST_ARCHIVE=D:\115us\115-server\server\work\client-build\Script.inner.pvf`，重新执行 `TestImportBakalRaidRules`，真实归档解析与断言通过。
- `go vet ./internal/legion ./internal/catalog ./internal/workflow ./cmd/wireprobe` 通过。
- 独立 Go overlay 复现测试：9 个预期正确行为断言全部失败，确认下面所列运行问题。测试只在 `.tmp/bakal-review-20261006/` 保存，通过虚拟 test 文件载入，未加入生产源码。
- 未重新运行全仓测试；任务文档记载的 character 历史失败不在本次结论范围。没有做新的实机验收。

复现命令（模块目录执行）：

```powershell
$env:DFO_PVF_CORE_TEST_ARCHIVE='D:\115us\115-server\server\work\client-build\Script.inner.pvf'
go test -overlay .tmp/bakal-review-20261006/overlay.json ./cmd/wireprobe -run TestBakalReviewRepro -count=1 -v
```

复现源：`.tmp/bakal-review-20261006/review_repro_test.go`；输出：同目录 `repro-results.txt`。断言失败是发现缺陷的证据，不是本次修复后的测试结果。

## 确认的问题

### 1. [P1] 默认启动没有装载巴卡尔规则

位置：`configs/pvf-default.json:4`；`internal/gamedata/catalogs_content.go:582`；`cmd/wireprobe/bakal_flow.go:120`。

默认 `DFO_PVF_CATALOGS` 没有 `bakal-raid`。规则加载受 `selected["bakal-raid"]` 控制，因此默认路径 `pvfCatalogs.Bakal=nil`，奖励服务也为 nil，CMD656 总是返回等待室未绑定错误。把域加入 `SupportedDomains` 仅表示支持，并不会自动选择它。即使重新构建源码，使用此默认 profile 仍无法建团。

复现：`default_profile_loads_raid`，输出 `default profile omits bakal-raid`。

### 2. [P1] 正常进图用的 CMD2062 被吞掉

位置：`cmd/wireprobe/bakal_flow.go:100`。

只要 `w.bakal!=nil`，2062 就记录事件并返回 handled=true，没有修改归属地下城、没有下发进图帧，也不再进入普通 dungeon handler。交接包成功实录第一张图实际通过 2062 进入：12:29:54.491 请求，body 的目标地下城为 100003160，随后 N28/N29 和 2073 加载闭环。当前代码会让之后的 2073 因未拥有地下城而拒绝。

复现：`captured_direct_move_enters_dungeon`，输出 `handled=true packets=0 ownedDungeon=0`。

### 3. [P1] 22/707 的进图路径只发导航通知，没有启动地下城

位置：`cmd/wireprobe/bakal_flow.go:322`；`internal/legion/bakal_opening.go:237`。

处理器没有读取请求目标；只调用 `NextPortalDungeon()` 后返回 N2281/N2285。没有复用原生地下城选图、地图准备链，也没有创建 `w.activeDungeon` 或下发 N28/N29。导航位置通知无法承担地图启动与房间怪物初始化。成功实录包含这条通用进图链，但导出的 241 条 bakal 时间线将这些通用帧过滤了，造成实现遗漏。

复现：`portal_produces_native_map_start`，输出 `no dungeon-start N29; only 2 packets produced, activeDungeon=<nil>`。

### 4. [P1] 终局状态不可通过当前入口到达，且触发地下城选错

位置：`internal/legion/bakal_opening.go:294`、`:450`。

真实 PVF 的有类型地下城只有 100003149..152；最终地下城 100003165 没有 Type。`NextPortalDungeon()` 排除所有 Type 为空的图，所以只能选四张首领图，随后返回 0。然而 `DefeatMonster` 仅在击败 `FinalClearDungeon=100003165` 时进入 Final。

另外，PVF 已区分 `SettlementDungeon=100003149`（击败巴卡尔后 move last bakal dungeon、启动结算计时）和 `FinalClearDungeon=100003165`（终局 clear phase）。实现将前者行为绑到了后者，149 的击败只走普通清图分支，没有终局移动。现有单测直接调用 `EnterDungeon(100003165,24,...)`，绕过了实际进图选择，并使用与 PVF 165 对应位置 56 不一致的位置 24，掩盖此问题。

复现：`portal_can_reach_final_dungeon`，四张候选图清完后 `stage=2 next=0`，仍为 Active。

### 5. [P1] 战报未核对怪物身份，任意战报可直接清首领图

位置：`cmd/wireprobe/bakal_flow.go:240`；`internal/legion/bakal_opening.go:283`。

解码器有 Monster 和伤害字段，但调用 `DefeatMonster` 仅传地下城与位置，完全不使用 Monster，没有确认原生房间实体死亡。只要归属图已加载，一个 Monster=0、伤害=0 的合法长度战报也会让该首领地下城被标记 cleared。实际房内还有杂兵与多阶段首领，不能把任意战报视为整图首领已死。

交接包终局实录也与文档归因不符：12:36:42.015 是 CMD39；12:36:42.046 发死亡确认；12:36:42.047 发 final_selection；2069 在 12:36:42.061 才到达。该场终局由已确认死亡链触发的证据更强，不支持把所有 2069 直接当作死亡确认。

复现：`unrelated_monster_does_not_clear_boss`，输出 `monster=0 damage=0 cleared owned boss`。

### 6. [P1] 结算 N13 被发送成 CMD13 应答

位置：`cmd/wireprobe/bakal_flow.go:419`。

`bakalFrameKind` 仅按 ID 判断，将 ID13 标为 Kind1。结算 `bakal_source_reward_inventory` 实际是标准背包通知，应为 Kind0。同 ID 的双向语义不能决定通知/命令类别。客户端将收到错误分发表的包，无法按背包通知消费其全量正文。普通翻牌链 `card_inventory_committed` 已使用 `{Kind:0,ID:13}`。

复现：`inventory_notice_keeps_kind_zero`，输出 `inventory N13 sent with kind=1`。

### 7. [P1] 冻结奖励失败仍发送通关结束，不能重试

位置：`cmd/wireprobe/bakal_flow.go:360`、`:374`。

`bakalSettleSplice` 吞掉 Freeze 错误，Bootstrap 失败也静默返回；随后 Tick 仍将状态改为 Ended，发送 N588/N574。`SettleDue` 从此不成立，无法再次冻结。数据库暂时不可用等失败可造成客户端显示通关、服务端没有奖励计划，重连 Recover 也无计划可恢复。

复现：`failed_freeze_does_not_end_raid`，用不可用的奖励服务强制 Freeze 失败，状态仍进入 Ended。复现不访问数据库；真实事务失败同样经过上述无条件 Tick。

### 8. [P1] 领奖后没有同步新增物品

位置：`cmd/wireprobe/bakal_flow.go:208`、`:387`；`internal/workflow/bakal_rewards.go:233`。

Freeze 仅保存 Items/Products 计划，不入背包。结算 Bootstrap 因此发送领奖前的背包。后续 Claim 才真正 Grant 并替换 `w.role`，但处理器返回空 packets，没有任何背包更新。即使修复 N13 类型，客户端仍只看到旧物品列表，实际奖励需后续重载才可能显示。

“B 层 CMD13 无应答”不能证明新实现发奖后可以不通知：该实录六个 CMD13 均带 `unimplemented_sample=true`，没有 `bakal_reward_claimed` 记录。它们分别发生在 12:41/12:42 和 12:51/12:52，部分紧邻 657 离队。这组证据不足以将 CMD13 定义为已实现的领奖请求。领奖入口和自动领取时机应回到二进制 Claim/结算调用链确认。

### 9. [P1] 补领奖调用早于角色载入

位置：`cmd/wireprobe/client_connection.go:103`、`:114`；`cmd/wireprobe/world_flow.go:280`。

新建 worldSession 后立即 Recover，此时没有设置 role，ID=0、State为空。Recover 读到空计划 map，直接返回；实际角色在之后选角 `worldSession.enter` 才赋值。全仓只有该处 Recover 调用，因此重连角色的待领取计划不会被处理。应在角色载入后、发送该角色的背包快照前恢复，并处理成功部分与失败部分的角色状态。

### 10. [P2] 已解析的 PVF 规则没有驱动实际状态与帧

位置：`internal/legion/bakal_opening.go:210`、`:216`；`internal/legion/bakal.go:104`、`:464`、`:485`；`internal/legion/bakal_script.go:45`。

- `[PHASE TIME OVER]` 已解析，但 N584 总是 9999；`Remaining()` 读规则却没有接到实际发包。变更源为 4321，开局帧仍发 9999。复现 `remaining_follows_source`。
- `[DUNGEON INFO]` 已解析，但开放列表和进图授权仍由 Go 的固定范围 100003149..165 决定。
- `[CREATE MONSTER]` 已解析，但开局 roster 固定八组位置，不消费 phase.InitMonsters；该处重复了内容源的位置定义。
- `[CHECK TIMER END]` 怒气窗、每秒10点、`[FAIL PHASE]` 阈值8000等已解析但不执行；advanceAnger 始终以每秒1点累计，不检测失败，也不发符号同步。复现 `anger_threshold_ends_failed_raid`：怒气9997、阈值8000，状态仍Active。地图超时失败同样没有完整执行。

这违反任务自身 PVF 单一规则目标：源 `contents/2022/bakalraid/etc/bakal.etc` → catalog 的 Dungeons/InitMonsters/PhaseTimeOverSecs/AngerWindow/AngerFailThreshold 已建立，但消费者仍使用重复 Go 内容定义或忽略这些规则。需要让协议构造接受已解析规则，并补齐触发器执行；不能仅以“解析测试通过”声称规则已收敛。

## 已记录缺口与待证语义

文档已记录 N570 符号、场景怪定时波次、buff 授予/使用、N2194 首领同步、竞价投影等缺口。这些仍影响完整玩法，不能标记为已经完成。尤其 buff 面板有初始次数，但网关没有使用入口和 GrantHook 注入，当前面板不会完成授予/消耗链。

奖励 `freezeItems` 把 `[CHANNEL SLOT EXPECTED REWARD ITEM]` 全部作为固定奖励，并把 `[MONSTER PIECE REWARD]` 的两个 rank/template 都发一份。同一模板因此会发两次，rank 没有参与选择。现有测试用同一替代模板并断言份数，无法证明真实奖励语义。这是需要二进制与源引用表进一步核对的项目，本次不将实际应发数量写成确定结论，也不凭标签重新猜一套发奖表。

周清双计数器已按文档 A 层证据重建，本次没有仅凭常识否定其语义。

## 建议处理顺序

1. 接通默认内容加载及原生地下城进入链；确认2062/22/707目标解析和 N28/N29、activeDungeon、2073闭环。
2. 接入真实实体死亡确认与 SettlementDungeon → FinalClearDungeon 的阶段转换，覆盖源149→165的实际入口测试。
3. 修复通知 Kind；重建可靠的冻结/发奖/背包同步/恢复时机，使失败可重试。
4. 将开放图、开局怪、倒计时与触发器迁回已解析 PVF 规则，再补齐文档已有机制缺口。
5. 以原始 events.jsonl 的完整事件集做网关回归，保留通用地下城帧；最后由用户手动实机验收。

原始证据：`D:\115us\实时抓包\115us\server\work\dfo-lan\runtime\roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261005_200137_561796_next37\events.jsonl`。辅助 241 条 bakal 时间线仅用于定位，不能替代完整事件流。
