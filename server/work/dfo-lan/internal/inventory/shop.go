package inventory

import (
	"fmt"
	"math"
	"strings"
)

func normalizeStackableType(s string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "`", "")))
}

// stackableSlotRange maps a stackable type string to its bag slot range.
// If not specified or absent from rules, defaults to the throw/consumables range [65, 120].
func stackableSlotRange(r BagRules, stackableType string) [2]uint16 {
	rng, _ := classifyStackableSlot(r, stackableType)
	return rng
}

// classifyStackableSlot reports whether the range comes from a declared or
// built-in category. Unknown types use the consumables fallback, which is not
// enough evidence to move an existing saved item.
func classifyStackableSlot(r BagRules, stackableType string) ([2]uint16, bool) {
	norm := normalizeStackableType(stackableType)
	if rng, ok := r.Slots[norm]; ok && rng != [2]uint16{} {
		return rng, true
	}
	switch {
	case strings.HasPrefix(norm, "[material]") && strings.HasSuffix(norm, "4"):
		return [2]uint16{345, 359}, true
	case strings.HasPrefix(norm, "[material]"):
		return [2]uint16{121, 176}, true
	case strings.HasPrefix(norm, "[quest]"):
		return [2]uint16{177, 232}, true
	case strings.HasPrefix(norm, "[material expert job]"):
		return [2]uint16{233, 288}, true
	case strings.HasPrefix(norm, "[avatar emblem]"):
		return [2]uint16{289, 344}, true
	default:
		if rng, ok := r.Slots["[throw]"]; ok && rng != [2]uint16{} {
			return rng, false
		}
		return [2]uint16{65, 120}, false
	}
}

// StackableSlotRange exposes the category slot mapping to the package and
// reward delivery paths outside this package, so a pool item with an unusual
// type cannot strand a whole open on a missing rule entry.
func StackableSlotRange(r BagRules, stackableType string) [2]uint16 {
	return stackableSlotRange(r, stackableType)
}

// Buy adds an item purchased from an NPC shop into the bag and charges gold.
// NPC shop items (templates 1-3175) are catalog-independent: if stackableType is
// unspecified or not in catalog, they default to the throw range [65, 120] (consumables).
//
// Shops whose source prices a good with [need material] do not come through
// here — see PayMaterials.
func (b Bag) Buy(r BagRules, template, count, cost uint32, stackableType ...string) (Bag, uint16, error) {
	if template == 0 || count == 0 {
		return b, 0, fmt.Errorf("invalid buy parameters")
	}
	if b.Gold < cost {
		return b, 0, fmt.Errorf("insufficient gold: need %d, have %d", cost, b.Gold)
	}
	st := ""
	if len(stackableType) > 0 {
		st = stackableType[0]
	}
	next, slot, err := b.addStackable(r, template, count, st)
	if err != nil {
		return b, 0, err
	}
	next.Gold -= cost
	return next, slot, nil
}

// stackLimitFor 给出某物品的堆叠上限。
//
// 源里"无限"类物品通常**不写** [stackable limit]：银币/金币（10418036/10418035）的 .stk
// 只有 [stackable type] `[unlimited waste]`，靠类型名声明"无限"。服务端过去把"没写上限"
// 一律兜成 missing_stack_limit（默认 1000），于是 GM 发 1000 银币会和手里的 23 个分成
// 两叠（实机 2026-09-23 观察）。这里按源语义处理：显式上限优先，其次认 unlimited 类型，
// 最后才用 bag 规则的 missing_stack_limit。
//
// 上限取值：wire 里的 amount 是无符号 u32，但客户端内部按有符号 int 消费，超过 2^31-1
// 有显示成负数的风险，所以"无限"类实际取 int32 上限。
func stackLimitFor(r BagRules, stackableType string, explicit uint32) uint32 {
	if explicit > 0 {
		return explicit
	}
	if strings.Contains(normalizeStackableType(stackableType), "unlimited") {
		return math.MaxInt32
	}
	if r.MissingStackLimit > 0 {
		return r.MissingStackLimit
	}
	return 1000
}

// addStackable places count of template into the bag's category range without
// charging anything: stack onto a same-template row first, else take the first
// free slot in the range.
func (b Bag) addStackable(r BagRules, template, count uint32, stackableType string) (Bag, uint16, error) {
	slots := stackableSlotRange(r, stackableType)
	limit := stackLimitFor(r, stackableType, 0)
	if count > limit {
		return b, 0, fmt.Errorf("buy count exceeds stack limit")
	}

	occupied := map[uint16]bool{}
	for _, eq := range b.Equipment {
		occupied[eq.Slot] = true
	}

	// 1. Try stacking into an existing slot of the same template within the target range
	b.Items = append([]BagItem(nil), b.Items...)
	for i, it := range b.Items {
		occupied[it.Slot] = true
		if it.Template == template && it.Slot >= slots[0] && it.Slot <= slots[1] {
			if uint64(it.Amount)+uint64(count) <= uint64(limit) {
				b.Items[i].Amount += count
				return b, it.Slot, nil
			}
		}
	}

	// 2. Allocate the first unoccupied slot in the category range
	for n := uint32(slots[0]); n <= uint32(slots[1]); n++ {
		slot := uint16(n)
		if !occupied[slot] {
			b.Items = append(b.Items, BagItem{Slot: slot, Template: template, Amount: count})
			return b, slot, nil
		}
	}

	return b, 0, fmt.Errorf("bag category is full")
}

