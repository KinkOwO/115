package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// BakalRaidRules preserves the normal source phase, battlefield, actors and
// reward pools. Separate hard-mode blocks are never merged into normal rules.
type BakalRaidRules struct {
	PhaseMax, StartDelay, TimeLimit     uint32
	Dungeons                            []RaidPhaseDungeon
	InitialVariables                    map[string]int32
	Symbols                             map[string]uint32
	PortalEdges                         map[[2]uint32]string
	PortalObjects                       map[[2]uint32][]string
	Monsters, ReservedMonsters          []RaidMonsterPlacement
	Timers                              []RaidPhaseTimer
	Buffs                               []RaidPhaseBuff
	Slots                               map[uint32]BakalRaidSlot
	MonsterDefinitions                  map[string]BakalRaidMonster
	EnterSymbolRules                    []BakalEnterSymbolRule
	Events                              []BakalScriptEvent
	Locations                           map[uint32]BakalScriptLocation
	BuffDefinitions                     map[string]BakalBuffDefinition
	MinimumClearCount                   uint32
	WeeklyClearCount, WeeklyRewardCount uint32
	RaidBuffCooldownMillis              uint32
	Rewards                             []BakalRewardEntry
	Stage                               BakalStageRule
	Bidding                             BakalBiddingRules
}

// Only complete, supported event blocks are projected. Mixed behaviours such
// as random awakening and timers remain outside this symbol-only executor.
type BakalSymbolCondition struct {
	Name, Operator string
	Value          int32
}
type BakalSymbolAssignment struct {
	Name  string
	Value int32
}
type BakalEnterSymbolRule struct {
	Dungeon     uint32
	Conditions  []BakalSymbolCondition
	Assignments []BakalSymbolAssignment
}

type BakalRaidSlot struct {
	ID, Dungeon, TownArea uint32
	Kind                  string
	Maps                  []uint32
}
type BakalRaidMonster struct {
	Kind                         string
	ID, SecondID                 uint32
	SpecificMap, X, Y            int32
	Grid, SecondGrid             [2]byte
	HasGrid, HasSecondGrid       bool
	BuffCount                    uint32
	BuffCheck                    string
	BuffPool, AdditionalBuffPool []BakalWeightedBuff
}

type BakalWeightedBuff struct {
	Kind   string
	Weight uint32
}
type BakalBuffDefinition struct {
	Kind                                   string
	Index, Cooltime, Duration, ServerValue uint32
	Raid                                   bool
	Appendage                              []int32
}

