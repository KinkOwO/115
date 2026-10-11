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

func TestTownRobotDiversityPreparation(t *testing.T) {
	input := os.Getenv("DFO_ROBOT_DIVERSITY_FIXTURE")
	if input == "" {
		t.Skip("explicit offline fixture")
	}
	if filepath.Base(input) != "r26-diversity-input.json" {
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
	// Explicit operator loadout choices. Source definitions and ordinary wear/
	// advancement/skill services remain the authority; no runtime content table.
	jobs := []byte{0, 2, 3, 1, 4, 5, 6}
	advances := map[byte]byte{0: 3, 1: 1, 2: 1, 3: 1, 4: 3, 5: 1, 6: 1}
	report := []any{}
	profiles := []townRobotRecord{}
	for n := 0; n < 21; n++ {
		id := n + 1
		job := jobs[n%len(jobs)]
		advance := advances[job]
		variant := n / len(jobs)
		r := seed
		r.ID = id
		r.Revision = 1
		prof := c.Characters.Professions[job]
		if prof.RawSHA256 == "" {
			t.Fatal("missing source profession", job)
		}
		if n > 0 {
			role, e := s.TownDisplayCharacter(uint16(60000+id), fmt.Sprintf("旅人%04d", id), job, 115, growth)
			if e != nil {
				t.Fatal(id, e)
			}
			role.State, e = s.ApplyAdvancement(role, advance)
			if e != nil {
				t.Fatal(id, e)
			}
			for stage := byte(1); stage <= 3; stage++ {
				role.State, e = s.ApplyAwakening(role, stage)
				if e != nil {
					t.Fatal(id, e)
				}
			}
			role = learnNaturalRobot(t, s, c, role, advance, script)
			// Fresh robot only: deep-copy shared combat gear, retaining solely items
			// whose actual source job/advance requirements match this profession.
			copied, _ := json.Marshal(seedBag)
			var b inventory.Bag
			if e = json.Unmarshal(copied, &b); e != nil {
				t.Fatal(e)
			}
			b.Worn = nil
			for _, item := range seedBag.Worn {
				d, e := c.Selection.Definition(item.Template)
				if e != nil {
					t.Fatal(e)
				}
				kind := d.Fields["[equipment type]"]
				if item.Slot < 12 || item.Slot == 12 || item.Slot == 13 {
					continue
				}
				if len(kind) == 0 {
					t.Fatal("source kind missing", item.Template)
				}
				if inventory.WearableBy(d.Fields, kind[0].Text, prof.Job, advance, 115) == nil {
					b.Worn = append(b.Worn, item)
				}
			}
			role.State, e = inventory.SaveBag(role.State, b)
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
		chosen := []uint32{}
		if n > 0 {
			// Match the source advancement's starter weapon subtype when available.
			weaponSubtype := int32(-1)
			if first := prof.CreateEquipmentBySlot["[weapon]"][advance]; first != 0 {
				d, e := c.Selection.Definition(first)
				if e == nil && len(d.Fields["[sub type]"]) > 0 {
					weaponSubtype = d.Fields["[sub type]"][0].Value
				}
			}
			weapon := uint32(0)
			for _, ref := range audit.WeaponRefs {
				d, e := c.Selection.Definition(uint32(ref.ID))
				if e != nil {
					t.Fatal(e)
				}
				level, rarity, kind := d.Fields["[minimum level]"], d.Fields["[rarity]"], d.Fields["[equipment type]"]
				if len(level) != 1 || level[0].Value != 115 || len(rarity) != 1 || rarity[0].Value != 4 || len(kind) == 0 || kind[0].Text != "[weapon]" {
					continue
				}
				if inventory.WearableBy(d.Fields, "[weapon]", prof.Job, advance, 115) != nil {
					continue
				}
				subtype := d.Fields["[sub type]"]
				if weaponSubtype >= 0 && (len(subtype) == 0 || subtype[0].Value != weaponSubtype) {
					continue
				}
				weapon = uint32(ref.ID)
				break
			}
			if weapon == 0 {
				t.Fatal("no source 115 epic weapon", job, prof.Job, weaponSubtype)
			}
			chosen = append(chosen, weapon)
			slots := map[uint16][]uint32{}
			titles := []uint32{}
			auras := []uint32{}
			for _, row := range audit.Items {
				d, e := c.Selection.Definition(uint32(row.ID))
				if e != nil {
					t.Fatal(e)
				}
				kind := d.Fields["[equipment type]"]
				if len(kind) == 0 {
					continue
				}
				if inventory.WearableBy(d.Fields, kind[0].Text, prof.Job, advance, 115) != nil {
					continue
				}
				slot, ok := s.WearRules.Slots[kind[0].Text]
				if !ok {
					continue
				}
				if slot < 8 {
					mall := strings.Contains(d.Path, "dragonrobe4")
					if (variant == 0) != mall {
						continue
					}
					slots[slot] = append(slots[slot], uint32(row.ID))
				} else if slot == 9 {
					auras = append(auras, uint32(row.ID))
				} else if slot == 13 && strings.Contains(d.Path, "cn115") {
					titles = append(titles, uint32(row.ID))
				}
			}
			for slot := uint16(0); slot < 8; slot++ {
				pool := slots[slot]
				if len(pool) == 0 {
					t.Fatal("no source costume", job, variant, slot)
				}
				chosen = append(chosen, pool[(n*3+int(slot)*5)%len(pool)])
			}
			if len(titles) < 4 || len(auras) < 3 {
				t.Fatal("insufficient title/aura source", job, len(titles), len(auras))
			}
			chosen = append(chosen, titles[n%len(titles)], auras[(n+variant)%len(auras)])
		}
		sourceReport := []any{}
		for _, template := range chosen {
			d, e := c.Selection.Definition(template)
			if e != nil {
				t.Fatal(e)
			}
			slot, ok := s.WearRules.Slots[d.Fields["[equipment type]"][0].Text]
			if !ok {
				t.Fatal("no wear slot", template)
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
			for _, v := range b.Equipment {
				if v.Template == template {
					item = v
					found = true
					break
				}
			}
			if !found {
				for k, pool := range b.Special {
					for _, v := range pool {
						if v.Template == template {
							item = v
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
				t.Fatal("grant absent", template)
			}
			old := uint32(0)
			for _, v := range b.Worn {
				if v.Slot == slot {
					old = v.Template
				}
			}
			move := protocol.ItemMoveRequest{SourceList: list, SourceSlot: item.Slot, SourceItem: template, DestinationList: 3, DestinationSlot: slot, DestinationItem: old, Count: 1, Selection: 0xffffffff}
			if e = f.changeRobot(townRobotChange{ID: id, Revision: m.records[id].Revision, Action: "move", Move: move}); e != nil {
				t.Fatal(id, template, e)
			}
			sourceReport = append(sourceReport, map[string]any{"template": template, "slot": slot, "path": d.Path, "source_sha256": d.SHA256, "variation": d.Fields["[variation]"]})
		}
		r = m.records[id]
		profiles = append(profiles, r)
		var state character.State
		if err = json.Unmarshal(r.Role.State, &state); err != nil {
			t.Fatal(err)
		}
		if state.Advancement != advance || state.Awakening != 3 || state.Level != 115 {
			t.Fatal("progression", id)
		}
		report = append(report, map[string]any{"id": id, "job": job, "source_job": prof.Job, "advance": advance, "appearance_variant": variant, "loadout_source": sourceReport})
	}
	output := map[int]townRobotRecord{}
	for id := 1; id <= 50; id++ {
		data, _ := json.Marshal(profiles[(id-1)%len(profiles)])
		var r townRobotRecord
		if err = json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
		r.ID = id
		r.Role.ID = -int64(id)
		r.Role.Name = fmt.Sprintf("旅人%04d", id)
		r.Revision = 1
		if id == 1 {
			r.Role.State = append(json.RawMessage(nil), seed.Role.State...)
		}
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
		b, e := inventory.ReadBag(r.Role.State)
		if e != nil || len(b.Worn) < 35 {
			t.Fatal("combat loadout missing", id, len(b.Worn), e)
		}
		for _, item := range b.Worn {
			d, e := c.Selection.Definition(item.Template)
			if e != nil {
				t.Fatal(e)
			}
			kind := d.Fields["[equipment type]"]
			if e = inventory.WearableBy(d.Fields, kind[0].Text, c.Characters.Professions[r.Role.Profession].Job, advances[r.Role.Profession], 115); e != nil {
				t.Fatal("incompatible worn", id, item.Template, e)
			}
		}
		if id <= 21 {
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
			vector, _ := json.MarshalIndent(map[string]any{"payload_hex": hex.EncodeToString(addition), "types": types, "slots": slots, "source_sha256": c.SourceChecksum, "role": projection, "basic": basic, "companion": companion}, "", "  ")
			if e = os.WriteFile(filepath.Join(filepath.Dir(input), fmt.Sprintf("r26-entry-%d.json", id)), vector, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	data, err := json.MarshalIndent(townRobotArchive{Version: 1, Records: output}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(filepath.Dir(input), "r26-diversity-prepared.json")
	if err = os.WriteFile(out, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readTownRobotArchive(out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output[1].Role.State, seed.Role.State) {
		t.Fatal("robot1 changed")
	}
	now, e := os.ReadFile(input)
	if e != nil || !bytes.Equal(original, now) {
		t.Fatal("input changed")
	}
	data, _ = json.MarshalIndent(map[string]any{"profiles": report, "records": 50, "jobs": jobs, "robot1_preserved": true, "player_database_opened": false}, "", "  ")
	if e = os.WriteFile(filepath.Join(filepath.Dir(input), "r26-diversity-report.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
}
