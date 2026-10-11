package main

import (
	"bytes"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Explicit operator fixture. Never opens a player database or live robot store;
// only emits a prepared archive beside the supplied offline copy.
func TestTownRobotCombatGearPreparation(t *testing.T) {
	input := os.Getenv("DFO_ROBOT_GEAR_FIXTURE")
	if input == "" {
		t.Skip("explicit offline robot inventory fixture")
	}
	if filepath.Base(input) != "r22-gear-input.json" {
		t.Fatal("offline fixture name required")
	}
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	records, err := readTownRobotArchive(input)
	if err != nil {
		t.Fatal(err)
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "characters,items,quests,loot,equipment,equipment-selection,fame", ArchivePath: `D:\115us\server\work\client-build\Script.inner.pvf`, ArchiveChecksum: "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc", CharacterPolicyPath: "../../configs/pvf-character-policy.json", DropPolicyPath: "../../configs/pvf-drop-policy.json", DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	c.Selection.Full = c.Equipment
	if err = c.SupplementStackables(c.Loot, ""); err != nil {
		t.Fatal(err)
	}
	s := &character.Service{Catalog: *c.Characters, Equipment: c.Selection}
	s.WearRules, err = inventory.LoadWearRules("../../configs/equipment-wear.full-candidate.json", c.SourceChecksum)
	if err != nil {
		t.Fatal(err)
	}
	bagRules, err := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	bagRules.Source = c.SourceChecksum
	restore, err := c.InstallFameRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	m := &townRobotManager{records: records, characters: s, award: &inventory.Awarder{Catalog: *c.Loot, Rules: bagRules, Equipment: c.Selection}, wear: &inventory.WearService{Catalog: c.Selection, Professions: *c.Characters, BagRules: bagRules, Rules: s.WearRules}, items: &inventory.ItemService{Catalog: *c.Loot, Equipment: c.Selection, EmblemInlay: c.EmblemInlay}}
	f := &townRobotFleet{manager: m, hub: newLanHub(), stop: make(chan struct{})}
	before := records[1]

	bag, err := inventory.ReadBag(before.Role.State)
	if err != nil {
		t.Fatal(err)
	}
	ids := []uint32{100051286, 100101169, 100151110, 100201082, 100251122, 100301829, 100313532, 100323422, 100345967, 100354142, 100391020, 101011321}
	inserted := 0
	for _, id := range ids {
		current, err := inventory.ReadBag(m.records[1].Role.State)
		if err != nil {
			t.Fatal(err)
		}
		done := false
		for _, e := range current.Worn {
			if e.Template == id {
				done = true
			}
		}
		if done {
			continue
		}
		if err = f.changeRobot(townRobotChange{ID: 1, Revision: m.records[1].Revision, Action: "grant", Template: id, Amount: 1}); err != nil {
			t.Fatal(id, err)
		}
		current, _ = inventory.ReadBag(m.records[1].Role.State)
		found := false
		for _, e := range current.Equipment {
			if e.Template != id {
				continue
			}
			v := m.itemView(0, e)
			if v.WearSlot == nil {
				t.Fatal("missing source wear slot", id)
			}
			req := protocol.ItemMoveRequest{SourceList: 0, SourceSlot: e.Slot, SourceItem: id, DestinationList: 3, DestinationSlot: *v.WearSlot, Count: 1, Selection: 4294967295}
			for _, w := range current.Worn {
				if w.Slot == *v.WearSlot {
					req.DestinationItem = w.Template
				}
			}
			if err = f.changeRobot(townRobotChange{ID: 1, Revision: m.records[1].Revision, Action: "move", Move: req}); err != nil {
				t.Fatal(id, err)
			}
			found = true
			inserted++
			t.Logf("worn %d slot %d", id, *v.WearSlot)
			break
		}
		if !found {
			t.Fatal("new grant missing", id)
		}
	}
	after := m.records[1]
	final, err := inventory.ReadBag(after.Role.State)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		found := false
		for _, e := range final.Worn {
			if e.Template == id {
				found = true
			}
		}
		if !found {
			t.Fatal("not equipped", id)
		}
	}
	for _, old := range bag.Worn {
		found := false
		for _, e := range final.Worn {
			if e.Template == old.Template {
				a, _ := json.Marshal(e)
				b, _ := json.Marshal(old)
				if !bytes.Equal(a, b) {
					t.Fatal("cosmetic changed", old.Template)
				}
				found = true
			}
		}
		if !found {
			t.Fatal("old worn lost", old.Template)
		}
	}
	// Preserve every unrelated role-state key and all other robot records.
	var a, b map[string]json.RawMessage
	json.Unmarshal(before.Role.State, &a)
	json.Unmarshal(after.Role.State, &b)
	for k, v := range a {
		var x, y bytes.Buffer
		if err := json.Compact(&x, v); err != nil {
			t.Fatal(err)
		}
		if err := json.Compact(&y, b[k]); err != nil {
			t.Fatal(err)
		}
		if k != "inventory" && !bytes.Equal(x.Bytes(), y.Bytes()) {
			t.Fatalf("unrelated state changed: %s", k)
		}
	}
	disk, err := os.ReadFile(input)
	if err != nil || !bytes.Equal(original, disk) {
		t.Fatal("input fixture changed")
	}
	out := filepath.Join(filepath.Dir(input), "r22-gear-prepared.json")
	raw, err := json.MarshalIndent(townRobotArchive{Version: 1, Records: m.records}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readTownRobotArchive(out); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(original)
	report := map[string]any{"input_sha256": hex.EncodeToString(hash[:]), "input_revision": before.Revision, "output_revision": after.Revision, "equipped": inserted, "status": "PREPARED_NOT_APPLIED"}
	raw, _ = json.MarshalIndent(report, "", "  ")
	if err = os.WriteFile(filepath.Join(filepath.Dir(input), "r22-gear-prepared-report.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("prepared %d equipment pieces; source inputs, original items, unrelated role state preserved; not installed", inserted)
}
