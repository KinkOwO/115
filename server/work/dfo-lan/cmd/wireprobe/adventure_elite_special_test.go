package main

import (
	"context"
	"testing"

	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"

	"github.com/stretchr/testify/require"
)

func TestAdventureEliteAllSourceSpecialCategoriesLoadOriginalRoster(t *testing.T) {
	for name, attrs := range map[string]catalog.ChannelAttributes{
		"raid": {IsRaid: true}, "legion": {IsLegion: true}, "pre-raid": {IsPreRaid: true},
		"semi-raid": {IsSemiRaid: true}, "guide": {GuideDungeon: 7}, "panel": {Panel: "fixture-panel"},
		"isolated": {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(adventureelite.EnvKey, "1")
			w, apc, slot := eliteEligibilityWorld(t, 1, 0, false)
			w.channelType = 121
			attrs.Type = w.channelType
			w.eliteChannelDirectory = &catalog.ChannelDirectory{ByType: map[uint32]catalog.ChannelAttributes{w.channelType: attrs}}
			w.channelWorldIsolated = name == "isolated"
			r := eliteEligibilityRequest(t, slot, 19)
			_, err := w.setAdventureElite(context.Background(), r, r, "all-special-source")
			require.NoError(t, err)
			plan, err := w.loadAdventureElite(context.Background(), []byte{2, 0})
			require.NoError(t, err)
			require.Equal(t, []uint16{1382, 1879}, []uint16{plan[0].ID, plan[1].ID})
			require.Equal(t, [3]int64{apc.ID, 0, 0}, w.adventureElitePrepared.Selected)
			prepared := w.adventureElitePrepared
			profile, err := w.prepareAdventure(context.Background())
			require.NoError(t, err)
			w.specialWarpPending = true
			refresh, err := w.refreshAdventureEliteSelections(context.Background(), profile)
			require.NoError(t, err)
			require.Empty(t, refresh, "special town warp cannot remove/reinsert the same native roster")
			require.Same(t, prepared, w.adventureElitePrepared)
			require.False(t, w.ordinaryElitePreparationAllowed())
			w.specialWarpPending = false
			w.activeDungeon = &dungeon.Session{}
			require.False(t, w.ordinaryElitePreparationAllowed())
			_, err = w.loadAdventureElite(context.Background(), []byte{2, 0})
			require.Error(t, err)
			require.Equal(t, [3]int64{apc.ID, 0, 0}, profile.Data.EliteSelections[2])
		})
	}
}

func TestAdventureEliteSpecialEntryDelegatesToExistingContentRules(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w := probeWorld()
	w.eliteChannelDirectory.ByType[w.channelType] = catalog.ChannelAttributes{Type: w.channelType, IsRaid: true}
	// Native special entries own their mode/party/selection grammar. No new
	// normalization or ordinary-only mode restriction is imposed here.
	r := protocol.DungeonSelection{ID: 3, Mode: 255, Party: 1}
	require.NoError(t, w.validateEliteEntryProbe(r))
	_, err := dungeon.Select(*w.dungeons, r, w.level, nil)
	require.Error(t, err, "original domain must still reject unsupported entry grammar")
	w.adventureEliteEntryProbeUsed = true
	w.activeDungeon = &dungeon.Session{}
	require.NoError(t, w.eliteEntryProbeRequest(2062), "original special transition validates its own state")
	w.adventureElitePrepared.Owner++
	require.Error(t, w.eliteEntryProbeRequest(2062), "special scope cannot bypass companion identity")
}

func TestAdventureEliteSpecialPacketObservationDoesNotCertifyEntry(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w := probeWorld()
	w.eliteChannelDirectory.ByType[w.channelType] = catalog.ChannelAttributes{Type: w.channelType, IsLegion: true}
	prepared := w.adventureElitePrepared
	serial := w.adventureEliteEntrySerial
	row := w.eliteSpecialPacketObservation("native-special-entry", 0, 28)
	require.NotNil(t, row)
	require.Equal(t, false, row["state_at_send"].(map[string]any)["active"])
	require.NotContains(t, row, "accepted")
	require.NotContains(t, row, "native_registration_success")
	require.Same(t, prepared, w.adventureElitePrepared)
	require.Equal(t, serial, w.adventureEliteEntrySerial)
	require.False(t, w.adventureEliteEntryProbeUsed)
	require.Nil(t, w.eliteSpecialPacketObservation("ack", 1, 28))
	require.Nil(t, w.eliteSpecialPacketObservation("loaded", 0, 30))
	t.Setenv(adventureelite.EnvKey, "0")
	require.Nil(t, w.eliteSpecialPacketObservation("native-special-entry", 0, 28))
}
