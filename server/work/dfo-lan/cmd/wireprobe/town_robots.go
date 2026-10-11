package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Synthetic actors retain independent town behavior and robot-owned state.
// They never impersonate accounts or earn dungeon rewards.
type townRobot struct {
	recordID    int
	peer        *lanPeer
	rect        [4]int32
	step        uint32
	random      uint32
	idleUntil   uint64
	destination [2]int32
	walking     bool
	announced   bool
	suspended   bool
	facing      byte
	stride      float64
}
type townRobotFleet struct {
	parties     map[*lanPeer]*townRobotParty
	nextPartyID uint16
	manager     *townRobotManager
	mu          sync.Mutex
	hub         *lanHub
	target      int
	bots        []*townRobot
	stop        chan struct{}
	done        chan struct{}
	once        sync.Once
	clock       uint64
}

func townRobotCount(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 || n > 100 {
		return 0, fmt.Errorf("DFO_TOWN_ROBOTS must be an integer from 0 to 100")
	}
	return n, nil
}

func startTownRobots(h *lanHub) (func(), error) {
	n, err := townRobotCount(os.Getenv("DFO_TOWN_ROBOTS"))
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return func() {}, nil
	}
	f := &townRobotFleet{hub: h, target: n, stop: make(chan struct{}), done: make(chan struct{})}
	if err := f.startManagement(); err != nil {
		return nil, err
	}
	h.townRobots = f
	go func() {
		defer close(f.done)
		t := time.NewTicker(200 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-f.stop:
				return
			case <-t.C:
				f.tick()
			}
		}
	}()
	log.Printf("town robots ready: target=%d player_persistence=false managed_inventory=%t", n, f.manager != nil)
	return f.close, nil
}

func (f *townRobotFleet) close() {
	f.once.Do(func() {
		close(f.stop)
		<-f.done
		if f.manager != nil && f.manager.server != nil {
			f.manager.server.Close()
			os.Remove(f.manager.accessFile)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		for owner := range f.parties {
			f.releaseTownPartyLocked(owner, false)
		}
		for _, b := range f.bots {
			f.hub.depart(b.peer)
		}
		f.bots = nil
	})
}

// The hub lock is held. Also inspect unplaced real peers: their entry is in
// flight and their scene identity must never be assigned to a temporary actor.
func (h *lanHub) actorUsedByPeerLocked(actor uint16) bool {
	for p := range h.peers {
		if p.actorID == actor {
			return true
		}
	}
	return false
}

func (h *lanHub) registerTownRobot(p *lanPeer) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.actorUsedByPeerLocked(p.actorID) {
		return false
	}
	h.peers[p] = struct{}{}
	return true
}

func (f *townRobotFleet) alive(b *townRobot) bool {
	for _, party := range f.parties {
		for _, member := range party.bots {
			if member == b {
				f.hub.mu.Lock()
				_, present := f.hub.peers[party.owner]
				f.hub.mu.Unlock()
				if present {
					return true
				}
			}
		}
	}
	f.hub.mu.Lock()
	defer f.hub.mu.Unlock()
	if _, ok := f.hub.peers[b.peer]; !ok {
		return false
	}
	for p := range f.hub.peers {
		if !p.robot && visible(p, b.peer) {
			return true
		}
	}
	return false
}

func (f *townRobotFleet) prune() {
	keep := f.bots[:0]
	for _, b := range f.bots {
		if f.alive(b) {
			keep = append(keep, b)
		} else {
			f.hub.depart(b.peer)
		}
	}
	f.bots = keep
	f.pruneTownPartiesLocked()
}

// Geometry is a source walkable rectangle. Clipping to the signed 16-bit wire
// coordinate range preserves negative coordinates without inventing map bounds.
func robotRectangle(a catalog.WorldArea) ([4]int32, bool) {
	for _, r := range a.Walkable {
		x0, y0 := int64(r[0]), int64(r[1])
		x1, y1 := x0+int64(r[2]), y0+int64(r[3])
		if r[2] <= 0 || r[3] <= 0 {
			continue
		}
		if x0 < -32768 {
			x0 = -32768
		}
		if y0 < -32768 {
			y0 = -32768
		}
		if x1 > 32767 {
			x1 = 32767
		}
		if y1 > 32767 {
			y1 = 32767
		}
		if x1 > x0 && y1 > y0 {
			return [4]int32{int32(x0), int32(y0), int32(x1 - x0), int32(y1 - y0)}, true
		}
	}
	return [4]int32{}, false
}

