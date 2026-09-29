# Evil Justice（任务 12893）：第二房间过场回退

状态：C2S **attempt 3/3 已由用户实机确认战斗房恢复，纳入已确认基线**。attempt 1/3 不再回退但锁门与隐形墙，attempt 2/3 仍显示空的错误房间；attempt 3/3 采用只读实机缓存和权威IDB证据修复，用户确认过场后进入正确战斗房。后续清场与进入第三房间未单独记录实机验收。未改客户端、DLL、数据库或玩家存档。回滚只需恢复旧候选程序，本次没有数据迁移。

## 现场与原因

用户截图任务 Evil Justice，目标 Follow Lemidia Capella in the White Land。源任务 12893（`sanctusbellum_act_18.qst`）要求 `[clear map] 100004552`，副本 100002721，迷宫 0。

用户操作会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_003719_477815_next37/events.jsonl`：

- 17:22:24 UTC 从 (0,1) / 100004322 入场；17:22:27 进入 (1,1) / 100004325，再进入同格剧情层图 100004546。
- 17:22:29 原生 CMD45 在 (1,1) 上报 layer change，记录 `000000000405a50021010000000000000000`，落点 (165,289)。服务端的通用末层出口按房间枚举顺序选首个相邻非层图格，错误返回 (0,1) / 100004322。
- 17:22:33 再进 (1,1) 时 `latestLayer` 载入已经访问的 100004546，但下发初始化模式，失去原生缓存演出状态。第二次进图在 17:23:16、17:24:33 再现同样回程。

此处源地图与导出配置没有缺图。当前 `client/Script.pvf` 的地图和副本原始脚本 SHA-256 与配置逐项一致；地图索引的 bridgesoftheatonement 路径是源中的同名共享资源，不凭目录名推断错误。

## 源资源与客户端消费证据

- 副本脚本 SHA-256：`a5491c6f945c13383b1393ab85ba3b1b377bccf8335a20176de92215bf35f5f4`；(1,1) 的层序列只有 `[100004546]`。
- 地图 100004546 SHA-256：`865a33f304256c08c4d7d468f6c47b37d1fb3095d9f921712d26bd15eeca0f88`。该图 `[basic action] Action/100004546.act` 的 ON START MAP 播放 cinematic 13042；action SHA-256 `bc2dad3b47b73e67d45659998445801ba976daabaa3d74a5165bfed1bbb984c8`。
- `contents/2022/110levelscenario/sanctusbellum/cinematic/q12893_13042.cmt`，SHA-256 `1cf7200d226b99dea7425ea76cf59bf47a259f25b3c18efac27585e61cfbb977`。源 `[MAP] 100004546`；结尾 `[CHANGE MAP]` X MIN/MAX=165、Y MIN/MAX=289、Z=0、CHARACTER VIEW=1，与两轮原生请求一致。源并未指定另一个迷宫格，也没有 DESTROY 战斗敌人的行为。
- 权威 IDB 已有 NOTI29 handler `0x1452B7100` 的末层缓存消费证据，见 `analysis/tasks/lotus-terminal-layer-20260923.md`。本轮重新从当前 DFO.exe 对照 `0x1452B7793..0x1452B788B`：模式 0 将同格层索引前进并钳在最后一层，使用缓存层图；`0x1452B78F0` 跳过地图和怪物初始化。既有 `protocol.StartMap` 的 ReuseRoom 正是该路径，未修改协议布局或 codec。

临时导出及只读校验保留在 `runtime/evil-justice-source-20260929/`，不入 Git；源地图片段固化在 `cmd/wireprobe/testdata/evil_justice_layer.json`。

## attempt 1/3 历史修复边界

新增配置 `configs/dungeons.layer-revisits.json` 记录已闭环的同格末层回程；本次仅收录此过场。启动校验源 checksum、副本/地图 SHA、任务迷宫和末层位置。运行时仅当当前房间已访问、处于最后一层，且任务、位置、源版本与完整 18 字节记录全部匹配时复用原房间。该匹配先于现有相邻格兜底处理。

ACK45 + NOTI29 保留 (1,1)、layer flag 1、mode 0 和原始落点记录；不重建怪物或 ON START MAP 演出，不标记通关，不绕过剩余敌人清场。其它现有副本分支保持原样。

## attempt 2/3 历史验收步骤（已失败）

关闭旧游戏环境后，通过根目录 `启动游戏.cmd` 启动新候选。重新进入 Evil Justice 的 White Land，走到第二房间并完整播放过场；确认仍在第二格、恢复原战斗房，人物可操作、贴图和敌人正常。打败该房敌人后进入第三房间，并继续完成任务。新日志应出现 `source layer resume`，from=100004546、to=100004325；对应 NOTI29 前三字节 `010100`、mode=0、长度34，随后收到 CMD37。实机结论未获得前不升级 confirmed baseline。

## attempt 1/3 构建与验证

Go 1.26.0：针对性 TestEvilJustice 回归、全量 `go test ./...`、`go vet ./...`、候选构建全部通过。回归覆盖同格缓存包与落点、敌人清场守卫、第三房间进入、返回第二房间后的缓存，以及源/任务/记录/地图 hash 不匹配时不走新回程。

候选 `bin/wireprobe-eviljustice-attempt1-20260929.exe`，已同步至日常启动的 `bin/wireprobe-handoff-source.exe`；SHA-256 `7C421AC20923AD4A0BC7867F3ECE145A5A7C222081645420A205FE74DE5E07A4`。旧候选备份 `runtime/evil-justice-source-20260929/wireprobe-before-attempt1.exe`，未覆盖归档39。

## attempt 1/3 实机结果与新取证

用户确认没有回退首房，但门锁定且隐形墙阻挡。用户操作会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_013715_842941_next37/events.jsonl`：17:38:08 UTC 回程 from/to 都是100004546，随后 CMD37 正常完成加载。到17:40:41没有下一次 CMD45、CMD38 或 CMD39。只确认回退消失，不确认房间流程恢复。

