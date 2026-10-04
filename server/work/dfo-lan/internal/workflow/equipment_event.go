package workflow

import (
	"context"
	"dfolan/internal/database"
	"encoding/json"
)

type equipmentEventStore interface {
	CommitCharacterEvent(context.Context, int64, int64, string, string, string, func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error)
	CharacterEventReceipt(context.Context, int64, int64, string) (json.RawMessage, error)
}

// commitEquipmentEvent restores the persisted receipt even on replay, when apply
// is skipped. Account-material operations retain their separate transaction.
func commitEquipmentEvent[T any](ctx context.Context, store equipmentEventStore, role database.Character, key, model string,
	apply func(database.Character) (json.RawMessage, T, error)) (database.Character, T, error) {
	var out T
	saved, _, err := store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, model,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
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
