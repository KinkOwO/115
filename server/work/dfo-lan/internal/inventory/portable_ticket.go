package inventory

// 便携强化器 / 增幅器 / 锻造炉：消耗性道具，使用后对装备执行一次
// 强化 / 增幅 / 锻造，只消耗道具本身（不耗无色、矛盾结晶体、Powerful Energy 或金币）。
//
// 判据是 PVF 物品脚本里的 **[action type]**（客户端强制语义，不会随服务端改动而变）：
//   - 强化器  = `[portable upgrade]`
//   - 增幅器  = `[portable amplify]`
//   - 锻造炉  = `[portable genuine damage upgrade upgrede]`
//
// 成功率沿用普通强化 / 普通增幅 / 普通锻造的成功率表（不固定）。
//
// 请求形态与普通强化/增幅/锻造完全一致（CMD80 mode 0/1、CMD430），唯一差别是
// 材料槽（强化/增幅的 TicketSlot、锻造的 MaterialSlot）放的是道具本身。
//
// 当前 115 版本用户实际持有的三个模板（2026-10-07 实机事件日志确认）：
//   590723097 = stackable/cash/590723097.stk            "Single-Use Equipment Reinforcer"
//   590723096 = stackable/cash/590723096.stk            "Single-Use Equipment Amplifier"
//   10307734  = stackable/10307001/10307734.stk         "Single-use Weapon Refiner"
//
// 失败规则由用户确认：强化/增幅失败**沿用普通强化/增幅惩罚**（掉级/降级/摧毁，
// 支持保护券）；锻造失败沿用官方锻造规则（等级不变、装备不碎）。

// portableUpgradeTemplates 是便携强化器模板 id 全集。
var portableUpgradeTemplates = map[uint32]bool{
	590723097: true,
}

// portableAmplifyTemplates 是便携增幅器模板 id 全集。
var portableAmplifyTemplates = map[uint32]bool{
	590723096: true,
}

// portableRefineTemplates 是便携锻造炉模板 id 全集。
var portableRefineTemplates = map[uint32]bool{
	10307734: true,
}

// IsPortableUpgradeTemplate 报告模板是否为便携强化器（[portable upgrade]）。
func IsPortableUpgradeTemplate(template uint32) bool {
	return portableUpgradeTemplates[template]
}

// IsPortableAmplifyTemplate 报告模板是否为便携增幅器（[portable amplify]）。
func IsPortableAmplifyTemplate(template uint32) bool {
	return portableAmplifyTemplates[template]
}

// IsPortableRefineTemplate 报告模板是否为便携锻造炉（[portable genuine damage upgrade upgrede]）。
func IsPortableRefineTemplate(template uint32) bool {
	return portableRefineTemplates[template]
}
