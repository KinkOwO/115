# 商城发货装备分类修复 —— 称号进错消耗品栏（2026-09-26）

## 症状（实机）

商城购买「Adventurer's Will Title Box」（模板 590713836，330 Cera，`[usable cera package]`），
购买时就地展开开出称号装备「Adventurer's Will」（500330719，`equipment/character/common/title/500330719.equ`），
落进了背包 **Use（消耗品）页**；客户端拖回 Equip 页被服务端拒绝
（`equipment_move_refused: slot outside equipment bag` / `invalid stack destination`）。

## 取证链

1. 存档验证：`characters.state->'inventory'->'items'` slot 66 = 500330719（Use 区），
   `equipment` 无该模板 —— 服务端确实把它当堆叠物入库。
2. `configs/items.index.json` 分类正确：`500330719 → kind=equipment, path=equipment/.../title/*.equ`。
3. 根因：`internal/cashshop/pilot.go` `resolveDeliveryType` 的兜底分支——
   `LootCatalog` 只投影 stackable（`internal/catalog/loot.go` 对 equipment 刻意
   "refuse equipment projection"，见 Pending 备注），因此 ItemCatalog 里没有任何
   equipment 条目；装备模板落到最终兜底 `deliveryType{Kind:"[etc]", Slots:{65,120}}`
   （65-120 = Use 区），再经 `Bag.Add` 入库。

## 修复（服务端优先，与开盒/任务/掉落发放同规则）

- `resolveDeliveryType`：`info.Kind == "equipment"` → `[equipment]`（槽位 [9,64]，即
  `inventory.current37.json` 的 `equipment_slots`，与开盒 Destination 3 一致）；
  源路径含 `creature/` 的 .equ（宠物蛋）仍走 `[creature]` → Special[7]。
- `deliverAmount`：新增 `[equipment]` 分支，写 `b.Equipment`；耐久由宿主注入的
  `EquipmentCatalog.Reward`（同 `boosterEquipmentDurability`：读源 .equ 的
  `[durability]`，durabilityOptional 部位本就是 0，解析失败记日志按 0 发放）。
- `cmd/wireprobe/main.go`：用 `-item-index`（599,771 条，equipment 102,998 /
  avatar 321,218）补全 Pilot 分类目录（`SupplementItemKinds`，已有条目不覆盖，
  堆叠物仍以 LootCatalog 投影为准），并注入耐久规则。
- 回归测试：`internal/cashshop/equipment_delivery_test.go`（装备进 Equip 页 /
  creature 按路径改投宠物栏 / 未补全模板保持 `[etc]` 兜底不漂移）。

## 边界

- 商城直售的装备商品（非礼包展开）仍走 `classify → ordinaryHandler`，其中只有
  `[creature]` 有特判；若将来直售称号/装备需要同等路由，应在 `ordinaryHandler`
  层处理，本次未动（当前商城无此类在售品）。
- avatar 分支不受影响；本修复同时让"礼包展开出 avatar"也能正确改投装扮栏
  （ItemCatalog 补全后 `resolveDeliveryType` 的 avatar 分支首次可达）。

## 存档修复

误发的称号已由一次性 SQL 从 items(slot 66) 迁至 equipment(slot 16, durability 0，
称号属 durabilityOptional)，改前整行状态备份于
`AI产出/临时/save-repair-20260926/character2_state_before.json`。
