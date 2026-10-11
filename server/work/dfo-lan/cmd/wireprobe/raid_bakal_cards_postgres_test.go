package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBakalNativeCardsPostgresGatewayPlans(t *testing.T) {
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
	account, err := store.DevelopmentAccount(ctx, "native-card-gateway")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := inventory.SaveBag(json.RawMessage(`{"keep":4711}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 555})
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("f", 64)
	source := pvf.ArchiveSnapshot{Checksum: hash}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "CardOwner", ConfigVersion: source.SaveIdentity(), State: raw, Request: []byte{0, 255}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 9
	w := &worldSession{role: role, store: store, loot: &loot.Service{Catalog: catalog.LootCatalog{Source: source, Items: map[uint32]catalog.LootItem{21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 22: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, BagRules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}}}
	rules := &catalog.BakalRaidRules{Mode: catalog.BakalHard, RewardCardSlots: 12, MinimumClearCount: 3, WeeklyRewardCount: 1, Rewards: []catalog.BakalRewardEntry{{Kind: "party_card", Template: 21, Weight: 1}, {Kind: "squad_item", Template: 22, Weight: 1}}}
	saved, p, err := w.bakalRewardService().Freeze(ctx, w.role, rules, "gateway-native", 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	w.role = saved
	w.bakalCardPlan = &p
	w.bakalSettledRun = p.Run
	packets, err := w.bakalCardSelectionPlan(protocol.RaidCardSelection115{Kind: 1, Index: 3})
	if err != nil {
		t.Fatal(err)
	}
	ids := func(p []outboundPacket) []uint16 {
		out := make([]uint16, 0, len(p))
		for _, packet := range p {
			out = append(out, packet.ID)
		}
		return out
	}
	if !reflect.DeepEqual(ids(packets), []uint16{13}) {
		t.Fatal("finished before both card choices", ids(packets))
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || len(bag.Items) != 1 || bag.Items[0].Template != 22 {
		t.Fatal("wrong first category", bag, err)
	}
	packets, err = w.bakalCardSelectionPlan(protocol.RaidCardSelection115{Kind: 0, Index: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids(packets), []uint16{13, 587, 2302, 574}) {
		t.Fatal("wrong finish order", ids(packets))
	}
	if !reflect.DeepEqual(packets[2].Payload, []byte{0}) || !reflect.DeepEqual(packets[3].Payload, []byte{0, 0}) {
		t.Fatal("wrong native completion bodies")
	}
	before, err := f.CharacterState(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, selection := range []protocol.RaidCardSelection115{{Kind: 1, Index: 3}, {Kind: 0, Index: 5}, {Kind: 2, Index: 0}} {
		if _, err = w.bakalCardSelectionPlan(selection); err != nil {
			t.Fatal(err)
		}
	}
	after, err := f.CharacterState(ctx, role.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("gateway replay paid again", err)
	}
	if _, err = w.bakalCardSelectionPlan(protocol.RaidCardSelection115{Kind: 1, Index: 4}); err == nil {
		t.Fatal("gateway replay changed choice")
	}
	if _, err = w.bakalCardSelectionPlan(protocol.RaidCardSelection115{Kind: 0, Index: 12}); err == nil {
		t.Fatal("gateway accepted foreign card")
	}
}
