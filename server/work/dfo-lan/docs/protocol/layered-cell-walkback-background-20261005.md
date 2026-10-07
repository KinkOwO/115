# 走回头路后房间背景全黑（贵族机要 100004968 层图格）

状态：C2S45 **attempt 2/3 已由用户实机确认「没有问题」，纳入已确认基线**（`bin/wireprobe-pvf.exe` = `2dd64fc0…`）。attempt 1/3（flag 2 + mode 0 恢复 base）实机背景不再黑、角色可见，但恢复的是入场那间宫殿 —— 内容选错房间，已被用户否定。未改客户端、DLL、数据库或玩家存档；回滚只需恢复旧候选程序。

## 现场

用户报告「走回头路后房间背景不对」，附两张同房间截图：正常那张有完整远景（宫殿、雪山、天空），异常那张近景道具（铁丝网、红黄路障、木灯柱）位置完全一致，但整片远景区域发黑。

用户实机会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261005_154639_264381_next37/events.jsonl`，副本 100004968「贵族机要」，迷宫 0。

- 07:48:01..:02 入场 (0,2)：base 100015973 → 层图 100015974 → 层图 100016356（后两包 CMD45 layer flag=1，均带 map 行、真建图）。07:48:07 前进 (0,1)/100016357。07:48:12 **回头进 (0,2)**：NOTI29 = `00 02 00 …` 34 字节，layer flag 0 + mode 0 复用，无 map 行无怪物行。07:55:54、07:56:16 两次重试同形态。
- 08:17:44..08:18:29 第二次重走（本次判据来源）：入场同前，一路推进 (0,1)…(6,1) 七间战斗房，然后连续回头 7 格 (5,1)(4,1)(3,1)(2,1)(1,1)(0,1)(0,2)。七包 NOTI29 **逐字节同构**：`pos / flag 0 / 默认18字节记录 / 00 00 ff`（34 字节），每包客户端都在 ~260ms 内回 CMD37 + NOTI30 装载完成。玩家对六个普通格一路跑过不停，只在 (0,2) 停 2.5 秒后 CMD42 退本。
- 用户确认：六个普通房背景正常，只有 (0,2) 黑。⇒ 不是「mode 0 复用从不恢复远景」，问题专属 layered 格。

## 源与原生证据

- `contents/2026/aradodyssey/dungeon/33_wahrheit/nobless_code.dgn` 的 `[map specification]`：`map 0 2 100015973` + `layered 0 2 100015974 100016356`，`[start map] 0 2`。这一格既是起点又挂两张层图，是全局唯一的 layered 回头格。
- 层图 100016356（`map/100016356_cc.map`）的 `[background animation]` = Rokeren `Back_Far.ani [distantback]` + `Back_Middle.ani [middleback]`，近景道具 `Fence_01/SteelBarricade_04/Wall_07/Wall_08` —— 与两张截图里都还在的那批道具一致。base 100015973 用另一套 Shonan 远景。
- 已实机确认的原生 NOTI29 语义（`docs/protocol/evil-justice-quest12893-layer-revisit-20260929.md`，attempt 3/3 基线）：flag 1 进入/推进层图；**flag 0 在 `0x1452B787F..7886` 设完层标志直接跳 LABEL93，绕过层序号清零**；只有 flag 2 走 `0x145B45C50(...,-1)` 写 `dungeon+544` 序号；`0x145EA25F0` 按序号有效性在层图与 base 之间选渲染房间；mode 0 在 `0x1452B78F0` 跳过地图与怪物初始化。

## 原因

`cmd/wireprobe/dungeon_flow.go` 的通用奥德赛回访分支（`visited && Odyssey && !LayerChange`）不看目标格有没有层序列，一律给 flag 0 + mode 0。在 layered 格上，客户端此时层序号还停在末张层图、flag 0 又去选 base 描述符，两边拼在一起：层图的近景道具留存，而该格 base 描述符的远景 `[background animation]` 层没人装配 —— 背景全黑。同一包形在无层序列的普通格上没有这半歧义，所以背景正常（本次重走的六格即是判据）。这条 flag 0 + mode 0 正是 Evil Justice attempt 2/3 被实机否定的形态，被这条通用分支重新发了出来。

## 修改（attempt 1/3）

只动服务端运行路径，包形态复用已确认基线，不新增包、不删挡路物、不伪造清场：

- `internal/dungeon/scene_transition.go`：新增 `Session.LayerRevisitResume`。判据全部来自源与已访问记录：`Definition.Odyssey`、当前格有层序列、序列末张已访问（= 已播完）、该格 base 图存在且不等于末张层图、且 base 图已被客户端真建过（`Visited` 命中，mode 0 复用才有缓存）。命中时把服务端房间换成 base 图并登记 resumed base（抽出 `markResumedSceneBase`，与 `MoveScene` 的 `SourceLayerResume` 同一套机制）。`MoveScene` 里原来手抄的 map 复制改调该 helper，行为不变。
- `cmd/wireprobe/dungeon_flow.go`：普通 `Move` 分支接 `LayerRevisitResume`，命中后把 NOTI29 改成已确认的 **flag 2 + mode 0**（`ExitLayer`+`ReuseRoom`、无怪物行），记录用 StartMap 的原生默认那份。

### attempt 1/3 实机结果

用户反馈「场景不对啊」并附截图：远景背景**不再发黑**、角色可见可操作、小地图是 Retreat —— flag 2 清层序号后选 base 缓存这条原生语义被实机证实，「背景全黑」的机制判断成立。但恢复出来的房间是**入场那间宫殿庭院**（NPC、樱花、城墙，即 base 100015973），而玩家离开 (0,2) 时站的是演出层图 100016356。内容选错了房间。

这与 Evil Justice 的结论并不矛盾，差别在**这一格的 base 是什么**：

| | Evil Justice 100002721 (1,1) | 贵族机要 100004968 (0,2) |
| --- | --- | --- |
| base 图 | 100004325，**12 只敌人的战斗房**（玩家该待的地方） | 100015973，过场**之前**的宫殿庭院 |
| 层图 | 100004546，过场演出房 | 100015974 → 100016356，播完后的战场废墟 |
| 回头要恢复 | base（所以 flag 2 清序号正确） | 末张层图（要**保住**序号） |

attempt 1/3 的代码已被 attempt 2/3 取代（`LayerRevisitResume` 删除）。

## 修改（attempt 2/3）

同一格只改 layer flag：**flag 1 + mode 0**，服务端回访房间停在末张层图不再换 base。

- `internal/dungeon/scene_transition.go`：`LayerRevisitResume` 换成 `Session.OnFinishedLayer()` 谓词 —— `Definition.Odyssey` ∧ 当前格有层序列 ∧ 末张已访问 ∧ 当前房间图就是末张。判据全部来自源与 `Visited`（`latestLayer` 在无 resumed base 登记时本就把回访房间定为最后访问过的层图，所以服务端侧无需改动）。`finishedLayerLastMap`/`markResumedSceneBase` 保留。
- `cmd/wireprobe/dungeon_flow.go`：命中后 NOTI29 = `state.LayerChange=true` + `ReuseRoom` + 无怪物行，带 StartMap 原生默认 18 字节记录（flag 1 与 ExitLayer 一样要求记录存在）。
- 依据：原生 flag 1 进入/推进层图，mode 0 把同格层索引前进并**钳在最后一张**后用缓存层图（`0x1452B77EE..0x1452B788B`）；`0x145EA25F0` 按有效序号选中的就是层图那套渲染房间与实体管理器，远景 `[background animation]` 与近景道具同源，不再出现 attempt 前那种「层图道具 + base 描述符」拼接。mode 0 在 `0x1452B78F0` 跳过建图与 ON START MAP 演出，不重播剧情。
- 这条包形态不是新假设：本服已在跑 `r.LayerChange && 目标图==当前图` 的层图往返（实机安图恩讨伐战 100004950 的 164↔165 就是 flag 1 + mode 0 的 34 字节复用包）。Evil Justice attempt 1/3 用的也是 flag 1 + mode 0，当时房间正常渲染、卡住的是那一格**层图本身**的隐形墙与锁门（无敌露西尔 + `[do not pass]` 挡路物），与包形态无关；本格 100016356 是玩家点门走出来的，门可用。
- 未动：协议布局、codec、`SourceLayerResume`/`configs/dungeons.layer-revisits.json` 的配置准入、`layerSequenceAdvance` 的「序列走完一律前进、绝不回 base」规则、普通格沿用既有的 flag 0 + mode 0。

## 验证

Go 1.26：`go build ./...`、`go vet ./...`、全量 `go test ./...` 全绿。

回归 `cmd/wireprobe/noblesse_layer_revisit_test.go`，用 `dungeons.odyssey-scenes-release.json` 的 100004968 实机形态（base 100015973 + 层序列 [100015974, 100016356]，播完后前进到 (0,1)）：

- `TestLayeredCellWalkBackKeepsLayer`：回头那包要求 34 字节、首三字节 `{0,2,1}`（flag 1 + mode 0）、记录为原生默认；服务端房间仍停在末张层图且**未**登记 resumed base；再走开、走回，第二包仍是 flag 1 + mode 0。
- `TestUnfinishedLayerSequenceDoesNotExit`：末张层图未访问时绝不发 flag 1/flag 2 的复用包，也不把房间换成 base 或末张（保护 LAYER-SEQUENCE-EXIT 那条不重播剧情的约束）。

候选 `bin/wireprobe-pvf.exe` = SHA256 `2dd64fc0cb7dae2b8ce3723e4ef356cd86eba46b918c8d11a242199c12be9c65`（`-trimpath`，同名副本在 `server/work/dfo-lan/.tmp/layer-revisit-bg-20261005/wireprobe-attempt2-layer-flag.exe`）；attempt 1/3 原件 `db3b446143aef456759bbd1fe606fb28c042924c5733bc0de2705b4c65e46fc7` 存档为 `server/work/dfo-lan/.tmp/layer-revisit-bg-20261005/wireprobe-layer-revisit-bg.exe`；已确认基线 `8bcf1d9e861266175f8c218c59327a2590cb1637d08574470ccc662be53fc203` 备份在 `server/work/dfo-lan/.tmp/layer-revisit-bg-20261005/baseline-8bcf1d9e.exe`。

## 实机验收（attempt 2/3，已通过）

会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261005_173509_665764_next37/events.jsonl`（副本 100004968，用户操作 09:36:05..09:36:52 UTC）：

