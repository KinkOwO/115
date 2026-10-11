package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBakalAuctionPostgresCardsGatewayAndResume(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("disposable PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	store := f.Storage()
	runBakalAuctionCardsGatewayAndResume(t, ctx, store)
}

func runBakalAuctionCardsGatewayAndResume(t *testing.T, ctx context.Context, store *database.Store) {
	t.Helper()
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "auction-gateway")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := inventory.SaveBag(json.RawMessage(`{"keep":4711}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 1000})
	hash := strings.Repeat("e", 64)
	source := pvf.ArchiveSnapshot{Checksum: hash}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "AuctionOwner", ConfigVersion: source.SaveIdentity(), State: raw, Request: []byte{0, 255}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 9
	boxes := bundleTestBoxes{11: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Count: 4, Weight: 1}}}}}}
	lootService := &loot.Service{Catalog: catalog.LootCatalog{Source: source, Items: map[uint32]catalog.LootItem{11: {Kind: "stackable", StackableType: "[virtual]"}, 21: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}, BagRules: inventory.BagRules{Source: hash, Slots: map[string][2]uint16{"[etc]": {68, 69}}}, RewardBoxes: boxes}
	rules := &catalog.BakalRaidRules{Mode: catalog.BakalHard, RewardCardSlots: 12, MinimumClearCount: 3, WeeklyRewardCount: 1, Rewards: []catalog.BakalRewardEntry{{Kind: "party_card", Template: 21, Weight: 1}, {Kind: "squad_item", Template: 21, Weight: 1}, {Kind: "bidding_item", Template: 11, Weight: 1}}, Bidding: catalog.BakalBiddingRules{Count: []catalog.BakalWeightedNumber{{Value: 1, Weight: 1}}, WeeklyCount: []catalog.BakalWeightedNumber{{Value: 0, Weight: 1}}, WeeklyGroups: []catalog.BakalWeightedNumber{{Value: 1, Weight: 1}}, WeeklyItems: map[uint32][]catalog.BakalBiddingItem{1: {{Template: 11, Weight: 1}}}, MinimumPrice: 100, IncreaseRatio: 1.05, Time: 8, StartDelay: 0, BreakTime: 3, LowestTime: 5}}
	w := &worldSession{role: role, store: store, loot: lootService, channelType: 82, bakalRules: rules}
	saved, plan, err := w.bakalRewardService().Freeze(ctx, w.role, rules, "auction-gateway", 3, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	w.role = saved
	w.bakalCardPlan = &plan
	w.bakalSettledRun = plan.Run
	for _, kind := range []byte{1, 0} {
		packets, e := w.bakalCardSelectionPlan(protocol.RaidCardSelection115{Kind: kind, Index: 0})
		if e != nil {
			t.Fatal(e)
		}
		for _, p := range packets {
			if p.ID == 574 {
				t.Fatal("cards finalized before auction")
			}
		}
	}
	if w.bakalAuction == nil || w.bakalAuction.Mode != catalog.BakalHard {
		t.Fatal("cards did not create owned hard auction")
	}
	log := func(map[string]any) {}
	keys := make([]byte, wire.SessionKeyBytes)
	conn := &recordingConnection{}
	c := &gameConnection{gatewayRuntime: &gatewayRuntime{}, worldState: w, bootstrapped: true, event: log, output: newConnectionOutput(conn, keys, "auction", log)}
	decoded := func() map[uint16][]byte {
		out := map[uint16][]byte{}
		raw := conn.Bytes()
		for len(raw) > 0 {
			if len(raw) < wire.ServerHeaderSize {
				t.Fatal("short header")
			}
			size := int(binary.LittleEndian.Uint32(raw[3:]))
			if size > len(raw) || size < wire.ServerHeaderSize {
				t.Fatal("invalid frame")
			}
			id := binary.LittleEndian.Uint16(raw[1:])
			body, e := wire.DecryptPayload(keys, id, raw[wire.ServerHeaderSize:size])
			if e != nil {
				t.Fatal(e)
			}
			out[id] = body
			raw = raw[size:]
		}
		conn.Reset()
		return out
	}
	if err = c.tickBakalAuction(time.Now()); err != nil {
		t.Fatal(err)
	}
	started := decoded()
	if len(started) != 3 || len(started[2060]) == 0 || len(started[2061]) == 0 || binary.LittleEndian.Uint32(started[2059]) != 11 {
		t.Fatal("native auction start sequence", started)
	}
	bid := []byte{1, 100, 0, 0, 0}
	if got := c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 1902}, plaintext: bid, verified: true}); got != dispatchHandled {
		t.Fatal("Bakal bid not routed", got)
	}
	ack := decoded()
	if len(ack[1902]) == 0 || ack[1902][0] != 1 || len(ack[13]) == 0 {
		t.Fatal("native bid/wallet missing", ack)
	}
	bag, _ := inventory.ReadBag(w.role.State)
	if bag.Gold != 900 {
		t.Fatal("gateway did not reserve gold", bag.Gold)
	}
	for _, amount := range []uint32{200, 100} {
		request := make([]byte, 5)
		request[0] = 1
		binary.LittleEndian.PutUint32(request[1:], amount)
		if got := c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 1902}, plaintext: request, verified: true}); got != dispatchHandled {
			t.Fatal("bid/replay not handled", got)
		}
		body := decoded()[1902]
		n := int(binary.LittleEndian.Uint32(body[3:]))
		if binary.LittleEndian.Uint32(body[7+n+9:]) != 200 {
			t.Fatal("older bid receipt rewound current UI price", body)
		}
	}
	// A foreign raid channel cannot consume the Bakal bid or its auction clock.
	w.channelType = 86
	if got := c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 1902}, plaintext: bid, verified: true}); got != dispatchNext {
		t.Fatal("foreign channel accepted Bakal bid")
	}
	if err = c.tickBakalAuction(time.Now()); err != nil || conn.Len() != 0 {
		t.Fatal("foreign auction tick", err)
	}
	// Recreate only the session; the database owns the frozen lot and timer.
	restarted := &worldSession{role: w.role, store: store, loot: lootService, channelType: 82}
	c.worldState = restarted
	if err = c.tickBakalAuction(time.Now()); err != nil {
		t.Fatal(err)
	}
	resumed := decoded()
	p := resumed[1902]
	n := int(binary.LittleEndian.Uint32(p[3:]))
	at := 7 + n
	if binary.LittleEndian.Uint32(p[at+9:]) != 200 || binary.LittleEndian.Uint32(p[at+14:]) == 0 {
		t.Fatal("resumed price/timer missing", p)
	}
	if err = c.tickBakalAuction(restarted.bakalAuction.Deadline); err != nil {
		t.Fatal(err)
	}
	settled := decoded()
	for _, id := range []uint16{1902, 13, 2061, 574} {
		if len(settled[id]) == 0 {
			t.Fatal("missing settlement projection", id)
		}
	}
	before := append([]byte(nil), restarted.role.State...)
	if err = c.tickBakalAuction(time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, []byte(restarted.role.State)) || conn.Len() != 0 {
		t.Fatal("finished auction replayed")
	}
	bag, _ = inventory.ReadBag(restarted.role.State)
	if bag.Gold != 800 || len(bag.Items) != 1 || bag.Items[0].Amount != 6 {
		t.Fatal("cards/auction awards duplicated or lost", bag)
	}
}
