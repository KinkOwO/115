package dungeon

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

type Session struct {
	RunID            string
	StartedAt        time.Time
	Definition       catalog.DungeonDefinition
	Maze             catalog.DungeonMaze
	Room             catalog.DungeonRoom
	Monsters         []protocol.DungeonMonster
	Loaded           bool
	Dead             map[uint16]bool
	// Unowned marks a monster this character did not kill. It dies and the
	// room clears, but it pays no loot and no experience.
	Unowned          map[uint16]bool
	Visited          map[uint32][]protocol.DungeonMonster
	NextEntity       uint16
	completionTarget uint16
	completed        bool
}

func Select(c catalog.DungeonCatalog, r protocol.DungeonSelection, level byte, accepted map[uint16]bool) (*Session, error) {
	d, ok := c.Dungeons[r.ID]
	if !ok {
		return nil, fmt.Errorf("dungeon absent from imported source")
	}
	if uint32(level) < d.MinimumLevel {
		return nil, fmt.Errorf("dungeon minimum level not met")
	}
	// 客户端的难度是 1 起算的（1=普通 2=专家 3=达人 4=王者 5=英雄），
	// 原来要求 Difficulty==0，导致正常选图全被拒
	// （日志：dungeon_request_refused / unsupported dungeon option，
	//   请求字节 Difficulty=1 而客户端界面显示的就是 Normal）。
	if r.Difficulty > 5 {
		return nil, fmt.Errorf("unsupported dungeon difficulty %d", r.Difficulty)
	}
	if d.Tutorial || r.Extra != 0 || r.Mode != 0 || r.Flag != 0 || r.Party != 65535 || r.Reserved != 0 || r.Tail != 0 || r.Options != [2]byte{} || r.Event != 0 {
		return nil, fmt.Errorf("unsupported dungeon option")
	}
	if r.Quest > 65535 || r.Quest != 0 && !accepted[uint16(r.Quest)] {
		return nil, fmt.Errorf("quest is not accepted by this character")
	}
	var chosen *catalog.DungeonMaze
	for _, m := range d.Mazes {
		if uint32(m.Quest) == r.Quest {
			if chosen != nil {
				return nil, fmt.Errorf("ambiguous source maze")
			}
			copy := m
			chosen = &copy
		}
	}
	if chosen == nil || len(chosen.Pending) > 0 {
		return nil, fmt.Errorf("no resolved source maze for requested quest")
	}
	return newSession(c, d, *chosen)
}