func loadBakalRaidTables(a *pvf.Archive, r *BakalRaidRules) error {
	symbols, err := readBakalCOSTokens(a, "contents/2022/bakalraid/etc/bakal.etc.symbol")
	if err != nil {
		return err
	}
	r.Symbols, err = parseBakalSymbols(symbols)
	if err != nil {
		return err
	}
	ts, err := readBakalCOSTokens(a, "contents/2022/bakalraid/etc/clientbakalslotscript.cos")
	if err != nil {
		return err
	}
	r.Slots, err = parseBakalRaidSlots(ts)
	if err != nil {
		return err
	}
	ts, err = readBakalCOSTokens(a, "contents/2022/bakalraid/etc/bakalmonster.cos")
	if err != nil {
		return err
	}
	r.MonsterDefinitions, err = parseBakalRaidMonsters(ts)
	if err != nil {
		return err
	}
	ts, err = readBakalCOSTokens(a, "contents/2022/bakalraid/etc/bakalbuff.cos")
	if err != nil {
		return err
	}
	r.BuffDefinitions, err = parseBakalBuffDefinitions(ts)
	if err != nil {
		return err
	}
	for i, t := range ts {
		if t.Text == "[RAID BUFF COOLTIME]" && i+1 < len(ts) && ts[i+1].Type == 0 && ts[i+1].Value >= 0 {
			r.RaidBuffCooldownMillis = uint32(ts[i+1].Value)
		}
	}
	ts, err = a.Tokens("contents/system/raidsystem/raidreward.etc")
	if err != nil {
		return err
	}
	r.Rewards, err = parseBakalRewards(ts, r.Dungeons[0].ID)
	if err != nil {
		return err
	}
	r.Stage, err = parseBakalStage(a, r.Symbols)
	if err != nil {
		return err
	}
	ts, err = a.Tokens("contents/2022/bakalraid/etc/bakal.etc")
	if err != nil {
		return err
	}
	r.Bidding, err = loadBakalBidding(a, ts)
	if err != nil {
		return err
	}
	dungeons := map[uint32]bool{}
	for _, d := range r.Dungeons {
		dungeons[d.ID] = true
	}
	for _, slot := range r.Slots {
		if slot.Kind != "camp" && !dungeons[slot.Dungeon] {
			return fmt.Errorf("bakal slot %d refers outside phase dungeons", slot.ID)
		}
	}
	for _, placements := range [][]RaidMonsterPlacement{r.Monsters, r.ReservedMonsters} {
		for _, m := range placements {
			if _, ok := r.Slots[m.Location]; !ok {
				return fmt.Errorf("bakal placement has missing slot %d", m.Location)
			}
			if _, ok := r.MonsterDefinitions[m.Kind]; !ok {
				return fmt.Errorf("bakal placement has missing monster %s", m.Kind)
			}
		}
	}
	r.PortalEdges = map[[2]uint32]string{}
	r.PortalObjects = map[[2]uint32][]string{}
	// Native warp assets explicitly name both connected dungeons. Require
	// the actual source object to declare warp-object behavior as well.
	pattern := regexp.MustCompile(`^contents/2022/bakalraid/passive/warp_object/\d+_(\d+)to(\d+)\.obj$`)
	for _, file := range a.Files() {
		match := pattern.FindStringSubmatch(file.ArchivePath)
		if match == nil {
			continue
		}
		from, e1 := strconv.ParseUint(match[1], 10, 32)
		to, e2 := strconv.ParseUint(match[2], 10, 32)
		if e1 != nil || e2 != nil || !dungeons[uint32(from)] || !dungeons[uint32(to)] {
			continue
		}
		cells, err := a.Tokens(file.ArchivePath)
		if err != nil {
			return err
		}
		for i, c := range cells {
			if c.Type == 3 && c.Text == "[passive object type]" && i+1 < len(cells) && cells[i+1].Text == "[warp object]" {
				r.PortalEdges[[2]uint32{uint32(from), uint32(to)}] = file.ArchivePath
				r.PortalObjects[[2]uint32{uint32(from), uint32(to)}] = append(r.PortalObjects[[2]uint32{uint32(from), uint32(to)}], file.ArchivePath)
			}
		}
	}
	return nil
}

func parseBakalSymbols(ts []pvf.Token) (map[string]uint32, error) {
	if len(ts) == 0 || len(ts)%2 != 0 {
		return nil, fmt.Errorf("invalid native Bakal symbol pairs")
	}
	out, ids := map[string]uint32{}, map[uint32]bool{}
	for i := 0; i < len(ts); i += 2 {
		if ts[i].Type != 0 || ts[i].Value <= 0 || ts[i+1].Type != 3 || out[ts[i+1].Text] != 0 || ids[uint32(ts[i].Value)] {
			return nil, fmt.Errorf("invalid or duplicate native Bakal symbol")
		}
		out[ts[i+1].Text] = uint32(ts[i].Value)
		ids[uint32(ts[i].Value)] = true
	}
	return out, nil
}

// These COS entries are native type 3 UTF-16 text, whereas bakal.etc uses
// type 1 five-byte cells. Keep the two archive representations distinct.
func readBakalCOSTokens(a *pvf.Archive, path string) ([]pvf.Token, error) {
	s, err := a.ReadText(path)
	if err != nil {
		return nil, err
	}
	var out []pvf.Token
	s = strings.Trim(s, "\ufeff\x00 \t\r\n")
	for len(s) > 0 {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			break
		}
		if strings.HasPrefix(s, "//") {
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
				continue
			}
			break
		}
		if s[0] == '[' || s[0] == '`' {
			end := byte(']')
			typ := byte(3)
			if s[0] == '`' {
				end = '`'
				typ = 6
			}
			i := strings.IndexByte(s[1:], end)
			if i < 0 {
				return nil, fmt.Errorf("%s: unclosed COS token", path)
			}
			i++
			value := s[:i+1]
			if typ == 6 {
				value = s[1:i]
			}
			out = append(out, pvf.Token{Type: typ, Text: value})
			s = s[i+1:]
			continue
		}
		i := strings.IndexAny(s, " \t\r\n[")
		if i < 0 {
			i = len(s)
		}
		if i == 0 {
			return nil, fmt.Errorf("%s: invalid COS token", path)
		}
		word := s[:i]
		s = s[i:]
		if v, e := strconv.ParseInt(word, 10, 32); e == nil {
			out = append(out, pvf.Token{Type: 0, Value: int32(v)})
		} else if v, e := strconv.ParseFloat(word, 32); e == nil {
			out = append(out, pvf.Token{Type: 2, Number: float32(v)})
		} else {
			out = append(out, pvf.Token{Type: 255, Text: word})
		}
	}
	return out, nil
}

