package main

import "dfolan/internal/game/protocol"

func (client *gameConnection) dispatchTownRobotInspect(q *clientRequest) dispatchAction {
	if q.frame.ID != 8 || client.hub == nil || client.hub.townRobots == nil {
		return dispatchNext
	}
	actor, mode, err := protocol.DecodeUserInfoRequest(q.plaintext)
	if err != nil || mode != 3 {
		return dispatchNext
	}
	w := client.worldState
	if !q.verified || !client.bootstrapped || w == nil || w.peer == nil || client.selectedCharacterID == 0 || w.role.ID != client.selectedCharacterID || w.role.AccountID != w.account || w.peer.roleID != w.role.ID {
		return dispatchHandled
	}
	f := client.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.manager == nil || f.manager.characters == nil {
		return dispatchNext
	}
	var robot *townRobot
	client.hub.mu.Lock()
	if _, joined := client.hub.peers[w.peer]; joined {
		for _, b := range f.bots {
			if _, present := client.hub.peers[b.peer]; present && b.peer.robot && b.peer.actorID == actor && visible(w.peer, b.peer) {
				robot = b
				break
			}
		}
	}
	client.hub.mu.Unlock()
	if robot == nil {
		return dispatchNext
	}
	r, ok := f.manager.records[robot.recordID]
	if !ok {
		return dispatchHandled
	}
	role := r.Role
	role.WireID = actor
	// Use the identity already introduced to the client. The native lookup key
	// includes both context bytes; zeroing them selects a different object.
	identity, ok := townRobotSceneIdentity(robot.peer)
	if !ok {
		client.event(map[string]any{"kind": "town_robot_inspect_rejected", "target_actor": actor, "error": "missing scene identity"})
		return dispatchHandled
	}
	payload, err := f.manager.characters.TownRobotInfo115(role, identity)
	if err != nil {
		client.event(map[string]any{"kind": "town_robot_inspect_rejected", "target_actor": actor, "error": err.Error()})
		return dispatchHandled
	}
	if err = client.output.send(0, 2, payload); err != nil {
		return dispatchClose
	}
	client.event(map[string]any{"kind": "town_robot_inspect_sent", "target_actor": actor, "robot_id": r.ID, "revision": r.Revision, "attempt": "2/3"})
	return dispatchHandled
}

// townRobotSceneIdentity reads the validated mode-0 introduction envelope,
// rather than substituting the manager's default channel or a zero context.
func townRobotSceneIdentity(p *lanPeer) ([2]byte, bool) {
	if p == nil || !p.robot || len(p.info) < 5 || p.info[0] != 0 || p.info[1] != 1 || p.info[2] != 0 {
		return [2]byte{}, false
	}
	return [2]byte{p.info[3], p.info[4]}, true
}
