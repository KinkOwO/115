# 图像通讯器 CMD467 取证缺口（2026-09-26）

## 用户实机现象

- 任务书显示 75 级任务 **The Princess's Contact**，目标文字要求通过 Ria's Video Communicator 与 Imperial Princess Erje 通话。
- 通讯器 UI 正常打开，选项为 Imperial Princess Erje，费用 10000 gold；点击 Communicate 后窗口关闭，任务未完成。
- 同次服务端日志：`{"bytes":13,"checksum_ok":true,"hex":"01d3010d00000082ed3908ed02","id":467,"kind":"client_frame","plain_hex":"","type":1,"unimplemented_sample":true}`。13 字节为完整空参数请求，不是缺包或解密失败。

## 已确认的客户端证据

- 权威 IDB 的 CMD 表将 467 (`0x01D3`) 命名为 `ENUM_CMDPACKET_IMAGE_COMMUNICATION_EQUIPMENT_USE`；注册表字符串 VA `0x14B048280`，槽位 `0x14EF39DF8`。
- 客户端发送入口 `sub_145949640` / `sub_145957990` 在条件通过后发送 CMD467；发送路径调用 `sub_14667BB90(..., 467, 0)` 与 `sub_14668C520(..., 467, ..., 1)`。本次实机请求无 body。
- **修正前次误判：**`sub_1452F9420` 注册的是同编号 `ENUM_NOTIPACKET_GROWTH_ITEM_CHANGE_DATA` 的 NOTI467 reader `sub_1452D34F0`，并非通讯器的 CMD467 确认包。
- CMD 确认注册链 `sub_1452A1F90` 将 CMD467 绑定到 `sub_1452812C0`。通用确认分发层先把首字节成功标志作为 `a2` 传入；该函数仅在 `a2 != 0` 时连续读取两个小端 u32，第一个在该函数内丢弃，第二个交给 `sub_14570DD60` 召出 NPC。读取函数 `sub_146EA0BA0` 确认为 4 字节小端读取。对照同表 CMD31 的已验证成功响应：`QuestAccepted` 先写首字节 `1`，再写 quest ID 和字段；对应 handler `sub_145261B40(a1, char a2, short a3)` 也先检查 `a2`。
- 当前 Go 服务端 `cmd/wireprobe/main.go` 对 467 只有通用未实现采样，没有业务处理与回复。

## 任务配置线索，尚非任务 ID 定论

- 当前 `configs/quests.generated.json` 的 PowerStation 剧情链 3741 为 75 级 `[meet npc]`，目标 NPC=2001；路径为 `contents/2022/new_scenario_renewal/season_5/powerstation/quest/powerstation_07.qst`。截图文字与该虚拟 NPC 目标可能相符，但任务名 `<11::name_PowerStation_07>` 尚未与截图标题做原生资源映射，因此不能按 3741 硬编码完成。
- 从与 `quests.generated.json` 同源的 `server/work/client-build/Script.inner.pvf` 读取 `etc/imagecommunication.etc`：`[npc index] 2000` 需要任务 3734，`[npc index] 2001` 需要任务 3741；`[summon time]` 为 20000 毫秒。任务 3741 的 `[meet npc]` 目标确为 2001。任务 3734 不在当前导出任务目录中。资源的 `[charge] 10000` 与 UI 的 10000 gold 一致；参考服将其命名为毫秒，但本候选版不依此推断计时或扣款协议。
- 当前 `client/Script.pvf` 与 `server/work/client-build/Script.required.pvf` 的 SHA-256 均为 `5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`，确认运行外层资源与本次使用的 client-build 配套资源一致。
- `analysis/dumps/dstr_id_to_text.json` 的 30078 映射为 `Ria's Video Communicator`。

## 暂停实现的原因与下一步

