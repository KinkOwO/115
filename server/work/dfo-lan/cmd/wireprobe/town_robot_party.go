package main

import (
	"dfolan/internal/game/protocol"
	"fmt"
	"net"
)

// A robot party is session-only. It never grants a robot an account or a
// player-owned companion slot. Ordinary dungeon movement uses the same peers.
type townRobotParty struct {
	id            uint16
	identity      [2]byte
	owner         *lanPeer
	bots          []*townRobot
	transport     *townRobotTransport
	entryPrepared bool
	dungeon       bool
}

func (f *townRobotFleet) releaseTownPartyLocked(owner *lanPeer, notify bool) bool {
	party := f.parties[owner]
	if party == nil {
		return false
	}
	if t := f.parties[owner].transport; t != nil {
		t.close()
	}
	delete(f.parties, owner)
	if notify && owner.send != nil {
		// N10 resets connection state; N9 clears cached actor/roster bindings.
		owner.send(0, 10, []byte{0, 0})
		gone, _ := protocol.OrdinaryPartyGone115(party.id, party.identity)
		owner.send(0, 9, gone)
	}
	return true
}

func (f *townRobotFleet) releaseTownParty(owner *lanPeer, notify bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseTownPartyLocked(owner, notify)
}

func (f *townRobotFleet) pruneTownPartiesLocked() {
	for owner, party := range f.parties {
		f.hub.mu.Lock()
		_, present := f.hub.peers[owner]
		for _, bot := range party.bots {
			_, exists := f.hub.peers[bot.peer]
			// Scene visibility is transient during travel; membership is not.
			present = present && exists
		}
		f.hub.mu.Unlock()
		if !present {
			f.releaseTownPartyLocked(owner, true)
		}
	}
}

func (f *townRobotFleet) nextTownPartyIDLocked() (uint16, error) {
	for i := 0; i < 65534; i++ {
		f.nextPartyID++
		if f.nextPartyID == 0 || f.nextPartyID == 65535 {
			f.nextPartyID = 1
		}
		used := false
		for _, p := range f.parties {
			if p.id == f.nextPartyID {
				used = true
				break
			}
		}
		if !used {
			return f.nextPartyID, nil
		}
	}
	return 0, fmt.Errorf("no session party identity available")
}

