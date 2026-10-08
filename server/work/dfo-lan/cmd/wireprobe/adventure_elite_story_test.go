package main

import (
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

// A small source-reader fixture, not a runtime dungeon or quest policy.
func eliteStoryFixture(t *testing.T, minimum int32) catalog.DungeonCatalog {
	t.Helper()
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	num := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	str := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	d, err := catalog.ParseDungeon(3, catalog.ScriptRecord{Path: "elite-story-fixture.dgn", Cells: []pvf.Token{
		tag("[minimum required level]"), num(minimum), tag("[maze info]"),
		tag("[quest connection]"), num(0), num(17), num(-1), tag("[size]"), num(1), num(1),
		tag("[map specification]"), str("boss"), num(0), num(0), num(100), tag("[/map specification]"),
		tag("[start map]"), num(0), num(0), tag("[/start map]"), tag("[boss map]"), num(0), num(0), tag("[/boss map]"),
	}})
	require.NoError(t, err)
	return catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{3: d}, Maps: map[uint32]catalog.ScriptRecord{100: {Path: "fixture.map", Cells: []pvf.Token{tag("[monster]")}}}}
}

func TestEliteStoryAdmissionRetainsSourceAndQuestRules(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	c := eliteStoryFixture(t, 5)
	w := probeWorld()
	w.dungeons = &c
	r := protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 17}
	storyRequest := r
	frozen := *w.adventureElitePrepared
	require.NoError(t, w.validateEliteEntryProbe(r))
	_, err := dungeon.Select(c, r, 5, nil)
	require.ErrorContains(t, err, "quest is not accepted")
	s, err := dungeon.Select(c, r, 5, map[uint16]bool{17: true})
	require.NoError(t, err)
	require.Equal(t, uint16(17), s.Maze.Quest)
	c = eliteStoryFixture(t, 6)
	_, err = dungeon.Select(c, r, 5, map[uint16]bool{17: true})
	require.ErrorContains(t, err, "minimum level")
	r.Quest = 18
	_, err = dungeon.Select(c, r, 6, map[uint16]bool{18: true})
	require.ErrorContains(t, err, "no resolved source maze")
	require.Equal(t, frozen, *w.adventureElitePrepared)
	require.Zero(t, w.adventureEliteEntrySerial)
	require.Nil(t, w.activeDungeon)
	row := eliteEntryProbeDiagnostic(w, s, nil, storyRequest)
	require.Equal(t, uint16(17), row["quest"])
	require.Equal(t, "ordinary-story", row["candidate_stage"])
	require.Equal(t, uint32(17), row["requested_quest"])
}

func TestEliteStoryNative3146SourceAdmission(t *testing.T) {
	archive := os.Getenv("DFO_ELITE_STORY_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set DFO_ELITE_STORY_TEST_ARCHIVE for read-only current PVF proof")
	}
	t.Setenv(adventureelite.EnvKey, "1")
	src, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: archive, ExpectedChecksum: os.Getenv("DFO_ELITE_STORY_TEST_SHA256")})
	require.NoError(t, err)
	defer src.Close()
	body, err := hex.DecodeString("050000000000000000ffff00000000004a0c0000000000000000000000000000")
	require.NoError(t, err)
	r, err := protocol.DecodeDungeonSelection(body)
	require.NoError(t, err)
	require.Equal(t, uint32(3146), r.Quest)
	c, err := src.Dungeons([]uint32{r.ID})
	require.NoError(t, err)
	w := probeWorld()
	w.dungeons = &c
	require.NoError(t, w.validateEliteEntryProbe(r))
	d := c.Dungeons[r.ID]
	require.Equal(t, "dungeon/act1/sunderland.dgn", d.Script.Path)
	require.Equal(t, "f2ad25fd1a365ac38b57e8d1a7ab122ca3625d3e2faeea862828deb77e93181b", d.Script.SHA256)
	_, err = dungeon.Select(c, r, byte(d.MinimumLevel), nil)
	require.ErrorContains(t, err, "quest is not accepted")
	s, err := dungeon.Select(c, r, byte(d.MinimumLevel), map[uint16]bool{3146: true})
	require.NoError(t, err)
	require.Equal(t, uint16(3146), s.Maze.Quest)
	require.Equal(t, uint32(76131), s.Room.Map)
	require.Equal(t, [2]byte{3, 1}, s.Maze.Boss)
	if d.MinimumLevel > 0 {
		_, err = dungeon.Select(c, r, byte(d.MinimumLevel-1), map[uint16]bool{3146: true})
		require.ErrorContains(t, err, "minimum level")
	}
	t.Logf("source=%s sha256=%s maze=%d quest=%d start=%d boss=%v; native admission candidate only", d.Script.Path, d.Script.SHA256, s.Maze.Index, s.Maze.Quest, s.Room.Map, s.Maze.Boss)
}
