// Package admin implements operator hand-outs: cera top-ups, gold and items.
//
// These are server-side actions with no client command behind them, so they
// need no recovered protocol. What they do need is every guarantee the ordinary
// reward paths already have: one transaction, an idempotency key, an audit row,
// and strict account ownership. A hand-out also never invents an item — every
// template must exist in the imported source and be acceptable to the very
// same bag code the game itself uses.
package admin

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

type Service struct {
	Store    *storage.Store
	Awarder  *inventory.Awarder
	Operator string
}

type Receipt struct {
	Grant     string                   `json:"grant"`
	Cera      uint64                   `json:"cera_balance,omitempty"`
	Gold      uint32                   `json:"gold_added,omitempty"`
	GoldAfter uint32                   `json:"gold_after,omitempty"`
	Items     []inventory.AwardReceipt `json:"items,omitempty"`
}

// Apply hands out one grant. Gold uses the bag's own currency path and items
// the shared awarder, so a full bag refuses the whole grant instead of
// silently dropping part of it.
func (s *Service) Apply(ctx context.Context, g storage.Grant) (Receipt, bool, error) {
	var out Receipt
	if s == nil || s.Store == nil {
		return out, false, fmt.Errorf("grant service is not configured")
	}
	if g.Operator == "" {
		g.Operator = s.Operator
	}
	if (g.Gold != 0 || len(g.Items) > 0) && s.Awarder == nil {
		return out, false, fmt.Errorf("gold and item grants need the source item catalog")
	}
	for _, it := range g.Items {
		if it.Template == 0 || it.Amount == 0 {
			return out, false, fmt.Errorf("an item grant needs a real template and amount")
		}
	}
	result, e := s.Store.ApplyGrant(ctx, g, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		state := current.State
		r := Receipt{Grant: g.ID}
		if g.Gold != 0 {
			b, err := inventory.ReadBag(state)
			if err != nil {
				return nil, nil, err
			}
			// Identity zero is the wallet, the same path a gold drop uses.
			b, _, err = b.Add(s.Awarder.Catalog, s.Awarder.Rules, 0, g.Gold)
			if err != nil {
				return nil, nil, err
			}
			if state, err = inventory.SaveBag(state, b); err != nil {
				return nil, nil, err
			}
			r.Gold, r.GoldAfter = g.Gold, b.Gold
		}
		for _, it := range g.Items {
			updated, award, err := s.Awarder.Grant(state, it.Template, it.Amount)
			if err != nil {
				return nil, nil, fmt.Errorf("item %d: %w", it.Template, err)
			}
			state = updated
			r.Items = append(r.Items, award)
		}
		receipt, err := json.Marshal(r)
		return state, receipt, err
	})
	if e != nil {
		return out, false, e
	}
	if len(result.Receipt) > 0 {
		if e = json.Unmarshal(result.Receipt, &out); e != nil {
			return out, result.Applied, e
		}
	}
	out.Grant, out.Cera = g.ID, result.Cera
	return out, result.Applied, nil
}
