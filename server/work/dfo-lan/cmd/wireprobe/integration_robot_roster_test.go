package main

import (
	"bytes"
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

func TestIntegrationCleanRobotRoster(t *testing.T) {
	input := os.Getenv("INTEGRATION_ROBOT_ROSTER")
	if input == "" {
		t.Skip("explicit offline fixture")
	}
	if filepath.Base(input) != "r27-roster-input.json" {
		t.Fatal("offline fixture required")
	}
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "characters,progression,skills,items,quests,loot,equipment,equipment-selection,fame", ArchivePath: `G:/电脑问题/115US整合制作-20261010/resource-work/output/Script.inner.pvf`, ArchiveChecksum: "11eb18737c3f2d2687d76669fa2f4ddefc2c943eff826520709ca13569c9147d", CharacterPolicyPath: "../../configs/pvf-character-policy.json", DropPolicyPath: "../../configs/pvf-drop-policy.json", DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
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
	a, err := pvf.OpenReadOnly(pvf.Options{Path: `G:/电脑问题/115US整合制作-20261010/resource-work/output/Script.inner.pvf`, MaxBytes: 1 << 30}, c.SourceChecksum)
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
	m.items = &inventory.ItemService{Catalog: *c.Loot, Equipment: c.Selection, EmblemInlay: c.EmblemInlay}
	f := &townRobotFleet{manager: m, hub: newLanHub(), stop: make(chan struct{})}
	var audit struct {
		Items []struct {
			ID     int32                `json:"id"`
			Script catalog.ScriptRecord `json:"script"`
		} `json:"items"`
		WeaponRefs []struct {
			ID   int32  `json:"id"`
			Path string `json:"path"`
		} `json:"weapon_refs"`
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(input), "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	type recipe struct {
		Job     byte   `json:"job"`
		Advance byte   `json:"advance"`
		Name    string `json:"name"`
		Weapon  string `json:"weapon_family"`
		Amp     byte   `json:"amplify_type"`
		Fixed   bool   `json:"fixed_damage"`
		Support bool   `json:"support"`
	}
	recipes := []recipe{}
	raw, e := os.ReadFile(filepath.Join(filepath.Dir(input), "recipes.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &recipes); e != nil {
		t.Fatal(e)
	}
	// Current source package categories are intact eight-piece wardrobe sets.
	pack, e := catalog.ResolveScript(a, "stackable/dragonrobe4/930440001.stk")
	if e != nil {
		t.Fatal(e)
	}
	sets := [][]uint32{}
	for i, v := range pack.Cells {
		if v.Text != "[booster select category]" {
			continue
		}
		nums := []uint32{}
		for j := i + 1; j < len(pack.Cells) && pack.Cells[j].Text != "[/booster select category]"; j++ {
			if pack.Cells[j].Type == 0 {
				nums = append(nums, uint32(pack.Cells[j].Value))
			}
		}
		if len(nums) != 34 {
			t.Fatal("eight-piece package layout", len(nums))
		}
		set := []uint32{}
		for j := 2; j < len(nums); j += 4 {
			if nums[j+1] != 1 {
				t.Fatal("costume quantity")
			}
			set = append(set, nums[j])
		}
		sets = append(sets, set)
	}
	if len(sets) != 17 {
		t.Fatal("source costume categories", len(sets))
	}
	if err = inventory.LoadRefineRules("../../configs/refine.json"); err != nil {
		t.Fatal(err)
	}
	// Ordinary amplification operation consumes an actual source book. Its
	// source table permits this explicit administrator-selected high roll.
	idx, e := catalog.ResolveScript(a, "list/stackable.lst")
	if e != nil {
		t.Fatal(e)
	}
	refs, e := catalog.ParseIndex(idx.Cells)
	if e != nil {
		t.Fatal(e)
	}
	book := uint32(50022037)
	bookPath := ""
	for _, r := range refs {
		if uint32(r.ID) == book {
			bookPath = r.Path
			break
		}
	}
	bs, e := catalog.ResolveScript(a, bookPath)
	if e != nil {
		t.Fatal(e)
	}
	permitted := false
	for i, x := range bs.Cells {
		if x.Text == "[amplification random value]" {
			for j := i + 1; j+1 < len(bs.Cells) && bs.Cells[j].Type != 3; j += 2 {
				if bs.Cells[j].Value == 12 && bs.Cells[j+1].Value > 0 {
					permitted = true
				}
			}
		}
	}
	if !permitted {
		t.Fatal("source book does not permit chosen +12 roll")
	}
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
	report := []any{}
	output := map[int]townRobotRecord{}
	seen := map[[2]byte]bool{}
	jobs := map[byte]bool{}
	petsUsed := map[uint32]bool{}
	for n, choice := range recipes {
		id := n + 1
		job, advance := choice.Job, choice.Advance
		key := [2]byte{job, advance}
		if seen[key] {
			t.Fatal("duplicate career", key)
		}
		seen[key] = true
		jobs[job] = true
		prof := c.Characters.Professions[job]
		if prof.RawSHA256 == "" {
			t.Fatal("profession absent", job)
		}
		role, e := s.TownDisplayCharacter(uint16(60000+id), fmt.Sprintf("旅人%04d", id), job, 115, growth)
		if e != nil {
			t.Fatal(id, e)
		}
		if advance > 0 {
			role.State, e = s.ApplyAdvancement(role, advance)
			if e != nil {
				t.Fatal(id, choice.Name, e)
			}
		}
		for stage := byte(1); stage <= 3; stage++ {
			role.State, e = s.ApplyAwakening(role, stage)
			if e != nil {
				t.Fatal(id, choice.Name, e)
			}
		}
		role = learnNaturalRobot(t, s, c, role, advance, script)
		// Each role starts from newly generated source attributes/skills. No saved
		// inventory, currencies, item instances or dungeon progress is inherited.
		b, e := inventory.ReadBag(role.State)
		if e != nil {
			t.Fatal(e)
		}
		b.Worn = nil
		b.Equipment = nil
		b.Special = nil
		role.State, e = inventory.SaveBag(role.State, b)
		if e != nil {
			t.Fatal(e)
		}
		role.ID = -int64(id)
		role.AccountID = 0
		role.WireID = 0
		m.records[id] = townRobotRecord{ID: id, Revision: 1, Role: role}
		integrationGrantCommonRobotGear(t, f, id, filepath.Join(filepath.Dir(input), "common-gear-design.json"))
		weapon := uint32(0)
		bestScore := -1
		weaponName := ""
		for _, ref := range audit.WeaponRefs {
			if !strings.Contains(ref.Path, "/weapon/"+choice.Weapon+"/") {
				continue
			}
			d, e := c.Selection.Definition(uint32(ref.ID))
			if e != nil {
				t.Fatal(e)
			}
			lev, rar, kind := d.Fields["[minimum level]"], d.Fields["[rarity]"], d.Fields["[equipment type]"]
			if len(lev) != 1 || lev[0].Value != 115 || len(rar) != 1 || (rar[0].Value != 4 && rar[0].Value != 8) || len(kind) == 0 || kind[0].Text != "[weapon]" || inventory.WearableBy(d.Fields, "[weapon]", prof.Job, advance, 115) != nil {
				continue
			}
			mastery := inventory.RequiredJobSkill(d.Fields, prof.Job)
			if mastery != 0 {
				skill, ok, e := c.Learning.Definition(job, mastery)
				if e != nil {
					t.Fatal(e)
				}
				if !ok || (!skill.ForAdvancement(int(advance)) && !skill.ForAwakening(int(advance), 3)) {
					continue
				}
			}
			name, _, e := sourceName(d.Path)
			if e != nil {
				t.Fatal(e)
			}
			score := 0
			if rar[0].Value == 8 {
				score += 1000
			}
			if strings.Contains(name, "传世") {
				score += 100
			}
			if strings.Contains(name, "太初") {
				score += 20
			}
			if score > bestScore {
				weapon = uint32(ref.ID)
				bestScore = score
				weaponName = name
			}
		}
		if weapon == 0 {
			t.Fatal("no appropriate 115 weapon", id, choice)
		}
		equip(id, weapon)
		compatible := [][]uint32{}
		for _, set := range sets {
			slots := map[uint16]bool{}
			valid := true
			for _, template := range set {
				d, e := c.Selection.Definition(template)
				if e != nil {
					t.Fatal(e)
				}
				kind := d.Fields["[equipment type]"]
				if len(kind) == 0 || inventory.WearableBy(d.Fields, kind[0].Text, prof.Job, advance, 115) != nil {
					valid = false
					break
				}
				slot, ok := s.WearRules.Slots[kind[0].Text]
				if !ok || slot >= 8 || slots[slot] {
					valid = false
					break
				}
				slots[slot] = true
			}
			if valid && len(slots) == 8 {
				compatible = append(compatible, set)
			}
		}
		if len(compatible) == 0 {
			t.Fatal("no full source costume set", id, choice)
		}
		outfit := compatible[n%len(compatible)]
		for _, template := range outfit {
			equip(id, template)
		}
		titles, auras, pets := []uint32{}, []uint32{}, []uint32{}
		for _, row := range audit.Items {
			d, e := c.Selection.Definition(uint32(row.ID))
			if e != nil {
				t.Fatal(e)
			}
			kind := d.Fields["[equipment type]"]
			if len(kind) == 0 || inventory.WearableBy(d.Fields, kind[0].Text, prof.Job, advance, 115) != nil {
				continue
			}
			slot, ok := s.WearRules.Slots[kind[0].Text]
			if !ok {
				continue
			}
			if slot == 13 && strings.Contains(d.Path, "cn115") {
				titles = append(titles, uint32(row.ID))
			}
			if slot == 9 {
				auras = append(auras, uint32(row.ID))
			}
			if slot == 26 && (strings.Contains(d.Path, "2025") || strings.Contains(d.Path, "2026")) {
				sub := d.Fields["[sub type]"]
				atk := d.Fields["[physical attack]"]
				damage := d.Fields["[equipment damage rate]"]
				if len(sub) == 1 && sub[0].Value == 0 && len(atk) > 0 && atk[0].Value >= 100 && len(damage) > 0 && damage[0].Value >= 10 {
					pets = append(pets, uint32(row.ID))
				}
			}
		}
		if len(titles) < 10 || len(auras) < 3 || len(pets) < 5 {
			t.Fatal("insufficient strong cosmetics", id, len(titles), len(auras), len(pets))
		}
		title, aura, pet := titles[n%len(titles)], auras[n%len(auras)], pets[n%len(pets)]
		petsUsed[pet] = true
		equip(id, title)
		equip(id, aura)
		equip(id, pet)
		// Fill source-opened sockets using ordinary inlay operations and current
		// localized item names; actual socket/color eligibility stays in the domain.
		emblemReport := []any{}
		mainStat := map[byte]string{1: "体力", 2: "精神", 3: "力量", 4: "智力"}[choice.Amp]
		socketBag, e := inventory.ReadBag(m.records[id].Role.State)
		if e != nil {
			t.Fatal(e)
		}
		for _, it := range socketBag.Worn {
			if len(it.AvatarOptions) != 30 {
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
		r := m.records[id]
		b, e = inventory.ReadBag(r.Role.State)
		if e != nil {
			t.Fatal(e)
		}
		crafts := []any{}
		targets := []inventory.BagEquipment{}
		for _, it := range b.Worn {
			if it.Slot >= 12 && it.Slot <= 25 && it.Slot != 13 {
				targets = append(targets, it)
			}
		}
		for _, it := range targets {
			if e = f.changeRobot(townRobotChange{ID: id, Revision: m.records[id].Revision, Action: "grant", Template: book, Amount: 1}); e != nil {
				t.Fatal(e)
			}
			r = m.records[id]
			b, e = inventory.ReadBag(r.Role.State)
			if e != nil {
				t.Fatal(e)
			}
			bookSlot := uint16(0)
			found := false
			for _, item := range b.Items {
				if item.Template == book && item.Amount > 0 {
					bookSlot = item.Slot
					found = true
					break
				}
			}
			if !found {
				t.Fatal("source book grant")
			}
			next, receipt, e := m.wear.ApplyAmplifyGrimoire(inventory.Role{AccountID: 0, Profession: job, ConfigVersion: r.Role.ConfigVersion, State: r.Role.State}, protocol.AmplifyOptionRequest{Aux: 2, EquipmentSlot: it.Slot, EquipmentTemplate: it.Template, BookSlot: bookSlot, BookTemplate: book, Type: choice.Amp}, 12, false, false)
			if e != nil {
				t.Fatal(id, "amplify", e)
			}
			if receipt.AmplifyLevel != 12 || receipt.BookRemaining != 0 {
				t.Fatal("craft receipt")
			}
			r.Role.State = next
			m.records[id] = r
		}
		r = m.records[id]
		b, e = inventory.ReadBag(r.Role.State)
		if e != nil {
			t.Fatal(e)
		}
		for i := range b.Worn {
			it := &b.Worn[i]
			if it.Slot >= 12 && it.Slot <= 25 && it.Slot != 13 {
				level := byte(12)
				refine := byte(0)
				if it.Slot == 12 {
					level = 13
					if choice.Fixed || choice.Support {
						refine = 8
					}
				}
				inventory.SetCraftedStatus(it, level, refine)
				row := inventory.EquipmentRow(*it)
				if row[10]&31 != level || row[19] != choice.Amp || (refine > 0 && (it.Refine != 8 || row[76] != 8)) {
					t.Fatal("native crafted instance", id, it.Slot)
				}
				crafts = append(crafts, map[string]any{"slot": it.Slot, "amplify_level": level, "amplify_type": choice.Amp, "refine": refine})
			}
		}
		r.Role.State, e = inventory.SaveBag(r.Role.State, b)
		if e != nil {
			t.Fatal(e)
		}
		r.Revision++
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
		types := map[uint32]int{}
		slots := map[uint16]bool{}
		sources := []any{}
		for _, it := range companion.Worn {
			d, e := c.Selection.Definition(it.Template)
			if e != nil {
				t.Fatal(e)
			}
			kind := d.Fields["[equipment type]"]
			if len(kind) == 0 {
				t.Fatal("kind")
			}
			slot, ok := s.WearRules.Slots[kind[0].Text]
			if !ok {
				t.Fatal("slot")
			}
			if e = inventory.WearableBy(d.Fields, kind[0].Text, prof.Job, advance, 115); e != nil {
				t.Fatal("worn compatibility", id, it.Template, e)
			}
			types[it.Template] = int(slot)
			slots[it.Slot] = true
			sources = append(sources, map[string]any{"template": it.Template, "slot": it.Slot, "path": d.Path, "sha256": d.SHA256})
		}
		if len(companion.Worn) < 35 || !slots[26] || !slots[47] {
			t.Fatal("full companion loadout", id, len(companion.Worn))
		}
		vector, _ := json.MarshalIndent(map[string]any{"payload_hex": hex.EncodeToString(addition), "types": types, "slots": slots, "source_sha256": c.SourceChecksum, "role": projection, "basic": basic, "companion": companion}, "", "  ")
		if e = os.WriteFile(filepath.Join(filepath.Dir(input), fmt.Sprintf("r27-entry-%d.json", id)), vector, 0600); e != nil {
			t.Fatal(e)
		}
		report = append(report, map[string]any{"id": id, "job": job, "advance": advance, "career": choice.Name, "support": choice.Support, "fixed_damage": choice.Fixed, "weapon": weapon, "weapon_name": weaponName, "outfit_set": outfit, "title": title, "aura": aura, "pet": pet, "crafts": crafts, "emblems": emblemReport, "loadout_source": sources})
	}
	if len(output) != 69 || len(jobs) != 17 || len(petsUsed) < 5 {
		t.Fatal("roster coverage", len(output), len(jobs), len(petsUsed))
	}
	data, e := json.MarshalIndent(townRobotArchive{Version: 1, Records: output}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(filepath.Dir(input), "r27-roster-prepared.json")
	if e = os.WriteFile(out, data, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = readTownRobotArchive(out); e != nil {
		t.Fatal(e)
	}
	now, e := os.ReadFile(input)
	if e != nil || !bytes.Equal(original, now) {
		t.Fatal("input changed")
	}
	data, _ = json.MarshalIndent(map[string]any{"profiles": report, "records": len(output), "base_jobs": len(jobs), "distinct_pets": len(petsUsed), "player_database_opened": false, "full_costume_sets": true}, "", "  ")
	if e = os.WriteFile(filepath.Join(filepath.Dir(input), "r27-roster-report.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
}
