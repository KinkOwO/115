package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// Live 051649: restoring a roster after the town warp's empty view can race
// with CMD15. Its native reload is rejected in selection state, leaving no APC.
// Keep the account view stable across either polling/selection ordering while
// preserving the separate unsettled-scene loading and admission gates.
func TestAdventureEliteTownWarpKeepsRosterAcrossPollingAndSelectionOrder(t *testing.T) {
	for count := 1; count <= 3; count++ {
		for _, selectionFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("companions-%d-selection-first-%t", count, selectionFirst), func(t *testing.T) {
				t.Setenv(adventureelite.EnvKey, "1")
				ctx := context.Background()
				w, slots, ids := eliteReloadWorld(t)
				selected := ids
				for i := count; i < 3; i++ {
					slots[i], selected[i] = -1, 0
				}
				request := eliteReloadRequest(t, slots, 19)
				_, err := w.setAdventureElite(ctx, request, request, "stable-warp")
				require.NoError(t, err)
				_, err = w.loadAdventureElite(ctx, []byte{2, 0})
				require.NoError(t, err)
				profile, err := w.prepareAdventure(ctx)
				require.NoError(t, err)
				prepared := w.adventureElitePrepared
				frozen := *prepared
				w.adventureEliteEntryProbeUsed, w.adventureEliteEntrySerial = true, 7
				for cycle := 0; cycle < 2; cycle++ {
					w.specialWarpPending = true
					require.False(t, w.ordinaryElitePreparationAllowed())
					require.False(t, w.ordinaryEliteSelectionVisible(), "warp still blocks battle admission")
					packets, err := w.refreshAdventureEliteSelections(ctx, profile)
					require.NoError(t, err)
					require.Empty(t, packets, "town warp must not remove mode2 and release native actors")
					_, err = w.loadAdventureElite(ctx, []byte{2, 0})
					require.ErrorContains(t, err, "当前频道未启用")
					w.selectingDungeon = true
					require.Error(t, w.validateEliteEntryProbe(protocol.DungeonSelection{ID: 3, Party: 65535}))
					w.selectingDungeon = false
					require.Same(t, prepared, w.adventureElitePrepared)
					require.Equal(t, frozen, *prepared)
					w.specialWarpPending = false
					w.selectingDungeon = selectionFirst
					for poll := 0; poll < 3; poll++ {
						packets, err = w.refreshAdventureEliteSelections(ctx, profile)
						require.NoError(t, err)
						require.Empty(t, packets, "polling before or after CMD15 must not trigger a reload")
					}
					w.selectingDungeon = true
					require.NoError(t, w.validateEliteEntryProbe(protocol.DungeonSelection{ID: 3, Party: 65535}))
					_, err = w.loadAdventureElite(ctx, []byte{2, 0})
					require.Error(t, err, "selection must not authorize rebuilding existing companions")
					require.Same(t, prepared, w.adventureElitePrepared)
					require.Equal(t, frozen, *prepared)
					require.Equal(t, selected, prepared.Selected)
					w.selectingDungeon = false
					_, err = w.loadAdventureElite(ctx, []byte{2, 0})
					require.ErrorContains(t, err, "重复加载")
					require.True(t, w.adventureEliteEntryProbeUsed)
					require.Equal(t, uint64(7), w.adventureEliteEntrySerial)
					saved, err := w.prepareAdventure(ctx)
					require.NoError(t, err)
					require.Equal(t, profile.Data, saved.Data, "no persisted roster or skill changes")
				}
			})
		}
	}
}

func TestAdventureEliteProjectionRefreshKeepsIdenticalNativeActors(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, slots, _ := eliteReloadWorld(t)
	ctx := context.Background()
	request := eliteReloadRequest(t, slots, 19)
	_, err := w.setAdventureElite(ctx, request, request, "refresh-same-roster")
	require.NoError(t, err)
	_, err = w.loadAdventureElite(ctx, []byte{2, 0})
	require.NoError(t, err)
	prepared := w.adventureElitePrepared
	profile, err := w.prepareAdventure(ctx)
	require.NoError(t, err)
	// A second mode changes the full N1754 hash without changing mode-2 actors.
	profile.Data.EliteSelections[3] = [3]int64{}
	packets, err := w.refreshAdventureEliteSelections(ctx, profile)
	require.NoError(t, err)
	require.Len(t, packets, 1)
	require.Same(t, prepared, w.adventureElitePrepared)
	require.Equal(t, sha256.Sum256(packets[0].Payload), prepared.Settings)
	_, err = w.loadAdventureElite(ctx, []byte{2, 0})
	require.ErrorContains(t, err, "重复加载")
}

func TestAdventureEliteProjectionRefreshRetainsSpecialScopeBoundary(t *testing.T) {
	for _, scope := range []string{"legion", "raid", "guide", "isolated", "tutorial"} {
		t.Run(scope, func(t *testing.T) {
			t.Setenv(adventureelite.EnvKey, "1")
			ctx := context.Background()
			w, slots, _ := eliteReloadWorld(t)
			request := eliteReloadRequest(t, slots, 19)
			_, err := w.setAdventureElite(ctx, request, request, "scope-projection")
			require.NoError(t, err)
			_, err = w.loadAdventureElite(ctx, []byte{2, 0})
			require.NoError(t, err)
			profile, err := w.prepareAdventure(ctx)
			require.NoError(t, err)
			switch scope {
			case "legion":
				w.eliteChannelDirectory.ByType[22] = catalog.ChannelAttributes{Type: 22, IsLegion: true}
			case "raid":
				w.eliteChannelDirectory.ByType[22] = catalog.ChannelAttributes{Type: 22, IsRaid: true}
			case "guide":
				w.eliteChannelDirectory.ByType[22] = catalog.ChannelAttributes{Type: 22, GuideDungeon: 7}
			case "isolated":
				w.channelWorldIsolated = true
			case "tutorial":
				w.inTutorial = true
			}
			hidden, err := w.refreshAdventureEliteSelections(ctx, profile)
			require.NoError(t, err)
			require.Len(t, hidden, 1)
			require.Equal(t, []byte{0}, hidden[0].Payload)
			require.Nil(t, w.adventureElitePrepared)
			_, err = w.loadAdventureElite(ctx, []byte{2, 0})
			require.ErrorContains(t, err, "当前频道未启用")
		})
	}
}

func TestAdventureEliteProjectionSyncDoesNotAlterDisabledOrNativePreparation(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprint(native), func(t *testing.T) {
			t.Setenv(adventureelite.EnvKey, "0")
			w := probeWorld()
			if native {
				t.Setenv(adventureelite.EnvKey, "1")
				w.channelType = 73
			}
			frozen := *w.adventureElitePrepared
			w.syncAdventureElitePreparation(database.AccountAdventure{})
			require.Equal(t, frozen, *w.adventureElitePrepared)
		})
	}
}