// The projections retain native dungeon/map/monster identities. Unrelated UI
// animation and display fields are not needed to authorize server entry.
func parseBakalRaidSlots(ts []pvf.Token) (map[uint32]BakalRaidSlot, error) {
	out := map[uint32]BakalRaidSlot{}
	var slot *BakalRaidSlot
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if t.Type != 3 {
			continue
		}
		switch t.Text {
		case "[location]":
			if slot != nil {
				return nil, fmt.Errorf("bakal nested slot")
			}
			slot = &BakalRaidSlot{}
		case "[slot]", "[dungeon]", "[town]":
			if slot == nil || i+1 >= len(ts) || ts[i+1].Type != 0 || ts[i+1].Value <= 0 {
				return nil, fmt.Errorf("bakal invalid %s", t.Text)
			}
			v := uint32(ts[i+1].Value)
			switch t.Text {
			case "[slot]":
				slot.ID = v
			case "[dungeon]":
				slot.Dungeon = v
			case "[town]":
				slot.TownArea = v
			}
		case "[type]":
			if slot == nil || i+1 >= len(ts) || ts[i+1].Type != 6 || ts[i+1].Text == "" {
				return nil, fmt.Errorf("bakal missing slot type")
			}
			slot.Kind = ts[i+1].Text
		case "[map]":
			if slot == nil {
				return nil, fmt.Errorf("bakal map outside slot")
			}
			for i++; i < len(ts) && ts[i].Type == 0; i++ {
				if ts[i].Value <= 0 {
					return nil, fmt.Errorf("bakal invalid slot map")
				}
				slot.Maps = append(slot.Maps, uint32(ts[i].Value))
			}
			if i == len(ts) || ts[i].Text != "[/map]" {
				return nil, fmt.Errorf("bakal unclosed slot maps")
			}
		case "[/location]":
			if slot == nil || slot.ID == 0 || slot.ID > 54 || slot.Kind == "" {
				return nil, fmt.Errorf("bakal incomplete slot")
			}
			if _, exists := out[slot.ID]; exists {
				return nil, fmt.Errorf("bakal duplicate slot %d", slot.ID)
			}
			if slot.Kind == "camp" {
				if slot.TownArea == 0 || slot.Dungeon != 0 || len(slot.Maps) != 0 {
					return nil, fmt.Errorf("bakal invalid camp")
				}
			} else if slot.Dungeon == 0 || len(slot.Maps) == 0 || slot.TownArea != 0 {
				return nil, fmt.Errorf("bakal incomplete combat slot")
			}
			out[slot.ID] = *slot
			slot = nil
		}
	}
	if slot != nil || len(out) == 0 {
		return nil, fmt.Errorf("bakal incomplete slot table")
	}
	return out, nil
}

