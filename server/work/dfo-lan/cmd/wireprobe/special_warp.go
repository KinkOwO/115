package main

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

func (w *worldSession) prepareSpecialWarp(p []byte) ([]outboundPacket, error) {
	if err := protocol.DecodeSpecialWarpPreparation(p); err != nil {
		return nil, err
	}
	if w == nil || w.role.ID == 0 || w.role.AccountID != w.account || w.service == nil || w.activeDungeon != nil || w.selectingDungeon {
		return nil, fmt.Errorf("special warp requires owned town character")
	}
	// The preparation only proves the character is standing where the server
	// last saved it, so it uses the restoration gate.
	if err := w.service.ValidateRestoredPosition(w.level, w.odyssey, w.state.Position); err != nil {
		return nil, err
	}
	if w.specialWarpPending {
		return nil, nil
	}
	body, err := protocol.SpecialWarpStart(w.role.WireID)
	if err != nil {
		return nil, err
	}
	w.specialWarpPending = true
	return []outboundPacket{{"special_warp_prepared", 0, 365, body}}, nil
}
