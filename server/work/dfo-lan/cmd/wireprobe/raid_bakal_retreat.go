package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

func (w *worldSession) bakalNativeCampReturn(area uint32) ([]outboundPacket, error) {
	if w.bakalOpening == nil || w.state.Position.Area != area {
		return nil, fmt.Errorf("Bakal camp return is outside the owned camp origin")
	}
	_, plan, err := w.bakalRetreat(protocol.SettlementExit{State: 1, Option: 2})
	if err != nil {
		return nil, err
	}
	plan[0] = outboundPacket{"bakal_native_camp_return_ack", 1, 2074, []byte{1}}
	w.activeDungeon = nil
	w.deathSent = nil
	w.completionSent = false
	w.drops = nil
	w.resetCards()
	return plan, nil
}

// Native retreat sends CMD72 01 02 01 while combat remains unfinished.
// It returns the real subparty to its persisted camp without ordinary cards
// or a raid clear. Post-send settlement_exit_ack owns dungeon cleanup.
func (w *worldSession) bakalRetreat(r protocol.SettlementExit) (*dungeon.Session, []outboundPacket, error) {
	if w.channelType != 82 || !w.raidWaiting || !w.bakalOpening.Active() || w.activeDungeon == nil || w.bakalRules == nil || r.Option != 2 {
		return nil, nil, fmt.Errorf("Bakal retreat requires active owned combat and camp exit")
	}
	members := w.bakalOpening.Members()
	if len(members) != 1 || members[0].Actor != w.role.WireID || members[0].Position != w.bakalParty {
		return nil, nil, fmt.Errorf("Bakal retreat member ownership changed")
	}
	var camp uint32
	for id, slot := range w.bakalRules.Slots {
		if slot.Kind == "camp" && slot.TownArea == w.state.Position.Area && w.bakalTown != 0 && w.state.Position.Town == w.bakalTown {
			camp = id
		}
	}
	if camp == 0 {
		return nil, nil, fmt.Errorf("Bakal retreat has no native camp origin")
	}
	ack := outboundPacket{"settlement_focus_ack", 1, 72, protocol.SettlementExitSuccess(r)}
	if r.State == 2 {
		return nil, []outboundPacket{ack}, nil
	}
	if len(w.bakalRules.Events) > 0 {
		if err := w.bakalOpening.GiveupDungeon(w.activeDungeon.Definition.ID, time.Now()); err != nil {
			return nil, nil, err
		}
	}
	party, err := w.bakalPartyAt(camp)
	if err != nil {
		return nil, nil, err
	}
	body, err := protocol.BakalPartyInfoPayload([]protocol.BakalPartyInfo{party})
	if err != nil {
		return nil, nil, err
	}
	route, err := w.leaveDungeon()
	if err != nil {
		return nil, nil, err
	}
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	ack.Name = "settlement_exit_ack"
	plan := append([]outboundPacket{ack}, route[1:]...)
	return nil, append(plan, outboundPacket{"bakal_retreat_to_camp", 0, 2285, body}), nil
}
