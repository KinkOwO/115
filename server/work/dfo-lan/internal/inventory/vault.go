package inventory

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type VaultRules struct {
	SourceSHA256  string   `json:"source_sha256"`
	InitialSlots  uint16   `json:"initial_slots"`
	VerifiedSlots []uint16 `json:"verified_slots"`
}

func LoadVaultRules(path string) (VaultRules, error) {
	var r VaultRules
	b, e := os.ReadFile(path)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	if len(r.SourceSHA256) != 64 || !r.allows(r.InitialSlots) {
		return r, fmt.Errorf("invalid source vault configuration")
	}
	return r, nil
}

func (r VaultRules) allows(n uint16) bool {
	for _, v := range r.VerifiedSlots {
		if n != 0 && n == v {
			return true
		}
	}
	return false
}

type VaultItem struct {
	Slot       uint16 `json:"slot"`
	Template   uint32 `json:"template"`
	Amount     uint32 `json:"amount,omitempty"`
	Durability uint16 `json:"durability,omitempty"`
	IsEquip    bool   `json:"is_equip,omitempty"`
}

func (v VaultItem) Row() [protocol.CurrentItemRecordSize]byte {
	if v.IsEquip {
		r := protocol.OrdinaryItem(v.Slot, v.Template, 0)
		binary.LittleEndian.PutUint16(r[11:], v.Durability)
		return r
	}
	return protocol.OrdinaryItem(v.Slot, v.Template, v.Amount)
}

type Vault struct {
	Slots uint16      `json:"slots"`
	Items []VaultItem `json:"items"`
}

func ReadVault(v storage.VaultState) (Vault, error) {
	var vault Vault
	vault.Slots = v.Slots
	if len(v.Items) == 0 || string(v.Items) == "[]" {
		vault.Items = nil
		return vault, nil
	}
	if err := json.Unmarshal(v.Items, &vault.Items); err != nil {
		return vault, err
	}
	return vault, nil
}

func SaveVault(v Vault) (json.RawMessage, error) {
	if len(v.Items) == 0 {
		return json.RawMessage("[]"), nil
	}
	sort.Slice(v.Items, func(i, j int) bool {
		return v.Items[i].Slot < v.Items[j].Slot
	})
	b, err := json.Marshal(v.Items)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func (v Vault) Rows() [][protocol.CurrentItemRecordSize]byte {
	items := append([]VaultItem(nil), v.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].Slot < items[j].Slot })
	var rows [][protocol.CurrentItemRecordSize]byte
	for _, it := range items {
		rows = append(rows, it.Row())
	}
	return rows
}

func (v Vault) ItemAt(slot uint16) *VaultItem {
	for i := range v.Items {
		if v.Items[i].Slot == slot {
			return &v.Items[i]
		}
	}
	return nil
}

type VaultService struct {
	Store *storage.Store
	Rules VaultRules
}

func (s *VaultService) Bootstrap(ctx context.Context, role storage.Character) ([]byte, error) {
	v, e := s.Store.LoadVault(ctx, role.AccountID, role.ID, s.Rules.InitialSlots, s.Rules.SourceSHA256)
	if e != nil {
		return nil, e
	}
	if v.ConfigVersion != s.Rules.SourceSHA256 || !s.Rules.allows(v.Slots) {
		return nil, fmt.Errorf("vault requires configuration migration")
	}
	vault, e := ReadVault(v)
	if e != nil {
		return nil, e
	}
	return protocol.PersonalVaultRestore(vault.Slots, vault.Rows())
}

