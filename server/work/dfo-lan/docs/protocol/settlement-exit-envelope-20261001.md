# CMD72 结算出口公共成功字节修正（2026-10-01）

状态：用户手动实机确认。attempt 1/3；本轮只修正已有 ACK72 的公共包络，没有试探新字段或放宽状态门禁。

## 用户范围与日志

用户反馈部分副本通关后“返回城镇”和“开始下个任务”均遇到服务端拒绝，legacy 可用，指定任务 3189 关联副本。

- `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_230503_953919_next37/events.jsonl`：角色10在 15:09:06.7795532Z 发 CMD16 `110000000000000000ffff0000000000750c0000000000000000000000000000`，副本17、任务3189；15:09:55 通关，15:10:00 任务完成。该次通关后先有 CMD71 `card choice before reveal`，随后 CMD69/70；该日志未捕获通关后的 CMD72，因此不能将这个早期翻牌拒绝认定为 CMD72 修正已覆盖的问题。
- 同一会话前一次出口 15:09:06.7335007Z ACK72 为 `0101`；之后又收到 CMD72 `010301` 并拒绝 `cards before owned settlement`，但随后的 CMD16 已进入任务3189。不能把这次拒绝直接等同于下一副本进不去。
- `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_231848_739362_next37/events.jsonl`：15:25:35.9220894Z 角色1返回城镇 ACK72 `0102`，清理会话后立刻收到 CMD72 `010301`，15:25:35.9252991Z 拒绝 `cards before owned settlement`。15:31:42 再出现相同顺序。
- 修复后的 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_234806_594272_next37/events.jsonl`：15:50:57.2089391Z 任务3189完成；15:50:59.1328248Z CMD72选择副本 ACK `010101` 并返回 CMD15 gate ACK；15:50:59.2321517Z 客户端发起任务3190的 CMD16，服务端于15:50:59.2637206Z回 `dungeon_select_ack` 成功。用户确认能离开副本并进入下个副本。

legacy 分支源码也含 2026-09-24 的 ACK 收窄，不能仅凭分支差异断言用户正常 legacy 程序的行为。以下结论直接来自当前客户端。

## 当前 115 客户端接收闭环

采用已打开的权威 `client/DFO.exe.i64`，不重新打开数据库。关键三个区域 `0x1459A1C90`/40字节、`0x145244590`/50字节、`0x14524C650`/28字节与当前 `client/DFO.exe` 的 PE 映射逐字节相同。

1. 接收分发 `sub_1459A1BB0` 在 `0x1459A1C06` 初始化包 reader；kind=1 时在 **0x1459A1CA2** 读公共成功 `u8`，成功为非零，失败则在 `0x1459A1D1B` 另读错误码 `u16`。
2. `0x1459A1F06` 调用 `sub_14599D200`，传入该成功值和错误码；`0x14599D357` 进入注册 handler 执行器 `sub_1459A2D70`。
3. 注册函数 `sub_14524C580` 的 `0x14524C650..65E` 将 opcode72 注册为 `sub_145244570`。
4. CMD72 handler 判断传入的成功值后，在 **0x1452445AD** 读 state、**0x1452445B9** 读 option；普通结算在 `0x145244973` 调用 `sub_146ABAF50(window, option, state)`。
5. UI setter 存 option/state；`0x146ABB201` 对 state!=1 提前返回。普通返回城镇依赖 state=1、option=2。

因此完整 ACK72 plaintext 必须为 **`01 state option`**。现有 `native_card_exit_*.json` 的两个字节是公共分发之后的 handler body，scope 明确注明 UI/game stub，不是完整线上 CMD 包。`wire.ServerFrame`、`wire.EncryptPayload` 与 `preparePackets` 没有自动补公共成功字节。

旧回包 `0102` 被依次消费为 success=1、state=2、option=0（加密补零），不能触发普通返回城镇 UI 分支。旧 `0101` 被读为 success=1、state=1、option=0，变成重开选项。此为确定的包络缺陷；与任务3189完整症状的对应仍待手动回归。

## 修改与回退

- `SettlementExitSuccess` 恢复 `01 state option`，拒绝包、C2S解码、入口/出口状态门禁、奖励事务保持。
- 原生向量比较改为跳过公共成功字节，和 CMD70/71 相同；增加加密/封帧/解密后的公共分发与 CMD72 cursor 回归，覆盖重开、选择副本、回城、下个任务、无缝及 focus。
- selectingDungeon 继续从解码请求推导，不恢复依赖 ACK 字节索引的旧网关写法。
- 不涉及 schema、角色存档、数据库、客户端资源或 DLL。
- 回退本轮仅撤回上述 ACK 包络及对应回归/注释；构建前源码程序另作本地备份。不要回滚用户启动脚本或其它工作区修改。

## 验证与实机验收

- 专项 `TestSettlementExit*`、`TestCurrentNativeCardPackets`、`TestCardTransportPreflight` 通过。
- 用 overlay 仅恢复修正前 `cards.go`，新增传输 cursor 回归六个输入全部失败（回城被读为 `01 02 00`、下个任务 `01 03 00`）；恢复候选实现全部通过。该负向检查证明测试能捕获原缺陷，不只检查自回环相等。
- `go vet ./...` 通过。`go test ./...` 有5项既有失败：`TestAdventureAuditProvenanceAllowanceIsNarrow`、`TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback`、`TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader`、`TestOdysseyChapterFinalLordDrop`、`TestOdysseyCurrencySceneRetryAndPoolIsolation`。overlay 恢复本轮全部5个源码/测试文件至 HEAD，独立复核这5项同样失败且原因相同；本轮未新增失败。完整日志本地 `.tmp/settlement-exit-test.log`，基线复核 `.tmp/settlement-exit/baseline-test.log`，不入库。
- Go1.26 `go build -trimpath` 源码候选 SHA256 `9594b7440046e106337bc5de66277d5931bf3a08202f40bb9d3b6671148cb75b`。构建前源码与默认程序均为 `6b15b723263f28ee50137046529b50d5605f38cdd9797533be88f7b4895ed8ab`；源码备份 `.tmp/settlement-exit/wireprobe-handoff-source.before.exe`，默认程序保留。
- 已在当前 IDB 命名接收/公共 CMD 分发、handler 执行器、注册函数、CMD72 handler 和 UI setter，并写入 `analysis/dumps/idb_funcs.tsv`。
- 实机确认范围为用户反馈能返回城镇及进入下一个副本，并有任务3189通关、CMD72副本选择和任务3190 CMD16准入成功日志佐证。不扩展为所有异常副本逐一回归。
- 用户已确认，源码和收口时默认程序均为候选SHA256 `9594b7440046e106337bc5de66277d5931bf3a08202f40bb9d3b6671148cb75b`；纳入此项 confirmed baseline。
