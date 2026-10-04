package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
)

func clearSelectedWorld(w *worldSession) {
	if w != nil {
		w.moon = moonSoloState{}
		w.role = database.Character{}
		w.level = 0
		w.odyssey = false
		w.state = database.WorldState{}
		w.activeDungeon = nil
		w.soloPartyReady = false
		w.ispinsRepeatPending = false
		w.ispinsRetryPending = false
		w.ispins = nil
		w.specialWarpPending = false
	}
}

func (w *worldSession) returnDestination() ([]byte, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("return request without selected character")
	}
	position := w.state.Position
	r := position.Return
	if r == nil {
		return nil, fmt.Errorf("character has no saved room origin")
	}
	// Reuse normal source geometry, level and ownership-backed position
	// validation. The response never accepts a client-supplied destination.
	req := protocol.AreaChangeRequest{Town: r.Town, Area: r.Area, X: r.X, Y: r.Y, PreviousTown: position.Town, PreviousArea: uint16(position.Area)}
	if _, e := w.service.Transition(w.level, w.odyssey, position, req); e != nil {
		return nil, e
	}
	return protocol.VillageReturnSuccess(r.Town, r.Area), nil
}
