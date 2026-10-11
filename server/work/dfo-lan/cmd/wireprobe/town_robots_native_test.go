package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestTownRobotNativeArchive(t *testing.T) {
	archive := os.Getenv("DFO_TOWN_ROBOT_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set DFO_TOWN_ROBOT_TEST_ARCHIVE for read-only current-PVF actor validation")
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "world,characters,progression,items,quests,loot,equipment,equipment-selection,fame", ArchivePath: archive, ArchiveChecksum: os.Getenv("DFO_TOWN_ROBOT_TEST_SHA256"), CharacterPolicyPath: "../../configs/pvf-character-policy.json", DropPolicyPath: "../../configs/pvf-drop-policy.json", DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	if c.Characters == nil || c.Progression == nil || c.World == nil {
		t.Fatal("source domains missing")
	}
	rules, err := character.LoadGrowthRules("../../configs/experience.compat90.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &character.Service{Catalog: *c.Characters}
	c.Selection.Full = c.Equipment
	s.Equipment = c.Selection
	s.WearRules, err = inventory.LoadWearRules("../../configs/equipment-wear.full-candidate.json", c.SourceChecksum)
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.InstallFameRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	growth := &character.ProgressionService{Catalog: *c.Progression, Professions: *c.Characters, Rules: rules}
	jobs := make([]int, 0, len(c.Characters.Professions))
	for job := range c.Characters.Professions {
		jobs = append(jobs, int(job))
	}
	sort.Ints(jobs)
	for _, level := range []byte{1, 15, 55, 100, 115} {
		for _, job := range jobs {
			role, err := s.TownDisplayCharacter(uint16(60000+job), "旅人测试", byte(job), level, growth)
			if err != nil {
				t.Fatalf("source job=%d level=%d: %v", job, level, err)
			}
			if _, err = s.EntryBasicProbe(role, [2]byte{}); err != nil {
				t.Fatal(job, level, err)
			}
			if _, err = s.EntryAddition(role); err != nil {
				t.Fatal(job, level, err)
			}
		}
	}
	areas := 0
	for _, a := range c.World.Areas {
		if a.SeriaReturnWarp {
			continue
		}
		r, ok := robotRectangle(a)
		if !ok {
			continue
		}
		for seed := uint32(0); seed < 50; seed++ {
			x, y := robotPoint(r, seed)
			found := false
			for _, raw := range a.Walkable {
				if robotPointInSource(raw, x, y) {
					found = true
					break
				}
			}
			if !found {
				t.Fatal("geometry escaped source", catalog.AreaKey(a.Town, a.Area))
			}
		}
		areas++
	}
	t.Logf("native professions=%d levels=5 public areas with bounded geometry=%d; no store/listener opened", len(jobs), areas)
	t.Run("managed_source_weapon_avatar_and_reload", func(t *testing.T) {
		bagRules, err := inventory.LoadBagRules("../../configs/inventory.current37.json")
		if err != nil {
			t.Fatal(err)
		}
		bagRules.Source = c.SourceChecksum
		role, err := s.TownDisplayCharacter(60001, "旅人0001", byte(jobs[0]), 115, growth)
		if err != nil {
			t.Fatal(err)
		}
		role.ID = -1
		role.WireID = 0
		m := &townRobotManager{path: filepath.Join(t.TempDir(), "robots.json"), records: map[int]townRobotRecord{}, characters: s, growth: growth, award: &inventory.Awarder{Catalog: *c.Loot, Rules: bagRules, Equipment: c.Selection}, wear: &inventory.WearService{Catalog: c.Selection, Professions: *c.Characters, BagRules: bagRules, Rules: s.WearRules}}
		if err = m.store(townRobotRecord{ID: 1, Revision: 1, Role: role}); err != nil {
			t.Fatal(err)
		}
		f := &townRobotFleet{hub: newLanHub(), manager: m, stop: make(chan struct{})}
		weapon := uint32(0)
		for _, candidate := range c.Selection.DropPool() {
			kind, _ := c.Selection.EquipmentKind(candidate.ID)
			if kind != "[weapon]" {
				continue
			}
			raw, _, e := m.award.Grant(role.State, candidate.ID, 1)
			if e != nil {
				continue
			}
			b, _ := inventory.ReadBag(raw)
			var granted inventory.BagEquipment
			for _, it := range b.Equipment {
				if it.Template == candidate.ID {
					granted = it
					break
				}
			}
			move := protocol.ItemMoveRequest{SourceSlot: granted.Slot, SourceItem: candidate.ID, DestinationList: 3, DestinationSlot: s.WearRules.Slots["[weapon]"], Count: 1, Selection: 0xffffffff}
			for _, it := range b.Worn {
				if it.Slot == move.DestinationSlot {
					move.DestinationItem = it.Template
				}
			}
			if _, e = m.wear.MoveOrdinary(inventory.Role{Profession: role.Profession, ConfigVersion: role.ConfigVersion, State: raw}, move); e == nil {
				weapon = candidate.ID
				break
			}
		}
		if weapon == 0 {
			t.Fatal("no source weapon fits synthetic profession")
		}
		if err = f.changeRobot(townRobotChange{ID: 1, Revision: 1, Action: "grant", Template: weapon, Amount: 1}); err != nil {
			t.Fatal(err)
		}
		bag, _ := inventory.ReadBag(m.records[1].Role.State)
		var granted inventory.BagEquipment
		for _, e := range bag.Equipment {
			if e.Template == weapon {
				granted = e
				break
			}
		}
		old := uint32(0)
		for _, e := range bag.Worn {
			if e.Slot == s.WearRules.Slots["[weapon]"] {
				old = e.Template
			}
		}
		if err = f.changeRobot(townRobotChange{ID: 1, Revision: 2, Action: "move", Move: protocol.ItemMoveRequest{SourceSlot: granted.Slot, SourceItem: weapon, DestinationList: 3, DestinationSlot: s.WearRules.Slots["[weapon]"], DestinationItem: old, Count: 1, Selection: 0xffffffff}}); err != nil {
			t.Fatal(err)
		}
		ids := []uint32{}
		for id, item := range c.Items.Items {
			if item.Kind == "avatar" {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		avatar := uint32(0)
		reasons := map[string]int{}
		var avatarMove protocol.ItemMoveRequest
		for _, id := range ids {
			if avatar != 0 {
				break
			}
			raw, grantErr := m.grant(m.records[1].Role.State, id, 1)
			if grantErr != nil {
				reasons["grant: "+grantErr.Error()]++
				continue
			}
			kind, kindErr := c.Selection.EquipmentKind(id)
			slot, slotOK := s.WearRules.Slots[kind]
			if kindErr != nil || !slotOK || slot > 11 {
				reasons["unmapped kind "+kind]++
				continue
			}
			b, _ := inventory.ReadBag(raw)
			var row inventory.BagEquipment
			for _, e := range b.Special[1] {
				if e.Template == id {
					row = e
					break
				}
			}
			if row.Template == 0 {
				reasons["not in avatar bag"]++
				continue
			}
			move := protocol.ItemMoveRequest{SourceList: 1, SourceSlot: row.Slot, SourceItem: id, DestinationList: 3, DestinationSlot: slot, Count: 1, Selection: 0xffffffff}
			for _, e := range b.Worn {
				if e.Slot == slot && e.Group == 1 {
					move.DestinationItem = e.Template
				}
			}
			_, wearErr := m.wear.MoveOrdinary(inventory.Role{Profession: role.Profession, ConfigVersion: role.ConfigVersion, State: raw}, move)
			if wearErr != nil {
				reasons["wear: "+wearErr.Error()]++
			}
			if wearErr == nil {
				avatar = id
				avatarMove = move
			}
		}
		if avatar == 0 {
			t.Fatalf("no source avatar fits synthetic profession: %v; candidates=%d", reasons, len(ids))
		}
		if err = f.changeRobot(townRobotChange{ID: 1, Revision: 3, Action: "grant", Template: avatar, Amount: 1}); err != nil {
			t.Fatal(err)
		}
		if err = f.changeRobot(townRobotChange{ID: 1, Revision: 4, Action: "move", Move: avatarMove}); err != nil {
			t.Fatal(err)
		}
		saved, err := readTownRobotArchive(m.path)
		if err != nil || saved[1].Revision != 5 {
			t.Fatal(err)
		}
		projection := saved[1].Role
		projection.WireID = 60001
		snapshot, err := s.TownRobotCompanionSnapshot(projection)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.TagCharacterSnapshot(projection); err == nil {
			t.Fatal("synthetic record passed account companion gate")
		}
		encoded, _ := json.Marshal(snapshot)
		if len(encoded) == 0 {
			t.Fatal("empty snapshot")
		}
		info, err := s.TownRobotInfo115(projection, [2]byte{3, 86})
		if err != nil {
			t.Fatal("current source native inspection:", err)
		}
		if path := os.Getenv("DFO_ROBOT_INSPECT_VECTOR_OUTPUT"); path != "" {
			types := map[uint32]int{}
			for _, item := range snapshot.Worn {
				kind, err := c.Selection.EquipmentKind(item.Template)
				if err != nil {
					t.Fatal(err)
				}
				slot, ok := s.WearRules.Slots[kind]
				if !ok {
					t.Fatal("no source kind mapping", kind)
				}
				types[item.Template] = int(slot)
			}
			vector, _ := json.Marshal(map[string]any{"payload_hex": fmt.Sprintf("%x", info), "types": types, "source_sha256": c.SourceChecksum})
			if err := os.WriteFile(path, vector, 0600); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("current PVF weapon=%d avatar=%d grant/wear/reload/source companion serialization passed; no native party enabled", weapon, avatar)
	})
}

func robotPointInSource(r [4]int32, x, y uint16) bool {
	px, py := int64(int16(x)), int64(int16(y))
	return px >= int64(r[0]) && py >= int64(r[1]) && px <= int64(r[0])+int64(r[2]) && py <= int64(r[1])+int64(r[3])
}
