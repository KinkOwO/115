package dungeon

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

// BossCheck may precede the target death. Retain its identity and wait for
// the actual room reports; do not manufacture story-dummy or actor deaths.
func (s *Session) BossCheck(r protocol.BossCheckRequest, actor uint16) error {
	if s == nil || !s.Loaded || r.Actor != actor {
		return fmt.Errorf("boss check requires the owned loaded boss room")
	}
	position := [2]byte{s.Room.X, s.Room.Y}
	for _, layer := range s.Maze.Layers {
		if layer.Position == position && len(layer.Maps) > 0 && s.Room.Map != layer.Maps[len(layer.Maps)-1] {
			return fmt.Errorf("boss scene sequence has not reached its final map")
		}
	}
	found := false
	if s.Definition.Odyssey {
		found = true
	} else {
		for _, m := range s.Monsters {
			if m.Entity == r.Target && m.Rank == 3 {
				if s.Definition.Odyssey && s.Definition.HuntBoss != 0 {
					// Source hunt targets can finish an epilogue outside the map's
					// boss coordinate. A real owned target/death is still required.
					found = m.Template == s.Definition.HuntBoss
				} else {
					found = s.Room.Boss && position == s.Maze.Boss
				}
			}
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
	if s.completionTarget == 0 {
		return
	}
	if s.Definition.Odyssey {
		s.completed = true
		return
	}
	if !s.Dead[s.completionTarget] {
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
