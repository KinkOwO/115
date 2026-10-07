// Package raid owns raid phase state independently of ordinary dungeon runs.
package raid

import (
	"dfolan/internal/catalog"
	"fmt"
	"strings"
	"time"
)

type Member struct {
	Actor    uint16
	Position uint32
}

// BakalOpening owns the real solo roster and source phase event state. The
// wire adapter and durable reward workflow are separate consumers of effects.
type BakalOpening struct {
	rules          *catalog.BakalRaidRules
	roster         []Member
	readyAt        time.Time
	startedAt      time.Time
	variables      map[string]int32
	monsters       map[uint32]catalog.BakalRaidMonster
	dungeons       map[uint32]string
	timers         map[uint32]catalog.RaidPhaseTimer
	active         bool
	clock          time.Time
	deadlines      map[[2]uint32]time.Time
	scheduled      []BakalScheduled
	effects        []BakalEffect
	currentDungeon uint32
	clearCounts    map[uint32]bool
	ended          string
	rng            uint64
	bootstrapping  bool
	partyBuffs     map[string]time.Time
	raidBuffs      [5]uint32
	buffCooldowns  map[string]time.Time
	grantedBuffs   map[string]string
	monsterHealth  map[uint32]int32
}

func PrepareBakalOpening(entry catalog.RaidEntrance, members []Member, now time.Time) (*BakalOpening, error) {
	r := entry.Bakal
	if r == nil || r.PhaseMax != 1 || r.TimeLimit == 0 || entry.StartMinimum == 0 || entry.PartyMax == 0 {
		return nil, fmt.Errorf("missing native Bakal opening rules")
	}
	if uint32(len(members)) < entry.StartMinimum || uint32(len(members)) > entry.MemberMax {
		return nil, fmt.Errorf("roster outside native start limits")
	}
	// Native member +2d is party assignment: 0=unassigned, 1..3=parties.
	// This opening executor currently owns one real member only.
	if len(members) != 1 {
		return nil, fmt.Errorf("multi-member Bakal ownership is not implemented")
	}
	actors, positions := map[uint16]bool{}, map[uint32]bool{}
	for _, m := range members {
		if m.Actor == 0 || m.Position == 0 || m.Position > entry.PartyMax || actors[m.Actor] || positions[m.Position] {
			return nil, fmt.Errorf("invalid or duplicate real raid member")
		}
		actors[m.Actor], positions[m.Position] = true, true
	}
	s := &BakalOpening{rules: r, roster: append([]Member(nil), members...), readyAt: now.Add(time.Duration(r.StartDelay) * time.Second), variables: map[string]int32{}, monsters: map[uint32]catalog.BakalRaidMonster{}, dungeons: map[uint32]string{}, timers: map[uint32]catalog.RaidPhaseTimer{}}
	for k, v := range r.InitialVariables {
		s.variables[k] = v
	}
	for k := range r.Symbols {
		if _, exists := s.variables[k]; !exists {
			s.variables[k] = 0
		}
	}
	for _, d := range r.Dungeons {
		if d.ID == 0 || (d.State != "open" && d.State != "close") {
			return nil, fmt.Errorf("invalid source dungeon state")
		}
		if _, exists := s.dungeons[d.ID]; exists {
			return nil, fmt.Errorf("duplicate phase dungeon")
		}
		s.dungeons[d.ID] = d.State
	}
	for _, p := range r.Monsters {
		slot, ok := r.Slots[p.Location]
		if !ok || slot.Kind == "camp" || s.dungeons[slot.Dungeon] == "" {
			return nil, fmt.Errorf("initial monster outside combat battlefield")
		}
		m, ok := r.MonsterDefinitions[p.Kind]
		if !ok || m.ID == 0 {
			return nil, fmt.Errorf("unresolved native initial monster")
		}
		if _, exists := s.monsters[p.Location]; exists {
			return nil, fmt.Errorf("two initial monsters occupy one slot")
		}
		s.monsters[p.Location] = m
	}
	for _, t := range r.Timers {
		if _, exists := s.timers[t.ID]; exists {
			return nil, fmt.Errorf("duplicate phase timer")
		}
		s.timers[t.ID] = t
	}
	// The native CREATE MONSTER operation sets its presence symbol. Road
	// actors check these values in Incoming_ready.act and destroy themselves
	// when their corresponding named monster is absent.
	for _, m := range s.monsters {
		key := "[IS EXIST " + strings.ToUpper(m.Kind) + "]"
		if _, ok := r.Symbols[key]; ok {
			s.variables[key] = 1
		}
	}
	if len(s.dungeons) == 0 || len(s.monsters) == 0 {
		return nil, fmt.Errorf("empty battlefield opening")
	}
	return s, nil
}

func (s *BakalOpening) SymbolValues() map[uint32]int32 {
	out := map[uint32]int32{}
	if s == nil || s.rules == nil {
		return out
	}
	for name, value := range s.variables {
		if id, ok := s.rules.Symbols[name]; ok {
			out[id] = value
		}
	}
	return out
}

