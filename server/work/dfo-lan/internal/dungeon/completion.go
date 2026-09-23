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
	if s.Definition.Odyssey {
		for _, layer := range s.Maze.Layers {
			if layer.Position == position && len(layer.Maps) > 0 && s.Room.Map != layer.Maps[len(layer.Maps)-1] {
				return fmt.Errorf("boss scene sequence has not reached its final map")
			}
		}
	}
	found := false
	if s.Definition.Odyssey || s.Definition.ID == 100003126 {
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
		// q3215's final room reports no BOSS_CHECK. The validated closing
		// cinematic returns to its cached final map after the fighting ends;
		// only that transition may finish this story run.
		if s.lotusClosingReached && s.Definition.ID == 26 && s.Maze.Index == 3 && s.Room.Map == 100008697 && s.RoomCleared() && s.reportableLotusTarget() != 0 {
			s.completed = true
		}
		return
	}
	if s.Definition.Odyssey || s.Definition.ID == 100003126 {
		s.completed = true
		return
	}
	if !s.Dead[s.completionTarget] {
		return
	}
	// Team-0 cinematic actors do not fight or report a death. A team-100
	// display dummy can report one and must still be confirmed before clear.
	for _, m := range s.Monsters {
		if m.Rank == 3 && m.Team != 0 && !s.Dead[m.Entity] {
			return
		}
	}
	position := [2]byte{s.Room.X, s.Room.Y}
	for _, layer := range s.Maze.Layers {
		if layer.Position == position && len(layer.Maps) > 0 && s.Room.Map != layer.Maps[len(layer.Maps)-1] {
			return
		}
	}
	s.completed = true
}

func (s *Session) TryComplete() { s.tryComplete() }

func (s *Session) Completed() bool { return s != nil && s.completed }
func (s *Session) CompletionTarget() uint16 {
	if !s.Completed() {
		return 0
	}
	if s.completionTarget == 0 {
		return s.reportableLotusTarget()
	}
	return s.completionTarget
}

// A story display boss is present in the final map's NOTI29 rows. Use its
// actual entity as the confirmation identity when no CMD117 was sent.
func (s *Session) reportableLotusTarget() uint16 {
	if s == nil || !s.lotusClosingReached || s.Definition.ID != 26 || s.Maze.Index != 3 || s.Room.Map != 100008697 {
		return 0
	}
	for _, m := range s.Monsters {
		if m.Rank == 3 && m.Team == 100 && m.Entity != 0 && m.Entity != 65535 {
			return m.Entity
		}
	}
	return 0
}
