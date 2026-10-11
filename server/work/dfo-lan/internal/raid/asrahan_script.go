package raid

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
	"sort"
	"time"
)

type AsrahanTimerKey struct{ Kind, Location uint32 }
type AsrahanSignal struct {
	Op, Symbol string
	Timer      AsrahanTimerKey
}
type AsrahanSymbolChange struct {
	Name  string
	Value int32
}
type AsrahanDungeonChange struct {
	ID    uint32
	State string
}
type AsrahanView struct {
	Run, SourceSHA256 string
	Hard, Cleared     bool
	Now, Deadline     time.Time
	CoinLimit         int64
	Symbols           map[string]int32
	Dungeons          map[uint32]string
	Timers            map[AsrahanTimerKey]time.Time
}
type AsrahanActionResult struct {
	Symbols  []AsrahanSymbolChange
	Dungeons []AsrahanDungeonChange
}

// Bindings prepare source actions against an owned snapshot. They must be
// pure: persistence and wire delivery occur only after the full event commits.
// Missing bindings fail construction, instead of making a partial raid run.
type AsrahanActionBinding func(AsrahanView, catalog.AsrahanInstruction) (AsrahanActionResult, error)
type asrahanCombatOwner struct {
	group catalog.AsrahanCombatGroup
	party int
}
type AsrahanScript struct {
	view        AsrahanView
	program     []catalog.AsrahanScriptEvent
	identities  map[string]uint32
	states      map[string]bool
	bindings    map[string]AsrahanActionBinding
	groups      map[uint32]catalog.AsrahanCombatGroup
	combats     map[string]asrahanCombatOwner
	usedCombats map[string]bool
	effects     []catalog.AsrahanInstruction
	started     bool
}

func asrahanCoreAction(op string) bool {
	switch op {
	case "[SET]", "[ASSIGN]", "[SUM]", "[SUB]", "[SUB MAX]", "[SET DUNGEON STATE]", "[SET TIMER]", "[RESET TIMER]", "[ADD REMAIN TIME]", "[ADD COIN LIMIT]", "[CLEAR PHASE]":
		return true
	}
	return false
}

func NewAsrahanScript(r *catalog.AsrahanRaidRules, hard bool, run string, now time.Time, bindings map[string]AsrahanActionBinding) (*AsrahanScript, error) {
	if r == nil || r.SHA256 == "" || run == "" || now.IsZero() {
		return nil, fmt.Errorf("Asrahan run/source/clock missing")
	}
	p := r.Normal
	if hard {
		p = r.Hard
	}
	program, err := catalog.CompileAsrahanPhase(r, p)
	if err != nil {
		return nil, err
	}
	s := &AsrahanScript{program: program, identities: map[string]uint32{}, states: map[string]bool{}, bindings: map[string]AsrahanActionBinding{}, groups: map[uint32]catalog.AsrahanCombatGroup{}, combats: map[string]asrahanCombatOwner{}, usedCombats: map[string]bool{}}
	if len(r.Groups) > 0 {
		groups, err := catalog.CompileAsrahanCombatGroups(r, p)
		if err != nil {
			return nil, err
		}
		for _, group := range groups {
			s.groups[group.Index] = group
		}
	}
	s.view = AsrahanView{Run: run, SourceSHA256: r.SHA256, Hard: hard, Now: now, Deadline: now.Add(time.Duration(p.TimeoutSeconds) * time.Second), CoinLimit: int64(p.CoinLimit), Symbols: map[string]int32{}, Dungeons: map[uint32]string{}, Timers: map[AsrahanTimerKey]time.Time{}}
	for name, id := range r.Symbols {
		s.identities[name] = id
		s.view.Symbols[name] = 0
	}
	for id, d := range p.Dungeons {
		s.view.Dungeons[id] = d.InitialState
		s.states[d.InitialState] = true
	}
	for _, event := range program {
		for _, action := range event.Behavior {
			if action.Op == "[SET DUNGEON STATE]" {
				s.states[action.Args[1].Text] = true
			}
			if !asrahanCoreAction(action.Op) {
				if bindings[action.Op] == nil {
					return nil, fmt.Errorf("Asrahan %s requires complete source action binding", action.Op)
				}
				s.bindings[action.Op] = bindings[action.Op]
			}
		}
	}
	return s, nil
}

