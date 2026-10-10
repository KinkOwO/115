package main

import (
	"context"
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func bindEliteMoonFixture(w *worldSession, channel uint32) {
	w.channelType = channel
	w.moonConfig = &moonSoloConfig{Channel: channel, SourceRewards: loot.MoonSourcePolicy{Source: "fixture-pvf"}}
	w.eliteChannelDirectory = &catalog.ChannelDirectory{ByType: map[uint32]catalog.ChannelAttributes{
		channel: {Type: channel, IsSemiRaid: true, GuideDungeon: 100004137, SlotDungeon: 100004137, Panel: "conquestDungeonPanel"},
	}}
	w.channelWorldIsolated = true
}

func TestAdventureEliteMoonExecutorBindingRemainsDistinctFromChannelPermission(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	cases := map[string]func(*worldSession){
		"unbound":         func(w *worldSession) { w.moonConfig = nil },
		"foreign-channel": func(w *worldSession) { w.moonConfig.Channel++ },
		"legacy-json":     func(w *worldSession) { w.moonConfig.SourceRewards.Source = "" },
		"raid": func(w *worldSession) {
			a := w.eliteChannelDirectory.ByType[w.channelType]
			a.IsRaid = true
			w.eliteChannelDirectory.ByType[w.channelType] = a
		},
		"legion": func(w *worldSession) {
			a := w.eliteChannelDirectory.ByType[w.channelType]
			a.IsLegion = true
			w.eliteChannelDirectory.ByType[w.channelType] = a
		},
		"different-slot": func(w *worldSession) {
			a := w.eliteChannelDirectory.ByType[w.channelType]
			a.SlotDungeon++
			w.eliteChannelDirectory.ByType[w.channelType] = a
		},
		"unknown-source": func(w *worldSession) { delete(w.eliteChannelDirectory.ByType, w.channelType) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			w := &worldSession{}
			bindEliteMoonFixture(w, 101)
			mutate(w)
			require.False(t, w.eliteMoonChannel())
			require.Equal(t, name != "unknown-source", w.ordinaryEliteRosterVisible(), "channel permission does not fabricate a bound Moon executor")
		})
	}
}

func TestAdventureEliteMoonIsolatedTownLoadsOriginalCompanions(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, apc, slot := eliteEligibilityWorld(t, 1, 0, false)
	bindEliteMoonFixture(w, 101)
	r := eliteEligibilityRequest(t, slot, 19)
	_, err := w.setAdventureElite(context.Background(), r, r, "moon-town")
	require.NoError(t, err)
	require.True(t, w.ordinaryEliteRosterVisible())
	packets, err := w.loadAdventureElite(context.Background(), []byte{2, 0})
	require.NoError(t, err)
	require.Equal(t, uint16(1382), packets[0].ID)
	require.Equal(t, uint16(1879), packets[1].ID)
	require.Equal(t, [3]int64{apc.ID, 0, 0}, w.adventureElitePrepared.Selected)
	profile, err := w.prepareAdventure(context.Background())
	require.NoError(t, err)
	before := profile.Data.EliteSelections[2]
	prepared := w.adventureElitePrepared
	w.specialWarpPending = true
	require.True(t, w.ordinaryEliteRosterVisible())
	require.False(t, w.ordinaryElitePreparationAllowed())
	w.syncAdventureElitePreparation(profile)
	require.Same(t, prepared, w.adventureElitePrepared)
	require.Equal(t, before, profile.Data.EliteSelections[2])
	w.specialWarpPending = false
	require.ErrorContains(t, w.validateEliteEntryProbe(protocol.DungeonSelection{}), "沉月湖")
	w.moon.owner = &dungeon.MoonSoloOwner{}
	require.False(t, w.ordinaryElitePreparationAllowed(), "retreated ongoing run must not rebuild companions")
	t.Setenv(adventureelite.EnvKey, "0")
	require.False(t, w.ordinaryEliteRosterVisible())
}

func TestAdventureEliteMoonEntryObservationRejectsMissingIdentity(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w := &worldSession{role: database.Character{ID: 7, WireID: 7}}
	bindEliteMoonFixture(w, 101)
	require.Nil(t, w.eliteMoonEntryObservation())
	w.adventureElitePrepared = &adventureElitePreparation{Owner: 7, Channel: 101, Selected: [3]int64{8, 0, 0}}
	row := w.eliteMoonEntryObservation()
	require.Equal(t, false, row["accepted"])
	require.Equal(t, uint16(0), row["entry_opcode"])
	require.False(t, w.adventureEliteEntryProbeUsed)
	require.Zero(t, w.adventureEliteEntrySerial)
}

func TestAdventureEliteMoonCurrentPVFChannelBinding(t *testing.T) {
	path := os.Getenv("DFO_ELITE_MOON_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_ELITE_MOON_TEST_ARCHIVE for current source binding")
	}
	t.Setenv(adventureelite.EnvKey, "1")
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_ELITE_MOON_TEST_SHA256"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close() })
	dir, err := catalog.ImportChannelDirectory(a)
	require.NoError(t, err)
	attrs, ok := dir.Attributes(101)
	require.True(t, ok)
	w := &worldSession{channelType: attrs.Type, channelWorldIsolated: true, eliteChannelDirectory: &dir,
		moonConfig: &moonSoloConfig{Channel: attrs.Type, SourceRewards: loot.MoonSourcePolicy{Source: a.Snapshot().Checksum}}}
	require.True(t, w.eliteMoonChannel())
	require.True(t, w.ordinaryElitePreparationAllowed())
	t.Logf("source=%s client=%s slot=%s attributes=%+v", a.Snapshot().Checksum, dir.ClientSHA256, dir.SlotSHA256, attrs)
	// Exercise the real solo executor, keeping its session, roster and preparation.
	c, err := catalog.ImportDungeons(a, []uint32{100004136, attrs.GuideDungeon})
	require.NoError(t, err)
	owner, err := dungeon.NewMoonSolo(c, 115, 7, 0, time.Now())
	require.NoError(t, err)
	w.role = database.Character{ID: 7, WireID: 7}
	w.moon.owner = owner
	w.activeDungeon = owner.Session()
	w.adventureElitePrepared = &adventureElitePreparation{Owner: 7, Channel: attrs.Type, Selected: [3]int64{8, 0, 0}}
	prepared, session := w.adventureElitePrepared, w.activeDungeon
	row := w.eliteMoonEntryObservation()
	require.Equal(t, true, row["accepted"])
	require.True(t, w.adventureEliteEntryProbeUsed)
	require.EqualValues(t, 1, w.adventureEliteEntrySerial)
	w.moon.resuming = true
	row = w.eliteMoonEntryObservation()
	require.Equal(t, true, row["accepted"])
	require.Equal(t, true, row["moon_resuming"])
	require.EqualValues(t, 2, w.adventureEliteEntrySerial)
	require.Same(t, prepared, w.adventureElitePrepared)
	require.Same(t, session, owner.Session())
	require.Same(t, session, w.activeDungeon)
	require.False(t, w.ordinaryElitePreparationAllowed())
	t.Setenv(adventureelite.EnvKey, "0")
	require.Equal(t, false, w.eliteMoonEntryObservation()["accepted"])
	require.EqualValues(t, 2, w.adventureEliteEntrySerial)
}