// newSession builds the owned run for an already-resolved maze. Both the
// ordinary quest/town entry and the job tutorial entry share it so a tutorial
// room is spawned by exactly the same verified source rules.
func newSession(c catalog.DungeonCatalog, d catalog.DungeonDefinition, chosen catalog.DungeonMaze) (*Session, error) {
	s := &Session{Definition: d, Maze: chosen, StartedAt: time.Now()}
	var run [16]byte
	if _, e := rand.Read(run[:]); e != nil {
		return nil, e
	}
	s.RunID = hex.EncodeToString(run[:])
	found := false
	for _, room := range chosen.Rooms {
		if [2]byte{room.X, room.Y} == chosen.Start {
			if found {
				return nil, fmt.Errorf("ambiguous start room")
			}
			s.Room = room
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("missing source start room")
	}
	script, ok := c.Maps[s.Room.Map]
	if !ok {
		return nil, fmt.Errorf("start map not imported")
	}
	monsters, e := fixedMonsters(script, d.BasisLevel)
	if e != nil {
		return nil, e
	}
	s.Monsters = monsters
	s.Dead = map[uint16]bool{}
	s.Visited = map[uint32][]protocol.DungeonMonster{s.Room.Map: monsters}
	s.NextEntity = uint16(4096 + len(monsters))
	return s, nil
}

func (s *Session) ConfirmDeath(entity uint32, killer, actor uint16) (bool, error) {
	if s == nil || !s.Loaded || actor == 0 || actor == 65535 {
		return false, fmt.Errorf("death does not belong to loaded solo actor")
	}
	for _, m := range s.Monsters {
		if uint32(m.Entity) == entity {
			// killerFFFF means the client attributes the death to nobody.
			// Live capture 20260912T001341 shows the whole boss room report
			// six deaths inside three milliseconds: 0x1015, 0x1019 and 0x101a
			// with this actor as killer, and 0x1018, 0x101b and 0x101c with
			// FFFF, because the boss dying clears the rest of its room. The
			// same sentinel carries the cinematic despawn already seen in
			// live76123. Refusing it withheld the death confirmation, so the
			// client kept those monsters alive, the gate never opened, and
			// the room could only be left by returning to town.
			//
			// A death with no killer is therefore accepted and clears the
			// room, but it stays unowned: no loot, no experience. A foreign
			// killer that names some other actor is still refused.
			unowned := killer == 65535
			if killer != actor && !unowned {
				return false, fmt.Errorf("foreign combat killer")
			}
			if s.Dead[m.Entity] {
				return false, nil
			}
			if unowned {
				if s.Unowned == nil {
					s.Unowned = map[uint16]bool{}
				}
				s.Unowned[m.Entity] = true
			}
			s.Dead[m.Entity] = true
			s.tryComplete()
			return true, nil
		}
	}
	return false, fmt.Errorf("monster absent from current source room")
}
func (s *Session) RoomCleared() bool {
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
func (s *Session) Move(c catalog.DungeonCatalog, target [2]byte) (*Session, error) {
	if s.Completed() || s.completionTarget != 0 {
		return nil, fmt.Errorf("boss completion is pending or already accepted")
	}
	if !s.RoomCleared() {
		return nil, fmt.Errorf("current room not loaded or still has live enemies")
	}
	dx, dy := int(target[0])-int(s.Room.X), int(target[1])-int(s.Room.Y)
	if dx*dx+dy*dy != 1 {
		return nil, fmt.Errorf("target is not adjacent")
	}
	var room *catalog.DungeonRoom
	for _, r := range s.Maze.Rooms {
		if [2]byte{r.X, r.Y} == target {
			v := r
			room = &v
			break
		}
	}
	if room == nil {
		return nil, fmt.Errorf("target absent from source maze")
	}
	next := *s
	next.Room = *room
	next.Loaded = false
	next.Visited = map[uint32][]protocol.DungeonMonster{}
	for id, m := range s.Visited {
		next.Visited[id] = m
	}
	monsters, seen := next.Visited[room.Map]
	if !seen {
		script, ok := c.Maps[room.Map]
		if !ok {
			return nil, fmt.Errorf("target map not imported")
		}
		var e error
		monsters, e = fixedMonsters(script, s.Definition.BasisLevel)
		if e != nil {
			return nil, e
		}
		for i := range monsters {
			if next.NextEntity >= 65535 {
				return nil, fmt.Errorf("monster identity exhausted")
			}
			monsters[i].Entity = next.NextEntity
			next.NextEntity++
		}
		next.Visited[room.Map] = monsters
	}
	next.Monsters = monsters
	return &next, nil
}

// ClearedMaps lists every source map this run actually entered. A room
// transition requires the previous room to be cleared, so a completed run has
// cleared each of them; a source [clear map] objective may name any one, not
// only the final boss room.
func (s *Session) ClearedMaps() []uint32 {
	if s == nil {
		return nil
	}
	seen := map[uint32]bool{}
	for id := range s.Visited {
		seen[id] = true
	}
	// A run that never moved has no visit history but has still cleared the
	// room it is standing in.
	if s.Room.Map != 0 {
		seen[s.Room.Map] = true
	}
	out := make([]uint32, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (s *Session) LivingMonsters() []protocol.DungeonMonster {
	var out []protocol.DungeonMonster
	for _, m := range s.Monsters {
		if !s.Dead[m.Entity] {
			out = append(out, m)
		}
	}
	return out
}

// Preserve source indices: the native map parser uses them to retrieve spawn
// coordinates, behavior and cinematic flags from its matching PVF map row.
// Only fixed, exactly-one spawns are supported here; random rows are refused.
func fixedMonsters(script catalog.ScriptRecord, basis uint32) ([]protocol.DungeonMonster, error) {
	var out []protocol.DungeonMonster
	active := false
	c := script.Cells
	for i := 0; i < len(c); i++ {
		if c[i].Type == 3 {
			active = c[i].Text == "[monster]"
			continue
		}
		if !active {
			continue
		}
		if i+8 > len(c) {
			return nil, fmt.Errorf("short monster row")
		}
		var v [8]int32
		for j := range v {
			if c[i+j].Type != 0 {
				return nil, fmt.Errorf("unsupported monster row")
			}
			v[j] = c[i+j].Value
		}
		i += 8
		fixed, rankSeen, nonCombat := false, false, false
		var rank byte
		for i < len(c) && c[i].Type == 6 {
			switch c[i].Text {
			case "[fixed]":
				fixed = true
			case "[normal]", "[champion]", "[boss]":
				// Monster ranks: normal 0, champion 1, boss 3. Live capture
				// 20260912T025417 refused CMD45 (move to the next room) with
				// "unresolved monster spawn option [champion]" - a champion in
				// the target room stalled dungeon progression entirely because
				// the parser only knew [normal] and [boss].
				if rankSeen {
					return nil, fmt.Errorf("duplicate source monster rank")
				}
				rankSeen = true
				switch c[i].Text {
				case "[champion]":
					rank = 1
				case "[boss]":
					rank = 3
				}
			case "[cinematic]": // The native source index retains animation/event behavior.
			case "[dummy]", "[displayhuntdummy]":
				nonCombat = true
			default:
				return nil, fmt.Errorf("unresolved monster spawn option %q", c[i].Text)
			}
			i++
		}
		// A champion row carries one extra trailing field after its options
		// that normal and boss rows do not. The sole champion in the runtime
		// catalog (map 58570) shows a single 0 here; without consuming it the
		// next row is read from the wrong offset and reports a template-0
		// "invalid source row". Consume exactly one trailing zero for a
		// champion so the rest of the room aligns.
		if rank == 1 && i < len(c) && c[i].Type == 0 && c[i].Value == 0 {
			i++
		}
		i--
		if !fixed || !rankSeen || v[0] <= 0 || v[6] < 0 || v[7] < 0 || len(out) >= 255 {
			return nil, fmt.Errorf("unsupported random monster placement or invalid source row")
		}
		level := int64(v[2])
		if v[1] == 1 {
			level += int64(basis)
		} else if v[1] != 0 {
			return nil, fmt.Errorf("unsupported monster level expression")
		}
		if level < 1 || level > 255 {
			return nil, fmt.Errorf("invalid monster level")
		}
		out = append(out, protocol.DungeonMonster{Entity: uint16(4096 + len(out)), SourceIndex: uint32(len(out)), Level: byte(level), Template: uint32(v[0]), Rank: rank, Team: 100, NonCombat: nonCombat, SourceTail: [2]int32{v[6], v[7]}})
	}
	// Source teams are parallel to monster rows. Team0 supplies friendly
	// cinematic actors in this route; team100 supplies enemies. Never remove
	// either from the spawn list: client scripts address their source index.
	var teams []int32
	active = false
	for _, cell := range c {
		if cell.Type == 3 {
			active = cell.Text == "[monster team]"
			continue
		}
		if !active {
			continue
		}
		if cell.Type != 0 || cell.Value != 0 && cell.Value != 100 {
			return nil, fmt.Errorf("unresolved monster team")
		}
		teams = append(teams, cell.Value)
	}
	if len(teams) > 0 {
		if len(teams) != len(out) {
			return nil, fmt.Errorf("source monster/team count mismatch")
		}
		for i, team := range teams {
			out[i].Team = uint32(team)
			out[i].NonCombat = out[i].NonCombat || team == 0
		}
	}
	return out, nil
}
