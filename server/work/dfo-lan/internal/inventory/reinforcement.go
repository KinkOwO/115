package inventory

import (
	"crypto/rand"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type reinforcementTicket struct {
	Path   string                 `json:"path"`
	Fields map[string][]pvf.Token `json:"fields"`
}

var reinforcementTickets map[uint32]reinforcementTicket

func IsReinforcementTicket(template uint32) bool {
	_, ok := reinforcementTickets[template]
	return ok
}

func LoadReinforcementTickets(path string) error {
	var c struct {
		Version      int                            `json:"version"`
		ClientSHA256 string                         `json:"client_sha256"`
		Items        map[uint32]reinforcementTicket `json:"items"`
	}
	if loaded, err := loadOptionalJSON(path, &c); !loaded || err != nil {
		return err
	}

	if c.Version != 1 || len(c.ClientSHA256) != 64 || len(c.Items) == 0 {
		return fmt.Errorf("强化券源目录无效")
	}
	for id, item := range c.Items {
		if id < 2 || item.Path == "" || len(item.Fields["[equipment reinforcement ticket]"]) != 4 {
			return fmt.Errorf("强化券 %d 源定义不完整", id)
		}
	}
	reinforcementTickets = c.Items
	return nil
}

type ReinforcementReceipt struct {
	Request            protocol.ReinforcementRequest `json:"request"`
	Ticket             uint32                        `json:"ticket"`
	Remaining          uint32                        `json:"remaining"`
	Old, Level, Result byte
	Equipment          BagEquipment `json:"equipment"`
}

// 普通强化与增幅是不同系统。本入口只处理普通固定等级券，不代替金币强化。
func (s *WearService) ApplyReinforcement(role Role, r protocol.ReinforcementRequest) (json.RawMessage, ReinforcementReceipt, error) {
	var out ReinforcementReceipt
	fail := func(kind RefusalKind, reason string) (json.RawMessage, ReinforcementReceipt, error) {
		return nil, out, Refuse(kind, "%s", reason)
	}
	if r.Mode != 0 || (r.EquipmentSpace != 0 && r.EquipmentSpace != 3) || r.TicketSpace != 0 ||
		r.TicketSlot == 0xffff || r.MaterialSlot != 0xffff || r.ProtectionSlot != 0xffff || r.Multiple != 0 {
		return fail(RefusalUnsupported, "当前只支持单张普通强化券，不支持金币、增幅、保护券或批量强化")
	}
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, out, err
	}
	ticketIndex := -1
	for i, item := range bag.Items {
		if item.Slot == r.TicketSlot {
			ticketIndex = i
			break
		}
	}
	if ticketIndex < 0 || bag.Items[ticketIndex].Amount == 0 {
		return fail(RefusalItems, "强化券不在所属角色背包")
	}
	item := bag.Items[ticketIndex]
	rule, ok := reinforcementTickets[item.Template]
	if !ok {
		return fail(RefusalUnsupported, "未找到这张物品的普通强化券规则")
	}
	v := rule.Fields["[equipment reinforcement ticket]"]
	if len(v) != 4 || v[0].Type != 0 || v[1].Type != 0 || v[2].Type != 6 || v[2].Text != "fixed" || v[3].Type != 0 || v[3].Value != -1 {
		return fail(RefusalGeneric, "该券不是已核实的普通固定等级强化券")
	}
	if v[0].Value < 1 || v[0].Value > 15 || !protocol.FixedReinforcementSupported(byte(v[0].Value)) {
		return fail(RefusalLimit, "客户端不支持该固定强化券的目标等级；普通固定券仅支持 +1 到 +15，不扣券")
	}
	if v[1].Value < 0 || v[1].Value > 100 {
		return fail(RefusalGeneric, "强化券成功率无效")
	}
	if item.ExpireTime >= 946684800 && item.ExpireTime != 2147483647 && protocol.StoredItemExpired(item.ExpireTime, time.Now().Unix()) {
		if _, usable := rule.Fields["[usable expired item]"]; !usable {
			return fail(RefusalGeneric, "强化券已经过期")
		}
	}
	// 尚未核实的专用券限制不能静默忽略；拒绝时事务不扣除任何物品。
	for _, tag := range []string{"[need material]", "[used trade type]", "[mod attr available level]", "[advanced enchant item by ticket window type]", "[action usable place]"} {
		if len(rule.Fields[tag]) > 0 {
			return fail(RefusalGeneric, "该专用强化券包含尚未支持的限制："+tag)
		}
	}
	var state struct {
		Level byte `json:"level"`
	}
	if err = json.Unmarshal(role.State, &state); err != nil {
		return nil, out, err
	}
	if min := rule.Fields["[minimum level]"]; len(min) > 0 && (len(min) != 1 || min[0].Type != 0 || int32(state.Level) < min[0].Value) {
		return fail(RefusalGeneric, "角色等级不符合强化券要求")
	}
	jobs := rule.Fields["[usable job]"]
	job, jobKnown := s.Professions.Professions[role.Profession]
	allowedJob := false
	for _, j := range jobs {
		if j.Type == 6 && (j.Text == "[all]" || jobKnown && j.Text == job.Job) {
			allowedJob = true
		}
	}
	if !allowedJob {
		return fail(RefusalGeneric, "角色职业不符合强化券要求")
	}
	items := bag.Equipment
	if r.EquipmentSpace == 3 {
		items = bag.Worn
	}
	gearIndex := -1
	for i, gear := range items {
		if gear.Slot == r.EquipmentSlot && gear.Template == r.EquipmentTemplate {
			gearIndex = i
			break
		}
	}
	if gearIndex < 0 {
		return fail(RefusalGeneric, "目标装备不在指定的所属角色槽位")
	}
	gear := items[gearIndex]
	if err = gear.ValidateRecord(); err != nil {
		return nil, out, err
	}
	d, err := s.Catalog.Definition(gear.Template)
	if err != nil {
		return nil, out, err
	}
	kind := d.Fields["[equipment type]"]
	if len(kind) == 0 || kind[0].Type != 6 {
		return fail(RefusalGeneric, "目标装备类型无效")
	}
	switch kind[0].Text {
	case "[weapon]", "[coat]", "[pants]", "[shoulder]", "[waist]", "[shoes]", "[amulet]", "[wrist]", "[ring]", "[support]", "[magic stone]", "[earring]":
	default:
		return fail(RefusalGeneric, "此类物品不能使用普通装备强化券")
	}
	if ids := rule.Fields["[reinforcement usable item list]"]; len(ids) > 0 {
		matched := false
		for _, id := range ids {
			if id.Type == 0 && uint32(id.Value) == gear.Template {
				matched = true
			}
		}
		if !matched {
			return fail(RefusalGeneric, "目标装备不在强化券允许列表")
		}
	}
	if limits := rule.Fields["[check usable itemlevel]"]; len(limits) > 0 {
		level, valid := singleInt(d, "[minimum level]")
		if !valid || len(limits) != 2 || limits[0].Type != 0 || limits[1].Type != 0 || level < limits[0].Value || level > limits[1].Value {
			return fail(RefusalGeneric, "装备等级不符合强化券限制")
		}
	}
	for _, tag := range []string{"[check usable equip type]", "[possible equipment part]"} {
		if values := rule.Fields[tag]; len(values) > 0 {
			matched := false
			for _, value := range values {
				if value.Type == 6 && strings.TrimPrefix(value.Text, "equip_") == strings.Trim(kind[0].Text, "[]") {
					matched = true
				}
			}
			if !matched {
				return fail(RefusalGeneric, "装备部位不符合强化券限制")
			}
		}
	}
	if values := rule.Fields["[unused rarity]"]; len(values) > 0 {
		rarity, valid := singleInt(d, "[rarity]")
		if !valid {
			return fail(RefusalUnsupported, "无法核对目标装备品质")
		}
		for _, value := range values {
			if value.Type != 0 || value.Value == rarity {
				return fail(RefusalGeneric, "装备品质不符合强化券限制")
			}
		}
	}
	row := EquipmentRow(gear)
	// 14576D8B0 -> 145770B50：行偏移 10 的低五位为强化等级，高三位保留。
	old := row[10] & 31
	if row[13] != 0 || row[18] != 0 || row[19] != 0 || row[20] != 0 || row[21] != 0 {
		return fail(RefusalGeneric, "封装或带特殊强化属性的装备不能走普通强化券路径")
	}
	if old >= byte(v[0].Value) {
		return fail(RefusalGeneric, "当前强化等级已达到或超过券的目标等级")
	}
	roll, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return nil, out, err
	}
	level, result := old, byte(1)
	if roll.Int64() < int64(v[1].Value) {
		level, result = byte(v[0].Value), 0
	}
	row[10] = row[10]&0xe0 | level
	gear.Record = append([]byte(nil), row[:]...)
	items[gearIndex] = gear
	if r.EquipmentSpace == 3 {
		bag.Worn = items
	} else {
		bag.Equipment = items
	}
	remaining := item.Amount - 1
	if remaining == 0 {
		bag.Items = append(bag.Items[:ticketIndex:ticketIndex], bag.Items[ticketIndex+1:]...)
	} else {
		bag.Items[ticketIndex].Amount = remaining
	}
	out = ReinforcementReceipt{Request: r, Ticket: item.Template, Remaining: remaining, Old: old, Level: level, Result: result, Equipment: gear}
	if _, err = protocol.ReinforcementTicketReply(r, remaining, old, level, result); err != nil {
		return nil, out, err
	}
	next, err := SaveBag(role.State, bag)
	if err == nil {
		_, err = protocol.InventoryUpdate(bag.Rows())
	}
	if err == nil && r.EquipmentSpace == 3 {
		_, err = WornSpaceUpdate(next)
	}
	return next, out, err
}