源地图100004546的 passive object index3/4 都是109062708，位于(1140,255)、(-246,239)。该模板 `passiveobject/kcontents2/siroco_raid_hard/map/dummy/dummy.obj` 明确 width=500/500、pass type=`[do not pass]`；其 basic action Dummy.act只有动画，没有销毁触发。该图仅有入口 CMT13042，没有第二段地图过场。

地图 monster index0 的模板109013430是露西尔 `contents/2022/110levelscenario/sanctusbellum/monster/a_npc/lucille/lucille.mob`，声明 category=`[non outline invincible]`。她的 event.ai为空，相关动作没有后续 CINEMATIC 引用。源 map将其 team设为100，但这不是可供玩家击杀的战斗敌人；第一轮按剩余敌人留在末层的判断错误，代码中的模拟击杀测试也没有验证客户端实际可击杀性。

原战斗房100004325有12只实际敌人（109013405、109014292），没有该演出房的阻挡物。其入口动作100004324.act播CMT13014进入当前演出层，所以需要保留该房的入场缓存，不能用初始化包再播入口剧情。

本轮重新对照当前客户端 NOTI29 原生 `0x1452B778C..0x1452B78A5`：layer flag=1选层缓存；flag=0经 `0x1452B787D..0x1452B78A5` 关闭层状态并调用 `0x145B2F650` 选择同格 base缓存；mode=0经 `0x1452B78F0..0x1452B78F9` 跳过地图/怪物初始化。以上是已确认消费分支；将此任务结尾接到base缓存的行为仍属第二轮候选，待实机确认，不宣称已通关。

## attempt 2/3 候选

同一份源匹配记录增加resume_map=100004325及其SHA，启动校验必须是迷宫同格base地图。运行时要求base已实际访问，过场回程恢复该格原战斗怪物、死亡与实体状态；NOTI29仍为mode0、保留原18字节落点，但layer flag改为0，选择base缓存。记录该格演出已结束，后续进门回访该格沿同一个base缓存，防止 `latestLayer` 把房间换回剧情层或重播CMT13014。没有开门补包、删除挡路物、伪造NPC死亡、提前通关或修改客户端资源。

旧attempt1候选仍保留为 `bin/wireprobe-eviljustice-attempt1-20260929.exe`；恢复该程序可回滚本轮，不影响存档。实机若仍卡住，剩余attempt 3/3必须针对新的动态/源证据，不能继续无依据试包。

### attempt 2/3 构建与验收边界

Go 1.26.0 全量 `go test ./...` 与 `go vet ./...` 通过；候选 `bin/wireprobe-eviljustice-attempt2-20260929.exe` 已同步至日常 `bin/wireprobe-handoff-source.exe`，SHA-256 `93A3E2B5CBF09C5B1AF6B2981F1EC352DCDA5FBDF4185CB614120A69F1DF214E`。回滚副本 `runtime/evil-justice-source-20260929/wireprobe-before-attempt2.exe` 保留attempt1，未停止或启动玩家客户端。

回归检验恢复base的12只原敌人、模式0/layer flag0/同格/原落点，base未访问、hash错误、目标不在同格、任务/源记录不符不走新分支；清场前拒绝进入第三房，清场后可进入，回访base仍使用缓存包且不推进剧情层。实机待用户验证第二房过场后墙与锁门状态、战斗和后续任务，未标记全部修好。