func parseBakalRaidMonsters(ts []pvf.Token) (map[string]BakalRaidMonster, error) {
	out := map[string]BakalRaidMonster{}
	var m *BakalRaidMonster
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if t.Type != 3 {
			continue
		}
		switch t.Text {
		case "[MONSTER INFO]":
			if m != nil {
				return nil, fmt.Errorf("bakal nested monster")
			}
			m = &BakalRaidMonster{SpecificMap: -1}
		case "[MONSTER TYPE]":
			if m == nil || i+1 >= len(ts) || ts[i+1].Type != 6 || ts[i+1].Text == "" {
				return nil, fmt.Errorf("bakal missing monster type")
			}
			m.Kind = ts[i+1].Text
		case "[MONSTER INDEX]", "[MONSTER INDEX2]", "[SPECIFIC MAP]":
			if m == nil || i+1 >= len(ts) || ts[i+1].Type != 0 {
				return nil, fmt.Errorf("bakal invalid monster field %s", t.Text)
			}
			v := ts[i+1].Value
			if t.Text == "[SPECIFIC MAP]" {
				m.SpecificMap = v
			} else {
				if v <= 0 {
					return nil, fmt.Errorf("bakal invalid monster ID")
				}
				if t.Text == "[MONSTER INDEX]" {
					m.ID = uint32(v)
				} else {
					m.SecondID = uint32(v)
				}
			}
		case "[POSITION]":
			if m == nil || i+2 >= len(ts) || ts[i+1].Type != 0 || ts[i+2].Type != 0 {
				return nil, fmt.Errorf("bakal invalid monster position")
			}
			m.X, m.Y = ts[i+1].Value, ts[i+2].Value
		case "[BUFF COUNT]":
			if m == nil || i+1 >= len(ts) || ts[i+1].Type != 0 || ts[i+1].Value < 0 {
				return nil, fmt.Errorf("invalid normal buff count")
			}
			m.BuffCount = uint32(ts[i+1].Value)
			i++
		case "[BUFF CHECK]":
			if m == nil || i+1 >= len(ts) || ts[i+1].Type != 6 {
				return nil, fmt.Errorf("invalid normal buff check")
			}
			m.BuffCheck = ts[i+1].Text
			i++
		case "[BUFF LIST]", "[ADDITIONAL BUFF LIST]":
			if m == nil {
				return nil, fmt.Errorf("buff pool outside monster")
			}
			additional := t.Text == "[ADDITIONAL BUFF LIST]"
			end := "[/BUFF LIST]"
			if additional {
				end = "[/ADDITIONAL BUFF LIST]"
			}
			for i++; i < len(ts) && ts[i].Text != end; i += 2 {
				if i+1 >= len(ts) || ts[i].Type != 6 || ts[i+1].Type != 0 || ts[i+1].Value <= 0 {
					return nil, fmt.Errorf("invalid normal buff weight")
				}
				row := BakalWeightedBuff{ts[i].Text, uint32(ts[i+1].Value)}
				if additional {
					m.AdditionalBuffPool = append(m.AdditionalBuffPool, row)
				} else {
					m.BuffPool = append(m.BuffPool, row)
				}
			}
		case "[APPEAR GRID2]":
			if m == nil || i+2 >= len(ts) || ts[i+1].Type != 0 || ts[i+2].Type != 0 || ts[i+1].Value < 0 || ts[i+1].Value > 255 || ts[i+2].Value < 0 || ts[i+2].Value > 255 {
				return nil, fmt.Errorf("invalid second arena grid")
			}
			m.SecondGrid = [2]byte{byte(ts[i+1].Value), byte(ts[i+2].Value)}
			m.HasSecondGrid = true
		case "[/MONSTER INFO]":
			if m == nil || m.Kind == "" || m.ID == 0 {
				return nil, fmt.Errorf("bakal incomplete monster")
			}
			if _, exists := out[m.Kind]; exists {
				return nil, fmt.Errorf("bakal duplicate monster %s", m.Kind)
			}
			out[m.Kind] = *m
			m = nil
		}
	}
	if m != nil || len(out) == 0 {
		return nil, fmt.Errorf("bakal incomplete monster table")
	}
	return out, nil
}

type RaidPhaseDungeon struct {
	ID, PartyMax uint32
	Kind, State  string
}
type RaidMonsterPlacement struct {
	Location uint32
	Kind     string
}
type RaidPhaseTimer struct{ ID, Location, Seconds uint32 }
type RaidPhaseBuff struct{ ID, Count uint32 }