// EnterDungeon applies source-defined, fully supported symbol events after a
// real loading acknowledgement. Preview keeps rejected/aborted entry inert.
func (s *BakalOpening) EnterDungeon(id uint32, now time.Time, commit bool) (map[uint32]int32, error) {
	if err := s.AuthorizeDungeon(id, now); err != nil {
		return nil, err
	}
	if len(s.rules.Events) > 0 {
		copyState := s.cloneScript()
		if copyState.currentDungeon != id {
			copyState.currentDungeon = id
			if err := copyState.dispatchScript(BakalSignal{Op: "[ON ENTER DUNGEON]", ID: id}, now); err != nil {
				return nil, err
			}
		}
		if commit {
			*s = *copyState
		}
		return copyState.SymbolValues(), nil
	}
	values := make(map[string]int32, len(s.variables))
	for k, v := range s.variables {
		values[k] = v
	}
	for _, r := range s.rules.EnterSymbolRules {
		if r.Dungeon != id {
			continue
		}
		matches := true
		for _, c := range r.Conditions {
			v, exists := values[c.Name]
			if !exists {
				return nil, fmt.Errorf("unknown source condition %s", c.Name)
			}
			switch c.Operator {
			case "=":
				matches = matches && v == c.Value
			case "[>]":
				matches = matches && v > c.Value
			case "[<]":
				matches = matches && v < c.Value
			default:
				return nil, fmt.Errorf("unsupported source comparison %s", c.Operator)
			}
		}
		if !matches {
			continue
		}
		for _, a := range r.Assignments {
			if _, exists := values[a.Name]; !exists {
				return nil, fmt.Errorf("unknown source assignment %s", a.Name)
			}
			values[a.Name] = a.Value
		}
	}
	out := map[uint32]int32{}
	for name, value := range values {
		if symbol, exists := s.rules.Symbols[name]; exists {
			out[symbol] = value
		}
	}
	if commit {
		s.variables = values
	}
	return out, nil
}

func (s *BakalOpening) Activate(now time.Time) error {
	if s == nil || s.active {
		return fmt.Errorf("raid opening is absent or already active")
	}
	if now.Before(s.readyAt) {
		return fmt.Errorf("native start countdown has not elapsed")
	}
	s.startedAt = now
	s.active = true
	if len(s.rules.Events) > 0 {
		if err := s.initializeScript(now); err != nil {
			s.active = false
			return err
		}
	}
	return nil
}

func (s *BakalOpening) Remaining(now time.Time) uint32 {
	if s == nil || !s.active {
		return 0
	}
	left := s.startedAt.Add(time.Duration(s.rules.TimeLimit) * time.Second).Sub(now)
	if left <= 0 {
		return 0
	}
	if left > time.Duration(s.rules.TimeLimit)*time.Second {
		return s.rules.TimeLimit
	}
	return uint32((left + time.Second - 1) / time.Second)
}

// AuthorizeSlot binds a destination to the current phase's real source map,
// rather than accepting arbitrary client-supplied dungeon or map IDs.
func (s *BakalOpening) AuthorizeSlot(slotID, dungeonID, mapID uint32, now time.Time) (catalog.BakalRaidSlot, error) {
	if s == nil || !s.active || s.Remaining(now) == 0 {
		return catalog.BakalRaidSlot{}, fmt.Errorf("raid is not active")
	}
	slot, ok := s.rules.Slots[slotID]
	if !ok || slot.Kind == "camp" || slot.Dungeon != dungeonID || s.dungeons[dungeonID] != "open" {
		return catalog.BakalRaidSlot{}, fmt.Errorf("destination is not an open native combat slot")
	}
	for _, id := range slot.Maps {
		if id == mapID {
			copySlot := slot
			copySlot.Maps = append([]uint32(nil), slot.Maps...)
			return copySlot, nil
		}
	}
	return catalog.BakalRaidSlot{}, fmt.Errorf("map does not belong to native battlefield slot")
}

func (s *BakalOpening) InitialMonster(slotID uint32) (catalog.BakalRaidMonster, bool) {
	if s == nil || !s.active {
		return catalog.BakalRaidMonster{}, false
	}
	m, ok := s.monsters[slotID]
	if ok {
		if loc, exists := s.rules.Locations[slotID]; exists {
			m.HasGrid = true
			m.Grid = loc.Grid
			if loc.HasSpecificGrid {
				m.Grid = loc.SpecificGrid
			}
			if m.HasSecondGrid && s.variables["[BAKAL PHASE]"] > 0 {
				m.Grid = m.SecondGrid
				m.ID = m.SecondID
			}
		}
	}
	return m, ok
}

func (s *BakalOpening) Members() []Member {
	if s == nil {
		return nil
	}
	return append([]Member(nil), s.roster...)
}

func (s *BakalOpening) ReadyAt() time.Time { return s.readyAt }
func (s *BakalOpening) Active() bool       { return s != nil && s.active }

func (s *BakalOpening) AuthorizeDungeon(id uint32, now time.Time) error {
	if s == nil || !s.active || s.ended != "" || s.Remaining(now) == 0 || s.dungeons[id] != "open" {
		return fmt.Errorf("raid dungeon is not open in the active native phase")
	}
	return nil
}
