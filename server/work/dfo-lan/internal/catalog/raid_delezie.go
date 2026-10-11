package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"path"
	"strings"
)

// These values are the native difficulty enum at 14759DC80, not content IDs.
// 1476FD390 constructs .<difficulty>.ctp/.object.ctp/.buff.ctp siblings.
type DelezieDifficulty int32

const (
	DelezieNormal DelezieDifficulty = iota
	DelezieHard
	DelezieSingle
	DelezieMatch
)

func (d DelezieDifficulty) Name() string {
	switch d {
	case DelezieNormal:
		return "normal"
	case DelezieHard:
		return "hard"
	case DelezieSingle:
		return "single"
	case DelezieMatch:
		return "match"
	}
	return ""
}
func delezieDifficulty(name string) (DelezieDifficulty, error) {
	for d := DelezieNormal; d <= DelezieMatch; d++ {
		if d.Name() == name {
			return d, nil
		}
	}
	return 0, fmt.Errorf("unsupported native Delezie difficulty %q", name)
}

type DelezieDungeonRule struct {
	ID   uint32
	Rows []int
}
type DelezieLocationRule struct {
	ID      uint32
	Row     int
	Movable []uint32
	// Some mode tables retain links to locations omitted by that mode.
	// Keep them visible for admission/executor validation; do not invent nodes
	// or merge normal-mode nodes without a verified native inheritance rule.
	UndeclaredMovable []uint32
}
type DelezieObjectRule struct {
	ID, Dungeon        uint32
	Row                int
	StaticHP           int64
	HasStaticHP        bool
	CreatableLocations []uint32
	LockSymbol         string
	LockSymbolID       uint32
}

// An instruction retains its own subtree. Nested OR/AND conditions and mixed
// expressions must not be flattened or interpreted as Bakal token bytecode.
type DelezieScriptEvent struct {
	Trigger, Behavior       []pvf.CTPRecord
	TriggerRow, BehaviorRow int
}
type DeleziePhaseRules struct {
	Row       int
	Dungeons  map[uint32]DelezieDungeonRule
	Locations map[uint32]DelezieLocationRule
	Events    []DelezieScriptEvent
}
type DelezieModeRules struct {
	Difficulty            DelezieDifficulty
	Phase, Objects, Buffs *pvf.CTPTree
	Phases                []DeleziePhaseRules
	ObjectDefinitions     map[uint32]DelezieObjectRule
	BuffDefinitions       map[string]DelezieBuffRule
	ScalarExpressions     map[int]DelezieScalarExpression
	SummonActions         map[int]DelezieSummonAction
	PhaseTimerReferences  map[int]DelezieTimerReference
	ObjectTimerReferences map[int]DelezieTimerReference
	ObjectGraph           *DelezieObjectGraph
	MechanismBindings     *DelezieMechanismBindings
}
type DelezieRaidRules struct {
	Source                  pvf.ArchiveSnapshot
	Header                  *pvf.CTPTree
	Symbols, Timers         *pvf.CTPTree
	Modes                   map[DelezieDifficulty]*DelezieModeRules
	ChannelModes            map[uint32][]DelezieDifficulty
	SymbolIDs               map[string]uint32
	TimerDefinitions        map[string]DelezieTimerRule
	HeaderMechanismBindings *DelezieMechanismBindings
	Rewards                 *DelezieRewardRules
	// Source tables include native rewards, notices and client-only metadata.
	// They are source trees, never exported JSON used as runtime content.
	Tables map[string]*pvf.CTPTree
}

// ModeForChannel reads allowed modes from RAID LIST WINDOW DIFFICULTY INFO.
// Loading these rules does not enable the network/battle executor.
func (r *DelezieRaidRules) ModeForChannel(channel uint32, d DelezieDifficulty) (*DelezieModeRules, error) {
	if r == nil {
		return nil, fmt.Errorf("Delezie rules missing")
	}
	for _, allowed := range r.ChannelModes[channel] {
		if allowed == d {
			if mode := r.Modes[d]; mode != nil {
				return mode, nil
			}
		}
	}
	return nil, fmt.Errorf("Delezie channel %d does not declare difficulty %d", channel, d)
}

