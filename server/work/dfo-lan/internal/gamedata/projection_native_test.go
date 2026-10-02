package gamedata

import (
	"dfolan/internal/adventure"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProjectionCachesLocalArchiveParity(t *testing.T) {
	requireHeavyArchiveSweep(t)
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for full native projection parity")
	}
	dir := t.TempDir()
	s, err := Open(Options{Mode: PVF, ArchivePath: p, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	joint, err := s.ItemCatalogs(catalog.ItemBasicOptions{Periods: true, Prices: true, Materials: true, Skins: true, Boosters: true}, true, "../../configs/pvf-enhancement-policy.json", true)
	if err != nil {
		t.Fatal(err)
	}
	index := joint.Basics.Index
	w, err := s.World("")
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.Quests("")
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Training []uint32 `json:"training_dungeons"`
		Disabled []uint32 `json:"disabled_full_dungeons"`
	}
	read := func(path string, v any) {
		t.Helper()
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(b, v); e != nil {
			t.Fatal(e)
		}
	}
	read("../../configs/pvf-scene-policy.json", &policy)
	excluded := append(policy.Training, policy.Disabled...)
	var warpPolicy catalog.ScriptWarpPolicy
	read("../../configs/pvf-script-warp-policy.json", &warpPolicy)
	type projections struct {
		Equipment inventory.PVFEquipmentProjection
		Loot      catalog.LootCatalog
		Dungeons  catalog.DungeonCatalog
		Season    *adventure.SeasonRules
		Roster    *character.RosterBackgroundTicketCatalog
		Warps     []catalog.ScriptWarpRoute
		Terminal  catalog.TerminalSceneOverlay
	}
	load := func(cache string) projections {
		t.Helper()
		s.cacheDir = cache
		var r projections
		check := func(e error) {
			t.Helper()
			if e != nil {
				t.Fatal(e)
			}
		}
		r.Equipment, err = cachedProjection(s, "equipment", itemIndexIdentity(index), func() (inventory.PVFEquipmentProjection, error) {
			return inventory.ProjectPVFEquipment(s.archive, index)
		}, func(r inventory.PVFEquipmentProjection) (inventory.PVFEquipmentProjection, error) {
			return r, inventory.ValidatePVFEquipmentProjection(s.archive, r)
		})
		check(err)
		r.Loot, err = s.Loot(130)
		check(err)
		r.Dungeons, err = s.RuntimeFullDungeons(w, excluded)
		check(err)
		r.Season, err = s.Season(index)
		check(err)
		r.Roster, err = s.RosterBackgrounds(index)
		check(err)
		r.Warps, err = s.ScriptWarpRoutes(r.Dungeons, warpPolicy)
		check(err)
		r.Terminal, err = s.TerminalScenes(r.Dungeons, q)
		check(err)
		return r
	}
	baseline := load("")
	defer baseline.Dungeons.CloseMapSource()
	compare := func(r projections) {
		t.Helper()
		defer r.Dungeons.CloseMapSource()
		r.Equipment.Source = baseline.Equipment.Source
		r.Loot.Source = baseline.Loot.Source
		r.Dungeons.Source = baseline.Dungeons.Source
		if !reflect.DeepEqual(r.Equipment, baseline.Equipment) || !reflect.DeepEqual(r.Loot, baseline.Loot) || !reflect.DeepEqual(r.Dungeons.Dungeons, baseline.Dungeons.Dungeons) || !reflect.DeepEqual(r.Dungeons.Maps, baseline.Dungeons.Maps) || !reflect.DeepEqual(r.Dungeons.Skipped, baseline.Dungeons.Skipped) || !reflect.DeepEqual(r.Season, baseline.Season) || !reflect.DeepEqual(r.Roster, baseline.Roster) || !reflect.DeepEqual(r.Warps, baseline.Warps) || !reflect.DeepEqual(r.Terminal, baseline.Terminal) {
			t.Fatal("changed native projections/private lookup indexes")
		}
		for _, b := range r.Roster.Backgrounds {
			if !r.Roster.ValidRosterBackground(b) {
				t.Fatal("background index not restored", b)
			}
		}
		for id := range r.Dungeons.Maps {
			a, e := baseline.Dungeons.MapScript(id)
			if e != nil {
				t.Fatal(e)
			}
			b, e := r.Dungeons.MapScript(id)
			if e != nil || !reflect.DeepEqual(a, b) {
				t.Fatal("changed native map", id, e)
			}
		}
	}
	compare(load(dir))
	before := s.DerivedCacheStats()
	compare(load(dir))
	after := s.DerivedCacheStats()
	if after.Hits-before.Hits != 7 {
		t.Fatal("seven projections did not hit", before, after)
	}
	files, e := filepath.Glob(filepath.Join(dir, "projection-*.pvfc"))
	if e != nil || len(files) != 7 {
		t.Fatal(files, e)
	}
	for _, file := range files {
		b, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		b[len(b)-1] ^= 1
		if e = os.WriteFile(file, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	before = s.DerivedCacheStats()
	compare(load(dir))
	after = s.DerivedCacheStats()
	if after.Invalid-before.Invalid != 7 || after.Writes-before.Writes != 7 {
		t.Fatal("corruption not rebuilt", before, after)
	}
	blocked := filepath.Join(t.TempDir(), "foreign")
	os.WriteFile(blocked, []byte("preserve"), 0600)
	compare(load(blocked))
	b, _ := os.ReadFile(blocked)
	if string(b) != "preserve" {
		t.Fatal("foreign file changed")
	}
	t.Logf("seven full projection/private-index comparisons; all %d native maps; corruption/write-failure fallbacks; equipment bindings=%d", len(baseline.Dungeons.Maps), len(baseline.Equipment.Bindings))
}
