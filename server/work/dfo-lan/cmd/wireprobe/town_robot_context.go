package main

import "dfolan/internal/game/protocol"

// Reuses the current-client REQUEST_PEER action-15 vector. Only visible
// synthetic actors are handled here; native player/party flows remain separate.
func (client *gameConnection) dispatchTownRobotContext(q *clientRequest) dispatchAction {
	if q.frame.ID != 10 || len(q.plaintext) < 3 || q.plaintext[2] != 15 || client.hub == nil || client.hub.townRobots == nil {
		return dispatchNext
	}
	w := client.worldState
	if !q.verified || !client.bootstrapped || w == nil || w.peer == nil || client.selectedCharacterID == 0 || w.role.ID != client.selectedCharacterID || w.role.AccountID != w.account || w.peer.roleID != w.role.ID {
		return dispatchHandled
	}
	actor, err := protocol.DecodePlayerContext115(q.plaintext)
	if err != nil {
		return dispatchHandled
	}
	f := client.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	client.hub.mu.Lock()
	found := false
	if _, joined := client.hub.peers[w.peer]; joined {
		for p := range client.hub.peers {
			if p.robot && p.actorID == actor && visible(w.peer, p) {
				found = true
				break
			}
		}
	}
	client.hub.mu.Unlock()
	if !found {
		return dispatchNext
	}
	partyID := uint16(65535)
	for _, party := range f.parties {
		for _, bot := range party.bots {
			if bot.peer.actorID == actor {
				partyID = party.id
			}
		}
	}
	body, err := protocol.PlayerContextReply115(actor, w.peer.actorID, partyID)
	if err != nil {
		return dispatchHandled
	}
	if err = client.output.send(0, 7, body); err != nil {
		return dispatchClose
	}
	client.event(map[string]any{"kind": "town_robot_context_sent", "target_actor": actor, "native_party_supported": true, "attempt": "1/3"})
	return dispatchHandled
}
