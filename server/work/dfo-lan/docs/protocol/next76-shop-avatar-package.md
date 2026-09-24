# next76 商城时装单件与礼盒购买(购买侧补全)

## 2026-09-24 合入与实机确认基线

- 将随修复计划交付的商城源码、测试、三个目录 JSON 与审计工具合入当前服务端；网关主循环仅将 CMD64 回执调用接到 `shopPilotSpaces`，保留其他现有逻辑。
- 合入时保留现有普通装备的 `[correction equipped level]` 等级修正规则，并扩展时装缺少 `[minimum level]` 时的穿戴判定；相应旧测试已传入装备类别。
- `go test -p 1 ./...` 与 `go vet -p 1 ./...` 通过；用户已手动实机验证并确认商城购买及装扮穿戴修复通过。候选程序 `bin/wireprobe-handoff-source.exe` SHA256 为 `925F2BE6702EAA447839B3B81B9DF438E3EA3F2AC77D0929F095BA5D996AE44B`，原候选程序保存在同目录的 `.bak-20260924-before-shop` 文件。
- 本机 `server/work/client-build/Script.inner.pvf` SHA256 为 `7EF2DB59331F7E5B18B2F250B8B907526BF2C94B17A7312036CF599644D88E80`，与商城目录源校验一致。该修复作为当前商城购买与装扮穿戴确认基线。

日期:2026-09-23 · 状态:四轮实机取证后修复完成,待复测

## 第四轮:光辉宝箱购买 / 契约合并购买 / 激活即时生效(2026-09-23 23:13 会话日志)

前三轮部署后实机确认:时装单件购买+穿戴打通、礼盒/宠物蛋/契约单买成功。日志新证据:

1. **`product3400232: special delivery action radiant treasure box`** —— Mystical Arcane Box(590712474)是 `[stackable type] [waste]` + `[action type] [radiant treasure box]` 的普通消耗品,开盒走既有 CMD160 光辉流程(next44),但购买侧被 `[action type]` 守卫整段拒绝(3400232/33/34/35 四个 SKU)。修复:删除 ordinaryHandler 对 `[action type]` 的拦截(原逻辑只拦 radiant,其余本就放行;现在 radiant 也随 `[stackable type]` 普通交付)。
2. **`premium contracts require a separate order` ×7** —— 五张契约一张购物车被旧设计整单拒绝。修复:存储层 `purchaseCash` 的单激活参数改为**按行索引的激活表**(`PurchaseCashMixed`:一次扣款 + 逐行激活契约 + 其余行走背包交付;契约行不进 cash_inventory);`Pilot.Purchase` 把购物车分区为契约行/普通行,契约行以合成商品参与同一 `Quote`,不再有单独订单限制。删除 `TryPurchaseContract`/`purchaseContract`/`PremiumLedger`。
3. **契约购买后不立即生效** —— 激活持久化正确但客户端权益状态要等重登(`premiums_restored`)。修复:回执携带激活结果(`receipt.Premiums`),包构建器在 ACK64 之后对每个激活追加 NOTI66 `CeraSpecialItemNotification(type, endTime)`(与 consume_flow/booster_flow 实机验证过的编码一致;置于 ACK 之后避免旧记录中"购买弹窗期间广播 NOTI66 崩溃"的时序)。

新增回归:`contract_cart_test.go`(三契约一单、数量缩放、契约+普通混单)、`TestShopPilotPremiumActivationNotice`/`TestShopPilotContractCartPurchase`(真实目录 3500001/3500009/3500016 合并购买 + NOTI66 包序)。

## 第三轮:穿戴被拒(皮肤/武器装扮缺 [minimum level])

购买与入库全部打通后,实机穿戴部分时装被拒:`equipment_move_refused "equipment minimum level not met or unavailable"` → 客户端弹 "The target inventory is full..."(通用 Refusal(4))。同批裤子(502510504)穿戴成功,被拒的是皮肤 502580005(`[skin avatar]`)与武器装扮 502600206(`[weapon avatar]`)。

真源对比:`equipment/character/fighter/avatar/pants/502510504.equ` 带 `[minimum level] 1`,而 `.../avatar/skin/502580005.equ` **没有该段**(时装源脚本常缺)。`inventory.WearableBy` 把字段缺失当"不可识别定义"拒绝。

修复:`WearableBy` 增加 equipment-kind 参数;`* avatar]` 家族缺 `[minimum level]` 视为无等级要求(职业/转职约束照旧生效);普通装备缺字段维持原守卫。槽位映射无需改动(`equipment-wear.full-candidate.json` 已含 `[skin avatar]`=8、`[weapon avatar]`=10)。新增 `wearable_avatar_test.go` 回归(真实脚本形状)。

## 第二轮:实机取证修复(2026-09-23 22:35 会话日志)

第一版部署后实机测试,`events.jsonl` 给出决定性证据:

