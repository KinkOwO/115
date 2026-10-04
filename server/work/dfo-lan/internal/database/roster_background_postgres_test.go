package database

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"dfolan/internal/character"
)

func TestRosterBackgroundTransactionUsesOwningAccount(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func(context.Context) error{s.Migrate, s.MigrateCharacterEvents, s.MigrateRosterBackgrounds} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "background")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	var roles []Character
	for _, name := range []string{"BackgroundA", "BackgroundB"} {
		role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: name, Request: []byte{0},
			ConfigVersion: version, State: json.RawMessage(`{"unknown":"keep"}`)}, 24)
		if err != nil {
			t.Fatal(err)
		}
		roles = append(roles, role)
	}
	now := time.Now()
	background := character.RosterBackground{Category: 1, ID: 7}
	grant := character.RosterBackgroundUnlock{RosterBackground: background, ExpiresAt: uint32(now.Unix() + 3600)}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, role := range roles {
		wg.Add(1)
		go func(role Character) {
			defer wg.Done()
			_, _, err := s.CommitCharacterEventTx(ctx, account, role.ID, version, "background:unlock", "background-v1",
				func(tx *Tx, current Character) (json.RawMessage, json.RawMessage, error) {
					if err := tx.UnlockRosterBackground(ctx, grant, now); err != nil {
						return nil, nil, err
					}
					return current.State, json.RawMessage(`{"unlocked":true}`), nil
				})
			results <- err
		}(role)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if err.Error() != "该背景已经解锁，未消耗道具" {
			t.Error(err)
		}
	}
	if success != 1 {
		t.Fatalf("duplicate account unlocks: %d", success)
	}
	state, err := s.SelectRosterBackground(ctx, account, 4, background)
	if err != nil || state.Selected[4] != background || len(state.Owned) != 1 || state.Owned[0].ExpiresAt != grant.ExpiresAt {
		t.Fatalf("selection or expiry changed: %+v %v", state, err)
	}
	// Expired authorization falls back in the read view without erasing the
	// stored selection; a later authorized renewal can restore it.
	if _, err := s.db.Exec(ctx, `UPDATE account_roster_background_unlocks SET expires_at=1 WHERE account_id=$1`, account); err != nil {
		t.Fatal(err)
	}
	state, err = s.RosterBackgrounds(ctx, account)
	if err != nil || len(state.Owned) != 0 || state.Selected[4] == background {
		t.Fatalf("expired grant remains active: %+v %v", state, err)
	}
	var stored int
	if err := s.db.QueryRow(ctx, `SELECT background_id FROM account_roster_backgrounds WHERE account_id=$1 AND page=4`, account).Scan(&stored); err != nil || stored != int(background.ID) {
		t.Fatalf("expiry read rewrote stored selection: %d %v", stored, err)
	}
	grant.ExpiresAt = 0
	_, applied, err := s.CommitCharacterEventTx(ctx, account, roles[0].ID, version, "background:renew", "background-v1",
		func(tx *Tx, current Character) (json.RawMessage, json.RawMessage, error) {
			if err := tx.UnlockRosterBackground(ctx, grant, now); err != nil {
				return nil, nil, err
			}
			return current.State, json.RawMessage(`{"renewed":true}`), nil
		})
	if err != nil || !applied {
		t.Fatalf("expired grant renewal: %v %v", applied, err)
	}
	state, err = s.RosterBackgrounds(ctx, account)
	if err != nil || state.Selected[4] != background || len(state.Owned) != 1 || state.Owned[0].ExpiresAt != 0 {
		t.Fatalf("renewal: %+v %v", state, err)
	}
}
