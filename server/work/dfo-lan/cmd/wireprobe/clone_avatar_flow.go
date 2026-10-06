package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
)

func cloneAvatarSourcePackets(state json.RawMessage) ([]outboundPacket, error) {
	full, last, err := inventory.CloneAvatarSourcePayloads(state)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"clone_avatar_sources_replaced", 0, protocol.CloneAvatarSourceOpcode, full}, {"clone_avatar_source_slot11_updated", 0, protocol.CloneAvatarSourceOpcode, last}}, nil
}
