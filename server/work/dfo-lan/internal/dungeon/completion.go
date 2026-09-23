package dungeon

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

// BossCheck may precede the target death. Retain its identity and wait for
// the actual room reports; do not manufacture story-dummy or actor deaths.
func (s *Session) BossCheck(r protocol.BossCheckRequest, actor uint16) error {
	if s == nil || !s.Loaded || r.Actor != actor || r.Target == 0 || r.Target == 65535 {
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
			if m.Entity == r.Target && (m.Rank == 3 || m.APC && m.Rank >= 5 && m.Rank <= 8) {
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

// hasLayerEntry reports whether the current room belongs to a layered story
// sequence at all. A room in no layer entry has no scene sequence to wait for.
func (s *Session) hasLayerEntry() bool {
	position := [2]byte{s.Room.X, s.Room.Y}
	for _, layer := range s.Maze.Layers {
		if layer.Position == position && len(layer.Maps) > 0 {
			return true
		}
	}
	return false
}

// atLayerFinalMap reports whether the current room sits on the last map of
// every layered sequence entry it belongs to. A room outside all layer entries
// is reported false - callers keep hasLayerEntry alongside it for that case.
func (s *Session) atLayerFinalMap() bool {
	position := [2]byte{s.Room.X, s.Room.Y}
	final := false
	for _, layer := range s.Maze.Layers {
		if layer.Position != position || len(layer.Maps) == 0 {
			continue
		}
		if s.Room.Map != layer.Maps[len(layer.Maps)-1] {
			return false
		}
		final = true
	}
	return final
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
		// map short-circuits every generic room-clear fallback below: they would
		// complete the run the moment the fighting stops, before the scene
		// plays. Live-verified, see analysis/tasks/lotus-terminal-layer-20260923.md.
		if s.Definition.ID == 26 && s.Maze.Index == 3 && s.Room.Map == 100008697 {
			if s.lotusClosingReached && s.RoomCleared() && s.reportableLotusTarget() != 0 {
				s.completed = true
			}
			return
		}
		// A plain source boss room can end without BOSS_CHECK when its only
		// boss is a non-combat display actor. The source map and boss position
		// must both match; layer scenes reuse the boss position while changing
		// maps. An actual fightable boss must still wait for its check.
		if s.Loaded && s.atSourceBossMap() && !s.hasFightableBoss() && s.roomEnemiesDead() && s.reportableDisplayBoss() != 0 {
			s.completed = true
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
		// the way to the end cannot complete early. This is the layer-scene
		// counterpart of the source-boss room above: that one matches the boss
		// coordinate outside every layer entry, this one matches a layer's own
		// last map, and the two never both hold.
		if s.Loaded && s.atLayerFinalMap() && !s.hasFightableBoss() && s.roomEnemiesDead() && s.reportableDisplayBoss() != 0 {
			s.completed = true
			return
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
	// A layered sequence only completes on its final map; a room outside every
	// layer entry has no such sequence to wait for.
	if s.hasLayerEntry() && !s.atLayerFinalMap() {
		return
	}
	s.completed = true
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
	if s.completionTarget == 0 {
		if target := s.reportableDisplayBoss(); target != 0 && !s.lotusClosingReached {
			return target
		}
		return s.reportableLotusTarget()
	}
	return s.completionTarget
}

func (s *Session) atSourceBossMap() bool {
	position := [2]byte{s.Room.X, s.Room.Y}
	if !s.Room.Boss || position != s.Maze.Boss {
		return false
	}
	for _, layer := range s.Maze.Layers {
		if layer.Position == position && len(layer.Maps) > 0 {
			return false
		}
	}
	for _, room := range s.Maze.Rooms {
		if room.Boss && [2]byte{room.X, room.Y} == position && room.Map == s.Room.Map {
			return true
		}
	}
	return false
}

func (s *Session) hasFightableBoss() bool {
	for _, m := range s.Monsters {
		if !m.NonCombat && m.Team != 0 && (m.Rank == 3 || m.APC && m.Rank >= 5 && m.Rank <= 8) {
			return true
		}
	}
	return false
}

func (s *Session) roomEnemiesDead() bool {
	for _, m := range s.Monsters {
		if m.Team != 0 && !m.NonCombat && !s.Dead[m.Entity] {
			return false
		}
	}
	return true
}

func (s *Session) reportableDisplayBoss() uint16 {
	for _, m := range s.Monsters {
		if m.Rank == 3 && m.Team != 0 && m.Entity != 0 && m.Entity != 65535 {
			return m.Entity
		}
	}
	return 0
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
