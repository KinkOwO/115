package main

import (
	"bytes"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Explicit operator fixture. Never opens a player database or live robot store;
// only emits a prepared archive beside the supplied offline copy.
func TestTownRobotMallInlayPreparation(t *testing.T) {
	input := os.Getenv("DFO_ROBOT_INLAY_FIXTURE")
	if input == "" {
		t.Skip("explicit offline robot inventory fixture")
	}
	if filepath.Base(input) != "r21-inlay-input.json" {
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
	// This is this user's one-time loadout choice, not a runtime gameplay table.
	choices := map[uint16]uint32{1: 590702742, 2: 590702743, 4: 590702738, 8: 590702736, 16: 50032056, 65519: 590702742}
	bag, err := inventory.ReadBag(before.Role.State)
	if err != nil {
		t.Fatal(err)
	}
	inserted := 0
	for _, worn := range bag.Worn {
		if len(worn.AvatarOptions) != 30 {
			continue
		}
		req := protocol.UseEmblemRequest{Space: 3, AvatarSlot: worn.Slot, Template: worn.Template}
		current, err := inventory.ReadBag(m.records[1].Role.State)
		if err != nil {
			t.Fatal(err)
		}
		for socket := 0; socket < 5; socket++ {
			mask := binary.LittleEndian.Uint16(worn.AvatarOptions[socket*6:])
			if mask == 0 {
				continue
			}
			emblem := choices[mask]
			if emblem == 0 {
				t.Fatalf("unselected mask %d", mask)
			}
			if binary.LittleEndian.Uint32(worn.AvatarOptions[socket*6+2:]) != 0 {
				t.Fatal("refuse replacing an existing emblem")
			}
			found := false
			for _, stack := range current.Items {
				if stack.Template == emblem {
					req.Inputs = append(req.Inputs, protocol.UseEmblemInput{Slot: stack.Slot, Template: emblem, Socket: byte(socket)})
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("owned emblem missing %d", emblem)
			}
		}
		if len(req.Inputs) == 0 {
			continue
		}
		if err = f.changeRobot(townRobotChange{ID: 1, Revision: m.records[1].Revision, Action: "emblem", Emblem: req}); err != nil {
			t.Fatal(worn.Template, err)
		}
		inserted += len(req.Inputs)
	}
	if inserted != 20 {
		t.Fatalf("expected 20 source-open sockets, got %d", inserted)
	}
	after := m.records[1]
	final, err := inventory.ReadBag(after.Role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(final.Worn) != len(bag.Worn) {
		t.Fatal("worn item loss")
	}
	for _, it := range final.Items {
		if it.Template == 50032208 {
			if it.Amount != 2 {
				t.Fatal("buff platinum reserve changed")
			}
			continue
		}
		for _, emblem := range choices {
			if it.Template == emblem {
				t.Fatal("unexpected unconsumed selected emblem")
			}
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
	out := filepath.Join(filepath.Dir(input), "r21-inlay-prepared.json")
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
	report := map[string]any{"input_sha256": hex.EncodeToString(hash[:]), "input_revision": before.Revision, "output_revision": after.Revision, "sockets": inserted, "status": "PREPARED_NOT_APPLIED"}
	raw, _ = json.MarshalIndent(report, "", "  ")
	if err = os.WriteFile(filepath.Join(filepath.Dir(input), "r21-inlay-prepared-report.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("prepared %d sockets; source inputs, original items, unrelated role state preserved; not installed", inserted)
}