func (client *gameConnection) dispatchTownRobotParty(q *clientRequest) dispatchAction {
	if client.hub == nil || client.hub.townRobots == nil {
		return dispatchNext
	}
	invite := q.frame.ID == 10 && len(q.plaintext) >= 3 && q.plaintext[2] == 0
	leave := q.frame.ID == 13
	kick := q.frame.ID == 14
	entry := q.frame.ID == 15
	connectionCheck := q.frame.ID == 2116
	if !invite && !leave && !kick && !entry && !connectionCheck {
		return dispatchNext
	}
	w := client.worldState
	if !q.verified || !client.bootstrapped || w == nil || w.peer == nil || client.selectedCharacterID == 0 || w.role.ID != client.selectedCharacterID || w.role.AccountID != w.account || w.peer.roleID != w.role.ID {
		return dispatchHandled
	}
	f := client.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	if connectionCheck {
		party := f.parties[w.peer]
		if party == nil || party.transport == nil {
			return dispatchNext
		}
		r, e := protocol.DecodePartyConnectionCheck115(q.plaintext)
		if e != nil {
			return dispatchHandled
		}
		if e = party.transport.check(r); e != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if entry {
		// CMD15 only opens selection. Preserve peers until admission succeeds.
		if p := f.parties[w.peer]; p != nil {
			p.entryPrepared = false
		}
		return dispatchNext
	}
	if leave {
		if f.parties[w.peer] == nil && w.ordinarySoloParty == nil {
			return dispatchNext
		}
		if protocol.DecodeOrdinaryPartyLeave115(q.plaintext) != nil || w.activeDungeon != nil || w.selectingDungeon {
			return dispatchHandled
		}
		if err := client.output.send(1, 13, []byte{1}); err != nil {
			return dispatchClose
		}
		f.releaseTownPartyLocked(w.peer, true)
		if gone := w.takeOrdinarySoloPartyGone(); gone != nil {
			if client.output.send(0, 10, []byte{0, 0}) != nil || client.output.send(0, 9, gone) != nil {
				return dispatchClose
			}
		}
		w.soloPartyReady = false
		client.event(map[string]any{"kind": "town_robot_party_left", "attempt": "3/3"})
		return dispatchHandled
	}
	if kick {
		return client.kickTownRobotLocked(q, f)
	}
	r, err := protocol.DecodeOrdinaryPartyInvite115(q.plaintext)
	if err != nil {
		return dispatchHandled
	}
	var bot *townRobot
	f.hub.mu.Lock()
	_, joined := f.hub.peers[w.peer]
	for _, b := range f.bots {
		_, present := f.hub.peers[b.peer]
		if joined && present && b.peer.robot && b.peer.actorID == r.Actor && visible(w.peer, b.peer) {
			bot = b
			break
		}
	}
	f.hub.mu.Unlock()
	if bot == nil {
		return dispatchNext
	} // Real-player invitation is not captured.
	reject := func(reason string) dispatchAction {
		client.event(map[string]any{"kind": "town_robot_party_rejected", "target_actor": r.Actor, "error": reason, "attempt": "3/3"})
		// ACK10 error3 has an explicit native generic error branch; body action0.
		if client.output.send(1, 10, []byte{0, 3, 0, 0}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	identity, ok := townRobotSceneIdentity(bot.peer)
	if !ok || w.characters == nil || identity != w.characters.ChannelContext || w.activeDungeon != nil || w.selectingDungeon || w.channelWorldIsolated || w.raidEntrance != nil || w.specialTowns[w.state.Position.Town] {
		return reject("ordinary public town required")
	}
	f.pruneTownPartiesLocked()
	party := f.parties[w.peer]
	for owner, p := range f.parties {
		for _, b := range p.bots {
			if b == bot && owner != w.peer {
				return reject("robot already in another party")
			}
		}
	}
	duplicate := false
	if party != nil {
		for _, b := range party.bots {
			duplicate = duplicate || b == bot
		}
		if len(party.bots) >= 3 && !duplicate {
			return reject("party full")
		}
	}
	if party == nil {
		id, e := f.nextTownPartyIDLocked()
		if e != nil {
			return reject(e.Error())
		}
		party = &townRobotParty{id: id, identity: identity, owner: w.peer}
	}
	if duplicate {
		// N9 constructs peers again. An already joined member must not restart
		// a completed connection when its context menu is used repeatedly.
		if client.output.send(1, 10, []byte{1}) != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "town_robot_party_duplicate_invite", "target_actor": r.Actor, "party_id": party.id, "attempt": "3/3"})
		return dispatchHandled
	}
	if party.transport == nil {
		host, _, e := net.SplitHostPort(client.peer)
		if e != nil || !net.ParseIP(host).IsLoopback() {
			return reject("native robot UDP requires a local session")
		}
		key, e := client.townRobotNativeKey()
		if e != nil {
			return reject(e.Error())
		}
		party.transport = &townRobotTransport{ownerActor: w.peer.actorID, ownerKey: key, send: client.output.send, event: client.event, authorityPath: robotAuthorityPath()}
	}
	if !duplicate {
		if e := party.transport.add(bot.peer.actorID); e != nil {
			if f.parties[w.peer] == nil {
				party.transport.close()
			}
			return reject(e.Error())
		}
	}
	bots := append([]*townRobot(nil), party.bots...)
	if !duplicate {
		bots = append(bots, bot)
	}
	actors := []uint16{w.peer.actorID}
	for _, b := range bots {
		actors = append(actors, b.peer.actorID)
	}
	roster, e := protocol.OrdinaryPartyRoster115(party.id, party.identity, actors)
	if e != nil {
		party.transport.close()
		return reject(e.Error())
	}
	endpoints, e := party.transport.endpoints()
	if e != nil {
		party.transport.close()
		return reject(e.Error())
	}
	if client.output.send(1, 10, []byte{1}) != nil {
		party.transport.close()
		return dispatchClose
	}
	if gone := w.takeOrdinarySoloPartyGone(); gone != nil {
		if client.output.send(0, 10, []byte{0, 0}) != nil || client.output.send(0, 9, gone) != nil {
			party.transport.close()
			return dispatchClose
		}
	}
	// Publish peer addresses before N9 calls the native peer constructor.
	if client.output.send(0, 11, endpoints) != nil {
		party.transport.close()
		return dispatchClose
	}
	if client.output.send(0, 9, roster) != nil {
		party.transport.close()
		return dispatchClose
	}
	party.bots = bots
	if f.parties == nil {
		f.parties = map[*lanPeer]*townRobotParty{}
	}
	f.parties[w.peer] = party
	if e := f.sendTownRobotVitalsLocked(party); e != nil {
		return dispatchClose
	}
	w.soloPartyReady = false // Ordinary robot roster is not a dungeon bootstrap.
	client.event(map[string]any{"kind": "town_robot_party_joined", "target_actor": r.Actor, "party_id": party.id, "actors": actors, "duplicate": duplicate, "attempt": "3/3", "synthetic_dungeon_ai": false})
	return dispatchHandled
}
