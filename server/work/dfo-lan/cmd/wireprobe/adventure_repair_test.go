package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"dfolan/internal/adventure"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func adventureTestWorld(t *testing.T) (*worldSession, *database.TestFixture) {
	t.Helper()
	ctx := context.Background()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	db := f.Storage()
	if err := db.MigrateAdventure(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := db.DevelopmentAccount(ctx, "adventure-repair")
	if err != nil {
		t.Fatal(err)
	}
	role, err := db.CreateCharacter(ctx, database.Character{AccountID: account, Name: "First", Profession: 0,
		ConfigVersion: strings.Repeat("a", 64), Request: []byte{},
		State: json.RawMessage(`{"level":100,"advancement":1,"awakening":2,"future_field":{"keep":true}}`)}, 100)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 27
	return &worldSession{account: account, role: role, store: db,
		characters: &character.Service{Store: db, ChannelContext: [2]byte{1, 10}},
		fatigue:    &character.FatigueService{Location: time.UTC}, adventureReady: true}, f
}

func adventurePacket(packets []outboundPacket, id uint16) []byte {
	for _, p := range packets {
		if p.ID == id {
			return p.Payload
		}
	}
	return nil
}

func TestAdventureRefreshTracksRosterChanges(t *testing.T) {
	w, f := adventureTestWorld(t)
	ctx := context.Background()
	before, err := w.refreshAdventure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(adventurePacket(before, 1331)) == 0 {
		t.Fatal("missing initial account details")
	}
	// Do not change the account profile or earned experience. A profession's
	// level/awakening can change independently, including at adventure level cap.
	if err := f.SeedCharacterState(ctx, w.role.ID, json.RawMessage(`{"level":110,"advancement":1,"awakening":3,"future_field":{"keep":true}}`)); err != nil {
		t.Fatal(err)
	}
	after, err := w.refreshAdventure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if body := adventurePacket(after, 1331); len(body) == 0 || bytes.Equal(body, adventurePacket(before, 1331)) {
		t.Fatal("profession collection/main character stayed stale after level and awakening changed")
	}
	unchanged, err := w.refreshAdventure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(adventurePacket(unchanged, 1331)) != 0 {
		t.Fatal("unchanged roster repeatedly rebuilds the window")
	}
}

func TestAdventureEliteHolesKeepSkillRowsAligned(t *testing.T) {
	profile := database.AccountAdventure{Data: database.AdventureData{
		EliteSelections: map[uint16][3]int64{2: {10, 0, 30}},
		EliteSkillUsage: map[uint16]map[int64][30]int32{2: {10: {101}, 30: {303}}},
	}}
	roles := []database.Character{{ID: 30}, {ID: 10}}
	body, err := adventureElitePayloadForRoles(profile, roles)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 532 || body[0] != 1 {
		t.Fatal("unexpected selection payload")
	}
	row := body[1:]
	// NOTI1382 contains a compact list. NOTI1879 consumes skill row (i+1),
	// so the second companion must occupy the second settings/skill row.
	if binary.LittleEndian.Uint32(row[0x27:]) != 1 || binary.LittleEndian.Uint32(row[0x2b:]) != 0 ||
		binary.LittleEndian.Uint32(row[0x2f:]) != ^uint32(0) ||
		binary.LittleEndian.Uint32(row[0x33+240:]) != 303 {
		t.Fatal("sparse selection points at a different companion's skill row")
	}
	if profile.Data.EliteSelections[2] != [3]int64{10, 0, 30} {
		t.Fatal("projection rewrote saved preferences")
	}
}

func TestAdventureMainCharacterCanReturnToEarlierChoice(t *testing.T) {
	w, _ := adventureTestWorld(t)
	ctx := context.Background()
	other, err := w.store.CreateCharacter(ctx, database.Character{AccountID: w.account, Name: "Other", Profession: 1,
		ConfigVersion: w.role.ConfigVersion, Request: []byte{}, State: json.RawMessage(`{"level":100,"advancement":1,"awakening":2}`)}, 100)
	if err != nil {
		t.Fatal(err)
	}
	request := func(prof byte) []byte {
		p := make([]byte, 12)
		p[0], p[1], p[2] = prof, 1, 2
		binary.LittleEndian.PutUint32(p[4:], 100)
		binary.LittleEndian.PutUint32(p[8:], 1)
		return p
	}
	for _, id := range []int64{w.role.ID, other.ID, w.role.ID, other.ID} {
		prof := byte(0)
		if id == other.ID {
			prof = 1
		}
		p := request(prof)
		if _, err := w.setAdventureBestHonor(ctx, p, p, "same-session"); err != nil {
			t.Fatal(err)
		}
		info, err := w.adventureInfo(ctx, w.role.ID)
		if err != nil {
			t.Fatal(err)
		}
		if info.BestHonor == nil || int64(info.BestHonor.CharacterID) != id {
			t.Fatalf("A/B/A/B setting reused an obsolete receipt: want %d, got %+v", id, info.BestHonor)
		}
	}
}

func TestAdventureShopAtomicDeliveryReplayAndRefusals(t *testing.T) {
	w, f := adventureTestWorld(t)
	ctx := context.Background()
	rules, err := adventure.NewRules(adventure.Rules{SourceChecksum: w.role.ConfigVersion, MaxLevel: 3, ExpRate: 0.3,
		Experience: map[uint32]uint64{1: 0, 2: 6, 3: 9},
		Shops: map[byte]adventure.Shop{0: {MaxPoints: 100, Items: []adventure.ShopItem{
			{Template: 42, Level: 1, Price: 2, Limit: 6}, {Template: 43, Level: 2, Price: 1, Limit: 5},
		}}}, Items: map[uint32]adventure.Item{
			42: {Path: "fixture/42.stk", Type: "[throw]", Limit: 3, UsagePeriod: []int64{1}},
			43: {Path: "fixture/43.stk", Type: "[throw]", Limit: 3},
		}})
	if err != nil {
		t.Fatal(err)
	}
	restore, err := adventure.InstallRules(rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	w.loot = &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: w.role.ConfigVersion}},
		BagRules: inventory.BagRules{Source: w.role.ConfigVersion, Slots: map[string][2]uint16{"[throw]": {65, 65}}, MissingStackLimit: 3}}
	if _, err := w.prepareAdventure(ctx); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = w.store.CommitAdventure(ctx, w.account, w.role.ID, "seed-points", func(role database.Character, p *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		p.Data.Points[0] = 10
		return role.State, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := func(template, count uint32) []byte {
		p := make([]byte, 9)
		binary.LittleEndian.PutUint32(p[1:], template)
		binary.LittleEndian.PutUint32(p[5:], count)
		return p
	}
	p := request(42, 2)
	for i := 0; i < 2; i++ {
		packets, err := w.buyAdventureItem(ctx, p, p, "purchase-session")
		if err != nil || !bytes.Equal(adventurePacket(packets, 1406), []byte{1}) || len(adventurePacket(packets, 13)) == 0 || len(adventurePacket(packets, 1331)) == 0 {
			t.Fatal("purchase/replayed receipt did not restore actual inventory and account", err)
		}
	}
	profile, err := w.store.LoadAdventure(ctx, w.account, w.role.ID, "unused")
	if err != nil || profile.Data.Points[0] != 6 || profile.Data.Purchases[42] != 2 {
		t.Fatal("replay deducted points or limit twice", profile, err)
	}
	state, err := f.CharacterState(ctx, w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(state)
	if err != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 2 || bag.Items[0].ExpireTime != inventory.GrantExpireTime || !bytes.Contains(state, []byte(`"future_field":{"keep":true}`)) {
		t.Fatal("delivery quantity/period/preserved fields incorrect", bag, err)
	}
	// Two units would partially fill the stack, then need another slot. Failure
	// must roll back that partial fill together with points and purchase count.
	for _, test := range []struct {
		name        string
		item, count uint32
		want        error
	}{{"full-after-partial-stack", 42, 2, inventory.ErrMailBagFull}, {"insufficient-points", 42, 4, errAdventurePoints},
		{"over-limit", 42, 7, errAdventureLimit}, {"cumulative-limit", 42, 5, errAdventureLimit}, {"level", 43, 1, errAdventureLevel}} {
		payload := request(test.item, test.count)
		_, err := w.buyAdventureItem(ctx, payload, append(append([]byte(nil), payload...), test.name...), "purchase-session")
		if !errors.Is(err, test.want) {
			t.Fatalf("%s: want %v, got %v", test.name, test.want, err)
		}
		after, err := f.CharacterState(ctx, w.role.ID)
		if err != nil || !bytes.Equal(after, state) {
			t.Fatal("failed delivery changed character", test.name, err)
		}
		afterProfile, err := w.store.LoadAdventure(ctx, w.account, w.role.ID, "unused")
		if err != nil || !reflect.DeepEqual(afterProfile, profile) {
			t.Fatal("failed delivery changed account", test.name, err)
		}
	}
	reopened, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := reopened.LoadAdventure(ctx, w.account, w.role.ID, "unused")
	if err != nil || !reflect.DeepEqual(persisted, profile) {
		t.Fatal("shop account state did not survive reopen", err)
	}
}

func TestAdventureEliteSelectionCanBeClearedAndRestored(t *testing.T) {
	w, f := adventureTestWorld(t)
	ctx := context.Background()
	other, err := w.store.CreateCharacter(ctx, database.Character{AccountID: w.account, Name: "Companion", Profession: 1,
		ConfigVersion: w.role.ConfigVersion, Request: []byte{}, State: json.RawMessage(`{"level":100,"advancement":1,"awakening":2}`)}, 100)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		t.Fatal(err)
	}
	var otherSlot, activeSlot int32
	for i, role := range roles {
		if role.ID == other.ID {
			otherSlot = int32(i)
		}
		if role.ID == w.role.ID {
			activeSlot = int32(i)
		}
	}
	request := func(slot int32) []byte {
		p := make([]byte, 531)
		binary.LittleEndian.PutUint16(p[0x0d:], 2)
		for i, value := range [3]int32{slot, -1, -1} {
			binary.LittleEndian.PutUint32(p[0x27+i*4:], uint32(value))
		}
		return p
	}
	for _, slot := range []int32{otherSlot, -1, otherSlot, -1, otherSlot} {
		p := request(slot)
		packets, err := w.setAdventureElite(ctx, p, p, "same-session")
		if err != nil || !bytes.Equal(adventurePacket(packets, 1719), []byte{1}) {
			t.Fatal("save elite", err)
		}
		profile, err := w.store.LoadAdventure(ctx, w.account, w.role.ID, "unused")
		want := int64(0)
		if slot >= 0 {
			want = other.ID
		}
		if err != nil || profile.Data.EliteSelections[2] != [3]int64{want, 0, 0} {
			t.Fatal("repeated selection reused stale receipt", profile, err)
		}
	}
	p := request(activeSlot)
	if _, err := w.setAdventureElite(ctx, p, p, "same-session"); err == nil {
		t.Fatal("current character accepted as own companion")
	}
	reopened, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	profile, err := reopened.LoadAdventure(ctx, w.account, w.role.ID, "unused")
	if err != nil || profile.Data.EliteSelections[2] != [3]int64{other.ID, 0, 0} {
		t.Fatal("failed selection or reopen lost saved companion", profile, err)
	}
	// A fresh login can open the adventure window before sending CMD35.
	// The periodic refresh is gated on movement, so initialization must
	// independently restore the persisted elite settings.
	fresh := *w
	fresh.store = reopened
	fresh.adventureReady = false
	fresh.adventureIdentitySent = false
	fresh.adventureEliteSnapshot = [32]byte{}
	packets, err := fresh.adventureIdentityPlan(ctx)
	want, wantErr := adventureElitePayloadForRoles(profile, roles, w.role.ID)
	if err != nil || wantErr != nil || !bytes.Equal(adventurePacket(packets, 1754), want) {
		t.Fatal("relogin before movement omitted elite restoration", err, wantErr)
	}
	// Exercise the real own-account query dispatch, not only the plan builder.
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	keys := make([]byte, wire.SessionKeyBytes)
	log := func(map[string]any) {}
	client := &gameConnection{worldState: &fresh, bootstrapped: true,
		selectedCharacterID: fresh.role.ID, event: log,
		output: newConnectionOutput(server, keys, "elite-relogin", log)}
	query := make([]byte, 6)
	binary.LittleEndian.PutUint16(query, fresh.role.WireID)
	result := make(chan dispatchAction, 1)
	go func() {
		result <- client.dispatchAccountQueries(&clientRequest{frame: wire.Frame{Type: 1, ID: 1395}, plaintext: query, verified: true})
	}()
	for _, id := range []uint16{1395, 1363, 1760, 1754, 2858, 2799} {
		frame := readDispatchServerFrame(t, peer)
		if binary.LittleEndian.Uint16(frame[1:3]) != id {
			t.Fatalf("relogin restore packet order: want %d, got %x", id, frame[:3])
		}
		if id == 1754 {
			body, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
			if err != nil || !bytes.HasPrefix(body, want) {
				t.Fatal("relogin dispatch lost saved companion", err)
			}
		}
	}
	if <-result != dispatchHandled || !fresh.adventureIdentitySent || fresh.adventureEliteSnapshot != sha256.Sum256(want) || fresh.adventureReady {
		t.Fatal("restoration depended on movement or failed to track successful delivery")
	}
	if fresh.seasonLevelSnapshot != sha256.Sum256(adventurePacket(packets, 2799)) || fresh.seasonOathSnapshot != sha256.Sum256(adventurePacket(packets, 2858)) {
		t.Fatal("season initialization did not track successful delivery")
	}
	// Switching to the selected companion must project it out without deleting
	// the saved preference, and deleted characters cannot shift skill ownership.
	profile.Data.EliteSelections[2] = [3]int64{99999, other.ID, w.role.ID}
	profile.Data.EliteSkillUsage[2] = map[int64][30]int32{w.role.ID: {101}, other.ID: {202}}
	body, err := adventureElitePayloadForRoles(profile, roles, other.ID)
	if err != nil || int32(binary.LittleEndian.Uint32(body[1+0x27:])) != activeSlot ||
		binary.LittleEndian.Uint32(body[1+0x33+120:]) != 101 || binary.LittleEndian.Uint32(body[1+0x2b:]) != ^uint32(0) {
		t.Fatal("active/deleted companion projection misaligned", err)
	}
}

func TestAdventureProfessionCollectionUsesCurrentAccountRepresentatives(t *testing.T) {
	w, _ := adventureTestWorld(t)
	role := func(id int64, prof byte, state string) database.Character {
		return database.Character{ID: id, AccountID: w.account, Name: "Roster", Profession: prof, State: json.RawMessage(state)}
	}
	roles := []database.Character{
		role(50, 9, `{"level":100}`), role(40, 0, `{"level":115,"advancement":1,"awakening":2}`),
		role(30, 0, `{"level":110,"advancement":1,"awakening":3}`), role(20, 0, `{"level":110,"advancement":1,"awakening":3}`),
		role(10, 1, `{"level":115}`), role(60, 10, `{"level":105}`),
	}
	before := append([]database.Character(nil), roles...)
	profile := database.AccountAdventure{Level: 1, Name: "Saved guild", Data: database.AdventureData{BestHonorCharacter: 40}}
	info, err := w.adventureInfoForRoles(profile, roles)
	if err != nil || info.CharacterCount != 6 || len(info.Characters) != 3 || info.Characters[0].CharacterID != 20 ||
		info.Characters[1].Advancement != 1 || info.Characters[2].Advancement != 1 || info.BestHonor.CharacterID != 40 ||
		!reflect.DeepEqual(roles, before) {
		t.Fatal("profession representative/manual main/order incorrect", info, err)
	}
	// Removed manual main falls back to the actual live roster, rather than
	// showing a deleted character or registering an unadvanced profession.
	roles = append(roles[:1:1], roles[2:]...)
	info, err = w.adventureInfoForRoles(profile, roles)
	if err != nil || info.CharacterCount != 5 || info.BestHonor.CharacterID != 20 {
		t.Fatal("deleted main did not fall back to current collection", info, err)
	}
}
