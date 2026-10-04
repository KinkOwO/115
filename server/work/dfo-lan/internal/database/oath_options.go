package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// EquippedOathOption projects the equipped core and its persisted option.
// The old character JSON needs no migration: its worn slot remains authority.
type EquippedOathOption struct {
	Level  int
	ItemID uint32
	Option int
}

func oathEquippedFromState(raw json.RawMessage) (EquippedOathOption, error) {
	var state struct {
		Level     int `json:"level"`
		Inventory struct {
			Worn []struct {
				Slot     uint16 `json:"slot"`
				Template uint32 `json:"template"`
			} `json:"worn"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return EquippedOathOption{}, err
	}
	if state.Level < 1 || state.Level > 255 {
		return EquippedOathOption{}, fmt.Errorf("invalid character level for oath")
	}
	out := EquippedOathOption{Level: state.Level}
	for _, worn := range state.Inventory.Worn {
		if worn.Slot == 47 {
			if worn.Template == 0 || out.ItemID != 0 {
				return EquippedOathOption{}, fmt.Errorf("invalid worn oath core")
			}
			out.ItemID = worn.Template
		}
	}
	return out, nil
}

func oathCoreKey(itemID uint32) string { return fmt.Sprintf("oath:%d", itemID) }

// EquippedOathSelection reloads level and slot 47 from the database. A
// bootstrap C2S2382 must call this instead of trusting its local option 1.
func (s *Store) EquippedOathSelection(ctx context.Context, accountID, characterID int64) (EquippedOathOption, error) {
	if accountID <= 0 || characterID <= 0 {
		return EquippedOathOption{}, errors.New("invalid oath character")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return EquippedOathOption{}, err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	raw, err := queries.ShareActiveCharacterState(ctx, sqlcgen.ShareActiveCharacterStateParams{AccountID: accountID, CharacterID: characterID})
	if err != nil {
		return EquippedOathOption{}, err
	}
	out, err := oathEquippedFromState(raw)
	if err != nil || out.Level < 115 || out.ItemID == 0 {
		if err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	selected, err := queries.EquippedOathSelection(ctx, sqlcgen.EquippedOathSelectionParams{CharacterID: characterID, CoreInstanceKey: oathCoreKey(out.ItemID)})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return EquippedOathOption{}, err
	}
	out.Option = 1
	if err == nil {
		if selected < 1 || selected > 3 {
			return EquippedOathOption{}, fmt.Errorf("invalid stored oath selection")
		}
		out.Option = int(selected)
	}
	return out, tx.Commit(ctx)
}

// SelectEquippedOathOption validates the selected character, level, and worn
// core while holding the same character lock used by equipment moves.
func (s *Store) SelectEquippedOathOption(ctx context.Context, accountID, characterID int64, itemID uint32, option int) (EquippedOathOption, error) {
	if accountID <= 0 || characterID <= 0 || itemID == 0 || option < 1 || option > 3 {
		return EquippedOathOption{}, errors.New("invalid oath selection")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return EquippedOathOption{}, err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	raw, err := queries.LockActiveCharacterState(ctx, sqlcgen.LockActiveCharacterStateParams{AccountID: accountID, CharacterID: characterID})
	if err != nil {
		return EquippedOathOption{}, err
	}
	out, err := oathEquippedFromState(raw)
	if err != nil {
		return EquippedOathOption{}, err
	}
	if out.Level < 115 || out.ItemID != itemID {
		return EquippedOathOption{}, fmt.Errorf("oath core is locked or request item is not worn in slot 47")
	}
	key := oathCoreKey(itemID)
	err = queries.SelectEquippedOathOption(ctx, sqlcgen.SelectEquippedOathOptionParams{CharacterID: characterID, CoreInstanceKey: key, SelectedOption: int64(option)})
	if err != nil {
		return EquippedOathOption{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return EquippedOathOption{}, err
	}
	out.Option = option
	return out, nil
}

var ErrOathOptionRevisionConflict = errors.New("oath option revision conflict")

type OathOptionState struct {
	CharacterID     int64
	CoreInstanceKey string
	SelectedOption  int
	Revision        int64
}

// MigrateOathOptions creates the per-core option ledger. It is additive and
// safe for existing character saves; no JSON state is rewritten.
func (s *Store) MigrateOathOptions(ctx context.Context) error {
	return s.execMigration(ctx, "0032_oath_options.sql")
}

func (s *Store) OathOption(ctx context.Context, characterID int64, coreInstanceKey string) (OathOptionState, bool, error) {
	if characterID <= 0 || coreInstanceKey == "" || len(coreInstanceKey) > 256 {
		return OathOptionState{}, false, errors.New("invalid oath option key")
	}
	row, err := s.queries.OathOption(ctx, sqlcgen.OathOptionParams{CharacterID: characterID, CoreInstanceKey: coreInstanceKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return OathOptionState{}, false, nil
	}
	if err != nil {
		return OathOptionState{}, false, err
	}
	return OathOptionState{CharacterID: row.CharacterID, CoreInstanceKey: row.CoreInstanceKey, SelectedOption: int(row.SelectedOption), Revision: row.Revision}, true, nil
}

// SaveOathOption performs an insert at expectedRevision=0 or an optimistic
// update at the exact current revision. The row lock covers the decision, so
// stale concurrent writes cannot overwrite newer state.
func (s *Store) SaveOathOption(ctx context.Context, characterID int64, coreInstanceKey string, selectedOption int, expectedRevision int64) (OathOptionState, error) {
	if characterID <= 0 || coreInstanceKey == "" || len(coreInstanceKey) > 256 || selectedOption < 0 || expectedRevision < 0 {
		return OathOptionState{}, errors.New("invalid oath option write")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return OathOptionState{}, err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if _, err = queries.LockActiveCharacterID(ctx, characterID); err != nil {
		return OathOptionState{}, err
	}
	current, err := queries.LockOathOptionRevision(ctx, sqlcgen.LockOathOptionRevisionParams{CharacterID: characterID, CoreInstanceKey: coreInstanceKey})
	if errors.Is(err, pgx.ErrNoRows) {
		if expectedRevision != 0 {
			return OathOptionState{}, fmt.Errorf("%w: expected %d, row absent", ErrOathOptionRevisionConflict, expectedRevision)
		}
		current = 1
		err = queries.InsertOathOption(ctx, sqlcgen.InsertOathOptionParams{CharacterID: characterID, CoreInstanceKey: coreInstanceKey, SelectedOption: int64(selectedOption), Revision: current})
	} else if err == nil {
		if current != expectedRevision {
			return OathOptionState{}, fmt.Errorf("%w: expected %d, current %d", ErrOathOptionRevisionConflict, expectedRevision, current)
		}
		current++
		err = queries.UpdateOathOption(ctx, sqlcgen.UpdateOathOptionParams{CharacterID: characterID, CoreInstanceKey: coreInstanceKey, SelectedOption: int64(selectedOption), Revision: current})
	}
	if err != nil {
		return OathOptionState{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return OathOptionState{}, err
	}
	return OathOptionState{CharacterID: characterID, CoreInstanceKey: coreInstanceKey, SelectedOption: selectedOption, Revision: current}, nil
}
