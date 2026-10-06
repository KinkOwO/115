package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"errors"
	"fmt"
	"strings"
)

var ErrMailUntradeable = errors.New("附件不可交易或尚未支持该特殊物品")
var ErrMailGold = errors.New("邮件金币不足或超出携带上限")
var ErrMailBagFull = errors.New("背包没有空位，邮件附件仍保留在邮箱")

// MailItem 保存完整实例，而非领取时重新按模板生成物品。
type MailItem struct {
	Stack     *BagItem      `json:"stack,omitempty"`
	Equipment *BagEquipment `json:"equipment,omitempty"`
	Space     byte          `json:"space,omitempty"` // 旧邮件缺省为普通背包；1 为时装栏。
}

func (m MailItem) Row() ([protocol.CurrentItemRecordSize]byte, error) {
	if m.Space != 0 && (m.Space != 1 || m.Equipment == nil || m.Stack != nil) {
		return [protocol.CurrentItemRecordSize]byte{}, fmt.Errorf("邮件附件容器无效")
	}
	if (m.Stack == nil) == (m.Equipment == nil) {
		return [protocol.CurrentItemRecordSize]byte{}, fmt.Errorf("邮件物品实例无效")
	}
	if m.Equipment != nil {
		if m.Space == 1 && (len(m.Equipment.AvatarOptions) > 30 || len(m.Equipment.AvatarSockets) > 4) {
			return [protocol.CurrentItemRecordSize]byte{}, fmt.Errorf("时装邮件扩展超出客户端容量")
		}
		if m.Equipment.Template < 2 {
			return [protocol.CurrentItemRecordSize]byte{}, fmt.Errorf("邮件装备模板无效")
		}
		if err := m.Equipment.ValidateRecord(); err != nil {
			return [protocol.CurrentItemRecordSize]byte{}, err
		}
		return EquipmentRow(*m.Equipment), nil
	}
	if m.Stack.Template < 2 || m.Stack.Amount == 0 {
		return [protocol.CurrentItemRecordSize]byte{}, fmt.Errorf("邮件堆叠物品无效")
	}
	return protocol.OrdinaryItem(m.Stack.Slot, m.Stack.Template, m.Stack.Amount, m.Stack.ExpireTime), nil
}

// TakeMailItem 只接受背包中的真实实例；不会读取客户端提供的装备属性。
// 绑定、任务物品和特殊容器不因邮件路径绕过原有规则。
func (b Bag) TakeMailItem(c catalog.LootCatalog, equipment *EquipmentCatalog, r protocol.MailSendItem) (Bag, MailItem, error) {
	fail := func(err error) (Bag, MailItem, error) { return b, MailItem{}, err }
	if r.List != 0 || r.Slot < 2 || r.Template < 2 || r.Amount == 0 {
		return fail(ErrMailUntradeable)
	}
	for i, row := range b.Items {
		if row.Slot != r.Slot {
			continue
		}
		if row.Template != r.Template || r.Amount > row.Amount {
			return fail(fmt.Errorf("邮件附件槽位或数量已改变"))
		}
		definition, ok := c.Items[row.Template]
		if !ok || definition.Kind != "stackable" {
			return fail(ErrMailUntradeable)
		}
		free := false
		for j, t := range definition.Script.Cells {
			if t.Type != 3 {
				continue
			}
			if t.Text == "[attach type]" && j+1 < len(definition.Script.Cells) {
				free = definition.Script.Cells[j+1].Text == "[free]"
			}
			switch t.Text {
			case "[cannot trade]", "[cannot send mail]", "[impossible contents]":
				return fail(ErrMailUntradeable)
			}
		}
		if !free || strings.Contains(definition.StackableType, "quest") {
			return fail(ErrMailUntradeable)
		}
		b.Items = append([]BagItem(nil), b.Items...)
		b.Items[i].Amount -= r.Amount
		if b.Items[i].Amount == 0 {
			b.Items = append(b.Items[:i], b.Items[i+1:]...)
		}
		row.Amount = r.Amount
		return b, MailItem{Stack: &row}, nil
	}
	for i, row := range b.Equipment {
		if row.Slot != r.Slot {
			continue
		}
		if b.TutorialSealed(row) {
			return fail(ErrMailUntradeable)
		}
		if row.Template != r.Template || r.Amount != 1 || equipment == nil {
			return fail(ErrMailUntradeable)
		}
		definition, err := equipment.Definition(row.Template)
		if err != nil {
			return fail(err)
		}
		attach, kind := definition.Fields["[attach type]"], definition.Fields["[equipment type]"]
		if len(attach) != 1 || attach[0].Text != "[free]" || len(kind) == 0 || len(row.AvatarOptions) != 0 || len(row.AvatarSockets) != 0 {
			return fail(ErrMailUntradeable)
		}
		// 时装与宠物走不同的邮件扩展 reader，不能用普通装备包截断它们。
		switch kind[0].Text {
		case "[creature]", "[avatar]", "[aura]":
			return fail(ErrMailUntradeable)
		}
		if err = row.ValidateRecord(); err != nil {
			return fail(err)
		}
		b.Equipment = append([]BagEquipment(nil), b.Equipment...)
		b.Equipment = append(b.Equipment[:i], b.Equipment[i+1:]...)
		return b, MailItem{Equipment: &row}, nil
	}
	return fail(fmt.Errorf("邮件附件不在背包中"))
}

