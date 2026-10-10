package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const PetConsumableFirst uint16 = 376
const PetConsumableLast uint16 = 431
const PetGearFirst uint16 = 320
const PetGearLast uint16 = 375

func IsPetGear(kind string) bool {
	switch kind {
	case "[artifact red]", "[artifact blue]", "[artifact green]":
		return true
	}
	return false
}

func IsPetFeed(stackableType string) bool {
	return strings.HasPrefix(normalizeStackableType(stackableType), "[feed]")
}

func (b Bag) AddPetGear(item BagEquipment) (Bag, uint16, error) {
	occupied := make(map[uint16]bool, len(b.Special[7])+len(b.PetItems))
	for _, row := range b.Special[7] {
		occupied[row.Slot] = true
	}
	for _, row := range b.PetItems {
		occupied[row.Slot] = true
	}
	for slot := PetGearFirst; slot <= PetGearLast; slot++ {
		if occupied[slot] {
			continue
		}
		next := b
		next.Special = make(map[byte][]BagEquipment, len(b.Special)+1)
		for space, rows := range b.Special {
			next.Special[space] = rows
		}
		item.Slot = slot
		next.Special[7] = append(append([]BagEquipment(nil), b.Special[7]...), item)
		return next, slot, nil
	}
	return b, 0, fmt.Errorf("pet equipment container is full")
}

// PetCreatureFirst/Last 是**宠物本体**在宠物容器（list 7）里的槽位区间。
//
// 依据：NOTI105 的生物列表只收录 slot < 140 的行（internal/game/protocol/creature_list.go 的 reader），
// 而既有两条写本体的路径也都扫 0..139（internal/cashshop/pilot.go 的 deliverAmount、
// cmd/wireprobe/booster_flow.go 的开箱）。宠物**装备**在 320..375、宠物**用品**在 376..431，
// 三段不能混用（bag.go 的校验会因槽号相撞而拒整份存档）。
const (
	PetCreatureFirst uint16 = 0
	PetCreatureLast  uint16 = 139
)

// AddPetCreature 把一只**宠物本体**放进宠物容器，返回落到的槽位。
//
// 与 AddPetGear 同一套写法（copy-on-write：先复制 Special 再追加，不动调用方的 Bag）。
//
// 三条刻意的留白（照既有本体路径的口径，别"顺手补上"）：
//   - **不写 Record**：实例 key 由槽位推导（key = 槽位+2，客户端 reader 与
//     equipment_record.go 两侧一致）；塞一个自造的 Record 会因 Record[2:6] != 模板而对不上。
//   - **不写 Durability/Period**：本体的期限要按脚本 [usable period] 现算（显示成"剩余 N 天"），
//     这里写 GrantExpireTime 会让它显示 24856 天。
//   - **不合并**：本体是"一只一行"，不像堆叠物那样并堆（同一只宠物可以有多行）。
func (b Bag) AddPetCreature(template uint32) (Bag, uint16, error) {
	if template == 0 {
		return b, 0, fmt.Errorf("pet creature template is required")
	}
	occupied := make(map[uint16]bool, len(b.Special[7]))
	for _, row := range b.Special[7] {
		if row.Slot >= PetCreatureFirst && row.Slot <= PetCreatureLast {
			occupied[row.Slot] = true
		}
	}
	for slot := PetCreatureFirst; slot <= PetCreatureLast; slot++ {
		if occupied[slot] {
			continue
		}
		next := b
		next.Special = make(map[byte][]BagEquipment, len(b.Special)+1)
		for space, rows := range b.Special {
			next.Special[space] = rows
		}
		next.Special[7] = append(append([]BagEquipment(nil), b.Special[7]...), BagEquipment{Slot: slot, Template: template})
		return next, slot, nil
	}
	return b, 0, fmt.Errorf("creature inventory is full")
}

