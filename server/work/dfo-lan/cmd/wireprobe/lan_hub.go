package main

import (
	"dfolan/internal/game/protocol"
	"sort"
	"sync"
)

// Multiplayer on this build is a hub of player connections that share a scene.
//
// An actor exists for another client only when both are on the same channel
// and the same source area. Seria's room is a personal instance: the client
// shows a single actor there even when the town outside is crowded, so such an
// area never publishes anyone.
//
// The hub only ever serializes packets this build already verifies against the
// client: NOTI 2 (actor info, name/profession/level/worn appearance), NOTI 23
// (a known actor's placement) and NOTI 24 (the area's actor list). Nothing here
// invents a wire layout for a notification that has not been recovered.
//
// Leaving a scene is announced with NOTI 23, not with a shortened NOTI 24: the
// client treats an area list as "place these actors" and keeps drawing anyone
// it is not explicitly told about, so re-sending a shorter list leaves the
// departed actor standing where it was. NOTI 23 is the placement update the
// client already consumes for exactly this purpose.

// lanPeer is one player connection as seen by the others.
type lanPeer struct {
	roleID  int64
	actorID uint16
	channel uint32

	// town, area, x, y, flags, private and placed are rewritten under the hub
	// lock and must not be read outside it.
	town    uint32
	area    uint32
	x, y    uint16
	flags   [3]byte
	private bool
	placed  bool

	// motion and speed are the last values this actor reported with CMD 35. They
	// live on the peer rather than only on the reporting session, so a player
	// entering the scene later can be handed the pose everyone is actually in
	// instead of seeing them stand at the default facing.
	motion byte
	speed  uint16

	// info and addition are the two NOTI 2 payloads that introduce this actor:
	// minimum information (mode 0) and its addition (mode 1). The client reads an
	// actor it does not own through its own pair of readers, so both are needed
	// before the other side builds the actor.
	info     []byte
	addition []byte

	// send writes one notification to this peer's own connection. It must be
	// safe to call from another player's goroutine.
	send func(kind byte, id uint16, payload []byte) error
}

// visible reports whether two actors share a scene. Both directions have to
// hold: a player inside a private instance must not see the town, and the town
// must not see that player.
func visible(a, b *lanPeer) bool {
	return a != nil && b != nil && a != b && a.info != nil && b.info != nil &&
		a.placed && b.placed && !a.private && !b.private &&
		a.channel == b.channel && a.town == b.town && a.area == b.area
}

// peerPush is one notification that has to reach one specific connection.
// Payloads are serialized under the lock and written outside it, so a slow
// client cannot stall every other player.
type peerPush struct {
	to      *lanPeer
	id      uint16
	payload []byte
}

type lanHub struct {
	mu    sync.Mutex
	peers map[*lanPeer]struct{}
}

func newLanHub() *lanHub {
	return &lanHub{peers: map[*lanPeer]struct{}{}}
}

func (h *lanHub) add(p *lanPeer) {
	if h == nil || p == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.peers[p] = struct{}{}
}

// publish places an actor in a scene, tells the scene it left that the actor
// has moved, and returns the peers that now share its new scene.
func (h *lanHub) publish(p *lanPeer, channel uint32, town, area uint32, x, y uint16, private bool) []*lanPeer {
	if h == nil || p == nil {
		return nil
	}
	h.mu.Lock()
	moved := p.placed && (p.channel != channel || p.town != town || p.area != area || p.private != private)
	var stale []*lanPeer
	if moved {
		stale = h.sharedLocked(p)
	}
	p.channel, p.town, p.area, p.x, p.y, p.private, p.placed = channel, town, area, x, y, private, true
	var pushes []peerPush
	if moved {
		pushes = append(pushes, h.movedLocked(stale, p.actorID, town, area, x, y, p.flags)...)
	}
	joined := h.sharedLocked(p)
	h.mu.Unlock()
	h.flush(pushes)
	return joined
}

// move records a new position inside the scene the actor is already in and
// returns the peers that should see it.
func (h *lanHub) move(p *lanPeer, x, y uint16, motion byte, speed uint16) []*lanPeer {
	if h == nil || p == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !p.placed {
		return nil
	}
	p.x, p.y, p.motion, p.speed = x, y, motion, speed
	return h.sharedLocked(p)
}

// shared lists the actors that can currently see p.
func (h *lanHub) shared(p *lanPeer) []*lanPeer {
	if h == nil || p == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sharedLocked(p)
}

// peerPose is where one peer stands and which way it faces, copied out under
// the hub lock so a caller can build notifications without holding it.
type peerPose struct {
	actor  uint16
	x, y   uint16
	motion byte
	speed  uint16
}