- 前次尚未区分 CMD 与 NOTI 两套同编号注册表；上面的修正使 CMD467 确认包的实际 reader 与 PVF 任务映射可追踪。仍缺原生成功回包向量和费用扣款分支的实机验证。
- 用户在 2026-09-26 明确要求按参考服的业务思路实现。首次 Go 候选实现使用空参数 CMD467、进行中任务筛选、8 字节 CMD 确认（第一个 reader 忽略的 u32 为 0，第二个为源 PVF NPC index），随后允许 20 秒内同区域召出 NPC 的 CMD33 对话推进。
- **attempt 1/3 实机失败：**`2026-09-25T20:36:18.837Z` 和 `20:36:57.293Z`，候选版均收到校验通过的 CMD467，并记录 `image_communication_ack`：角色 11、任务 3741、NPC 2001、body `00000000d1070000`。其后没有 CMD33 或任务触发；用户反馈界面仍直接关闭。该证据排除了处理器未命中和任务筛选失败。
- **attempt 2/3 修正：**补齐通用 CMD 确认首字节成功标志，body 调整为 `0100000000d1070000`（首字节 1、未使用 u32 0、NPC 2001）。前次遗漏首字节使 handler 的 `a2` 为 0，跳过了两个 u32 的读取及召 NPC 调用；修正后经用户实机确认通过。

## 候选版检查

- `go vet ./...` 通过；通讯器、协议及现有 NPC 对话相关定向测试通过；`go build -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe` 通过。
- `go test ./...` 仅 `cmd/wireprobe/TestAwakeningTownUsesSourceQuestGate` 失败：未跟踪的觉醒城镇测试读取现有 `configs/world.generated.json`，期望镇 194/195 共 138 个区域，实际为 0；该测试和配置不是本次修改范围。其余包通过。
- attempt 2/3 重新执行 `go test ./...` 与 `go vet ./...` 均通过；先前觉醒城镇测试在此轮已通过（其他工作区文件在两次检查之间更新，本任务没有编辑这些文件）。更新后的 `bin/wireprobe-handoff-source.exe` 构建于 2026-09-26 04:41。
- 用户实机确认成功：点击 Communicate 后 NPC 出现，与 NPC 对话后任务完成。

## Confirmed baseline（2026-09-26）

- 手动实机事件位于 `server/work/dfo-lan/runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_044404_928709_next37/events.jsonl`：20:44:33Z 收到角色 11、任务 3741、NPC 2001 的 `image_communication_ack`；20:44:36Z 记录 `quest_npc_objective`；20:44:42Z 记录 `quest_finished`，quest=3741。随后任务 3742 也通过普通 NPC 对话完成，日志在 20:45:07Z、20:45:09Z 分别记录目标交互和完成。
- 用户确认 NPC 出现、能够对话并完成任务。成功 ACK 使用客户端 reader 所需的 9 字节 body `0100000000d1070000`；NPC 会面推进仍走 CMD33 与已有任务状态流。
- `go test ./...` 与 `go vet ./...` 通过；候选版 SHA-256 为 `9B1E4EB235B893F1EFD2C34674D6529ED9D30B68C085892613D8A0BB57C2777C`。无客户端补丁或数据库 schema 变更。

## 参考服 ServerS4A21 对照（2026-09-26）

- `Server/DfoServer/Network/Core/PacketTypes.cs` 的同名命令是 `0x01DC` (476)，与当前 115 客户端的 `0x01D3` (467) 不同。
- `Network/Parsers/Quest/QuestCommandParser.cs` 只接受空 body；这一点与本次 115 实机 13 字节空参数请求相符。
- `GameWorld/ImageCommunicationDefinitionCatalog.cs` 从 PVF `etc/imagecommunication.etc` 读取 NPC 编号、坐标、`[require quest]`、充能和召唤时间。`Game/Quests/ImageCommunicationApplicationService.cs` 遍历配置项，要求角色有相应进行中任务且 `TriggerValue != 0`，任务类型为 `[meet npc]`，目标 NPC 与配置一致。
- `Game/Quests/QuestManager.cs` 对成功或拒绝都发送该命令的 ACK；`Network/Builders/Quest/ImageCommunicationAckBuilder.cs` 的 body 只有一个小端 `int32 npcIndex`，拒绝时为 0。该路径没有写任务完成状态，也没有扣除金币，因而不能据此断言使用通讯器即直接完成任务。
- 当前 115 客户端的 CMD467 确认 reader `sub_1452812C0` 读取两个 u32；参考服的 4 字节 ACK 会把 NPC index 放到第一个、被忽略的位置，因此不能直接移植其回包。较复杂的 `sub_1452D34F0` 属于 NOTI467，已与 CMD 路径区分。
