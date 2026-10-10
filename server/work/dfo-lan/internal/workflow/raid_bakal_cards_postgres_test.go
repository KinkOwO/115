package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestBakalNativeCardsPostgresRollbackReconnectConcurrentReplay(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("explicit disposable PostgreSQL database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	store := f.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	service, fake, rules := nativeBakalCardFixture()
	service.Store = store
	account, err := store.DevelopmentAccount(ctx, "native-card-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "NativeCardFixture", ConfigVersion: fake.role.ConfigVersion, Request: []byte{0, 255}, State: fake.role.State}, 24)
	if err != nil {
		t.Fatal(err)
	}
	frozen, p, err := service.Freeze(ctx, role, rules, "native-card-pg", 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	selected, card, err := service.SelectCard(ctx, frozen, p.Run, 1, 7)
	if err != nil || card.Selection == nil || *card.Selection != 7 {
		t.Fatal(card, err)
	}
	if _, _, err = service.SelectCard(ctx, selected, p.Run, 1, 8); err == nil {
		t.Fatal("persisted choice changed")
	}
	if err = f.SeedInventoryItems(ctx, role.ID, json.RawMessage(`[{"slot":68,"Template":99,"Amount":1},{"slot":69,"Template":99,"Amount":1}]`)); err != nil {
		t.Fatal(err)
	}
	before, err := f.CharacterState(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.ClaimCard(ctx, selected, p.Run, 1); err == nil {
		t.Fatal("full bag granted card")
	}
	after, err := f.CharacterState(ctx, role.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed grant mutated state", err)
	}
	if n, e := f.EventCount(ctx, role.ID); e != nil || n != 2 {
		t.Fatal("failed grant consumed event", n, e)
	}
	if err = f.SeedInventoryItems(ctx, role.ID, json.RawMessage(`[]`)); err != nil {
		t.Fatal(err)
	}
	reopened, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	service.Store = reopened
	roles, err := reopened.Characters(ctx, account)
	if err != nil || len(roles) != 1 {
		t.Fatal(roles, err)
	}
	saved, err := service.Recover(ctx, roles[0])
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	amounts := map[uint32]uint32{}
	for _, item := range bag.Items {
		amounts[item.Template] += item.Amount
	}
	if !reflect.DeepEqual(amounts, map[uint32]uint32{21: 1, 22: 1}) || bag.Gold != 555 || bag.Coin != 6 {
		t.Fatal("wrong recovery", amounts, bag)
	}
	persisted, err := f.CharacterState(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 16)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copyService := *service
			_, _, err := copyService.SelectCard(ctx, frozen, p.Run, 1, 7)
			failures <- err
			_, _, err = copyService.ClaimCard(ctx, frozen, p.Run, 1)
			failures <- err
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	final, err := f.CharacterState(ctx, role.ID)
	if err != nil || !reflect.DeepEqual(persisted, final) {
		t.Fatal("parallel replay paid again", err)
	}
	if n, e := f.EventCount(ctx, role.ID); e != nil || n != 5 {
		t.Fatal("wrong freeze/select/grant count", n, e)
	}
	_, card, err = service.ClaimCard(ctx, saved, p.Run, 1)
	if err != nil || *card.Selection != 7 {
		t.Fatal("recovery lost choice", card, err)
	}
}
