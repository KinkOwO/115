package main

import "time"

// The controlling player's C39 carries the actual last hitter, including its
// ordinary synthetic teammates (captured FFFE). Attribute rewards to the owner
// only while that actor belongs to this loaded room's active native controller.
// Unknown killers still reach Session.ConfirmDeath unchanged and are refused.
func (w *worldSession) townRobotCombatKiller(killer uint16) uint16 {
	if killer == w.role.WireID || killer == 65535 || killer == 0 || w.hub == nil || w.hub.townRobots == nil || w.peer == nil || w.activeDungeon == nil || !w.activeDungeon.Loaded {
		return killer
	}
	f := w.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.parties[w.peer]
	if p == nil || p.owner != w.peer || !p.dungeon || p.transport == nil {
		return killer
	}
	tr := p.transport
	tr.mu.Lock()
	defer tr.mu.Unlock()
	age := time.Since(tr.nativeAI.lastAck)
	room := w.activeDungeon.Room
	if tr.closed || tr.roomLoading || !tr.completed || tr.room == nil || tr.ownerActor != w.role.WireID || *tr.room != [2]uint16{uint16(room.X), uint16(room.Y)} || tr.nativeAI.generation == 0 || tr.nativeAI.phase != 2 || age < 0 || age > 2500*time.Millisecond {
		return killer
	}
	for _, bot := range p.bots {
		if bot == nil || bot.peer == nil || !bot.peer.robot || bot.peer.actorID != killer {
			continue
		}
		for _, peer := range tr.peers {
			if peer != nil && peer.actor == killer {
				return w.role.WireID
			}
		}
	}
	return killer
}