func (b Bag) AddMailItem(c catalog.LootCatalog, r BagRules, equipment *EquipmentCatalog, m MailItem) (Bag, error) {
	if _, err := m.Row(); err != nil {
		return b, err
	}
	if m.Equipment != nil {
		item := *m.Equipment
		if m.Space == 1 {
			if equipment == nil || equipment.Source.Checksum != r.Source {
				return b, fmt.Errorf("时装邮件目录版本无效")
			}
			definition, err := equipment.definitionResolved(item.Template, 0)
			if err != nil {
				return b, err
			}
			kind := definition.Fields["[equipment type]"]
			if len(kind) == 0 || EquipmentBagSpace(kind[0].Text) != 1 {
				return b, fmt.Errorf("时装邮件模板类型不匹配")
			}
			// 使用角色的原生时装栏容量，保留附件实例全部属性。
			occupied := make(map[uint16]bool, len(b.Special[1]))
			for _, row := range b.Special[1] {
				occupied[row.Slot] = true
			}
			for slot := uint16(0); slot < protocol.AvatarInventorySlots(b.AvatarExpansion); slot++ {
				if occupied[slot] {
					continue
				}
				next := b
				next.Special = make(map[byte][]BagEquipment, len(b.Special)+1)
				for space, rows := range b.Special {
					next.Special[space] = rows
				}
				item.Slot = slot
				next.Special[1] = append(append([]BagEquipment(nil), b.Special[1]...), item)
				return next, nil
			}
			return b, ErrMailBagFull
		}
		if equipment == nil || r.EquipmentSlots[0] == 0 || r.EquipmentSlots[0] > r.EquipmentSlots[1] {
			return b, fmt.Errorf("邮件装备背包规则无效")
		}
		if _, err := equipment.Reward(item.Template); err != nil {
			return b, err
		}
		kind, err := equipment.EquipmentKind(item.Template)
		if err != nil {
			return b, err
		}
		if IsPetGear(kind) {
			next, _, err := b.AddPetGear(item)
			return next, err
		}
		next, slots, err := b.AddEquipment(equipment, r.EquipmentSlots, item.Template, 1)
		if err != nil {
			return b, ErrMailBagFull
		}
		item.Slot = slots[0]
		next.Equipment[len(next.Equipment)-1] = item
		return next, nil
	}
	item := *m.Stack
	definition, ok := c.Items[item.Template]
	if !ok || definition.Kind != "stackable" || r.Source != c.Source.Checksum {
		return b, fmt.Errorf("邮件物品目录版本无效")
	}
	limit := stackLimitFor(r, definition.StackableType, definition.StackLimit)
	if item.Amount > limit || limit == 0 {
		return b, fmt.Errorf("邮件附件超过堆叠上限")
	}
	if IsPetConsumable(definition.StackableType) {
		next, _, err := b.addPetStack(r, item.Template, item.Amount, item.ExpireTime, definition.StackLimit)
		return next, err
	}
	slots := stackableSlotRange(r, definition.StackableType)
	occupied := map[uint16]bool{}
	for _, e := range b.Equipment {
		occupied[e.Slot] = true
	}
	original := b
	b.Items = append([]BagItem(nil), b.Items...)
	for i, row := range b.Items {
		occupied[row.Slot] = true
		// 不同期限的堆叠必须分开，永久物品不能继承另一堆的到期时间。
		if row.Template == item.Template && row.ExpireTime == item.ExpireTime && (r.Quick(row.Slot) || row.Slot >= slots[0] && row.Slot <= slots[1]) && row.Amount < limit {
			added := min(limit-row.Amount, item.Amount)
			b.Items[i].Amount += added
			item.Amount -= added
			if item.Amount == 0 {
				return b, nil
			}
		}
	}
	for n := uint32(slots[0]); n <= uint32(slots[1]); n++ {
		if !occupied[uint16(n)] {
			item.Slot = uint16(n)
			b.Items = append(b.Items, item)
			return b, nil
		}
	}
	return original, ErrMailBagFull
}
