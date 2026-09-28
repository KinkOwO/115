package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type MoonSoloStamp struct {
	RunID string
	Room  uint64
}
type MoonSoloSnapshot struct {
	Stamp   MoonSoloStamp
	Dungeon *Session
}
type MoonCompletedPhase struct {
	Dungeon uint32
	Visited map[uint32][]protocol.DungeonMonster
	Dead    map[uint16]bool
}

const (
	moonSoloLoading = 1
	moonSoloRunning = 2
)

// One owner per connection, never shared between actors. This is deliberately
// NOT the donor's multiplayer world, UDP relay, or admission coordinator.
type MoonSoloOwner struct {
	mu            sync.Mutex
	session       *Session
	source        string
	leader        uint16
	room          uint64
	phase         int
	seed          uint32
	ready         map[uint16]bool
	timeout       time.Duration
	deadline      time.Time
	roomRequester uint16
	transition    protocol.DungeonRoomTransition
	reuseRoom     bool
}

func NewMoonSolo(c catalog.DungeonCatalog, level byte, actor uint16, seed uint32, now time.Time) (*MoonSoloOwner, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid Moon solo actor")
	}
	s, e := SelectMoon(c, protocol.DungeonSelection{ID: 100004136, Party: 65535}, level, nil, seed)
	if e != nil {
		return nil, e
	}
	if e = s.InitializeMoon(c); e != nil {
		return nil, e
	}
	s.StartedAt = now
	return &MoonSoloOwner{session: s, source: c.Source.Checksum, leader: actor, room: 1, phase: moonSoloLoading, seed: seed, timeout: 45 * time.Second, deadline: now.Add(45 * time.Second)}, nil
}
func (r *MoonSoloOwner) validate(stamp MoonSoloStamp, actor uint16) error {
	if r == nil || r.session == nil || stamp != (MoonSoloStamp{r.session.RunID, r.room}) || actor != r.leader {
		return fmt.Errorf("stale/foreign Moon solo command")
	}
	return nil
}
func (r *MoonSoloOwner) Stamp() MoonSoloStamp { return MoonSoloStamp{r.session.RunID, r.room} }

// Access is confined to the gateway's existing single-writer connection loop.
func (r *MoonSoloOwner) Session() *Session { return r.session }
func (r *MoonSoloOwner) LoadExpired(now time.Time) bool {
	return r.phase == moonSoloLoading && now.After(r.deadline)
}
func (r *MoonSoloOwner) snapshot() (MoonSoloSnapshot, error) {
	b, e := json.Marshal(r.session)
	if e != nil {
		return MoonSoloSnapshot{}, e
	}
	var s Session
	if e = json.Unmarshal(b, &s); e != nil {
		return MoonSoloSnapshot{}, e
	}
	s.completed, s.completionTarget = r.session.completed, r.session.completionTarget
	return MoonSoloSnapshot{r.Stamp(), &s}, nil
}
func (s *Session) archivedPhases() []MoonCompletedPhase {
	p := MoonCompletedPhase{Dungeon: s.Definition.ID, Visited: map[uint32][]protocol.DungeonMonster{}, Dead: map[uint16]bool{}}
	for id, v := range s.Visited {
		p.Visited[id] = append([]protocol.DungeonMonster(nil), v...)
	}
	for id, v := range s.Dead {
		p.Dead[id] = v
	}
	return append(append([]MoonCompletedPhase(nil), s.PhaseHistory...), p)
}
func (r *MoonSoloOwner) Loaded(now time.Time) error {
	if r.phase != moonSoloLoading && r.phase != moonSoloRunning {
		return fmt.Errorf("invalid Moon load phase")
	}
	if r.phase == moonSoloLoading && now.After(r.deadline) {
		return fmt.Errorf("Moon load timeout")
	}
	r.phase = moonSoloRunning
	r.session.Loaded = true
	return nil
}
func (r *MoonSoloOwner) Move(c catalog.DungeonCatalog, request protocol.DungeonRoomTransition, seed uint32, now time.Time) error {
	if c.Source.Checksum != r.source || r.phase != moonSoloRunning || request.LayerChange || request.Record[0] != 0 || request.Dungeon != 0 && request.Dungeon != r.session.Definition.ID {
		return fmt.Errorf("invalid Moon solo move")
	}
	record, e := moonDoorTransition([2]byte{r.session.Room.X, r.session.Room.Y}, request.Position)
	if e != nil {
		return e
	}
	next, e := r.session.moveMoonSecondFloor(c, request.Position)
	if e != nil {
		return e
	}
	request.Record, request.Dungeon = record, next.Definition.ID
	r.session = next
	r.room++
	r.seed = seed
	r.phase = moonSoloLoading
	r.deadline = now.Add(r.timeout)
	r.transition = request
	return nil
}
func (r *MoonSoloOwner) StartMap() ([]byte, error) {
	s := r.session
	var fixed []protocol.DungeonMonster
	for _, m := range s.LivingMonsters() {
		if _, dynamic := s.MoonDynamic[m.Entity]; !dynamic {
			fixed = append(fixed, m)
		}
	}
	p := protocol.StartMapState{Position: [2]byte{s.Room.X, s.Room.Y}, Seed: r.seed, Map: s.Room.Map, Monsters: fixed}
	if r.transition.Dungeon == s.Definition.ID {
		p.Transition = &r.transition.Record
	}
	return protocol.StartMap(p)
}
