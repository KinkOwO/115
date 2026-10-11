package workflow

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"time"
)

// ExchangeSymbolSocket commits the bag and equipped emblem together using the
// existing cross-engine event store. A retried request reuses its persisted
// sequence and receipt; it cannot consume a second item after a lost reply.
// No gateway calls this candidate yet. The admission and effect execution
// contracts listed on PrepareSymbolSocketExchange remain release blockers.
func (s *ItemService) ExchangeSymbolSocket(ctx context.Context, role database.Character, layout *inventory.SymbolSocketLayout, req inventory.SymbolSocketRequest, now time.Time) (database.Character, bool, error) {
	if s == nil || s.Store == nil || s.Items == nil || layout == nil || req.Source != s.Items.Catalog.Source.Checksum {
		return role, false, fmt.Errorf("symbol socket service unavailable or mismatched")
	}
	identity, e := json.Marshal(req)
	if e != nil {
		return role, false, e
	}
	key := fmt.Sprintf("symbol-socket:%d:%x", req.Sequence, sha256.Sum256(identity))
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Items.Catalog.Source.SaveIdentity(), key, "symbol-socket-exchange-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		next, err := inventory.PrepareSymbolSocketExchange(current.State, s.Items.Catalog, s.Items.BagRules, layout, s.Items.EmblemInlay, req, now.Unix())
		if err != nil {
			return nil, nil, err
		}
		return next, identity, nil
	})
	if e != nil {
		return role, false, e
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return role, false, e
	}
	var stored inventory.SymbolSocketRequest
	if e = json.Unmarshal(receipt, &stored); e != nil {
		return role, false, e
	}
	if stored != req {
		return role, false, fmt.Errorf("symbol socket receipt conflict")
	}
	saved.WireID = role.WireID
	return saved, applied, nil
}