func robotPoint(r [4]int32, seed uint32) (uint16, uint16) {
	x := int64(r[0]) + int64((seed*73+17)%uint32(r[2]+1))
	y := int64(r[1]) + int64((seed*31+11)%uint32(r[3]+1))
	return uint16(int16(x)), uint16(int16(y))
}

func (w *worldSession) ensureTownRobots() {
	if w == nil || w.hub == nil || w.hub.townRobots == nil || w.peer == nil || w.characters == nil || w.service == nil || w.channelWorldIsolated || adventureEliteChannel(w.channelType) || w.privateArea(w.state.Position) {
		return
	}
	// enterArea publishes town placement before calling us. On a dungeon
	// return the old activeDungeon remains set until the ACK is sent, so use
	// actual scene presence rather than that pending lifecycle pointer.
	w.hub.mu.Lock()
	placed := w.peer.placed
	w.hub.mu.Unlock()
	if !placed {
		return
	}
	a, ok := w.service.Catalog.Areas[catalog.AreaKey(w.state.Position.Town, w.state.Position.Area)]
	if !ok || a.SeriaReturnWarp || a.MinimumLevel > 255 {
		return
	}
	r, ok := robotRectangle(a)
	if !ok {
		return
	}
	f := w.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.stop:
		return
	default:
	}
	// Keep this player's already-existing scene alive while pruning empty ones.
	f.prune()
	jobs := make([]int, 0, len(w.characters.Catalog.Professions))
	for job := range w.characters.Catalog.Professions {
		jobs = append(jobs, int(job))
	}
	sort.Ints(jobs)
	if len(jobs) == 0 {
		return
	}
	projection := *w.characters
	projection.Store = nil
	projection.ProfileFrames = nil
	projection.Rewards = nil
	level := byte(a.MinimumLevel)
	if level == 0 {
		level = 1
	}
	created := 0
	residents := robotResidentIDs(w.service.Catalog, f.target, a.Town, a.Area)
	for _, recordID := range residents {
		alreadyPresent := false
		for _, bot := range f.bots {
			if bot.recordID == recordID {
				alreadyPresent = true
				break
			}
		}
		if alreadyPresent {
			continue
		}
		actor := uint16(65534)
		for ; actor > 0; actor-- {
			w.hub.mu.Lock()
			used := w.hub.actorUsedByPeerLocked(actor)
			w.hub.mu.Unlock()
			if !used {
				break
			}
		}
		if actor == 0 {
			break
		}
		job := byte(jobs[(f.target-recordID)%len(jobs)])
		role, err := projection.TownDisplayCharacter(actor, fmt.Sprintf("旅人%04d", recordID), job, level, w.progression)
		if err == nil {
			role, err = f.managedRole(w, recordID, role)
		}
		if err != nil {
			log.Printf("town robot source rejected: job=%d error=%v", job, err)
			break
		}
		basic, err := projection.EntryBasicProbe(role, w.characters.ChannelContext)
		if err != nil {
			log.Printf("town robot basic rejected: %v", err)
			break
		}
		addition, err := projection.TownRobotEntryAddition(role)
		if err != nil {
			log.Printf("town robot detail rejected: %v", err)
			break
		}
		x, y := robotPoint(r, uint32(actor))
		p := &lanPeer{roleID: role.ID, actorID: actor, channel: w.peer.channel, info: basic, addition: addition, robot: true}
		if !w.hub.registerTownRobot(p) {
			continue
		}
		others := w.hub.publish(p, p.channel, a.Town, a.Area, x, y, false)
		placement, err := protocol.UserArea(a.Town, a.Area, protocol.AreaUser{ActorServerID: actor, X: x, Y: y})
		if err != nil {
			w.hub.depart(p)
			break
		}
		for _, other := range others {
			// The entering client receives these through its ordinary entry plan.
			if other == w.peer || other.send == nil {
				continue
			}
			if other.send(0, 2, basic) == nil && other.send(0, 2, addition) == nil {
				other.send(0, 23, placement)
			}
		}
		f.bots = append(f.bots, &townRobot{peer: p, rect: r, recordID: recordID})
		created++
	}
	if created > 0 {
		log.Printf("town robots populated: channel=%d town=%d area=%d created=%d total=%d", w.peer.channel, a.Town, a.Area, created, len(f.bots))
	}
}

