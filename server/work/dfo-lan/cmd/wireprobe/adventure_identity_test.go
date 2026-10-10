package main

import (
	"bytes"
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestAdventureIdentitySaveRestoreAndRefusals(t *testing.T) {
	w, _ := adventureTestWorld(t)
	ctx := context.Background()
	r, err := adventure.Current()
	if err != nil {
		t.Fatal(err)
	}
	rules := *r
	rules.Symbols = map[uint32]bool{0: true, 777: true, 888: true}
	restore, err := adventure.InstallRules(&rules)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	// Old saves remain intact and get a one-time native naming prompt.
	_, _, _, err = w.store.CommitAdventure(ctx, w.account, w.role.ID, "seed-identity", func(c database.Character, a *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		a.Experience = 123
		a.Data.Points[2] = 456
		return c.State, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := w.adventureIdentityPlan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := adventurePacket(plan, 1363)
	if len(n) < 3 || n[len(n)-2] != 1 {
		t.Fatal("old default never offers naming")
	}
	name := append([]byte{24, 0, 0, 0}, []byte("一二三四五六七八")...)
	for _, id := range []uint16{1436, 1430, 1430} {
		result, err := w.handleAdventureIdentity(ctx, id, name, name, "session-a")
		if err != nil {
			t.Fatal(id, err)
		}
		// Both native success handlers consume the UTF-8 name after success.
		// A one-byte ACK makes confirmation submit the localized no-name text.
		if !bytes.Equal(adventurePacket(result, id), append([]byte{1}, name...)) {
			t.Fatalf("CMD%d success omitted its name: %x", id, adventurePacket(result, id))
		}
	}
	if _, err := w.handleAdventureIdentity(ctx, 1430, []byte{1, 0, 0, 0, 'X'}, []byte("other"), "session-a"); err == nil {
		t.Fatal("creation became unlimited rename")
	}
	other, err := w.store.DevelopmentAccount(ctx, "identity-other")
	if err != nil {
		t.Fatal(err)
	}
	role, err := w.store.CreateCharacter(ctx, database.Character{AccountID: other, Name: "Other", ConfigVersion: w.role.ConfigVersion, Request: []byte{}, State: json.RawMessage(`{"level":100}`)}, 100)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = w.store.CommitAdventure(ctx, other, role.ID, "duplicate-name", func(c database.Character, a *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		a.Name = "一二三四五六七八"
		a.Data.NameChosen = true
		return c.State, json.RawMessage(`{}`), nil
	})
	if !errors.Is(err, database.ErrAdventureNameTaken) {
		t.Fatal("duplicate committed", err)
	}
	for _, id := range []uint32{777, 888, 777, 0, 777} {
		body := make([]byte, 20)
		body[0], body[14], body[15] = 0xee, 0xab, 0xcd
		binary.LittleEndian.PutUint32(body[16:], id)
		plan, err = w.handleAdventureIdentity(ctx, 1726, body, body, "session-a")
		if err != nil || !bytes.Equal(adventurePacket(plan, 1726), protocol.AdventureSymbolChanged(id)) {
			t.Fatal("A/B/A badge selection", id, err)
		}
	}
	invalid := make([]byte, 20)
	binary.LittleEndian.PutUint32(invalid[16:], ^uint32(0))
	if _, err := w.handleAdventureIdentity(ctx, 1726, invalid, invalid, "session-a"); err == nil {
		t.Fatal("non-source badge accepted")
	}
	// Fresh initialization is reconstructed from the account store, including settings.
	plan, err = w.adventureIdentityPlan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantName, _ := protocol.AdventureNameInfo("一二三四五六七八", false)
	if !bytes.Equal(adventurePacket(plan, 1363), wantName) || !bytes.Equal(adventurePacket(plan, 1760), protocol.AdventureSymbolInfo(777)) {
		t.Fatal("relogin identity did not restore")
	}
	a, err := w.store.LoadAdventure(ctx, w.account, w.role.ID, "unused")
	if err != nil || a.Experience != 123 || a.Data.Points[2] != 456 || !bytes.Contains(w.role.State, []byte("future_field")) {
		t.Fatal("identity changed existing progress", a, err)
	}
	info, err := w.adventureInfo(ctx, w.role.ID)
	if err != nil || info.SymbolID != 777 || info.Name != "一二三四五六七八" {
		t.Fatal("basic info differs from restored identity", info, err)
	}
}

func TestAdventureIdentityRestoresSeasonBeforeMovement(t *testing.T) {
	w, _ := adventureTestWorld(t)
	ctx := context.Background()
	rules, err := adventure.CurrentSeason()
	if err != nil {
		t.Fatal(err)
	}
	// Actual account feedback: stage 19, no milestone reward claimed.
	state := adventure.SeasonState{Season: rules.Season, Experience: 252138, Week: adventure.SeasonWeek(time.Now())}
	_, before, _, err := w.store.CommitAdventure(ctx, w.account, w.role.ID, "seed-season-init", func(c database.Character, a *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		a.Data.SeasonLevel = state
		raw, err := adventure.SaveSeason(c.State, state)
		return raw, json.RawMessage(`{}`), err
	})
	if err != nil {
		t.Fatal(err)
	}
	w.adventureReady = false
	packets, err := w.adventureIdentityPlan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(adventurePacket(packets, 2799), adventure.SeasonLevelHistory(state)) ||
		!bytes.Equal(adventurePacket(packets, 2858), []byte{0, 0, 0, 0}) {
		t.Fatal("opening before CMD35 omitted season experience/reward/acquisition initialization")
	}
	after, err := w.store.LoadAdventure(ctx, w.account, w.role.ID, "unused")
	if err != nil || !reflect.DeepEqual(before.Data.SeasonLevel, after.Data.SeasonLevel) || w.adventureReady {
		t.Fatal("season initialization changed progress or pretended movement", err)
	}
}
