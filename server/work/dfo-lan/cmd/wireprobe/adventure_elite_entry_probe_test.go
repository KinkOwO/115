package main

import (
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"github.com/stretchr/testify/require"
	"testing"
)

func probeWorld() *worldSession {
	w := &worldSession{role: database.Character{ID: 2, WireID: 2}, channelType: 22, eliteChannelDirectory: elitePreparationDirectory(22), selectingDungeon: true, dungeons: &catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{3: {ID: 3}}}}
	w.adventureElitePrepared = &adventureElitePreparation{Owner: 2, Channel: 22, Selected: [3]int64{1, 3, 0}}
	return w
}
func TestAdventureEliteEntryProbeScopeAndFrozenIdentity(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	request := protocol.DungeonSelection{ID: 3, Party: 65535}
	require.NoError(t, probeWorld().validateEliteEntryProbe(request))
	for _, name := range []string{"disabled", "owner", "channel", "settings", "empty", "pending-town", "active", "not-selecting", "odyssey-role", "mode", "party", "unknown-dungeon", "raid", "tutorial", "tower", "hell", "odyssey-source"} {
		t.Run(name, func(t *testing.T) {
			w := probeWorld()
			r := request
			switch name {
			case "disabled":
				t.Setenv(adventureelite.EnvKey, "0")
			case "owner":
				w.adventureElitePrepared.Owner = 4
			case "channel":
				w.adventureElitePrepared.Channel = 121
			case "settings":
				w.adventureElitePrepared.Settings[0] = 1
			case "empty":
				w.adventureElitePrepared.Selected = [3]int64{}
			case "pending-town":
				w.pendingTownArrival = &dungeon.Session{}
			case "active":
				w.activeDungeon = &dungeon.Session{}
			case "not-selecting":
				w.selectingDungeon = false
			case "odyssey-role":
				w.odyssey = true
				t.Setenv("DFO_ODYSSEY_MODE", "0")
				d := w.dungeons.Dungeons[3]
				d.Odyssey = true
				w.dungeons.Dungeons[3] = d
			case "mode":
				r.Mode = 1
			case "party":
				r.Party = 1
			case "unknown-dungeon":
				r.ID = 7
			case "raid":
				w.eliteChannelDirectory.ByType[22] = catalog.ChannelAttributes{Type: 22, IsRaid: true}
			default:
				d := w.dungeons.Dungeons[3]
				if name == "tutorial" {
					d.Tutorial = true
				}
				if name == "tower" {
					d.Tower = &catalog.TowerRuntime{}
				}
				if name == "hell" {
					d.HellParty = &catalog.DungeonHellParty{}
				}
				if name == "odyssey-source" {
					d.Odyssey = true
				}
				w.dungeons.Dungeons[3] = d
			}
			frozen := *w.adventureElitePrepared
			require.Error(t, w.validateEliteEntryProbe(r))
			require.Equal(t, frozen, *w.adventureElitePrepared)
		})
	}
}
func TestAdventureEliteOwnedCombatKeepsScopeAndReusesOrdinaryHandlers(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w := probeWorld()
	w.adventureEliteEntryProbeUsed = true
	w.activeDungeon = &dungeon.Session{Loaded: true}
	for _, id := range []uint16{2015, 2062} {
		require.Error(t, w.eliteEntryProbeRequest(id))
	}
	for _, id := range []uint16{37, 38, 39, 40, 42, 43, 45, 46, 69, 70, 71, 72, 117} {
		require.NoError(t, w.eliteEntryProbeRequest(id))
	}
	w.activeDungeon = nil
	require.NoError(t, w.eliteEntryProbeRequest(39))
	w.adventureElitePrepared = nil
	require.NoError(t, w.validateEliteEntryProbe(protocol.DungeonSelection{}))
}

