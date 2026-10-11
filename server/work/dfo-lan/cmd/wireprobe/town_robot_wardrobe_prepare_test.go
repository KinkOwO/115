package main

import (
	"bytes"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestTownRobotWardrobePreparation(t *testing.T) {
	input := os.Getenv("DFO_ROBOT_WARDROBE_FIXTURE")
	if input == "" {
		t.Skip("explicit offline fixture")
	}
	if filepath.Base(input) != "r28-wardrobe-input.json" {
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
	m.items = &inventory.ItemService{Catalog: *c.Loot, Equipment: c.Selection, EmblemInlay: c.EmblemInlay}
	f := &townRobotFleet{manager: m, hub: newLanHub(), stop: make(chan struct{})}
	m.records = records
	equip := func(id int, template uint32) {
		d, e := c.Selection.Definition(template)
		if e != nil {
			t.Fatal(id, template, e)
		}
		kind := d.Fields["[equipment type]"]
		if len(kind) == 0 {
			t.Fatal("equipment kind")
		}
		slot, ok := s.WearRules.Slots[kind[0].Text]
		if !ok {
			t.Fatal("equipment slot", template)
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
		list := byte(0)
		for _, it := range b.Equipment {
			if it.Template == template {
				item = it
				found = true
				break
			}
		}
		if !found {
			for k, pool := range b.Special {
				for _, it := range pool {
					if it.Template == template {
						item = it
						list = k
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
		if !found {
			t.Fatal("grant missing", template)
		}
		old := uint32(0)
		for _, it := range b.Worn {
			if it.Slot == slot {
				old = it.Template
			}
		}
		move := protocol.ItemMoveRequest{SourceList: list, SourceSlot: item.Slot, SourceItem: template, DestinationList: 3, DestinationSlot: slot, DestinationItem: old, Count: 1, Selection: 0xffffffff}
		if e = f.changeRobot(townRobotChange{ID: id, Revision: m.records[id].Revision, Action: "move", Move: move}); e != nil {
			t.Fatal(id, template, e)
		}
	}
	type emblemChoice struct {
		id              uint32
		mask            uint16
		name, path, sha string
		grade           int32
	}
	// Offline audit indexes each original localization table once.
	tables, e := a.StringTables()
	if e != nil {
		t.Fatal(e)
	}
	localized := map[int]map[string]string{}
	sourceName := func(path string) (string, bool, error) {
		ref, found, e := a.ItemNameRef(path)
		if e != nil || !found {
			return "", false, e
		}
		if !ref.IsRef {
			return ref.Raw, true, nil
		}
		table, ok := localized[ref.Ref.Table]
		if !ok {
			text, e := a.ReadText(tables[ref.Ref.Table])
			if e != nil {
				return "", false, e
			}
			table = map[string]string{}
			for _, line := range strings.Split(text, "\n") {
				if i := strings.Index(line, ">"); i > 0 {
					table[strings.TrimSpace(line[:i])] = strings.TrimRight(line[i+1:], "\r")
				}
			}
			localized[ref.Ref.Table] = table
		}
		name, found := table[ref.Ref.Key]
		return name, found, nil
	}
	emblemPool := []emblemChoice{}
	for id, mask := range c.EmblemInlay.Masks {
		it := c.Items.Items[id]
		if it.Path == "" {
			continue
		}
		name, found, e := sourceName(it.Path)
		if e != nil {
			t.Fatal(e)
		}
		if !found {
			continue
		}
		source, e := catalog.ResolveScript(a, it.Path)
		if e != nil {
			t.Fatal(e)
		}
		grade := int32(0)
		for i, x := range source.Cells {
			if x.Text == "[rarity]" && i+1 < len(source.Cells) {
				grade = source.Cells[i+1].Value
			}
		}
		emblemPool = append(emblemPool, emblemChoice{id, mask, name, it.Path, source.SHA256, grade})
	}
	sort.Slice(emblemPool, func(i, j int) bool {
		if emblemPool[i].grade != emblemPool[j].grade {
			return emblemPool[i].grade > emblemPool[j].grade
		}
		return emblemPool[i].id < emblemPool[j].id
	})
	var pool struct {
		Source pvf.ArchiveSnapshot `json:"source"`
		Sets   []struct {
			Package   string   `json:"package"`
			SHA       string   `json:"sha256"`
			Category  []int    `json:"category"`
			Row       int      `json:"row"`
			Templates []uint32 `json:"templates"`
		} `json:"sets"`
	}
	raw, e := os.ReadFile(filepath.Join(filepath.Dir(input), "mall-pool.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &pool); e != nil {
		t.Fatal(e)
	}
	if pool.Source.Checksum != c.SourceChecksum {
		t.Fatal("wardrobe source mismatch")
	}
	var recipes []struct {
		Job     byte   `json:"job"`
		Advance byte   `json:"advance"`
		Name    string `json:"name"`
		Amp     byte   `json:"amplify_type"`
	}
	raw, e = os.ReadFile(filepath.Join(filepath.Dir(input), "recipes.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &recipes); e != nil {
		t.Fatal(e)
	}
	seed, e := os.ReadFile(filepath.Join(filepath.Dir(input), "seed.txt"))
	if e != nil || len(seed) < 32 {
		t.Fatal("random seed missing")
	}
	type outfit struct {
		Package   string   `json:"package"`
		SHA       string   `json:"sha256"`
		Row       int      `json:"row"`
		Templates []uint32 `json:"templates"`
		Visual    string   `json:"visual_sha256"`
		Sources   []any    `json:"sources"`
	}
	visualTokens := func(tokens []pvf.Token) []any {
		out := []any{}
		for _, token := range tokens {
			if token.Text != "" {
				out = append(out, token.Text)
			} else {
				out = append(out, token.Value)
			}
		}
		return out
	}
	libraries := map[byte][]outfit{}
	librarySeen := map[byte]map[string]bool{}
	// Stable seeded shuffle, then reject incomplete, invisible, clone and low-grade
	// sets. At most 100 source-proven visual alternatives per compatible base job.
	sort.Slice(pool.Sets, func(i, j int) bool {
		left := sha256.Sum256(append(append([]byte(nil), seed...), []byte(fmt.Sprint(pool.Sets[i].Templates))...))
		right := sha256.Sum256(append(append([]byte(nil), seed...), []byte(fmt.Sprint(pool.Sets[j].Templates))...))
		return bytes.Compare(left[:], right[:]) < 0
	})
	for _, choice := range recipes {
		job := choice.Job
		if librarySeen[job] != nil {
			continue
		}
		librarySeen[job] = map[string]bool{}
		prof := c.Characters.Professions[job]
		for _, row := range pool.Sets {
			slots := map[uint16]bool{}
			visual := map[uint16]any{}
			valid := true
			sources := []any{}
			for _, template := range row.Templates {
				d, e := c.Selection.Definition(template)
				if e != nil {
					valid = false
					break
				}
				kind, rar := d.Fields["[equipment type]"], d.Fields["[rarity]"]
				if len(kind) == 0 || len(rar) != 1 || rar[0].Value < 2 || d.IsCloneAvatar() || inventory.WearableBy(d.Fields, kind[0].Text, prof.Job, choice.Advance, 115) != nil {
					valid = false
					break
				}
				slot, ok := s.WearRules.Slots[kind[0].Text]
				if !ok || (slot > 7 && slot != 8) || slots[slot] {
					valid = false
					break
				}
				if slot < 8 {
					opts := c.Selection.DefaultAvatarSockets(template)
					opened := 0
					for socket := 0; socket < 5; socket++ {
						if binary.LittleEndian.Uint16(opts[socket*6:]) != 0 {
							opened++
						}
					}
					if opened < 2 {
						valid = false
						break
					}
				}
				if len(d.Fields["[variation]"]) == 0 && len(d.Fields["[layer variation]"]) == 0 {
					valid = false
					break
				}
				slots[slot] = true
				visual[slot] = map[string]any{"animation": visualTokens(d.Fields["[animation job]"]), "variation": visualTokens(d.Fields["[variation]"]), "layers": visualTokens(d.Fields["[layer variation]"])}
				sources = append(sources, map[string]any{"template": template, "slot": slot, "path": d.Path, "sha256": d.SHA256})
			}
			for slot := uint16(0); slot < 8; slot++ {
				if !slots[slot] {
					valid = false
				}
			}
			if !valid {
				continue
			}
			graphics, _ := json.Marshal(visual)
			digest := sha256.Sum256(graphics)
			key := hex.EncodeToString(digest[:])
			if librarySeen[job][key] {
				continue
			}
			librarySeen[job][key] = true
			libraries[job] = append(libraries[job], outfit{row.Package, row.SHA, row.Row, row.Templates, key, sources})
			if len(libraries[job]) == 100 {
				break
			}
		}
		t.Log("wardrobe alternatives", job, len(libraries[job]))
	}
	used := map[string]bool{}
	report := []any{}
	for n, choice := range recipes {
		id := n + 1
		r := m.records[id]
		beforeRole := r.Role
		beforeBag, e := inventory.ReadBag(r.Role.State)
		if e != nil {
			t.Fatal(e)
		}
		var selected outfit
		for _, candidate := range libraries[choice.Job] {
			if !used[candidate.Visual] {
				selected = candidate
				break
			}
		}
		if selected.Visual == "" {
			t.Fatal("not enough distinct full outfits", choice.Name)
		}
		// Rebind the exact original row; never trust an editable audit as content.
		originalPack, e := catalog.ResolveScript(a, selected.Package)
		if e != nil || originalPack.SHA256 != selected.SHA {
			t.Fatal("package changed", e)
		}
		category := 0
		found := false
		for i, x := range originalPack.Cells {
			if x.Text == "[booster select category]" {
				category++
			}
			if category != selected.Row || x.Text != "[avatar]" {
				continue
			}
			nums := []uint32{}
			for j := i + 1; j < len(originalPack.Cells) && originalPack.Cells[j].Type != 3; j++ {
				if originalPack.Cells[j].Type != 0 {
					t.Fatal("source row type")
				}
				nums = append(nums, uint32(originalPack.Cells[j].Value))
			}
			actual := []uint32{}
			for j := 0; j+3 < len(nums); j += 4 {
				if nums[j+1] != 1 {
					t.Fatal("quantity")
				}
				actual = append(actual, nums[j])
			}
			if fmt.Sprint(actual) != fmt.Sprint(selected.Templates) {
				t.Fatal("package row mismatch")
			}
			found = true
			break
		}
		if !found {
			t.Fatal("source row absent")
		}
		for _, template := range selected.Templates {
			equip(id, template)
		}
		used[selected.Visual] = true
		emblemReport := []any{}
		mainStat := map[byte]string{1: "体力", 2: "精神", 3: "力量", 4: "智力"}[choice.Amp]
		socketBag, e := inventory.ReadBag(m.records[id].Role.State)
		if e != nil {
			t.Fatal(e)
		}
		for _, it := range socketBag.Worn {
			if len(it.AvatarOptions) != 30 || it.Slot > 8 {
				continue
			}
			for socket := 0; socket < 5; socket++ {
				mask := binary.LittleEndian.Uint16(it.AvatarOptions[socket*6:])
				if mask == 0 {
					continue
				}
				want := []string{mainStat}
				switch mask {
				case 4:
					if choice.Amp == 4 || choice.Amp == 2 {
						want = []string{"魔法暴击"}
					} else {
						want = []string{"物理暴击"}
					}
				case 2:
					if choice.Amp == 4 || choice.Amp == 2 {
						want = []string{"施放速度", "施放"}
					} else {
						want = []string{"攻击速度"}
					}
				case 8:
					want = []string{"移动速度"}
				case 16:
					want = []string{"白金徽章[冒险家]", "白金徽章[全属性]"}
				}
				selected := emblemChoice{}
				for _, candidate := range emblemPool {
					if candidate.mask&mask == 0 {
						continue
					}
					matches := false
					for _, key := range want {
						if strings.Contains(candidate.name, key) {
							matches = true
						}
					}
					if matches {
						selected = candidate
						break
					}
				}
				if selected.id == 0 && mask == 16 {
					for _, candidate := range emblemPool {
						if candidate.mask == 16 && strings.Contains(candidate.name, "冒险家") {
							selected = candidate
							break
						}
					}
				}
				if selected.id == 0 {
					t.Fatal("no appropriate source emblem", choice.Name, mask, want)
				}
				if e = f.changeRobot(townRobotChange{ID: id, Revision: m.records[id].Revision, Action: "grant", Template: selected.id, Amount: 1}); e != nil {
					t.Fatal(e)
				}
				current, e := inventory.ReadBag(m.records[id].Role.State)
				if e != nil {
					t.Fatal(e)
				}
				found := false
				ownedSlot := uint16(0)
				for _, stack := range current.Items {
					if stack.Template == selected.id && stack.Amount > 0 {
						found = true
						ownedSlot = stack.Slot
						break
					}
				}
				if !found {
					t.Fatal("emblem grant absent")
				}
				req := protocol.UseEmblemRequest{Space: 3, AvatarSlot: it.Slot, Template: it.Template, Inputs: []protocol.UseEmblemInput{{Slot: ownedSlot, Template: selected.id, Socket: byte(socket)}}}
				if e = f.changeRobot(townRobotChange{ID: id, Revision: m.records[id].Revision, Action: "emblem", Emblem: req}); e != nil {
					t.Fatal(choice.Name, selected.name, e)
				}
				emblemReport = append(emblemReport, map[string]any{"avatar_slot": it.Slot, "socket": socket, "template": selected.id, "name": selected.name, "path": selected.path, "sha256": selected.sha})
			}
		}
		if len(emblemReport) < 16 {
			t.Fatal("missing source-opened emblems", id, len(emblemReport))
		}
		// Reuse ordinary item/instance operations; no synthetic attack multiplier.

		r = m.records[id]
		afterBag, e := inventory.ReadBag(r.Role.State)
		if e != nil {
			t.Fatal(e)
		}
		for _, old := range beforeBag.Worn {
			if old.Slot <= 8 {
				continue
			}
			equal := false
			for _, now := range afterBag.Worn {
				if old.Slot == now.Slot {
					l, _ := json.Marshal(old)
					rr, _ := json.Marshal(now)
					equal = bytes.Equal(l, rr)
				}
			}
			if !equal {
				t.Fatal("non-costume equipment changed", id, old.Slot)
			}
		}
		beforeRole.State = nil
		afterRole := r.Role
		afterRole.State = nil
		l, _ := json.Marshal(beforeRole)
		rr, _ := json.Marshal(afterRole)
		if !bytes.Equal(l, rr) {
			t.Fatal("role identity changed")
		}
		projection := r.Role
		projection.WireID = uint16(60000 + id)
		addition, e := s.TownRobotEntryAddition(projection)
		if e != nil {
			t.Fatal(e)
		}
		basic, e := s.EntryBasicProbe(projection, [2]byte{})
		if e != nil {
			t.Fatal(e)
		}
		companion, e := s.TownRobotCompanionSnapshot(projection)
		if e != nil {
			t.Fatal(e)
		}
		types := map[uint32]int{}
		slots := map[uint16]bool{}
		for _, it := range companion.Worn {
			d, e := c.Selection.Definition(it.Template)
			if e != nil {
				t.Fatal(e)
			}
			kind := d.Fields["[equipment type]"]
			slot, ok := s.WearRules.Slots[kind[0].Text]
			if !ok {
				t.Fatal("slot")
			}
			types[it.Template] = int(slot)
			slots[it.Slot] = true
		}
		vector, _ := json.MarshalIndent(map[string]any{"payload_hex": hex.EncodeToString(addition), "types": types, "slots": slots, "source_sha256": c.SourceChecksum, "role": projection, "basic": basic, "companion": companion}, "", "  ")
		if e = os.WriteFile(filepath.Join(filepath.Dir(input), fmt.Sprintf("r28-entry-%d.json", id)), vector, 0600); e != nil {
			t.Fatal(e)
		}
		outfitName, _, e := sourceName(selected.Package)
		if e != nil {
			t.Fatal(e)
		}
		report = append(report, map[string]any{"id": id, "career": choice.Name, "job": choice.Job, "outfit": selected, "outfit_name": outfitName, "emblems": emblemReport})
	}
	if len(used) != 69 {
		t.Fatal("visual outfits not unique", len(used))
	}
	data, e := json.MarshalIndent(townRobotArchive{Version: 1, Records: m.records}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	output := filepath.Join(filepath.Dir(input), "r28-wardrobe-prepared.json")
	if e = os.WriteFile(output, data, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = readTownRobotArchive(output); e != nil {
		t.Fatal(e)
	}
	now, e := os.ReadFile(input)
	if e != nil || !bytes.Equal(now, original) {
		t.Fatal("input changed")
	}
	data, _ = json.MarshalIndent(map[string]any{"profiles": report, "library": libraries, "distinct_visual_outfits": len(used), "seed": string(seed), "source_checksum": c.SourceChecksum, "non_costume_worn_unchanged": true, "player_database_opened": false}, "", "  ")
	if e = os.WriteFile(filepath.Join(filepath.Dir(input), "r28-wardrobe-report.json"), data, 0600); e != nil {
		t.Fatal(e)
	}

}
