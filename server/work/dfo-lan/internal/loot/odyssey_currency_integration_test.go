package loot_test

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/testfixture"
	"dfolan/internal/workflow"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestOdysseyCurrencyPickupDatabase(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("isolated PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dbFixture, e := database.OpenTestFixture(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := dbFixture.Close(); err != nil {
			t.Error(err)
		}
	}()
	store := dbFixture.Storage()
	for _, f := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if e = f(ctx); e != nil {
			t.Fatal(e)
		}
	}
	account, e := store.DevelopmentAccount(ctx, "coin-fixture")
	if e != nil {
		t.Fatal(e)
	}
	c, e := catalog.LoadLoot(testfixture.LootLevel150Path(t))
	if e != nil {
		t.Fatal(e)
	}
	role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "CoinFixture", Request: []byte{}, ConfigVersion: c.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 24)
	if e != nil {
		t.Fatal(e)
	}
	rules, e := loot.LoadRules("../../configs/drop.current36.json")
	if e != nil {
		t.Fatal(e)
	}
	tables, e := loot.Parse(c)
	if e != nil {
		t.Fatal(e)
	}
	coins, e := loot.LoadOdysseyCurrency("../../configs/odyssey-currency.json")
	if e != nil {
		t.Fatal(e)
	}
	bagRules, e := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if e != nil {
		t.Fatal(e)
	}
	d := &dungeon.Session{RunID: "0123456789abcdef0123456789abcdef", Loaded: true, Definition: catalog.DungeonDefinition{Odyssey: true}, Room: catalog.DungeonRoom{Map: 1}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Level: 115, Rank: 3}}, Dead: map[uint16]bool{4096: true}, NextEntity: 4097}
	session := loot.NewSession(c, tables, rules, nil, d.RunID, role.AccountID, role.ID, role.WireID)
	session.Currency = coins
	if _, e = session.Death(d, 4096); e != nil {
		t.Fatal(e)
	}
	var object uint32
	for id, drop := range session.Objects {
		if drop.Award.Template == 10418035 {
			object = id
		}
	}
	if object == 0 {
		t.Fatal("no source currency object")
	}
	domain := loot.Service{Catalog: c, Rules: rules, Tables: tables, BagRules: bagRules, Currency: coins}
	s := workflow.LootService{Store: store, Loot: &domain}
	request := protocol.PickupRequest{Object: object}
	next, _, applied, e := s.Pickup(ctx, role, session, d, request)
	if e != nil || !applied {
		t.Fatal(applied, e)
	}
	replay, _, applied, e := s.Pickup(ctx, role, session, d, request)
	if e != nil || applied {
		t.Fatal("pickup replay", applied, e)
	}
	for _, r := range []database.Character{next, replay} {
		b, e := inventory.ReadBag(r.State)
		if e != nil || len(b.Items) != 1 || b.Items[0].Template != 10418035 || b.Items[0].Amount != 1 {
			t.Fatal("coin ledger", b, e)
		}
	}
	other := role
	other.AccountID++
	if _, _, _, e = s.Pickup(ctx, other, session, d, request); e == nil {
		t.Fatal("cross account pickup")
	}
	d.Room.Map++
	if _, _, _, e = s.Pickup(ctx, role, session, d, request); e == nil {
		t.Fatal("cross map pickup")
	}
	t.Log("DB PASS boss coin pickup once; stale retransmission does not duplicate; cross-account/map denied")
}
