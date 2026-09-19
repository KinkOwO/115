package inventory

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
)

type WearRules struct {
	Source  string            `json:"source"`
	Slots   map[string]uint16 `json:"slots"`
	Special bool              `json:"special,omitempty"`
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
		if !EquipmentBodySlot(slot) || (!r.Special && (slot < 12 || slot > 25)) {
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

func (s *WearService) EggHatchTarget(template uint32) uint32 {
	if s != nil && s.Catalog != nil {
		if d, err := s.Catalog.Definition(template); err == nil {
			kind := d.Fields["[equipment type]"]
			subType := d.Fields["[sub type]"]
			output := d.Fields["[output index]"]
			if len(kind) > 0 && kind[0].Text == "[creature]" && len(subType) > 0 && subType[0].Value == 1 && len(output) > 0 && output[0].Value > 0 {
				return uint32(output[0].Value)
			}
		}
	}
	return EggHatchOutputs[template]
}

func (s *WearService) wearable(role storage.Character, item BagEquipment, slot uint16) error {
	d, err := s.Catalog.Definition(item.Template)
	if err != nil {
		return err
	}
	// Wearing existing instances must not inherit quest/drop eligibility.
	if e := item.ValidateRecord(); e != nil {
		return e
	}
	kind := d.Fields["[equipment type]"]
	if len(kind) == 0 || kind[0].Type != 6 {
		return fmt.Errorf("missing equipment type")
	}
	expected, ok := s.Rules.Slots[kind[0].Text]
	talismanSlot := s.Rules.Special && kind[0].Text == "[talisman]" && slot >= 33 && slot <= 35
	primerSlot := s.Rules.Special && kind[0].Text == "[primer]" && slot >= 36 && slot <= 46
	if !ok || (expected != slot && !talismanSlot && !primerSlot) {
		return fmt.Errorf("equipment does not fit destination slot")
	}
	if kind[0].Text == "[creature]" {
		subType := d.Fields["[sub type]"]
		if len(subType) > 0 && subType[0].Value == 1 {
			return fmt.Errorf("creature must be hatched before equipping")
		}
	}
	if durability := d.Fields["[durability]"]; len(durability) > 0 {
		if len(durability) != 1 || durability[0].Type != 0 || durability[0].Value < 0 || durability[0].Value > 65535 {
			return fmt.Errorf("invalid equipment durability")
		}
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
	validSpace := func(v byte) bool { return v == 0 || v == 3 || (s.Rules.Special && (v == 1 || v == 7)) }
	if !validSpace(r.SourceList) || !validSpace(r.DestinationList) || r.Count > 1 || r.Extra != 0 || r.Selection != 0xffffffff || r.Flags != [3]byte{} {
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
		} else if list == 1 || list == 7 {
			rows = b.Special[list]
		} else if !EquipmentBodySlot(slot) || (!s.Rules.Special && (slot < 12 || slot > 25)) {
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
		if r.DestinationSlot == 26 {
			if hatched := s.EggHatchTarget(a.Template); hatched != 0 {
				a.Template = hatched
			}
		}
		if e = s.wearable(role, *a, r.DestinationSlot); e != nil {
			return nil, e
		}
	}
	if z != nil && r.SourceList == 3 {
		if e = s.wearable(role, *z, r.SourceSlot); e != nil {
			return nil, e
		}
	}
	for _, move := range []struct {
		item *BagEquipment
		list byte
	}{{a, r.DestinationList}, {z, r.SourceList}} {
		if move.item == nil || move.list == 3 {
			continue
		}
		d, err := s.Catalog.Definition(move.item.Template)
		if err != nil {
			return nil, err
		}
		kind := d.Fields["[equipment type]"]
		if len(kind) == 0 {
			return nil, fmt.Errorf("missing equipment kind")
		}
		expected := EquipmentBagSpace(kind[0].Text)
		if move.list != expected {
			return nil, fmt.Errorf("equipment inventory family mismatch")
		}
	}
	replace := func(list byte, slot uint16, item *BagEquipment) {
		rows := &b.Worn
		if list == 0 {
			rows = &b.Equipment
		}
		var special []BagEquipment
		if list == 1 || list == 7 {
			special = b.Special[list]
			rows = &special
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
			if slot == 26 && list == 3 {
				var rec [protocol.CurrentItemRecordSize]byte
				if len(v.Record) == protocol.CurrentItemRecordSize {
					copy(rec[:], v.Record)
				}
				binary.LittleEndian.PutUint16(rec[0:], 26)
				binary.LittleEndian.PutUint32(rec[2:], v.Template)
				binary.LittleEndian.PutUint32(rec[6:], 1)
				v.Record = rec[:]
			}
			if list == 7 {
				var rec [protocol.CurrentItemRecordSize]byte
				if len(v.Record) == protocol.CurrentItemRecordSize {
					copy(rec[:], v.Record)
				}
				binary.LittleEndian.PutUint16(rec[0:], slot)
				binary.LittleEndian.PutUint32(rec[2:], v.Template)
				var key uint32
				if slot < 140 {
					key = uint32(slot + 2)
					if len(v.Record) == protocol.CurrentItemRecordSize {
						if existingKey := binary.LittleEndian.Uint32(v.Record[6:10]); existingKey != 0 && existingKey != 1 {
							key = existingKey
						}
					}
				}
				binary.LittleEndian.PutUint32(rec[6:], key)
				v.Record = rec[:]
			}
			kept = append(kept, v)
		}
		*rows = kept
		if list == 1 || list == 7 {
			if b.Special == nil {
				b.Special = map[byte][]BagEquipment{}
			}
			b.Special[list] = kept
		}
	}
	replace(r.SourceList, r.SourceSlot, z)
	replace(r.DestinationList, r.DestinationSlot, a)
	if _, e = EquipmentPayload(3, b.Worn, false); e != nil {
		return nil, e
	}
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
	return EquipmentPayload(3, b.Worn, false)
}

func WornPayload(state json.RawMessage) ([]byte, error) {
	b, e := ReadBag(state)
	if e != nil {
		return nil, e
	}
	return EquipmentPayload(3, b.Worn, true)
}
