package main

import (
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

// Source-shaped fixtures verify that the candidate preserves source difficulty
// and player gates. They are not a runtime dungeon list or reward policy.
func eliteOdysseyFixture(t *testing.T, minimum int32, difficulty int32) catalog.DungeonCatalog {
	t.Helper()
	c := eliteStoryFixture(t, minimum)
	d := c.Dungeons[3]
	cells := append([]pvf.Token{{Type: 3, Text: "[dungeon mode script]"}, {Type: 6, Text: "arad odyssey"}, {Type: 3, Text: "[designate dungeon difficulty]"}, {Type: 0, Value: difficulty}}, d.Script.Cells...)
	d, err := catalog.ParseDungeon(3, catalog.ScriptRecord{Path: "elite-odyssey-fixture.dgn", Cells: cells})
	require.NoError(t, err)
	c.Dungeons[3] = d
	return c
}

func TestEliteOdysseyAdmissionPreservesModeSourceAndOwner(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	t.Setenv("DFO_ODYSSEY_MODE", "1")
	w := probeWorld()
	w.odyssey = true
	c := eliteOdysseyFixture(t, 5, 2)
	w.dungeons = &c
	r := protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 17, Difficulty: 2}
	frozen := *w.adventureElitePrepared
	require.NoError(t, w.validateEliteEntryProbe(r))
	s, err := dungeon.Select(c, r, 5, map[uint16]bool{17: true})
	require.NoError(t, err)
	w.adventureEliteEntryProbeUsed = true
	s.Loaded = true
	w.activeDungeon = s
	for _, opcode := range []uint16{39, 43, 45, 46, 69, 70, 71, 72, 117} {
		require.NoError(t, w.eliteEntryProbeRequest(opcode))
	}
	require.Error(t, w.eliteEntryProbeRequest(2062))
	row := eliteEntryProbeDiagnostic(w, s, nil, r)
	require.Equal(t, "ordinary-odyssey", row["candidate_stage"])
	require.Equal(t, true, row["source_odyssey"])
	require.Equal(t, byte(2), row["source_designated_difficulty"])
	state := w.eliteCombatState(45, nil)
	require.Equal(t, true, state["source_odyssey"])
	require.Equal(t, frozen, *w.adventureElitePrepared)
	require.Zero(t, w.adventureEliteEntrySerial)
	w.activeDungeon = nil
	r.Difficulty = 1
	_, err = dungeon.Select(c, r, 5, map[uint16]bool{17: true})
	require.ErrorContains(t, err, "difficulty")
	c = eliteOdysseyFixture(t, 5, 1)
	_, err = dungeon.Select(c, r, 5, map[uint16]bool{17: true})
	require.NoError(t, err)
	c = eliteOdysseyFixture(t, 6, 1)
	_, err = dungeon.Select(c, r, 5, map[uint16]bool{17: true})
	require.ErrorContains(t, err, "minimum level")
	_, err = dungeon.Select(c, r, 6, nil)
	require.ErrorContains(t, err, "quest is not accepted")
	w.odyssey = false
	require.Error(t, w.validateEliteEntryProbe(r))
	w.odyssey = true
	t.Setenv("DFO_ODYSSEY_MODE", "0")
	require.Error(t, w.validateEliteEntryProbe(r))
	w.activeDungeon = s
	require.Error(t, w.eliteEntryProbeRequest(39))
}

func TestEliteOdysseyCharacterAllowsSpecialSourceAndRetainsOrdinaryModeBoundary(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w := probeWorld()
	w.odyssey = true
	r := protocol.DungeonSelection{ID: 3, Party: 65535}
	require.NoError(t, w.validateEliteEntryProbe(r))
	w.eliteChannelDirectory.ByType[22] = catalog.ChannelAttributes{Type: 22, IsLegion: true}
	require.NoError(t, w.validateEliteEntryProbe(r))
	w = probeWorld()
	w.odyssey = true
	r.Mode = 1
	require.Error(t, w.validateEliteEntryProbe(r))
	w = probeWorld()
	w.odyssey = true
	w.channelWorldIsolated = true
	r.Mode = 0
	require.NoError(t, w.validateEliteEntryProbe(r))
}

func TestEliteOdysseyCurrentPVFJournalSourceAdmission(t *testing.T) {
	archive := os.Getenv("DFO_ELITE_ODYSSEY_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set DFO_ELITE_ODYSSEY_TEST_ARCHIVE for current read-only PVF proof")
	}
	t.Setenv(adventureelite.EnvKey, "1")
	t.Setenv("DFO_ODYSSEY_MODE", "1")
	src, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: archive, ExpectedChecksum: os.Getenv("DFO_ELITE_ODYSSEY_TEST_SHA256")})
	require.NoError(t, err)
	defer src.Close()
	oldSource := catalog.OdysseySource
	t.Cleanup(func() { catalog.OdysseySource = oldSource })
	catalog.SetOdysseySource(src.Snapshot().Checksum)
	routes, err := src.OdysseyJournalRoutes()
	require.NoError(t, err)
	var ids []uint32
	for _, node := range routes.Nodes {
		ids = append(ids, node.Dungeons...)
	}
	c, err := src.Dungeons(ids)
	require.NoError(t, err)
	w := probeWorld()
	w.odyssey = true
	w.dungeons = &c
	for _, id := range ids {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			d := c.Dungeons[id]
			require.True(t, d.Odyssey)
			r := protocol.DungeonSelection{ID: id, Party: 65535, Difficulty: d.DesignatedDifficulty}
			require.NoError(t, w.validateEliteEntryProbe(r))
			s, err := dungeon.Select(c, r, byte(d.MinimumLevel), nil)
			require.NoError(t, err)
			require.Equal(t, d.Script.SHA256, s.Definition.Script.SHA256)
			require.Equal(t, d.DesignatedDifficulty, s.Difficulty)
			t.Logf("id=%d source=%s sha=%s minimum=%d difficulty=%d map=%d hunt=%d conditions=%v", id, d.Script.Path, d.Script.SHA256, d.MinimumLevel, d.DesignatedDifficulty, s.Room.Map, d.HuntBoss, d.BossEntranceConditionIDs)
		})
	}
	t.Logf("journal=%s sha=%s routes=%d; candidate policy and original Select only, no native client acceptance", routes.Path, routes.SHA256, len(ids))
}
