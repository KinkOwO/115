package inventory

// AvatarPresetTicketTemplate 是 "Avatar Preset Expansion Ticket" 的模板 id。
//
// 链：etc/(r)cerashop.etc product 3003017 → template 590723098
// → stackable/cash/590723098.stk（**平铺路径**，不是衣柜那种 dfo/cash/2017/0613/…）。
//
// 与衣柜扩展券（脚本里没有 [action type]、走"购买即生效"）不同，这张的道具脚本是
// 右键使用类：
//
//	[action type] `[add avatar preset]`
//	[use action packet] 0
//	[action usable place] `[village]`
//
// ★ 它的 CMD507 动作号**读不出来**：[use action packet] 恒为 0（已知动作 197 的
// 宠物幻化栏券该字段同样是 0），动作号只存在于客户端 exe 里，只能靠实机抓包取。
// 所以这里的识别一律走"模板"，不依赖动作号——与 DecodeQuestAirshipAction 的
// "item identity is resolved from the owned bag slot" 同一口径。
const AvatarPresetTicketTemplate uint32 = 590723098

// HoldsAvatarPresetTicket 判断某个槽位里装的是不是装扮预设扩展券。
func HoldsAvatarPresetTicket(b Bag, slot uint16) bool {
	for _, item := range b.Items {
		if item.Slot == slot {
			return item.Template == AvatarPresetTicketTemplate
		}
	}
	return false
}

// AvatarPresetTicketSlot 找出主背包里第一张装扮预设扩展券所在的格子。
// 只在请求没有点名槽位时兜底（客户端一般会点名）。
func AvatarPresetTicketSlot(b Bag) (uint16, bool) {
	for _, item := range b.Items {
		if item.Template == AvatarPresetTicketTemplate {
			return item.Slot, true
		}
	}
	return 0, false
}