// posesOf copies the placement of each of these peers, read under the hub lock
// and in the same order as the input, so caller index i always matches
// peers[i]. The area list carries coordinates only, so a player entering a
// scene has no way of knowing where anyone stands or faces without this.
func (h *lanHub) posesOf(peers []*lanPeer) []peerPose {
	if h == nil || len(peers) == 0 {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]peerPose, 0, len(peers))
	for _, o := range peers {
		out = append(out, peerPose{actor: o.actorID, x: o.x, y: o.y, motion: o.motion, speed: o.speed})
	}
	return out
}

// retire unpublishes an actor, for instance while it is inside a dungeon, and
// tells the scene it left. The peer stays registered so it can be published
// again when the actor returns to a shared area.
//
// The departure is announced with NOTI 6, which deletes an actor outright.
// Neither NOTI 24 nor NOTI 23 removes one - the client only places actors -
// which is why a departure used to leave the actor standing in place.
func (h *lanHub) retire(p *lanPeer) {
	if h == nil || p == nil {
		return
	}
	h.mu.Lock()
	if !p.placed {
		h.mu.Unlock()
		return
	}
	leaving := h.sharedLocked(p)
	p.placed = false
	pushes := h.leaveLocked(leaving, p.actorID)
	h.mu.Unlock()
	h.flush(pushes)
}

// depart removes an actor for good, as on disconnect or a channel switch, and
// tells the scene it left the same way retire does.
func (h *lanHub) depart(p *lanPeer) {
	if h == nil || p == nil {
		return
	}
	h.mu.Lock()
	var leaving []*lanPeer
	if p.placed {
		leaving = h.sharedLocked(p)
	}
	p.placed = false
	delete(h.peers, p)
	pushes := h.leaveLocked(leaving, p.actorID)
	h.mu.Unlock()
	h.flush(pushes)
}

// movedLocked reports that an actor is now standing somewhere else. This is the
// notification the client uses to drop the actor from its old scene.
func (h *lanHub) movedLocked(peers []*lanPeer, actor uint16, town, area uint32, x, y uint16, flags [3]byte) []peerPush {
	var pushes []peerPush
	for _, o := range peers {
		if o.send == nil {
			continue
		}
		payload, e := protocol.UserArea(town, area, protocol.AreaUser{ActorServerID: actor, X: x, Y: y, Flags: flags})
		if e != nil {
			continue
		}
		pushes = append(pushes, peerPush{to: o, id: 23, payload: payload})
	}
	return pushes
}

// leaveLocked tells each of these peers to drop the actor entirely.
func (h *lanHub) leaveLocked(peers []*lanPeer, actor uint16) []peerPush {
	var pushes []peerPush
	for _, o := range peers {
		if o.send == nil {
			continue
		}
		payload, e := protocol.UserLeave(actor)
		if e != nil {
			continue
		}
		pushes = append(pushes, peerPush{to: o, id: 6, payload: payload})
	}
	return pushes
}

// NOTI 24 is deliberately never re-sent at runtime. The client reads an area
// list as "this is the scene, build it", so pushing a refreshed list whenever
// somebody enters or leaves rebuilds every actor and resets all of their
// poses and facings - which is exactly what a bystander sees when a newcomer
// arrives. Actors appearing, moving and leaving are announced with NOTI 2,
// NOTI 23 and NOTI 6 instead, and only a client loading a scene for the first
// time ever receives NOTI 24.

func (h *lanHub) flush(pushes []peerPush) {
	for _, push := range pushes {
		push.to.send(0, push.id, push.payload)
	}
}

// sharedLocked lists the actors that can currently see p. The caller holds the
// lock.
func (h *lanHub) sharedLocked(p *lanPeer) []*lanPeer {
	if p.private || !p.placed {
		return nil
	}
	var out []*lanPeer
	for o := range h.peers {
		if visible(p, o) {
			out = append(out, o)
		}
	}
	return out
}

// roster returns every actor a NOTI 24 area list may carry for p, including p
// itself, ordered by actor id so the same scene always serializes identically.
func (h *lanHub) roster(p *lanPeer) []*lanPeer {
	if h == nil || p == nil {
		return []*lanPeer{p}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rosterLocked(p)
}

// rosterLocked is the same list without taking the lock. Two connections on the
// same character collapse into one row: the client's area list rejects a
// duplicated actor id.
func (h *lanHub) rosterLocked(p *lanPeer) []*lanPeer {
	out := []*lanPeer{p}
	seen := map[uint16]bool{p.actorID: true}
	for _, o := range h.sharedLocked(p) {
		if seen[o.actorID] {
			continue
		}
		seen[o.actorID] = true
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].actorID < out[j].actorID })
	return out
}

func areaUsersPayload(town, area uint32, roster []*lanPeer) ([]byte, error) {
	users := make([]protocol.AreaUser, 0, len(roster))
	for _, o := range roster {
		users = append(users, protocol.AreaUser{ActorServerID: o.actorID, X: o.x, Y: o.y, Flags: o.flags})
	}
	return protocol.AreaUsers(town, area, users)
}
