package inventory

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

type VaultRules struct {
	SourceSHA256  string             `json:"source_sha256"`
	InitialSlots  uint16             `json:"initial_slots"`
	VerifiedSlots []uint16           `json:"verified_slots"`
	Account       *AccountVaultRules `json:"account,omitempty"`
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
	if r.Account != nil {
		if e = r.Account.Validate(); e != nil {
			return r, e
		}
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
	Slot       uint16        `json:"slot"`
	Template   uint32        `json:"template"`
	Amount     uint32        `json:"amount,omitempty"`
	ExpireTime uint32        `json:"expire_time,omitempty"`
	Durability uint16        `json:"durability,omitempty"`
	IsEquip    bool          `json:"is_equip,omitempty"`
	Equipment  *BagEquipment `json:"equipment,omitempty"`
}

func (v VaultItem) Row() [protocol.CurrentItemRecordSize]byte {
	if v.IsEquip {
		if v.Equipment != nil {
			item := *v.Equipment
			item.Slot = v.Slot
			return EquipmentRow(item)
		}
		r := protocol.OrdinaryItem(v.Slot, v.Template, 0)
		binary.LittleEndian.PutUint16(r[11:], v.Durability)
		return r
	}
	return protocol.OrdinaryItem(v.Slot, v.Template, v.Amount, v.ExpireTime)
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

func ReadExtendedVault(v storage.VaultState) (Vault, error) {
	var vault Vault
	vault.Slots = v.Slots
	d := json.NewDecoder(bytes.NewReader(v.Items))
	d.DisallowUnknownFields()
	if err := d.Decode(&vault.Items); err != nil {
		return vault, err
	}
	var tail any
	if err := d.Decode(&tail); err != io.EOF || vault.Items == nil || v.Slots == 0 {
		return vault, fmt.Errorf("invalid vault array or trailing data")
	}
	seen := map[uint16]bool{}
	for _, item := range vault.Items {
		if item.Slot >= v.Slots || item.Template == 0 || seen[item.Slot] || (!item.IsEquip && (item.Amount == 0 || item.Equipment != nil || item.Durability != 0)) {
			return vault, fmt.Errorf("invalid vault item")
		}
		if item.Equipment != nil && (item.Equipment.Template != item.Template || item.Equipment.ValidateRecord() != nil) {
			return vault, fmt.Errorf("invalid vault equipment metadata")
		}
		seen[item.Slot] = true
	}
	return vault, nil
}

func ReadVaultBagItems(v storage.VaultState) ([]BagItem, error) {
	var items []BagItem
	d := json.NewDecoder(bytes.NewReader(v.Items))
	d.DisallowUnknownFields()
	if e := d.Decode(&items); e != nil {
		return nil, e
	}
	var tail any
	if e := d.Decode(&tail); e != io.EOF {
		return nil, fmt.Errorf("trailing vault data")
	}
	if items == nil || v.Slots == 0 {
		return nil, fmt.Errorf("invalid vault array/capacity")
	}
	seen := map[uint16]bool{}
	for _, i := range items {
		if i.Slot >= v.Slots || i.Template == 0 || i.Amount == 0 || seen[i.Slot] {
			return nil, fmt.Errorf("invalid vault item")
		}
		seen[i.Slot] = true
	}
	return items, nil
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
	Store    *storage.Store
	Rules    VaultRules
	Catalog  catalog.LootCatalog
	BagRules BagRules
}

func (s *VaultService) Bootstrap(ctx context.Context, role storage.Character) ([]byte, error) {
	return s.BootstrapSpace(ctx, role, 2)
}

// 两个个人金库各自初始化，使用同一容量档位规则，不复制另一金库的物品或升级。
func (s *VaultService) BootstrapSpace(ctx context.Context, role storage.Character, space byte) ([]byte, error) {
	v, e := s.Store.LoadVault(ctx, role.AccountID, role.ID, s.Rules.InitialSlots, s.Rules.SourceSHA256, space)
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
	return protocol.PersonalVaultSpace(space, vault.Slots, vault.Rows())
}

func VaultPayload(v storage.VaultState, space ...byte) ([]byte, error) {
	list := byte(2)
	if len(space) > 1 {
		return nil, fmt.Errorf("个人金库快照容器参数重复")
	}
	if len(space) == 1 {
		list = space[0]
	}
	items, e := ReadExtendedVault(v)
	if e != nil {
		return nil, e
	}
	return protocol.PersonalVaultSpace(list, v.Slots, items.Rows())
}

// MoveVaultItem handles item transfer between Bag and Vault, or within Vault.
func MoveVaultItem(b Bag, v Vault, rules BagRules, r protocol.ItemMoveRequest) (Bag, Vault, uint32, error) {
	if r.SourceList != 2 && r.DestinationList != 2 {
		return b, v, 0, fmt.Errorf("not a vault move operation")
	}
	// 保留调用者的旧快照，供失败回滚与所有受影响槽位的增量比较使用。
	b.Items = append([]BagItem(nil), b.Items...)
	b.Equipment = append([]BagEquipment(nil), b.Equipment...)
	v.Items = append([]VaultItem(nil), v.Items...)

	// Case 1: Bag -> Vault
	if r.SourceList == 0 && r.DestinationList == 2 {
		if r.DestinationSlot >= v.Slots {
			return b, v, 0, fmt.Errorf("target vault slot %d exceeds capacity %d", r.DestinationSlot, v.Slots)
		}

		// 装备处理分支
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
			instance := *srcEquip
			targetSlot := r.DestinationSlot
			if v.ItemAt(targetSlot) != nil {
				found := false
				for s := uint16(0); s < v.Slots; s++ {
					if v.ItemAt(s) == nil {
						targetSlot = s
						found = true
						break
					}
				}
				if !found {
					return b, v, 0, fmt.Errorf("target vault slot %d is already occupied", r.DestinationSlot)
				}
			}
			v.Items = append(v.Items, VaultItem{
				Slot:       targetSlot,
				Template:   srcEquip.Template,
				Durability: srcEquip.Durability,
				IsEquip:    true,
				Equipment:  &instance,
			})
			b.Equipment = append(b.Equipment[:srcEquipIdx], b.Equipment[srcEquipIdx+1:]...)
			return b, v, 1, nil
		}

		// 可堆叠物品处理分支
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

		toMoveTotal := r.Count
		if toMoveTotal == 0 || toMoveTotal > srcItem.Amount {
			toMoveTotal = srcItem.Amount
		}
		limit := rules.MissingStackLimit
		if limit == 0 {
			limit = 1000
		}

		remainingToMove := toMoveTotal

		// 步骤 A：优先合并到客户端指定的 DestinationSlot（如果是相同物品且未满）
		if destItem := v.ItemAt(r.DestinationSlot); destItem != nil && !destItem.IsEquip && destItem.Template == srcItem.Template && destItem.ExpireTime == srcItem.ExpireTime && destItem.Amount < limit {
			space := limit - destItem.Amount
			n := remainingToMove
			if n > space {
				n = space
			}
			destItem.Amount += n
			remainingToMove -= n
		}

		// 步骤 B：自动合并到金库中其它已有相同物品的槽位（按 slot 升序）
		if remainingToMove > 0 {
			for i := range v.Items {
				if v.Items[i].Slot == r.DestinationSlot {
					continue
				}
				if !v.Items[i].IsEquip && v.Items[i].Template == srcItem.Template && v.Items[i].ExpireTime == srcItem.ExpireTime && v.Items[i].Amount < limit {
					space := limit - v.Items[i].Amount
					n := remainingToMove
					if n > space {
						n = space
					}
					v.Items[i].Amount += n
					remainingToMove -= n
					if remainingToMove == 0 {
						break
					}
				}
			}
		}

		// 步骤 C：所有已有同类堆叠都已填满（或原本没有同类物品），若仍有剩余数量，放入空格子
		if remainingToMove > 0 {
			targetSlot := r.DestinationSlot
			if v.ItemAt(targetSlot) != nil {
				found := false
				for s := uint16(0); s < v.Slots; s++ {
					if v.ItemAt(s) == nil {
						targetSlot = s
						found = true
						break
					}
				}
				if !found {
					actualMoved := toMoveTotal - remainingToMove
					if actualMoved == 0 {
						return b, v, 0, fmt.Errorf("target vault is full")
					}
					srcItem.Amount -= actualMoved
					if srcItem.Amount == 0 {
						b.Items = append(b.Items[:srcItemIdx], b.Items[srcItemIdx+1:]...)
					}
					return b, v, actualMoved, nil
				}
			}

			v.Items = append(v.Items, VaultItem{
				Slot:       targetSlot,
				Template:   srcItem.Template,
				Amount:     remainingToMove,
				ExpireTime: srcItem.ExpireTime,
				IsEquip:    false,
			})
			remainingToMove = 0
		}

		actualMoved := toMoveTotal - remainingToMove
		srcItem.Amount -= actualMoved
		if srcItem.Amount == 0 {
			b.Items = append(b.Items[:srcItemIdx], b.Items[srcItemIdx+1:]...)
		}
		return b, v, actualMoved, nil
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
			instance := BagEquipment{
				Slot:       r.DestinationSlot,
				Template:   srcItem.Template,
				Durability: srcItem.Durability,
			}
			if srcItem.Equipment != nil {
				instance = *srcItem.Equipment
				instance.Slot = r.DestinationSlot
			}
			b.Equipment = append(b.Equipment, instance)
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
			if targetBagItem.Template != srcItem.Template || targetBagItem.ExpireTime != srcItem.ExpireTime {
				return b, v, 0, fmt.Errorf("target bag slot %d occupied by different item", r.DestinationSlot)
			}
			if targetBagItem.Amount >= limit {
				return b, v, 0, fmt.Errorf("目标背包堆叠已满")
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
			Slot:       r.DestinationSlot,
			Template:   srcItem.Template,
			Amount:     count,
			ExpireTime: srcItem.ExpireTime,
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

		itemAtSrc := v.ItemAt(r.SourceSlot)
		itemAtDest := v.ItemAt(r.DestinationSlot)

		// 识别客户端反转请求（实机抓包 131224：拖拽到空格子时，客户端将空目标作为 Source，被拖拽的原位作为 Destination）
		fromSlot := r.SourceSlot
		toSlot := r.DestinationSlot
		if itemAtSrc == nil && itemAtDest != nil {
			fromSlot, toSlot = r.DestinationSlot, r.SourceSlot
			itemAtSrc, itemAtDest = itemAtDest, nil
		}

		if itemAtSrc == nil {
			return b, v, 0, fmt.Errorf("source item not found in vault at slot %d", fromSlot)
		}

		// 移动到空槽位
		if itemAtDest == nil {
			count := r.Count
			if count == 0 || count >= itemAtSrc.Amount || itemAtSrc.IsEquip {
				itemAtSrc.Slot = toSlot
				moved := itemAtSrc.Amount
				if itemAtSrc.IsEquip {
					moved = 1
				}
				return b, v, moved, nil
			}
			// 先扣原堆叠，再追加新槽位，避免 append 扩容后旧指针失效。
			itemAtSrc.Amount -= count
			v.Items = append(v.Items, VaultItem{
				Slot:       toSlot,
				Template:   itemAtSrc.Template,
				Amount:     count,
				ExpireTime: itemAtSrc.ExpireTime,
				IsEquip:    false,
			})
			return b, v, count, nil
		}

		// 目标槽位已有物品：同类合并
		if !itemAtSrc.IsEquip && !itemAtDest.IsEquip && itemAtSrc.Template == itemAtDest.Template && itemAtSrc.ExpireTime == itemAtDest.ExpireTime {
			limit := rules.MissingStackLimit
			if limit == 0 {
				limit = 1000
			}
			if itemAtDest.Amount >= limit {
				return b, v, 0, fmt.Errorf("目标金库堆叠已满")
			}
			spaceLeft := limit - itemAtDest.Amount
			count := r.Count
			if count == 0 || count > itemAtSrc.Amount {
				count = itemAtSrc.Amount
			}
			if count > spaceLeft {
				count = spaceLeft
			}
			itemAtDest.Amount += count
			itemAtSrc.Amount -= count
			if itemAtSrc.Amount == 0 {
				for i := range v.Items {
					if v.Items[i].Slot == fromSlot {
						v.Items = append(v.Items[:i], v.Items[i+1:]...)
						break
					}
				}
			}
			return b, v, count, nil
		}

		// 不同物品：互换位置 (Swap)
		itemAtSrc.Slot = toSlot
		itemAtDest.Slot = fromSlot
		return b, v, 1, nil
	}

	return b, v, 0, fmt.Errorf("unsupported vault move combination: source=%d dest=%d", r.SourceList, r.DestinationList)
}
