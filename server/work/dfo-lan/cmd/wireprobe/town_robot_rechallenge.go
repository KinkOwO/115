package main

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

// Rechallenge replaces per-run connections while preserving the server party.
// Current-PE N9 action3 synchronously invokes manager vtable+a0 (146662ac0),
// clears cached member bindings and resets initialized+10a088. The following
// native full roster can then invoke vtable+98 (1466624a0) on a cold manager.
// N10 is deliberately absent: it marks deferred leave state, not this reset.
// Both native routines and their dispatcher calls were replayed in private
// memory; actor cache / OS boundaries were explicit fixtures (attempt 2/3).
func (w *worldSession) rebindRobotRechallenge(route []outboundPacket) ([]outboundPacket, error) {
	if w == nil || w.hub == nil || w.hub.townRobots == nil || !w.townRobotDungeonChannelAllowed() {
		return route, nil
	}
	f := w.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.parties[w.peer]
	if p == nil {
		return route, nil
	}
	if !p.dungeon || !p.entryPrepared || p.transport == nil || len(p.bots) < 1 || len(p.bots) > 3 {
		return nil, fmt.Errorf("robot retry has no prepared owned dungeon party")
	}
	head := -1
	for i, packet := range route {
		if packet.Kind == 0 && packet.ID == 27 {
			head = i
			break
		}
	}
	if head < 0 {
		return nil, fmt.Errorf("robot retry is missing the native scene reset")
	}
	previous := p.transport
	next := &townRobotTransport{ownerActor: previous.ownerActor, ownerKey: previous.ownerKey,
		send: previous.send, event: previous.event, authorityPath: previous.authorityPath}
	actors := []uint16{w.peer.actorID}
	for _, b := range p.bots {
		if err := next.add(b.peer.actorID); err != nil {
			next.close()
			return nil, err
		}
		actors = append(actors, b.peer.actorID)
	}
	endpoints, err := next.endpoints()
	var roster []byte
	if err == nil {
		roster, err = protocol.OrdinaryPartyRoster115(p.id, p.identity, actors)
	}
	if err != nil {
		next.close()
		return nil, err
	}
	gone, err := protocol.OrdinaryPartyGone115(p.id, p.identity)
	if err != nil {
		next.close()
		return nil, err
	}
	rebound := make([]outboundPacket, 0, len(route)+3)
	rebound = append(rebound, route[:head+1]...)
	rebound = append(rebound,
		outboundPacket{"robot_retry_clear_native_party", 0, 9, gone},
		outboundPacket{"robot_retry_peer_endpoints", 0, 11, endpoints},
		outboundPacket{"robot_retry_party_roster", 0, 9, roster})
	rebound = append(rebound, route[head+1:]...)
	// Closing first invalidates the old AI offer and stops reliable retries.
	// A fresh endpoint must prove bidirectional UDP again; completion is never
	// carried over. The normal post-send room/load hooks claim its new lease.
	previous.close()
	p.transport = next
	if next.event != nil {
		next.event(map[string]any{"kind": "town_robot_rechallenge_rebound", "party": p.id, "actors": actors, "attempt": "r33-rechallenge2 attempt2/3"})
	}
	return rebound, nil
}
