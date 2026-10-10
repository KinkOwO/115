package catalog

import "fmt"

// NativeCharacterSlotTicket is the current PE's role-capacity item identity
// (1468762c0). It binds an execution capability, not a table of item rewards.
const NativeCharacterSlotTicket uint32 = 2660239

type CharacterSlotTicketRule struct {
	Amount int
	Limit  int
}

// CharacterSlotTicket reads the authorized resource repair: [int data] holds
// capacity increment and absolute limit. The original [etc] item has no action
// and is refused, rather than silently spending a coupon without expanding.
func (c LootCatalog) CharacterSlotTicket(id uint32) (CharacterSlotTicketRule, bool, error) {
	var rule CharacterSlotTicketRule
	if id != NativeCharacterSlotTicket {
		return rule, false, nil
	}
	script, err := c.ItemScript(id)
	if err != nil {
		return rule, true, err
	}
	kind := sectionCells(script.Cells, "[stackable type]")
	data := sectionCells(script.Cells, "[int data]")
	if len(kind) != 2 || kind[0].Type != 6 || kind[0].Text != "[town and dungeon]" || kind[1].Type != 0 || kind[1].Value != 0 || len(data) != 2 || data[0].Type != 0 || data[1].Type != 0 {
		return rule, true, fmt.Errorf("角色栏扩展券缺少已验证的 PVF 使用定义")
	}
	rule.Amount, rule.Limit = int(data[0].Value), int(data[1].Value)
	// NOTI66/8 reads a byte; this PE supports 23 base + 77 paid slots.
	if rule.Amount < 1 || rule.Limit < 1 || rule.Limit > 100 || rule.Amount > rule.Limit {
		return rule, true, fmt.Errorf("角色栏扩展券的源数量或上限无效")
	}
	return rule, true, nil
}
