package main

import "dfolan/internal/game/protocol"

// A roster must use the same identity as the selected actor's N2 introduction.
// Nil character services occur in isolated world-plan tests without actors.
func (w *worldSession) soloPartyInfo(contextOverride *[2]byte) ([]byte, error) {
	var identity [2]byte
	if w.characters != nil {
		identity = w.characters.ChannelContext
	}
	if contextOverride != nil {
		identity = *contextOverride
	}
	return protocol.SoloPartyInfo(w.role.WireID, identity)
}