// MoveVaultItem handles item transfer between Bag and Vault, or within Vault.
func MoveVaultItem(b Bag, v Vault, rules BagRules, r protocol.ItemMoveRequest) (Bag, Vault, uint32, error) {
	if r.SourceList != 2 && r.DestinationList != 2 {
		return b, v, 0, fmt.Errorf("not a vault move operation")
	}

	// Case 1: Bag -> Vault
	if r.SourceList == 0 && r.DestinationList == 2 {
		if r.DestinationSlot >= v.Slots {
			return b, v, 0, fmt.Errorf("target vault slot %d exceeds capacity %d", r.DestinationSlot, v.Slots)
		}

		var srcEquip *BagEquipment
		srcEquipIdx := -1
		for i := range b.Equipment {
			if b.Equipment[i].Slot == r.SourceSlot {
				srcEquip = &b.Equipment[i]
				srcEquipIdx = i
				break
			}
		}
		if srcEquip != nil {
			if v.ItemAt(r.DestinationSlot) != nil {
				return b, v, 0, fmt.Errorf("target vault slot %d is already occupied", r.DestinationSlot)
			}
			v.Items = append(v.Items, VaultItem{
				Slot:       r.DestinationSlot,
				Template:   srcEquip.Template,
				Durability: srcEquip.Durability,
				IsEquip:    true,
			})
			b.Equipment = append(b.Equipment[:srcEquipIdx], b.Equipment[srcEquipIdx+1:]...)
			return b, v, 1, nil
		}

		var srcItem *BagItem
		srcItemIdx := -1
		for i := range b.Items {
			if b.Items[i].Slot == r.SourceSlot {
				srcItem = &b.Items[i]
				srcItemIdx = i
				break
			}
		}
		if srcItem == nil {
			return b, v, 0, fmt.Errorf("source item not found in bag at slot %d", r.SourceSlot)
		}

		count := r.Count
		if count == 0 || count > srcItem.Amount {
			count = srcItem.Amount
		}

		if destItem := v.ItemAt(r.DestinationSlot); destItem != nil {
			if destItem.IsEquip || destItem.Template != srcItem.Template {
				return b, v, 0, fmt.Errorf("target vault slot %d occupied by different item", r.DestinationSlot)
			}
			limit := rules.MissingStackLimit
			if limit == 0 {
				limit = 1000
			}
			spaceLeft := limit - destItem.Amount
			if spaceLeft == 0 {
				return b, v, 0, fmt.Errorf("target vault stack is full")
			}
			toMove := count
			if toMove > spaceLeft {
				toMove = spaceLeft
			}
			destItem.Amount += toMove
			srcItem.Amount -= toMove
			if srcItem.Amount == 0 {
				b.Items = append(b.Items[:srcItemIdx], b.Items[srcItemIdx+1:]...)
			}
			return b, v, toMove, nil
		}

		v.Items = append(v.Items, VaultItem{
			Slot:     r.DestinationSlot,
			Template: srcItem.Template,
			Amount:   count,
			IsEquip:  false,
		})
		srcItem.Amount -= count
		if srcItem.Amount == 0 {
			b.Items = append(b.Items[:srcItemIdx], b.Items[srcItemIdx+1:]...)
		}
		return b, v, count, nil
	}

	// Case 2: Vault -> Bag
	if r.SourceList == 2 && r.DestinationList == 0 {
		if r.SourceSlot >= v.Slots {
			return b, v, 0, fmt.Errorf("source vault slot %d exceeds capacity %d", r.SourceSlot, v.Slots)
		}
		srcIdx := -1
		for i := range v.Items {
			if v.Items[i].Slot == r.SourceSlot {
				srcIdx = i
				break
			}
		}
		if srcIdx == -1 {
			return b, v, 0, fmt.Errorf("source item not found in vault at slot %d", r.SourceSlot)
		}
		srcItem := &v.Items[srcIdx]

		if srcItem.IsEquip {
			if rules.EquipmentSlots != [2]uint16{} && (r.DestinationSlot < rules.EquipmentSlots[0] || r.DestinationSlot > rules.EquipmentSlots[1]) {
				return b, v, 0, fmt.Errorf("target slot %d outside equipment bag range", r.DestinationSlot)
			}
			for _, it := range b.Equipment {
				if it.Slot == r.DestinationSlot {
					return b, v, 0, fmt.Errorf("target bag slot %d occupied", r.DestinationSlot)
				}
			}
			for _, it := range b.Items {
				if it.Slot == r.DestinationSlot {
					return b, v, 0, fmt.Errorf("target bag slot %d occupied", r.DestinationSlot)
				}
			}
			b.Equipment = append(b.Equipment, BagEquipment{
				Slot:       r.DestinationSlot,
				Template:   srcItem.Template,
				Durability: srcItem.Durability,
			})
			v.Items = append(v.Items[:srcIdx], v.Items[srcIdx+1:]...)
			return b, v, 1, nil
		}

		count := r.Count
		if count == 0 || count > srcItem.Amount {
			count = srcItem.Amount
		}

		var targetBagItem *BagItem
		for i := range b.Items {
			if b.Items[i].Slot == r.DestinationSlot {
				targetBagItem = &b.Items[i]
				break
			}
		}
		for _, it := range b.Equipment {
			if it.Slot == r.DestinationSlot {
				return b, v, 0, fmt.Errorf("target bag slot %d occupied by equipment", r.DestinationSlot)
			}
		}

		limit := rules.MissingStackLimit
		if limit == 0 {
			limit = 1000
		}

		if targetBagItem != nil {
			if targetBagItem.Template != srcItem.Template {
				return b, v, 0, fmt.Errorf("target bag slot %d occupied by different item", r.DestinationSlot)
			}
			spaceLeft := limit - targetBagItem.Amount
			if spaceLeft == 0 {
				return b, v, 0, fmt.Errorf("target bag stack is full")
			}
			toMove := count
			if toMove > spaceLeft {
				toMove = spaceLeft
			}
			targetBagItem.Amount += toMove
			srcItem.Amount -= toMove
			if srcItem.Amount == 0 {
				v.Items = append(v.Items[:srcIdx], v.Items[srcIdx+1:]...)
			}
			return b, v, toMove, nil
		}

		b.Items = append(b.Items, BagItem{
			Slot:     r.DestinationSlot,
			Template: srcItem.Template,
			Amount:   count,
		})
		srcItem.Amount -= count
		if srcItem.Amount == 0 {
			v.Items = append(v.Items[:srcIdx], v.Items[srcIdx+1:]...)
		}
		return b, v, count, nil
	}

	// Case 3: Vault -> Vault
	if r.SourceList == 2 && r.DestinationList == 2 {
		if r.SourceSlot >= v.Slots || r.DestinationSlot >= v.Slots {
			return b, v, 0, fmt.Errorf("vault slot out of bounds (source=%d dest=%d max=%d)", r.SourceSlot, r.DestinationSlot, v.Slots)
		}
		if r.SourceSlot == r.DestinationSlot {
			return b, v, 0, nil
		}
		srcIdx := -1
		for i := range v.Items {
			if v.Items[i].Slot == r.SourceSlot {
				srcIdx = i
				break
			}
		}
		if srcIdx == -1 {
			return b, v, 0, fmt.Errorf("source item not found in vault at slot %d", r.SourceSlot)
		}
		srcItem := &v.Items[srcIdx]

		destItem := v.ItemAt(r.DestinationSlot)
		if destItem == nil {
			srcItem.Slot = r.DestinationSlot
			moved := srcItem.Amount
			if srcItem.IsEquip {
				moved = 1
			}
			return b, v, moved, nil
		}

		if !srcItem.IsEquip && !destItem.IsEquip && srcItem.Template == destItem.Template {
			limit := rules.MissingStackLimit
			if limit == 0 {
				limit = 1000
			}
			spaceLeft := limit - destItem.Amount
			count := r.Count
			if count == 0 || count > srcItem.Amount {
				count = srcItem.Amount
			}
			if count > spaceLeft {
				count = spaceLeft
			}
			destItem.Amount += count
			srcItem.Amount -= count
			if srcItem.Amount == 0 {
				v.Items = append(v.Items[:srcIdx], v.Items[srcIdx+1:]...)
			}
			return b, v, count, nil
		}

		srcItem.Slot = r.DestinationSlot
		destItem.Slot = r.SourceSlot
		return b, v, 1, nil
	}

	return b, v, 0, fmt.Errorf("unsupported vault move combination: source=%d dest=%d", r.SourceList, r.DestinationList)
}