// MaterialCost is one unit of "pay with items": Count of Template per purchase.
type MaterialCost struct {
	Template uint32
	Count    uint32
}

// PayMaterials charges a purchase whose source prices it with [need material]
// (the Odyssey shop asks for silver/gold coins, templates 10418036/10418035).
//
// The whole bill is checked before anything is deducted, so a purchase either
// pays in full or is refused — no half-paid state. Rows that reach zero are
// dropped, exactly like any other consumption.
func (b Bag) PayMaterials(materials []MaterialCost, multiplier uint32) (Bag, error) {
	if len(materials) == 0 {
		return b, nil
	}
	if multiplier == 0 {
		multiplier = 1
	}
	for _, m := range materials {
		need := uint64(m.Count) * uint64(multiplier)
		var have uint64
		for _, it := range b.Items {
			if it.Template == m.Template {
				have += uint64(it.Amount)
			}
		}
		if have < need {
			return b, fmt.Errorf("need %d of item %d to pay, have %d", need, m.Template, have)
		}
	}
	b.Items = append([]BagItem(nil), b.Items...)
	for _, m := range materials {
		need := uint64(m.Count) * uint64(multiplier)
		for i := range b.Items {
			if need == 0 {
				break
			}
			if b.Items[i].Template != m.Template {
				continue
			}
			take := uint64(b.Items[i].Amount)
			if take > need {
				take = need
			}
			b.Items[i].Amount -= uint32(take)
			need -= take
		}
	}
	kept := make([]BagItem, 0, len(b.Items))
	for _, it := range b.Items {
		if it.Amount > 0 {
			kept = append(kept, it)
		}
	}
	b.Items = kept
	return b, nil
}

// BuyWithMaterials places a purchase paid for with materials instead of gold.
func (b Bag) BuyWithMaterials(r BagRules, template, count uint32, materials []MaterialCost, stackableType ...string) (Bag, uint16, error) {
	if template == 0 || count == 0 {
		return b, 0, fmt.Errorf("invalid buy parameters")
	}
	paid, err := b.PayMaterials(materials, count)
	if err != nil {
		return b, 0, err
	}
	st := ""
	if len(stackableType) > 0 {
		st = stackableType[0]
	}
	return paid.addStackable(r, template, count, st)
}

// Sell sells an item from the bag by its slot and inventory list type.
// Dispatch rules:
// - equipment_slots [9,64] -> b.Equipment (unworn equipment in bag)
// - throw [65,120] / material [121,176] -> b.Items (stackables)
// - b.Worn (equipped gear) is rejected
// Slot overlap resolution: equipment slots 12-25 overlap with worn slots 12-25.
// Equipment range checks b.Equipment first; only if absent from b.Equipment does it check Worn.
func (b Bag) Sell(r BagRules, list byte, slot uint16, unitPrice uint32) (Bag, uint32, uint32, error) {
	if list != 0 {
		return b, 0, 0, fmt.Errorf("unsupported inventory list %d", list)
	}
	if slot == 0 {
		return b, 0, 0, fmt.Errorf("invalid sell slot")
	}

	goldGained := unitPrice
	if goldGained == 0 {
		goldGained = 1
	}
	if uint64(b.Gold)+uint64(goldGained) > math.MaxUint32 {
		return b, 0, 0, fmt.Errorf("gold overflow")
	}

	eqSlots := r.EquipmentSlots
	if eqSlots == [2]uint16{} {
		eqSlots = [2]uint16{9, 64}
	}

	// 1. If slot is within the equipment range [9, 64], check b.Equipment first.
	if slot >= eqSlots[0] && slot <= eqSlots[1] {
		for i, eq := range b.Equipment {
			if eq.Slot == slot {
				template := eq.Template
				b.Equipment = append([]BagEquipment(nil), b.Equipment...)
				b.Equipment = append(b.Equipment[:i], b.Equipment[i+1:]...)
				b.Gold += goldGained
				return b, template, goldGained, nil
			}
		}
		// If not in b.Equipment, check if worn at slot 12-25
		for _, w := range b.Worn {
			if w.Slot == slot {
				return b, 0, 0, fmt.Errorf("cannot sell worn equipment at slot %d", slot)
			}
		}
	}

	// 2. Check stackable items in b.Items
	for i, it := range b.Items {
		if it.Slot == slot {
			template := it.Template
			b.Items = append([]BagItem(nil), b.Items...)
			if it.Amount <= 1 {
				b.Items = append(b.Items[:i], b.Items[i+1:]...)
			} else {
				b.Items[i].Amount--
			}
			b.Gold += goldGained
			return b, template, goldGained, nil
		}
	}

	// 3. Fallback check for worn slots
	for _, w := range b.Worn {
		if w.Slot == slot {
			return b, 0, 0, fmt.Errorf("cannot sell worn equipment at slot %d", slot)
		}
	}

	return b, 0, 0, fmt.Errorf("no such owned bag slot %d", slot)
}