| 时刻 | 落点 | layer flag | 包形 |
| --- | --- | --- | --- |
| 09:36:05/06/07 | (0,2) | 0 / 1 / 1 | 入场建图 + 两张层图（100015974、100016356）真建图 |
| 09:36:08 | (0,1) | 0 | 前进战斗房 100016357（195 字节，7 怪） |
| **09:36:14** | **(0,2)** | **1** | **34 字节 flag 1 + mode 0 复用缓存层图（第 1 次回头）** |
| 09:36:16/27/29… | 普通格 | 0 | 34 字节 flag 0 + mode 0，与修前逐字节同构（不动） |
| **09:36:31**、**09:36:52** | **(0,2)** | **1** | 第 2、3 次回头同一形态 |

21 张图全部有 CMD37 装载确认 + NOTI30 装载完成（21/21），回头之后继续前进到 (4,1) 才 CMD42 退本；`gateway.err` 无运行期错误。用户结论「我实机是没有问题的」⇒ 恢复的是离开时那张层图、背景不再黑、不重播剧情、门仍可走出去。对照 attempt 1/3 会话 `…_20261005_172124_…`：同格那一包是 flag 2（`000202…`），即宫殿 base。

## 覆盖面：其它奥德赛副本

判据是**会话级、与副本无关**的（Odyssey ∧ 该格有层序列 ∧ 末张已访问 ∧ 当前图==末张），所以不是给 100004968 开的特例。按当前内层 PVF（`7ef2db59…`）用 `dfo-tool dungeonscenesaudit` 穷举 56 个奥德赛副本：

