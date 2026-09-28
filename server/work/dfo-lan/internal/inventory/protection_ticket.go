package inventory

import "fmt"

// 强化/增幅保护券：失败到会碎的等级区间时，自动消耗一张保护券，
// 让装备不被破坏但强化/增幅等级归零（PVF 物品描述原文）。
//
// 判据是 PVF 物品脚本里的 **[action type]**，不是文件路径名：
//   - 增幅保护券 = `[amplify protect equipment]`
//   - 强化保护券 = `[protect equipment]`
//
// 同一张保护券在不同年份/活动有不同模板 id（两套命名混用：protect_amplifying /
// protect_reinforcement 与 amp_protection / amp_protect / re_protect），
// 按路径名扫会漏（实测：按路径名只找到 16 个增幅券，按 [action type] 是 38 个）。
//
// 客户端强化窗口**没有独立保护券槽位**（请求里的 ProtectionSlot 恒 0xffff），
// 原版语义是「背包持有保护券、失败到会碎区间自动消耗」⇒ 服务端按模板 id 全背包查找。
//
// 触发门槛（物品描述）：
//   - 强化保护券：其他装备失败到 +11（当前 +10）或以上、武器失败到 +13（当前 +12）或以上会碎 → 触发
//   - 增幅保护券：增幅失败到 +11（当前 +10）或以上会碎 → 触发

// 默认（2017 cera_shop 版）参考模板。
const (
	amplifyProtectionTicket   uint32 = 50022395
	reinforceProtectionTicket uint32 = 50022396
)

// amplifyProtectionTemplates 是增幅保护券模板 id 全集（38 个）。
//
// 口径：对 PVF 内层归档做「路径含 protect/amp 的 .stk」候选扫描，
// 再逐个读物品脚本、按 `[action type] = [amplify protect equipment]` 定罪
// （`[protect equipment]` 不是它的子串，两段互斥）。
// [action type] 是**客户端强制**语义，不会随服务端改动而变，故可写死在代码里。
var amplifyProtectionTemplates = map[uint32]bool{
	// 2015~2018 商城 / 活动（protect_amplifying.stk）
	50002930: true, 50020513: true, 50021430: true, 50021685: true,
	50022355: true, 50022395: true, 50023487: true, 50042794: true,
	// 2016 callofdfo：文件名写 boost，脚本段却是增幅保护（只能按段判）
	50006291: true,
	// 2019~2022 cash shop 各版本（amp_protection.stk / amp_protect.stk / protect_amplifying.stk）
	50041932: true, 501600220: true, 590004570: true, 590004933: true,
	590005093: true, 590005107: true, 590005194: true, 590005244: true,
	590005429: true, 590700823: true, 590701231: true, 590701399: true,
	590701402: true, 590701768: true, 590701785: true, 590701792: true,
	590701859: true, 590702699: true, 590702772: true, 590703153: true,
	590703285: true, 590703309: true, 590703383: true, 590703386: true,
	590703971: true, 590703998: true, 590704157: true, 590704273: true,
	// 当前 115 版本增幅保护券（用户原持有）
	10014791: true,
}

// reinforceProtectionTemplates 是强化保护券模板 id 全集（25 个）。
//
// 口径同上，判据为 `[action type] = [protect equipment]`。
var reinforceProtectionTemplates = map[uint32]bool{
	// 2015~2018 商城 / 活动（protect_reinforcement.stk / protectticket / goldprotection / equipment_protection）
	50002929: true, 50004009: true, 50004010: true, 50006290: true,
	50020512: true, 50021016: true, 50021174: true, 50021288: true,
	50021686: true, 50021848: true, 50022356: true, 50022396: true,
	50023488: true, 590005428: true, 590701786: true, 590703282: true,
	590704154: true,
	// 2019~2022 cash shop（re_protect.stk）
	50041931: true, 590701230: true, 590702698: true, 590703970: true,
	// 当前 115 版本强化保护券（用户原持有）
	10093371: true, 10088155: true, 8050: true, 10316796: true,
}

// IsAmplifyProtectionTicket 报告模板是否为任一版本的增幅保护券。
func IsAmplifyProtectionTicket(template uint32) bool {
	return amplifyProtectionTemplates[template]
}

// IsReinforceProtectionTicket 报告模板是否为任一版本的强化保护券。
func IsReinforceProtectionTicket(template uint32) bool {
	return reinforceProtectionTemplates[template]
}

// IsProtectionTicket 报告模板是否为任一保护券。
func IsProtectionTicket(template uint32) bool {
	return IsAmplifyProtectionTicket(template) || IsReinforceProtectionTicket(template)
}

// consumeProtectionTicket 从背包指定槽位扣掉一张保护券。
// 返回扣券后的背包；扣空则移除该行。slot 上不是保护券或数量不足时报错。
func consumeProtectionTicket(b Bag, slot uint16) (Bag, error) {
	rows := append([]BagItem(nil), b.Items...)
	for i, row := range rows {
		if row.Slot != slot {
			continue
		}
		if !IsProtectionTicket(row.Template) {
			return b, fmt.Errorf("保护券槽位放的不是保护券")
		}
		if row.Amount < 1 {
			return b, fmt.Errorf("保护券数量不足")
		}
		if row.Amount == 1 {
			rows = append(rows[:i:i], rows[i+1:]...)
		} else {
			rows[i].Amount = row.Amount - 1
		}
		b.Items = rows
		return b, nil
	}
	return b, fmt.Errorf("保护券不在所属角色背包")
}

// protectionTicketAtSlot 返回指定槽位上的保护券类型（强化/增幅），没有保护券返回 false。
func protectionTicketAtSlot(b Bag, slot uint16) (isAmplify bool, present bool) {
	for _, item := range b.Items {
		if item.Slot != slot {
			continue
		}
		if IsAmplifyProtectionTicket(item.Template) {
			return true, true
		}
		if IsReinforceProtectionTicket(item.Template) {
			return false, true
		}
		return false, false
	}
	return false, false
}

// findProtectionTicket 在背包里查找指定类型的保护券（wantAmplify=true 找增幅保护券，
// false 找强化保护券），返回第一个匹配槽位。客户端强化窗口没有独立保护券槽位，
// 原版语义是「背包持有保护券、失败到会碎区间自动消耗」，故按类型全背包查找。
func findProtectionTicket(b Bag, wantAmplify bool) (slot uint16, present bool) {
	for _, item := range b.Items {
		if wantAmplify {
			if IsAmplifyProtectionTicket(item.Template) {
				return item.Slot, true
			}
		} else if IsReinforceProtectionTicket(item.Template) {
			return item.Slot, true
		}
	}
	return 0, false
}
