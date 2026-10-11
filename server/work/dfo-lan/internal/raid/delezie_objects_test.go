package raid_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"dfolan/internal/raid"
	"os"
	"reflect"
	"testing"
)

func objectStateFixture(t *testing.T) (*catalog.DelezieRaidRules, *catalog.DelezieModeRules) {
	t.Helper()
	rows := []pvf.CTPRecord{{Index: 0, Parent: -1, Name: "[OBJECTS]", Refs: []pvf.CTPRef{{Name: "[OBJECT]", Values: []uint64{1}}}},
		{Index: 1, Parent: 0, Name: "[OBJECT]", Refs: []pvf.CTPRef{{Name: "[INDEX]", Values: []uint64{2}}, {Name: "[STATIC HP]", Values: []uint64{3}}, {Name: "[CREATABLE LOCATION INDEX]", Values: []uint64{4}}, {Name: "[DUNGEON INDEX]", Values: []uint64{5}}}},
		{Index: 2, Parent: 1, Name: "[INDEX]", Cells: []pvf.CTPCell{scalarNumber(3)}},
		{Index: 3, Parent: 1, Name: "[STATIC HP]", Cells: []pvf.CTPCell{{Tag: 4, Kind: "float", Float: 4294967297}}},
		{Index: 4, Parent: 1, Name: "[CREATABLE LOCATION INDEX]", Cells: []pvf.CTPCell{scalarNumber(7), scalarNumber(8)}},
		{Index: 5, Parent: 1, Name: "[DUNGEON INDEX]", Cells: []pvf.CTPCell{scalarNumber(15)}},
	}
	tree, err := pvf.NewCTPTree(&pvf.CTPTable{Path: "fixture.single.object.ctp", Records: rows})
	if err != nil {
		t.Fatal(err)
	}
	mode := &catalog.DelezieModeRules{Difficulty: catalog.DelezieSingle, Objects: tree, Phases: []catalog.DeleziePhaseRules{{Locations: map[uint32]catalog.DelezieLocationRule{7: {ID: 7}, 9: {ID: 9}}}}}
	rules := &catalog.DelezieRaidRules{Modes: map[catalog.DelezieDifficulty]*catalog.DelezieModeRules{catalog.DelezieSingle: mode}}
	return rules, mode
}

func TestDelezieObjectTransactionsAndSourceSnapshots(t *testing.T) {
	rules, mode := objectStateFixture(t)
	state, err := raid.NewDelezieObjects(rules, mode, 0)
	if err != nil {
		t.Fatal(err)
	}
	mode.Objects.Table.Records[3].Cells[0] = scalarNumber(11)
	mode.Phases[0].Locations[8] = catalog.DelezieLocationRule{ID: 8}
	if got, err := state.Spawn([]raid.DelezieObjectSpawn{{Definition: 3, Location: 7}, {Definition: 3, Location: 8}}); err == nil || got != nil || len(state.Snapshot()) != 0 {
		t.Fatal("failed spawn partially committed or borrowed changed locations")
	}
	objects, err := state.Spawn([]raid.DelezieObjectSpawn{{Definition: 3, Location: 7}})
	if err != nil || objects[0].Key != 1 || objects[0].Dungeon != 15 || objects[0].HP != 4294967297 {
		t.Fatalf("source snapshot or identity: %+v %v", objects, err)
	}
	before := state.Snapshot()
	if _, err := state.SetHealth(1, -1); err == nil {
		t.Fatal("negative health accepted")
	}
	if _, err := state.SetHealth(1, 4294967298); err == nil {
		t.Fatal("health above maximum accepted")
	}
	if err := state.Remove([]uint32{1, 99}); err == nil || !reflect.DeepEqual(before, state.Snapshot()) {
		t.Fatal("partial removal")
	}
	if err := state.Remove([]uint32{1, 1}); err == nil {
		t.Fatal("duplicate removal accepted")
	}
	if object, err := state.SetHealth(1, 0); err != nil || object.HP != 0 || len(state.Snapshot()) != 1 {
		t.Fatal("zero health implied unsupported automatic removal")
	}
	if err := state.Remove([]uint32{1}); err != nil {
		t.Fatal(err)
	}
	objects, err = state.Spawn([]raid.DelezieObjectSpawn{{Definition: 3, Location: 7}})
	if err != nil || objects[0].Key != 2 {
		t.Fatal("removed identity reused", objects, err)
	}
	if _, err = state.SetHealth(1, 1); err == nil {
		t.Fatal("stale object report accepted")
	}
	state.Snapshot()[2] = raid.DelezieObjectInstance{}
	if state.Snapshot()[2].Definition != 3 {
		t.Fatal("snapshot exposed mutable state")
	}
	changed, err := raid.NewDelezieObjects(rules, mode, 0)
	if err != nil {
		t.Fatal(err)
	}
	objects, err = changed.Spawn([]raid.DelezieObjectSpawn{{Definition: 3, Location: 8}})
	if err != nil || objects[0].HP != 11 {
		t.Fatal("new source ignored", objects, err)
	}
	foreign := *mode
	if _, err := raid.NewDelezieObjects(rules, &foreign, 0); err == nil {
		t.Fatal("foreign mode accepted")
	}
	if _, err := raid.NewDelezieObjects(rules, mode, 1); err == nil {
		t.Fatal("missing phase accepted")
	}
	if _, err := state.Spawn([]raid.DelezieObjectSpawn{{Definition: 3, Location: 9}}); err == nil {
		t.Fatal("undeclared object spawn pool location accepted")
	}
}

func TestDelezieObjectStateNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("current read-only native PVF required")
	}
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: "-"})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	directory, err := source.ChannelDirectory()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := source.RaidEntrances(directory)
	if err != nil {
		t.Fatal(err)
	}
	rules := entries[112].Delezie
	for difficulty := catalog.DelezieNormal; difficulty <= catalog.DelezieMatch; difficulty++ {
		t.Run(difficulty.Name(), func(t *testing.T) {
			mode := rules.Modes[difficulty]
			state, err := raid.NewDelezieObjects(rules, mode, 0)
			if err != nil {
				t.Fatal(err)
			}
			for row, action := range mode.SummonActions {
				operands, err := state.SummonOperands(row, nil)
				if err != nil || len(operands) != len(action.Pairs) || len(state.Snapshot()) != 0 {
					t.Fatal("native summon operands mutated objects or lost pair count", row, operands, err)
				}
				for i, pair := range action.Pairs {
					if pair.First.IsSymbol || pair.Second.IsSymbol || operands[i] != [2]int32{pair.First.Value, pair.Second.Value} {
						t.Fatal("current native paired vector lost order or -1", row, operands)
					}
				}
			}
			var requests []raid.DelezieObjectSpawn
			for _, definition := range mode.ObjectDefinitions {
				if !definition.HasStaticHP || definition.StaticHP < 0 {
					continue
				}
				for _, location := range definition.CreatableLocations {
					if _, ok := mode.Phases[0].Locations[location]; ok {
						requests = append(requests, raid.DelezieObjectSpawn{Definition: definition.ID, Location: location})
						break
					}
				}
			}
			objects, err := state.Spawn(requests)
			if err != nil || len(objects) == 0 {
				t.Fatal("source static object lifecycle", err)
			}
			var keys []uint32
			for _, object := range objects {
				definition := mode.ObjectDefinitions[object.Definition]
				if object.HP != definition.StaticHP || object.Dungeon != definition.Dungeon {
					t.Fatal("spawn ignored source", object)
				}
				if _, err := state.SetHealth(object.Key, object.HP/2); err != nil {
					t.Fatal(err)
				}
				keys = append(keys, object.Key)
			}
			if err := state.Remove(keys); err != nil || len(state.Snapshot()) != 0 {
				t.Fatal("object removal failed", err)
			}
			t.Logf("mode=%s static-source-objects=%d complete-initialization=false", difficulty.Name(), len(objects))
		})
	}
}

func TestDeleziePresetExecutionPreservesSourceOrderAndAtomicity(t *testing.T) {
	rules, mode := objectStateFixture(t)
	rows := mode.Objects.Table.Records
	rows = append(rows,
		pvf.CTPRecord{Index: 6, Parent: -1, Name: "[OBJECT PRESETS]", Refs: []pvf.CTPRef{{Name: "[OBJECT PRESET]", Values: []uint64{7}}}},
		pvf.CTPRecord{Index: 7, Parent: 6, Name: "[OBJECT PRESET]", Refs: []pvf.CTPRef{{Name: "[INDEX]", Values: []uint64{8}}, {Name: "[OBJECT INDEX]", Values: []uint64{9}}}},
		pvf.CTPRecord{Index: 8, Parent: 7, Name: "[INDEX]", Cells: []pvf.CTPCell{scalarNumber(4)}},
		pvf.CTPRecord{Index: 9, Parent: 7, Name: "[OBJECT INDEX]", Cells: []pvf.CTPCell{scalarNumber(3), scalarNumber(3)}},
	)
	var err error
	mode.Objects, err = pvf.NewCTPTree(&pvf.CTPTable{Path: "preset.ctp", Records: rows})
	if err != nil {
		t.Fatal(err)
	}
	state, err := raid.NewDelezieObjects(rules, mode, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range [][]raid.DeleziePresetSpawn{
		{{Preset: 4, Locations: []uint32{7, 7}}, {Preset: 9, Locations: []uint32{7}}},
		{{Preset: 4, Locations: []uint32{7}}},
		{{Preset: 4, Locations: []uint32{7, 9}}},
	} {
		if _, err := state.SpawnPresets(request); err == nil || len(state.Snapshot()) != 0 {
			t.Fatal("partial preset or bad pool accepted", err)
		}
	}
	// The owned execution snapshot must retain the original two entries.
	mode.Objects.Table.Records[9].Cells = []pvf.CTPCell{scalarNumber(3)}
	objects, err := state.SpawnPresets([]raid.DeleziePresetSpawn{{Preset: 4, Locations: []uint32{7, 7}}})
	if err != nil || len(objects) != 2 || objects[0].Key != 1 || objects[1].Key != 2 || objects[0].Definition != 3 || objects[1].Definition != 3 {
		t.Fatal(objects, err)
	}
	changed, err := raid.NewDelezieObjects(rules, mode, 0)
	if err != nil {
		t.Fatal(err)
	}
	objects, err = changed.SpawnPresets([]raid.DeleziePresetSpawn{{Preset: 4, Locations: []uint32{7}}})
	if err != nil || len(objects) != 1 {
		t.Fatal("new source preset list ignored", objects, err)
	}
}
