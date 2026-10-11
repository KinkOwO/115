package inventory

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"time"
)

// The native portable window sends the selected tool in CMD80's ordinary
// ticket slot. Its action chooses the mode; it is neither a fixed-level ticket
// nor the normal material. Contents and limits are imported from the same STK
// scan as the other enhancement families.
var portableEnhancements map[uint32]reinforcementTicket

func portableMode(fields map[string][]pvf.Token) (byte, bool) {
	v := fields["[action type]"]
	if len(v) == 0 || v[0].Type != 6 {
		return 0, false
	}
	switch v[0].Text {
	case "[portable upgrade]":
		return 0, true
	case "[portable amplify]":
		return 1, true
	}
	return 0, false
}

func IsPortableEnhancement(template uint32, mode byte) bool {
	rule, ok := portableEnhancements[template]
	if !ok {
		return false
	}
	m, ok := portableMode(rule.Fields)
	return ok && m == mode
}

func (c *EnhancementCatalog) importPortable(id uint32, path string, fields map[string][]pvf.Token) error {
	if _, ok := portableMode(fields); !ok {
		return nil
	}
	v := fields["[action type]"]
	if len(v) != 3 || v[1].Type != 0 || v[2].Type != 0 || v[1].Value < 0 || v[2].Value < v[1].Value || v[2].Value > 31 {
		return fmt.Errorf("invalid portable enhancement action %d: %s", id, path)
	}
	if c.Portable == nil {
		c.Portable = map[uint32]reinforcementTicket{}
	}
	c.Portable[id] = reinforcementTicket{Path: path, Fields: fields}
	return nil
}

func (s *WearService) validatePortable(role Role, r protocol.ReinforcementRequest, item BagItem, gear BagEquipment, d EquipmentDefinition, level int) error {
	rule, ok := portableEnhancements[item.Template]
	if !ok || !IsPortableEnhancement(item.Template, r.Mode) {
		return Refuse(RefusalUnsupported, "一次性强化/增幅道具与请求模式不符")
	}
	if (r.EquipmentSpace != 0 && r.EquipmentSpace != 3) || r.TicketSpace != 0 || r.TicketSlot == 0xffff || r.MaterialSlot != 0xffff || r.ProtectionSlot != 0xffff || r.Multiple != 0 {
		return Refuse(RefusalUnsupported, "一次性强化/增幅只支持背包中的单次道具")
	}
	if item.Amount == 0 {
		return Refuse(RefusalItems, "一次性强化/增幅道具已用完")
	}
	if item.ExpireTime >= 946684800 && item.ExpireTime != 2147483647 && protocol.StoredItemExpired(item.ExpireTime, time.Now().Unix()) {
		return Refuse(RefusalUnsupported, "一次性强化/增幅道具已经过期")
	}
	v := rule.Fields["[action type]"]
	if level < int(v[1].Value) || level > int(v[2].Value) {
		return Refuse(RefusalLimit, "装备当前强化/增幅等级不符合道具要求")
	}
	if levels := rule.Fields["[check usable itemlevel]"]; len(levels) > 0 {
		actual, ok := singleInt(d, "[minimum level]")
		if !ok || len(levels) != 2 || levels[0].Type != 0 || levels[1].Type != 0 || actual < levels[0].Value || actual > levels[1].Value {
			return Refuse(RefusalUnsupported, "装备等级不符合一次性道具要求")
		}
	}
	if allowed := rule.Fields["[reinforcement usable item list]"]; len(allowed) > 0 {
		found := false
		for _, t := range allowed {
			if t.Type == 0 && uint32(t.Value) == gear.Template {
				found = true
			}
		}
		if !found {
			return Refuse(RefusalUnsupported, "装备不在一次性道具的专用清单内")
		}
	}
	var state struct {
		Level byte `json:"level"`
	}
	if err := json.Unmarshal(role.State, &state); err != nil {
		return err
	}
	if min := rule.Fields["[minimum level]"]; len(min) > 0 && (len(min) != 1 || min[0].Type != 0 || int32(state.Level) < min[0].Value) {
		return Refuse(RefusalUnsupported, "角色等级不符合一次性道具要求")
	}
	if jobs := rule.Fields["[usable job]"]; len(jobs) > 0 {
		job, known := s.Professions.Professions[role.Profession]
		allowed := false
		for _, j := range jobs {
			if j.Type == 6 && (j.Text == "[all]" || known && j.Text == job.Job) {
				allowed = true
			}
		}
		if !allowed {
			return Refuse(RefusalUnsupported, "角色职业不符合一次性道具要求")
		}
	}
	// This executor is reached only from the town gateway. Keep special event
	// contexts and extra-payment tools closed until their native contract is known.
	for _, p := range rule.Fields["[action usable place]"] {
		if p.Type != 6 || (p.Text != "[village]" && p.Text != "[seria room]" && p.Text != "[dont use assult]") {
			return Refuse(RefusalUnsupported, "一次性道具包含尚未核实的使用场所限制")
		}
	}
	for _, tag := range []string{"[impossible contents]"} {
		if len(rule.Fields[tag]) > 0 {
			return Refuse(RefusalUnsupported, "该专用一次性道具的限制尚待核实：%s", tag)
		}
	}
	bag, err := ReadBag(role.State)
	if err != nil {
		return err
	}
	_, err = spendPortableMaterials(bag, item.Template, item.Slot, time.Now().Unix())
	return err
}

// Native 147128685 reads [need material] as (template, amount) pairs into
// the common material vector at +0xA8. Ordinary town actions pay those costs
// atomically with the tool. Account-storage references need a separate request.
func spendPortableMaterials(bag Bag, tool uint32, toolSlot uint16, now int64) (Bag, error) {
	rule, ok := portableEnhancements[tool]
	if !ok {
		return Bag{}, Refuse(RefusalUnsupported, "一次性道具源规则不可用")
	}
	cells := rule.Fields["[need material]"]
	if len(cells)%2 != 0 {
		return Bag{}, Refuse(RefusalUnsupported, "一次性道具材料源表不完整")
	}
	costs := map[uint32]uint64{}
	for n := 0; n < len(cells); n += 2 {
		if cells[n].Type != 0 || cells[n+1].Type != 0 || cells[n].Value <= 0 || cells[n+1].Value <= 0 {
			return Bag{}, Refuse(RefusalUnsupported, "一次性道具材料源表无效")
		}
		costs[uint32(cells[n].Value)] += uint64(cells[n+1].Value)
	}
	for id, count := range costs {
		var available uint64
		for _, item := range bag.Items {
			if item.Template == id && item.Slot != toolSlot && !protocol.StoredItemExpired(item.ExpireTime, now) {
				available += uint64(item.Amount)
			}
		}
		if available < count {
			return Bag{}, Refuse(RefusalMaterials, "一次性道具材料不足：模板 %d 需要 %d", id, count)
		}
	}
	out := bag
	out.Items = append([]BagItem(nil), bag.Items...)
	rows := out.Items[:0]
	for _, item := range out.Items {
		if need := costs[item.Template]; need > 0 && item.Slot != toolSlot && !protocol.StoredItemExpired(item.ExpireTime, now) {
			take := min(need, uint64(item.Amount))
			item.Amount -= uint32(take)
			costs[item.Template] -= take
		}
		if item.Amount > 0 {
			rows = append(rows, item)
		}
	}
	out.Items = rows
	return out, nil
}
