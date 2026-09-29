package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

func (s *Session) MoonPortalReady() bool {
	if s == nil || !s.Loaded || s.Definition.ID != 100004136 || !s.Completed() {
		return false
	}
	gate, ok := s.MoonNamed[s.Room.Map]
	return ok && gate.Slot == 0 && gate.Dead && gate.Spawn.Template == 109017560 && gate.Spawn.Entity == s.CompletionTarget() && s.Dead[gate.Spawn.Entity]
}

// One owned run crosses the source-defined floor boundary. Never create a
// second reward identity or recycle monster UIDs from floor1.
func (r *MoonSoloOwner) AdvanceMoonFloor(stamp MoonSoloStamp, leader uint16, c catalog.DungeonCatalog, seed uint32, now time.Time) (MoonSoloSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.validate(stamp, leader); e != nil {
		return MoonSoloSnapshot{}, e
	}
	if leader != r.leader || r.phase != moonSoloRunning || !r.session.MoonPortalReady() || r.room == ^uint64(0) {
		return MoonSoloSnapshot{}, fmt.Errorf("Moon Lake first floor is not complete")
	}
	next, e := Select(c, protocol.DungeonSelection{ID: 100004137, Party: 65535}, 115, nil)
	if e != nil {
		return MoonSoloSnapshot{}, e
	}
	if uint32(r.session.NextEntity)+uint32(len(next.Monsters)) >= 65535 {
		return MoonSoloSnapshot{}, fmt.Errorf("floor entity budget exhausted")
	}
	next.RunID, next.StartedAt = r.session.RunID, r.session.StartedAt
	next.PhaseHistory = r.session.archivedPhases()
	next.MoonNamed = r.session.MoonNamed // owned history moves with the same run
	next.MoonFeverUntil, next.MoonFeverCooldown = r.session.MoonFeverUntil, r.session.MoonFeverCooldown
	next.MoonDynamic = r.session.MoonDynamic
	next.MoonCleared = r.session.MoonCleared
	next.MoonFirstGrid = r.session.MoonFirstGrid
	next.MoonGridReady, next.MoonGrid = r.session.MoonGridReady, r.session.MoonGrid
	next.MoonZermioGrid, next.MoonZermioDefeated = r.session.MoonZermioGrid, r.session.MoonZermioDefeated
	next.MoonScored, next.MoonTroopScore, next.MoonFeverScore = r.session.MoonScored, r.session.MoonTroopScore, r.session.MoonFeverScore
	for i := range next.Monsters {
		next.Monsters[i].Entity = r.session.NextEntity + uint16(i)
	}
	next.NextEntity = r.session.NextEntity + uint16(len(next.Monsters))
	next.Visited = map[uint32][]protocol.DungeonMonster{next.Room.Map: next.Monsters}
	r.session = next
	r.room++
	r.seed = seed
	r.phase = moonSoloLoading
	r.ready = map[uint16]bool{}
	r.deadline = now.Add(r.timeout)
	r.roomRequester = leader
	r.transition = protocol.DungeonRoomTransition{}
	r.reuseRoom = false
	return r.snapshot()
}
