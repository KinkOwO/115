package main

import (
	"bytes"
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Exercise three durable own-account companions alongside a transient town
// fleet over repeated entry/abandon cycles. This is server-side regression,
// not a claim that the client's AI/animations were played automatically.
func TestTownRobotsAndThreeCompanionsRepeatedEntry(t *testing.T) {
	companionTestRules(t, adventure.EliteContents{2: {100000527}})
	t.Setenv("DFO_ELITE_COMPANION_PILOT", "attunement")
	w, _ := adventureTestWorld(t)
	ctx := context.Background()
	source := strings.Repeat("b", 64)
	w.characters.Catalog = catalog.Characters{Professions: map[byte]catalog.Profession{1: {RawSHA256: source}}}
	raw := json.RawMessage(`{"level":115,"advancement":1,"awakening":2,"source_sha256":"` + source + `","attributes":{"[hp max]":100,"[mp max]":50}}`)
	var ids [3]int64
	for i := range ids {
		r, err := w.store.CreateCharacter(ctx, database.Character{AccountID: w.account, Name: fmt.Sprintf("Companion%d", i), Profession: 1, ConfigVersion: w.role.ConfigVersion, Request: []byte{}, State: raw}, 100)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = r.ID
	}
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		t.Fatal(err)
	}
	req := make([]byte, 531)
	binary.LittleEndian.PutUint16(req[0x0d:], 2)
	var slots []byte
	var snapshots []protocol.TagCharacter
	for i, id := range ids {
		for slot, r := range roles {
			if r.ID == id {
				binary.LittleEndian.PutUint32(req[0x27+i*4:], uint32(slot))
				slots = append(slots, byte(slot))
				snap, e := w.characters.TagCharacterSnapshot(r)
				if e != nil {
					t.Fatal(e)
				}
				snapshots = append(snapshots, snap)
			}
		}
	}
	if _, err = w.setAdventureElite(ctx, req, req, "three-companion-cycles"); err != nil {
		t.Fatal(err)
	}
	roles, err = w.store.Characters(ctx, w.account)
	if err != nil {
		t.Fatal(err)
	}
	want, err := protocol.AdventureEliteCharacterInfo(w.role.WireID, slots, snapshots)
	if err != nil {
		t.Fatal(err)
	}
	town, f := townRobotWorld(t, 20)
	w.hub, w.peer = town.hub, town.peer
	w.channelType = 1
	for cycle := 0; cycle < 10; cycle++ {
		w.hub.publish(w.peer, 1, 38, 0, 500, 250, false)
		town.ensureTownRobots()
		if len(f.bots) != 20 {
			t.Fatal("town population not restored")
		}
		sel := protocol.DungeonSelection{ID: 17, Party: 65535}
		s := &dungeon.Session{Definition: attunementPilotParsedDefinition(t), Room: catalog.DungeonRoom{Map: 81}}
		plan, e := w.dungeonEntryPlan(ctx, "combined_cycle", 16, sel, s)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(adventurePacket(plan, 1382), want) || !bytes.Equal(adventurePacket(plan, 1879), []byte{2, 0}) {
			t.Fatal("town fleet contaminated companion data")
		}
		w.leaveScene()
		f.tick()
		if len(f.bots) != 0 {
			t.Fatal("town actors remained during solo dungeon")
		}
		w.activeDungeon = s
		s.Loaded = true
		if _, e = w.leaveDungeon(); e != nil {
			t.Fatal(e)
		}
		if w.resultSent || w.completionSent || s.Completed() {
			t.Fatal("abandon awarded a clear")
		}
		// The real dispatcher clears active state after it has sent the return plan.
		w.activeDungeon = nil
	}
	after, err := w.store.Characters(ctx, w.account)
	if err != nil || len(after) != len(roles) {
		t.Fatal("actor creation changed saved roster", err)
	}
	for i := range roles {
		if roles[i].ID != after[i].ID || !bytes.Equal(roles[i].State, after[i].State) {
			t.Fatal("companion cycle changed durable equipment/skills")
		}
	}
}
