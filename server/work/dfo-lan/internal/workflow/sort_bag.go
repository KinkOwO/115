package workflow

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// SortBag adopts one CMD20 arrangement into the stored bag under the owning
// character's lock. The receipt key makes a replayed frame a no-op instead of
// applying the same permutation twice. The permutation itself stays in
// inventory (BagRules/SortItems); this workflow owns the transaction.
func SortBag(ctx context.Context, store *storage.Store, role storage.Character, rules inventory.BagRules, key string, r protocol.SortItemRequest) (storage.Character, bool, error) {
	if store == nil {
		return role, false, fmt.Errorf("wear storage unavailable")
	}
	saved, applied, e := store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "ordinary-item-sort-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		b, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, nil, e
		}
		b, e = inventory.SortItems(b, rules, r)
		if e != nil {
			return nil, nil, e
		}
		raw, e := inventory.SaveBag(current.State, b)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(map[string]any{"list": r.List, "slots": len(r.Slots)})
		return raw, receipt, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}
