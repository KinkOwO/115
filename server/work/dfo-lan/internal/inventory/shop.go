package inventory

import (
	"dfolan/internal/catalog"
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
// A declared stack limit wins. Items without one use the client's signed
// 32-bit maximum; MissingStackLimit is retained only for old policy files.
//
// 上限取值：wire 里的 amount 是无符号 u32，但客户端内部按有符号 int 消费，超过 2^31-1
// 有显示成负数的风险，所以"无限"类实际取 int32 上限。
func stackLimitFor(_ BagRules, _ string, explicit uint32) uint32 {
	if explicit > 0 {
		return explicit
	}
	return math.MaxInt32
}

// StackLimitForTemplate resolves a saved stack's limit at vault boundaries.
// Unknown templates retain the compatibility cap because their script cannot
// establish whether a larger amount is legal.
func StackLimitForTemplate(c catalog.LootCatalog, r BagRules, template uint32) uint32 {
	if item, ok := c.Items[template]; ok && item.Kind == "stackable" {
		return stackLimitFor(r, item.StackableType, item.StackLimit)
	}
	if r.MissingStackLimit != 0 {
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
	original := b

	occupied := map[uint16]bool{}
	for _, eq := range b.Equipment {
		occupied[eq.Slot] = true
	}

	// 1. Try stacking into an existing slot of the same template within the target range
	b.Items = append([]BagItem(nil), b.Items...)
	for i, it := range b.Items {
		occupied[it.Slot] = true
		if it.Template == template && it.Slot >= slots[0] && it.Slot <= slots[1] && it.Amount < limit {
			added := min(count, limit-it.Amount)
			b.Items[i].Amount += added
			count -= added
			if count == 0 {
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

	return original, 0, fmt.Errorf("bag category is full")
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

// PayMaterialsWithStore 与 PayMaterials 相同，但材料可以从**账号材料仓库**补足：
// 账号共享材料（3033..3037 等，见 accountMaterialSlotByTemplate）平时不在角色背包里，
// 每个模板先扣角色背包，背包不够的差额再扣账号仓库（space 35）。两处的总量都不足才拒绝。
//
// 商店里「用材料交换」的商品走的正是这条 —— 技能无色消耗早已能用账号仓库里的晶块，
// 商店以前只查背包，于是「包里有晶块但商店说 have 0」。
func PayMaterialsWithStore(b Bag, m AccountMaterials, materials []MaterialCost, multiplier uint32) (Bag, AccountMaterials, error) {
	if len(materials) == 0 {
		return b, m, nil
	}
	if multiplier == 0 {
		multiplier = 1
	}
	for _, mt := range materials {
		need := uint64(mt.Count) * uint64(multiplier)
		var have uint64
		for _, it := range b.Items {
			if it.Template == mt.Template {
				have += uint64(it.Amount)
			}
		}
		have += uint64(m.Count(mt.Template))
		if have < need {
			return b, m, fmt.Errorf("need %d of item %d to pay, have %d", need, mt.Template, have)
		}
	}
	b.Items = append([]BagItem(nil), b.Items...)
	for _, mt := range materials {
		need := uint64(mt.Count) * uint64(multiplier)
		for i := range b.Items {
			if need == 0 {
				break
			}
			if b.Items[i].Template != mt.Template {
				continue
			}
			take := uint64(b.Items[i].Amount)
			if take > need {
				take = need
			}
			b.Items[i].Amount -= uint32(take)
			need -= take
		}
		if need > 0 {
			next, _, e := m.Spend(mt.Template, uint32(need))
			if e != nil {
				return b, m, e
			}
			m = next
		}
	}
	kept := make([]BagItem, 0, len(b.Items))
	for _, it := range b.Items {
		if it.Amount > 0 {
			kept = append(kept, it)
		}
	}
	b.Items = kept
	return b, m, nil
}

// BuyWithMaterialsWithStore places a purchase paid with materials that may be
// drawn from the account-shared store as well as the character bag.
func (b Bag) BuyWithMaterialsWithStore(r BagRules, m AccountMaterials, template, count uint32, materials []MaterialCost, stackableType ...string) (Bag, AccountMaterials, uint16, error) {
	if template == 0 || count == 0 {
		return b, m, 0, fmt.Errorf("invalid buy parameters")
	}
	paid, store, err := PayMaterialsWithStore(b, m, materials, count)
	if err != nil {
		return b, m, 0, err
	}
	st := ""
	if len(stackableType) > 0 {
		st = stackableType[0]
	}
	out, slot, err := paid.addStackable(r, template, count, st)
	if err != nil {
		return b, m, 0, err
	}
	return out, store, slot, nil
}

// Sell sells an item from the bag by its slot and inventory list type.
// Dispatch rules:
// - equipment_slots [9,64] -> b.Equipment (unworn equipment in bag)
// - throw [65,120] / material [121,176] -> b.Items (stackables)
// - b.Worn (equipped gear) is rejected
// Slot overlap resolution: equipment slots 12-25 overlap with worn slots 12-25.
// Equipment range checks b.Equipment first; only if absent from b.Equipment does it check Worn.
func (b Bag) Sell(r BagRules, list byte, slot uint16, count, unitPrice uint32) (Bag, uint32, uint32, error) {
	if list != 0 {
		return b, 0, 0, fmt.Errorf("unsupported inventory list %d", list)
	}
	if slot == 0 || count == 0 || count > math.MaxInt32 {
		return b, 0, 0, fmt.Errorf("invalid sell slot or quantity")
	}

	total := uint64(count) * uint64(unitPrice)
	if total > math.MaxUint32 || uint64(b.Gold)+total > math.MaxUint32 {
		return b, 0, 0, fmt.Errorf("gold overflow")
	}
	goldGained := uint32(total)

	eqSlots := r.EquipmentSlots
	if eqSlots == [2]uint16{} {
		eqSlots = [2]uint16{9, 64}
	}

	// 1. If slot is within the equipment range [9, 64], check b.Equipment first.
	if slot >= eqSlots[0] && slot <= eqSlots[1] {
		for i, eq := range b.Equipment {
			if eq.Slot == slot {
				if count != 1 {
					return b, 0, 0, fmt.Errorf("equipment sale requires quantity one")
				}
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
			if count > it.Amount {
				return b, 0, 0, fmt.Errorf("sell quantity %d exceeds owned amount %d", count, it.Amount)
			}
			template := it.Template
			b.Items = append([]BagItem(nil), b.Items...)
			if it.Amount == count {
				b.Items = append(b.Items[:i], b.Items[i+1:]...)
			} else {
				b.Items[i].Amount -= count
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
