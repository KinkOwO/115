package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func learnNaturalRobot(t *testing.T, s *character.Service, c *gamedata.Catalogs, role character.Character, advance byte, script catalog.ScriptRecord) character.Character {
	var err error
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
		if values[i] == int32(role.Profession) && values[i+1] == int32(advance) {
			id := uint16(values[i+2])
			priority = append(priority, id)
			selected[id] = true
		}
	}
	if len(priority) == 0 {
		t.Fatal("no source AI priority")
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
		if ok && (!v.Active || d.ForAwakening(int(advance), 3) && !d.ForAdvancement(int(advance))) {
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
	if state.Advancement != advance || state.Awakening != 3 {
		t.Fatal("progression absent")
	}
	active := 0
	for _, v := range final {
		if v.Active && v.Rank > 0 {
			active++
		}
	}
	minimumActive := 8
	if advance == 0 {
		minimumActive = 6
	}
	if active < minimumActive {
		t.Fatalf("only %d learned active skills", active)
	}

	t.Logf("job=%d advance=%d active=%d SP=%v", role.Profession, advance, active, state.SkillPoints)
	return role
}

func TestTownRobotNaturalVarietyPreparation(t *testing.T) {
	input := os.Getenv("DFO_ROBOT_NATURAL_FIXTURE")
	if input == "" {
		t.Skip("explicit offline fixture")
	}
	if filepath.Base(input) != "r25-natural-input.json" {
		t.Fatal("offline fixture required")
	}
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	records, err := readTownRobotArchive(input)
	if err != nil {
		t.Fatal(err)
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "characters,progression,skills,items,quests,loot,equipment,equipment-selection,fame", ArchivePath: `D:\115us\server\work\client-build\Script.inner.pvf`, ArchiveChecksum: "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc", CharacterPolicyPath: "../../configs/pvf-character-policy.json", DropPolicyPath: "../../configs/pvf-drop-policy.json", DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	c.Selection.Full = c.Equipment
	if err = c.SupplementStackables(c.Loot, ""); err != nil {
		t.Fatal(err)
	}
	s := &character.Service{Catalog: *c.Characters, Learning: c.Learning, Equipment: c.Selection}
	s.WearRules, err = inventory.LoadWearRules("../../configs/equipment-wear.full-candidate.json", c.SourceChecksum)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := character.LoadGrowthRules("../../configs/experience.compat90.json")
	if err != nil {
		t.Fatal(err)
	}
	growth := &character.ProgressionService{Catalog: *c.Progression, Professions: *c.Characters, Rules: rules}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: `D:\115us\server\work\client-build\Script.inner.pvf`, MaxBytes: 1 << 30}, c.SourceChecksum)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	script, err := catalog.ResolveScript(a, "etc/mycharactersapc/mycharacters_apc_skill_autoset_info.etc")
	if err != nil {
		t.Fatal(err)
	}

	restore, err := c.InstallFameRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	bagRules, err := inventory.LoadBagRules("../../configs/inventory.current37.json", c.SourceChecksum)
	if err != nil {
		t.Fatal(err)
	}
	m := &townRobotManager{records: map[int]townRobotRecord{}, characters: s, growth: growth, award: &inventory.Awarder{Catalog: *c.Loot, Rules: bagRules, Equipment: c.Selection}, wear: &inventory.WearService{Catalog: c.Selection, Professions: *c.Characters, BagRules: bagRules, Rules: s.WearRules}}
	f := &townRobotFleet{manager: m, hub: newLanHub(), stop: make(chan struct{})}
	seed := records[1]
	seedBag, err := inventory.ReadBag(seed.Role.State)
	if err != nil {
		t.Fatal(err)
	}
	// One-time source loadouts: original mall fire costume; dark knight;
	// gold warrior; silver armor. Every wear operation uses ordinary source rules.
	styles := [][]uint32{nil, {39085, 39524, 39869, 40265, 40734, 41133, 41468, 41909}, {39113, 39546, 39886, 40287, 40759, 41157, 41491, 41934}, {39117, 39550, 39890, 40291, 40763, 41161, 41495, 41938}}
	advances := []byte{3, 1, 4, 2}
	profiles := []townRobotRecord{}
	report := []any{}
	for n, advance := range advances {
		id := n + 1
		r := seed
		r.ID = id
		r.Revision = 1
		if n > 0 {
			role, e := s.TownDisplayCharacter(uint16(60000+id), fmt.Sprintf("旅人%04d", id), 0, 115, growth)
			if e != nil {
				t.Fatal(e)
			}
			role.State, e = s.ApplyAdvancement(role, advance)
			if e != nil {
				t.Fatal(e)
			}
			for stage := byte(1); stage <= 3; stage++ {
				role.State, e = s.ApplyAwakening(role, stage)
				if e != nil {
					t.Fatal(e)
				}
			}
			role = learnNaturalRobot(t, s, c, role, advance, script)
			role.State, e = inventory.SaveBag(role.State, seedBag)
			if e != nil {
				t.Fatal(e)
			}
			r.Role = role
		}
		r.Role.ID = -int64(id)
		r.Role.AccountID = 0
		r.Role.WireID = 0
		r.Role.Name = fmt.Sprintf("旅人%04d", id)
		m.records[id] = r
		styleReport := []any{}
		for _, template := range styles[n] {
			d, e := c.Selection.Definition(template)
			if e != nil {
				t.Fatal(e)
			}
			if !d.IsAvatar() {
				t.Fatal("non-avatar style", template)
			}
			slot, ok := s.WearRules.Slots[d.Fields["[equipment type]"][0].Text]
			if !ok {
				t.Fatal("no slot", template)
			}
			if e = f.changeRobot(townRobotChange{ID: id, Revision: m.records[id].Revision, Action: "grant", Template: template, Amount: 1}); e != nil {
				t.Fatal(id, template, e)
			}
			b, e := inventory.ReadBag(m.records[id].Role.State)
			if e != nil {
				t.Fatal(e)
			}
			var item inventory.BagEquipment
			found := false
			for _, v := range b.Special[1] {
				if v.Template == template {
					item = v
					found = true
					break
				}
			}
			if !found {
				t.Fatal("grant not in avatar wardrobe", template)
			}
			old := uint32(0)
			for _, v := range b.Worn {
				if v.Slot == slot {
					old = v.Template
				}
			}
			move := protocol.ItemMoveRequest{SourceList: 1, SourceSlot: item.Slot, SourceItem: template, DestinationList: 3, DestinationSlot: slot, DestinationItem: old, Count: 1, Selection: 0xffffffff}
			if e = f.changeRobot(townRobotChange{ID: id, Revision: m.records[id].Revision, Action: "move", Move: move}); e != nil {
				t.Fatal(id, template, e)
			}
			styleReport = append(styleReport, map[string]any{"template": template, "source_path": d.Path, "source_sha256": d.SHA256, "slot": slot, "variation": d.Fields["[variation]"]})
		}
		r = m.records[id]
		profiles = append(profiles, r)
		var state character.State
		if e := json.Unmarshal(r.Role.State, &state); e != nil {
			t.Fatal(e)
		}
		if state.Advancement != advance || state.Awakening != 3 || state.Level != 115 {
			t.Fatal("profile progression")
		}
		report = append(report, map[string]any{"id": id, "advance": advance, "awakening": state.Awakening, "styles": styleReport, "skill_points": state.SkillPoints, "inventory_old_items_retained": true})
	}
	output := map[int]townRobotRecord{}
	for id := 1; id <= 50; id++ {
		data, e := json.Marshal(profiles[(id-1)%len(profiles)])
		if e != nil {
			t.Fatal(e)
		}
		var r townRobotRecord
		if e = json.Unmarshal(data, &r); e != nil {
			t.Fatal(e)
		}
		r.ID = id
		r.Role.ID = -int64(id)
		r.Role.Name = fmt.Sprintf("旅人%04d", id)
		r.Revision = 1
		output[id] = r
		projection := r.Role
		projection.WireID = uint16(60000 + id)
		basic, e := s.EntryBasicProbe(projection, [2]byte{})
		if e != nil {
			t.Fatal(id, e)
		}
		addition, e := s.TownRobotEntryAddition(projection)
		if e != nil {
			t.Fatal(id, e)
		}
		companion, e := s.TownRobotCompanionSnapshot(projection)
		if e != nil {
			t.Fatal(id, e)
		}
		if id <= 4 {
			types := map[uint32]int{}
			slots := map[uint16]bool{}
			for _, it := range companion.Worn {
				kind, e := c.Selection.EquipmentKind(it.Template)
				if e != nil {
					t.Fatal(e)
				}
				slot, ok := s.WearRules.Slots[kind]
				if !ok {
					t.Fatal(kind)
				}
				types[it.Template] = int(slot)
				slots[it.Slot] = true
			}
			vector, _ := json.MarshalIndent(map[string]any{"payload_hex": hex.EncodeToString(addition), "types": types, "slots": slots, "source_sha256": c.SourceChecksum, "role": projection, "basic": basic, "addition": addition, "companion": companion}, "", "  ")
			if e = os.WriteFile(filepath.Join(filepath.Dir(input), fmt.Sprintf("r25-entry-%d.json", id)), vector, 0600); e != nil {
				t.Fatal(e)
			}
		}
		b, e := inventory.ReadBag(r.Role.State)
		if e != nil || len(b.Worn) != len(seedBag.Worn) {
			t.Fatal("worn item loss", id, e)
		}
		for _, old := range seedBag.Worn {
			if old.Slot <= 7 {
				continue
			}
			present := false
			for _, v := range b.Worn {
				if v.Slot == old.Slot && v.Template == old.Template {
					present = true
				}
			}
			if !present {
				t.Fatal("combat equipment lost", id, old.Slot)
			}
		}
	}
	data, err := json.MarshalIndent(townRobotArchive{Version: 1, Records: output}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(filepath.Dir(input), "r25-natural-prepared.json")
	if err = os.WriteFile(out, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readTownRobotArchive(out); err != nil {
		t.Fatal(err)
	}
	data, _ = json.MarshalIndent(map[string]any{"profiles": report, "records": 50, "robot1_preserved": bytes.Equal(output[1].Role.State, seed.Role.State), "player_database_opened": false}, "", "  ")
	if err = os.WriteFile(filepath.Join(filepath.Dir(input), "r25-natural-report.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	now, err := os.ReadFile(input)
	if err != nil || !bytes.Equal(original, now) {
		t.Fatal("offline input changed")
	}

}

func TestTownRobotNativeStyleSource(t *testing.T) {
	out := os.Getenv("DFO_ROBOT_STYLE_AUDIT")
	if out == "" {
		t.Skip("explicit source style audit")
	}
	a, e := pvf.OpenReadOnly(pvf.Options{Path: `D:\115us\server\work\client-build\Script.inner.pvf`, MaxBytes: 1 << 30}, "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	index, e := catalog.ResolveScript(a, "list/equipment.lst")
	if e != nil {
		t.Fatal(e)
	}
	refs, e := catalog.ParseIndex(index.Cells)
	if e != nil {
		t.Fatal(e)
	}
	rows := []any{}
	for _, ref := range refs {
		if !strings.Contains(ref.Path, "/avatar/") {
			continue
		}
		if !strings.Contains(ref.Path, "/swordman/") && !strings.Contains(ref.Path, "dragonrobe") {
			continue
		}
		if strings.Contains(ref.Path, "dragonrobe4") {
			continue
		}
		v, e := catalog.ResolveScript(a, ref.Path)
		if e != nil {
			t.Fatal(e)
		}
		rows = append(rows, map[string]any{"id": ref.ID, "script": v})
		if len(rows) > 3000 {
			break
		}
	}
	raw, _ := json.MarshalIndent(rows, "", "  ")
	if e = os.WriteFile(out, raw, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("avatar source rows", len(rows))
}