1. **时装单件购买被 `Quote` 的 Kind 严格比对拒绝**:客户端购买时装的购物车条目是 `{option:0, kind:4, product:3107371}`,而服务端商品 Kind=0 → "unsupported product option" → 整车取消,客户端表现为"没反应"。对照源行,**3107370..73 的 `[dont trade avatar]` cell2 全部为 4** —— cell2 就是客户端的购买类别,投影到 `Product.Kind` 后 Quote 天然匹配。10,833 行中 cell2=4 有 10,571 行、=3 有 261 行,按源值逐行投影,不猜语义(3 是否为赠送待取证)。
2. **整套购物车被 `[package related]` 商品拖死**:实机整车为 3 件时装(kind=4)+ 3400489 Tropical Vacation Package(kind=0)+ 3400490 Arad Pass 整装盒(kind=0),任意一行失败整车回滚。3400490 的形状是 `col9=1`、`col12=20260915`(YYYYMMDD 档期尾),被旧的"display policy/sale condition"两道检查拒绝。
   - 修复:`[package related]` 加入普通家族(移除整族拒绝),其商品强制走 `packageHandler`(只许带源内容表的礼盒/礼包,非礼盒类照旧拒绝);`[item event]` 与 `[package related]` 放行 `col9∈{1,90}` 与 8 位日期 `col12`(档期显示由客户端把关,交付沿用 HasExpiration→MaxExpireTime 既有规则);`col3≠0` 的点数价格变体仍因 Cera=0 自然被拒。
3. **实机同时确认已生效**:3400268 称号礼盒(330 点券)与宠物蛋 3300002 购买扣款交付成功。
4. 新增回归:`TestMixedAvatarAndBoxCart`(时装 kind=4 + 礼盒 kind=0 混装整车)、Kind 投影与错 Kind 拒绝断言、`[package data]` 子项为模板 1(点券钱包)时不误判空交付(实例 3400296 Life Token Box(100) → Coin+100)。

严格模式可购 15,926 → **16,127**;`DFO_SHOP_OPEN_ALL=1` 时 17,154。

## 背景与真源

商城(CeraShop,CMD64 购买)此前只有普通商品(药水、复活币、宠物蛋、契约、金库扩容等约 31 个 SKU)可购买。时装(avatar)单件与整套礼盒(.package)从未实现:

- `etc/(r)cerashop.etc` 的商品行分散在多个 section。旧导入器只扫描 8 个普通 section(`[item]`、`[item etc]`、`[item second]`、`[item event]`、`[item mod or ext]`、`[item period or contract]`、`[creature]`、`[package related]`),且模板只从 `list/stackable.lst` 解析。
- **时装单件全部在 `[dont trade avatar]` section**(10,833 行,列分布:col0=商品号、col1=时装模板、col2∈{3,4}、col5=点券价 120..240、col10..12=-1、col13=空串)。时装模板(如 515540507,`equipment/character/priest/at_avatar/shoes/515540507.equ`)不在 stackable.lst,必须走 `list/equipment.lst` 解析,所以旧导入器全部标 "template missing from stackable.lst" 被拒。
- **整套礼盒/时装盒全部在 `[package]` section**(5,120 行,13 格/行:col0=商品号、col1=模板、col4=点券价、col7=名称、col8=90、col10..11=-1、col12=空串)。其中 `[usable cera package]` 2,681 个(2,583 个带 `[package data]`)、`[booster]` 2,351、`[cera booster]` 11。
- 旧版 `(f)cerashop.etc` 的 `[avatar]` section 是 2012 时期旧商城结构,新客户端不使用,不采纳。

审计结论(临时工具 `cmd/shopaudit`,保留在仓库):

- 两个新 section 的商品号与全部其它 section **零冲突**;
- [dont trade avatar] 10,833 个模板全部能在 equipment.lst 解析且路径都含 `avatar/`,脚本全部可读、路径匹配;
- [package] 5,120 行全部模板在 stackable.lst;77 行 col2∈{12000000,15000000} 是替代货币价格策略,拒绝。

## 实现

### 导入与分类(internal/cashshop/pvf_catalog.go)

- `importShopScripts` 增扫 `[dont trade avatar]`(14 格)与 `[package]`(13 格);商品号/模板 ≤0 的占位行直接丢弃。
- `resolveCreatureEquipment` 扩展为 `resolveEquipmentEntries`:除宠物蛋外,同法从 equipment.lst 解析时装模板(路径必须含 `avatar` 且 `.equ`,校验脚本路径匹配与 `(r)` 变体)。
- `validate()` 按 section 接受 13 格行([package])。
- `classify` 前置路由到两个新分类器:
  - `classifyAvatarPiece`:强制 Units=1、点券价=col5、按审计形状逐格校验(2∈{3,4}、3..9=0、10..12=-1、13 空串),交付类型 `Kind:"[avatar]"`、槽位 0..209、每格 1 件。
  - `classifyPackage`:Units=1、点券价=col4,拒绝替代货币行与未知标记;交付走 `packageHandler`(盒子本身作为不可叠加消耗品入库,开盒走既有 booster/package 流)。
