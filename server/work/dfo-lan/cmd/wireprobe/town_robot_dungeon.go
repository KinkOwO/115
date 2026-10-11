package main

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

func (w *worldSession) townRobotPartySnapshot() *townRobotParty {
	if w == nil || w.hub == nil || w.peer == nil || w.hub.townRobots == nil {
		return nil
	}
	f := w.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.parties[w.peer]
	if p == nil {
		return nil
	}
	copy := *p
	copy.bots = make([]*townRobot, len(p.bots))
	f.hub.mu.Lock()
	defer f.hub.mu.Unlock()
	for i, bot := range p.bots {
		b, peer := *bot, *bot.peer
		peer.info = append([]byte(nil), peer.info...)
		peer.addition = append([]byte(nil), peer.addition...)
		b.peer = &peer
		copy.bots[i] = &b
	}
	return &copy
}
func (w *worldSession) ownsTownRobotParty(id uint16) bool {
	p := w.townRobotPartySnapshot()
	return p != nil && w.townRobotDungeonChannelAllowed() && p.id == id && len(p.bots) > 0
}

func (w *worldSession) localRobotMonsterAuthority() bool {
	p := w.townRobotPartySnapshot()
	return p != nil && len(p.bots) > 0 && w.townRobotDungeonChannelAllowed()
}

func (w *worldSession) townRobotDungeonChannelAllowed() bool {
	// Ordinary channel types come from the channel catalog (including type 22).
	// Specialized/isolated loaders retain their own member lifecycle.
	return w != nil && !w.channelWorldIsolated && !adventureEliteChannel(w.channelType)
}

func (w *worldSession) resetRobotEntryPreparation() {
	if w == nil || w.hub == nil || w.hub.townRobots == nil {
		return
	}
	f := w.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	if p := f.parties[w.peer]; p != nil {
		p.entryPrepared = false
	}
}
func (f *townRobotFleet) robotInDungeonLocked(bot *townRobot) bool {
	for _, p := range f.parties {
		if p.dungeon {
			for _, b := range p.bots {
				if b == bot {
					return true
				}
			}
		}
	}
	return false
}
func (f *townRobotFleet) enterRobotDungeonOrRelease(owner *lanPeer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.parties[owner]
	if p == nil {
		return
	}
	if !p.entryPrepared {
		f.releaseTownPartyLocked(owner, false)
		return
	}
	p.entryPrepared = false
	p.dungeon = true
	if p.transport != nil {
		p.transport.setRoom(nil)
	}
	for _, b := range p.bots {
		f.hub.retire(b.peer)
	}
}
func (w *worldSession) endRobotDungeonParty() bool {
	p := w.townRobotPartySnapshot()
	if p == nil || !p.dungeon {
		return false
	}
	f := w.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	current := f.parties[w.peer]
	if current == nil || current.id != p.id {
		return false
	}
	// Return their original town actors before ensureTownRobots prunes empty scenes.
	for _, b := range current.bots {
		f.hub.publish(b.peer, b.peer.channel, b.peer.town, b.peer.area, b.peer.x, b.peer.y, false)
	}
	f.releaseTownPartyLocked(w.peer, false)
	w.soloPartyReady = false
	return true
}

// A settlement town return is a scene transition, not a party leave. Publish
// the owner and companions under the fleet lock before lifting dungeon mode:
// the periodic town pruner must never see a half-returned, invisible roster.
func (w *worldSession) returnRobotDungeonParty() bool {
	if w == nil || w.hub == nil || w.hub.townRobots == nil || w.peer == nil || !w.townRobotDungeonChannelAllowed() {
		return false
	}
	f := w.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.parties[w.peer]
	if p == nil || !p.dungeon {
		return false
	}
	if p.transport != nil {
		p.transport.setRoomState(nil, false)
	}
	origin := w.state.Position
	f.hub.publish(w.peer, w.peer.channel, origin.Town, origin.Area, origin.X, origin.Y, w.privateArea(origin))
	for _, b := range p.bots {
		f.hub.publish(b.peer, b.peer.channel, b.peer.town, b.peer.area, b.peer.x, b.peer.y, false)
	}
	p.entryPrepared, p.dungeon = false, false
	return true
}

func (f *townRobotFleet) sendTownRobotVitalsLocked(p *townRobotParty) error {
	if p == nil || p.dungeon || p.owner.send == nil {
		return nil
	}
	var actors []uint16
	for _, b := range p.bots {
		actors = append(actors, b.peer.actorID)
	}
	body, e := protocol.PartyTownVitals115(actors)
	if e != nil {
		return e
	}
	return p.owner.send(0, 153, body)
}
func (w *worldSession) robotDungeonRoomPlan() ([]outboundPacket, error) {
	p := w.townRobotPartySnapshot()
	if p == nil || !p.dungeon {
		return nil, nil
	}
	if w.activeDungeon == nil || len(p.bots) < 1 || len(p.bots) > 3 {
		return nil, fmt.Errorf("robot dungeon party lost its room")
	}
	var plan []outboundPacket
	for i, b := range p.bots {
		state, e := protocol.UserState(b.peer.actorID, protocol.UserStateDungeon)
		if e != nil {
			return nil, e
		}
		slot := byte(i + 1)
		start, e := protocol.StartMap(protocol.StartMapState{ReuseRoom: true, Position: [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y}, Map: w.activeDungeon.Room.Map, PartyActorSlot: &slot})
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"robot_dungeon_actor_state", 0, 3, state}, outboundPacket{"robot_dungeon_actor_room", 0, 29, start})
	}
	return plan, nil
}
func (w *worldSession) commitRobotRoom() {
	p := w.townRobotPartySnapshot()
	if p == nil || !p.dungeon || p.transport == nil || w.activeDungeon == nil {
		return
	}
	room := [2]uint16{uint16(w.activeDungeon.Room.X), uint16(w.activeDungeon.Room.Y)}
	p.transport.setRoom(&room)
}
