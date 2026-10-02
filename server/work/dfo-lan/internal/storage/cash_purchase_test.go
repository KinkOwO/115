package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func cashFixture() CashOrder {
	return CashOrder{Key: "fixture-order-0001", Account: 1, Character: 1, Source: strings.Repeat("a", 64), Lines: []CashOrderLine{{Product: 3400489, Template: 590722921, Quantity: 1, Units: 1, UnitPrice: 3180}}}
}
func TestCashOrderValidation(t *testing.T) {
	o := cashFixture()
	if n, e := o.Total(); e != nil || n != 3180 {
		t.Fatal(n, e)
	}
	for _, f := range []func(*CashOrder){func(o *CashOrder) { o.Account = 0 }, func(o *CashOrder) { o.Source = "bad" }, func(o *CashOrder) { o.Key = "" }, func(o *CashOrder) { o.Lines[0].Quantity = 0 }, func(o *CashOrder) { o.Lines[0].UnitPrice = 0 }, func(o *CashOrder) { o.Lines[0].UnitPrice = 4294967295 }, func(o *CashOrder) { o.Lines[0].Units = 4294967295; o.Lines[0].Quantity = 2 }} {
		q := cashFixture()
		f(&q)
		if _, e := q.Total(); e == nil {
			t.Fatal("invalid order accepted")
		}
	}
}

