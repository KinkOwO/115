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

// hasKillableBoss reports whether the current room holds a rank-3 monster the
// client will actually raise a BOSS_CHECK for. A story dummy boss carries
// [displayhuntdummy] and is spawned with NonCombat set, so the client never
// treats it as a boss and never sends CMD 117 for it: waiting on a completion
// target that can never arrive would strand the run.
func (s *Session) hasKillableBoss() bool {
	for _, m := range s.Monsters {
		if m.Rank == 3 && !m.NonCombat {
			return true
		}
	}
	return false
}

// atLayerFinalMap reports whether the current room sits on the last map of the
// layered story sequence it belongs to. A room outside every layer entry is
// reported false; callers must not treat that alone as a reason to withhold a
// completion.
func (s *Session) atLayerFinalMap() bool {
	position := [2]byte{s.Room.X, s.Room.Y}
	for _, layer := range s.Maze.Layers {
		if layer.Position == position && len(layer.Maps) > 0 {
			return s.Room.Map == layer.Maps[len(layer.Maps)-1]
		}
	}
	return false
}

// hasLayerEntry reports whether the current room belongs to a layered story
// sequence at all.
func (s *Session) hasLayerEntry() bool {
	position := [2]byte{s.Room.X, s.Room.Y}
	for _, layer := range s.Maze.Layers {
		if layer.Position == position && len(layer.Maps) > 0 {
			return true
		}
	}
	return false
}

// A source boss death clears its own room: the client removes the remaining
// ordinary monsters with the boss and never reports them individually, so a
// completion must not wait for those reports. Every source boss in the room
// still requires its own death report before the run is complete.
func (s *Session) tryComplete() {
	if s.completionTarget == 0 {
		// Dungeon 26 maze 3's terminal layer is the opposite shape. Its last map
		// is entered with a live combat target, and the validated closing
		// [CHANGE MAP] cinematic returns to that cached final map once the
		// fighting ends. Only that transition may finish this story run, so the
		// map is deliberately kept out of the room-clear fallback below: the
		// fallback would complete the run the moment the target dies, before the
		// scene plays. Live-verified, see
		// analysis/tasks/lotus-terminal-layer-20260923.md.
		if s.Definition.ID == 26 && s.Maze.Index == 3 && s.Room.Map == 100008697 {
			if s.lotusClosingReached && s.RoomCleared() && s.storyDisplayTarget() != 0 {
				s.completed = true
			}
			return
		}
		// A layered story sequence whose only rank-3 actor is a display dummy
		// never yields a BOSS_CHECK - the client raises command 117 only for a
		// real boss - so completionTarget stays zero and no death report for
		// that dummy can ever close the run. The client ends the run on the map
		// itself instead: quest 3191 (dungeon 15 maze 6, palaceofload) walks
		// 100008695 -> 100008694 -> 100008684 -> 100008683 without a single
		// fight, so there is nothing to wait for but the arrival. Close on the
		// condition the layer does have - the sequence reached its final map and
		// every killable enemy is dead - and only there, so an ordinary room on
		// the way to the end cannot complete early.
		if s.hasKillableBoss() || !s.atLayerFinalMap() {
			return
		}
		if !s.roomSettled() {
			return
		}
		s.completed = true
		return
	}
	if s.Definition.Odyssey || s.Definition.ID == 100003126 {
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
	// A layered sequence only completes on its final map; a room outside every
	// layer entry has no such sequence to wait for.
	if s.hasLayerEntry() && !s.atLayerFinalMap() {
		return
	}
	s.completed = true
}

// roomSettled reports whether every killable enemy in the active room is dead.
// A cinematic actor the client never raises combat for is not an enemy here,
// which is the same predicate RoomCleared applies to open the door.
func (s *Session) roomSettled() bool {
	if s == nil || !s.Loaded {
		return false
	}
	for _, m := range s.Monsters {
		if !m.NonCombat && !s.Dead[m.Entity] {
			return false
		}
	}
	return true
}

func (s *Session) TryComplete() { s.tryComplete() }

func (s *Session) Completed() bool { return s != nil && s.completed }

// CompletionTarget is the boss identity echoed back in the NOTI 115 payload. A
// story layer never raises a BOSS_CHECK, so no requested identity exists; the
// client is told about the room's display boss instead, which is the only
// rank-3 actor it knows there. The value must stay encodable - the wire
// encoder rejects 0 and 65535 - or the entire completion batch is dropped
// before the clear-enable ships, which looks exactly like nothing happening.
func (s *Session) CompletionTarget() uint16 {
	if !s.Completed() {
		return 0
	}
	if s.completionTarget != 0 {
		return s.completionTarget
	}
	return s.storyDisplayTarget()
}

// storyDisplayTarget picks the rank-3 actor standing in for the boss when the
// client never sent a boss check. A team-100 row wins: that is the hostile
// display boss the client knows. A team-0 story actor is only a last resort.
func (s *Session) storyDisplayTarget() uint16 {
	var fallback uint16
	for _, m := range s.Monsters {
		if m.Rank != 3 || m.Entity == 0 || m.Entity == 65535 {
			continue
		}
		if m.Team == 100 {
			return m.Entity
		}
		if fallback == 0 {
			fallback = m.Entity
		}
	}
	return fallback
}
