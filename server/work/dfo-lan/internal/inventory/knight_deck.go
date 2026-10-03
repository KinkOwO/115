package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

type KnightDeckApply struct {
	Equipped    uint32 `json:"equipped"`
	WornChanged bool   `json:"worn_changed"`
}

// Socket zero has one source of truth, including legacy/inconsistent saves.
func (b Bag) KnightDeck() [protocol.KnightDeckSize]uint32 {
	var deck [protocol.KnightDeckSize]uint32
	copy(deck[:], b.KnightShieldDeck)
	deck[0] = 0
	for _, w := range b.Worn {
		if w.Slot == 24 && w.Group == 0 {
			deck[0] = w.Template
			break
		}
	}
	return deck
}

func (s *WearService) knightGate(role Role) error {
	if s == nil || s.Shields == nil {
		return ShieldRefusal("knight shield catalog unavailable")
	}
	if s.Catalog == nil || s.Catalog.Source.SaveIdentity() != role.ConfigVersion || s.Professions.Source.SaveIdentity() != role.ConfigVersion || s.Shields.Source.SaveIdentity() != role.ConfigVersion {
		return ShieldRefusal("knight shield source mismatch")
	}
	if job, ok := s.Professions.Professions[role.Profession]; !ok || job.Job != "[knight]" || role.Profession != s.Shields.Profession {
		return ShieldRefusal("shield window requires knight profession")
	}
	return nil
}

func (s *WearService) ApplyKnightDeck(role Role, deck [protocol.KnightDeckSize]uint32) (json.RawMessage, KnightDeckApply, error) {
	var result KnightDeckApply
	if e := s.knightGate(role); e != nil {
		return nil, result, e
	}
	b, e := ReadBag(role.State)
	if e != nil {
		return nil, result, ShieldRefusal(e.Error())
	}
	old := b.KnightDeck()[0]
	if old != 0 && !s.Shields.Offers(old) {
		return nil, result, ShieldRefusal("worn support slot is outside shield window")
	}
	var state struct {
		Level byte `json:"level"`
	}
	if e = json.Unmarshal(role.State, &state); e != nil {
		return nil, result, ShieldRefusal(e.Error())
	}
	// The client reports its absolute map; never infer a reserve placement.
	for _, item := range deck {
		if item == 0 {
			continue
		}
		if ok, why := s.Shields.Allowed(item, state.Level); !ok {
			return nil, result, ShieldRefusal(why)
		}
		def, err := s.Catalog.Definition(item)
		if err != nil {
			return nil, result, ShieldRefusal(err.Error())
		}
		if def.SHA256 != s.Shields.index[item].EquSHA256 {
			return nil, result, ShieldRefusal("shield equipment provenance mismatch")
		}
	}
	if deck[0] != 0 {
		d, err := s.Catalog.Reward(deck[0])
		if err != nil {
			return nil, result, ShieldRefusal(err.Error())
		}
		item := BagEquipment{Slot: s.Shields.Slot(), Template: deck[0], Durability: d}
		if err = s.wearable(role, item, s.Shields.Slot()); err != nil {
			return nil, result, ShieldRefusal(err.Error())
		}
		if old != deck[0] {
			b.Worn = replaceKnightWorn(b.Worn, s.Shields.Slot(), &item)
		}
	} else if old != 0 {
		b.Worn = replaceKnightWorn(b.Worn, s.Shields.Slot(), nil)
	}
	result = KnightDeckApply{Equipped: deck[0], WornChanged: old != deck[0]}
	b.KnightShieldDeck = append([]uint32(nil), deck[:]...)
	// Mirror after the decision, rather than trusting a second stored slot 0.
	b.KnightShieldDeck[0] = b.KnightDeck()[0]
	raw, err := SaveBag(role.State, b)
	return raw, result, err
}

func replaceKnightWorn(rows []BagEquipment, slot uint16, item *BagEquipment) []BagEquipment {
	out := make([]BagEquipment, 0, len(rows)+1)
	for _, r := range rows {
		if r.Slot != slot || r.Group != 0 {
			out = append(out, r)
		}
	}
	if item != nil {
		out = append(out, *item)
	}
	return out
}

func (s *WearService) moveKnightShield(role Role, r protocol.ItemMoveRequest) (json.RawMessage, error) {
	if e := s.knightGate(role); e != nil {
		return nil, e
	}
	if r.Count > 1 || r.Extra != 0 || r.Selection != 0xffffffff || r.Flags[0] != 0 || r.Flags[1] != 0 || r.Flags[2] > 1 {
		return nil, ShieldRefusal("unsupported shield move shape")
	}
	b, e := ReadBag(role.State)
	if e != nil {
		return nil, ShieldRefusal(e.Error())
	}
	deck := b.KnightDeck()
	switch {
	case r.SourceList == ShieldGridSpace && r.DestinationList == ShieldActiveSpace:
		if r.DestinationSlot != 0 || r.SourceItem == 0 {
			return nil, ShieldRefusal("shield shelf can only equip socket zero")
		}
		if r.DestinationItem != 0 && r.DestinationItem != deck[0] {
			return nil, ShieldRefusal("stale shield destination")
		}
		deck[0] = r.SourceItem
	case r.SourceList == ShieldActiveSpace && r.DestinationList == ShieldGridSpace:
		if r.SourceSlot != 0 || deck[0] == 0 || !s.Shields.Offers(deck[0]) {
			return nil, ShieldRefusal("shield unequip requires occupied socket zero")
		}
		if r.SourceItem != 0 && r.SourceItem != deck[0] {
			return nil, ShieldRefusal("stale shield source")
		}
		deck[0] = 0
	case r.SourceList == ShieldActiveSpace && r.DestinationList == ShieldActiveSpace:
		if r.SourceSlot >= protocol.KnightDeckSize || r.DestinationSlot >= protocol.KnightDeckSize || r.SourceSlot == r.DestinationSlot {
			return nil, ShieldRefusal("invalid shield deck positions")
		}
		if deck[r.SourceSlot] == 0 {
			return nil, ShieldRefusal("empty shield source")
		}
		if (r.SourceItem != 0 && r.SourceItem != deck[r.SourceSlot]) || (r.DestinationItem != 0 && r.DestinationItem != deck[r.DestinationSlot]) {
			return nil, ShieldRefusal("stale shield deck identity")
		}
		deck[r.SourceSlot], deck[r.DestinationSlot] = deck[r.DestinationSlot], deck[r.SourceSlot]
		if r.SourceSlot == 0 {
			deck[0] = 0
		} // dragging equipped out is an unequip
	default:
		return nil, ShieldRefusal("unsupported shield move direction")
	}
	raw, _, err := s.ApplyKnightDeck(role, deck)
	return raw, err
}

func (s *WearService) KnightDeckPayload(role Role) ([]byte, error) {
	if s == nil || s.Shields == nil {
		return nil, nil
	}
	job, ok := s.Professions.Professions[role.Profession]
	if !ok || job.Job != "[knight]" {
		return nil, nil
	}
	if e := s.knightGate(role); e != nil {
		return nil, e
	}
	b, e := ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	if len(b.KnightShieldDeck) > protocol.KnightDeckSize {
		return nil, fmt.Errorf("saved knight deck exceeds five slots")
	}
	deck := b.KnightDeck()
	if len(b.KnightShieldDeck) == 0 && deck[0] == 0 {
		return nil, nil
	}
	return protocol.KnightDeckInfo(deck), nil
}
