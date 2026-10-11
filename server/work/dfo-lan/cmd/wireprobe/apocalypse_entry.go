package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
)

// Initial navigation uses destination key0 of the native COS table, not a
// general stage->destination mapping. Later stages require the owned route
// scheduler. CTP operation choice remains independent of the map transport.
func (s *legionSession) prepareNavigationEntry(w *worldSession, stage uint32) (legionResult, error) {
	fail := func(reason string) (legionResult, error) {
		return legionResult{}, fmt.Errorf("apocalypse entry executor unavailable: operation=%d stage=%d: %s", s.plan.OperationID, stage, reason)
	}
	if stage != 0 || w.channelType != 119 || w.dungeons == nil || w.service == nil || w.characters == nil || w.activeDungeon != nil || w.selectingDungeon {
		return fail("source navigation entry context required")
	}
	if s.party == nil || s.party.CharacterID != w.role.ID || s.party.Actor != w.role.WireID || s.party.Context != w.characters.ChannelContext {
		return fail("owned party required")
	}
	entry, err := legion.BuildEntryPlan(s.contents, s.catalog, *w.dungeons, dungeon.ValidateSelection)
	if err != nil {
		return fail(err.Error())
	}
	if s.clock == nil {
		return fail("source phase clock unavailable")
	}
	first, ok := s.clock.Next(0)
	if !ok || first <= 0 || first >= int64(len(entry.Destinations)) || len(s.projection) != legion.LegionInfoSize || binary.LittleEndian.Uint16(s.projection) != legion.ApocalypseContent {
		return fail("source first combat phase or delivered projection unavailable")
	}
	initial := append([]byte(nil), s.projection...)
	initial[2+27], initial[2+27+12] = 0, byte(first)
	if w.service.Catalog.Source.Checksum != entry.Source || w.level < entry.MinimumLevel || w.state.Position.Town != entry.Waiting.Town || w.state.Position.Area != entry.Waiting.Area {
		return fail("source generation, level or waiting area mismatch")
	}
	if err := w.service.ValidateRestoredPosition(w.level, w.odyssey, w.state.Position); err != nil {
		return fail(err.Error())
	}
	// This is the existing common transport's base maze selection; operation
	// difficulty is already owned by the source CTP plan, never the stage byte.
	sel := protocol.DungeonSelection{ID: entry.Destinations[0], Party: 65535}
	navigation, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return fail(err.Error())
	}
	content := s.contents.Contents["Apocalypse"]
	if content.NeedFatigue != 0 {
		return fail("fatigue-bearing content admission is not bound")
	}
	navigation.Definition.NoFatigue = true // source COS [need fatigue], private run only
	frames, err := w.dungeonEntryPlanPreservingParty(context.Background(), "apocalypse_enter_ack", legion.CmdEnterDungeon, sel, navigation, &s.party.Context, true)
	if err != nil {
		return fail(err.Error())
	}
	frames[0].Payload = legion.EnterDungeonAck()
	actor, err := protocol.UserState(w.role.WireID, protocol.UserStateDungeon)
	if err != nil {
		return fail(err.Error())
	}
	plan := []outboundPacket{frames[0], {"apocalypse_initial_destinations", 0, legion.NotiLegionInfo, initial}}
	for _, packet := range frames[1:] {
		if packet.ID == 28 {
			// Current N27 14530383D ->1459B53C0 establishes world state2.
			// N28 1452AC840 binds DGN; N29 1452B7100 consumes that DGN and
			// starts map loading (146D554A0/146D30300). N3 sets the actor's
			// separate room-movement state. No unsolicited ACK16 or FX toggle.
			plan = append(plan, outboundPacket{"apocalypse_actor_dungeon", 0, 3, actor}, outboundPacket{"apocalypse_navigation_selection", 0, 27, protocol.EnterDungeonSelection()})
		}
		plan = append(plan, packet)
	}
	return legionResult{Packets: plan, Navigation: navigation, Route: &apocalypseRoute{Entry: entry, Active: navigation, Character: w.role.ID, Operation: s.plan.OperationID, First: byte(first), CatalogHash: s.catalog.SHA256}, Events: []map[string]any{{"kind": "apocalypse_navigation_prepared", "character_id": w.role.ID, "stage": stage, "operation": s.plan.OperationID, "dungeon": navigation.Definition.ID, "map": navigation.Room.Map, "source": entry.Source, "attempt": "1/3"}}}, nil
}
