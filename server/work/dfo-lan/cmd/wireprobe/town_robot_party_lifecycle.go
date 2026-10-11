package main

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
)

type ordinarySoloPartyIdentity struct {
	id      uint16
	context [2]byte
}

func (w *worldSession) rememberOrdinarySoloParty(p []byte) {
	// Track only the ordinary bootstrap emitted in this opt-in fleet session.
	// Specialized party handlers retain their own lifecycle and never opt in.
	if w.hub == nil || w.hub.townRobots == nil || !w.townRobotDungeonChannelAllowed() || len(p) < 10 || p[8] != 0 || p[9] != 0 {
		return
	}
	w.ordinarySoloParty = &ordinarySoloPartyIdentity{binary.LittleEndian.Uint16(p[4:]), [2]byte{p[6], p[7]}}
}

func (w *worldSession) takeOrdinarySoloPartyGone() []byte {
	if w.ordinarySoloParty == nil {
		return nil
	}
	p := w.ordinarySoloParty
	w.ordinarySoloParty = nil
	w.soloPartyReady = false
	gone, _ := protocol.OrdinaryPartyGone115(p.id, p.context)
	return gone
}

func (client *gameConnection) kickTownRobotLocked(q *clientRequest, f *townRobotFleet) dispatchAction {
	w := client.worldState
	p := f.parties[w.peer]
	if p == nil {
		return dispatchNext
	}
	slot, err := protocol.DecodeOrdinaryPartyKick115(q.plaintext)
	if err != nil || int(slot) > len(p.bots) || w.activeDungeon != nil || w.selectingDungeon || p.dungeon {
		return dispatchHandled
	}
	removed := p.bots[slot-1].peer.actorID
	remaining := append([]*townRobot(nil), p.bots[:slot-1]...)
	remaining = append(remaining, p.bots[slot:]...)
	// The transport's native seat numbers are immutable. Prepare a replacement
	// for the surviving members before retiring the old roster and sockets.
	var next *townRobotTransport
	var endpoints, roster []byte
	if len(remaining) > 0 {
		next = &townRobotTransport{ownerActor: p.transport.ownerActor, ownerKey: p.transport.ownerKey, send: client.output.send, event: client.event, authorityPath: p.transport.authorityPath}
		actors := []uint16{w.peer.actorID}
		for _, b := range remaining {
			if err = next.add(b.peer.actorID); err != nil {
				next.close()
				return dispatchClose
			}
			actors = append(actors, b.peer.actorID)
		}
		endpoints, err = next.endpoints()
		if err == nil {
			roster, err = protocol.OrdinaryPartyRoster115(p.id, p.identity, actors)
		}
		if err != nil {
			next.close()
			return dispatchClose
		}
	}
	if client.output.send(1, 14, []byte{1}) != nil {
		if next != nil {
			next.close()
		}
		return dispatchClose
	}
	f.releaseTownPartyLocked(w.peer, true)
	w.soloPartyReady = false
	if next != nil {
		p.bots, p.transport, p.entryPrepared = remaining, next, false
		f.parties[w.peer] = p
		if client.output.send(0, 11, endpoints) != nil || client.output.send(0, 9, roster) != nil || f.sendTownRobotVitalsLocked(p) != nil {
			next.close()
			return dispatchClose
		}
	}
	client.event(map[string]any{"kind": "town_robot_party_kicked", "actor": removed, "slot": slot, "remaining": len(remaining)})
	return dispatchHandled
}
