package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventureelite"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEliteSharedRosterProjectsCurrentRoleWithoutChangingSave(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	t.Setenv("DFO_ODYSSEY_MODE", "") // Live mode is the character's native creation flag.
	ctx := context.Background()
	w, odyssey, otherSlot := eliteEligibilityWorld(t, 1, 0, true)
	human := w.role
	saved := [3]int64{odyssey.ID, human.ID, 0}
	_, _, _, err := w.store.CommitAdventure(ctx, w.account, human.ID, "shared-self-fixture", func(role database.Character, p *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		p.Data.EliteSelections = map[uint16][3]int64{2: saved}
		p.Data.EliteSkillUsage = map[uint16]map[int64][30]int32{2: {human.ID: {19}, odyssey.ID: {19}}}
		return role.State, json.RawMessage(`{}`), nil
	})
	require.NoError(t, err)
	w.role = odyssey
	w.odyssey = character.OdysseyRole(odyssey)
	require.True(t, w.odyssey)
	w.channelType = 22
	w.eliteChannelDirectory = elitePreparationDirectory(22)
	profile, err := w.prepareAdventure(ctx)
	require.NoError(t, err)
	view := w.eliteProfileView(profile)
	require.Equal(t, [3]int64{0, human.ID, 0}, view.Data.EliteSelections[2])
	require.Equal(t, saved, profile.Data.EliteSelections[2])
	roles, err := w.store.Characters(ctx, w.account)
	require.NoError(t, err)
	var humanSlot byte
	for i, r := range roles {
		if r.ID == human.ID {
			humanSlot = byte(i)
			human = r
		}
	}
	expectedRow := protocol.AdventureEliteSelection{Mode: 2, Slots: [3]int32{-1, int32(humanSlot), -1}}
	expectedRow.SkillUsage[1][0] = 19
	wantSettings, err := protocol.AdventureEliteSelections([]protocol.AdventureEliteSelection{expectedRow})
	require.NoError(t, err)
	actualSettings, err := w.adventureElitePayload(ctx, profile)
	require.NoError(t, err)
	require.Equal(t, wantSettings, actualSettings)
	w.adventureEliteSnapshot = sha256.Sum256(actualSettings)
	packets, err := w.loadAdventureElite(ctx, []byte{2, 0})
	require.NoError(t, err)
	require.Len(t, packets, 2, "stable projected1754 must not trigger a second native request")
	snapshot, err := w.characters.TagCharacterSnapshot(human)
	require.NoError(t, err)
	wantCharacters, err := protocol.AdventureEliteCharacterInfo(odyssey.WireID, []byte{humanSlot}, []protocol.TagCharacter{snapshot})
	require.NoError(t, err)
	require.Equal(t, uint16(1382), packets[0].ID)
	require.Equal(t, wantCharacters, packets[0].Payload)
	require.Equal(t, uint16(1879), packets[1].ID)
	require.Equal(t, []byte{2, 0}, packets[1].Payload)
	require.Equal(t, [3]int64{0, human.ID, 0}, w.adventureElitePrepared.Selected)
	require.Equal(t, odyssey.ID, w.adventureElitePrepared.Owner)
	require.Equal(t, sha256.Sum256(wantSettings), w.adventureElitePrepared.Settings)
	after, err := w.prepareAdventure(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, after.Data.EliteSelections[2])
	require.Equal(t, profile.Data.EliteSkillUsage, after.Data.EliteSkillUsage)
	// Switching back changes only the recipient's transient view, not the save.
	w.role = human
	w.odyssey = false
	w.adventureElitePrepared = nil
	back := w.eliteProfileView(after)
	require.Equal(t, [3]int64{odyssey.ID, 0, 0}, back.Data.EliteSelections[2])
	require.Equal(t, saved, after.Data.EliteSelections[2])
	require.Equal(t, int32(1), otherSlot)
}

func TestEliteSelfProjectionRetainsScopeModesSlotsAndEmptyList(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w := probeWorld()
	p := database.AccountAdventure{}
	p.Data.EliteSelections = map[uint16][3]int64{2: {w.role.ID, 9, 11}, 3: {w.role.ID, 9, 11}}
	for _, slot := range []int{0, 1, 2} {
		saved := [3]int64{9, 11, 13}
		saved[slot] = w.role.ID
		p.Data.EliteSelections[2] = saved
		view := w.eliteProfileView(p)
		expected := saved
		expected[slot] = 0
		require.Equal(t, expected, view.Data.EliteSelections[2])
		require.Equal(t, saved, p.Data.EliteSelections[2])
		require.Equal(t, p.Data.EliteSelections[3], view.Data.EliteSelections[3])
	}
	w.channelType = 73
	require.Equal(t, p.Data.EliteSelections, w.eliteProfileView(p).Data.EliteSelections)
	w.channelType = 22
	t.Setenv(adventureelite.EnvKey, "0")
	require.Equal(t, p.Data.EliteSelections, w.eliteProfileView(p).Data.EliteSelections)
	t.Setenv(adventureelite.EnvKey, "1")
	w.channelWorldIsolated = true
	require.NotContains(t, w.eliteProfileView(p).Data.EliteSelections, uint16(2))
	require.Equal(t, p.Data.EliteSelections[3], w.eliteProfileView(p).Data.EliteSelections[3])
	w.channelWorldIsolated = false
	p.Data.EliteSelections[2] = [3]int64{w.role.ID}
	require.Equal(t, [3]int64{}, w.eliteProfileView(p).Data.EliteSelections[2])
}
