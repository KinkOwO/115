# Cera 商城金币购买（2026-10-01，已确认）

## 状态与运行假设

attempt 1/3 已完成并由用户确认可购买。普通14列商城商品按当前PVF金币列定价，通过原有CMD64购买和同一笔角色事务扣金币、发货、保存审计。确认范围包括下述SKU3400476金币购买实机订单，不扩大为扩容、特殊货币或所有商城商品逐项验收。

## 证据

- 实机会话：roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_205110_953979_next37。
- events.jsonl 179–181（20:53:18）及188–190（20:53:37）：CMD64 body=00000100007be2330001000000000000；SKU3400315、数量1、校验/解码通过，拒绝 product 3400315 not enabled for cash delivery。未扣款。
- 183–187（20:53:26）记录普通 Cera SKU3000127 成功，用户也确认 Cera 点购买可用；该身份修复已提交9bba6d6。
- 当前内层 PVF 直读：SKU3400315，模板590715403，单位1，r[3]=100、r[5]=0。原生数据库回归验证实际价格100金币，不以历史 JSON 定价。
- 复用现有权威 IDB 会话 def2fcf1，只读分析，没有直接打开或修改 IDB。1477C2FC0 的 [item event] 分支调用1477C7870(category21)；普通行读取顺序将r[3]写至结构+96、r[5]写至+104。1446FFF70读取价格（+104优先，否则+96），1446FFFD0返回货币类型4/0；1408829C0按该价格检查余额，14087B6D0的类型0从角色背包钱包取余额，类型4取Cera。
- 140889950统一发送CMD64：首字节只有类型5为1，金币类型0与Cera类型4同为0，随后14088DFF0写购物车。现场Gold body与现有DecodeCeraCart匹配。
- 成功消费145267D40没有Gold专属字段分支，继续调用140887530收口每条SKU/数量。保留既有CeraPurchaseOrdinarySuccess布局。
- 金币刷新使用既有NOTI13 list0：1452D5A80读取u8列表、u16扩展格数、u16行数和181字节记录；1452D61E6读记录，1452D714F回写列表，1452D73F8完成刷新。与账号金库金币现有实现一致，发送包含槽0的完整快照；商城NOTI14仍排除金币/复活币槽，避免虚拟货币触发物品提示路径。
- legacy da3d36f与重构前后检查的普通商城定价代码一致，仅支持Cera；旧版金币购买的具体成功路径尚无同一SKU动态日志。此项依据当前115和失败现场修复，不把旧代码当成协议事实。

## 实现及兼容性

- cashshop.Product.Gold 与订单行 GoldUnitPrice 存储源价格；每条商品只准一种货币，整车分别累计Gold/Cera。金币负数、双价格、未知替代货币拒绝，启动器DFO_SHOP_OPEN_ALL也不能绕过金币货币门禁。
- 原有Store事务锁定角色和钱包。Pilot的发货回调先校验并扣角色Bag.Gold，再发放商品；Cera扣款、角色保存、订单与发货审计仍在同一事务。编码预检失败或满包均回滚。
- GoldCharged随订单回执保存，发出成功ACK后通过NOTI13恢复当前金币；重复请求使用当前角色状态，不再扣款或复发物品。
- 无数据库表结构变更；Gold订单字段均可选，旧Cera订单JSON摘要逐字节兼容，旧订单/回执仍可读。SaveIdentity检查、存档未知字段、商品来源校验继续保留。没有操作玩家数据库，也未修改客户端、profile或内容导出配置。
- 本次支持已具备发货处理器的普通14列金币商品；其他货币、特殊价策略、13列时装等不据此扩大支持。

## 验证

- internal/cashshop 和 internal/storage 整包通过。GoldShopDebitAndDelivery覆盖1000→800金币、分堆、余额不足和满包原状态保留；GoldCashOrderValidationAndLegacyJSON覆盖混合价格、免费/双货币/溢出拒绝和旧Cera订单JSON兼容。
- 独立 PostgreSQL16.4/Redis5.0.14（25459/26459）与临时schema：直接读Script.inner.pvf，真实捕获Gold body成功扣100金币、模板590715403×1到账、Cera不变；余额不足、错误存档身份、编码失败无扣款；重放保留更新后的777金币及无关数据，没有可重复领取的待发物品。最终NOTI13等于当前完整背包快照。
- 原生Cera回归SKU3400232（7900点、模板590712474×100）继续通过。
- 原生混合车Gold100+Cera7900：两种余额不足和编码失败均不提交；同时两个同key请求只有一个applied，最终777→677金币、20000→12100Cera，两条订单审计（含此前单独Gold订单）。
- 上述原生回归在默认门禁和启动器DFO_SHOP_OPEN_ALL=1分别通过。
- go vet ./...通过，build -trimpath通过；go test ./...仍有改动前已在干净HEAD复现的5项失败：TestAdventureAuditProvenanceAllowanceIsNarrow、TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback、TestEnhancementAuditAllowsOnlyMissingOrdinaryTicketExpirationHeader、TestOdysseyChapterFinalLordDrop、TestOdysseyCurrencySceneRetryAndPoolIsolation。没有新增失败，见先前身份修复证据文档。

## 实机确认与回退

- 用户确认金币商品能够购买。会话 roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261001_213843_140102_next37 的 events.jsonl 于本地时间2026-10-01 21:53:40记录 cera_purchase_committed：applied=true、SKU3400476、扣100金币、Cera 921780→921780、模板590721400×1到账；同一毫秒记录CMD64帧。
- 源码程序SHA256 c518e50af555178018e85604eb45c814d77f0d108175b8c5170196b5f3e468b5 纳入商城购买confirmed baseline，继续使用启动游戏.cmd --source-build。确认不覆盖默认二进制、扩容商品或其他特殊价格策略。
- 修复前已确认Cera普通购买程序SHA256 ffda6686159700d293a1e9395190f218ce8f094a25208ed84fc7220b94e72263备份于 .tmp/gold-shop-20261001/wireprobe-handoff-source.before.exe。需要回退时关闭会话后复制回 bin/wireprobe-handoff-source.exe，再通过 --source-build 启动；无需迁移或回滚数据库。