// N22 uses the native town position report: low bit is facing; bit 2 is
// stationary (the current client reports 4/5 when releasing movement).
// Only destination selection is synthetic; positions stay inside PVF geometry.
func (b *townRobot) choose(n uint32) uint32 {
	if b.random == 0 {
		b.random = uint32(b.peer.actorID)*747796405 + uint32(b.recordID)*2891336453 + 1
	}
	b.random ^= b.random << 13
	b.random ^= b.random >> 17
	b.random ^= b.random << 5
	return b.random % n
}
func robotBound(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func (f *townRobotFleet) publishTownMotion(b *townRobot, x, y int32, motion byte) {
	others := f.hub.move(b.peer, uint16(int16(x)), uint16(int16(y)), motion, 100)
	body, err := protocol.UserPosition(b.peer.actorID, uint16(int16(x)), uint16(int16(y)), motion, 100)
	if err != nil {
		return
	}
	for _, p := range others {
		if p.send != nil {
			_ = p.send(0, 22, body)
		}
	}
}
func (f *townRobotFleet) tick() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prune()
	f.clock++
	// Preserve the existing health heartbeat cadence rather than multiplying it.
	if f.clock%15 == 0 {
		for _, party := range f.parties {
			if !party.dungeon {
				_ = f.sendTownRobotVitalsLocked(party)
			}
		}
	}
	for _, b := range f.bots {
		if f.robotInDungeonLocked(b) {
			b.suspended = true
			b.walking = false
			b.announced = false
			continue
		}
		f.hub.mu.Lock()
		x, y := int32(int16(b.peer.x)), int32(int16(b.peer.y))
		f.hub.mu.Unlock()
		if b.suspended {
			b.suspended = false
			b.idleUntil = 0
		}
		if !b.announced {
			b.announced = true
			b.walking = false
			b.idleUntil = f.clock + 125 + uint64(b.choose(326))
			b.stride = 8 + float64(b.choose(9))
			f.publishTownMotion(b, x, y, 4|b.facing)
			continue
		}
		if !b.walking {
			// Operator-requested town ambience: two thirds remain standing.
			// Dungeon AI is independent of this town-only policy.
			if b.recordID%3 != 0 || f.clock < b.idleUntil {
				continue
			}
			b.destination = [2]int32{robotBound(x+int32(b.choose(101))-50, b.rect[0], b.rect[0]+b.rect[2]), robotBound(y+int32(b.choose(31))-15, b.rect[1], b.rect[1]+b.rect[3])}
			if b.destination == [2]int32{x, y} {
				b.idleUntil = f.clock + 225 + uint64(b.choose(376))
				continue
			}
			b.walking = true
		}
		dx, dy := float64(b.destination[0]-x), float64(b.destination[1]-y)
		distance := math.Hypot(dx, dy)
		nx, ny := b.destination[0], b.destination[1]
		if distance > b.stride {
			nx = x + int32(math.Round(dx*b.stride/distance))
			ny = y + int32(math.Round(dy*b.stride/distance))
		}
		if nx > x {
			b.facing = 1
		} else if nx < x {
			b.facing = 0
		}
		if nx == x && ny == y || nx == b.destination[0] && ny == b.destination[1] {
			b.walking = false
			b.idleUntil = f.clock + 225 + uint64(b.choose(376))
			f.publishTownMotion(b, nx, ny, 4|b.facing)
		} else {
			f.publishTownMotion(b, nx, ny, b.facing)
		}
		b.step++
	}
}
