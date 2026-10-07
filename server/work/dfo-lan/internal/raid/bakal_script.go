package raid

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
	"strings"
	"time"
)

type BakalSignal struct {
	Op, Name     string
	ID, Location uint32
}
type BakalScheduled struct {
	At          time.Time
	Instruction catalog.BakalScriptInstruction
}
type BakalEffect struct {
	Op, Kind     string
	ID, Location uint32
	Value        int32
}

func (s *BakalOpening) cloneScript() *BakalOpening {
	c := *s
	c.partyBuffs = map[string]time.Time{}
	for k, v := range s.partyBuffs {
		c.partyBuffs[k] = v
	}
	c.buffCooldowns = map[string]time.Time{}
	for k, v := range s.buffCooldowns {
		c.buffCooldowns[k] = v
	}
	c.grantedBuffs = map[string]string{}
	for k, v := range s.grantedBuffs {
		c.grantedBuffs[k] = v
	}
	c.monsterHealth = map[uint32]int32{}
	for k, v := range s.monsterHealth {
		c.monsterHealth[k] = v
	}
	c.variables = map[string]int32{}
	for k, v := range s.variables {
		c.variables[k] = v
	}
	c.monsters = map[uint32]catalog.BakalRaidMonster{}
	for k, v := range s.monsters {
		c.monsters[k] = v
	}
	c.dungeons = map[uint32]string{}
	for k, v := range s.dungeons {
		c.dungeons[k] = v
	}
	c.deadlines = map[[2]uint32]time.Time{}
	for k, v := range s.deadlines {
		c.deadlines[k] = v
	}
	c.clearCounts = map[uint32]bool{}
	for k, v := range s.clearCounts {
		c.clearCounts[k] = v
	}
	c.scheduled = append([]BakalScheduled(nil), s.scheduled...)
	c.effects = append([]BakalEffect(nil), s.effects...)
	return &c
}

func (s *BakalOpening) initializeScript(now time.Time) error {
	s.clock = now
	s.rng = uint64(now.UnixNano()) | 1
	s.deadlines = map[[2]uint32]time.Time{}
	s.clearCounts = map[uint32]bool{}
	s.variables = map[string]int32{}
	for k := range s.rules.Symbols {
		s.variables[k] = 0
	}
	s.monsters = map[uint32]catalog.BakalRaidMonster{}
	s.scheduled = nil
	s.effects = nil
	s.partyBuffs = map[string]time.Time{}
	s.buffCooldowns = map[string]time.Time{}
	s.grantedBuffs = map[string]string{}
	s.monsterHealth = map[uint32]int32{}
	s.raidBuffs = [5]uint32{}
	s.bootstrapping = true
	err := s.dispatchScript(BakalSignal{Op: "init"}, now)
	s.bootstrapping = false
	return err
}

func (s *BakalOpening) matchScript(trigger []catalog.BakalScriptInstruction, signal BakalSignal) bool {
	selector := false
	for _, t := range trigger {
		a := t.Args
		switch t.Op {
		case "[IF]":
			v := s.variables[a[0].Text]
			cmp := "="
			if len(a) == 3 {
				cmp = a[1].Text
			}
			r := a[len(a)-1].Value
			pass := false
			switch cmp {
			case "=":
				pass = v == r
			case "[>]":
				pass = v > r
			case "[<]":
				pass = v < r
			case "[=>]":
				pass = v >= r
			case "[=<]":
				pass = v <= r
			case "[!=]":
				pass = v != r
			}
			if !pass {
				return false
			}
		case "[CHECK DUNGEON STATE]":
			if s.dungeons[uint32(a[0].Value)] != a[1].Text {
				return false
			}
		case "[ON ENTER DUNGEON]", "[ON GIVEUP DUNGEON]", "[ON CHANGE DUNGEON STATE]":
			selector = true
			if signal.Op != t.Op || signal.ID != uint32(a[0].Value) {
				return false
			}
		case "[ON CHANGE SYMBOL]":
			selector = true
			if signal.Op != t.Op || signal.Name != a[0].Text {
				return false
			}
		case "[CHECK TIMER END]":
			selector = true
			if signal.Op != t.Op || signal.ID != uint32(a[0].Value) || signal.Location != uint32(a[1].Value) {
				return false
			}
		default:
			return false
		}
	}
	return selector || signal.Op == "init"
}

