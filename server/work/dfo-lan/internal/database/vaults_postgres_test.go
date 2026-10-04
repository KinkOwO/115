package database

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSQLCVaultContainersAndSharedTransaction(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func() error{func() error { return s.Migrate(ctx) }, func() error { return s.MigrateVault(ctx) }, func() error { return s.MigrateAccountVault(ctx) }, func() error { return s.MigrateAccountMaterials(ctx) }} {
		if err := migrate(); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "vaults")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "Vaults", Request: []byte{1}, ConfigVersion: version, State: json.RawMessage(`{"unknown":true,"bag":1}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, space := range []byte{2, 45} {
		v, err := s.LoadVault(ctx, account, role.ID, 8, version, space)
		if err != nil || v.Slots != 8 || !sameJSON(t, v.Items, json.RawMessage(`[]`)) {
			t.Fatalf("vault %d: %+v %v", space, v, err)
		}
	}
	if _, err := s.LoadVault(ctx, account+100, role.ID, 8, version, 45); !errors.Is(err, ErrNotFound) {
		t.Fatalf("vault ownership: %v", err)
	}
	if _, err := s.LoadVault(ctx, account, role.ID, 8, version, 12); err == nil {
		t.Fatal("invalid personal container allowed")
	}
	shared, err := s.LoadAccountVault(ctx, account, role.ID)
	if err != nil || shared.Slots != 0 || shared.Gold != 0 || !sameJSON(t, shared.Items, json.RawMessage(`[]`)) {
		t.Fatalf("unopened shared vault: %+v %v", shared, err)
	}
	var n int
	if err := testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM account_vaults`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("login created shared capacity: %d %v", n, err)
	}
	calls := 0
	apply := func(c Character, counts json.RawMessage, v AccountVaultState) (json.RawMessage, json.RawMessage, AccountVaultState, error) {
		calls++
		v.Slots = 8
		v.Gold = 100
		v.Items = json.RawMessage(`[{"unknown_item":true}]`)
		return c.State, json.RawMessage(`{"100":2}`), v, nil
	}
	for i := 0; i < 2; i++ {
		_, _, v, ok, err := s.CommitAccountVault(ctx, account, role.ID, version, "shared-open", 1, apply)
		if err != nil || ok != (i == 0) || v.Slots != 8 || v.Gold != 100 {
			t.Fatalf("shared commit %d: %+v %v %v", i, v, ok, err)
		}
	}
	if calls != 1 {
		t.Fatalf("shared replay callbacks: %d", calls)
	}
	if _, _, _, _, err := s.CommitAccountVault(ctx, account, role.ID, version, "shared-open", 2, apply); err == nil {
		t.Fatal("shared replay operation conflict allowed")
	}
	if _, _, err := s.CommitVaultMove(ctx, account, role.ID, func(c Character, v VaultState) (json.RawMessage, json.RawMessage, error) {
		return c.State, json.RawMessage(`[{"retained":1}]`), nil
	}, 45); err != nil {
		t.Fatal(err)
	}
	shared, err = s.CommitAccountVaultSort(ctx, account, role.ID, func(v AccountVaultState) (json.RawMessage, error) { return json.RawMessage(`[{"sorted":true}]`), nil })
	if err != nil || shared.Gold != 100 || shared.Slots != 8 {
		t.Fatalf("shared sort changed balances: %+v %v", shared, err)
	}
	cross := func(c Character, a AccountVaultState, v VaultState) (json.RawMessage, AccountVaultState, json.RawMessage, error) {
		a.Items = json.RawMessage(`[{"from_secondary":true}]`)
		return c.State, a, json.RawMessage(`[]`), nil
	}
	if _, _, _, ok, err := s.CommitAccountVaultCrossMove(ctx, account, role.ID, version, "shared-cross", 45, cross); err != nil || !ok {
		t.Fatalf("cross move: %v %v", ok, err)
	}
	if _, _, _, ok, err := s.CommitAccountVaultCrossMove(ctx, account, role.ID, version, "shared-cross", 45, func(Character, AccountVaultState, VaultState) (json.RawMessage, AccountVaultState, json.RawMessage, error) {
		t.Error("cross replay called callback")
		return nil, AccountVaultState{}, nil, nil
	}); err != nil || ok {
		t.Fatalf("cross replay: %v %v", ok, err)
	}
	// Fail the ledger after all three state updates; every save must roll back.
	if _, err := testPool(t, s).Exec(ctx, `ALTER TABLE account_vault_events ADD CONSTRAINT reject_fixture CHECK(event_key<>'shared-fail')`); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := s.CommitAccountVault(ctx, account, role.ID, version, "shared-fail", 3, func(c Character, counts json.RawMessage, v AccountVaultState) (json.RawMessage, json.RawMessage, AccountVaultState, error) {
		v.Gold = 1
		return json.RawMessage(`{"unknown":false}`), json.RawMessage(`{}`), v, nil
	}); err == nil {
		t.Fatal("ledger failure committed")
	}
	shared, err = s.LoadAccountVault(ctx, account, role.ID)
	if err != nil || shared.Gold != 100 || !sameJSON(t, shared.Items, json.RawMessage(`[{"from_secondary":true}]`)) {
		t.Fatalf("shared rollback: %+v %v", shared, err)
	}
	if counts, err := s.AccountMaterials(ctx, account); err != nil || !sameJSON(t, counts, json.RawMessage(`{"100":2}`)) {
		t.Fatalf("material rollback: %s %v", counts, err)
	}
	roles, err := s.Characters(ctx, account)
	if err != nil || len(roles) != 1 || !sameJSON(t, roles[0].State, role.State) {
		t.Fatalf("character rollback: %+v %v", roles, err)
	}
	primary, err := s.LoadVault(ctx, account, role.ID, 100, strings.Repeat("b", 64), 2)
	if err != nil || primary.Slots != 8 || primary.ConfigVersion != version || !sameJSON(t, primary.Items, json.RawMessage(`[]`)) {
		t.Fatalf("primary overwritten: %+v %v", primary, err)
	}
	if _, _, _, err := s.CommitVaultCrossMove(ctx, account, role.ID, func(c Character, a, b VaultState) (json.RawMessage, json.RawMessage, json.RawMessage, error) {
		return c.State, json.RawMessage(`[{"first":true}]`), json.RawMessage(`[{"second":true}]`), nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpgradeSecondaryVaultCapacity(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool(t, s).Exec(ctx, `CREATE TABLE character_cargos(character_id bigint,kind integer,slots integer)`); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool(t, s).Exec(ctx, `INSERT INTO character_cargos VALUES($1,45,80)`, role.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.UpgradeSecondaryVaultCapacity(ctx); err != nil {
			t.Fatal(err)
		}
	}
	second, err := s.LoadVault(ctx, account, role.ID, 8, version, 45)
	if err != nil || second.Slots != 80 || !sameJSON(t, second.Items, json.RawMessage(`[{"second":true}]`)) {
		t.Fatalf("legacy capacity erased items: %+v %v", second, err)
	}
}

func TestSQLCCashVaultUpgradeTargetsAndReplay(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func() error{func() error { return s.Migrate(ctx) }, func() error { return s.MigrateGrants(ctx) }, func() error { return s.MigrateVault(ctx) }, func() error { return s.MigrateAccountVault(ctx) }, func() error { return s.MigrateAccountMaterials(ctx) }, func() error { return s.MigrateCashShop(ctx) }} {
		if err := migrate(); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "cash-vault")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "CashVault", Request: []byte{1}, ConfigVersion: version, State: json.RawMessage(`{"unknown":true}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyGrant(ctx, Grant{ID: "cash-vault-funds", AccountID: account, Cera: 100000, Operator: "test", Reason: "fixture"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadVault(ctx, account, role.ID, 200, version, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadVault(ctx, account, role.ID, 8, version, 45); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := s.CommitAccountVault(ctx, account, role.ID, version, "open", 1, func(c Character, m json.RawMessage, v AccountVaultState) (json.RawMessage, json.RawMessage, AccountVaultState, error) {
		v.Slots = 8
		v.Gold = 7
		return c.State, m, v, nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, space := range []byte{0, 45, 12} {
		order := cashFixture()
		order.Key = "fixture-vault-target-" + string(rune('a'+space))
		order.Account = account
		order.Character = role.ID
		order.VaultSpace = space
		calls := 0
		upgrade := func(v VaultState) (VaultState, error) { calls++; v.Slots += 8; return v, nil }
		first, ok, err := s.PurchaseCashVault(ctx, order, upgrade)
		if err != nil || !ok || first.Vault == nil {
			t.Fatalf("purchase space %d: %+v %v %v", space, first, ok, err)
		}
		replay, ok, err := s.PurchaseCashVault(ctx, order, upgrade)
		if err != nil || ok || calls != 1 || replay.After != first.After || replay.Vault.Slots != first.Vault.Slots {
			t.Fatalf("replay space %d: %+v %v %v calls=%d", space, replay, ok, err, calls)
		}
		if resolved, err := s.VaultPurchaseSpace(ctx, account, role.ID, order.Key); err != nil || resolved != space {
			t.Fatalf("target replay: %d %d %v", space, resolved, err)
		}
	}
	if shared, err := s.LoadAccountVault(ctx, account, role.ID); err != nil || shared.Slots != 16 || shared.Gold != 7 {
		t.Fatalf("shared upgrade erased gold: %+v %v", shared, err)
	}
	for _, space := range []byte{2, 45} {
		v, err := s.LoadVault(ctx, account, role.ID, 8, version, space)
		want := uint16(16)
		if space == 2 {
			want = 208
		}
		if err != nil || v.Slots != want {
			t.Fatalf("vault target %d: %+v %v", space, v, err)
		}
	}
	if inventory, err := s.CashInventory(ctx, account, role.ID); err != nil || inventory == nil || len(inventory) != 0 {
		t.Fatalf("upgrade leaked claimable inventory: %+v %v", inventory, err)
	}
	if balance, err := s.AccountCera(ctx, account); err != nil || balance != 100000-3*3180 {
		t.Fatalf("upgrade charged twice: %d %v", balance, err)
	}
}
