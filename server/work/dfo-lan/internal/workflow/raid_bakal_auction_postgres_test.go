package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"
)

func TestBakalAuctionPostgresConcurrentReplayAndRestart(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("explicit disposable PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	f, e := database.OpenTestFixture(ctx)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := f.Close(); e != nil {
			t.Error(e)
		}
	})
	store := f.Storage()
	runBakalAuctionConcurrentReplayAndRestart(t, ctx, store)
}

func runBakalAuctionConcurrentReplayAndRestart(t *testing.T, ctx context.Context, store *database.Store) {
	t.Helper()
	if e := store.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e := store.MigrateCharacterEvents(ctx); e != nil {
		t.Fatal(e)
	}
	s, fake, r, now := auctionFixture(t, catalog.BakalHard)
	account, e := store.DevelopmentAccount(ctx, "auction-fixture")
	if e != nil {
		t.Fatal(e)
	}
	role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "AuctionFixture", ConfigVersion: fake.role.ConfigVersion, Request: []byte{0, 255}, State: fake.role.State}, 24)
	if e != nil {
		t.Fatal(e)
	}
	clear := BakalRewardPlan{Run: "fixture", Source: role.ConfigVersion, Content: s.Awarder.Catalog.Source.Checksum, Mode: r.Mode, Eligible: true}
	role, _, e = commitEquipmentEvent(ctx, store, role, "bakal-clear:fixture", bakalRewardModel, func(current database.Character) (json.RawMessage, BakalRewardPlan, error) {
		return current.State, clear, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	s.Store = store
	role, p, e := s.Freeze(ctx, role, r, clear.Run, now)
	if e != nil {
		t.Fatal(e)
	}
	role, p, e = s.Open(ctx, role, p.Run, 0, p.ReadyAt)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, e := s.Bid(ctx, role, p.Run, 0, 100, false, p.ReadyAt); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("concurrent bid replay", e)
		}
	}
	role, _, e = s.Bid(ctx, role, p.Run, 0, 100, false, p.ReadyAt)
	if e != nil {
		t.Fatal(e)
	}
	bag, _ := inventory.ReadBag(role.State)
	if bag.Gold != 900 {
		t.Fatal("concurrent bids duplicated charge", bag.Gold)
	}
	// Recreate the service to model process restart; only the saved plan owns
	// the charge and products, independent of its previous in-memory pointer.
	restarted := &BakalAuctionService{Store: store, Awarder: s.Awarder, Boxes: s.Boxes}
	p, e = restarted.Snapshot(role, p.Run)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, e := restarted.Settle(ctx, role, p.Run, 0, p.Deadline.Add(time.Second))
			if e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	role, _, e = restarted.Settle(ctx, role, p.Run, 0, p.Deadline.Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	bag, _ = inventory.ReadBag(role.State)
	if bag.Gold != 900 || len(bag.Items) != 1 || bag.Items[0].Amount != 4 {
		t.Fatal("restart settlement duplicated award", bag)
	}
}