- `ordinaryHandler` 增加礼盒预检:普通 section 里 `[stackable type]` 为礼盒族(`[booster]`/`[booster selection]`/`[booster random]`/`[cera booster]`/`[usable cera package]`)且脚本带内容表(`[package data]`/`[booster info]`/`[booster select category]`)的商品,作为封闭礼盒放行——例如 [item] 3400268 冒险者意志称号礼盒、[item event] 里的整套盒。限时字段继续按既有规则(交付时 expire=MaxInt32),不新增拒绝。
- `[immediately adaptive product]` 交叉检查改为预计算模板索引(`deriveImmediateTemplates`,LoadPilot/ImportPilot 时构建),避免 1.7 万条目下 O(E²)。

### 交付(internal/cashshop/pilot.go)

- `deliverAmount` 新增 `[avatar]` 分支:逐件放入 `Bag.Special[1]`(0..209 找空位,与 booster_flow 的实机验证路径一致),带 Period(限时商品时为 MaxInt32);ItemCatalog 可用时交叉校验 Kind 必须是 avatar。
- `resolveDeliveryType` 的 ItemCatalog 回退识别 `Kind=="avatar"`(覆盖礼包 `[package data]` 子项直接是时装的情况);回退宽度放宽到 13 格。
- `findEntry` 接受 13/14 两种行宽(礼包展开、契约、交付空间判断都依赖它)。
- 新增 `DeliverySpaces(receipt)`:按行/礼包子项解析购买触及的 space(1=装扮栏、7=宠物栏),供包构建器刷新对应分页。
- `products()` 增加 openAll 键控缓存;`LoadPilot` 预构建策略索引。

### 包下发(cmd/wireprobe/shop_pilot.go、main.go)

- `shopPilotPackets` 拆出 `shopPilotSpaces(pilot, receipt, balance, applied)`:有 pilot 时按 `DeliverySpaces` 追加 NOTI14 space1 装扮栏刷新(在主背包刷新之后、宠物/余额之前);宠物刷新在 SKU 段判定之外增加模板判定(礼包开出的宠物也能刷新)。
- `preparedBagLedger` 持有 pilot 引用,编码预检与真实发送(main.go CMD64 处)走同一函数。

### 顺带修复

- `internal/inventory/shop.go` 导出 `StackableSlotRange`(package_flow.go 引用的导出名此前不存在,**cmd/wireprobe 在本次改动前就已无法编译**)。
- `internal/cashshop/pvf_catalog_test.go`:3400268 从"禁止启用"移入"封闭礼盒启用"断言;全目录端到端循环改为感知礼包展开;OpenAll 数量 1292→17157。

## 目录重生成

```powershell
go run ./cmd/shopimport -source ../client-build/Script.inner.pvf -output configs/shop-vault-release.json -report configs/shop-purchase-report.json
go run ./cmd/shopimport -source ../client-build/Script.inner.pvf -output configs/shop-next-candidate.json -report configs/shop-purchase-report.json
```

- 严格模式启用 **15,926** 个商品(此前约 31):普通 1,292 中 43 个 + 时装单件 10,833 + 礼盒 5,050;`DFO_SHOP_OPEN_ALL=1` 时 17,157。
- 拒绝原因示例(报告见 configs/shop-purchase-report.json):礼盒无内容表、替代货币价格、时装行形状异常、模板缺索引。

## 验证

- `go vet ./internal/... ./cmd/...` 0 项。
- `go test -p 1 ./internal/... ./cmd/...` 21 包全绿(含 shop-vault-release.json 实目录端到端:3107733 单件时装入装扮栏、3400245 礼包展开子项、宠物蛋包序回归)。
- 新增回归:`internal/cashshop/avatar_package_test.go`(合成目录形状/拒绝矩阵 + 实目录抽查)。
- 服务端二进制已重建:`bin/wireprobe-handoff-source.exe`,SHA256 `E84FA6411222A3FFBDD32F5200CC6FE23DE60D7409B0C868541056847DF518DB`(旧版备份为 `.bak-20260923-shopavatar`)。

## 边界(不声明完成)

- 客户端 CMD64 购物车条目的 Kind 字节在时装/礼盒购买时的取值未实机取证;`Quote` 沿用严格校验,若实机报 "unsupported product option" 需按抓包补 Kind 投影(事件日志会记录 cera_purchase_rejected 原因)。
- 礼包 `[package data]` 子项若直接是契约模板,购买时按普通物品入包(未走契约激活);既有礼盒子项均为盒/材料,未观察到该形状。
- 替代货币(12M/15M)礼盒、限时租赁时装保持未实现;`[purchasing limit]` 等策略命中的商品维持拒绝。
- 实机验收项:商城时装页买单件、整套(购物车多件)一次扣款逐件入库;装扮栏即时刷新;礼盒购买→右键开盒;礼包购买→子盒展开;重登后装扮栏与背包一致。
