package main

import (
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"encoding/binary"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

// Native CMD2062 from the failed APC Odyssey run, events.jsonl:619.
const eliteQuickNextNativeBody = "5400000000000000ffffffffff47f4f505020000000000000001000000a0000000c8000000140000000a000000000000"

func eliteQuickNextWorld(t *testing.T) (*worldSession, []byte) {
	t.Helper()
	t.Setenv(adventureelite.EnvKey, "1")
	t.Setenv("DFO_ODYSSEY_MODE", "")
	w, odyssey, _ := eliteEligibilityWorld(t, 1, 0, true)
	human := w.role
	w.role = odyssey
	w.odyssey = character.OdysseyRole(odyssey)
	require.True(t, w.odyssey)
	w.channelType = 22
	w.eliteChannelDirectory = elitePreparationDirectory(22)
	w.characters = nil
	w.fatigue = nil
	w.level = 15
	c := eliteOdysseyFixture(t, 5, 2)
	d := c.Dungeons[3]
	d.ID = 4
	d.Mazes[0].Quest = 0
	c.Dungeons[4] = d
	w.dungeons = &c
	w.activeDungeon = &dungeon.Session{RunID: "completed-previous", Loaded: true, Definition: d, Dead: map[uint16]bool{}, Unowned: map[uint16]bool{}}
	w.activeDungeon.MarkCompleted()
	w.completionSent = true
	w.resultSent = true
	w.adventureElitePrepared = &adventureElitePreparation{Owner: odyssey.ID, Channel: 22, Selected: [3]int64{human.ID}}
	w.adventureEliteEntryProbeUsed = true
	w.adventureEliteEntrySerial = 1
	raw, err := hex.DecodeString(eliteQuickNextNativeBody)
	require.NoError(t, err)
	// Fixture target uses the existing reader's native ID slot, not a runtime list.
	binary.LittleEndian.PutUint32(raw[13:17], 4)
	return w, raw
}

func TestEliteOdysseyQuickNextBuildsIndependentRunAndKeepsFrozenRoster(t *testing.T) {
	w, body := eliteQuickNextWorld(t)
	old := w.activeDungeon
	old.Dead[4096] = true
	old.Unowned[4096] = true
	frozen := *w.adventureElitePrepared
	require.NoError(t, w.eliteEntryProbeRequest(2062))
	before := w.eliteCombatState(2062, body)
	next, plan, err := w.directMoveDungeon(body)
	require.NoError(t, err)
	require.NotEqual(t, old.RunID, next.RunID)
	require.False(t, next.Loaded)
	require.False(t, next.Completed())
	require.Empty(t, next.Dead)
	require.Empty(t, next.Unowned)
	require.Same(t, old, w.activeDungeon, "transport dispatch commits the pending session later")
	require.Equal(t, frozen, *w.adventureElitePrepared)
	require.Equal(t, uint64(2), w.adventureEliteEntrySerial)
	ids := []uint16{}
	for _, p := range plan {
		ids = append(ids, p.ID)
		require.NotEqual(t, uint16(2062), p.ID)
	}
	require.GreaterOrEqual(t, len(ids), 5)
	require.Equal(t, []uint16{15, 27, 16, 28, 29}, ids[:5]) // Existing native buff notifications may follow START_MAP.
	var rows []map[string]any
	w.noteEliteCombatRequest(2062, body, before, next, plan, nil, func(row map[string]any) { rows = append(rows, row) })
	require.Len(t, rows, 2)
	require.Equal(t, uint32(4), rows[0]["pending_dungeon"])
	require.Equal(t, old.RunID, rows[1]["origin_run_id"])
	require.Equal(t, uint16(2062), rows[1]["entry_opcode"])
	require.Equal(t, next.RunID, rows[1]["run_id"])
	require.Equal(t, uint64(2), rows[1]["entry_serial"])
}

func TestEliteOdysseyQuickNextRejectsUnfinishedStaleAndSpecialRoutes(t *testing.T) {
	for _, name := range []string{"uncompleted", "no-result", "no-clear-sent", "unloaded", "unused", "owner", "channel", "settings", "empty", "wire-zero", "pending-town", "selecting", "ordinary-source", "ordinary-role", "raid", "target-ordinary", "target-tutorial", "target-tower", "target-hell", "target-unknown", "level", "difficulty", "malformed"} {
		t.Run(name, func(t *testing.T) {
			w, body := eliteQuickNextWorld(t)
			d := w.dungeons.Dungeons[4]
			switch name {
			case "uncompleted":
				w.activeDungeon = &dungeon.Session{RunID: "unfinished", Loaded: true, Definition: d}
			case "no-result":
				w.resultSent = false
			case "no-clear-sent":
				w.completionSent = false
			case "unloaded":
				w.activeDungeon.Loaded = false
			case "unused":
				w.adventureEliteEntryProbeUsed = false
			case "owner":
				w.adventureElitePrepared.Owner++
			case "channel":
				w.adventureElitePrepared.Channel++
			case "settings":
				w.adventureEliteSnapshot[0]++
			case "empty":
				w.adventureElitePrepared.Selected = [3]int64{}
			case "wire-zero":
				w.role.WireID = 0
			case "pending-town":
				w.pendingTownArrival = &dungeon.Session{}
			case "selecting":
				w.selectingDungeon = true
			case "ordinary-source":
				w.activeDungeon.Definition.Odyssey = false
			case "ordinary-role":
				w.odyssey = false
			case "raid":
				w.eliteChannelDirectory.ByType[22] = catalog.ChannelAttributes{Type: 22, IsRaid: true}
			case "target-ordinary":
				d.Odyssey = false
			case "target-tutorial":
				d.Tutorial = true
			case "target-tower":
				d.Tower = &catalog.TowerRuntime{}
			case "target-hell":
				d.HellParty = &catalog.DungeonHellParty{}
			case "target-unknown":
				binary.LittleEndian.PutUint32(body[13:17], 7)
			case "level":
				w.level = 4
			case "difficulty":
				body[17] = 1
			case "malformed":
				body = body[:47]
			}
			w.dungeons.Dungeons[4] = d
			old := w.activeDungeon
			frozen := *w.adventureElitePrepared
			next, plan, err := w.directMoveDungeon(body)
			require.Error(t, err)
			require.Nil(t, next)
			require.Empty(t, plan)
			require.Same(t, old, w.activeDungeon)
			require.Equal(t, uint64(1), w.adventureEliteEntrySerial)
			require.Equal(t, frozen, *w.adventureElitePrepared)
			if w.adventureEliteEntryProbeUsed {
				require.Error(t, w.eliteEntryProbeRequest(2015))
			}
		})
	}
}

func TestEliteQuickNextPreservesOriginalRouteWithoutPreparedAPCs(t *testing.T) {
	w, body := eliteQuickNextWorld(t)
	w.adventureElitePrepared = nil
	w.resultSent = false // Original handler retains its existing scope.
	next, _, err := w.directMoveDungeon(body)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, uint64(1), w.adventureEliteEntrySerial)
}