func TestCashPurchaseIntegration(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("CASH_INTEGRATION=1 runs isolated PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cfg, e := LoadConfig("../../runtime/storage/local.json")
	if e != nil {
		t.Fatal(e)
	}
	live, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer live.Close()
	schema := fmt.Sprintf("cash_test_%d", time.Now().UnixNano())
	if _, e = live.DB.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, e := live.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	cfg.PostgresSchema = schema
	cfg.MaxConnections = 12
	s, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, fn := range []func(context.Context) error{s.Migrate, s.MigrateGrants, s.MigrateCashShop} {
		if e = fn(ctx); e != nil {
			t.Fatal(e)
		}
	}
	a, e := s.DevelopmentAccount(ctx, "cash-fixture")
	if e != nil {
		t.Fatal(e)
	}
	o := cashFixture()
	c, e := s.CreateCharacter(ctx, Character{AccountID: a, Name: "CashFixture", Request: []byte{0}, ConfigVersion: o.Source, State: []byte(`{}`)}, 24)
	if e != nil {
		t.Fatal(e)
	}
	o.Account = a
	o.Character = c.ID
	setBalance := func(n int64) {
		t.Helper()
		if _, e = s.DB.Exec(ctx, `INSERT INTO account_currency(account_id,cera) VALUES($1,$2) ON CONFLICT(account_id) DO UPDATE SET cera=$2`, a, n); e != nil {
			t.Fatal(e)
		}
	}
	balance := func() uint64 {
		t.Helper()
		n, e := s.AccountCera(ctx, a)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	setBalance(10000)
	r, applied, e := s.PurchaseCash(ctx, o)
	if e != nil || !applied || r.After != 6820 || len(r.Deliveries) != 1 || r.Deliveries[0].Template != 590722921 {
		t.Fatalf("%+v %v %v", r, applied, e)
	}
	r2, applied, e := s.PurchaseCash(ctx, o)
	if e != nil || applied || r2.Deliveries[0].ID != r.Deliveries[0].ID || balance() != 6820 {
		t.Fatal("replay paid again", e)
	}
	q := o
	q.Lines = append([]CashOrderLine(nil), o.Lines...)
	q.Lines[0].Quantity = 2
	if _, _, e = s.PurchaseCash(ctx, q); e == nil {
		t.Fatal("key conflict accepted")
	}
	q = o
	q.Key = "insufficient-0001"
	setBalance(3179)
	if _, _, e = s.PurchaseCash(ctx, q); e == nil || balance() != 3179 {
		t.Fatal("insufficient balance mutated")
	}
	q = o
	q.Key = "bad-owner-000001"
	q.Character = c.ID + 1000
	if _, _, e = s.PurchaseCash(ctx, q); e == nil || balance() != 3179 {
		t.Fatal("ownership failure mutated")
	}
	q = o
	q.Key = "wrong-source-0001"
	q.Source = strings.Repeat("b", 64)
	if _, _, e = s.PurchaseCash(ctx, q); e == nil || balance() != 3179 {
		t.Fatal("source mismatch mutated")
	}
	// Force failure on the second delivery after the first insert and debit.
	if _, e = s.DB.Exec(ctx, `ALTER TABLE cash_inventory ADD CONSTRAINT reject_second CHECK(template<>999)`); e != nil {
		t.Fatal(e)
	}
	q = o
	q.Key = "atomic-failure-001"
	q.Lines = append(append([]CashOrderLine(nil), o.Lines...), CashOrderLine{Product: 2, Template: 999, Quantity: 1, Units: 1, UnitPrice: 1})
	setBalance(10000)
	if _, _, e = s.PurchaseCash(ctx, q); e == nil || balance() != 10000 {
		t.Fatal("partial delivery charged")
	}
	var count int
	if e = s.DB.QueryRow(ctx, `SELECT count(*) FROM cash_orders WHERE order_key=$1`, q.Key).Scan(&count); e != nil || count != 0 {
		t.Fatal("failed order persisted")
	}
	q = o
	q.Key = "concurrent-same-001"
	var wg sync.WaitGroup
	var paid atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, a, e := s.PurchaseCash(ctx, q)
			if e != nil {
				t.Error(e)
			}
			if a {
				paid.Add(1)
			}
		}()
	}
	wg.Wait()
	if paid.Load() != 1 || balance() != 6820 {
		t.Fatal("concurrent replay overcharged", paid.Load(), balance())
	}
	setBalance(3180)
	paid.Store(0)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q := o
			q.Key = fmt.Sprintf("competing-order-%03d", i)
			_, a, e := s.PurchaseCash(ctx, q)
			if a {
				paid.Add(1)
			} else if e == nil {
				t.Error("missing rejection")
			}
		}(i)
	}
	wg.Wait()
	if paid.Load() != 1 || balance() != 0 {
		t.Fatal("overspend", paid.Load(), balance())
	}
	items, e := s.CashInventory(ctx, a, c.ID)
	if e != nil || len(items) != 3 {
		t.Fatal("delivery ledger", len(items), e)
	}
	t.Log("PASS debit+delivery atomic; replay/conflict; insufficient; wrong owner/source; second-insert rollback; 8 concurrent retries; competing purchases; inventory survives reread")
	// Direct delivery must update character state in the same transaction and
	// must not leave a second unclaimed cash-inventory payout behind.
	setBalance(10000)
	q = o
	q.Key = "bag-delivery-0001"
	deliver := func(raw json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":65,"Template":15,"Amount":1}]}}`), nil
	}
	bagReceipt, applied, e := s.PurchaseCashToBag(ctx, q, deliver)
	if e != nil || !applied || balance() != 6820 || len(bagReceipt.CharacterState) == 0 {
		t.Fatalf("bag purchase %+v %v", bagReceipt, e)
	}
	var savedState json.RawMessage
	if e = s.DB.QueryRow(ctx, `SELECT state FROM characters WHERE id=$1`, c.ID).Scan(&savedState); e != nil {
		t.Fatal(e)
	}
	var state map[string]json.RawMessage
	if json.Unmarshal(savedState, &state) != nil || state["inventory"] == nil {
		t.Fatal("bag not persisted")
	}
	var unclaimed int
	if e = s.DB.QueryRow(ctx, `SELECT count(*) FROM cash_inventory WHERE account_id=$1 AND order_key=$2 AND claimed_at IS NULL`, a, q.Key).Scan(&unclaimed); e != nil || unclaimed != 0 {
		t.Fatal("bag item also claimable", e)
	}
	// Simulate subsequent gameplay saving a newer state, then replay purchase.
	if _, e = s.DB.Exec(ctx, `UPDATE characters SET state='{"newer":true}' WHERE id=$1`, c.ID); e != nil {
		t.Fatal(e)
	}
	bagReceipt, applied, e = s.PurchaseCashToBag(ctx, q, func(json.RawMessage) (json.RawMessage, error) { t.Fatal("replay delivery called"); return nil, nil })
	if e != nil || applied || balance() != 6820 || !strings.Contains(string(bagReceipt.CharacterState), "newer") {
		t.Fatal("replay restored stale bag", e)
	}
	q.Key = "bag-full-000001"
	if _, _, e = s.PurchaseCashToBag(ctx, q, func(json.RawMessage) (json.RawMessage, error) { return nil, fmt.Errorf("bag full") }); e == nil || balance() != 6820 {
		t.Fatal("full bag charged")
	}
	// Fail after both the debit and character UPDATE: audit delivery insertion
	// must roll them back, not just undo the new cash-inventory row.
	q.Key = "bag-late-fail-001"
	q.Lines = []CashOrderLine{{Product: 2, Template: 999, Quantity: 1, Units: 1, UnitPrice: 1}}
	if _, _, e = s.PurchaseCashToBag(ctx, q, deliver); e == nil || balance() != 6820 {
		t.Fatal("late bag failure charged")
	}
	if e = s.DB.QueryRow(ctx, `SELECT state FROM characters WHERE id=$1`, c.ID).Scan(&savedState); e != nil || !strings.Contains(string(savedState), "newer") {
		t.Fatal("late failure changed bag", e)
	}
	t.Log("PASS direct bag debit+state+audit; no unclaimed duplicate; replay keeps newer state; full bag and late insert failure roll back")
	if e = s.MigrateVault(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = s.LoadVault(ctx, a, c.ID, 8, o.Source); e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec(ctx, `UPDATE character_vaults SET items='[{"slot":0,"template":14,"amount":5}]' WHERE character_id=$1`, c.ID); e != nil {
		t.Fatal(e)
	}
	upgrade := func(v VaultState) (VaultState, error) {
		if v.Slots != 8 {
			return v, fmt.Errorf("wrong tier")
		}
		v.Slots = 24
		return v, nil
	}
	q = o
	q.Key = "vault-upgrade-0001"
	q.Lines = []CashOrderLine{{Product: 3000129, Template: 50, Quantity: 1, Units: 1, UnitPrice: 30}}
	setBalance(29)
	if _, _, e = s.PurchaseCashVault(ctx, q, upgrade); e == nil || balance() != 29 {
		t.Fatal("insufficient upgrade charged")
	}
	setBalance(100)
	r, applied, e = s.PurchaseCashVault(ctx, q, upgrade)
	if e != nil || !applied || balance() != 70 || r.Vault.Slots != 24 || !strings.Contains(string(r.Vault.Items), "14") {
		t.Fatal("upgrade failed", e)
	}
	if _, applied, e = s.PurchaseCashVault(ctx, q, upgrade); e != nil || applied || balance() != 70 {
		t.Fatal("upgrade replay charged", e)
	}
	q.Key = "vault-wrong-tier-001"
	if _, _, e = s.PurchaseCashVault(ctx, q, upgrade); e == nil || balance() != 70 {
		t.Fatal("wrong tier charged")
	}
	q.Key = "vault-late-fail-001"
	q.Lines = []CashOrderLine{{Product: 2, Template: 999, Quantity: 1, Units: 1, UnitPrice: 1}}
	if _, _, e = s.PurchaseCashVault(ctx, q, func(v VaultState) (VaultState, error) { v.Slots = 40; return v, nil }); e == nil || balance() != 70 {
		t.Fatal("late vault failure charged")
	}
	v, e := s.LoadVault(ctx, a, c.ID, 8, o.Source)
	if e != nil || v.Slots != 24 || !strings.Contains(string(v.Items), "14") {
		t.Fatal("vault persistence/rollback", e)
	}
	if e = s.DB.QueryRow(ctx, `SELECT count(*) FROM cash_inventory WHERE account_id=$1 AND order_key='vault-upgrade-0001' AND claimed_at IS NULL`, a).Scan(&unclaimed); e != nil || unclaimed != 0 {
		t.Fatal("used coupon left claimable", e)
	}
	t.Log("PASS vault atomic debit+capacity+audit; contents retained; replay/wrong tier/insufficient funds/late failure; reconnect and no unclaimed coupon")
}

