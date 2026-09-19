package main

import (
	"dfolan/internal/storage"
	"encoding/binary"
	"testing"
)

type capture struct {
	id      uint16
	payload []byte
}

// allPeer captures every notification sent to it, whatever its id.
func allPeer(role int64, actor uint16, out *[]capture) *lanPeer {
	p := newTestPeer(role, actor)
	p.send = func(kind byte, id uint16, payload []byte) error {
		*out = append(*out, capture{id: id, payload: payload})
		return nil
	}
	return p
}

func findCapture(all []capture, id uint16) []byte {
	for _, c := range all {
		if c.id == id {
			return c.payload
		}
	}
	return nil
}

func countCapture(all []capture, id uint16) int {
	n := 0
	for _, c := range all {
		if c.id == id {
			n++
		}
	}
	return n
}

func areaActorCount(t *testing.T, payload []byte) int {
	t.Helper()
	if len(payload) < 10 {
		t.Fatalf("area list too short: %d", len(payload))
	}
	return int(binary.LittleEndian.Uint16(payload[8:10]))
}

func newTestPeer(role int64, actor uint16) *lanPeer {
	return &lanPeer{roleID: role, actorID: actor, info: []byte{byte(actor)}, addition: []byte{byte(actor), 0xAD}}
}

// The whole multiplayer rule in one place: same channel and same source area,
// never inside a private instance, and never before the actor is published.
func TestHubVisibility(t *testing.T) {
	h := newLanHub()
	townA := newTestPeer(1, 3)
	townB := newTestPeer(2, 4)
	other := newTestPeer(3, 5)
	h.add(townA)
	h.add(townB)
	h.add(other)
	h.publish(townA, 1, 38, 0, 561, 234, false)
	h.publish(townB, 1, 38, 0, 700, 240, false)
	h.publish(other, 1, 38, 3, 100, 100, false)

	if got := len(h.shared(townA)); got != 1 {
		t.Fatalf("same-area peers: got %d want 1", got)
	}
	if got := h.shared(townA)[0]; got != townB {
		t.Fatalf("wrong peer returned: %+v", got)
	}
	if got := len(h.shared(other)); got != 0 {
		t.Fatalf("different area must be invisible: got %d", got)
	}

	// A second channel is a second world.
	channel2 := newTestPeer(4, 6)
	h.add(channel2)
	h.publish(channel2, 2, 38, 0, 561, 234, false)
	if got := len(h.shared(townA)); got != 1 {
		t.Fatalf("other channel must be invisible: got %d", got)
	}
}

// Seria's room is a personal instance: neither side may see the other.
func TestHubPrivateAreaIsolation(t *testing.T) {
	h := newLanHub()
	town := newTestPeer(1, 3)
	room := newTestPeer(2, 4)
	h.add(town)
	h.add(room)
	h.publish(town, 1, 38, 0, 561, 234, false)
	h.publish(room, 1, 38, 1, 100, 100, true)

	if got := len(h.shared(town)); got != 0 {
		t.Fatalf("a private instance must not see the town: got %d", got)
	}
	if got := len(h.shared(room)); got != 0 {
		t.Fatalf("a private instance must not be seen: got %d", got)
	}
	if roster := h.roster(room); len(roster) != 1 || roster[0] != room {
		t.Fatalf("private roster must hold only the local actor: %+v", roster)
	}
}

// Outside the room, the very same peer becomes visible again.
func TestHubLeavingPrivateAreaRepublishes(t *testing.T) {
	h := newLanHub()
	town := newTestPeer(1, 3)
	room := newTestPeer(2, 4)
	h.add(town)
	h.add(room)
	h.publish(town, 1, 38, 0, 561, 234, false)
	h.publish(room, 1, 38, 1, 100, 100, true)
	h.publish(room, 1, 38, 0, 600, 240, false)
	if got := len(h.shared(town)); got != 1 {
		t.Fatalf("leaving the room must publish again: got %d", got)
	}
}

