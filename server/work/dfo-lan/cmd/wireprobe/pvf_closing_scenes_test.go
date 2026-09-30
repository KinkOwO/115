package main

import (
	"dfolan/internal/catalog"
	"os"
	"testing"
)

func TestPVFClosingScenesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for terminal and tournament scene parity")
	}
	c, err := preparePVFCoreCatalogs("dungeons,dungeon-terminal,dungeon-tournament", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", scenePolicyPath: "../../configs/pvf-scene-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.terminalScenes.Scenes) != 7 || len(c.tournamentMaps.Maps) != 2 {
		t.Fatalf("source scene scope changed: terminal=%d tournament=%d", len(c.terminalScenes.Scenes), len(c.tournamentMaps.Maps))
	}
	base := clonePVFDungeons(*c.dungeons)
	if err := c.attachTerminalScenes(&base, "missing-terminal.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.attachTournamentMaps(&base, "missing-tournament.json"); err != nil {
		t.Fatal(err)
	}
	base.TerminalScenes[0].XMin++
	if base.TerminalScenes[0].XMin == c.terminalScenes.Scenes[0].XMin {
		t.Fatal("runtime scene wrapper mutated prepared source")
	}
	for _, id := range []uint32{100003298, 100003299} {
		if len(base.Dungeons[id].Mazes[0].Rooms) != 1 || len(c.dungeons.Dungeons[id].Mazes[0].Rooms) != 0 {
			t.Fatal("tournament arena failed to bind or mutated source maze")
		}
	}
	bad := *c.terminalScenes
	bad.Source.Checksum = "wrong-source"
	if catalog.ApplyTerminalScenes(&base, bad) == nil {
		t.Fatal("cross-source terminal scene accepted")
	}
	wrong := *c.tournamentMaps
	wrong.SourceChecksum = "wrong-source"
	if catalog.ApplyTournamentQuestMaps(&base, wrong) == nil {
		t.Fatal("cross-source arena accepted")
	}
	t.Log("complete terminal and tournament parity; absent export paths and immutable prepared mazes verified")
}
