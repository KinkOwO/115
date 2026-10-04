package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/world"
	"testing"
)

func TestMenuReturnUsesOwnedSavedOrigin(t *testing.T) {
	w := &worldSession{account: 7, role: database.Character{ID: 5, AccountID: 7, WireID: 3}, level: 1, service: &world.Service{
		Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
			"38/1": {SeriaReturnWarp: true, Walkable: [][4]int32{{400, 100, 400, 300}}},
			"38/0": {Walkable: [][4]int32{{0, 100, 1000, 300}}},
		}},
	}, state: database.WorldState{Position: database.WorldPosition{Town: 38, Area: 1, X: 544, Y: 311, Return: &database.WorldReturn{Town: 38, Area: 0, X: 561, Y: 234}}}}
	p, e := w.returnDestination()
	if e != nil {
		t.Fatal(e)
	}
	keys := make([]byte, wire.SessionKeyBytes)
	packets, e := preparePackets(keys, []outboundPacket{{"return", 1, 7, protocol.MenuLeaveSuccess()}, {"exit", 1, 3, protocol.MenuLeaveSuccess()}, {"town", 1, 1301, p}})
	if e != nil || len(packets) != 3 {
		t.Fatalf("menu transport: %v", e)
	}
	// A missing/unwalkable origin is not replaced by an arbitrary default map.
	w.state.Position.Return.X = 2000
	if _, e = w.returnDestination(); e == nil {
		t.Fatal("invalid saved origin accepted")
	}
	clearSelectedWorld(w)
	if _, e = w.returnDestination(); e == nil {
		t.Fatal("old role return survives character switch")
	}
	if e = w.handle(35, make([]byte, 8), nil, nil); e == nil {
		t.Fatal("old role movement survives character switch")
	}
	if w.account != 7 || w.service == nil {
		t.Fatal("account services lost during character switch")
	}
}
