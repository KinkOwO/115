package charactercheck

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

func deleteCheck(ctx context.Context, s, reopened *storage.Store, other int64, c catalog.Characters) error {
	account, e := s.DevelopmentAccount(ctx, "temporary-delete")
	if e != nil {
		return e
	}
	cs, e := character.New(s, c, character.Rules{MaxCharacters: 2, InitialLevel: 1})
	if e != nil {
		return e
	}
	a, e := cs.Create(ctx, account, request("DeleteFirst"))
	if e != nil {
		return e
	}
	b, e := cs.Create(ctx, account, request("DeleteSecond"))
	if e != nil {
		return e
	}
	if _, e = s.DeleteCharacter(ctx, other, 0, a.Name); e == nil {
		return fmt.Errorf("cross-owner delete accepted")
	}
	if _, e = s.DeleteCharacter(ctx, account, 0, b.Name); e == nil {
		return fmt.Errorf("wrong name delete accepted")
	}
	id, e := s.DeleteCharacter(ctx, account, 0, a.Name)
	if e != nil || id != a.ID {
		return fmt.Errorf("delete failed: %v", e)
	}
	if _, e = s.DeleteCharacter(ctx, account, 0, a.Name); e == nil {
		return fmt.Errorf("stale delete removed neighbour")
	}
	rows, e := reopened.Characters(ctx, account)
	if e != nil || len(rows) != 1 || rows[0].ID != b.ID {
		return fmt.Errorf("delete roster mismatch: %v", e)
	}
	var archived bool
	var state json.RawMessage
	if e = s.DB.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,state FROM characters WHERE id=$1`, a.ID).Scan(&archived, &state); e != nil || !archived || string(state) == "" {
		return fmt.Errorf("delete lost retained data: %v", e)
	}
	next, e := cs.Create(ctx, account, request("DeleteNew"))
	if e != nil || next.WireID <= b.WireID {
		return fmt.Errorf("delete failed to release capacity or reused ID: %v", e)
	}
	fmt.Println("DELETE_STORAGE_PASS archived=true owner_checked=true stale_slot_refused=true capacity_released=true reopen=true")
	return nil
}
