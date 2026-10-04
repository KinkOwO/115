package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestKnightShieldCatalogLoadsFromLauncherConfigDirectory(t *testing.T) {
	// channel_probe starts outside the module and passes this absolute path.
	rules, e := filepath.Abs("../../configs/equipment-wear.current35.json")
	if e != nil {
		t.Fatal(e)
	}
	jobs, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	t.Chdir(t.TempDir())
	p := knightShieldCatalogPath("equipment-knight-shield.full-candidate.json", rules)
	shields, e := inventory.LoadKnightShields(p, jobs.Source.Checksum)
	if e != nil || shields == nil || len(shields.Rows) != 25 {
		t.Fatalf("launcher side-car path %s: %v", p, e)
	}
	if knightShieldCatalogPath(p, rules) != p || knightShieldCatalogPath("", rules) != "" {
		t.Fatal("absolute path or explicit disable changed")
	}
}

func TestKnightShieldRepaintOrderAndDungeonBoundary(t *testing.T) {
	jobs, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	role := database.Character{WireID: 1, Name: "ShieldTest", Profession: 12, ConfigVersion: jobs.Source.SaveIdentity()}
	role.State, e = inventory.SaveBag(json.RawMessage(`{"level":90,"advancement":1}`), inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{{Slot: 24, Template: 113370008}}})
	if e != nil {
		t.Fatal(e)
	}
	w := &worldSession{role: role, characters: &character.Service{Catalog: jobs}}
	t.Setenv("DFO_EQUIP_AVATAR_REFRESH", "1")
	plan, e := knightShieldUpdates(w, role, true)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 4 || plan[0].ID != 13 || plan[1].Name != "knight_shield_appearance_refreshed" || plan[2].Name != "knight_shield_worn_basic_refreshed" || plan[3].ID != 14 {
		t.Fatalf("repaint sequence: %+v", plan)
	}
	if _, e = preparePackets(make([]byte, wire.SessionKeyBytes), plan); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name                     string
		shelf, disabled, dungeon bool
	}{
		{name: "shelf already paints", shelf: true}, {name: "refresh disabled", disabled: true}, {name: "dungeon actor rebuild guarded", dungeon: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.disabled {
				t.Setenv("DFO_EQUIP_AVATAR_REFRESH", "0")
			}
			child := *w
			if tc.dungeon {
				child.activeDungeon = &dungeon.Session{}
			}
			plan, e := knightShieldUpdates(&child, role, !tc.shelf)
			if e != nil {
				t.Fatal(e)
			}
			for _, p := range plan {
				if p.ID == 2 {
					t.Fatal("unexpected mode0 actor rebuild")
				}
			}
			if len(plan) == 0 || plan[len(plan)-1].ID != 14 {
				t.Fatal("missing equipment window update")
			}
		})
	}
	// Clearing the only worn row must explicitly clear slot 24. An empty
	// WornSpaceUpdate would skip the frame and leave the old window row alive.
	role.State = json.RawMessage(`{"level":90,"advancement":1}`)
	p, e := knightShieldWornUpdate(role)
	if e != nil {
		t.Fatal(e)
	}
	if p[0] != 3 || binary.LittleEndian.Uint16(p[1:3]) != 1 || binary.LittleEndian.Uint16(p[3:5]) != 24 || binary.LittleEndian.Uint32(p[5:9]) != 0xffffffff {
		t.Fatalf("missing slot-24 clear: %x", p[:9])
	}
}

func TestKnightDeckEntryAndUnboundedRequests(t *testing.T) {
	state := entryPayloads{KnightDeck: protocol.KnightDeckInfo([5]uint32{113370003, 113370008})}
	prepared, e := preparePackets(make([]byte, wire.SessionKeyBytes), state.packets())
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, p := range prepared {
		if p.ID == 567 {
			count++
			if p.Kind != 0 || len(p.Payload) != 20 {
				t.Fatal("entry shield frame shape")
			}
		}
	}
	if count != 1 {
		t.Fatal("entry deck missing")
	}
	prepared, e = preparePackets(make([]byte, wire.SessionKeyBytes), (entryPayloads{}).packets())
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range prepared {
		if p.ID == 567 {
			t.Fatal("empty entry sent a fabricated deck")
		}
	}
	seen := map[uint16]int{}
	for i := 0; i < BodySampleLimit+3; i++ {
		if !retainRequestBody(649, seen) {
			t.Fatal("649 fell into sampling cap")
		}
	}
}

func TestKnightDeckFailuresKeepInertAckAndObservableReason(t *testing.T) {
	s := equipmentSession{}
	for _, body := range [][]byte{make([]byte, 23), make([]byte, 20)} {
		_, e := s.handleKnightDeck(nil, nil, body, body)
		if e == nil {
			t.Fatal("accepted upload without service or shape")
		}
		event := knightShieldObservation(nil, 649, body, 0, e)
		if event["refuse_reason"] == "" || event["refusal_code"] == uint16(0) {
			t.Fatal("refusal lost reason")
		}
	}
}
