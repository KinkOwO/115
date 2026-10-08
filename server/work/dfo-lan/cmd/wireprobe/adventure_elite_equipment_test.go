package main

import (
	"context"
	"dfolan/internal/adventureelite"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Regression for live 20261008_155318: one saved companion has primer rows
// 36..45. Do not drop those rows or alter its inventory to unblock loading.
func TestAdventureEliteExtendedEquipmentLoadsWithoutChangingSave(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, apc, slot := eliteEligibilityWorld(t, 115, 2, false)
	w.channelType, w.eliteChannelDirectory = 22, elitePreparationDirectory(22)
	bag, err := inventory.ReadBag(apc.State)
	require.NoError(t, err)
	r := protocol.OrdinaryItem(12, 108000872, 0)
	r[76] = 8
	bag.Worn = append(bag.Worn, inventory.BagEquipment{Slot: 12, Template: 108000872, Durability: 48, Record: r[:]})
	for s := uint16(36); s <= 45; s++ {
		bag.Worn = append(bag.Worn, inventory.BagEquipment{Slot: s, Template: 100401592})
	}
	raw, err := inventory.SaveBag(apc.State, bag)
	require.NoError(t, err)
	ctx := context.Background()
	apc, _, err = w.store.CommitCharacterEvent(ctx, w.account, apc.ID, apc.ConfigVersion, "fixture:extended-equipment", "test", func(database.Character) (json.RawMessage, json.RawMessage, error) {
		return raw, json.RawMessage(`{}`), nil
	})
	require.NoError(t, err)
	req := eliteEligibilityRequest(t, slot, 19)
	_, err = w.setAdventureElite(ctx, req, req, "equipped-squad")
	require.NoError(t, err)
	packets, err := w.loadAdventureElite(ctx, []byte{2, 0})
	require.NoError(t, err)
	require.Len(t, packets, 2)
	require.Equal(t, uint16(1382), packets[0].ID)
	require.Equal(t, uint16(1879), packets[1].ID)
	snapshot, err := w.characters.TagCharacterSnapshot(apc)
	require.NoError(t, err)
	require.Len(t, snapshot.Worn, 11)
	require.Equal(t, byte(8), snapshot.Worn[0].Record[76])
	for i := 1; i < len(snapshot.Worn); i++ {
		require.Equal(t, uint16(35+i), snapshot.Worn[i].Slot)
	}
	want, err := protocol.AdventureEliteCharacterInfo(w.role.WireID, []byte{byte(slot)}, []protocol.TagCharacter{snapshot})
	require.NoError(t, err)
	require.Equal(t, want, packets[0].Payload)
	roles, err := w.store.Characters(ctx, w.account)
	require.NoError(t, err)
	for _, role := range roles {
		if role.ID == apc.ID {
			require.JSONEq(t, string(apc.State), string(role.State))
		}
	}
	require.NotNil(t, w.adventureElitePrepared)
	_, _, err = w.prepareDungeonEntry(protocol.DungeonSelection{})
	require.ErrorContains(t, err, "尚未完成战斗接入")
}

// Live 20261009_025432 owner4 selects role3 with newly worn creature key1
// duplicated at record+6/+24. A new owner exposes the equipped companion;
// loading must not remove its pet, rewrite its save or clear account slots.
func TestAdventureEliteCreatureMirrorLoadsWithoutChangingSave(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, apc, slot := eliteEligibilityWorld(t, 115, 2, false)
	w.channelType, w.eliteChannelDirectory = 22, elitePreparationDirectory(22)
	bag, err := inventory.ReadBag(apc.State)
	require.NoError(t, err)
	record := protocol.OrdinaryItem(26, 500991107, 0)
	binary.LittleEndian.PutUint32(record[6:10], 1)
	binary.LittleEndian.PutUint32(record[24:28], 1)
	bag.Worn = append(bag.Worn, inventory.BagEquipment{Slot: 26, Template: 500991107, Record: record[:]})
	raw, err := inventory.SaveBag(apc.State, bag)
	require.NoError(t, err)
	ctx := context.Background()
	apc, _, err = w.store.CommitCharacterEvent(ctx, w.account, apc.ID, apc.ConfigVersion, "fixture:creature-mirror", "test", func(database.Character) (json.RawMessage, json.RawMessage, error) {
		return raw, json.RawMessage(`{}`), nil
	})
	require.NoError(t, err)
	req := eliteEligibilityRequest(t, slot, 19)
	_, err = w.setAdventureElite(ctx, req, req, "creature-squad")
	require.NoError(t, err)
	profileBefore, err := w.prepareAdventure(ctx)
	require.NoError(t, err)
	packets, err := w.loadAdventureElite(ctx, []byte{2, 0})
	require.NoError(t, err)
	require.Len(t, packets, 2)
	require.Equal(t, uint16(1382), packets[0].ID)
	require.Equal(t, uint16(1879), packets[1].ID)
	require.NotNil(t, w.adventureElitePrepared)
	snapshot, err := w.characters.TagCharacterSnapshot(apc)
	require.NoError(t, err)
	require.Len(t, snapshot.Worn, 1)
	require.Equal(t, record[:], snapshot.Worn[0].Record)
	roles, err := w.store.Characters(ctx, w.account)
	require.NoError(t, err)
	for _, role := range roles {
		if role.ID == apc.ID {
			require.JSONEq(t, string(apc.State), string(role.State))
		}
	}
	profileAfter, err := w.prepareAdventure(ctx)
	require.NoError(t, err)
	require.Equal(t, profileBefore.Data.EliteSelections, profileAfter.Data.EliteSelections)
}
