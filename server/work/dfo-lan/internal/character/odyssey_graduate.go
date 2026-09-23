package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// OdysseyGraduationLevel is the level at which an Arad Odyssey character
// graduates into a regular character: the largest [grow up level on dungeon
// clear] target (100004990 -> 115) of aradodyssey.etc.
const OdysseyGraduationLevel byte = 115

// OdysseyGraduated reports whether the persisted state carries the graduation
// mark. It never consults the create request, so the mark survives even if the
// original packet is unavailable.
func OdysseyGraduated(role storage.Character) bool {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return false
	}
	return state.OdysseyGraduated
}

// OdysseyMember reports whether the character still lives the Arad Odyssey
// experience: created as one and not graduated yet. Town/world admission uses
// this so a graduated character is gated by the regular level rules.
func OdysseyMember(role storage.Character) bool {
	return CreatedAsOdyssey(role) && !OdysseyGraduated(role)
}

// OdysseyGraduateBoxCatalog exposes only the graduation reward template, so a
// tampered catalog can never make the server grant anything the growth log
// does not name.
func OdysseyGraduateBoxCatalog(r *catalog.OdysseyGrowth) catalog.LootCatalog {
	c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{}}
	if r == nil || r.GraduateReward == 0 {
		return c
	}
	c.Source.Checksum = r.Source
	c.Items[r.GraduateReward] = catalog.LootItem{ID: r.GraduateReward, Kind: "stackable", Grade: 1, Rarity: 2, StackableType: "[booster]", StackLimit: 1, Script: r.Items[r.GraduateReward]}
	return c
}

// ApplyOdysseyGraduation flips a max-level Odyssey character into a regular
// one: the persisted creation options lose the Odyssey marker (the original
// create request packet stays untouched history) and the graduation mark is
// recorded. Persisted-mode rewrite + mark land atomically in one receipt.
func (s *ProgressionService) ApplyOdysseyGraduation(role storage.Character) (json.RawMessage, json.RawMessage, error) {
	if s.Odyssey == nil || !CreatedAsOdyssey(role) || role.ConfigVersion != s.Odyssey.Source {
		return nil, nil, fmt.Errorf("invalid Odyssey graduation role")
	}
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, nil, e
	}
	if state.Level < OdysseyGraduationLevel {
		return nil, nil, fmt.Errorf("Odyssey graduation requires level %d", OdysseyGraduationLevel)
	}
	if state.OdysseyGraduated {
		return nil, nil, fmt.Errorf("Odyssey graduation already recorded")
	}
	state.OdysseyGraduated = true
	state.CreationMode = 0
	if len(state.CreationOptions) == 12 {
		state.CreationOptions[10] = 0
	}
	raw, e := json.Marshal(state)
	if e != nil {
		return nil, nil, e
	}
	proof, e := json.Marshal(map[string]any{"level": state.Level, "reward_template": s.Odyssey.GraduateReward, "source": s.Odyssey.Source})
	return raw, proof, e
}

// OdysseyGraduate commits the graduation receipt exactly once.
func (s *ProgressionService) OdysseyGraduate(ctx context.Context, role storage.Character) (storage.Character, bool, error) {
	if s.Odyssey == nil {
		return role, false, nil
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Odyssey.Source, "odyssey-graduate-v1", "odyssey-graduate-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return s.ApplyOdysseyGraduation(current)
	})
}

// ApplyOdysseyGraduationReward grants the graduation booster box. The receipt
// is keyed by the graduation mark instead of the Odyssey membership, so a full
// bag only owes the box; it can never block the graduation itself.
func (s *ProgressionService) ApplyOdysseyGraduationReward(role storage.Character) (json.RawMessage, json.RawMessage, error) {
	if s.Odyssey == nil || !OdysseyGraduated(role) || role.ConfigVersion != s.Odyssey.Source {
		return nil, nil, fmt.Errorf("invalid Odyssey graduation reward role")
	}
	id := s.Odyssey.GraduateReward
	if id == 0 {
		return nil, nil, fmt.Errorf("missing Odyssey graduation reward template")
	}
	a := inventory.Awarder{Catalog: OdysseyGraduateBoxCatalog(s.Odyssey), Rules: inventory.BagRules{Source: s.Odyssey.Source, Slots: map[string][2]uint16{"[booster]": {65, 120}}, MissingStackLimit: 1}}
	raw, receipt, e := a.Grant(role.State, id, 1)
	if e != nil {
		return nil, nil, e
	}
	proof, e := json.Marshal(receipt)
	return raw, proof, e
}

// OdysseyGraduationReward commits the graduation box receipt exactly once.
func (s *ProgressionService) OdysseyGraduationReward(ctx context.Context, role storage.Character) (storage.Character, bool, error) {
	if s.Odyssey == nil {
		return role, false, nil
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Odyssey.Source, "odyssey-graduate-reward-v1", "odyssey-graduate-reward-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return s.ApplyOdysseyGraduationReward(current)
	})
}
