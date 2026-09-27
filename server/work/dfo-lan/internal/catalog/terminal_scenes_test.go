package catalog

import "testing"

func TestCurrentTerminalSceneExportMatchesDungeonSource(t *testing.T) {
	c, err := LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := AttachTerminalScenes(&c, "../../configs/dungeons.terminal-scenes.json"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, scene := range c.TerminalScenes {
		if scene.Dungeon == 291100432 && scene.Quest == 12147 && scene.FinalMap == 100000294 &&
			scene.XMin == 741 && scene.YMin == 346 {
			found = true
		}
	}
	if !found {
		t.Fatal("confirmed closing scene missing from current source export")
	}
	c.Source.Checksum = "different-pvf"
	if err := AttachTerminalScenes(&c, "../../configs/dungeons.terminal-scenes.json"); err == nil {
		t.Fatal("accepted stale terminal scenes after a PVF change")
	}
}
