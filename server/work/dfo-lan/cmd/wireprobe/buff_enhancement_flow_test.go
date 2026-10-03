package main

import (
	"bytes"
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuffEnhancementRestoresAfterItemFamilies(t *testing.T) {
	p := entryPayloads{Skills: []byte{1}, Inventory: []byte{1}, Worn: []byte{1}, AvatarReady: []byte{1}, CreatureList: []byte{1}, Creatures: []byte{1}, WornSlots: []byte{1}, WornUpdate: []byte{1}, BuffEnhancement: []byte{0, 0, 0}}
	packets := p.packets()
	restore := -1
	for i, packet := range packets {
		if packet.ID == 1361 {
			restore = i
		}
	}
	if restore < 0 {
		t.Fatal("missing restore")
	}
	for i, packet := range packets {
		if (packet.ID == 19 || packet.ID == 13 || packet.ID == 14 || packet.ID == 105) && i >= restore {
			t.Fatalf("item/skill %s follows restore", packet.Name)
		}
	}
	if !observedGameRequest(1421) {
		t.Fatal("registration subject to eight-body cap")
	}
	seen := map[uint16]int{1421: BodySampleLimit + 1}
	if !retainRequestBody(1421, seen) {
		t.Fatal("repeated registration body was discarded")
	}
}

func TestBuffEnhancementDungeonAndTownActorReconstruction(t *testing.T) {
	t.Setenv("DFO_OATH_DIRECT_ENTRY", "0") // This fixture has no oath storage service.
	path := filepath.Join(t.TempDir(), "equipment.json")
	raw, _ := json.Marshal(inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: "fixture"}, Rows: []inventory.EquipmentDefinition{
		{ID: 500330719, Path: "title.equ", SHA256: strings.Repeat("a", 64), Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[title name]"}}}},
	}})
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := inventory.LoadEquipmentCatalog(path, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	item := inventory.BagEquipment{Slot: 9, Template: 500330719}
	identity := item
	identity.Slot = 0
	encoded, _ := json.Marshal(identity)
	sum := sha256.Sum256(encoded)
	state := map[string]any{"level": 10, "source_sha256": "fixture", "attributes": map[string]int{"[hp max]": 100, "[mp max]": 100}, "buff_enhancement": map[string]any{"version": 1, "skill": 281, "items": []map[string]any{{"kind": 13, "template": 500330719, "fingerprint": hex.EncodeToString(sum[:])}}}}
	raw, _ = json.Marshal(state)
	raw, err = inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Equipment: []inventory.BagEquipment{item}})
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{account: 1, role: storage.Character{ID: 8, AccountID: 1, WireID: 8, Profession: 11, ConfigVersion: "fixture", State: raw}, activeDungeon: &dungeon.Session{}, state: storage.WorldState{Position: storage.WorldPosition{Town: 38, Area: 2, X: 150, Y: 249}}, characters: &character.Service{Equipment: catalog, WearRules: inventory.WearRules{Slots: map[string]uint16{"[title name]": 13}}}}
	before := append([]byte(nil), w.role.State...)
	// Each new map/actor needs another binding; returning to town needs its
	// own replay after the actor and all live item updates, including death.
	for _, name := range []string{"first dungeon", "next map", "town return"} {
		var plan []outboundPacket
		if name == "town return" {
			plan, err = w.leaveDungeon()
		} else {
			plan, err = w.finishDungeonLoading(make([]byte, 16))
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		last := plan[len(plan)-1]
		if last.ID != 1361 || last.Kind != 0 || !bytes.Equal(last.Payload, []byte{0x19, 1, 1, 1, 13, 0, 9, 0}) {
			t.Fatalf("%s did not restore registered title last: %+v", name, last)
		}
		for i, p := range plan {
			if p.ID == 1361 && i != len(plan)-1 {
				t.Fatalf("%s rebound before final actor updates", name)
			}
		}
		if !bytes.Equal(w.role.State, before) {
			t.Fatal("scene transition rewrote saved registration")
		}
	}
	for _, damaged := range []string{`{"version":2}`, `{"version":1,"items":[{"kind":13},{"kind":13}]}`, `"invalid"`} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(before, &fields); err != nil {
			t.Fatal(err)
		}
		fields["buff_enhancement"] = json.RawMessage(damaged)
		w.role.State, err = json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		original := append([]byte(nil), w.role.State...)
		w.activeDungeon = &dungeon.Session{}
		for _, transition := range []func() ([]outboundPacket, error){
			func() ([]outboundPacket, error) { return w.finishDungeonLoading(make([]byte, 16)) },
			w.leaveDungeon,
		} {
			plan, err := transition()
			if err != nil || len(plan) == 0 {
				t.Fatalf("optional registration blocked transition: %v", err)
			}
			last := plan[len(plan)-1]
			if last.ID != 1361 || !bytes.Equal(last.Payload, []byte{0, 0, 0}) {
				t.Fatalf("damaged registration did not clear client cache: %+v", last)
			}
			if !bytes.Equal(original, w.role.State) {
				t.Fatal("damaged optional save was rewritten")
			}
		}
	}
}
