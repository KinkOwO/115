package character

import (
	"dfolan/internal/savecontract"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sort"
)

func OdysseyGiftCatalog(r *catalog.OdysseyGrowth) catalog.LootCatalog {
	c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{}}
	if r == nil {
		return c
	}
	c.Source.Checksum = r.Source
	for _, id := range r.Gifts {
		c.Items[id] = catalog.LootItem{ID: id, Kind: "stackable", Grade: 1, Rarity: 2, StackableType: "[booster]", StackLimit: 1, Script: r.Items[id]}
	}
	return c
}

func (s *ProgressionService) ApplyOdysseyGift(role storage.Character, level byte, id uint32) (json.RawMessage, json.RawMessage, error) {
	if s.Odyssey == nil || !OdysseyRole(role) || role.ConfigVersion != savecontract.Identity() || s.Odyssey.Gifts[level] != id {
		return nil, nil, fmt.Errorf("invalid Odyssey milestone")
	}
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, nil, e
	}
	if state.Level < level {
		return nil, nil, fmt.Errorf("Odyssey milestone level not reached")
	}
	a := inventory.Awarder{Catalog: OdysseyGiftCatalog(s.Odyssey), Rules: inventory.BagRules{Source: savecontract.Identity(), Slots: map[string][2]uint16{"[booster]": {65, 120}}, MissingStackLimit: 1}}
	raw, receipt, e := a.Grant(role.State, id, 1)
	if e != nil {
		return nil, nil, e
	}
	proof, e := json.Marshal(receipt)
	return raw, proof, e
}

// Each milestone has an independent receipt. A full bag does not roll back
// completed growth, and the unpaid gift is retried at the next login/clear.
func (s *ProgressionService) OdysseyGifts(ctx context.Context, role storage.Character) (storage.Character, bool, []error) {
	if s.Odyssey == nil || !OdysseyRole(role) {
		return role, false, nil
	}
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return role, false, []error{e}
	}
	var levels []int
	for level := range s.Odyssey.Gifts {
		if state.Level >= level {
			levels = append(levels, int(level))
		}
	}
	sort.Ints(levels)
	changed := false
	var pending []error
	for _, n := range levels {
		level := byte(n)
		id := s.Odyssey.Gifts[level]
		next, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, savecontract.Identity(), fmt.Sprintf("odyssey-level-gift:%d:%d", level, id), "odyssey-source-gift-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			return s.ApplyOdysseyGift(current, level, id)
		})
		if e != nil {
			pending = append(pending, fmt.Errorf("gift %d remains owed: %w", id, e))
			continue
		}
		next.WireID = role.WireID
		role = next
		changed = changed || applied
	}
	return role, changed, pending
}
