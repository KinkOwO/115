package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Explicit offline operation for the user's chosen Berserker. The game and
// live archive are never opened or mutated by this preparation test.
func TestTownRobotBerserkerPreparation(t *testing.T) {
	input := os.Getenv("DFO_ROBOT_PROGRESSION_FIXTURE")
	if input == "" {
		t.Skip("explicit offline robot fixture")
	}
	if filepath.Base(input) != "r21-progression-input.json" {
		t.Fatal("offline input required")
	}
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	records, err := readTownRobotArchive(input)
	if err != nil {
		t.Fatal(err)
	}
	before := records[1]
	role := before.Role
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "characters,skills,items,quests,loot,equipment,equipment-selection,fame", ArchivePath: `D:\115us\server\work\client-build\Script.inner.pvf`, ArchiveChecksum: "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc", CharacterPolicyPath: "../../configs/pvf-character-policy.json", DropPolicyPath: "../../configs/pvf-drop-policy.json", DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	c.Selection.Full = c.Equipment
	s := &character.Service{Catalog: *c.Characters, Learning: c.Learning, Equipment: c.Selection}
	s.WearRules, err = inventory.LoadWearRules("../../configs/equipment-wear.full-candidate.json", c.SourceChecksum)
	if err != nil {
		t.Fatal(err)
	}
	role.State, err = s.ApplyAdvancement(role, 3)
	if err != nil {
		t.Fatal(err)
	}
	for stage := byte(1); stage <= 3; stage++ {
		role.State, err = s.ApplyAwakening(role, stage)
		if err != nil {
			t.Fatal(err)
		}
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: `D:\115us\server\work\client-build\Script.inner.pvf`, MaxBytes: 1 << 30}, "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	script, err := catalog.ResolveScript(a, "etc/mycharactersapc/mycharacters_apc_skill_autoset_info.etc")
	if err != nil {
		t.Fatal(err)
	}
	values := []int32{}
	for _, v := range script.Cells {
		if v.Type == 0 {
			values = append(values, v.Value)
		}
	}
	if len(values)%5 != 0 {
		t.Fatal("native skill autoset row width changed")
	}
	priority := []uint16{}
	selected := map[uint16]bool{}
	for i := 0; i < len(values); i += 5 {
		if values[i] == 0 && values[i+1] == 3 {
			id := uint16(values[i+2])
			priority = append(priority, id)
			selected[id] = true
		}
	}
	if len(priority) == 0 {
		t.Fatal("no source Berserker AI priority")
	}
	buy := func(id uint16) bool {
		next, e := s.ApplySkillPurchase(role, protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: id, Delta: 1}}})
		if e != nil {
			return false
		}
		role.State = next
		return true
	}
	// Establish each source attack/buff first; then use normal SP/level caps.
	// Revisit prerequisites rather than forcing unavailable skill ranks.
	for pass := 0; pass < 10; pass++ {
		choices, e := s.CompanionSkillChoices(role)
		if e != nil {
			t.Fatal(e)
		}
		changed := false
		for _, v := range choices {
			if v.Rank == 0 && v.NextCost != nil && (selected[v.ID] || !v.Active) {
				if buy(v.ID) {
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	choices, err := s.CompanionSkillChoices(role)
	if err != nil {
		t.Fatal(err)
	}
	// Source passives and granted awakening skills, followed by the native
	// attack/buff priority. This is a one-time loadout choice, not a rules table.
	order := []uint16{}
	for _, v := range choices {
		d, ok, e := c.Learning.Definition(role.Profession, v.ID)
		if e != nil {
			t.Fatal(e)
		}
		if ok && (!v.Active || d.ForAwakening(3, 3) && !d.ForAdvancement(3)) {
			order = append(order, v.ID)
		}
	}
	order = append(order, priority...)
	for _, id := range order {
		for n := 0; n < 255 && buy(id); n++ {
		}
	}
	final, err := s.CompanionSkillChoices(role)
	if err != nil {
		t.Fatal(err)
	}
	var state character.State
	if err = json.Unmarshal(role.State, &state); err != nil {
		t.Fatal(err)
	}
	if state.Advancement != 3 || state.Awakening != 3 {
		t.Fatal("progression absent")
	}
	active := 0
	for _, v := range final {
		if v.Active && v.Rank > 0 {
			active++
		}
	}
	if active < 12 {
		t.Fatalf("only %d learned active skills", active)
	}
	var oldKeys, newKeys map[string]json.RawMessage
	json.Unmarshal(before.Role.State, &oldKeys)
	json.Unmarshal(role.State, &newKeys)
	// Equipment (including every newly inserted emblem) must stay exact.
	var oldBag, newBag bytes.Buffer
	json.Compact(&oldBag, oldKeys["inventory"])
	json.Compact(&newBag, newKeys["inventory"])
	if !bytes.Equal(oldBag.Bytes(), newBag.Bytes()) {
		t.Fatal("inventory changed")
	}
	if before.Role.ID != role.ID || before.Role.AccountID != role.AccountID || before.Role.WireID != role.WireID {
		t.Fatal("identity changed")
	}
	projection := role
	projection.WireID = 60001
	if _, err = s.EntryBasicProbe(projection, [2]byte{}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EntryAddition(projection); err != nil {
		t.Fatal(err)
	}
	if _, err = s.TownRobotCompanionSnapshot(projection); err != nil {
		t.Fatal(err)
	}
	record := before
	record.Role = role
	record.Revision++
	records[1] = record
	out, err := json.MarshalIndent(townRobotArchive{Version: 1, Records: records}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(filepath.Dir(input), "r21-progression-prepared.json"), out, 0600); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"advancement": state.Advancement, "awakening": state.Awakening, "sp": state.SkillPoints, "active_learned": active, "priority": priority, "skills": final}, "", "  ")
	if err = os.WriteFile(filepath.Join(filepath.Dir(input), "r21-progression-report.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
	disk, err := os.ReadFile(input)
	if err != nil || !bytes.Equal(original, disk) {
		t.Fatal("input changed")
	}
	t.Logf("Berserker stage=%d active=%d remaining SP=%v inventory preserved", state.Awakening, active, state.SkillPoints)
}
