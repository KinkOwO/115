package bakalreset

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/savecontract"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResetPreservesPendingRewardsAndUnknownFields(t *testing.T) {
	state := json.RawMessage(`{"inventory":{"gold":123,"items":[{"id":42}]},"unknown":{"x":true},"bakal_raid_rewards":{"week":"2026-10-06T09:00:00Z","clears":1,"rewards":1,"plans":{"old":{"products":[1,2]}},"future":{"keep":7}},"bakal_bidding":{"paid":true}}`)
	next, receipt, changed, err := resetState(database.Character{State: state})
	if err != nil || !changed {
		t.Fatalf("reset: changed=%v err=%v", changed, err)
	}
	var before, after map[string]any
	json.Unmarshal(state, &before)
	json.Unmarshal(next, &after)
	ledger := before["bakal_raid_rewards"].(map[string]any)
	ledger["clears"], ledger["rewards"] = float64(0), float64(0)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("reset changed unrelated save or pending reward data")
	}
	var audit struct{ Before, After json.RawMessage }
	if err := json.Unmarshal(receipt, &audit); err != nil || len(audit.Before) == 0 || len(audit.After) == 0 {
		t.Fatal("old ledger not preserved in audit receipt")
	}
	if _, _, changed, err := resetState(database.Character{State: next}); err != nil || changed {
		t.Fatal("already restored counters changed again")
	}
	for _, raw := range []string{`[]`, `null`, `{"bakal_raid_rewards":null}`, `{"bakal_raid_rewards":{"clears":-1}}`, `{"bakal_raid_rewards":{"rewards":"bad"}}`} {
		if _, _, _, err := resetState(database.Character{State: json.RawMessage(raw)}); err == nil {
			t.Fatalf("malformed save accepted: %s", raw)
		}
	}
}

func TestAccountResetIsScopedAuditedAndPreservesOtherAccountOnSQLite(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "reset.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	state := json.RawMessage(`{"level":115,"inventory":{"gold":123},"bakal_raid_rewards":{"week":"test","clears":1,"rewards":1,"plans":{"pending":{"keep":true}}}}`)
	for _, name := range []string{"probe", "other"} {
		account, err := store.DevelopmentAccount(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreateCharacter(ctx, database.Character{AccountID: account, WireID: 1, Name: name, ConfigVersion: savecontract.Identity(), Request: []byte{0}, State: state}, 8); err != nil {
			t.Fatal(err)
		}
		if name == "probe" {
			if _, err := store.CreateCharacter(ctx, database.Character{AccountID: account, WireID: 2, Name: "ProbeTwo", ConfigVersion: savecontract.Identity(), Request: []byte{0}, State: state}, 8); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := ResetAccount(ctx, store, "missing", true); err == nil {
		t.Fatal("missing account was created or reset")
	}
	if err := ResetAccount(ctx, store, "probe", false); err != nil {
		t.Fatal(err)
	}
	previewAccounts, _ := store.Accounts(ctx)
	for _, a := range previewAccounts {
		roles, err := store.Characters(ctx, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, role := range roles {
			if string(role.State) != string(state) {
				t.Fatal("preview changed stored character data")
			}
		}
	}
	if err := ResetAccount(ctx, store, "probe", true); err != nil {
		t.Fatal(err)
	}
	accounts, _ := store.Accounts(ctx)
	if len(accounts) != 2 {
		t.Fatal("reset created an extra account")
	}
	for _, a := range accounts {
		roles, err := store.Characters(ctx, a.ID)
		if err != nil || len(roles) == 0 {
			t.Fatal(err)
		}
		for _, role := range roles {
			var doc map[string]json.RawMessage
			json.Unmarshal(role.State, &doc)
			var ledger struct{ Clears, Rewards uint32 }
			json.Unmarshal(doc["bakal_raid_rewards"], &ledger)
			if a.Username == "probe" {
				if ledger.Clears != 0 || ledger.Rewards != 0 {
					t.Fatal("target account not restored")
				}
			} else if string(role.State) != string(state) {
				t.Fatal("other account save changed")
			}
		}
		if a.Username == "probe" {
			history, err := store.GrantHistory(ctx, a.ID, 10)
			if err != nil || len(history) != 2 || len(roles) != 2 {
				t.Fatal("reset audit missing", err)
			}
		}
	}
	if err := ResetAccount(ctx, store, "probe", true); err != nil {
		t.Fatal("repeated recovery failed", err)
	}
}
