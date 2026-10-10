package main

import (
	"bytes"
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func companionTestRules(t *testing.T, contents adventure.EliteContents) {
	t.Helper()
	baseline, err := adventure.EmbeddedRules()
	if err != nil {
		t.Fatal(err)
	}
	rules := *baseline
	rules.EliteContents = contents
	restore, err := adventure.InstallRules(&rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
}

func TestAdventureCompanionLoadUsesSavedRolesAndSourceDungeon(t *testing.T) {
	companionTestRules(t, adventure.EliteContents{2: {100000527}})
	w, _ := adventureTestWorld(t)
	w.channelType = 73
	ctx := context.Background()
	source := strings.Repeat("b", 64)
	w.characters.Catalog = catalog.Characters{Professions: map[byte]catalog.Profession{
		1: {RawSHA256: source, InitialSkillSlots: map[uint16]uint16{101: 0}},
	}}
	raw := json.RawMessage(`{"level":100,"advancement":1,"awakening":2,"source_sha256":"` + source +
		`","initial_skill_cells":[101,1,1],"attributes":{"[hp max]":100,"[mp max]":50},"future_field":{"keep":true}}`)
	avatar := protocol.OrdinaryItem(1, 769900045, 0)
	binary.LittleEndian.PutUint32(avatar[24:], 760050711)
	pet := protocol.OrdinaryItem(26, 500991107, 1)
	binary.LittleEndian.PutUint32(pet[24:], 1)
	raw, err := inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{
		{Slot: 1, Template: 769900045, Record: avatar[:]},
		{Slot: 26, Template: 500991107, Record: pet[:]},
		{Slot: 36, Template: 100401592},
		{Slot: 47, Template: 100313757},
	}})
	if err != nil {
		t.Fatal(err)
	}
	other, err := w.store.CreateCharacter(ctx, database.Character{AccountID: w.account, Name: "Companion",
		Profession: 1, ConfigVersion: w.role.ConfigVersion, Request: []byte{}, State: raw}, 100)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		t.Fatal(err)
	}
	slot := byte(255)
	for i, role := range roles {
		if role.ID == other.ID {
			slot = byte(i)
		}
	}
	p := make([]byte, 531)
	binary.LittleEndian.PutUint16(p[0x0d:], 2)
	for i, value := range [3]int32{int32(slot), -1, -1} {
		binary.LittleEndian.PutUint32(p[0x27+i*4:], uint32(value))
	}
	binary.LittleEndian.PutUint32(p[0x33+120:], 101)
	if _, err := w.setAdventureElite(ctx, p, p, "companion-source"); err != nil {
		t.Fatal(err)
	}
	before, err := w.store.Characters(ctx, w.account)
	if err != nil {
		t.Fatal(err)
	}
	packets, err := w.adventureEliteForDungeon(ctx, 100000527)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := w.characters.TagCharacterSnapshot(other)
	if err != nil {
		t.Fatal(err)
	}
	want, err := protocol.AdventureEliteCharacterInfo(w.role.WireID, []byte{slot}, []protocol.TagCharacter{snapshot})
	if err != nil || !bytes.Equal(adventurePacket(packets, 1382), want) ||
		!bytes.Equal(adventurePacket(packets, 1879), []byte{2, 0}) {
		t.Fatal("saved companion snapshot/load completion changed", err)
	}
	if len(packets) < 2 || packets[len(packets)-2].ID != 1382 || packets[len(packets)-1].ID != 1879 {
		t.Fatal("AI completion sent before actual character data")
	}
	after, err := w.store.Characters(ctx, w.account)
	if err != nil || len(before) != len(after) {
		t.Fatal("loading companion changed roster", err)
	}
	for i := range before {
		if !bytes.Equal(before[i].State, after[i].State) {
			t.Fatal("loading companion rewrote a character save")
		}
	}
	if _, err := w.adventureEliteForDungeon(ctx, 123); err == nil {
		t.Fatal("unlisted dungeon gained an AI mode")
	}
	w.channelType = 1
	if _, err := w.adventureEliteForDungeon(ctx, 100000527); err == nil {
		t.Fatal("ordinary channel bypassed native container gate")
	}
	// The pilot deliberately extends source-classified Attunement entries;
	// it reuses this exact saved equipment/skill projection and no fake stats.
	t.Setenv("DFO_ELITE_COMPANION_PILOT", "attunement")
	w.selectingDungeon = true
	sel := protocol.DungeonSelection{ID: 17, Party: 65535}
	s := &dungeon.Session{Definition: attunementPilotParsedDefinition(t), Room: catalog.DungeonRoom{Map: 81}}
	pilot, err := w.dungeonEntryPlan(ctx, "pilot_test", 16, sel, s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(adventurePacket(pilot, 1382), want) || !bytes.Equal(adventurePacket(pilot, 1879), []byte{2, 0}) {
		t.Fatal("pilot dropped or substituted the saved equipment/skills")
	}
	var sequence []uint16
	for _, packet := range pilot {
		if packet.ID == 28 || packet.ID == 1382 || packet.ID == 1879 || packet.ID == 29 {
			sequence = append(sequence, packet.ID)
		}
	}
	if len(sequence) != 4 || sequence[0] != 28 || sequence[1] != 1382 || sequence[2] != 1879 || sequence[3] != 29 {
		t.Fatal("pilot snapshot was not bounded by native dungeon info and map start", sequence)
	}
	if _, err := w.loadAdventureElite(ctx, []byte{2, 0}); err == nil {
		t.Fatal("pilot weakened the ordinary C1811 channel/busy boundary")
	}
	s.Definition.Type = "normal"
	ordinary, err := w.dungeonEntryPlan(ctx, "ordinary_test", 16, sel, s)
	if err != nil || len(adventurePacket(ordinary, 1382)) != 0 || len(adventurePacket(ordinary, 1879)) != 0 {
		t.Fatal("ordinary source acquired pilot AI", err)
	}
	afterPilot, err := w.store.Characters(ctx, w.account)
	if err != nil {
		t.Fatal(err)
	}
	for i := range before {
		if !bytes.Equal(before[i].State, afterPilot[i].State) {
			t.Fatal("pilot rewrote a character save")
		}
	}
}

func TestAdventureCompanionRefusesUnsupportedSourceAdapter(t *testing.T) {
	companionTestRules(t, adventure.EliteContents{4: {123}})
	w, _ := adventureTestWorld(t)
	w.channelType = 73
	if _, err := w.adventureEliteForDungeon(context.Background(), 123); err == nil || !strings.Contains(err.Error(), "适配尚未验证") {
		t.Fatal("source alone enabled unverified mode-4 adapter", err)
	}
}

func TestAdventureCompanionRosterCannotChangeDuringBattleOrTransfer(t *testing.T) {
	w, _ := adventureTestWorld(t)
	for name, configure := range map[string]func(*worldSession){
		"dungeon":         func(s *worldSession) { s.activeDungeon = &dungeon.Session{} },
		"selection":       func(s *worldSession) { s.selectingDungeon = true },
		"arrival":         func(s *worldSession) { s.pendingTownArrival = &dungeon.Session{} },
		"special warp":    func(s *worldSession) { s.specialWarpPending = true },
		"black purgatory": func(s *worldSession) { s.blackPurgatory.prepared = true },
	} {
		t.Run(name, func(t *testing.T) {
			copy := *w
			configure(&copy)
			if _, err := copy.setAdventureElite(context.Background(), nil, nil, "battle"); err == nil || !strings.Contains(err.Error(), "挑战或场景切换") {
				t.Fatal("battle/transfer allowed changing the companion roster", err)
			}
		})
	}
}