func ctpUint(row pvf.CTPRecord) (uint32, error) {
	if len(row.Cells) != 1 {
		return 0, fmt.Errorf("%s row %d: expected one integer", row.Name, row.Index)
	}
	c := row.Cells[0]
	v := c.Float
	if c.Kind != "float" || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > math.MaxUint32 || math.Trunc(v) != v {
		return 0, fmt.Errorf("%s row %d: invalid unsigned integer", row.Name, row.Index)
	}
	return uint32(v), nil
}
func ctpUintList(row pvf.CTPRecord) ([]uint32, error) {
	var out []uint32
	for _, cell := range row.Cells {
		r := row
		r.Cells = []pvf.CTPCell{cell}
		v, e := ctpUint(r)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func ctpID(t *pvf.CTPTree, parent int, tag string) (uint32, error) {
	r, e := t.One(parent, tag)
	if e != nil {
		return 0, e
	}
	v, e := ctpUint(r)
	if e != nil {
		return 0, fmt.Errorf("%s: %w", t.Table.Path, e)
	}
	return v, nil
}

func delezieChannelModes(header *pvf.CTPTree) (map[uint32][]DelezieDifficulty, error) {
	root, e := header.One(-1, "[RAID LIST WINDOW DIFFICULTY INFO]")
	if e != nil {
		return nil, e
	}
	out := map[uint32][]DelezieDifficulty{}
	for _, info := range header.Children(root.Index, "[INFO]") {
		channel, e := ctpID(header, info.Index, "[CHANNEL TYPE]")
		if e != nil {
			return nil, e
		}
		if channel == 0 || channel > 255 || len(out[channel]) > 0 {
			return nil, fmt.Errorf("%s: invalid/duplicate channel %d", header.Table.Path, channel)
		}
		row, e := header.One(info.Index, "[DIFFICULTY]")
		if e != nil {
			return nil, e
		}
		if len(row.Cells) == 0 {
			return nil, fmt.Errorf("%s: channel %d has no difficulties", header.Table.Path, channel)
		}
		seen := map[DelezieDifficulty]bool{}
		for _, c := range row.Cells {
			if c.Kind != "name" {
				return nil, fmt.Errorf("%s: invalid difficulty cell", header.Table.Path)
			}
			d, e := delezieDifficulty(c.Name)
			if e != nil {
				return nil, e
			}
			if seen[d] {
				return nil, fmt.Errorf("duplicate Delezie difficulty %s", c.Name)
			}
			seen[d] = true
			out[channel] = append(out[channel], d)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no channel/mode declarations", header.Table.Path)
	}
	return out, nil
}

func importDelezieRaidRules(a *pvf.Archive, base *pvf.CTPTable) (*DelezieRaidRules, error) {
	header, e := pvf.NewCTPTree(base)
	if e != nil {
		return nil, e
	}
	bindings, e := delezieChannelModes(header)
	if e != nil {
		return nil, e
	}
	r := &DelezieRaidRules{Source: a.Snapshot(), Header: header, ChannelModes: bindings, Modes: map[DelezieDifficulty]*DelezieModeRules{}, Tables: map[string]*pvf.CTPTree{base.Path: header}}
	if !strings.HasSuffix(base.Path, ".normal.ctp") {
		return nil, fmt.Errorf("Delezie canonical source must be .normal.ctp")
	}
	prefix := strings.TrimSuffix(base.Path, ".normal.ctp")
	load := func(p string) (*pvf.CTPTree, error) {
		if tree := r.Tables[p]; tree != nil {
			return tree, nil
		}
		t, e := a.CTP(p)
		if e != nil {
			return nil, e
		}
		tree, e := pvf.NewCTPTree(t)
		if e != nil {
			return nil, e
		}
		r.Tables[p] = tree
		return tree, nil
	}
	r.Symbols, e = load(prefix + ".symbol.ctp")
	if e != nil {
		return nil, e
	}
	r.Timers, e = load(prefix + ".timer.ctp")
	if e != nil {
		return nil, e
	}
	r.SymbolIDs, e = parseDelezieSymbols(r.Symbols)
	if e != nil {
		return nil, e
	}
	r.TimerDefinitions, e = parseDelezieTimers(r.Timers)
	if e != nil {
		return nil, e
	}
	r.HeaderMechanismBindings, e = parseDelezieMechanismBindings(header, r.SymbolIDs, r.TimerDefinitions)
	if e != nil {
		return nil, e
	}
	for _, modes := range bindings {
		for _, d := range modes {
			if r.Modes[d] != nil {
				continue
			}
			m := &DelezieModeRules{Difficulty: d, Phase: header, ObjectDefinitions: map[uint32]DelezieObjectRule{}}
			modePrefix := prefix + "." + d.Name()
			if d != DelezieNormal {
				m.Phase, e = load(modePrefix + ".ctp")
				if e != nil {
					return nil, e
				}
			}
			m.Objects, e = load(modePrefix + ".object.ctp")
			if e != nil {
				return nil, e
			}
			m.Buffs, e = load(modePrefix + ".buff.ctp")
			if e != nil {
				return nil, e
			}
			m.Phases, e = parseDeleziePhases(m.Phase)
			if e != nil {
				return nil, e
			}
			m.ScalarExpressions, e = parseDelezieScalarExpressions(m.Phase, r.SymbolIDs)
			if e != nil {
				return nil, e
			}
			m.SummonActions, e = parseDelezieSummonActions(m.Phase, r.SymbolIDs)
			if e != nil {
				return nil, e
			}
			m.ObjectDefinitions, e = parseDelezieObjects(m.Objects, r.SymbolIDs)
			if e != nil {
				return nil, e
			}
			m.BuffDefinitions, e = parseDelezieBuffs(m.Buffs)
			if e != nil {
				return nil, e
			}
			m.PhaseTimerReferences, e = parseDelezieTimerReferences(m.Phase, r.TimerDefinitions)
			if e != nil {
				return nil, e
			}
			m.ObjectTimerReferences, e = parseDelezieTimerReferences(m.Objects, r.TimerDefinitions)
			if e != nil {
				return nil, e
			}
			m.ObjectGraph, e = parseDelezieObjectGraph(m.Objects, m.ObjectDefinitions)
			if e != nil {
				return nil, e
			}
			if m.Phase == header {
				m.MechanismBindings = r.HeaderMechanismBindings
			} else {
				m.MechanismBindings, e = parseDelezieMechanismBindings(m.Phase, r.SymbolIDs, r.TimerDefinitions)
				if e != nil {
					return nil, e
				}
			}
			r.Modes[d] = m
		}
	}
	if e = a.IterateFilesUnder(path.Dir(base.Path), func(f pvf.File) error {
		p := strings.ToLower(f.ArchivePath)
		if !strings.HasSuffix(p, ".ctp") {
			return nil
		}
		_, e := load(p)
		return e
	}); e != nil {
		return nil, e
	}
	rewardTokens, e := a.Tokens(delezieRewardPath)
	if e != nil {
		return nil, e
	}
	r.Rewards, e = parseDelezieRewards(rewardTokens)
	if e != nil {
		return nil, e
	}
	for difficulty := range r.Modes {
		if _, ok := r.Rewards.Modes[difficulty]; !ok {
			return nil, fmt.Errorf("%s: no source rewards for %s", delezieRewardPath, difficulty.Name())
		}
	}
	return r, nil
}

func parseDeleziePhases(t *pvf.CTPTree) ([]DeleziePhaseRules, error) {
	var out []DeleziePhaseRules
	for _, root := range t.Children(-1, "[PHASE]") {
		phase := DeleziePhaseRules{Row: root.Index, Dungeons: map[uint32]DelezieDungeonRule{}, Locations: map[uint32]DelezieLocationRule{}}
		dungeons, e := t.One(root.Index, "[DUNGEONS]")
		if e != nil {
			return nil, e
		}
		for _, info := range t.Children(dungeons.Index, "[DUNGEON INFO]") {
			id, e := ctpID(t, info.Index, "[INDEX]")
			if e != nil {
				return nil, e
			}
			if id == 0 {
				return nil, fmt.Errorf("%s: zero dungeon", t.Table.Path)
			}
			// A mode may repeat a dungeon with distinct retry metadata. Preserve
			// every row in the tree; this map is only a membership index.
			dungeon := phase.Dungeons[id]
			dungeon.ID = id
			dungeon.Rows = append(dungeon.Rows, info.Index)
			phase.Dungeons[id] = dungeon
		}
		locations, e := t.One(root.Index, "[LOCATIONS]")
		if e != nil {
			return nil, e
		}
		for _, info := range t.Children(locations.Index, "[LOCATION INFO]") {
			id, e := ctpID(t, info.Index, "[LOCATION INDEX]")
			if e != nil {
				return nil, e
			}
			if _, dup := phase.Locations[id]; dup {
				return nil, fmt.Errorf("%s: duplicate location %d", t.Table.Path, id)
			}
			loc := DelezieLocationRule{ID: id, Row: info.Index}
			for _, list := range t.Children(info.Index, "[MOVABLE LOCATION]") {
				ids, e := ctpUintList(list)
				if e != nil {
					return nil, e
				}
				loc.Movable = append(loc.Movable, ids...)
			}
			phase.Locations[id] = loc
		}
		for key, loc := range phase.Locations {
			for _, id := range loc.Movable {
				if _, ok := phase.Locations[id]; !ok {
					loc.UndeclaredMovable = append(loc.UndeclaredMovable, id)
				}
			}
			phase.Locations[key] = loc
		}
		phase.Events, e = parseDeleziePhaseEvents(t, root.Index)
		if e != nil {
			return nil, e
		}
		if len(phase.Dungeons) == 0 || len(phase.Locations) == 0 || len(phase.Events) == 0 {
			return nil, fmt.Errorf("%s: incomplete phase", t.Table.Path)
		}
		out = append(out, phase)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no phase roots", t.Table.Path)
	}
	return out, nil
}

func parseDelezieObjects(t *pvf.CTPTree, symbols map[string]uint32) (map[uint32]DelezieObjectRule, error) {
	if t == nil || t.Table == nil {
		return nil, fmt.Errorf("Delezie object source missing")
	}
	root, e := t.One(-1, "[OBJECTS]")
	if e != nil {
		return nil, e
	}
	out := map[uint32]DelezieObjectRule{}
	for _, info := range t.Children(root.Index, "[OBJECT]") {
		id, e := ctpID(t, info.Index, "[INDEX]")
		if e != nil {
			return nil, e
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf("%s: duplicate object %d", t.Table.Path, id)
		}
		// Some map objects have no associated dungeon. Absence stays explicit.
		var dungeon uint32
		if len(t.Children(info.Index, "[DUNGEON INDEX]")) > 0 {
			dungeon, e = ctpID(t, info.Index, "[DUNGEON INDEX]")
			if e != nil {
				return nil, e
			}
		}
		object := DelezieObjectRule{ID: id, Dungeon: dungeon, Row: info.Index}
		if err := bindDelezieObjectExecution(t, info.Index, symbols, &object); err != nil {
			return nil, err
		}
		out[id] = object
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: empty objects", path.Base(t.Table.Path))
	}
	return out, nil
}
