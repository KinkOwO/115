# CMD 27 古代卡片罐取证（2026-09-25）

## 已确认

- 玩家截图为 `Coral's Ancient Card Pot`；实机帧 `2026-09-25T09:32:41.5206017Z`：`type=1,id=27,checksum_ok=true,plain_hex=5b00000000000000,unimplemented_sample=true`。同一操作在 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260925_173530_041653_next37/events.jsonl` 再次出现（`09:36:38.982559Z`），随后没有 CMD 27 应答。
- 权威 IDB 的 opcode 表将 27 命名为 `ENUM_CMDPACKET_USE_LOTTERY_ITEM`。客户端 S2C 注册函数 `sub_1452A1F90 @ 0x1452A21EE` 将 CMD 27 绑定到 `sub_14529EDE0`。
- 多条客户端原生发送路径（如 `sub_14139BD00 @ 0x14139BD41..65`、`sub_14492F090 @ 0x14492F20D..232`、`sub_1466EDFD0 @ 0x1466EE818..83C`）按顺序写 `CMD 27`、`u16 槽位`、`u32 0`。实机明文的有效 6 字节为 `5b 00 00 00 00 00`，其后两字节为加密块填充；槽位为 91。发送端在 `sub_145AD55B0` 查找所选物品的槽位。
- 当前 PostgreSQL 角色 11（日志显示的角色）存档里，背包槽 91 的物品为模板 `7772`、数量 1。导出索引 `configs/items.index.json` 将其指向 `stackable/ect/uplegacy_card01.stk`，类型 `[upgradable legacy]`；`configs/booster-catalog.json` 没有模板 7772。
- IDB 的 `sub_14529EDE0`：成功标志非零时先用 `sub_146EA1920` 读 `u16` 消耗槽位，再用 `sub_146EA0BE0` 读 181 字节物品记录；当奖励模板满足 `sub_145A83FA0`（装备类型分支）时，再用 `sub_146EA0BA0` 读附加 `u32`。失败标志为零时依错误码走提示分支。181 字节物品记录与服务端 `protocol.CurrentItemRecordSize` 一致。
- 当前 `client/Script.pvf`（SHA-256 `5dd03873edf2c1df7aea16db5ad146a776fb8461a947be73042cabd932e66f0a`）与 `server/work/client-build/Script.required.pvf` 完全相同。解出当前内层后，其 `stackable/ect/uplegacy_card01.stk` 与原始内层 `server/work/client-build/Script.inner.pvf` 的同名条目逐字节相同（条目 SHA-256 `418da31021ef6ad9dffec621542adc963e87cb52eb33f9b9e049f18093aa1810`）；`list/stackable.lst` 也逐字节相同（SHA-256 `7d747f6158803518d71527d28a1ad00839cdf83f4498f3ad587c237f32b80daf`）。因此当前资源中模板 7772 的路径和脚本奖池已确认。

## 模板 7772 奖池

- 当前脚本 `[int data]` 有 627 个整数，按原顺序分为 **209 组三元组**；第一列的 209 个奖励模板 ID 全部存在于当前 `items.index.json`，且没有重复；第三列全部是 `1`。
- 第二列的原始数值分布为：`1 × 37`、`5 × 24`、`10 × 10`、`17 × 27`、`36 × 1`、`99 × 25`、`477 × 41`、`1730 × 44`，合计 `98,904`。参考服把第二列解释为权重、第三列解释为奖励数量，但这仅是解释线索；尚未以本版原生抽取路径或实机中奖结果验证，不把这些数值写成已确认概率。
- 按导出索引，209 个奖励都是 `stackable`：其中 `stackable/monstercard/` 路径 81 个、`stackable/professional/` 路径 80 个、`stackable/10089001/` 路径 48 个；类型标记为 `[material expert job]` 的 129 个、`[enchant waste]` 的 80 个。仅凭路径和类型标记，不推定实际展示名或奖励类别。完整原序清单见 [`lottery-item-7772-pool-20260925.csv`](lottery-item-7772-pool-20260925.csv)，字段 `raw_second_value` 和 `raw_third_value` 刻意保留原始语义。
- 清单已逐组三个整数与当前客户端 PVF 条目核对：`209/209` 行一致。

## 三元组字段求证（2026-09-25 续）

| 列 | 当前证据 | 结论边界 |
| --- | --- | --- |
| 第 1 列 | 客户端罐子预览路径 `sub_146930120 @ 0x1469301e4..0x1469301fc` 从物品定义 `+432` 的整数数组每 3 个取一次第 1 列，传给物品查找 `sub_14603E2A0`；奖池中 209 个正 ID 均能在物品索引中定位。 | **奖励物品模板 ID 已确认**；其他同类罐允许 `0` 作为金币奖励哨兵。 |
| 第 2 列 | 当前客户端 PVF 的同类型金币罐 `10001558`、`50006033` 和物品罐 `7486` 分别按三元组求和为 `100,000`；另一个同类型金币罐 `50020774` 为 `10,000`。旧服实现 `../usdof/server_proto/game/character/random_container_index.py` 与 `../ServerS4A21/Tool/PvfLib/Models/StackableItemFile.cs` 都将这一列作为抽取权重。 | **权重语义有强交叉证据**，但 115 当前服务端没有抽取实现，客户端预览也不按此列抽取；尚不能确认本罐用 `98,904`、固定 `100,000` 或其他总量归一化，不能给出确定中奖概率或“剩余 1,096”去向。 |
| 第 3 列 | 同类型金币罐的第 1 列为 `0`，第 3 列出现 `20,000..1,000,000` 金币数额；旧服已实机确认其用于金币发放。当前客户端预览路径 `sub_146930120` 将第 3 列传入展示物品构造器 `sub_14576E560 @ 0x14576E560`。本罐每项均为 `1`。 | **奖励数量／数额语义有强交叉证据**；当前 115 罐子的实际发奖数量仍需原生成功结果验证。 |

以上四个对照条目已从当前运行资源对应的内层 PVF 读取，并与原始内层 PVF 的同名条目逐字节比对一致。客户端 `sub_146930120` 只用于预览列表；其从整数数组每 3 个取第 1、3 列，不执行服务端开奖。因此截图中的滚动卡片不是中奖实证。未执行任何客户端操作或服务端试包。

## 参考端开奖路径对照

- `../usdof/server_proto/game/character/random_container_index.py` 的 `parse_lottery_container_text()` 在 `[upgradable legacy]` 分支以步长 3 读取 `item_id, weight, count`；`_select_weighted()` 先过滤权重非正的候选，以剩余权重总和调用 `randbelow(total_weight)`，逐项累计到命中。`../usdof/server_proto/opcodes/opcode001b-use_lottery_item.py` 的 CMD27 路径调用 `definition.roll()`，再把 `reward.count` 交给发奖和结果构造；`helper-random_container.py` 的 `build_random_reward_grants()` 把它写入 `InventoryGrantSpec.count`。该参考项目的金币罐 `10001558` 有用户实机确认记录 `docs/baseline_pvf_gold_lottery_container_20260805.md`，可支持第 3 列对金币的数额解释，但不是当前 115 客户端开奖向量。
- `../ServerS4A21/Tool/PvfLib/Models/StackableItemFile.cs` 的 `ParseUpgradableLegacyRewards()` 同样按 `ItemId, Weight, Count` 解析；`Server/DfoServer/Game/Lottery/LotteryItemOpenService.cs` 的 `RollRewards()` 对每组求 `totalWeight = Σ max(0, Weight)`，调用 `ServerRandom.Next(totalWeight)` 后逐项累计命中；发奖请求使用 `Count`。它不是固定以 `10,000` 或 `100,000` 为分母。
- 将上述**参考算法**套到本罐原始 209 行：全部权重为正，权重总和 `98,904`，所以每行的参考概率为 `raw_second_value / 98,904`，每次选中后发 `1` 个。第二列为 `1` 的单行参考概率约 `0.001011%`，为 `1730` 的单行约 `1.749171%`。这仅是参考端算法的条件计算，**不能直接宣称当前 115 原版服务端的中奖率**；也没有证据表明差额 `1,096` 形成空奖或隐藏奖。

## 未闭环

1. 三元组第二列的具体抽取算法、归一化分母，以及第三列在本罐实际发奖中的应用，尚未通过当前 115 原生成功结果证实；当前资源本身、奖励 ID 和同类条目字段用途已有交叉证据。
2. CMD 27 成功 ACK 的 181 字节获奖记录读长、公共结果头与堆叠卡片的无附加字段分支已由当前 IDB 和现有物品记录编码对应。用户已实机确认开罐成功且奖励进入背包；开罐页面进度条没有更新，获得物品弹窗显示异常，结果通知与 UI 消费路径仍未闭环。
3. 槽 91 的物品可能在后续玩家操作后变化；实现时必须按请求当时的存档校验，不能硬编码角色或槽位。

## 服务端实现与用户确认基线（attempt 1/3）

- 仅对模板 7772 开启 CMD27。`configs/lottery-item-7772.json` 记录由当前 PVF 校验过的 209 行；加载时复核模板、脚本 SHA-256、奖励索引、行数、唯一 ID 和权重总和 98,904。随机抽取范围就是有效权重之和，使用服务端安全随机源。
- `protocol.DecodeLotteryItemUse` 按当前客户端发送路径读 `u16` 槽位、`u32 0` 和全零加密填充；`LotteryItemSuccess` 按当前客户端 reader 发送公共 `u8=1,u16=0`、`u16` 消耗槽位与 181 字节卡片物品记录。卡片为堆叠物品，不走装备附加 `u32`。
- `worldSession.openLotteryItem` 在 `CommitCharacterEvent` 中复核玩家当前槽位仍是 7772、扣除 1 个罐子、向可用物品槽发放 1 张卡片；满包或存档校验失败时事务整体回滚。成功后先发 CMD27 结果，再发 NOTI14 背包变更；失败发当前 reader 支持的错误码 4。没有数据库结构变更。
- 参考端 `../usdof` 的旧构建成功结果只有 46 字节，与本版 181 字节物品记录不同，所以未直接移植参考服的结果包。用户在候选版手动开罐并确认成功，奖励物品已进入背包。用户同时观察到开罐页面进度条未更新、获得物品弹窗显示异常；因此只确认开罐交易与入包，不确认 UI 结果显示正确。后续应结合 `lottery_item_open`、`lottery_item_inventory` 运行日志及客户端 reader/通知消费路径继续取证，不能用这次入包结果推定 UI 包字段已正确。
- 验证：定向 CMD27 协议与存档测试通过；`go test ./...` 和 `go vet ./...` 通过；已编译独立候选 `bin/wireprobe-handoff-source.exe`，未覆盖归档基线。

此前的静态取证阶段 C2S 尝试次数为 0/3；本次按已确认的当前客户端 reader 接入候选运行路径，计为 **attempt 1/3**。用户实机确认开罐及背包发奖成功，此行为作为当前确认基线；页面进度条和奖励弹窗仍是明确未解决项。