func importBakalRaidRules(ts []pvf.Token) (*BakalRaidRules, error) {
	r := &BakalRaidRules{InitialVariables: map[string]int32{}}
	number := func(i int) (uint32, error) {
		if i >= len(ts) || ts[i].Type != 0 || ts[i].Value < 0 {
			return 0, fmt.Errorf("bakal opening rule: expected nonnegative integer at %d", i)
		}
		return uint32(ts[i].Value), nil
	}
	text := func(i int) (string, error) {
		if i >= len(ts) || ts[i].Type != 6 || ts[i].Text == "" {
			return "", fmt.Errorf("bakal opening rule: expected string at %d", i)
		}
		return ts[i].Text, nil
	}
	phase, opening := false, false
	var d *RaidPhaseDungeon
	seen := map[uint32]bool{}
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if t.Type != 3 {
			continue
		}
		var err error
		switch t.Text {
		case "[PHASE MAX]", "[START DELAY TIME]", "[PHASE TIME OVER]", "[MINIMAL DUNGEON CLEAR COUNT]", "[WEEKLY CLEAR COUNT]", "[WEEKLY REWARD COUNT]":
			v, e := number(i + 1)
			if e != nil {
				return nil, e
			}
			switch t.Text {
			case "[PHASE MAX]":
				r.PhaseMax = v
			case "[START DELAY TIME]":
				r.StartDelay = v
			case "[PHASE TIME OVER]":
				r.TimeLimit = v
			case "[MINIMAL DUNGEON CLEAR COUNT]":
				r.MinimumClearCount = v
			case "[WEEKLY CLEAR COUNT]":
				r.WeeklyClearCount = v
			case "[WEEKLY REWARD COUNT]":
				r.WeeklyRewardCount = v
			}
		case "[PHASE]":
			if phase {
				return nil, fmt.Errorf("bakal opening rules require exactly one phase")
			}
			phase = true
		case "[DUNGEON INFO]":
			if !phase || d != nil {
				return nil, fmt.Errorf("bakal dungeon outside opening phase")
			}
			d = &RaidPhaseDungeon{}
		case "[INDEX]":
			if d != nil {
				d.ID, err = number(i + 1)
			}
		case "[TYPE]":
			if d != nil {
				d.Kind, err = text(i + 1)
			}
		case "[INIT STATE]":
			if d != nil {
				d.State, err = text(i + 1)
			}
		case "[PARTY MAX]":
			if d != nil {
				d.PartyMax, err = number(i + 1)
			}
		case "[/DUNGEON INFO]":
			if d == nil || d.ID == 0 || d.PartyMax == 0 || d.State == "" || seen[d.ID] {
				return nil, fmt.Errorf("bakal invalid or duplicate dungeon definition")
			}
			if d.State != "open" && d.State != "close" {
				return nil, fmt.Errorf("bakal unsupported initial state %q", d.State)
			}
			seen[d.ID] = true
			r.Dungeons = append(r.Dungeons, *d)
			d = nil
		case "[1PHASE INIT]":
			if !opening && phase {
				v, e := number(i + 1)
				if e != nil || v != 0 {
					return nil, fmt.Errorf("bakal missing zero initialization trigger")
				}
				opening = true
			}
		case "[BEHAVIOR]":
			if !opening {
				return nil, fmt.Errorf("bakal missing phase initialization before behavior")
			}
			for i++; i < len(ts) && ts[i].Text != "[/BEHAVIOR]"; i++ {
				switch ts[i].Text {
				case "[SET]":
					if i+2 >= len(ts) || ts[i+1].Type != 3 || ts[i+2].Type != 0 {
						return nil, fmt.Errorf("bakal unsupported initial assignment")
					}
					key := ts[i+1].Text
					if _, exists := r.InitialVariables[key]; exists {
						return nil, fmt.Errorf("bakal duplicate initial variable %s", key)
					}
					r.InitialVariables[key] = ts[i+2].Value
					i += 2
				case "[CREATE MONSTER]", "[RESERVE CREATE MONSTER]":
					location, e := number(i + 1)
					if e != nil {
						return nil, e
					}
					kind, e := text(i + 2)
					if e != nil {
						return nil, e
					}
					if location == 0 || location > 54 {
						return nil, fmt.Errorf("bakal invalid monster location")
					}
					m := RaidMonsterPlacement{location, kind}
					if ts[i].Text == "[CREATE MONSTER]" {
						r.Monsters = append(r.Monsters, m)
					} else {
						r.ReservedMonsters = append(r.ReservedMonsters, m)
					}
					i += 2
				case "[SET TIMER]":
					id, e := number(i + 1)
					if e != nil {
						return nil, e
					}
					location, e := number(i + 2)
					if e != nil {
						return nil, e
					}
					seconds, e := number(i + 3)
					if e != nil {
						return nil, e
					}
					r.Timers = append(r.Timers, RaidPhaseTimer{id, location, seconds})
					i += 3
				case "[ADD BAKAL RAID BUFF]":
					id, e := number(i + 1)
					if e != nil {
						return nil, e
					}
					count, e := number(i + 2)
					if e != nil {
						return nil, e
					}
					r.Buffs = append(r.Buffs, RaidPhaseBuff{id, count})
					i += 2
				default:
					return nil, fmt.Errorf("bakal unsupported opening behavior %s", ts[i].Text)
				}
			}
			if i == len(ts) {
				return nil, fmt.Errorf("bakal unclosed opening behavior")
			}
			if r.PhaseMax != 1 || r.TimeLimit == 0 || len(r.Dungeons) == 0 || len(r.Monsters) == 0 || r.InitialVariables["[1PHASE INIT]"] != 1 {
				return nil, fmt.Errorf("bakal incomplete opening rules")
			}
			r.EnterSymbolRules = parseBakalEnterSymbolRules(ts)
			r.Events, r.Locations, err = parseBakalScript(ts)
			if err != nil {
				return nil, err
			}
			return r, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("bakal missing opening behavior")
}
