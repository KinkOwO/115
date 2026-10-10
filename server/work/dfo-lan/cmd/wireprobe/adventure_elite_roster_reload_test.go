package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventureelite"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func eliteReloadWorld(t *testing.T) (*worldSession, [3]int32, [3]int64) {
	t.Helper()
	w, apc, _ := eliteEligibilityWorld(t, 1, 0, false)
	ctx := context.Background()
	ids := [3]int64{apc.ID}
	for i := 1; i < 3; i++ {
		name := fmt.Sprintf("EliteReload%d", i)
		request := binary.LittleEndian.AppendUint32([]byte{0}, uint32(len(name)))
		request = append(request, []byte(name)...)
		options := make([]byte, 12)
		options[6] = 255
		request = append(request, options...)
		role, err := w.store.CreateCharacter(ctx, database.Character{AccountID: w.account, Name: name,
			Profession: apc.Profession, ConfigVersion: apc.ConfigVersion, Request: request, State: apc.State}, 24)
		require.NoError(t, err)
		ids[i] = role.ID
	}
	roles, err := w.store.Characters(ctx, w.account)
	require.NoError(t, err)
	var slots [3]int32
	for index, role := range roles {
		for i, id := range ids {
			if role.ID == id {
				slots[i] = int32(index)
			}
		}
	}
	w.channelType, w.eliteChannelDirectory = 22, elitePreparationDirectory(22)
	w.dungeons = &catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{3: {ID: 3}}}
	return w, slots, ids
}

func eliteReloadRequest(t *testing.T, slots [3]int32, skill int32) []byte {
	t.Helper()
	row := protocol.AdventureEliteSelection{Mode: 2, Slots: slots}
	for i, slot := range slots {
		if slot >= 0 {
			row.SkillUsage[i][0] = skill
		}
	}
	encoded, err := protocol.AdventureEliteSelections([]protocol.AdventureEliteSelection{row})
	require.NoError(t, err)
	return encoded[1:]
}

// Live 034552: three -> one and two -> one saves succeeded, but retained the
// old frozen identity. N1754 released actors and requested1811; duplicate-load
// rejection then left CMD16 blocked on a changed settings hash.
func TestAdventureEliteRosterSaveReloadsChangedSelection(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	for _, tc := range []struct {
		name     string
		from, to [3]int
	}{
		{"three-to-one", [3]int{0, 1, 2}, [3]int{0, -1, -1}},
		{"two-to-one-live-middle-slot", [3]int{0, 1, -1}, [3]int{-1, 1, -1}},
		{"one-to-two", [3]int{0, -1, -1}, [3]int{0, 1, -1}},
		{"two-to-three", [3]int{0, 1, -1}, [3]int{0, 1, 2}},
		{"reordered-three", [3]int{0, 1, 2}, [3]int{2, 1, 0}},
		{"clear", [3]int{0, 1, 2}, [3]int{-1, -1, -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, slots, ids := eliteReloadWorld(t)
			ctx := context.Background()
			project := func(index [3]int) ([3]int32, [3]int64) {
				s := [3]int32{-1, -1, -1}
				var selected [3]int64
				for i, id := range index {
					if id >= 0 {
						s[i], selected[i] = slots[id], ids[id]
					}
				}
				return s, selected
			}
			first, _ := project(tc.from)
			request := eliteReloadRequest(t, first, 19)
			_, err := w.setAdventureElite(ctx, request, request, "roster-first")
			require.NoError(t, err)
			_, err = w.loadAdventureElite(ctx, []byte{2, 0})
			require.NoError(t, err)
			to, selected := project(tc.to)
			request = eliteReloadRequest(t, to, 19)
			saved, err := w.setAdventureElite(ctx, request, request, "roster-change")
			require.NoError(t, err)
			require.Nil(t, w.adventureElitePrepared, "N1754 changed slots must permit its native1811 reload")
			require.Equal(t, []uint16{1754, 1719}, []uint16{saved[0].ID, saved[1].ID})
			profile, err := w.prepareAdventure(ctx)
			require.NoError(t, err)
			require.Equal(t, selected, profile.Data.EliteSelections[2])
			loaded, err := w.loadAdventureElite(ctx, []byte{2, 0})
			if selected == ([3]int64{}) {
				require.ErrorContains(t, err, "至少一个")
				require.Nil(t, loaded)
				return
			}
			require.NoError(t, err)
			require.Len(t, loaded, 2, "reload must not emit N1754 and create another automatic request")
			require.Equal(t, []uint16{1382, 1879}, []uint16{loaded[0].ID, loaded[1].ID})
			require.Equal(t, selected, w.adventureElitePrepared.Selected)
			require.Equal(t, sha256.Sum256(saved[0].Payload), w.adventureElitePrepared.Settings)
			w.selectingDungeon = true
			require.NoError(t, w.validateEliteEntryProbe(protocol.DungeonSelection{ID: 3, Party: 65535}))
		})
	}
}

// Native142E5ABFC retains actors on identical slots and applies skill usage
// via142E653A0, without emitting1811. The settings hash must follow that view.
func TestAdventureEliteSameRosterSaveRetainsPreparedActors(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	for _, skill := range []int32{19, 0} {
		t.Run(fmt.Sprintf("skill-%d", skill), func(t *testing.T) {
			w, slots, _ := eliteReloadWorld(t)
			ctx := context.Background()
			request := eliteReloadRequest(t, slots, 19)
			_, err := w.setAdventureElite(ctx, request, request, "same-first")
			require.NoError(t, err)
			_, err = w.loadAdventureElite(ctx, []byte{2, 0})
			require.NoError(t, err)
			prepared := w.adventureElitePrepared
			request = eliteReloadRequest(t, slots, skill)
			saved, err := w.setAdventureElite(ctx, request, request, "same-resave")
			require.NoError(t, err)
			require.Same(t, prepared, w.adventureElitePrepared)
			require.Equal(t, sha256.Sum256(saved[0].Payload), prepared.Settings)
			w.selectingDungeon = true
			require.NoError(t, w.validateEliteEntryProbe(protocol.DungeonSelection{ID: 3, Party: 65535}))
		})
	}
}

func TestAdventureEliteRosterSaveRefusalPreservesAccountAndPreparation(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	for _, phase := range []string{"active", "selecting", "town-arrival", "mine", "invalid-skill"} {
		t.Run(phase, func(t *testing.T) {
			w, slots, ids := eliteReloadWorld(t)
			ctx := context.Background()
			request := eliteReloadRequest(t, slots, 19)
			_, err := w.setAdventureElite(ctx, request, request, "refuse-first")
			require.NoError(t, err)
			_, err = w.loadAdventureElite(ctx, []byte{2, 0})
			require.NoError(t, err)
			frozen := *w.adventureElitePrepared
			request = eliteReloadRequest(t, [3]int32{slots[0], -1, -1}, 19)
			switch phase {
			case "active":
				w.activeDungeon = &dungeon.Session{}
			case "selecting":
				w.selectingDungeon = true
			case "town-arrival":
				w.pendingTownArrival = &dungeon.Session{}
			case "mine":
				w.specialWarpPending = true
			case "invalid-skill":
				request = eliteReloadRequest(t, slots, 65534)
			}
			packets, err := w.setAdventureElite(ctx, request, request, "refuse-save")
			require.Error(t, err)
			require.Nil(t, packets)
			require.Equal(t, frozen, *w.adventureElitePrepared)
			profile, err := w.prepareAdventure(ctx)
			require.NoError(t, err)
			require.Equal(t, ids, profile.Data.EliteSelections[2])
		})
	}
}
