# 契约 NOTI66 闪退兼容开关

下游报告部分客户端在商城购买契约后，收到购买路径的即时 NOTI66 时闪退；2026-09-27 在开箱路径本地复现：radiant box x10 抽中契约奖后追加 4× 即时 NOTI66，约 1.1 秒后客户端 `0xC0000005`（violationAddr 0x0）崩溃退出，同会话中无 NOTI66 的 x10 开箱不崩（`roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260927_121732_450806_next37/events.jsonl` + `client-direct.out`）。

## 开关行为

- 设置 `DFO_CONTRACT_PURCHASE_CRASH_FIX=0` 时：保留原行为，即所有激活路径在回执之后继续追加即时 NOTI66 契约通知。
- **不设置该变量或设为其他值（默认）时：修复开启**，即商城购买、radiant 开箱、booster 开箱、契约道具直接使用四条激活路径都不再追加即时 NOTI66。

修复开启后：

- 契约激活仍按原事务扣款/消耗道具，并写入 `account_premiums`（选角时经 `premiums_restored` 恢复）。NOTI14、NOTI53、NOTI2551、逐条 ACK64/ACK160 等其它包全部保留。
- 选角时从账号存储读取有效契约，不再依赖 `town-entry-probe`。选角回包仍发送客户端读取链要求的**剩余秒数**；数据库继续保存绝对 Unix 到期时间。
- 开启期间，契约图标和剩余时间需要重新选角才刷新。

## 覆盖路径（2026-09-27 更新）

此前该开关只覆盖商城购买路径；2026-09-27 实机确认开箱路径同样触发同类闪退后，将同一门控扩展到全部即时 NOTI66 发送点：

| 路径 | 发送点 | 事件名 | 已纳入开关 |
| --- | --- | --- | --- |
| 商城购买契约 | `cmd/wireprobe/shop_pilot.go` | `cera_purchase_premium_activated` | 2026-09-25 |
| radiant 开箱（x1/x10） | `cmd/wireprobe/radiant_box_flow.go` | `radiant_box_contract_noti` | 2026-09-27（本次实机崩溃来源） |
| booster 开箱 | `cmd/wireprobe/booster_flow.go` | `booster_special_item_noti` | 2026-09-27 |
| 契约道具直接使用 | `cmd/wireprobe/booster_flow.go` | `contract_special_item_noti` | 2026-09-27 |

开关在服务端启动环境中设置，例如 PowerShell 使用 `$env:DFO_CONTRACT_PURCHASE_CRASH_FIX='1'`，随后从同一环境启动服务端。

下游需手动验证：开箱抽中契约奖不闪退、扣款/消耗与契约存档正确、重新选角后契约仍有效。本改动不修改数据库结构或玩家存档。
