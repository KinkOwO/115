package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
	"sort"
)

// Server policy: S4 manual A/B weights, one source-weighted group per wave,
// MOB dungeon basis level and AIC script level. This is not an official 115
// reward formula. The hidden-row queue itself follows the 115 native reader.
type HellPartyRun struct {
	Map    uint32
	Mode   byte
	Key    string
	Rows   []protocol.DungeonMonster
	Actors map[uint16]HellPartyRunActor
}

type HellPartyRunActor struct {
	Group       uint16
	Order       uint16
	RewardRolls uint32
	HellMonster bool
}

func newHellPartyRun(c catalog.DungeonCatalog, s *Session, draw func(uint64) (uint64, error)) (*HellPartyRun, error) {
	rules := c.HellRules
	if rules == nil || s.Definition.HellParty == nil {
		return nil, fmt.Errorf("Hell Party rules unavailable")
	}
	mapID := s.Definition.HellParty.SealMap
	script, err := c.MapScript(mapID)
	if err != nil {
		return nil, err
	}
	pillars, err := catalog.ParseHellPartyPillars(script)
	if err != nil {
		return nil, err
	}
	if len(pillars) != 1 {
		return nil, fmt.Errorf("Hell map %d has no single supported legacy pillar", mapID)
	}
	var weights uint64
	var modes []catalog.HellPartyDifficulty
	for _, row := range rules.Difficulties {
		if row.Key == "A" || row.Key == "B" {
			modes = append(modes, row)
			weights += uint64(row.Columns[3])
		}
	}
	if weights == 0 {
		return nil, fmt.Errorf("Hell A/B manual weights absent")
	}
	roll, err := draw(weights)
	if err != nil {
		return nil, err
	}
	var mode catalog.HellPartyDifficulty
	for _, row := range modes {
		w := uint64(row.Columns[3])
		if roll < w {
			mode = row
			break
		}
		roll -= w
	}
	run := &HellPartyRun{Map: mapID, Mode: 1, Key: mode.Key, Actors: map[uint16]HellPartyRunActor{}}
	if mode.Key == "B" {
		run.Mode = 2
	}
	choices := map[uint16][]catalog.HellPartyMapChoice{}
	for _, choice := range pillars[0].Choices {
		group, exists := rules.Groups[choice.Group]
		if !exists {
			return nil, fmt.Errorf("Hell map references missing group %d", choice.Group)
		}
		if group.Difficulty == mode.Key {
			choices[choice.Order] = append(choices[choice.Order], choice)
		}
	}
	orders := make([]int, 0, len(choices))
	for order := range choices {
		orders = append(orders, int(order))
	}
	sort.Ints(orders)
	if len(orders) == 0 {
		return nil, fmt.Errorf("Hell map %d has no groups for mode %s", mapID, mode.Key)
	}
	nextEntity := s.NextEntity
	for _, order := range orders {
		rows := choices[uint16(order)]
		var total uint64
		for _, row := range rows {
			total += uint64(row.Weight)
		}
		selected := rows[0] // S4's all-zero weight fallback, within this mode only
		if total > 0 {
			roll, err := draw(total)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				if roll < uint64(row.Weight) {
					selected = row
					break
				}
				roll -= uint64(row.Weight)
			}
		}
		group := rules.Groups[selected.Group]
		for _, actor := range group.Actors {
			source, known := rules.Actors[actor]
			if !known || source.Unavailable != "" || actor.EntityType > 1 {
				return nil, fmt.Errorf("Hell group %d actor %d type %d source unavailable: %s", group.ID, actor.Template, actor.EntityType, source.Unavailable)
			}
			if nextEntity == 0 || nextEntity >= 65535 || s.Definition.BasisLevel == 0 || s.Definition.BasisLevel > 255 {
				return nil, fmt.Errorf("Hell actor identity/level exhausted")
			}
			m := protocol.DungeonMonster{Entity: nextEntity, Template: actor.Template, Level: byte(s.Definition.BasisLevel), Team: 100, Hidden: true, SpawnOrder: uint16(order)}
			if actor.EntityType == 1 {
				m.APC = true
				m.SourceIndex = 10000
				m.Rank = 5
				m.Level = source.Level
				if m.Level == 0 {
					return nil, fmt.Errorf("Hell APC level absent")
				}
			}
			run.Rows = append(run.Rows, m)
			run.Actors[nextEntity] = HellPartyRunActor{Group: group.ID, Order: uint16(order), RewardRolls: mode.Columns[0], HellMonster: source.HellMonster}
			nextEntity++
		}
	}
	if len(run.Rows) == 0 || len(run.Rows) > 255 {
		return nil, fmt.Errorf("Hell wave roster exceeds native count")
	}
	s.NextEntity = nextEntity
	return run, nil
}

func (s *Session) HellPartyMode() byte {
	if s == nil || s.HellParty == nil {
		return 0
	}
	return s.HellParty.Mode
}

// HellPartyReward checks server-owned entity membership and the whole selected
// group. The ordinary death/loot deduplication owns the actual one-time award.
func (s *Session) HellPartyReward(entity uint16) (HellPartyRunActor, bool) {
	if s == nil || s.HellParty == nil || s.Room.Map != s.HellParty.Map {
		return HellPartyRunActor{}, false
	}
	actor, known := s.HellParty.Actors[entity]
	if !known {
		return actor, false
	}
	if actor.HellMonster || !s.Dead[entity] || s.Unowned[entity] || s.hellLastDeaths[[2]uint16{actor.Order, actor.Group}] != entity {
		actor.RewardRolls = 0
		return actor, true
	}
	for id, other := range s.HellParty.Actors {
		if other.Group == actor.Group && other.Order == actor.Order && !s.Dead[id] {
			actor.RewardRolls = 0
			break
		}
	}
	return actor, true
}

func (s *Session) recordHellDeath(entity uint16) {
	if s.HellParty == nil || s.Room.Map != s.HellParty.Map {
		return
	}
	actor, known := s.HellParty.Actors[entity]
	if !known {
		return
	}
	for id, other := range s.HellParty.Actors {
		if other.Order == actor.Order && other.Group == actor.Group && !s.Dead[id] {
			return
		}
	}
	if s.hellLastDeaths == nil {
		s.hellLastDeaths = map[[2]uint16]uint16{}
	}
	s.hellLastDeaths[[2]uint16{actor.Order, actor.Group}] = entity
}

func (s *Session) HellPartyCleared() bool {
	if s == nil || s.HellParty == nil || s.Room.Map != s.HellParty.Map || len(s.HellParty.Actors) == 0 {
		return false
	}
	for id := range s.HellParty.Actors {
		if !s.Dead[id] {
			return false
		}
	}
	return true
}

func (s *Session) HellPartyAPCs() []protocol.HellPartyAPC {
	if s == nil || s.HellParty == nil {
		return nil
	}
	var out []protocol.HellPartyAPC
	seen := map[protocol.HellPartyAPC]bool{}
	for _, m := range s.HellParty.Rows {
		if m.APC {
			row := protocol.HellPartyAPC{Template: m.Template, Level: uint32(m.Level)}
			if !seen[row] {
				out = append(out, row)
				seen[row] = true
			}
		}
	}
	return out
}
