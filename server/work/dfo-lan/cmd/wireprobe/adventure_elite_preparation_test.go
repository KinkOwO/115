package main

import (
	"context"
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func elitePreparationDirectory(channel uint32) *catalog.ChannelDirectory {
	return &catalog.ChannelDirectory{ClientPath: catalog.ClientChannelInfoPath,
		ByType: map[uint32]catalog.ChannelAttributes{channel: {Type: channel}}}
}

func TestAdventureEliteOrdinaryTownPreparationFreezesIdentityAndBlocksBattle(t *testing.T) {
	for _, channel := range []uint32{2, 3, 22, 121} {
		for _, odyssey := range []bool{false, true} {
			t.Run(fmt.Sprintf("channel-%d-apc-odyssey-%t", channel, odyssey), func(t *testing.T) {
				t.Setenv(adventureelite.EnvKey, "1")
				w, apc, slot := eliteEligibilityWorld(t, 1, 0, odyssey)
				w.channelType, w.eliteChannelDirectory = channel, elitePreparationDirectory(channel)
				request := eliteEligibilityRequest(t, slot, 19)
				_, err := w.setAdventureElite(context.Background(), request, request, "ordinary-town")
				require.NoError(t, err)
				packets, err := w.loadAdventureElite(context.Background(), []byte{2, 0})
				require.NoError(t, err)
				require.Len(t, packets, 2, "unchanged settings must not trigger a repeated N1754 request")
				require.Equal(t, uint16(1382), packets[0].ID)
				require.Equal(t, uint16(1879), packets[1].ID)
				require.Equal(t, []byte{2, 0}, packets[1].Payload)
				snapshot, err := w.characters.TagCharacterSnapshot(apc)
				require.NoError(t, err)
				want, err := protocol.AdventureEliteCharacterInfo(w.role.WireID, []byte{byte(slot)}, []protocol.TagCharacter{snapshot})
				require.NoError(t, err)
				require.Equal(t, want, packets[0].Payload, "APC snapshot must retain its real equipment/skills/level")
				require.Equal(t, [3]int64{apc.ID, 0, 0}, w.adventureElitePrepared.Selected)
				require.Equal(t, channel, w.adventureElitePrepared.Channel)
				require.Equal(t, w.role.ID, w.adventureElitePrepared.Owner)
				_, err = w.loadAdventureElite(context.Background(), []byte{2, 0})
				require.ErrorContains(t, err, "重复加载")
				entry, output, err := w.prepareDungeonEntry(protocol.DungeonSelection{})
				require.ErrorContains(t, err, "尚未完成战斗接入")
				require.Nil(t, entry)
				require.Nil(t, output)
				require.Nil(t, w.activeDungeon)
				// Empty selection clears only this session's candidate admission hold.
				binary.LittleEndian.PutUint32(request[0x27:], ^uint32(0))
				_, err = w.setAdventureElite(context.Background(), request, request, "ordinary-empty")
				require.NoError(t, err)
				require.Nil(t, w.adventureElitePrepared)
			})
		}
	}
}

func TestAdventureElitePreparationFollowsSourceChannelAttributesWithoutMutatingSavedList(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, apc, slot := eliteEligibilityWorld(t, 1, 0, false)
	w.channelType, w.eliteChannelDirectory = 121, elitePreparationDirectory(121)
	request := eliteEligibilityRequest(t, slot, 19)
	_, err := w.setAdventureElite(context.Background(), request, request, "source-scope")
	require.NoError(t, err)
	profile, err := w.prepareAdventure(context.Background())
	require.NoError(t, err)
	require.Equal(t, [3]int64{apc.ID, 0, 0}, profile.Data.EliteSelections[2])
	for _, flag := range []string{"raid", "legion", "pre-raid", "semi-raid", "guide", "panel", "unknown", "isolated", "tutorial", "warp"} {
		t.Run(flag, func(t *testing.T) {
			w.eliteChannelDirectory = elitePreparationDirectory(121)
			w.inTutorial, w.selectingDungeon, w.specialWarpPending, w.channelWorldIsolated = false, false, false, false
			a := w.eliteChannelDirectory.ByType[121]
			switch flag {
			case "raid":
				a.IsRaid = true
			case "legion":
				a.IsLegion = true
			case "pre-raid":
				a.IsPreRaid = true
			case "semi-raid":
				a.IsSemiRaid = true
			case "guide":
				a.GuideDungeon = 7
			case "panel":
				a.Panel = "dedicated-native-party"
			case "isolated":
				w.channelWorldIsolated = true
			case "tutorial":
				w.inTutorial = true
			case "warp":
				w.specialWarpPending = true
			}
			w.eliteChannelDirectory.ByType[121] = a
			if flag == "unknown" {
				delete(w.eliteChannelDirectory.ByType, 121)
			}
			blocked := flag == "unknown" || flag == "tutorial" || flag == "warp"
			require.Equal(t, !blocked, w.ordinaryElitePreparationAllowed())
			payload, err := w.adventureElitePayload(context.Background(), profile)
			require.NoError(t, err)
			if !blocked || flag == "warp" {
				require.NotEqual(t, []byte{0}, payload, "ordinary town warp preserves the roster, while preparation stays blocked")
			} else {
				require.Equal(t, []byte{0}, payload, "unsupported source view must not authorize the DLL transaction")
			}
			_, err = w.loadAdventureElite(context.Background(), []byte{2, 0})
			if blocked {
				require.ErrorContains(t, err, "当前频道未启用")
				require.Nil(t, w.adventureElitePrepared)
			} else {
				require.NoError(t, err)
				require.Equal(t, [3]int64{apc.ID, 0, 0}, w.adventureElitePrepared.Selected)
				w.adventureElitePrepared = nil
			}
			require.Equal(t, [3]int64{apc.ID, 0, 0}, profile.Data.EliteSelections[2])
		})
	}
	t.Setenv(adventureelite.EnvKey, "0")
	view := w.eliteProfileView(profile)
	require.Equal(t, profile.Data.EliteSelections, view.Data.EliteSelections, "off must preserve native list serialization")
}

// Replay the live CMD15 -> poll -> CMD132 -> poll sequence. The native N1754
// reader destroys existing companions when a removed mode-2 row reappears.
func TestAdventureEliteSelectionTransitionsDoNotDestroyPreparedCompanions(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	ctx := context.Background()
	w, apc, slot := eliteEligibilityWorld(t, 1, 0, false)
	w.channelType, w.eliteChannelDirectory = 22, elitePreparationDirectory(22)
	request := eliteEligibilityRequest(t, slot, 19)
	_, err := w.setAdventureElite(ctx, request, request, "selection-lifecycle")
	require.NoError(t, err)
	profile, err := w.prepareAdventure(ctx)
	require.NoError(t, err)
	w.adventureEliteSnapshot = [32]byte{}
	initial, err := w.refreshAdventureEliteSelections(ctx, profile)
	require.NoError(t, err)
	require.Len(t, initial, 1)
	require.Equal(t, uint16(1754), initial[0].ID)
	loaded, err := w.loadAdventureElite(ctx, []byte{2, 0})
	require.NoError(t, err)
	require.Equal(t, []uint16{1382, 1879}, []uint16{loaded[0].ID, loaded[1].ID})
	frozen := *w.adventureElitePrepared
	for _, phase := range []string{"selecting", "town-arrival", "active-dungeon", "town"} {
		t.Run(phase, func(t *testing.T) {
			w.selectingDungeon = phase == "selecting"
			w.pendingTownArrival, w.activeDungeon = nil, nil
			if phase == "town-arrival" {
				w.pendingTownArrival = &dungeon.Session{}
			}
			if phase == "active-dungeon" {
				w.activeDungeon = &dungeon.Session{}
			}
			visible, err := w.adventureElitePayload(ctx, profile)
			require.NoError(t, err)
			require.Equal(t, initial[0].Payload, visible, "scene state must not remove or reinsert mode2")
			for poll := 0; poll < 3; poll++ {
				packets, err := w.refreshAdventureEliteSelections(ctx, profile)
				require.NoError(t, err)
				require.Empty(t, packets, "no destructive N1754 or repeated native load on scene transition")
			}
			_, err = w.loadAdventureElite(ctx, []byte{2, 0})
			require.Error(t, err, "visibility must not authorize a duplicate or mid-scene load")
			require.Equal(t, frozen, *w.adventureElitePrepared)
			saved, err := w.prepareAdventure(ctx)
			require.NoError(t, err)
			require.Equal(t, [3]int64{apc.ID, 0, 0}, saved.Data.EliteSelections[2])
		})
	}
	w.selectingDungeon, w.activeDungeon, w.pendingTownArrival = false, nil, nil
	w.eliteChannelDirectory.ByType[22] = catalog.ChannelAttributes{Type: 22, IsLegion: true}
	hidden, err := w.refreshAdventureEliteSelections(ctx, profile)
	require.NoError(t, err)
	require.Empty(t, hidden, "source category changes no longer remove the identical roster")
	require.Equal(t, frozen, *w.adventureElitePrepared)
}

func TestAdventureElitePreparationNativeChannelSources(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native channel sources")
	}
	t.Setenv(adventureelite.EnvKey, "1")
	archive, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	require.NoError(t, err)
	defer archive.Close()
	dir, err := catalog.ImportChannelDirectory(archive)
	require.NoError(t, err)
	info, err := catalog.ImportChannelInfo(archive)
	require.NoError(t, err)
	w, _, _ := eliteEligibilityWorld(t, 1, 0, false)
	w.eliteChannelDirectory, w.eliteChannelInfo = &dir, &info
	for server, rows := range info.Servers {
		w.serverID = server
		for _, row := range rows.Rows {
			w.eliteChannelID, w.channelType = row.ID, row.Type
			expected := true
			if adventureEliteChannel(row.Type) {
				expected = false
			}
			require.Equal(t, expected, w.ordinaryElitePreparationAllowed(), "server=%d row=%+v", server, row)
		}
	}
	w.serverID, w.eliteChannelID, w.channelType = 1, 10, 22
	require.True(t, w.ordinaryElitePreparationAllowed(), "source ordinary ID with existing local Type override")
	w.odyssey = true
	require.True(t, w.ordinaryElitePreparationAllowed(), "Odyssey character shares source ordinary channel route")
	for _, id := range []uint32{106, 119} {
		w.channelType = id
		require.True(t, w.ordinaryElitePreparationAllowed(), "source special type is authorized independently of ordinary ID")
	}
	w.channelType, w.eliteChannelID = 9999, 9999
	require.False(t, w.ordinaryElitePreparationAllowed(), "unknown route must fail closed")
	t.Logf("archive=%s ordinary=%s/%s special=%s; all source rows verified", archive.Snapshot().Checksum, info.Path, info.SHA256, dir.ClientPath)
}
func TestAdventureElitePreparationSourceOrdinaryRowChangesEligibility(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, _, _ := eliteEligibilityWorld(t, 1, 0, false)
	w.channelType, w.eliteChannelID, w.serverID = 22, 10, 1
	w.eliteChannelDirectory = &catalog.ChannelDirectory{ByType: map[uint32]catalog.ChannelAttributes{106: {Type: 106, IsSemiRaid: true}}}
	w.eliteChannelInfo = &catalog.ChannelInfo{Servers: map[uint32]catalog.ChannelInfoServer{1: {ServerID: 1, Rows: []catalog.ChannelInfoRow{{ID: 10, Type: 2}}}}}
	require.True(t, w.ordinaryElitePreparationAllowed())
	w.eliteChannelInfo.Servers[1] = catalog.ChannelInfoServer{ServerID: 1, Rows: []catalog.ChannelInfoRow{{ID: 10, Type: 106}}}
	require.True(t, w.ordinaryElitePreparationAllowed(), "published special route is also allowed")
	w.eliteChannelInfo.Servers[1] = catalog.ChannelInfoServer{ServerID: 1, Rows: []catalog.ChannelInfoRow{{ID: 11, Type: 2}}}
	require.False(t, w.ordinaryElitePreparationAllowed())
}