func TestMigratePackagePlaceholdersUnit(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("CASH_INTEGRATION=1 runs isolated PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := LoadConfig("../../runtime/storage/local.json")
	if err != nil {
		t.Fatal(err)
	}
	live, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	schema := fmt.Sprintf("pkg_mig_%d", time.Now().UnixNano())
	if _, err = live.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = live.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	}()
	cfg.PostgresSchema = schema
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, fn := range []func(context.Context) error{s.Migrate, s.MigrateGrants, s.MigrateCashShop} {
		if err = fn(ctx); err != nil {
			t.Fatal(err)
		}
	}
	acc, err := s.DevelopmentAccount(ctx, "mig-fixture")
	if err != nil {
		t.Fatal(err)
	}
	initialState := `{"inventory":{"version":"ordinary-bag-v1","gold":0,"items":[{"slot":65,"Template":590722921,"Amount":1},{"slot":66,"Template":15,"Amount":10}]}}`
	c, err := s.CreateCharacter(ctx, Character{AccountID: acc, Name: "MigChar", Request: []byte{0}, ConfigVersion: "test", State: []byte(initialState)}, 24)
	if err != nil {
		t.Fatal(err)
	}

	// Run migration
	if err = s.MigrateCashShop(ctx); err != nil {
		t.Fatal(err)
	}

	var savedState json.RawMessage
	if err = s.DB.QueryRow(ctx, `SELECT state FROM characters WHERE id=$1`, c.ID).Scan(&savedState); err != nil {
		t.Fatal(err)
	}
	var stateMap map[string]json.RawMessage
	if err = json.Unmarshal(savedState, &stateMap); err != nil {
		t.Fatal(err)
	}
	type bagItem struct {
		Slot       uint16 `json:"slot"`
		Template   uint32 `json:"Template"`
		Amount     uint32 `json:"Amount"`
		ExpireTime uint32 `json:"expire_time,omitempty"`
	}
	type bagStruct struct {
		Items []bagItem `json:"items"`
	}
	var b bagStruct
	if err = json.Unmarshal(stateMap["inventory"], &b); err != nil {
		t.Fatal(err)
	}

	// 590722921 must be gone
	for _, it := range b.Items {
		if it.Template == 590722921 {
			t.Fatalf("placeholder still in bag: %+v", it)
		}
	}
	// Slot 66 with template 15 must be preserved
	foundPreserved := false
	for _, it := range b.Items {
		if it.Slot == 66 && it.Template == 15 && it.Amount == 10 {
			foundPreserved = true
		}
	}
	if !foundPreserved {
		t.Fatalf("preserved item missing: %+v", b.Items)
	}

	// 6 boxes must be present
	expected := map[uint32]bool{590722922: true, 590722923: true, 590722926: true, 590722927: true, 590722928: true, 590722929: true}
	for _, it := range b.Items {
		if expected[it.Template] {
			if it.ExpireTime != math.MaxInt32 {
				t.Fatalf("sub-item %d ExpireTime=%d, want MaxInt32", it.Template, it.ExpireTime)
			}
			delete(expected, it.Template)
		}
	}
	if len(expected) > 0 {
		t.Fatalf("missing expected sub-items: %+v", expected)
	}
}
