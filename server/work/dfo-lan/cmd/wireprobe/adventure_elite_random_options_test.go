package main

import (
	"context"
	"dfolan/internal/adventureelite"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdventureEliteStoredRandomOptionLoadPreservesCompanion(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, apc, slot := eliteEligibilityWorld(t, 7, 0, false)
	w.channelType, w.eliteChannelDirectory = 22, elitePreparationDirectory(22)
	ctx := context.Background()
	record := protocol.OrdinaryItem(14, 100060157, 0)
	copy(record[60:76], []byte{1, 74, 0, 0, 16, 0, 0, 23, 0, 0, 2, 255, 0, 0, 0, 0})
	state, err := inventory.SaveBag(apc.State, inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{
		{Slot: 14, Template: 100060157, Durability: 60, Record: record[:]},
	}})
	require.NoError(t, err)
	apc, _, err = w.store.CommitCharacterEvent(ctx, w.account, apc.ID, apc.ConfigVersion,
		"fixture-unseal", "fixture", func(database.Character) (json.RawMessage, json.RawMessage, error) {
			return state, json.RawMessage(`{}`), nil
		})
	require.NoError(t, err)
	r := eliteEligibilityRequest(t, slot, 19)
	_, err = w.setAdventureElite(ctx, r, r, "random-option-test")
	require.NoError(t, err)
	packets, err := w.loadAdventureElite(ctx, []byte{2, 0})
	require.NoError(t, err)
	require.Len(t, packets, 2)
	require.Equal(t, []uint16{1382, 1879}, []uint16{packets[0].ID, packets[1].ID})
	require.NotNil(t, w.adventureElitePrepared)
	require.Equal(t, [3]int64{apc.ID, 0, 0}, w.adventureElitePrepared.Selected)
	rows, err := w.store.Characters(ctx, w.account)
	require.NoError(t, err)
	for _, row := range rows {
		if row.ID == apc.ID {
			require.JSONEq(t, string(apc.State), string(row.State), "loading mutated saved equipment/skills")
		}
	}
	decision := adventureEliteDiagnostic(w, 1811, packets, nil)
	require.Equal(t, "0.3.20", decision["equipment_projection_version"])
	require.Equal(t, "1/3", decision["random_option_projection_attempt"])
	require.Equal(t, "0.3.21", decision["upstream_recovery_version"])
	require.Equal(t, "db707d59", decision["upstream_recovery_baseline"])
}

// Optional read-only snapshot exported from the reported account, never the
// live SQLite database. Also verifies the source archive used for unsealing.
func TestAdventureEliteRandomOptionNativeSavedSnapshots(t *testing.T) {
	file, archive := os.Getenv("DFO_ELITE_SAVED_SNAPSHOT_TEST"), os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if file == "" || archive == "" {
		t.Skip("set snapshot file and native archive for reported-account encoding check")
	}
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: archive,
		ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256")})
	require.NoError(t, err)
	defer source.Close()
	index, err := source.ItemIndex("")
	require.NoError(t, err)
	full, err := source.Equipment(index)
	require.NoError(t, err)
	defer full.Close()
	options, err := source.RandomOptions()
	require.NoError(t, err)
	for _, id := range []uint32{100060157, 101020323} {
		definition, err := full.Definition(id)
		require.NoError(t, err)
		t.Logf("source equipment=%d path=%s sha256=%s magic-seal=%v; random-option source hashes=%v",
			definition.ID, definition.Path, definition.SHA256, definition.Fields["[random option]"], options.TableSHA256)
	}
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	var roles []database.Character
	require.NoError(t, json.Unmarshal(b, &roles))
	require.NotEmpty(t, roles)
	jobs, err := source.Characters("")
	require.NoError(t, err)
	learning, err := source.Learning(jobs)
	require.NoError(t, err)
	service := &character.Service{Catalog: jobs, Learning: learning}
	for _, role := range roles {
		snapshot, err := service.TagCharacterSnapshot(role)
		require.NoError(t, err)
		// Keep the synthetic owner distinct from every saved companion wire ID.
		body, err := protocol.AdventureEliteCharacterInfo(65534, []byte{1}, []protocol.TagCharacter{snapshot})
		require.NoError(t, err)
		t.Logf("role=%d worn=%d encodedBytes=%d", role.ID, len(snapshot.Worn), len(body))
	}
}