// SweepPetGear relocates old rewards without changing any item instance.
func SweepPetGear(b Bag, equipment *EquipmentCatalog) (Bag, bool, error) {
	if equipment == nil {
		return b, false, fmt.Errorf("pet equipment catalog unavailable")
	}
	next := b
	next.Equipment = append([]BagEquipment(nil), b.Equipment...)
	moved := false
	for i := 0; i < len(next.Equipment); {
		item := next.Equipment[i]
		kind, err := equipment.EquipmentKind(item.Template)
		if err != nil || !IsPetGear(kind) {
			i++
			continue
		}
		placed, _, err := next.AddPetGear(item)
		if err != nil {
			return b, false, err
		}
		next = placed
		next.Equipment = append(next.Equipment[:i], next.Equipment[i+1:]...)
		moved = true
	}
	rows := append([]BagEquipment(nil), next.Special[7]...)
	for _, item := range rows {
		if item.Slot >= PetGearFirst && item.Slot <= PetGearLast {
			continue
		}
		kind, err := equipment.EquipmentKind(item.Template)
		if err != nil || !IsPetGear(kind) {
			continue
		}
		// Remove the old slot before finding a destination.
		kept := make([]BagEquipment, 0, len(next.Special[7])-1)
		for _, row := range next.Special[7] {
			if row.Slot != item.Slot {
				kept = append(kept, row)
			}
		}
		next.Special[7] = kept
		placed, _, err := next.AddPetGear(item)
		if err != nil {
			return b, false, err
		}
		next = placed
		moved = true
	}
	return next, moved, nil
}

func IsPetConsumable(stackableType string) bool {
	t := normalizeStackableType(stackableType)
	return strings.HasPrefix(t, "[feed]") || strings.HasPrefix(t, "[creature]")
}

func (b Bag) addPetStack(r BagRules, template, amount, expire, explicit uint32) (Bag, uint16, error) {
	limit := stackLimitFor(r, "", explicit)
	if amount == 0 || amount > limit {
		return b, 0, fmt.Errorf("invalid pet consumable amount")
	}
	original := b
	b.PetItems = append([]BagItem(nil), b.PetItems...)
	occupied := map[uint16]bool{}
	for _, row := range b.Special[7] {
		occupied[row.Slot] = true
	}
	for i, row := range b.PetItems {
		occupied[row.Slot] = true
		if row.Template != template || row.ExpireTime != expire || row.Amount >= limit {
			continue
		}
		added := min(amount, limit-row.Amount)
		b.PetItems[i].Amount += added
		amount -= added
		if amount == 0 {
			return b, row.Slot, nil
		}
	}
	for slot := PetConsumableFirst; slot <= PetConsumableLast; slot++ {
		if occupied[slot] {
			continue
		}
		b.PetItems = append(b.PetItems, BagItem{Slot: slot, Template: template, Amount: amount, ExpireTime: expire})
		return b, slot, nil
	}
	return original, 0, fmt.Errorf("pet consumable container is full")
}

// SweepPetConsumables preserves old character saves while relocating pet
// consumables that were formerly awarded into the ordinary bag.
func SweepPetConsumables(b Bag, c catalog.LootCatalog, r BagRules) (Bag, bool, error) {
	next := b
	next.Items = append([]BagItem(nil), b.Items...)
	moved := false
	for i := 0; i < len(next.Items); {
		row := next.Items[i]
		definition, ok := c.Items[row.Template]
		if !ok || definition.Kind != "stackable" || !IsPetConsumable(definition.StackableType) {
			i++
			continue
		}
		next.Items = append(next.Items[:i], next.Items[i+1:]...)
		placed, _, err := next.addPetStack(r, row.Template, row.Amount, row.ExpireTime, definition.StackLimit)
		if err != nil {
			return b, false, err
		}
		next = placed
		moved = true
	}
	return next, moved, nil
}

// List 7 contains both equipment rows and ordinary pet consumable rows.
func PetContainerBody(b Bag, restore bool) ([]byte, error) {
	body, err := EquipmentPayload(7, b.Special[7], restore)
	if err != nil {
		return nil, err
	}
	items := append([]BagItem(nil), b.PetItems...)
	sort.Slice(items, func(i, j int) bool { return items[i].Slot < items[j].Slot })
	if len(items)+len(b.Special[7]) > 65535 {
		return nil, fmt.Errorf("too many pet container rows")
	}
	binary.LittleEndian.PutUint16(body[1:3], uint16(len(items)+len(b.Special[7])))
	for _, item := range items {
		row := protocol.OrdinaryItem(item.Slot, item.Template, item.Amount, item.ExpireTime)
		body = append(body, row[:]...)
	}
	return body, nil
}

func PetContainerRestorePayload(state json.RawMessage) ([]byte, error) {
	b, err := ReadBag(state)
	if err != nil {
		return nil, err
	}
	return PetContainerBody(b, true)
}
