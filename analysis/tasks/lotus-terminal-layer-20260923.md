# 副本 26「击败罗特斯」末层黑屏：取证检查点

状态：用户实机确认任务现可完成，但结尾仍黑屏。提交当前已确认的任务推进修复；黑屏单独保留为未解决问题。C2S attempt 1/3 已验证切图成功；完成链 attempt 2/3 已验证任务可完成。

## 现场

- 会话：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260923_165320_596250_next37/events.jsonl`，角色 11。
- 源任务 3215（`behemoth_12.qst`）为 `[clear map] 53543`；源副本 26（`2ndbackbone.dgn`）迷宫 3 的 Boss 坐标为 `(3,0)`，层序列恰为 `[100008786, 100008697]`。
- 08:55:53 进入源 Boss 房 `53543`；08:55:55 有 `0x102b` 死亡确认；08:55:58 和 08:55:59 两次 CMD 45 切至 `100008786`、`100008697`。
- 08:56:03 在末图 `100008697` 确认 `0x102d` 死亡。08:56:05 客户端又发 CMD 45，160 字节，位置 `(3,0)`、`LayerChange=1`、副本 ID 26。其 18 字节附加记录为 `000000000405bd02e5000000030002000000`；前两次切层的对应记录均为 `000000000405000000000000000000000000`。
- 服务端 `internal/dungeon/scene_transition.go:MoveScene` 只按 `layer.Maps[currentIdx+1]` 前进，因此在末图返回 `no next layer map`；本次请求没有 ACK45 或 NOTI29。会话中没有 CMD117、NOTI115、NOTI31 或 CMD46。

## 客户端静态锚点

- 权威 `client/DFO.exe.i64` 已在 IDA MCP 中以只读查询；opcode 表确认 CMD 45 = `ENUM_CMDPACKET_MOVE_MAP`，NOTI29 = `START_MAP`，NOTI31 = `ENABLE_CLEAR_DUNGEON`，CMD117 = `BOSS_DIE_CHECK`。
- 从当前 IDB 字符串交叉引用定位 `NOTI29` 处理函数 `0x1452B7100`（`NOTIFUNC_ENUM_NOTIPACKET_START_MAP`）和 `NOTI31` 处理函数 `0x1452AE550`。尚未证实末图 CMD45 附加记录的字段语义和客户端等待的具体应答。

## 新增取证与候选修复（attempt 1/3）

- 当前客户端 PVF 中，末图 `100008697` 的 `Action/14948.act` 在怪物 70160 消失后播放 `q3215_14949.cmt`。该 CMT 的末场景明确为 `[CHANGE MAP]`，坐标范围 X=701..704、Y=229..231；请求记录中的 `bd 02 e5 00` 是小端 X=701、Y=229。当前 `2ndbackbone.dgn` 的迷宫 3 层序列仍只有上述两图，并未发生配置导出缺图。
- 权威 IDB 的 `NOTI29` 处理函数 `0x1452B7100` 在 `0x1452B7793..0x1452B788B` 对 layer flag 1、mode 0 的分支会将层索引前进并钳在最后一层，读取缓存层图；随后 mode 0 不读取地图/怪物初始化行。这是“末层原图复用”的客户端消费路径。
- 服务端仅对副本 26 / 迷宫 3 / 地图 100008697、准确的 CMD45 记录结构和 CMT 坐标范围放行。`MoveScene` 复用已访问的末图与死亡状态；`START_MAP` 返回 layer flag 1 + mode 0 + 原始 18 字节记录，不重新刷怪。其它末层请求仍被拒绝。
- `go test ./...` 和 `go vet ./...` 全部通过；候选版已在最后一次改动后重新编译。`bin/wireprobe-handoff-source.exe` SHA256 `D53E4A646910A47B8FCC0EA89A9FF52AD8A5A9DF9CDD42DF0505DDFF690FBB54`。

## 实机验收

### 首轮回归：2026-09-23 17:41:37

- `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260923_174137_332890_next37/events.jsonl`：末次 CMD45 在 L555，ACK45 在 L556，NOTI29 在 L557（layer flag 1、mode 0、原始记录），客户端 CMD37 在 L562，服务端在 L563..L570 完成加载应答。因此此前的 `no next layer map` 阻断已经解除。
- 用户仍见黑屏；整局没有 CMD117、NOTI115、NOTI31、CMD46。末次 CMD37 后没有进一步通关事件。L559 的 CMD585 是 `SECURITY_STATUS`，不是剧情结束指令。
- 当前 `completion.go:tryComplete` 在 `completionTarget==0` 时直接返回，且 `CompletionTarget()` 在这种情况下无法提供 `BossCheckConfirmed` 所需身份。L530 的末图 NOTI29 中，75099 为实体 `0x1032`、team100 的展示 Boss 行，另有 team0 的剧情表演 Boss；已有副本 15/25 的实机修复资料证实这类剧情末图不会产生 CMD117，但仍需通过 NOTI115/31 完成链。

### 第二候选版（attempt 2/3）

- 仅在副本 26 迷宫 3 的结尾 `[CHANGE MAP]` 请求通过原有校验、且末图加载后房间已清时，允许完成。初次进入末图、怪物未死、无对应过场请求时均不完成。
- NOTI115 身份取末图实际存在、team100、rank3 的展示 Boss 实体；不伪造 CMD117 请求，也不改动普通副本判定。
- CMD37 的 `completeDungeon()` 失败现在记录 `dungeon_completion_error`，避免静默吞错。

第二候选版 `go test ./...`、`go vet ./...` 全部通过；候选版 `bin/wireprobe-handoff-source.exe` SHA256 `0A938DCF0950404FB402212C87BC08253EAFFEABF6CA489283E135AB9797363F`。旧服务由用户手动关闭后编译替换。

### 用户确认检查点：2026-09-23

- 用户实机反馈：副本任务能够完成，但画面仍黑屏。至此确认任务完成链恢复；黑屏的客户端表现仍未解决，后续应按新日志继续调查，不将其写作已修复。

用户手动从任务 3215 进入副本 26 迷宫 3，打完末图目标后观察黑屏是否解除，任务和结算是否推进。最新 `events.jsonl` 应出现末次 CMD45 对应的 `dungeon_move_ack` 与 `dungeon_next_map_sent`，且后者 payload 为 layer flag 1、mode 0；`no next layer map` 应为 0。若仍无通关，则继续分别检查 CMD117、NOTI115、NOTI31、CMD46 和任务触发，不能把位置切换成功视为整个任务已完成。

## 原检查点

已证实阻断点是末层 CMD45 被服务端拒绝；源配置并不缺少中途地图。最后一次请求的附加记录与前两次不同。任务进度依赖 `completeDungeon()` 后的 `MapClear()`；原会话没有完成确认，所以任务目标保持未完成。

待查：任务能够完成后，客户端仍显示黑屏的具体等待条件。需结合后续会话确认 NOTI115、NOTI31、CMD46 及剧情暂停事件的实际顺序。
