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
	"reflect"
	"strings"
	"testing"
)

func allEliteWorld(t *testing.T) (*worldSession, database.Character) {
	t.Helper()
	companionTestRules(t, adventure.EliteContents{2: {100000527}})
	w, _ := adventureTestWorld(t)
	source := strings.Repeat("b", 64)
	w.characters.Catalog = catalog.Characters{Professions: map[byte]catalog.Profession{1: {RawSHA256: source, InitialSkillSlots: map[uint16]uint16{101: 0}}}}
	raw := json.RawMessage(`{"level":100,"advancement":1,"awakening":2,"source_sha256":"` + source + `","initial_skill_cells":[101,1,1],"attributes":{"[hp max]":100,"[mp max]":50},"future_field":{"keep":true}}`)
	other, err := w.store.CreateCharacter(context.Background(), database.Character{AccountID: w.account, Name: "EliteAll", Profession: 1, ConfigVersion: w.role.ConfigVersion, Request: []byte{}, State: raw}, 100)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := w.store.Characters(context.Background(), w.account)
	if err != nil {
		t.Fatal(err)
	}
	slot := int32(-1)
	for i, r := range roles {
		if r.ID == other.ID {
			slot = int32(i)
		}
	}
	p := make([]byte, 531)
	binary.LittleEndian.PutUint16(p[0x0d:], 2)
	for i, v := range [3]int32{slot, -1, -1} {
		binary.LittleEndian.PutUint32(p[0x27+i*4:], uint32(v))
	}
	binary.LittleEndian.PutUint32(p[0x33+120:], 101)
	if _, err = w.setAdventureElite(context.Background(), p, p, "all-fixture"); err != nil {
		t.Fatal(err)
	}
	return w, other
}

func TestAllEliteUsesAdmittedDefinitionAndRealSnapshotsAcrossChannels(t *testing.T) {
	t.Setenv("DFO_ELITE_COMPANION_PILOT", "all")
	w, other := allEliteWorld(t)
	before, err := w.store.Characters(context.Background(), w.account)
	if err != nil {
		t.Fatal(err)
	}
	for _, channel := range []uint32{0, 73, 82, 101, 102, 104, 106} {
		w.channelType = channel
		w.channelWorldIsolated = channel != 0
		// IDs and script paths are deliberately outside the old elite source table.
		for _, id := range []uint32{17, 91} {
			s := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: id, Script: catalog.ScriptRecord{Path: "contents/source-test.dgn"}, Type: "normal"}}
			plan := []outboundPacket{{"ack", 1, 16, []byte{1}}, {"info", 0, 28, []byte{28}}, {"linked_stage", 0, 1451, []byte{7}}, {"map", 0, 29, []byte{29}}, {"stage", 0, 2622, []byte{9}}}
			out, err := w.allDungeonEliteEntryPlan(context.Background(), s, plan)
			if err != nil {
				t.Fatal(channel, id, err)
			}
			snap, err := w.characters.TagCharacterSnapshot(other)
			if err != nil {
				t.Fatal(err)
			}
			want, err := protocol.AdventureEliteCharacterInfo(w.role.WireID, []byte{1}, []protocol.TagCharacter{snap})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(adventurePacket(out, 1382), want) || !bytes.Equal(adventurePacket(out, 1879), []byte{2, 0}) {
				t.Fatal("actual saved snapshot changed", channel, id)
			}
			n28, n1382, n1879, n29 := -1, -1, -1, -1
			for i, p := range out {
				if p.Kind != 0 {
					continue
				}
				switch p.ID {
				case 28:
					n28 = i
				case 1382:
					n1382 = i
				case 1879:
					n1879 = i
				case 29:
					n29 = i
				}
			}
			if !(n28 < n1382 && n1382 < n1879 && n1879 < n29) || !reflect.DeepEqual(out[0], plan[0]) || !reflect.DeepEqual(out[len(out)-1], plan[len(plan)-1]) {
				t.Fatal("native stage/frame order changed")
			}
		}
	}
	after, err := w.store.Characters(context.Background(), w.account)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("loading modified character storage", err)
	}
}

func TestAllEliteEmptyRosterAndSpecialFactoriesRemainSolo(t *testing.T) {
	t.Setenv("DFO_ELITE_COMPANION_PILOT", "all")
	w, _ := adventureTestWorld(t)
	s := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 17, Script: catalog.ScriptRecord{Path: "contents/example.dgn"}}}
	sel := protocol.DungeonSelection{ID: 17, Party: 65535}
	if p, err := w.allDungeonEliteEntryPackets(context.Background(), sel, s); err != nil || len(p) != 0 {
		t.Fatal("empty roster blocked solo entry", err)
	}
	w, _ = allEliteWorld(t)
	for name, change := range map[string]func(*worldSession, *protocol.DungeonSelection){
		"tutorial":         func(w *worldSession, _ *protocol.DungeonSelection) { w.inTutorial = true },
		"training factory": func(w *worldSession, _ *protocol.DungeonSelection) { w.boostTrainingAPCRequested = true },
		"mode4":            func(w *worldSession, _ *protocol.DungeonSelection) { w.channelType = 76 },
		"foreign party":    func(_ *worldSession, s *protocol.DungeonSelection) { s.Party = 9 },
		"wrong session":    func(_ *worldSession, s *protocol.DungeonSelection) { s.ID++ },
	} {
		t.Run(name, func(t *testing.T) {
			cw, cs := *w, sel
			change(&cw, &cs)
			p, err := cw.allDungeonEliteEntryPackets(context.Background(), cs, s)
			if err != nil || len(p) != 0 {
				t.Fatal("other factory acquired companions", err)
			}
		})
	}
	for _, value := range []string{"", "attunement", "1", "ALL"} {
		t.Setenv("DFO_ELITE_COMPANION_PILOT", value)
		if p, err := w.allDungeonEliteEntryPackets(context.Background(), sel, s); err != nil || len(p) != 0 {
			t.Fatal("candidate escaped launch scope", err)
		}
	}
}

func TestAllEliteEntryRejectsWrongFrameOrder(t *testing.T) {
	t.Setenv("DFO_ELITE_COMPANION_PILOT", "all")
	w, _ := allEliteWorld(t)
	s := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 17, Script: catalog.ScriptRecord{Path: "contents/example.dgn"}}}
	for _, plan := range [][]outboundPacket{nil, {{"map", 0, 29, nil}, {"info", 0, 28, nil}}, {{"ack", 1, 28, nil}, {"map", 0, 29, nil}}} {
		if _, err := w.allDungeonEliteEntryPlan(context.Background(), s, plan); err == nil {
			t.Fatal("accepted wrong native order")
		}
	}
}