func cloneAsrahanView(v AsrahanView) AsrahanView {
	c := v
	c.Symbols = make(map[string]int32, len(v.Symbols))
	for k, x := range v.Symbols {
		c.Symbols[k] = x
	}
	c.Dungeons = make(map[uint32]string, len(v.Dungeons))
	for k, x := range v.Dungeons {
		c.Dungeons[k] = x
	}
	c.Timers = make(map[AsrahanTimerKey]time.Time, len(v.Timers))
	for k, x := range v.Timers {
		c.Timers[k] = x
	}
	return c
}
func (s *AsrahanScript) View() AsrahanView { return cloneAsrahanView(s.view) }
func (s *AsrahanScript) stage() *AsrahanScript {
	c := *s
	c.view = s.View()
	c.effects = append([]catalog.AsrahanInstruction(nil), s.effects...)
	c.combats = make(map[string]asrahanCombatOwner, len(s.combats))
	for k, v := range s.combats {
		c.combats[k] = v
	}
	c.usedCombats = make(map[string]bool, len(s.usedCombats))
	for k, v := range s.usedCombats {
		c.usedCombats[k] = v
	}
	return &c
}
func (s *AsrahanScript) owned(run string, now time.Time) error {
	if s == nil || run == "" || run != s.view.Run || now.Before(s.view.Now) || s.view.Cleared {
		return fmt.Errorf("Asrahan signal outside owned active run")
	}
	if s.started && !now.Before(s.view.Deadline) {
		return fmt.Errorf("Asrahan owned run expired")
	}
	return nil
}
func (s *AsrahanScript) set(name string, value int64, queue *[]AsrahanSignal) error {
	before, exists := s.view.Symbols[name]
	if !exists || value < math.MinInt32 || value > math.MaxInt32 {
		return fmt.Errorf("invalid Asrahan source symbol assignment %s", name)
	}
	if before == int32(value) {
		return nil
	}
	s.view.Symbols[name] = int32(value)
	*queue = append(*queue, AsrahanSignal{Op: "[ON CHANGE SYMBOL]", Symbol: name})
	return nil
}
func (s *AsrahanScript) dungeon(id uint32, state string) error {
	if _, owned := s.view.Dungeons[id]; !owned {
		return fmt.Errorf("foreign Asrahan dungeon %d", id)
	}
	if !s.states[state] {
		return fmt.Errorf("unknown Asrahan dungeon state")
	}
	s.view.Dungeons[id] = state
	return nil
}
func (s *AsrahanScript) rangeSum(first, last string) (int64, error) {
	a, b := s.identities[first], s.identities[last]
	if a == 0 || b < a {
		return 0, fmt.Errorf("invalid Asrahan source symbol range")
	}
	var sum int64
	for name, id := range s.identities {
		if id >= a && id <= b {
			sum += int64(s.view.Symbols[name])
		}
	}
	return sum, nil
}
func (s *AsrahanScript) matches(trigger []catalog.AsrahanInstruction, signal AsrahanSignal) bool {
	selected := false
	for _, instruction := range trigger {
		a := instruction.Args
		switch instruction.Op {
		case "[ON CHANGE SYMBOL]":
			selected = true
			if signal.Op != instruction.Op || signal.Symbol != a[0].Text {
				return false
			}
		case "[CHECK TIMER END]":
			selected = true
			if signal.Op != instruction.Op || signal.Timer != (AsrahanTimerKey{uint32(a[0].Value), uint32(a[1].Value)}) {
				return false
			}
		case "[IF]":
			left, right := s.view.Symbols[a[0].Text], a[len(a)-1].Value
			pass := left == right
			if len(a) == 3 {
				switch a[1].Text {
				case "[!=]":
					pass = left != right
				case "[>]":
					pass = left > right
				case "[<]":
					pass = left < right
				case "[>=]":
					pass = left >= right
				case "[<=]":
					pass = left <= right
				}
			}
			if !pass {
				return false
			}
		default:
			return false
		}
	}
	return selected || signal.Op == "init"
}
func (s *AsrahanScript) execute(action catalog.AsrahanInstruction, queue *[]AsrahanSignal) error {
	a := action.Args
	v := func(i int) int64 { return int64(s.view.Symbols[a[i].Text]) }
	n := func(i int) int64 { return int64(a[i].Value) }
	var err error
	switch action.Op {
	case "[SET]":
		value := n(1)
		if len(a) == 3 {
			value = v(0) + v(2)
		}
		err = s.set(a[0].Text, value, queue)
	case "[ASSIGN]":
		err = s.set(a[0].Text, v(1), queue)
	case "[SUB]":
		err = s.set(a[0].Text, v(0)-v(1), queue)
	case "[SUM]":
		var total int64
		total, err = s.rangeSum(a[1].Text, a[2].Text)
		if err == nil {
			err = s.set(a[0].Text, total, queue)
		}
	case "[SUB MAX]":
		var total int64
		total, err = s.rangeSum(a[2].Text, a[3].Text)
		if err == nil {
			value := v(0) - total
			if value < 0 {
				value = 0
			}
			err = s.set(a[1].Text, value, queue)
		}
	case "[SET DUNGEON STATE]":
		err = s.dungeon(uint32(n(0)), a[1].Text)
	case "[SET TIMER]", "[RESET TIMER]":
		if n(0) < 0 || n(1) < 0 {
			return fmt.Errorf("invalid Asrahan timer identity")
		}
		key := AsrahanTimerKey{uint32(n(0)), uint32(n(1))}
		if action.Op == "[RESET TIMER]" {
			delete(s.view.Timers, key)
		} else {
			if n(2) < 0 {
				return fmt.Errorf("negative Asrahan source timer")
			}
			s.view.Timers[key] = s.view.Now.Add(time.Duration(n(2)) * time.Second)
		}
	case "[ADD REMAIN TIME]":
		s.view.Deadline = s.view.Deadline.Add(time.Duration(n(0)) * time.Second)
	case "[ADD COIN LIMIT]":
		if n(0) < 0 || s.view.CoinLimit+n(0) > math.MaxUint32 {
			return fmt.Errorf("invalid Asrahan coin limit change")
		}
		s.view.CoinLimit += n(0)
	case "[CLEAR PHASE]":
		s.view.Cleared = true
	default:
		binding := s.bindings[action.Op]
		if binding == nil {
			return fmt.Errorf("unbound Asrahan source action %s", action.Op)
		}
		copyAction := action
		copyAction.Args = append(copyAction.Args[:0:0], action.Args...)
		result, e := binding(s.View(), copyAction)
		if e != nil {
			return e
		}
		for _, c := range result.Symbols {
			if e = s.set(c.Name, int64(c.Value), queue); e != nil {
				return e
			}
		}
		for _, c := range result.Dungeons {
			if e = s.dungeon(c.ID, c.State); e != nil {
				return e
			}
		}
	}
	if err == nil {
		action.Args = append(action.Args[:0:0], action.Args...)
		s.effects = append(s.effects, action)
	}
	return err
}
func (s *AsrahanScript) dispatch(signals []AsrahanSignal) error {
	for count := 0; len(signals) != 0; count++ {
		if count >= 4096 {
			return fmt.Errorf("Asrahan source event cascade exceeded bound")
		}
		signal := signals[0]
		signals = signals[1:]
		for _, event := range s.program {
			if !s.matches(event.Trigger, signal) {
				continue
			}
			for _, action := range event.Behavior {
				if err := s.execute(action, &signals); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *AsrahanScript) Start(run string, now time.Time) error {
	if err := s.owned(run, now); err != nil {
		return err
	}
	if s.started {
		return fmt.Errorf("Asrahan source run already started")
	}
	c := s.stage()
	c.view.Now = now
	c.view.Deadline = c.view.Deadline.Add(now.Sub(s.view.Now))
	if err := c.dispatch([]AsrahanSignal{{Op: "init"}}); err != nil {
		return err
	}
	c.started = true
	*s = *c
	return nil
}

// OwnedSymbols is a server-adapter operation; it is never a client-supplied
// SET request. It commits the reports and their entire event cascade together.
func (s *AsrahanScript) OwnedSymbols(run string, now time.Time, changes []AsrahanSymbolChange) error {
	if err := s.owned(run, now); err != nil {
		return err
	}
	if !s.started {
		return fmt.Errorf("Asrahan reports before source initialization")
	}
	c := s.stage()
	c.view.Now = now
	var queue []AsrahanSignal
	for _, change := range changes {
		if err := c.set(change.Name, int64(change.Value), &queue); err != nil {
			return err
		}
	}
	if err := c.dispatch(queue); err != nil {
		return err
	}
	*s = *c
	return nil
}

// BeginCombat is called only after the adapter delivers a source-owned
// dungeon session. The distinct combat run prevents reports from a previous
// retreat/re-entry from modifying a new fight. No client command binds this API.
func (s *AsrahanScript) BeginCombat(run, combatRun string, now time.Time, groupIndex, dungeon, monster uint32, party int) error {
	if err := s.owned(run, now); err != nil {
		return err
	}
	g, exists := s.groups[groupIndex]
	if !s.started || !exists || combatRun == "" || s.usedCombats[combatRun] || g.Dungeon != dungeon || g.Monster != monster || party < 0 || party >= len(g.PartyDamage) || s.view.Dungeons[dungeon] != "open" {
		return fmt.Errorf("Asrahan combat outside source group/open dungeon ownership")
	}
	for _, owner := range s.combats {
		if owner.party == party {
			return fmt.Errorf("Asrahan party already owns an active combat run")
		}
	}
	s.combats[combatRun] = asrahanCombatOwner{g, party}
	s.usedCombats[combatRun] = true
	s.view.Now = now
	return nil
}

// VerifiedCombatDamage consumes server-verified cumulative damage in the
// source symbol's units. It does not infer C2190's unestablished ownership or
// accept a client clear flag. The source program computes HP and transitions.
func (s *AsrahanScript) VerifiedCombatDamage(run, combatRun string, now time.Time, cumulative int32) error {
	if err := s.owned(run, now); err != nil {
		return err
	}
	owner, exists := s.combats[combatRun]
	if !s.started || !exists || s.view.Dungeons[owner.group.Dungeon] != "open" || s.view.Symbols[owner.group.MaximumHP] <= 0 {
		return fmt.Errorf("Asrahan damage has no initialized owned combat")
	}
	symbol := owner.group.PartyDamage[owner.party]
	if cumulative < 0 || cumulative < s.view.Symbols[symbol] {
		return fmt.Errorf("Asrahan cumulative damage moved backwards")
	}
	return s.OwnedSymbols(run, now, []AsrahanSymbolChange{{symbol, cumulative}})
}

func (s *AsrahanScript) EndCombat(run, combatRun string, now time.Time) error {
	if err := s.owned(run, now); err != nil {
		return err
	}
	if _, exists := s.combats[combatRun]; !exists {
		return fmt.Errorf("Asrahan combat owner absent")
	}
	delete(s.combats, combatRun)
	s.view.Now = now
	return nil
}

// Each elapsed timer is removed before dispatch. A source reset/rearm inside
// the same tick invalidates the old due entry instead of firing it twice.
func (s *AsrahanScript) Tick(run string, now time.Time) error {
	if err := s.owned(run, now); err != nil {
		return err
	}
	if !s.started {
		return fmt.Errorf("Asrahan timers before source initialization")
	}
	c := s.stage()
	c.view.Now = now
	type dueTimer struct {
		key AsrahanTimerKey
		at  time.Time
	}
	var due []dueTimer
	for key, at := range c.view.Timers {
		if !now.Before(at) {
			due = append(due, dueTimer{key, at})
		}
	}
	sort.Slice(due, func(i, j int) bool {
		if !due[i].at.Equal(due[j].at) {
			return due[i].at.Before(due[j].at)
		}
		if due[i].key.Kind != due[j].key.Kind {
			return due[i].key.Kind < due[j].key.Kind
		}
		return due[i].key.Location < due[j].key.Location
	})
	for _, timer := range due {
		if c.view.Cleared {
			break
		}
		if at, exists := c.view.Timers[timer.key]; !exists || !at.Equal(timer.at) {
			continue
		}
		delete(c.view.Timers, timer.key)
		if err := c.dispatch([]AsrahanSignal{{Op: "[CHECK TIMER END]", Timer: timer.key}}); err != nil {
			return err
		}
	}
	*s = *c
	return nil
}

func (s *AsrahanScript) DrainEffects() []catalog.AsrahanInstruction {
	out := make([]catalog.AsrahanInstruction, len(s.effects))
	for i, action := range s.effects {
		out[i] = action
		out[i].Args = append(out[i].Args[:0:0], action.Args...)
	}
	s.effects = nil
	return out
}
