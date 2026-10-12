package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/wire"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// Replay the normal gate and bodyless cancel through the actual dispatcher.
// GF115 wire orders 20628/31677/31831 all return N23 then N24 without N132.
func TestDungeonSelectionCancelUsesOfficialTownTransition(t *testing.T) {
	client, _, events := newDispatchTestClient()
	client.selectedCharacterID = 1
	w := client.worldState
	w.role = database.Character{ID: 1, WireID: 1}
	w.level = 100
	w.state.Position = database.WorldPosition{Town: 40, Area: 7, X: 1045, Y: 215}
	w.service = &world.Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		"40/7": {Town: 40, Area: 7, Kind: "[dungeon gate]", Walkable: [][4]int32{{0, 0, 2000, 500}}},
	}}}
	origin := w.state.Position
	gate := &clientRequest{frame: wire.Frame{Type: 1, ID: 15}, plaintext: make([]byte, 8), verified: true}
	cancel := &clientRequest{frame: wire.Frame{Type: 1, ID: 132}, verified: true}
	for attempt := 0; attempt < 2; attempt++ {
		require.Equal(t, dispatchHandled, client.dispatchDungeon(gate))
		require.True(t, w.selectingDungeon, "normal gate must remain usable after returning")
		*events = nil
		require.Equal(t, dispatchHandled, client.dispatchDungeon(cancel))
		require.False(t, w.selectingDungeon, "one accepted cancel ends the owned selection")
		require.Zero(t, w.approvedDungeonGate)
		require.Nil(t, w.activeDungeon)
		require.Equal(t, origin, w.state.Position, "cancel restores the owned origin without moving the door")
		var ids []uint16
		for _, event := range *events {
			require.NotEqual(t, "dungeon_request_refused", event["kind"])
			if body, ok := event["plain_hex"].(string); ok {
				id := event["id"].(uint16)
				ids = append(ids, id)
				require.NotContains(t, []uint16{15, 27, 42, 132}, id, "cancel must not reset or reopen the selection UI")
				if id == 23 {
					payload, err := hex.DecodeString(body)
					require.NoError(t, err)
					require.Equal(t, origin.Town, binary.LittleEndian.Uint32(payload[2:]))
					require.Equal(t, origin.Area, binary.LittleEndian.Uint32(payload[6:]))
					require.Equal(t, origin.X, binary.LittleEndian.Uint16(payload[10:]))
					require.Equal(t, origin.Y, binary.LittleEndian.Uint16(payload[12:]))
				}
			}
		}
		require.GreaterOrEqual(t, len(ids), 2)
		require.Equal(t, []uint16{23, 24}, ids[:2], "GF115 town-transition head")
	}
}

func TestDungeonSelectionCancelRejectsUnownedAndMalformedRequests(t *testing.T) {
	for _, tc := range []struct {
		name      string
		selection bool
		active    *dungeon.Session
		body      []byte
	}{
		{name: "not selecting"},
		{name: "active dungeon", selection: true, active: &dungeon.Session{}},
		{name: "nonempty body", selection: true, body: []byte{0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &worldSession{selectingDungeon: tc.selection, activeDungeon: tc.active}
			plan, err := w.returnFromDungeonSelection(tc.body)
			require.ErrorContains(t, err, "owned dungeon selection")
			require.Empty(t, plan)
			require.Equal(t, tc.selection, w.selectingDungeon)
			require.Same(t, tc.active, w.activeDungeon)
		})
	}
}

func TestDungeonSelectionCancelKeepsGiveUpAcknowledgement(t *testing.T) {
	client, _, events := newDispatchTestClient()
	client.selectedCharacterID = 1
	w := client.worldState
	w.role = database.Character{ID: 1, WireID: 1}
	w.state.Position = database.WorldPosition{Town: 40, Area: 7, X: 1045, Y: 215}
	w.activeDungeon = &dungeon.Session{}
	require.Equal(t, dispatchHandled, client.dispatchDungeon(&clientRequest{
		frame: wire.Frame{Type: 1, ID: 42}, verified: true,
	}))
	require.Nil(t, w.activeDungeon)
	var ids []uint16
	for _, event := range *events {
		if _, ok := event["plain_hex"].(string); ok {
			ids = append(ids, event["id"].(uint16))
		}
	}
	require.Equal(t, []uint16{42, 3, 23, 24}, ids, "in-dungeon give-up retains its own ACK and state path")
}