func (s *BakalOpening) dispatchScript(signal BakalSignal, now time.Time) error {
	queue := []BakalSignal{signal}
	for count := 0; len(queue) > 0; count++ {
		if count > 4096 {
			return fmt.Errorf("native Bakal event cascade exceeded bound")
		}
		sig := queue[0]
		queue = queue[1:]
		for _, rule := range s.rules.Events {
			if !s.matchScript(rule.Trigger, sig) {
				continue
			}
			for _, op := range rule.Behavior {
				if err := s.executeScript(op, now, &queue); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *BakalOpening) setScriptSymbol(name string, value int32, queue *[]BakalSignal) error {
	if _, ok := s.rules.Symbols[name]; !ok {
		return fmt.Errorf("unknown source symbol %s", name)
	}
	if s.variables[name] == value && !s.bootstrapping {
		return nil
	}
	s.variables[name] = value
	s.effects = append(s.effects, BakalEffect{Op: "symbol", Kind: name, Value: value})
	*queue = append(*queue, BakalSignal{Op: "[ON CHANGE SYMBOL]", Name: name})
	return nil
}

func (s *BakalOpening) presence(kind string, queue *[]BakalSignal) error {
	value := int32(0)
	for _, m := range s.monsters {
		if m.Kind == kind {
			value = 1
			break
		}
	}
	name := "[IS EXIST " + strings.ToUpper(kind) + "]"
	if _, exists := s.rules.Symbols[name]; !exists {
		return nil
	}
	return s.setScriptSymbol(name, value, queue)
}

func (s *BakalOpening) randomIndex(n int) int {
	s.rng ^= s.rng << 13
	s.rng ^= s.rng >> 7
	s.rng ^= s.rng << 17
	return int(s.rng % uint64(n))
}

func (s *BakalOpening) createScriptMonster(location uint32, kind string, queue *[]BakalSignal) error {
	m, exists := s.rules.MonsterDefinitions[kind]
	if !exists {
		return fmt.Errorf("unknown native monster kind %s", kind)
	}
	if _, exists := s.rules.Locations[location]; !exists {
		return fmt.Errorf("unknown native monster location %d", location)
	}
	if _, occupied := s.monsters[location]; occupied {
		return nil
	}
	s.monsters[location] = m
	s.monsterHealth[location] = s.rules.InitialVariables["[MONSTER MAX HP]"]
	s.effects = append(s.effects, BakalEffect{Op: "monster", Kind: kind, Location: location, Value: 1})
	return s.presence(kind, queue)
}

func (s *BakalOpening) deleteScriptMonster(location uint32, queue *[]BakalSignal) error {
	m, exists := s.monsters[location]
	if !exists {
		return nil
	}
	delete(s.monsters, location)
	delete(s.monsterHealth, location)
	// Location timers belong to its occupant. A defeated hatchery must stop
	// producing waves; a defeated dragon must stop its repeating anger timer.
	for key := range s.deadlines {
		if key[1] == location {
			delete(s.deadlines, key)
		}
	}
	s.effects = append(s.effects, BakalEffect{Op: "monster", Kind: m.Kind, Location: location, Value: 0})
	return s.presence(m.Kind, queue)
}

func (s *BakalOpening) executeScript(op catalog.BakalScriptInstruction, now time.Time, queue *[]BakalSignal) error {
	a := op.Args
	n := func(i int) uint32 { return uint32(a[i].Value) }
	schedule := func(delay uint32, ins catalog.BakalScriptInstruction) {
		s.scheduled = append(s.scheduled, BakalScheduled{now.Add(time.Duration(delay) * time.Second), ins})
	}
	switch op.Op {
	case "[SET]":
		value := a[len(a)-1].Value
		if len(a) == 3 {
			value = s.variables[a[0].Text] - value
		}
		return s.setScriptSymbol(a[0].Text, value, queue)
	case "[SET TIMER]":
		key := [2]uint32{n(0), n(1)}
		s.deadlines[key] = now.Add(time.Duration(n(2)) * time.Second)
		s.effects = append(s.effects, BakalEffect{Op: "timer", ID: key[0], Location: key[1], Value: a[2].Value})
	case "[CREATE MONSTER]":
		return s.createScriptMonster(n(0), a[1].Text, queue)
	case "[RESERVE CREATE MONSTER]", "[RESERVE DELETE MONSTER]":
		schedule(n(0), catalog.BakalScriptInstruction{Op: op.Op + " due", Args: []pvf.Token{a[1]}})
	case "[RESERVE CREATE MONSTER] due":
		kind := a[0].Text
		for _, m := range s.monsters {
			if m.Kind == kind {
				return nil
			}
		}
		var candidates []int
		for id, loc := range s.rules.Locations {
			if _, occupied := s.monsters[id]; occupied {
				continue
			}
			for _, k := range loc.Creatable {
				if k == kind {
					candidates = append(candidates, int(id))
					break
				}
			}
		}
		if len(candidates) == 0 {
			return nil
		}
		sort.Ints(candidates)
		return s.createScriptMonster(uint32(candidates[s.randomIndex(len(candidates))]), kind, queue)
	case "[RESERVE DELETE MONSTER] due":
		var ids []int
		for id, m := range s.monsters {
			if m.Kind == a[0].Text {
				ids = append(ids, int(id))
			}
		}
		sort.Ints(ids)
		for _, id := range ids {
			if err := s.deleteScriptMonster(uint32(id), queue); err != nil {
				return err
			}
		}
	case "[DELETE MONSTER]":
		return s.deleteScriptMonster(n(0), queue)
	case "[MOVE MONSTER]":
		m, exists := s.monsters[n(0)]
		if !exists {
			return nil
		}
		// MOVABLE LOCATION describes player navigation. Scripted reinforcements
		// have their own explicit MOVE MONSTER route (including non-player edges).
		if _, exists := s.rules.Locations[n(1)]; !exists {
			return fmt.Errorf("missing source monster move destination")
		}
		if _, occupied := s.monsters[n(1)]; occupied {
			return nil
		}
		delete(s.monsters, n(0))
		s.monsters[n(1)] = m
		if hp, exists := s.monsterHealth[n(0)]; exists {
			delete(s.monsterHealth, n(0))
			s.monsterHealth[n(1)] = hp
		}
		s.effects = append(s.effects, BakalEffect{Op: "monster", Kind: m.Kind, Location: n(0), Value: 0}, BakalEffect{Op: "monster", Kind: m.Kind, Location: n(1), Value: 1})
	case "[CREATE RANDOM MONSTER]":
		if a[2].Value <= 0 || a[4].Value <= 0 {
			return fmt.Errorf("invalid native monster weights")
		}
		kind := a[1].Text
		if s.randomIndex(int(a[2].Value+a[4].Value)) >= int(a[2].Value) {
			kind = a[3].Text
		}
		return s.createScriptMonster(n(0), kind, queue)
	case "[AWAKE RANDOM DRAGON]":
		var options []catalog.BakalScriptInstruction
		for _, r := range s.rules.Events {
			if len(r.Trigger) < 3 || r.Trigger[0].Op != "[CHECK TIMER END]" || len(r.Behavior) != 1 || r.Behavior[0].Op != "[SET]" {
				continue
			}
			set := r.Behavior[0]
			if len(set.Args) != 2 || set.Args[1].Value != 1 || !strings.HasSuffix(set.Args[0].Text, " AWAKEN]") || s.variables[set.Args[0].Text] != 0 {
				continue
			}
			timer := r.Trigger[0]
			key := [2]uint32{uint32(timer.Args[0].Value), uint32(timer.Args[1].Value)}
			if _, already := s.deadlines[key]; already {
				continue
			}
			if s.variables[r.Trigger[2].Args[0].Text] <= 0 {
				continue
			}
			options = append(options, catalog.BakalScriptInstruction{Op: "[SET TIMER]", Args: []pvf.Token{timer.Args[0], timer.Args[1], a[0]}})
		}
		if len(options) > 0 {
			return s.executeScript(options[s.randomIndex(len(options))], now, queue)
		}
	case "[INCREASE BAKAL ANGER]":
		if loc, exists := s.rules.Locations[n(0)]; exists && loc.Dungeon == s.currentDungeon {
			return nil
		}
		return s.setScriptSymbol("[BAKAL ANGER]", s.variables["[BAKAL ANGER]"]+a[1].Value, queue)
	case "[SUB MAX]":
		first, last := s.rules.Symbols[a[2].Text], s.rules.Symbols[a[3].Text]
		if first == 0 || last < first {
			return fmt.Errorf("invalid source damage symbol range")
		}
		value := int64(s.variables[a[0].Text])
		for name, id := range s.rules.Symbols {
			if id >= first && id <= last {
				value -= int64(s.variables[name])
			}
		}
		if value < 0 {
			value = 0
		}
		return s.setScriptSymbol(a[1].Text, int32(value), queue)
	case "[SET DUNGEON STATE]":
		if _, exists := s.dungeons[n(0)]; !exists {
			return fmt.Errorf("unknown source dungeon state")
		}
		state := a[1].Text
		if state != "open" && state != "close" && state != "clear" {
			return fmt.Errorf("unsupported source dungeon state %s", state)
		}
		if s.dungeons[n(0)] != state {
			s.dungeons[n(0)] = state
			s.effects = append(s.effects, BakalEffect{Op: "dungeon", ID: n(0), Kind: state})
			*queue = append(*queue, BakalSignal{Op: "[ON CHANGE DUNGEON STATE]", ID: n(0)})
		}
	case "[ADD BAKAL RAID BUFF]":
		if n(0) >= 5 {
			return fmt.Errorf("native raid buff inventory index")
		}
		s.raidBuffs[n(0)] += n(1)
		s.effects = append(s.effects, BakalEffect{Op: "buff", ID: n(0), Value: a[1].Value})
	case "[KICK OUT DUNGEON NO PENALTY]":
		s.effects = append(s.effects, BakalEffect{Op: "kick", ID: n(0)})
		if s.currentDungeon == n(0) {
			s.currentDungeon = 0
			*queue = append(*queue, BakalSignal{Op: "[ON GIVEUP DUNGEON]", ID: n(0)})
		}
	case "[MOVE LAST BAKAL DUNGEON]":
		var last uint32
		for _, r := range s.rules.Events {
			for _, ins := range r.Behavior {
				if ins.Op == "[KICK OUT DUNGEON NO PENALTY]" {
					for _, t := range r.Trigger {
						if t.Op == "[CHECK TIMER END]" {
							if loc, ok := s.rules.Locations[uint32(t.Args[1].Value)]; ok && loc.Dungeon == uint32(ins.Args[0].Value) {
								last = loc.Dungeon
							}
						}
					}
				}
			}
		}
		if last == 0 {
			return fmt.Errorf("native last dungeon unresolved")
		}
		// MOVE LAST is a native phase transition, not only a map change.
		// The 115 client opens LastPhase/skip only when source symbol
		// [BAKAL PHASE] (230) becomes 2 (14254ebbd -> 14254edb4).
		if err := s.setScriptSymbol("[BAKAL PHASE]", 2, queue); err != nil {
			return err
		}
		s.effects = append(s.effects, BakalEffect{Op: "last", ID: last})
	case "[CLEAR PHASE]":
		s.ended = "clear"
		s.effects = append(s.effects, BakalEffect{Op: "clear"})
	case "[FAIL PHASE]":
		s.ended = "fail"
		s.effects = append(s.effects, BakalEffect{Op: "fail"})
	default:
		return fmt.Errorf("unimplemented source Bakal operation %s", op.Op)
	}
	return nil
}

func (s *BakalOpening) Tick(now time.Time) ([]BakalEffect, error) {
	if s == nil || !s.active || s.ended != "" {
		return nil, nil
	}
	c := s.cloneScript()
	for steps := 0; ; steps++ {
		if steps > 8192 {
			return nil, fmt.Errorf("Bakal delayed event backlog exceeds bound")
		}
		next := now.Add(time.Nanosecond)
		var timer [2]uint32
		scheduled := -1
		for key, at := range c.deadlines {
			if at.Before(next) || at.Equal(next) && (key[0] < timer[0] || key[0] == timer[0] && key[1] < timer[1]) {
				next = at
				timer = key
				scheduled = -1
			}
		}
		for i, task := range c.scheduled {
			if task.At.Before(next) {
				next = task.At
				scheduled = i
			}
		}
		if next.After(now) {
			break
		}
		if err := c.advanceAnger(next); err != nil {
			return nil, err
		}
		if c.ended != "" {
			break
		}
		if scheduled >= 0 {
			ins := c.scheduled[scheduled].Instruction
			c.scheduled = append(c.scheduled[:scheduled], c.scheduled[scheduled+1:]...)
			var queue []BakalSignal
			if err := c.executeScript(ins, next, &queue); err != nil {
				return nil, err
			}
			for _, sig := range queue {
				if err := c.dispatchScript(sig, next); err != nil {
					return nil, err
				}
			}
		} else {
			delete(c.deadlines, timer)
			if err := c.dispatchScript(BakalSignal{Op: "[CHECK TIMER END]", ID: timer[0], Location: timer[1]}, next); err != nil {
				return nil, err
			}
		}
		if c.ended != "" {
			break
		}
	}
	if c.ended == "" {
		if err := c.advanceAnger(now); err != nil {
			return nil, err
		}
	}
	if c.Remaining(now) == 0 && c.ended == "" {
		c.ended = "fail"
		c.effects = append(c.effects, BakalEffect{Op: "fail"})
	}
	*s = *c
	return s.DrainEffects(), nil
}

func (s *BakalOpening) advanceAnger(now time.Time) error {
	seconds := int64(now.Sub(s.clock) / time.Second)
	if seconds <= 0 {
		return nil
	}
	var queue []BakalSignal
	value := int64(s.variables["[BAKAL ANGER]"]) + seconds*int64(s.variables["[BAKAL ANGER PER SECOND]"])
	if value > 2147483647 {
		value = 2147483647
	}
	if err := s.setScriptSymbol("[BAKAL ANGER]", int32(value), &queue); err != nil {
		return err
	}
	for _, sig := range queue {
		if err := s.dispatchScript(sig, now); err != nil {
			return err
		}
	}
	s.clock = s.clock.Add(time.Duration(seconds) * time.Second)
	return nil
}

func (s *BakalOpening) DrainEffects() []BakalEffect { out := s.effects; s.effects = nil; return out }
func (s *BakalOpening) Monsters() map[uint32]catalog.BakalRaidMonster {
	out := map[uint32]catalog.BakalRaidMonster{}
	for k, v := range s.monsters {
		out[k] = v
	}
	return out
}
func (s *BakalOpening) Ended() string {
	if s == nil {
		return ""
	}
	return s.ended
}

func (s *BakalOpening) GiveupDungeon(id uint32, now time.Time) error {
	if s == nil || !s.active || s.currentDungeon != id {
		return fmt.Errorf("giveup outside current native dungeon")
	}
	c := s.cloneScript()
	c.currentDungeon = 0
	if err := c.dispatchScript(BakalSignal{Op: "[ON GIVEUP DUNGEON]", ID: id}, now); err != nil {
		return err
	}
	*s = *c
	return nil
}

func (s *BakalOpening) DefeatMonster(slot, template uint32, now time.Time) error {
	m, exists := s.monsters[slot]
	if !s.active || s.ended != "" || !exists || m.ID != template && m.SecondID != template {
		return fmt.Errorf("defeat does not own native monster")
	}
	loc := s.rules.Locations[slot]
	if s.currentDungeon != loc.Dungeon {
		return fmt.Errorf("defeat outside current battlefield")
	}
	c := s.cloneScript()
	var queue []BakalSignal
	if m.SecondID != 0 && template == m.ID {
		if c.rules.Stage.HealthPercent > 0 && (c.variables["[BAKAL HP UNLOCK GRADE]"] >= c.rules.Stage.UnlockGradeBelow || int64(c.variables["[BAKAL HP]"])*100 > int64(c.variables["[MONSTER MAX HP]"])*int64(c.rules.Stage.HealthPercent)) {
			return fmt.Errorf("Bakal first-stage report before native transition condition")
		}
		if err := c.setScriptSymbol("[BAKAL PHASE]", 1, &queue); err != nil {
			return err
		}
		for _, sig := range queue {
			if err := c.dispatchScript(sig, now); err != nil {
				return err
			}
		}
		*s = *c
		return nil
	}
	if err := c.deleteScriptMonster(slot, &queue); err != nil {
		return err
	}
	c.clearCounts[loc.Dungeon] = true
	if err := c.grantWeightedBuffs(m.Kind, m.BuffPool, m.BuffCount, now); err != nil {
		return err
	}
	if len(m.AdditionalBuffPool) > 0 {
		if err := c.grantWeightedBuffs(m.Kind, m.AdditionalBuffPool, 1, now); err != nil {
			return err
		}
	}
	for _, r := range c.rules.Events {
		for _, t := range r.Trigger {
			if t.Op == "[ON CHANGE DUNGEON STATE]" && uint32(t.Args[0].Value) == loc.Dungeon {
				if m.Kind == "bakal" && c.variables["[BAKAL HP UNLOCK GRADE]"] > 0 {
					return fmt.Errorf("Bakal death before native HP locks released")
				}
				c.dungeons[loc.Dungeon] = "clear"
				queue = append(queue, BakalSignal{Op: "[ON CHANGE DUNGEON STATE]", ID: loc.Dungeon})
				goto dispatch
			}
		}
	}
dispatch:
	for _, sig := range queue {
		if err := c.dispatchScript(sig, now); err != nil {
			return err
		}
	}
	*s = *c
	return nil
}

func (s *BakalOpening) DungeonStates() map[uint32]string {
	out := map[uint32]string{}
	for k, v := range s.dungeons {
		out[k] = v
	}
	return out
}

func (s *BakalOpening) FinalDungeon() uint32 {
	if s == nil || s.rules == nil {
		return 0
	}
	for _, r := range s.rules.Events {
		for _, t := range r.Trigger {
			if t.Op != "[CHECK TIMER END]" {
				continue
			}
			loc, exists := s.rules.Locations[uint32(t.Args[1].Value)]
			if !exists {
				continue
			}
			for _, op := range r.Behavior {
				if op.Op == "[CLEAR PHASE]" {
					return loc.Dungeon
				}
			}
		}
	}
	return 0
}

func (s *BakalOpening) SettlementIdentity() (string, uint32, bool) {
	if s == nil || s.ended != "clear" || s.startedAt.IsZero() {
		return "", 0, false
	}
	return fmt.Sprintf("%d:%d", s.roster[0].Actor, s.startedAt.UnixNano()), uint32(len(s.clearCounts)), true
}

func (s *BakalOpening) StageWarp(id uint32, from, to [2]byte) (catalog.BakalRaidMonster, bool) {
	if s == nil || s.rules == nil || s.ended != "" || s.currentDungeon != id || s.variables["[BAKAL PHASE]"] != 1 {
		return catalog.BakalRaidMonster{}, false
	}
	for slot, m := range s.monsters {
		loc := s.rules.Locations[slot]
		if m.SecondID == 0 || !m.HasSecondGrid || loc.Dungeon != id || !loc.HasSpecificGrid || from != loc.SpecificGrid || to != m.SecondGrid {
			continue
		}
		m.Grid = loc.SpecificGrid
		m.HasGrid = true
		return m, true
	}
	return catalog.BakalRaidMonster{}, false
}

func (s *BakalOpening) ClearFinalDungeon(id uint32, now time.Time) error {
	if s == nil || !s.active || s.currentDungeon != id || id != s.FinalDungeon() {
		return fmt.Errorf("final clear outside native source transition")
	}
	c := s.cloneScript()
	c.dungeons[id] = "clear"
	if err := c.dispatchScript(BakalSignal{Op: "[ON CHANGE DUNGEON STATE]", ID: id}, now); err != nil {
		return err
	}
	*s = *c
	return nil
}

func (s *BakalOpening) ReportCurrentDamage(value int32, now time.Time) error {
	for slot, m := range s.monsters {
		if s.rules.Locations[slot].Dungeon != s.currentDungeon {
			continue
		}
		name := fmt.Sprintf("[%s PARTY%d DAMAGE]", strings.ToUpper(m.Kind), s.roster[0].Position)
		id := s.rules.Symbols[name]
		if id != 0 {
			return s.ReportSymbol(id, value, now)
		}
	}
	return fmt.Errorf("current dungeon has no source shared-damage boss")
}

func (s *BakalOpening) ReportSymbol(symbol uint32, value int32, now time.Time) error {
	if s == nil || !s.active || value < 0 {
		return fmt.Errorf("invalid native symbol report")
	}
	var name string
	for k, v := range s.rules.Symbols {
		if v == symbol {
			name = k
			break
		}
	}
	var prefix string
	for slot, m := range s.monsters {
		if s.rules.Locations[slot].Dungeon == s.currentDungeon {
			prefix = strings.ToUpper(m.Kind)
			break
		}
	}
	owned := fmt.Sprintf("[%s PARTY%d DAMAGE]", prefix, s.roster[0].Position)
	if name != owned && !(prefix == "BAKAL" && name == "[BAKAL PHASE]" && value <= 1) {
		return fmt.Errorf("client cannot write this source raid symbol")
	}
	if strings.HasSuffix(name, " DAMAGE]") && (value < s.variables[name] || value > s.variables["[MONSTER MAX HP]"]) {
		return fmt.Errorf("invalid cumulative damage report")
	}
	c := s.cloneScript()
	var queue []BakalSignal
	if err := c.setScriptSymbol(name, value, &queue); err != nil {
		return err
	}
	for _, sig := range queue {
		if err := c.dispatchScript(sig, now); err != nil {
			return err
		}
	}
	*s = *c
	return nil
}
