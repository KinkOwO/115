package inventory

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
)

type WearRules struct {
	Source string            `json:"source"`
	Slots  map[string]uint16 `json:"slots"`
}

func LoadWearRules(path, source string) (WearRules, error) {
	var r WearRules
	b, e := os.ReadFile(path)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	if r.Source != source || len(r.Slots) == 0 {
		return r, fmt.Errorf("wear rules source mismatch")
	}
	for _, slot := range r.Slots {
		if slot < 12 || slot > 25 {
			return r, fmt.Errorf("unsupported ordinary equipment slot")
		}
	}
	return r, nil
}

type WearService struct {
	Store       *storage.Store
	Catalog     *EquipmentCatalog
	Professions catalog.Characters
	BagRules    BagRules
	Rules       WearRules
}

func (s *WearService) wearable(role storage.Character, item BagEquipment, slot uint16) error {
	d, ok := s.Catalog.index[item.Template]
	if !ok {
		return fmt.Errorf("equipment definition missing")
	}
	// Wearing uses the structural reward check, NOT the drop-pool Basic rule.
	// Basic additionally demands [free] attach and rarity <= 1, which are the
	// rules that decide what a monster may DROP, not what a character may WEAR.
	// Gating wear on them refused a piece the character already owns and
	// legitimately qualifies for: a [trade delete] bound shoe (e.g. the
	// Explorer-collection 100261068) and every rarity-2+ uncommon drop an
	// archer meets the level/job/grow-type for. Live capture 20260911T2159
	// shows exactly this "my own gear cannot be equipped" refusal. Structural
	// validity + slot fit + WearableBy (level/job/grow) is the wear gate.
	if _, e := s.Catalog.Reward(item.Template); e != nil {
		return e
	}
	kind := d.Fields["[equipment type]"]
	expected, ok := s.Rules.Slots[kind[0].Text]
	if !ok || expected != slot {
		return fmt.Errorf("equipment does not fit destination slot")
	}
	var state struct {
		Level       byte `json:"level"`
		Advancement byte `json:"advancement"`
	}
	if e := json.Unmarshal(role.State, &state); e != nil {
		return e
	}
	job, ok := s.Professions.Professions[role.Profession]
	if !ok {
		return fmt.Errorf("equipment profession unavailable")
	}
	return WearableBy(d.Fields, job.Job, state.Advancement, state.Level)
}

// MoveOrdinary validates both directions before swapping one physical item.
// Equipped items retain identity and durability; no reward or copy is created.
func (s *WearService) MoveOrdinary(role storage.Character, r protocol.ItemMoveRequest) (json.RawMessage, error) {
	if s == nil || s.Catalog == nil || s.Catalog.Source.Checksum != role.ConfigVersion || s.Rules.Source != role.ConfigVersion || s.Professions.Source.Checksum != role.ConfigVersion {
		return nil, fmt.Errorf("wear service source mismatch")
	}
	if (r.SourceList != 0 && r.SourceList != 3) || (r.DestinationList != 0 && r.DestinationList != 3) || r.Count > 1 || r.Extra != 0 || r.Selection != 0xffffffff || r.Flags != [3]byte{} {
		return nil, fmt.Errorf("unsupported ordinary equipment move")
	}
	if r.SourceList == r.DestinationList && r.SourceSlot == r.DestinationSlot {
		return nil, fmt.Errorf("identical equipment locations")
	}
	b, e := ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	find := func(list byte, slot uint16) (*BagEquipment, error) {
		rows := b.Worn
		if list == 0 {
			if slot < s.BagRules.EquipmentSlots[0] || slot > s.BagRules.EquipmentSlots[1] {
				return nil, fmt.Errorf("slot outside equipment bag")
			}
			rows = b.Equipment
			for _, v := range b.Items {
				if v.Slot == slot {
					return nil, fmt.Errorf("slot contains stackable item")
				}
			}
		} else if slot < 12 || slot > 25 {
			return nil, fmt.Errorf("slot outside body equipment")
		}
		for _, v := range rows {
			if v.Slot == slot {
				copy := v
				return &copy, nil
			}
		}
		return nil, nil
	}
	a, e := find(r.SourceList, r.SourceSlot)
	if e != nil {
		return nil, e
	}
	z, e := find(r.DestinationList, r.DestinationSlot)
	if e != nil {
		return nil, e
	}
	if a == nil && z == nil {
		return nil, fmt.Errorf("both equipment locations are empty")
	}
	if (a == nil && r.SourceItem != 0) || (a != nil && r.SourceItem != 0 && r.SourceItem != a.Template) || (r.DestinationItem != 0 && (z == nil || r.DestinationItem != z.Template)) {
		return nil, fmt.Errorf("stale equipment identity")
	}
	if a != nil && r.DestinationList == 3 {
		if e = s.wearable(role, *a, r.DestinationSlot); e != nil {
			return nil, e
		}
	}
	if z != nil && r.SourceList == 3 {
		if e = s.wearable(role, *z, r.SourceSlot); e != nil {
			return nil, e
		}
	}
	replace := func(list byte, slot uint16, item *BagEquipment) {
		rows := &b.Worn
		if list == 0 {
			rows = &b.Equipment
		}
		kept := make([]BagEquipment, 0, len(*rows)+1)
		for _, v := range *rows {
			if v.Slot != slot {
				kept = append(kept, v)
			}
		}
		if item != nil {
			v := *item
			v.Slot = slot
			kept = append(kept, v)
		}
		*rows = kept
	}
	replace(r.SourceList, r.SourceSlot, z)
	replace(r.DestinationList, r.DestinationSlot, a)
	return SaveBag(role.State, b)
}

func (s *WearService) Move(ctx context.Context, role storage.Character, key string, r protocol.ItemMoveRequest) (storage.Character, bool, error) {
	if s == nil || s.Store == nil {
		return role, false, fmt.Errorf("wear storage unavailable")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "ordinary-equipment-move-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		raw, e := s.MoveOrdinary(current, r)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(r)
		return raw, receipt, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}

// WornSpaceUpdate rebuilds the local actor's equipped visuals after entry.
// Restored for this handoff from the documented39 NOTI14 path; not a claim
// that the original39 source has been recovered byte for byte.
func WornSpaceUpdate(state json.RawMessage) ([]byte, error) {
	b, e := ReadBag(state)
	if e != nil {
		return nil, e
	}
	if len(b.Worn) == 0 {
		return nil, nil
	}
	rows := make([][protocol.CurrentItemRecordSize]byte, 0, len(b.Worn))
	for _, item := range b.Worn {
		rows = append(rows, EquipmentRow(item))
	}
	return protocol.InventorySpaceUpdate(3, rows)
}

func WornPayload(state json.RawMessage) ([]byte, error) {
	b, e := ReadBag(state)
	if e != nil {
		return nil, e
	}
	rows := make([][protocol.CurrentItemRecordSize]byte, 0, len(b.Worn))
	for _, item := range b.Worn {
		rows = append(rows, EquipmentRow(item))
	}
	return protocol.WornRestore(rows)
}
