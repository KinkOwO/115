package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBoostChallengeSQLRealEntry(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("explicit isolated database required")
	}
	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()
	fixture, e := database.OpenTestFixture(ctx)
	must115(t, e)
	defer func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	}()
	store := fixture.Storage()
	must115(t, store.Migrate(ctx))
	must115(t, store.MigrateCharacterEvents(ctx))
	must115(t, store.MigrateMailbox(ctx))
	assertBoostChallengeSQL(t, ctx, store)
}

func assertBoostChallengeSQL(t *testing.T, ctx context.Context, store *database.Store) {
	t.Helper()
	a, e := store.DevelopmentAccount(ctx, "boost-challenge")
	must115(t, e)
	version := strings.Repeat("a", 64)
	c := &boostup.Catalog{GoalLevel: 115, Town: 222, Steps: []boostup.Step{{Number: 1, Area: 0, Guide: "normal", Mission: "none", Rewards: []boostup.Reward{{Item: 6001, Count: 1}}}}, Challenges: []boostup.ChallengeDefinition{{Index: 0, UnlockKind: "level", UnlockValue: 115, Kind: "clear endkeeper of order", GuideDungeon: 100005014, Goal: 10, Repeat: 1, UnlockMail: true, ClearMail: true, UnlockRewards: []boostup.Reward{{Item: 6001, Count: 3}}, ClearRewards: []boostup.Reward{{Item: 6001, Count: 5}}}}}
	ls := &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: map[uint32]catalog.LootItem{6001: {ID: 6001, Kind: "stackable", StackableType: "[booster]", StackLimit: 100}}}, BagRules: inventory.BagRules{Source: version, Slots: map[string][2]uint16{"[booster]": {65, 67}}, MissingStackLimit: 100}}
	raw, e := boostup.WriteState(json.RawMessage(`{"level":115,"untouched":42}`), boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 1, Phase: 1}})
	must115(t, e)
	r, e := store.CreateCharacter(ctx, database.Character{AccountID: a, Name: "ChallengeEntry", ConfigVersion: version, Request: []byte{0}, State: raw}, 100)
	must115(t, e)
	w := &worldSession{account: a, role: r, loot: ls, store: store, boostup: c, characters: &character.Service{Store: store}, state: database.WorldState{Position: database.WorldPosition{Town: 222}}}
	graduation := make([]byte, 8)
	binary.LittleEndian.PutUint32(graduation, 662)
	binary.LittleEndian.PutUint32(graduation[4:], 1)
	_, e = w.boostStepRequest(ctx, graduation, 680)
	must115(t, e)
	st, e := boostup.ReadState(w.role.State)
	must115(t, e)
	if st.Challenge == nil || !st.Challenge.Rows[0].Unlocked {
		t.Fatal("real graduation did not enroll")
	}
	request := make([]byte, 12)
	binary.LittleEndian.PutUint32(request, 665)
	plan, e := w.boostStepRequest(ctx, request, 680, []byte("unique-transport-frame-1"))
	must115(t, e)
	// 2722 在 ACK 之前：布局已由 IDA 闭环（见 boostChallengeFrameProven 的注释）。
	if len(plan) != 2 || plan[0].ID != 2722 || plan[1].ID != 680 {
		t.Fatal("status must precede ACK", plan)
	}
	if len(plan[0].Payload) != protocol.BoostChallengeBodyLen {
		t.Fatalf("2722 包体 %d B, want 客户端读取的 %d B", len(plan[0].Payload), protocol.BoostChallengeBodyLen)
	}
	_, e = w.boostStepRequest(ctx, request, 680, []byte("unique-transport-frame-1"))
	must115(t, e)
	if _, e = w.boostStepRequest(ctx, request, 680, []byte("unique-transport-frame-2")); e == nil {
		t.Fatal("second unlock claim accepted")
	}
	mails, e := store.Mailbox(ctx, a, r.ID)
	must115(t, e)
	if len(mails) != 1 {
		t.Fatal("duplicate reward", len(mails))
	}
	var item inventory.MailItem
	must115(t, json.Unmarshal(mails[0].Assets[0].Item, &item))
	if item.Stack == nil || item.Stack.Template != 6001 || item.Stack.Amount != 3 {
		t.Fatal("wrong challenge reward", item)
	}
	binary.LittleEndian.PutUint32(request[8:], 1)
	if _, e = w.boostStepRequest(ctx, request, 680, []byte("unearned-clear")); e == nil {
		t.Fatal("claim fabricated clear progress")
	}
	fresh := boostReloadCharacter(t, ctx, store, a, r.ID)
	st, e = boostup.ReadState(fresh.State)
	must115(t, e)
	if !st.Challenge.Rows[0].UnlockClaimed || st.Challenge.Rows[0].Progress != 0 || st.Challenge.Rows[0].Claims != 0 {
		t.Fatal("claim corrupted progress", st.Challenge)
	}
}
