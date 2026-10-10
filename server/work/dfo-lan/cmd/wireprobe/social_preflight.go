package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"time"
)

// This preflight returns native failures for self/absent targets. Real target
// invitations stay on their original path until the relationship transaction
// and native nonempty notification contracts are complete.
func (client *gameConnection) dispatchSocialPreflight(request *clientRequest) dispatchAction {
	if !client.bootstrapped || client.selectedCharacterID == 0 || request.frame.ID != 1431 || client.worldState == nil || client.worldState.store == nil {
		return dispatchNext
	}
	if !request.verified {
		client.event(map[string]any{"kind": "social_request_rejected", "id": uint16(1431), "reason": "checksum failed"})
		return dispatchHandled
	}
	r, err := protocol.DecodeSocialAddRequest(request.plaintext)
	if err != nil {
		client.event(map[string]any{"kind": "social_request_rejected", "id": uint16(1431), "reason": err.Error()})
		return dispatchHandled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	profile, err := client.worldState.prepareAdventure(ctx)
	if err != nil {
		client.event(map[string]any{"kind": "social_request_rejected", "id": uint16(1431), "reason": err.Error()})
		return dispatchHandled
	}
	if r.Adventure != profile.Name {
		available, err := client.worldState.store.AdventureNameAvailable(ctx, 0, r.Adventure)
		if err != nil {
			client.event(map[string]any{"kind": "social_request_rejected", "id": uint16(1431), "reason": err.Error()})
			return dispatchHandled
		}
		if !available {
			return dispatchNext
		}
		// The native handler independently maps 21 to an absent adventure.
		// A missing target must not leave the dialog waiting or create a row.
		if client.output.send(1, 1431, protocol.SocialAddRejected(21)) != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "social_missing_target_rejected", "id": uint16(1431), "attempt": "2/3"})
		return dispatchHandled
	}
	// Error 217 is a registered native invalid-adventure-name branch, rather
	// than a made-up self-add opcode or a false successful invitation.
	if err := client.output.send(1, 1431, protocol.SocialAddRejected(217)); err != nil {
		return dispatchClose
	}
	client.event(map[string]any{"kind": "social_self_add_rejected", "id": uint16(1431), "character_id": client.selectedCharacterID, "attempt": "1/3"})
	return dispatchHandled
}
