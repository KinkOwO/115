package dungeon

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

// BossCheck may precede the target death. Retain its identity and wait for
// the actual room reports; do not manufacture story-dummy or actor deaths.
func (s *Session) BossCheck(r protocol.BossCheckRequest, actor uint16) error {
	if s == nil || !s.Loaded || r.Actor != actor || !s.Room.Boss || [2]byte{s.Room.X, s.Room.Y} != s.Maze.Boss {
		return fmt.Errorf("boss check requires the owned loaded boss room")
	}
	found := false
	for _, m := range s.Monsters {
		if m.Entity == r.Target && m.Rank == 3 {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("boss check target is not a source boss in this room")
	}
	if s.completionTarget != 0 && s.completionTarget != r.Target {
		return fmt.Errorf("conflicting boss completion target")
	}
	s.completionTarget = r.Target
	s.tryComplete()
	return nil
}

// A source boss death clears its own room: the client removes the remaining
// ordinary monsters with the boss and never reports them individually, so a
// completion must not wait for those reports. Every source boss in the room
// still requires its own death report before the run is complete.
func (s *Session) tryComplete() {
	if s.completionTarget == 0 || !s.Dead[s.completionTarget] {
		return
	}
	// Cinematic display bosses remain in NOTI29 and require their own death
	// report for final completion, even though they do not block ordinary doors.
	for _, m := range s.Monsters {
		if m.Rank == 3 && !s.Dead[m.Entity] {
			return
		}
	}
	s.completed = true
}

func (s *Session) Completed() bool { return s != nil && s.completed }
func (s *Session) CompletionTarget() uint16 {
	if !s.Completed() {
		return 0
	}
	return s.completionTarget
}
