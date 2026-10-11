package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"reflect"
	"testing"
)

func TestAllElitePreservedLegionPartyReloadsCompanions(t *testing.T) {
	t.Setenv("DFO_ELITE_COMPANION_PILOT", "all")
	w, _ := allEliteWorld(t)
	w.channelType, w.channelWorldIsolated, w.soloPartyBootstrap = 119, true, true
	for _, id := range []uint32{100005112, 100004995} {
		s := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: id, Script: catalog.ScriptRecord{Path: "legion/apocalypse.dgn"}}}
		s.Room.Map = 17
		sel := protocol.DungeonSelection{ID: id, Party: 65535}
		plan, err := w.dungeonEntryPlanPreservingParty(context.Background(), "apocalypse_ack", 2187, sel, s, &w.characters.ChannelContext, true)
		if err != nil {
			t.Fatal(err)
		}
		var ids []uint16
		for _, p := range plan {
			if p.Kind != 0 {
				continue
			}
			if p.ID == 9 {
				t.Fatal("elite reload replaced native legion party")
			}
			if p.ID == 28 || p.ID == 1382 || p.ID == 1879 || p.ID == 29 {
				ids = append(ids, p.ID)
			}
		}
		if !reflect.DeepEqual(ids, []uint16{28, 1382, 1879, 29}) {
			t.Fatalf("legion %d: %v", id, ids)
		}
		t.Setenv("DFO_ELITE_COMPANION_PILOT", "")
		base, err := w.dungeonEntryPlanPreservingParty(context.Background(), "apocalypse_ack", 2187, sel, s, &w.characters.ChannelContext, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(adventurePacket(base, 1382)) != 0 || len(adventurePacket(base, 1879)) != 0 {
			t.Fatal("default preserved-party path changed")
		}
		t.Setenv("DFO_ELITE_COMPANION_PILOT", "all")
	}
}