func TestAdventureEliteOwnedCombatRejectsStaleOrSpecialRun(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	for _, name := range []string{"missing", "owner", "channel", "settings", "empty", "wire-zero", "wire-sentinel", "unloaded", "odyssey-role", "raid", "tutorial", "tower", "hell", "source-odyssey"} {
		t.Run(name, func(t *testing.T) {
			w := probeWorld()
			w.adventureEliteEntryProbeUsed = true
			w.activeDungeon = &dungeon.Session{Loaded: true}
			switch name {
			case "missing":
				w.adventureElitePrepared = nil
			case "owner":
				w.adventureElitePrepared.Owner++
			case "channel":
				w.adventureElitePrepared.Channel++
			case "settings":
				w.adventureElitePrepared.Settings[0]++
			case "empty":
				w.adventureElitePrepared.Selected = [3]int64{}
			case "wire-zero":
				w.role.WireID = 0
			case "wire-sentinel":
				w.role.WireID = 65535
			case "unloaded":
				w.activeDungeon.Loaded = false
			case "odyssey-role":
				w.odyssey = true
				t.Setenv("DFO_ODYSSEY_MODE", "0")
				w.activeDungeon.Definition.Odyssey = true
			case "raid":
				w.eliteChannelDirectory.ByType[22] = catalog.ChannelAttributes{Type: 22, IsRaid: true}
			case "tutorial":
				w.activeDungeon.Definition.Tutorial = true
			case "tower":
				w.activeDungeon.Definition.Tower = &catalog.TowerRuntime{}
			case "hell":
				w.activeDungeon.Definition.HellParty = &catalog.DungeonHellParty{}
			case "source-odyssey":
				w.activeDungeon.Definition.Odyssey = true
			}
			require.Error(t, w.eliteEntryProbeRequest(39))
			require.Error(t, w.eliteEntryProbeRequest(45))
			require.NoError(t, w.eliteEntryProbeRequest(42))
		})
	}
}

func TestAdventureEliteReentryAfterTownRequiresSamePreparation(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w := probeWorld()
	frozen := *w.adventureElitePrepared
	w.adventureEliteEntryProbeUsed = true
	w.adventureEliteEntrySerial = 1
	request := protocol.DungeonSelection{ID: 3, Party: 65535}
	// Previous run has returned to town, then the player opens ordinary selection.
	require.NoError(t, w.validateEliteEntryProbe(request))
	require.Equal(t, frozen, *w.adventureElitePrepared)
	require.Equal(t, uint64(1), w.adventureEliteEntrySerial) // Admission does not mutate state.
	w.activeDungeon = &dungeon.Session{Loaded: true}
	require.Error(t, w.validateEliteEntryProbe(request))
	w.activeDungeon = nil
	w.pendingTownArrival = &dungeon.Session{}
	require.Error(t, w.validateEliteEntryProbe(request))
	w.pendingTownArrival = nil
	w.adventureEliteSnapshot[0]++
	require.Error(t, w.validateEliteEntryProbe(request))
}

func TestAdventureEliteRepeatedPreparationCreatesIndependentSourceSessions(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, apc, _ := eliteEligibilityWorld(t, 1, 0, false)
	w.channelType = 22
	w.eliteChannelDirectory = elitePreparationDirectory(22)
	w.characters = nil
	w.fatigue = nil
	w.level = 115
	w.selectingDungeon = true
	w.adventureElitePrepared = &adventureElitePreparation{Owner: w.role.ID, Channel: 22, Selected: [3]int64{apc.ID}}
	// Small source-shaped input exercises the existing reader/domain/entry plan.
	w.dungeons = &catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{3: {ID: 3, MinimumLevel: 1, BasisLevel: 1, Mazes: []catalog.DungeonMaze{{Index: 0, Rooms: []catalog.DungeonRoom{{Map: 1}}}}}}, Maps: map[uint32]catalog.ScriptRecord{1: {}}}
	request := protocol.DungeonSelection{ID: 3, Party: 65535, Difficulty: 1}
	first, plan, err := w.prepareDungeonEntry(request)
	require.NoError(t, err)
	require.NotEmpty(t, first.RunID)
	require.Equal(t, uint64(1), w.adventureEliteEntrySerial)
	require.Equal(t, uint16(16), plan[0].ID)
	first.Dead[4096] = true
	first.Unowned = map[uint16]bool{4096: true}
	second, _, err := w.prepareDungeonEntry(request)
	require.NoError(t, err)
	require.NotEqual(t, first.RunID, second.RunID)
	require.Equal(t, uint64(2), w.adventureEliteEntrySerial)
	require.Empty(t, second.Dead)
	require.Empty(t, second.Unowned)
	request.ID = 7
	_, _, err = w.prepareDungeonEntry(request)
	require.Error(t, err)
	require.Equal(t, uint64(2), w.adventureEliteEntrySerial)
}
