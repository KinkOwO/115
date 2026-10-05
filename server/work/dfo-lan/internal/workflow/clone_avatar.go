package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"strings"
)

// MigrateCloneAvatars writes no schema and moves no item outside one character
// event transaction. The receipt retains the exact old state for a guarded
// rollback; player moves after migration must not be overwritten by that state.
func (s *WearService) MigrateCloneAvatars(ctx context.Context, role database.Character) (database.Character, bool, error) {
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, false, fmt.Errorf("Clone migration requires owned storage and source")
	}
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		return role, false, err
	}
	_, changed, err := s.WearService.NormalizeCloneAvatars(b)
	if err != nil || !changed {
		return role, false, err
	}
	key := fmt.Sprintf("clone-avatar-native-v1:%x", sha256.Sum256(role.State))
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "clone-avatar-native-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		bag, err := inventory.ReadBag(current.State)
		if err != nil {
			return nil, nil, err
		}
		bag, _, err = s.WearService.NormalizeCloneAvatars(bag)
		if err != nil {
			return nil, nil, err
		}
		next, err := inventory.SaveBag(current.State, bag)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := json.Marshal(struct{ Before, After json.RawMessage }{current.State, next})
		return next, receipt, err
	})
	saved.WireID = role.WireID
	return saved, applied, err
}

func (s *WearService) RollbackCloneAvatarMigration(ctx context.Context, role database.Character, migrationKey string) (database.Character, bool, error) {
	if s == nil || s.Store == nil || !strings.HasPrefix(migrationKey, "clone-avatar-native-v1:") {
		return role, false, fmt.Errorf("invalid Clone migration rollback")
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, migrationKey)
	if err != nil {
		return role, false, err
	}
	var receipt struct{ Before, After json.RawMessage }
	if err = json.Unmarshal(raw, &receipt); err != nil {
		return role, false, err
	}
	if _, err = inventory.ReadBag(receipt.Before); err != nil {
		return role, false, err
	}
	if len(receipt.After) == 0 {
		return role, false, fmt.Errorf("Clone migration receipt missing after state")
	}
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "undo:"+migrationKey, "clone-avatar-native-undo-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		var currentFields, expectedFields any
		currentDecoder := json.NewDecoder(bytes.NewReader(current.State))
		currentDecoder.UseNumber()
		if e := currentDecoder.Decode(&currentFields); e != nil {
			return nil, nil, e
		}
		expectedDecoder := json.NewDecoder(bytes.NewReader(receipt.After))
		expectedDecoder.UseNumber()
		if e := expectedDecoder.Decode(&expectedFields); e != nil {
			return nil, nil, e
		}
		currentJSON, _ := json.Marshal(currentFields)
		expectedJSON, _ := json.Marshal(expectedFields)
		if !bytes.Equal(currentJSON, expectedJSON) {
			return nil, nil, fmt.Errorf("Clone migration rollback refused: character changed after migration")
		}
		return receipt.Before, json.RawMessage(`{"rolled_back":true}`), nil
	})
	saved.WireID = role.WireID
	return saved, applied, err
}
