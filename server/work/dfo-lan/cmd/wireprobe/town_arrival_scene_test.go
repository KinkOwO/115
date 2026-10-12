package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/world"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRejectedWestCoastSceneGateCannotStartDungeon(t *testing.T) {
	// Native CMD16 body from the 2026-09-28 West Coast capture, following a
	// rejected CMD15 for the same scenario ID.
	body, err := hex.DecodeString("34f2f5050000000000ffff0000000000782f0000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{
		role: database.Character{ID: 11}, dungeons: &catalog.DungeonCatalog{},
		state: database.WorldState{Position: database.WorldPosition{Town: 40, Area: 0}},
		townArrivalScenes: map[uint32]catalog.TownArrivalScene{
			100004404: {QuestID: 12152, Town: 40, Area: 0, DungeonID: 100004404},
		},
	}
	_, _, err = w.selectDungeon(body)
	if err == nil || !strings.Contains(err.Error(), "no approved matching gate") {
		t.Fatalf("rejected scene gate allowed dungeon selection: %v", err)
	}
	if w.activeDungeon != nil {
		t.Fatal("town character was moved into a dungeon session")
	}
}

// Keep the real dispatcher, world persistence, loading response and session
// promotion in the fixture: a helper-only check missed the original race.
func townArrivalLoadingClient(t *testing.T) (*gameConnection, *[]map[string]any) {
	t.Helper()
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "storage.sqlite")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateWorld} {
		require.NoError(t, migrate(ctx))
	}
	version := strings.Repeat("a", 64)
	account, err := store.DevelopmentAccount(ctx, "town-arrival-loading")
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "town-arrival-loading", ConfigVersion: version, Request: []byte{}, State: json.RawMessage(`{"level":22}`)}, 24)
	require.NoError(t, err)
	role.WireID = 1
	origin := database.WorldPosition{Town: 39, Area: 1, X: 558, Y: 351, Return: &database.WorldReturn{Town: 39, Area: 0, X: 558, Y: 351}}
	state, err := store.LoadWorld(ctx, account, role.ID, 0, origin, version)
	require.NoError(t, err)
	client, _, events := newDispatchTestClient()
	client.selectedCharacterID = role.ID
	client.worldState = &worldSession{
		role: role, account: account, store: store, level: 22, state: state,
		service: &world.Service{Store: store, Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
			"39/1": {Town: 39, Area: 1, Walkable: [][4]int32{{0, 0, 1500, 500}}, SeriaReturnWarp: true, ReturnWarpBounds: [][4]int32{{0, 0, 1500, 500}}, Portals: []catalog.Portal{{Town: 39, Area: 0, Bounds: [4]int32{0, 0, 1500, 500}}}},
			"39/0": {Town: 39, Area: 0, Walkable: [][4]int32{{0, 0, 1500, 500}}},
		}}},
		townArrivalScenes:  map[uint32]catalog.TownArrivalScene{100004400: {QuestID: 21029, Town: 39, Area: 1, DungeonID: 100004400}},
		pendingTownArrival: &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 100004400}, Room: catalog.DungeonRoom{Map: 100014294}},
	}
	return client, events
}

func TestTownArrivalPositionSyncThenLoading(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   uint16
		body string
	}{
		{"official_motion", 623, "2e025c01056400000000000000000000"},
		{"local_motion", 623, "2e025c01086400000000000000000000"},
		{"position", 35, "2e025c0108640000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, events := townArrivalLoadingClient(t)
			pending := client.worldState.pendingTownArrival
			body, err := hex.DecodeString(tc.body)
			require.NoError(t, err)
			require.Equal(t, dispatchHandled, client.dispatchWorldAndQuests(&clientRequest{frame: wire.Frame{Type: 1, ID: tc.id}, plaintext: body, verified: true}))
			require.Same(t, pending, client.worldState.pendingTownArrival)
			require.Equal(t, uint16(348), client.worldState.state.Position.Y)
			require.Equal(t, dispatchHandled, client.dispatchDungeon(&clientRequest{frame: wire.Frame{Type: 1, ID: 37}, plaintext: make([]byte, 16), verified: true}))
			require.Same(t, pending, client.worldState.activeDungeon)
			require.Nil(t, client.worldState.pendingTownArrival)
			require.True(t, pending.Loaded)
			var ack, loaded bool
			for _, event := range *events {
				require.NotEqual(t, "dungeon_request_refused", event["kind"])
				ack = ack || event["kind"] == "dungeon_loading_ack"
				loaded = loaded || event["kind"] == "dungeon_loading_complete"
			}
			require.True(t, ack)
			require.True(t, loaded)
		})
	}
}

func TestTownArrivalActualAreaExitCancelsLoading(t *testing.T) {
	for _, id := range []uint16{36, 1418} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			client, _ := townArrivalLoadingClient(t)
			var body []byte
			if id == 36 {
				var err error
				body, err = hex.DecodeString("27000000000000002e025f01002700000001000000000000")
				require.NoError(t, err)
			}
			require.Equal(t, dispatchHandled, client.dispatchWorldAndQuests(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, plaintext: body, verified: true}))
			require.Equal(t, uint32(0), client.worldState.state.Position.Area)
			require.Nil(t, client.worldState.pendingTownArrival)
			require.Nil(t, client.worldState.activeDungeon)
		})
	}
}

func TestTownArrivalRejectedAreaExitKeepsLoading(t *testing.T) {
	client, events := townArrivalLoadingClient(t)
	client.worldState.state.Position.Return = nil
	pending := client.worldState.pendingTownArrival
	// An absent destination is refused by the world owner; the character did
	// not leave the scene origin and its approved loading session still exists.
	body, err := hex.DecodeString("27000000020000002e025f01002700000001000000000000")
	require.NoError(t, err)
	require.Equal(t, dispatchHandled, client.dispatchWorldAndQuests(&clientRequest{frame: wire.Frame{Type: 1, ID: 36}, plaintext: body, verified: true}))
	require.Equal(t, uint32(1), client.worldState.state.Position.Area)
	require.Same(t, pending, client.worldState.pendingTownArrival)
	var refused bool
	for _, event := range *events {
		refused = refused || event["kind"] == "area_refused"
	}
	require.True(t, refused)
}

func TestWestCoastOriginSyncKeepsPendingScene(t *testing.T) {
	w := &worldSession{
		pendingTownArrival: &dungeon.Session{},
		state:              database.WorldState{Position: database.WorldPosition{Town: 40, Area: 0, X: 412, Y: 181}},
	}
	actual, err := hex.DecodeString("28000000000000009c01b500002800000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if !w.isTownArrivalOriginSync(36, actual) {
		t.Fatal("captured CMD36 origin sync would discard the pending scene")
	}
	actual[8]++
	if w.isTownArrivalOriginSync(36, actual) || w.isTownArrivalOriginSync(35, actual) {
		t.Fatal("a different town movement must not look like origin sync")
	}
}

func TestWestCoastSceneSelectsQuestMaze(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	body, err := hex.DecodeString("34f2f5050000000000ffff0000000000782f0000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	r, err := protocol.DecodeDungeonSelection(body)
	if err != nil {
		t.Fatal(err)
	}
	s, err := dungeon.Select(c, r, 96, map[uint16]bool{12152: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Definition.ID != 100004404 || s.Room.Map != 100014298 {
		t.Fatalf("scene selected dungeon %d map %d", s.Definition.ID, s.Room.Map)
	}
}