- **26 个副本挂着 `layered` 格**，其中多张序列（≥2，回头才可能撞上「已播完」）的是 100004983(2)、100004981(4)、100004980(3)、100004961(3)、100004968(2)、100004938(2)、100004937(4)，其余 19 个为单张序列。这些副本的 layered 格走回头路现在都发 flag 1 + mode 0。
- 26 个全部 `Odyssey=true`、`IndividualMapMovement=false`、`MoveMapEvenEnemy=false`。
- **非奥德赛不受影响**：通用复用分支只认 `Odyssey ∥ resumed base ∥ (individual map movement ∧ move map even enemy)`，普通副本回头是**真建图**（mode 1，包体带 map 行），远景与近景同出自那张图，拼不出这个黑背景；2022 主线 Evil Justice 100002721 那种「层图要回 base 战斗房」的格子走 `configs/dungeons.layer-revisits.json` 的 `SourceLayerResume`（flag 2），本次没有改动，仍是已确认基线。
- 日志侧事实：把 `runtime/` 全部会话按「该格先真建过 ≥2 张层图、后又收到复用包」筛一遍，**历史上只有 100004968 的 (0,2) 命中**（flag 0 ×9 次 → attempt 1 flag 2 ×1 → attempt 2 flag 1 ×3）。其它奥德赛副本的 layered 格还没被走回头路踩过，所以「它们也一起好了」是共用判据的推断，不是逐项实测。
- 仍未接的形态（无证据，不改）：回头请求带 `record[0]==1`（脚本传送 → `MoveScript`）或 `p10==1`（→ `MoveScene`）时不查该谓词。若在别的副本仍看到回头场景不对，先指出副本号，按那一局的报文确认它走的是哪条分支再处理。
