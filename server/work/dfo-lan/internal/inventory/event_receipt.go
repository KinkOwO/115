package inventory

import (
	"context"
	"dfolan/internal/storage"
	"encoding/json"
)

// Read the durable receipt on both first application and replay.
func commitEquipmentEvent[T any](ctx context.Context, store *storage.Store, role storage.Character, key, model string, apply func(storage.Character) (json.RawMessage, T, error)) (storage.Character, T, error) {
	var out T
	saved, _, err := store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		next, receipt, err := apply(current)
		if err != nil {
			return nil, nil, err
		}
		encoded, err := json.Marshal(receipt)
		if err != nil {
			return nil, nil, err
		}
		return next, encoded, nil
	})
	if err != nil {
		return role, out, err
	}
	receipt, err := store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err == nil {
		err = json.Unmarshal(receipt, &out)
	}
	saved.WireID = role.WireID
	return saved, out, err
}
