package gamedata

import (
	"dfolan/internal/catalog"
	"os"
	"reflect"
	"testing"
)

func TestPVFClosingScenesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for terminal and tournament scene parity")
	}
	c, err := prepareCatalogsForTest(t, "dungeons,dungeon-terminal,dungeon-tournament", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json", ScenePolicyPath: "../../configs/pvf-scene-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.TerminalScenes.Scenes) != 7 || len(c.TournamentMaps.Maps) != 2 {
		t.Fatalf("source scene scope changed: terminal=%d tournament=%d", len(c.TerminalScenes.Scenes), len(c.TournamentMaps.Maps))
	}
	base := clonePVFDungeons(*c.Dungeons)
	if err := c.AttachTerminalScenes(&base, "missing-terminal.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.AttachTournamentMaps(&base, "missing-tournament.json"); err != nil {
		t.Fatal(err)
	}
	base.TerminalScenes[0].XMin++
	if base.TerminalScenes[0].XMin == c.TerminalScenes.Scenes[0].XMin {
		t.Fatal("runtime scene wrapper mutated prepared source")
	}
	for _, id := range []uint32{100003298, 100003299} {
		if len(base.Dungeons[id].Mazes[0].Rooms) != 1 || len(c.Dungeons.Dungeons[id].Mazes[0].Rooms) != 0 {
			t.Fatal("tournament arena failed to bind or mutated source maze")
		}
	}
	bad := *c.TerminalScenes
	bad.Source.Checksum = "wrong-source"
	if catalog.ApplyTerminalScenes(&base, bad) == nil {
		t.Fatal("cross-source terminal scene accepted")
	}
	wrong := *c.TournamentMaps
	wrong.SourceChecksum = "wrong-source"
	if catalog.ApplyTournamentQuestMaps(&base, wrong) == nil {
		t.Fatal("cross-source arena accepted")
	}
	t.Log("complete terminal and tournament parity; absent export paths and immutable prepared mazes verified")
}

func TestPVFLayerRevisitsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native layer revisit source binding")
	}
	c, err := prepareCatalogsForTest(t, "dungeons,layer-revisits", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json", DungeonPath: "../../configs/dungeons.full.json", ScenePolicyPath: "../../configs/pvf-scene-policy.json", LayerRevisitPolicyPath: "../../configs/pvf-layer-revisit-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	base := clonePVFDungeons(*c.Dungeons)
	if err = c.AttachLayerRevisits(&base, "missing-layer-revisits.json"); err != nil {
		t.Fatal(err)
	}
	if len(base.LayerRevisits) != 1 || base.LayerRevisits[0].Map != 100004546 || base.LayerRevisits[0].ResumeMap != 100004325 || len(c.Dungeons.LayerRevisits) != 0 {
		t.Fatal("layer source scope or ownership changed")
	}
	base.LayerRevisits[0].Map = 0
	if c.LayerRevisits.Scenes[0].Map != 100004546 {
		t.Fatal("runtime overlay shares source storage")
	}
	t.Log("quest 12893 final cinematic and same-grid base match full native source hashes and witnessed record")
}

func TestPVFScenesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete scene parity")
	}
	c, err := prepareCatalogsForTest(t, "town,dungeons,training-dungeons,tutorial-dungeons,dungeon-towers,dungeon-hell,dungeon-maze", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json", ScenePolicyPath: "../../configs/pvf-scene-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if !c.Town.Allows(1, 561, 234) || len(c.TrainingDungeons.Dungeons) != 4 {
		t.Fatal("entry/training compatibility changed")
	}
	if _, err := c.LoadTown("missing.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.LoadDungeons("missing.json"); err != nil {
		t.Fatal(err)
	}
	base, err := c.LoadDungeons("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AttachTowerGrief(&base, "missing.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.AttachTowerDazzlement(&base, "missing.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.AttachHellMaps(&base, "missing.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.AttachMazeRates(&base, "missing.json"); err != nil {
		t.Fatal(err)
	}
	for _, layer := range c.Grief.Layers {
		if base.Dungeons[layer.Dungeon].TowerGriefFloor == 0 || c.Dungeons.Dungeons[layer.Dungeon].TowerGriefFloor != 0 {
			t.Fatal("tower attachment mutated prepared base")
		}
	}
	t.Log(len(c.Dungeons.Dungeons), len(c.Dungeons.Maps), len(c.TutorialDungeons.Dungeons))
}

func TestPVFOldDungeonBasisDiagnosticsAreNarrow(t *testing.T) {
	c := catalog.DungeonCatalog{Skipped: []string{"dungeon 5000: invalid [basis level]", "dungeon 5000: map missing", "dungeon 99: invalid [basis level]"}}
	got := normalizeOldDungeonBasisDiagnostics(c, pvfScenePolicy{Training: []uint32{5000}})
	if !reflect.DeepEqual(got.Skipped, c.Skipped[1:]) || len(c.Skipped) != 3 {
		t.Fatal("unexpected diagnostics suppressed or audit mutated its input")
	}
}

func TestPVFScriptWarpsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for full native script warp bindings")
	}
	c, err := prepareCatalogsForTest(t, "dungeons,script-warps", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../../configs/items.index.json", DungeonPath: "../../configs/dungeons.full.json", ScenePolicyPath: "../../configs/pvf-scene-policy.json", ScriptWarpPolicyPath: "../../configs/pvf-script-warp-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ScriptWarps) != 12 {
		t.Fatal("script warp scope changed")
	}
	restore, err := c.InstallScriptWarps()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	t.Log("all 11 cinematic and 1 forced monster routes match full source hashes, action ownership, grid and landing fields")
}
