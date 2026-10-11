package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
)

type BakalFrozenCard struct {
	Kind      byte
	Products  []loot.Award
	Selection *byte
	Claimed   bool
}

func (s *BakalRewardService) ownedCard(current database.Character, run string, kind byte) (map[string]json.RawMessage, bakalProgress, BakalRewardPlan, int, error) {
	state, p, err := readBakalProgress(current.State)
	if err != nil {
		return nil, p, BakalRewardPlan{}, 0, err
	}
	plan, ok := p.Pending[run]
	if !ok || plan.Run != run || plan.Source != current.ConfigVersion || plan.Content != s.Awarder.Catalog.Source.Checksum || current.ConfigVersion != s.Awarder.Catalog.Source.SaveIdentity() || !plan.Eligible {
		return nil, p, plan, 0, fmt.Errorf("no owned native raid card plan")
	}
	for i, card := range plan.Cards {
		if card.Kind == kind {
			return state, p, plan, i, nil
		}
	}
	return nil, p, plan, 0, fmt.Errorf("native raid card category unavailable")
}

// The selection survives a full bag or disconnect. A replay restores this
// receipt; it cannot change the chosen index or reroll the frozen products.
func (s *BakalRewardService) SelectCard(ctx context.Context, role database.Character, run string, kind, index byte) (database.Character, BakalFrozenCard, error) {
	var empty BakalFrozenCard
	if s == nil || s.Store == nil || s.Awarder == nil || run == "" || kind > 1 {
		return role, empty, fmt.Errorf("invalid raid card service/request")
	}
	saved, card, err := commitEquipmentEvent(ctx, s.Store, role, fmt.Sprintf("bakal-card-select:%s:%d", run, kind), "bakal-native-card-v1", func(current database.Character) (json.RawMessage, BakalFrozenCard, error) {
		state, p, plan, i, err := s.ownedCard(current, run, kind)
		if err != nil {
			return nil, empty, err
		}
		if uint32(index) >= plan.CardSlots || plan.CardSlots == 0 {
			return nil, empty, fmt.Errorf("raid card outside native source slots")
		}
		card := plan.Cards[i]
		if card.Selection != nil && *card.Selection != index {
			return nil, empty, fmt.Errorf("raid card choice is already frozen")
		}
		selected := index
		card.Selection = &selected
		plan.Cards[i] = card
		p.Pending[run] = plan
		next, err := saveBakalProgress(state, p)
		return next, card, err
	})
	if err == nil && (card.Selection == nil || *card.Selection != index) {
		err = fmt.Errorf("raid card choice is already frozen")
	}
	return saved, card, err
}

// Each category has one inventory/event transaction. Completing one card
// cannot consume the other category, and failed grants retain both the choice
// and the remaining receipt without partially mutating inventory.
func (s *BakalRewardService) ClaimCard(ctx context.Context, role database.Character, run string, kind byte) (database.Character, BakalFrozenCard, error) {
	var empty BakalFrozenCard
	if s == nil || s.Store == nil || s.Awarder == nil || run == "" || kind > 1 {
		return role, empty, fmt.Errorf("invalid raid card service/request")
	}
	return commitEquipmentEvent(ctx, s.Store, role, fmt.Sprintf("bakal-card-grant:%s:%d", run, kind), "bakal-native-card-v1", func(current database.Character) (json.RawMessage, BakalFrozenCard, error) {
		_, p, plan, i, err := s.ownedCard(current, run, kind)
		if err != nil {
			return nil, empty, err
		}
		card := plan.Cards[i]
		if card.Selection == nil || card.Claimed {
			return nil, empty, fmt.Errorf("raid card is not selected")
		}
		next := current.State
		for _, product := range card.Products {
			if product.Template == 0 || product.Amount == 0 {
				return nil, empty, fmt.Errorf("invalid frozen raid card product")
			}
			next, _, err = s.Awarder.Grant(next, product.Template, product.Amount)
			if err != nil {
				return nil, empty, err
			}
		}
		card.Claimed = true
		plan.Cards[i] = card
		complete := true
		for _, other := range plan.Cards {
			complete = complete && other.Claimed
		}
		if complete {
			delete(p.Pending, run)
		} else {
			p.Pending[run] = plan
		}
		state, _, err := readBakalProgress(next)
		if err != nil {
			return nil, empty, err
		}
		next, err = saveBakalProgress(state, p)
		return next, card, err
	})
}
