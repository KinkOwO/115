package charactercheck

import (
	"context"
	"dfolan/internal/admin"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"fmt"
	"sync"
)

// grantCheck covers operator hand-outs: cera top-ups, gold and items. The
// properties that matter are that a replayed grant pays out once, that a
// balance cannot be driven negative, and that another account can never be
// the target.
func grantCheck(ctx context.Context, s *database.TestFixture, reopened *database.Store, role database.Character, other int64) error {
	if e := s.MigrateGrants(ctx); e != nil {
		return e
	}
	c, e := loadNativeLootCatalog()
	if e != nil {
		return e
	}
	rules, e := inventory.LoadBagRules("configs/inventory.next29.json", c.Source.Checksum)
	if e != nil {
		return e
	}
	gear, e := loadNativeEquipmentCatalog(c.Source.Checksum)
	if e != nil {
		return e
	}
	var stackable uint32
	for id, v := range c.Items {
		// IDs 0/1 are wallet/coin balances, and pet consumables use another
		// container. This assertion exercises a regular bag item grant.
		if id > 1 && v.Kind == "stackable" && !inventory.IsPetConsumable(v.StackableType) && (stackable == 0 || id < stackable) {
			stackable = id
		}
	}
	if stackable == 0 {
		return fmt.Errorf("native catalog has no regular bag stackable fixture")
	}
	service := &admin.Service{Store: s.Storage(), Operator: "check",
		Awarder: &inventory.Awarder{Catalog: c, Rules: rules, Equipment: gear}}

	before, e := inventory.ReadBag(role.State)
	if e != nil {
		return e
	}
	grant := database.Grant{
		ID: "check-grant-1", AccountID: role.AccountID, Character: role.ID,
		Cera: 5000, Gold: 1234, Items: []database.GrantItem{{Template: stackable, Amount: 3}},
		Reason: "isolated check", Operator: "check",
	}
	// Twelve concurrent attempts at the same grant id must pay out exactly once.
	var wg sync.WaitGroup
	applied := make(chan bool, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := service.Apply(ctx, grant)
			if err != nil {
				errs <- err
				return
			}
			applied <- ok
		}()
	}
	wg.Wait()
	close(applied)
	close(errs)
	for err := range errs {
		return fmt.Errorf("concurrent grant failed: %w", err)
	}
	payouts := 0
	for ok := range applied {
		if ok {
			payouts++
		}
	}
	if payouts != 1 {
		return fmt.Errorf("grant paid out %d times, want exactly 1", payouts)
	}

	cera, e := reopened.AccountCera(ctx, role.AccountID)
	if e != nil || cera != 5000 {
		return fmt.Errorf("cera balance is %d after one grant, want 5000: %v", cera, e)
	}
	rows, e := reopened.Characters(ctx, role.AccountID)
	if e != nil {
		return e
	}
	var after inventory.Bag
	for _, r := range rows {
		if r.ID == role.ID {
			if after, e = inventory.ReadBag(r.State); e != nil {
				return e
			}
		}
	}
	if after.Gold != before.Gold+1234 {
		return fmt.Errorf("gold is %d, want %d", after.Gold, before.Gold+1234)
	}
	var held uint32
	var beforeHeld uint32
	for _, row := range before.Items {
		if row.Template == stackable {
			beforeHeld += row.Amount
		}
	}
	for _, row := range after.Items {
		if row.Template == stackable {
			held += row.Amount
		}
	}
	if held != beforeHeld+3 {
		return fmt.Errorf("granted items missing: template=%d before=%d after=%d", stackable, beforeHeld, held)
	}
	// A deduction larger than the balance must refuse the whole grant.
	if _, _, e = service.Apply(ctx, database.Grant{
		ID: "check-grant-overdraw", AccountID: role.AccountID,
		Cera: -999999, Reason: "overdraw", Operator: "check"}); e == nil {
		return fmt.Errorf("cera was driven negative")
	}
	if cera, e = reopened.AccountCera(ctx, role.AccountID); e != nil || cera != 5000 {
		return fmt.Errorf("refused overdraw changed the balance: %d %v", cera, e)
	}

	// A partial deduction works and is audited.
	if _, _, e = service.Apply(ctx, database.Grant{
		ID: "check-grant-spend", AccountID: role.AccountID,
		Cera: -1500, Reason: "spend", Operator: "check"}); e != nil {
		return e
	}
	if cera, e = reopened.AccountCera(ctx, role.AccountID); e != nil || cera != 3500 {
		return fmt.Errorf("cera after deduction is %d, want 3500: %v", cera, e)
	}
	// Another account must not be able to target this character.
	if _, _, e = service.Apply(ctx, database.Grant{
		ID: "check-grant-foreign", AccountID: other, Character: role.ID,
		Gold: 10, Reason: "foreign", Operator: "check"}); e == nil {
		return fmt.Errorf("grant crossed the account boundary")
	}
	// A hand-out must never invent an item the source does not have.
	if _, _, e = service.Apply(ctx, database.Grant{
		ID: "check-grant-unknown-item", AccountID: role.AccountID, Character: role.ID,
		Items:  []database.GrantItem{{Template: 4000000123, Amount: 1}},
		Reason: "unknown", Operator: "check"}); e == nil {
		return fmt.Errorf("granted an item that is absent from the source")
	}
	// Exactly the applied grants are audited. A refused grant rolls its audit
	// row back with the payout, so a failed attempt leaves no trail behind and
	// its id stays reusable — which the next step relies on.
	history, e := reopened.GrantHistory(ctx, role.AccountID, 50)
	if e != nil {
		return e
	}
	if len(history) != 2 {
		return fmt.Errorf("audit trail has %d rows, want exactly the 2 applied grants", len(history))
	}
	// The id a refused grant tried to claim must be free to use again.
	if _, applied, e := service.Apply(ctx, database.Grant{
		ID: "check-grant-overdraw", AccountID: role.AccountID,
		Cera: 10, Reason: "reuse after refusal", Operator: "check"}); e != nil || !applied {
		return fmt.Errorf("a refused grant id was not reusable: applied=%v err=%v", applied, e)
	}
	if cera, e = reopened.AccountCera(ctx, role.AccountID); e != nil || cera != 3510 {
		return fmt.Errorf("cera after reuse is %d, want 3510: %v", cera, e)
	}
	fmt.Println("GRANT_STORAGE_PASS concurrent_payout_once=true cera_floor=true deduction=true ownership=true unknown_item_refused=true audited=true refused_leaves_no_trail=true")
	return nil
}