func TestEliteOdysseyQuickNextCurrentPVFNativeTarget(t *testing.T) {
	archive := os.Getenv("DFO_ELITE_ODYSSEY_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set read-only current PVF archive to verify native2062 source")
	}
	w, _ := eliteQuickNextWorld(t)
	raw, err := hex.DecodeString(eliteQuickNextNativeBody)
	require.NoError(t, err)
	request, err := protocol.DecodeDungeonDirectMove(raw)
	require.NoError(t, err)
	require.Equal(t, uint32(100004935), request.ID)
	require.Equal(t, byte(2), request.Difficulty)
	require.Equal(t, [4]uint32{160, 200, 20, 10}, request.Gate)
	src, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: archive, ExpectedChecksum: os.Getenv("DFO_ELITE_ODYSSEY_TEST_SHA256")})
	require.NoError(t, err)
	defer src.Close()
	oldSource := catalog.OdysseySource
	t.Cleanup(func() { catalog.OdysseySource = oldSource })
	catalog.SetOdysseySource(src.Snapshot().Checksum)
	c, err := src.Dungeons([]uint32{100004934, request.ID})
	require.NoError(t, err)
	routes, err := src.OdysseyJournalRoutes()
	require.NoError(t, err)
	found := false
	for _, node := range routes.Nodes {
		for _, id := range node.Dungeons {
			if id == request.ID {
				found = true
			}
		}
	}
	require.True(t, found)
	w.dungeons = &c
	w.activeDungeon.Definition = c.Dungeons[100004934]
	next, plan, err := w.directMoveDungeon(raw)
	require.NoError(t, err)
	require.True(t, next.Definition.Odyssey)
	require.Equal(t, c.Dungeons[request.ID].Script.SHA256, next.Definition.Script.SHA256)
	require.Equal(t, c.Dungeons[request.ID].DesignatedDifficulty, next.Difficulty)
	require.NotEmpty(t, plan)
	t.Logf("native target=%d script=%s sha=%s minimum=%d difficulty=%d map=%d journal=%s; offline admission only", request.ID, next.Definition.Script.Path, next.Definition.Script.SHA256, next.Definition.MinimumLevel, next.Difficulty, next.Room.Map, routes.Path)
}