// The client's area list rejects a duplicated actor id, so two connections on
// the same character have to collapse into one row.
func TestHubRosterDeduplicatesActor(t *testing.T) {
	h := newLanHub()
	first := newTestPeer(1, 3)
	second := newTestPeer(2, 3)
	h.add(first)
	h.add(second)
	h.publish(first, 1, 38, 0, 561, 234, false)
	h.publish(second, 1, 38, 0, 700, 240, false)
	if roster := h.roster(first); len(roster) != 1 {
		t.Fatalf("duplicate actor id must collapse: %+v", roster)
	}
	payload, err := areaUsersPayload(38, 0, h.roster(first))
	if err != nil {
		t.Fatalf("area list for a duplicated actor must still encode: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("empty area list")
	}
}

// An actor that has not entered a world yet must not be published.
func TestHubUnplacedPeerStaysHidden(t *testing.T) {
	h := newLanHub()
	town := newTestPeer(1, 3)
	joining := newTestPeer(2, 4)
	h.add(town)
	h.add(joining)
	h.publish(town, 1, 38, 0, 561, 234, false)
	if got := len(h.shared(town)); got != 0 {
		t.Fatalf("unplaced peer must stay hidden: got %d", got)
	}
}

// Movement only reaches the scene the actor is actually standing in.
func TestHubMoveReachesSameAreaOnly(t *testing.T) {
	h := newLanHub()
	mover := newTestPeer(1, 3)
	near := newTestPeer(2, 4)
	far := newTestPeer(3, 5)
	h.add(mover)
	h.add(near)
	h.add(far)
	h.publish(mover, 1, 38, 0, 561, 234, false)
	h.publish(near, 1, 38, 0, 700, 240, false)
	h.publish(far, 1, 38, 3, 100, 100, false)
	beforeX, beforeY := near.x, near.y
	got := h.move(mover, 600, 250, 0, 4)
	if len(got) != 1 || got[0] != near {
		t.Fatalf("move must reach exactly the same-area peer: %+v", got)
	}
	if mover.x != 600 || mover.y != 250 {
		t.Fatal("move must record the mover's new position")
	}
	if near.x != beforeX || near.y != beforeY {
		t.Fatal("move must not rewrite another actor's position")
	}
}

// A pure position update inside one scene must not be mistaken for a move.
func TestHubMoveDoesNotSendUserArea(t *testing.T) {
	h := newLanHub()
	var got []capture
	stayed := allPeer(1, 3, &got)
	mover := newTestPeer(2, 4)
	h.add(stayed)
	h.add(mover)
	h.publish(stayed, 1, 38, 0, 561, 234, false)
	h.publish(mover, 1, 38, 0, 700, 240, false)
	got = nil
	h.move(mover, 640, 250, 1, 2)
	if findCapture(got, 23) != nil {
		t.Fatal("a same-scene move is broadcast by the caller, not by publish")
	}
}

// The reported bug: whenever somebody entered the scene, every other player's
// pose and facing was reset. The cause was re-sending NOTI 24 at runtime - the
// client reads an area list as "this is the scene, build it" and rebuilds every
// actor in it. Presence changes must use NOTI 2, NOTI 23 and NOTI 6 only.
func TestRuntimeNeverResendsAreaList(t *testing.T) {
	h := newLanHub()
	var got []capture
	early := allPeer(1, 3, &got)
	h.add(early)
	h.publish(early, 0, 38, 0, 561, 234, false)

	// Somebody arrives.
	late := newTestPeer(2, 4)
	h.add(late)
	session := &worldSession{
		hub:   h,
		peer:  late,
		role:  storage.Character{ID: 2, WireID: 4},
		state: storage.WorldState{Position: storage.WorldPosition{Town: 38, Area: 0, X: 700, Y: 240}},
	}
	session.enterArea()
	got = nil
	if err := session.announceSelf(func(map[string]any) {}); err != nil {
		t.Fatal(err)
	}
	if n := countCapture(got, 24); n != 0 {
		t.Fatalf("an arrival must not make the scene rebuild: %d area lists sent", n)
	}
	if findCapture(got, 2) == nil {
		t.Fatal("an arrival must still be introduced with NOTI 2")
	}
	if findCapture(got, 23) == nil {
		t.Fatal("an arrival must still be placed with NOTI 23")
	}

	// Somebody leaves.
	got = nil
	h.depart(late)
	if n := countCapture(got, 24); n != 0 {
		t.Fatalf("a departure must not make the scene rebuild: %d area lists sent", n)
	}
	if findCapture(got, 6) == nil {
		t.Fatal("a departure must remove the actor with NOTI 6")
	}

	// Somebody changes area.
	h.add(late)
	h.publish(late, 0, 38, 0, 700, 240, false)
	got = nil
	h.publish(late, 0, 38, 3, 100, 100, false)
	if n := countCapture(got, 24); n != 0 {
		t.Fatalf("an area change must not make the scene rebuild: %d area lists sent", n)
	}
	if findCapture(got, 23) == nil {
		t.Fatal("an area change must move the actor with NOTI 23")
	}
}

// The newcomer learns who is already here, and tells them about itself. Only
// NOTI 2 and NOTI 23 may be used: an area list would rebuild the scene.
func TestAnnouncePeersIntroducesSelfToOthers(t *testing.T) {
	h := newLanHub()
	var earlyGot []capture
	early := allPeer(1, 3, &earlyGot)
	h.add(early)
	h.publish(early, 0, 38, 0, 561, 234, false)

	late := newTestPeer(2, 4)
	h.add(late)
	session := &worldSession{
		hub:   h,
		peer:  late,
		role:  storage.Character{ID: 2, WireID: 4},
		state: storage.WorldState{Position: storage.WorldPosition{Town: 38, Area: 0, X: 700, Y: 240}},
	}
	session.enterArea()
	if len(session.joinedPeers) != 1 || session.joinedPeers[0] != early {
		t.Fatalf("the newcomer must find the actor already in the scene: %+v", session.joinedPeers)
	}

	earlyGot = nil
	var selfGot []capture
	self := func(kind byte, id uint16, payload []byte) error {
		selfGot = append(selfGot, capture{id: id, payload: payload})
		return nil
	}
	if err := session.introducePeers(self); err != nil {
		t.Fatal(err)
	}
	if err := session.announceSelf(func(map[string]any) {}); err != nil {
		t.Fatal(err)
	}

	// The newcomer is told about the actor already here, through both NOTI 2
	// payloads: the minimum information and its addition. The client reads an
	// actor it does not own through its own pair of readers and only enables
	// interaction once both have arrived.
	if findCapture(selfGot, 2) == nil {
		t.Fatal("the newcomer was never told about the actor already in the scene")
	}
	if got := countCapture(selfGot, 2); got < 2 {
		t.Fatalf("an actor needs both its minimum info and its addition, got %d", got)
	}
	// Neither NOTI 3 nor NOTI 22 may be sent while introducing a peer. Both are
	// read by the client against an actor that must already be fully alive, and
	// delivering them during the entry sequence crashes the joining client.
	// The pose still reaches other players through broadcastMove on NOTI 22.
	// And that actor learns about the newcomer, with an explicit placement: its
	// scene is already loaded, so only NOTI 23 can put the newcomer on screen.
	info := findCapture(earlyGot, 2)
	if info == nil {
		t.Fatal("the actor already in the scene was never told about the newcomer")
	}
	if string(info) != string(late.info) {
		t.Fatal("the wrong actor info was pushed to the waiting player")
	}
	if findCapture(earlyGot, 23) == nil {
		t.Fatal("the waiting player was never told where the newcomer stands")
	}
	if n := countCapture(earlyGot, 24); n != 0 {
		t.Fatalf("telling the waiting player must not rebuild its scene: %d lists", n)
	}
}

// A player who leaves for good - a disconnect, a channel switch or a dungeon
// run - is removed with NOTI 6. Nothing else deletes an actor: a shorter area
// list is ignored and leaves it standing there.
func TestLeavingForGoodRemovesActor(t *testing.T) {
	cases := []struct {
		name string
		call func(*lanHub, *lanPeer)
	}{
		{"depart", func(h *lanHub, p *lanPeer) { h.depart(p) }},
		{"retire", func(h *lanHub, p *lanPeer) { h.retire(p) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newLanHub()
			var got []capture
			watcher := allPeer(1, 3, &got)
			leaver := newTestPeer(2, 4)
			h.add(watcher)
			h.add(leaver)
			h.publish(watcher, 0, 38, 0, 561, 234, false)
			h.publish(leaver, 0, 38, 0, 700, 240, false)

			got = nil
			tc.call(h, leaver)

			leave := findCapture(got, 6)
			if leave == nil {
				t.Fatal("leaving must send NOTI 6: nothing else removes an actor")
			}
			if len(leave) != 2 {
				t.Fatalf("NOTI 6 carries one u16 actor, got %d bytes", len(leave))
			}
			if actor := binary.LittleEndian.Uint16(leave); actor != 4 {
				t.Fatalf("wrong actor in the departure notice: %d", actor)
			}
			if n := countCapture(got, 24); n != 0 {
				t.Fatalf("a departure must not rebuild the scene: %d lists", n)
			}
			if roster := h.roster(watcher); len(roster) != 1 || roster[0] != watcher {
				t.Fatalf("departed actor still registered: %+v", roster)
			}
		})
	}
}

// A player who changed area is moved out of the scene they came from: NOTI 23
// carries the destination, which is what makes the old scene drop them.
func TestHubLeavingAreaMovesActorInOldScene(t *testing.T) {
	h := newLanHub()
	var stayedGot []capture
	stayed := allPeer(1, 3, &stayedGot)
	mover := newTestPeer(2, 4)
	h.add(stayed)
	h.add(mover)
	h.publish(stayed, 1, 38, 0, 561, 234, false)
	h.publish(mover, 1, 38, 0, 700, 240, false)

	stayedGot = nil
	h.publish(mover, 1, 38, 2, 1085, 249, false)

	move := findCapture(stayedGot, 23)
	if move == nil {
		t.Fatal("the scene left behind must be told where the actor went")
	}
	// actor u16 + town u32 + area u32 + x u16 + y u16 + two flag bytes.
	if len(move) != 16 {
		t.Fatalf("unexpected NOTI 23 length: %d", len(move))
	}
	if actor := binary.LittleEndian.Uint16(move); actor != 4 {
		t.Fatalf("wrong actor in the move notice: %d", actor)
	}
	if town := binary.LittleEndian.Uint32(move[2:]); town != 38 {
		t.Fatalf("wrong town in the move notice: %d", town)
	}
	if area := binary.LittleEndian.Uint32(move[6:]); area != 2 {
		t.Fatalf("wrong destination area in the move notice: %d", area)
	}
	if x := binary.LittleEndian.Uint16(move[10:]); x != 1085 {
		t.Fatalf("wrong x in the move notice: %d", x)
	}
	if n := countCapture(stayedGot, 24); n != 0 {
		t.Fatalf("an area change must not rebuild the scene left behind: %d lists", n)
	}
	if roster := h.roster(mover); len(roster) != 1 || roster[0] != mover {
		t.Fatalf("the movers own scene must still hold one actor: %+v", roster)
	}
}

// The newcomer's own entry has to introduce the other players before the area
// list that places them. A list that arrives first is dropped, and the newcomer
// then sees nobody until somebody else happens to move.
func TestEntryIntroducesPeersBeforeAreaList(t *testing.T) {
	p := entryPayloads{Basic: []byte{1}, Area: []byte{2, 3}, Peers: [][]byte{{9}, {8}}}
	info, list := -1, -1
	for i, pk := range p.packets() {
		switch pk.Name {
		case "entry_peer_info_sent":
			info = i
		case "town_entry_probe_sent":
			list = i
		}
	}
	if info < 0 {
		t.Fatal("the entry sequence carries no peer info")
	}
	if list < 0 {
		t.Fatal("the entry sequence carries no area list")
	}
	if info > list {
		t.Fatal("peer info must be sent before the area list that places them")
	}
}
