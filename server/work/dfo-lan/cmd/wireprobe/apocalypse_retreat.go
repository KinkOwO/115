package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
)

// C72 01 02 00/01 are captured Apocalypse retreat confirmations, not a
// request for ordinary dungeon cards. The owned run and its COS waiting point
// authorize the return even before the navigation gate or any boss is cleared.
func (s *legionSession) prepareApocalypseRetreat(w *worldSession, body []byte) ([]outboundPacket, bool, error) {
	r, err := protocol.DecodeSettlementExit(body)
	if err != nil {
		return nil, false, err
	}
	// Current 14250A850 writes this byte with SETE, so both 0 and 1 are native
	// boolean branches. Late Lopes live confirmation uses 0. It is not an
	// authorization tag; the source-bound owner below supplies that authority.
	if r.Option != 2 || body[2] > 1 {
		return nil, false, fmt.Errorf("apocalypse retreat requires native waiting-area exit")
	}
	if err := s.validateApocalypseReturn(w); err != nil {
		return nil, false, err
	}
	ack := outboundPacket{"apocalypse_retreat_focus_ack", 1, 72, protocol.SettlementExitSuccess(r)}
	if r.State == 2 {
		return []outboundPacket{ack}, false, nil
	}
	town, err := s.apocalypseWaitingTransport(w)
	if err != nil {
		return nil, false, err
	}
	reset := s.apocalypseClosedProjection()
	ack.Name = "apocalypse_retreat_ack"
	plan := []outboundPacket{{"apocalypse_retreat_roles_and_ui_cleared", 0, legion.NotiLegionInfo, reset}, ack}
	return append(plan, town[1:]...), true, nil
}

func (s *legionSession) validateApocalypseReturn(w *worldSession) error {
	if s == nil || w == nil || w.channelType != 119 || !w.ownsApocalypseDungeon() || w.apocalypseRun == nil || w.activeDungeon != w.apocalypseRun.Active || !w.activeDungeon.Loaded || w.characters == nil || w.service == nil || w.dungeons == nil || s.party == nil || s.session == nil || s.plan == nil {
		return fmt.Errorf("apocalypse retreat requires an owned loaded run")
	}
	run := w.apocalypseRun
	if run.Character != w.role.ID || run.Operation != s.plan.OperationID || s.party.CharacterID != w.role.ID || s.party.Actor != w.role.WireID || s.party.Context != w.characters.ChannelContext || s.session.CharacterID != w.role.ID || !s.session.Entered || s.session.Failed {
		return fmt.Errorf("apocalypse retreat character or party ownership changed")
	}
	entry, err := legion.BuildEntryPlan(s.contents, s.catalog, *w.dungeons, dungeon.ValidateSelection)
	if err != nil || entry != run.Entry || s.catalog.Source != catalog.ApocalypseSource || s.catalog.SHA256 != run.CatalogHash || entry.Source != w.service.Catalog.Source.Checksum || w.state.Position.Town != entry.Waiting.Town || w.state.Position.Area != entry.Waiting.Area {
		return fmt.Errorf("apocalypse retreat source or persisted waiting origin changed")
	}
	ownedDestination := false
	for _, id := range entry.Destinations {
		ownedDestination = ownedDestination || id == w.activeDungeon.Definition.ID
	}
	if !ownedDestination || len(s.projection) != legion.LegionInfoSize || binary.LittleEndian.Uint16(s.projection) != legion.ApocalypseContent || s.projection[4] != byte(run.Operation-1) || binary.LittleEndian.Uint32(s.projection[13:]) != run.Stage {
		return fmt.Errorf("apocalypse retreat destination or delivered projection changed")
	}
	return nil
}

func (s *legionSession) apocalypseWaitingTransport(w *worldSession) ([]outboundPacket, error) {
	// Preparing the common town transport must not publish the actor or mutate
	// live run state before all frames have been delivered.
	copy := *w
	copy.hub, copy.peer = nil, nil
	// leaveDungeon also serves modes with mutable death/quota state. None is
	// an Apocalypse owner; refuse that mixed context before preparing a return.
	if w.inTutorial || w.bleedingMineStart != nil || w.blackPurgatory.prepared || w.blackPurgatory.created {
		return nil, fmt.Errorf("apocalypse return has foreign content state")
	}
	if w.pilotDeath != nil {
		death := *w.pilotDeath
		copy.pilotDeath = &death
	}
	route, err := copy.leaveDungeon()
	if err != nil {
		return nil, err
	}
	// N14 restores worn visuals, not the local N13 equipment container.
	// Restore that container after the returned actor and before its visual
	// update, using the same persisted snapshot as the verified entry path.
	worn, err := inventory.WornPayload(w.role.State)
	if err != nil {
		return nil, err
	}
	at := len(route)
	for i, packet := range route {
		if packet.Name == "town_worn_visuals_restored" {
			at = i
			break
		}
	}
	restored := make([]outboundPacket, 0, len(route)+1)
	restored = append(restored, route[:at]...)
	restored = append(restored, outboundPacket{"apocalypse_waiting_worn_inventory_restored", 0, 13, worn})
	route = append(restored, route[at:]...)
	return route, nil
}

func (s *legionSession) apocalypseClosedProjection() []byte {
	// 14069A4D0 applies role0 through14069B820 and clears/hides the main window
	// on State0. Preserve stage and unrelated fields so a return cannot trigger
	// the stage0->1 automatic portal while resetting the content UI.
	reset := append([]byte(nil), s.projection...)
	binary.LittleEndian.PutUint32(reset[5:], 0)
	clear(reset[109:125])
	return reset
}

func (client *gameConnection) finishApocalypseRetreat() {
	client.finishApocalypseReturn("apocalypse_retreat", "apocalypse_retreat_completed")
}

func (client *gameConnection) finishApocalypseReturn(reason, event string) {
	w := client.worldState
	w.activeDungeon, w.apocalypseNavigation, w.apocalypseRun = nil, nil, nil
	w.pendingTownArrival = nil
	w.selectingDungeon, w.completionSent, w.resultSent = false, false, false
	w.approvedDungeonGate = 0
	w.completionErr, w.deathSent, w.drops = nil, nil, nil
	w.resetCards()
	if w.pilotDeath != nil {
		w.pilotDeath.Dead = false
	}
	if note, closed := client.legionState.abandonOnLeave(reason, w.role.ID); closed {
		client.event(note)
	}
	w.enterArea()
	if err := w.announceSelf(client.event); err != nil {
		client.event(map[string]any{"kind": "area_presence_error", "error": err.Error()})
	}
	client.event(map[string]any{"kind": event, "character_id": w.role.ID, "town": w.state.Position.Town, "area": w.state.Position.Area, "attempt": "1/3", "client_acceptance": "pending"})
}