## attempt 2/3 实机失败与判断纠正

会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_014921_966801_next37/events.jsonl` 在17:50:09.797 UTC下发同格flag0/mode0缓存包，服务端记为base100004325/12敌人；用户截图仍是错误空房，无法前进。该轮未通过，以上“flag0恢复base”的判断已被本次动态采样否定。

用户随后手动复现并保持游戏。会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260929_020934_511163_next37/`，owned client PID13456；只读脚本 `analysis/evil_justice_room_snapshot.py` 在18:14:08..09 UTC采到三组稳定结果，保留于ignored runtime的 `client-room-cache.jsonl`。脚本仅OpenProcess QUERY_INFORMATION|VM_READ、ReadProcessMemory；没有调试器、断点、注入、写内存或代操作。

| 现场数据 | 普通战斗房缓存 | 剧情层缓存 / 当前实际房间 |
| --- | --- | --- |
| descriptor map | 100004325 | 100004546 |
| rendered room | 0x11273a200 | 0x18c33a180（等于scene.active_room） |
| entity manager | 0x1883b2b00 | 0x1883b0800（等于scene.active_entities） |
| packet spawn rows | 12 | 5 |
| 有效team100角色 | 12个战斗敌人 | 1个无敌露西尔 |

所在格index6的layer ordinal仍为0，base descriptor mode已经是0，scene.layer_flag已经是0。这直接证明：普通房间与敌人缓存存在，但flag0没有使客户端离开剧情层；房间不是因敌人缓存丢失而空。此前服务端状态测试只能证明服务端的12个实体保留，不能证明客户端选择了正确缓存。

重新完整读取权威IDB `0x1452B7100`：flag1进入/前进层图；flag0在 `0x1452B787F..7886` 调用layer标志setter后直接跳LABEL93，**绕过**层索引清零。flag2才走 `0x1452B7876` 调用 `0x145B45C50(...,-1)`；该setter在 `0x145B45CAB` 写dungeon+544的ordinal数组。随后 `0x145EA25F0` 在 `0x145EA2913..29AC` 根据ordinal的有效性选择层或base的rendered room和entity manager。mode0/既有room在 `0x145B235D3` 跳过重新构建，保留缓存ACT与敌人。完整伪代码保存为ignored runtime的 `ida-start-map.txt`、`ida-room-construction.txt`；关键分支已追加IDB注释并记录到analysis/dumps索引。

## 运行资源边界

本次确认启动器 `server/launcher.local.json` 实际client_dir是 `F:/wip/dof/115US`。该目录与仓库client的Script.pvf SHA均为 `5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`，sk.dat SHA均为 `59d78371335d22dae0eaeca2a827939576e85a5f5ede4ba286f0438d2fbc6eaf`。两份DFO.exe等长，.text只差4个字节（VA147220F48、49、4A、4D），上述房间加载/缓存分支字节一致；本次没有修改两份exe或任何客户端补丁。

## attempt 3/3 候选与验收

唯一运行路径变化：精确匹配CMT13042结尾时，NOTI29改为 **flag2 + mode0**，前3字节 `010102`，总长度34，原18字节落点不变。服务端仍恢复同格base100004325及12个原实体；后续普通门回访沿base缓存。新增ExitLayer字段只编码已证实的原生flag2，拒绝与LayerChange同时置位或缺失原落点；没有增加其它包、删挡路物、伪造NPC死亡或提前通关。

Go1.26.0全量 `go test ./...`、`go vet ./...` 和候选构建通过。回归现在明确要求flag2，覆盖原生18字节结尾记录、缓存包全字节、敌人清场门禁、第三房间进入和回访缓存。用户明确退出环境后才替换日常候选；回滚为 `runtime/evil-justice-source-20260929/wireprobe-before-attempt3.exe`（attempt2），同时保留前两轮独立候选，未覆盖归档39。

`bin/wireprobe-eviljustice-attempt3-20260929.exe` 与日常 `bin/wireprobe-handoff-source.exe` 的SHA-256为 `56066436CECAA0A69913674C5301405DEA1433362A6CD7881C7458AEE13BEA60`；回滚SHA为 `93A3E2B5CBF09C5B1AF6B2981F1EC352DCDA5FBDF4185CB614120A69F1DF214E`。

用户实机确认：第三轮候选播放过场后进入正确的战斗房间。该结果确认原生flag2退出剧情层并恢复base房间的修复有效，作为本功能已确认基线。房间清场后的下一房推进未单独确认；如后续观察到推进问题，应另行记录现场证据再处理，不据此扩大本次验收范围。
