package legion

import (
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
	"fmt"
	"maps"
	"math/big"
	"sort"
	"strings"
	"time"
)

type bakalSignal struct {
	op, name string
	id, sub  uint32
}
type bakalTask struct {
	at time.Time
	op catalog.BakalScriptInstruction
}
type bakalRuntime struct {
	timers   map[[2]uint32]time.Time
	tasks    []bakalTask
	selected map[uint32][]string
	history  map[string]string
	party    map[string]time.Time
	cooldown map[string]time.Time
	states   map[uint32]string
	clock    time.Time
	frames   []BakalFrame
	boot     bool
	coins    int
	potions  int
}

func (r *bakalRuntime) clone() *bakalRuntime {
	c := *r
	c.timers = maps.Clone(r.timers)
	c.tasks = append([]bakalTask(nil), r.tasks...)
	c.selected = maps.Clone(r.selected)
	c.history = maps.Clone(r.history)
	c.party = maps.Clone(r.party)
	c.cooldown = maps.Clone(r.cooldown)
	c.states = maps.Clone(r.states)
	c.frames = nil
	return &c
}
func (o *BakalOpening) eventClone() *BakalOpening {
	c := *o
	c.runtime = o.runtime.clone()
	c.symbolValues = maps.Clone(o.symbolValues)
	c.placements = maps.Clone(o.placements)
	c.defeatedLocations = maps.Clone(o.defeatedLocations)
	c.cleared = maps.Clone(o.cleared)
	c.loaded = maps.Clone(o.loaded)
	return &c
}
func (o *BakalOpening) startRuntime(now time.Time) error {
	o.runtime = &bakalRuntime{timers: map[[2]uint32]time.Time{}, selected: map[uint32][]string{}, history: map[string]string{}, party: map[string]time.Time{}, cooldown: map[string]time.Time{}, states: map[uint32]string{}, clock: now, boot: true}
	o.runtime.coins = o.rules.PartyCoinLimit
	o.runtime.potions = o.rules.GuaranteePartyPotion
	o.symbolValues = map[string]int32{}
	o.buffs = [5]byte{}
	for name := range o.rules.Symbols {
		o.symbolValues[name] = 0
	}
	o.placements = map[uint32]string{}
	for _, d := range o.rules.Dungeons {
		o.runtime.states[d.Index] = d.InitialState
	}
	if err := o.dispatchEvent(bakalSignal{op: "init"}, now); err != nil {
		return err
	}
	o.runtime.boot = false
	return nil
}
func (o *BakalOpening) eventFrames() []BakalFrame {
	out := o.runtime.frames
	o.runtime.frames = nil
	return out
}
func (o *BakalOpening) runEvent(signal bakalSignal, now time.Time) ([]BakalFrame, error) {
	c := o.eventClone()
	if err := c.dispatchEvent(signal, now); err != nil {
		return nil, err
	}
	*o = *c
	return o.eventFrames(), nil
}
func (o *BakalOpening) matchEvent(rule catalog.BakalScriptEvent, s bakalSignal) bool {
	selector := false
	for _, ins := range rule.Trigger {
		a := ins.Args
		switch ins.Op {
		case "[IF]":
			v := o.symbolValues[a[0].Text]
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
			if o.runtime.states[uint32(a[0].Value)] != a[1].Text {
				return false
			}
		case "[ON ENTER DUNGEON]", "[ON GIVEUP DUNGEON]", "[ON CHANGE DUNGEON STATE]":
			selector = true
			if s.op != ins.Op || s.id != uint32(a[0].Value) {
				return false
			}
		case "[ON CHANGE SYMBOL]":
			selector = true
			if s.op != ins.Op || s.name != a[0].Text {
				return false
			}
		case "[CHECK TIMER END]":
			selector = true
			if s.op != ins.Op || s.id != uint32(a[0].Value) || s.sub != uint32(a[1].Value) {
				return false
			}
		default:
			return false
		}
	}
	return selector || s.op == "init"
}
func (o *BakalOpening) dispatchEvent(signal bakalSignal, now time.Time) error {
	queue := []bakalSignal{signal}
	for steps := 0; len(queue) > 0; steps++ {
		if steps >= 4096 {
			return fmt.Errorf("Bakal source cascade exceeded bound")
		}
		s := queue[0]
		queue = queue[1:]
		for _, rule := range o.script.Events {
			if !o.matchEvent(rule, s) {
				continue
			}
			for _, ins := range rule.Behavior {
				if err := o.executeEvent(ins, now, &queue); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func (o *BakalOpening) setEventSymbol(name string, value int32, q *[]bakalSignal) error {
	if _, ok := o.rules.Symbols[name]; !ok {
		return fmt.Errorf("unknown source symbol %s", name)
	}
	if o.symbolValues[name] == value && !o.runtime.boot {
		return nil
	}
	o.symbolValues[name] = value
	o.runtime.frames = append(o.runtime.frames, o.SymbolFrame(name, value)...)
	*q = append(*q, bakalSignal{op: "[ON CHANGE SYMBOL]", name: name})
	if name == "[BAKAL ANGER]" {
		o.anger = int(value)
	}
	if name == "[BAKAL ANGER PER SECOND]" {
		o.angerRate = int(value)
	}
	if strings.HasSuffix(name, " HP]") {
		for loc, kind := range o.placements {
			if name == "["+strings.ToUpper(kind)+" HP]" {
				row := o.eventMonsterRow(loc)
				row.Count = 2
				row.MaxHP = uint32(max(0, value))
				o.runtime.frames = append(o.runtime.frames, BakalFrame{"bakal_script_actor_health", 2286, BakalMonstersFrame([]BakalMonster{row})})
			}
		}
	}
	return nil
}
func (o *BakalOpening) eventPresence(kind string, q *[]bakalSignal) error {
	name := "[IS EXIST " + strings.ToUpper(kind) + "]"
	if _, ok := o.rules.Symbols[name]; !ok {
		return nil
	}
	v := int32(0)
	for _, k := range o.placements {
		if k == kind {
			v = 1
			break
		}
	}
	return o.setEventSymbol(name, v, q)
}
func (o *BakalOpening) eventMonsterRow(loc uint32) BakalMonster {
	row := BakalMonster{Slot: BakalMonsterType(o.placements[loc]), Location: loc, MaxHP: uint32(o.script.MonsterMaxHP), HasBuffs: true, Buffs: [3]byte{25, 25, 25}}
	for i, name := range o.runtime.selected[loc] {
		if i >= len(row.Buffs) {
			break
		}
		id, _ := BakalBuffType(name)
		row.Buffs[i] = byte(id)
	}
	if name := "[" + strings.ToUpper(o.placements[loc]) + " HP]"; o.rules.Symbols[name] != 0 {
		row.MaxHP = uint32(max(0, o.symbolValues[name]))
		if o.runtime.boot {
			row.MaxHP = uint32(max(0, o.script.InitialSymbols[name]))
		}
	}
	return row
}
func (o *BakalOpening) eventRoster() []BakalMonster {
	var rows []BakalMonster
	for _, p := range o.MonsterPlacements() {
		rows = append(rows, o.eventMonsterRow(uint32(p.Location)))
	}
	return rows
}
func drawBakal(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("empty source random pool")
	}
	v, e := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if e != nil {
		return 0, e
	}
	return int(v.Int64()), nil
}
func (o *BakalOpening) chooseBuffs(kind string, pool []catalog.BakalWeightedBuff, count int) ([]string, error) {
	pool = append([]catalog.BakalWeightedBuff(nil), pool...)
	var chosen []string
	excluded := o.runtime.history[o.rules.Monsters[kind].BuffCheck]
	for count > 0 && len(pool) > 0 {
		total := 0
		for _, b := range pool {
			if b.Kind != excluded {
				total += b.Weight
			}
		}
		if total == 0 {
			break
		}
		roll, e := drawBakal(total)
		if e != nil {
			return nil, e
		}
		at := -1
		for i, b := range pool {
			if b.Kind == excluded {
				continue
			}
			if roll < b.Weight {
				at = i
				break
			}
			roll -= b.Weight
		}
		if at < 0 {
			return nil, fmt.Errorf("invalid source weights")
		}
		chosen = append(chosen, pool[at].Kind)
		pool = append(pool[:at], pool[at+1:]...)
		count--
	}
	return chosen, nil
}
func (o *BakalOpening) createEventMonster(loc uint32, kind string, q *[]bakalSignal) error {
	m, ok := o.rules.Monsters[kind]
	if !ok {
		return fmt.Errorf("missing source monster %s", kind)
	}
	valid := false
	for _, l := range o.rules.Locations {
		valid = valid || uint32(l.Index) == loc
	}
	if !valid {
		return fmt.Errorf("missing source location %d", loc)
	}
	if _, occupied := o.placements[loc]; occupied {
		return nil
	}
	count, pool, extra := m.BuffCount, m.BuffPool, m.AdditionalBuffPool
	if o.hard {
		count, pool, extra = m.HardBuffCount, m.HardBuffPool, m.HardAdditionalBuffPool
	}
	picked, e := o.chooseBuffs(kind, pool, count)
	if e != nil {
		return e
	}
	if len(picked) > 0 {
		o.runtime.history[kind] = picked[0]
	}
	additional, e := o.chooseBuffs(kind, extra, 1)
	if e != nil {
		return e
	}
	picked = append(picked, additional...)
	if len(picked) > 3 {
		return fmt.Errorf("source buff display exceeds native capacity")
	}
	for _, name := range picked {
		if _, ok := o.rules.BuffDefinitions[name]; !ok {
			return fmt.Errorf("source buff definition missing %s", name)
		}
		if _, ok := BakalBuffType(name); !ok {
			return fmt.Errorf("unbound native buff %s", name)
		}
	}
	o.placements[loc] = kind
	delete(o.defeatedLocations, loc)
	o.runtime.selected[loc] = picked
	o.runtime.frames = append(o.runtime.frames, BakalFrame{"bakal_source_monster_created", 2286, BakalMonstersFrame([]BakalMonster{o.eventMonsterRow(loc)})})
	return o.eventPresence(kind, q)
}
func (o *BakalOpening) deleteEventMonster(loc uint32, q *[]bakalSignal) error {
	kind, ok := o.placements[loc]
	if !ok {
		return nil
	}
	delete(o.placements, loc)
	delete(o.runtime.selected, loc)
	for key := range o.runtime.timers {
		if key[1] == loc {
			delete(o.runtime.timers, key)
		}
	}
	o.runtime.frames = append(o.runtime.frames, BakalFrame{"bakal_source_monster_removed", 2286, BakalMonstersFrame([]BakalMonster{BakalDefeatRow(loc)})})
	return o.eventPresence(kind, q)
}
func (o *BakalOpening) executeEvent(ins catalog.BakalScriptInstruction, now time.Time, q *[]bakalSignal) error {
	a := ins.Args
	n := func(i int) uint32 { return uint32(a[i].Value) }
	switch ins.Op {
	case "[SET]":
		v := a[len(a)-1].Value
		if len(a) == 3 {
			v = o.symbolValues[a[0].Text] - v
		}
		return o.setEventSymbol(a[0].Text, v, q)
	case "[SET TIMER]":
		key := [2]uint32{n(0), n(1)}
		if a[2].Value < 0 {
			return fmt.Errorf("negative source timer")
		}
		o.runtime.timers[key] = now.Add(time.Duration(a[2].Value) * time.Second)
	case "[CREATE MONSTER]":
		return o.createEventMonster(n(0), a[1].Text, q)
	case "[RESERVE CREATE MONSTER]", "[RESERVE DELETE MONSTER]":
		if a[0].Value < 0 {
			return fmt.Errorf("negative source reservation")
		}
		o.runtime.tasks = append(o.runtime.tasks, bakalTask{now.Add(time.Duration(a[0].Value) * time.Second), catalog.BakalScriptInstruction{Op: ins.Op + " due", Args: []pvf.Token{a[1]}}})
	case "[RESERVE CREATE MONSTER] due":
		kind := a[0].Text
		for _, k := range o.placements {
			if k == kind {
				return nil
			}
		}
		var candidates []uint32
		for _, l := range o.rules.Locations {
			if _, busy := o.placements[uint32(l.Index)]; busy {
				continue
			}
			for _, k := range l.Creatable {
				if k == kind {
					candidates = append(candidates, uint32(l.Index))
					break
				}
			}
		}
		if len(candidates) == 0 {
			return nil
		}
		sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
		at, e := drawBakal(len(candidates))
		if e != nil {
			return e
		}
		return o.createEventMonster(candidates[at], kind, q)
	case "[RESERVE DELETE MONSTER] due":
		var ids []uint32
		for loc, k := range o.placements {
			if k == a[0].Text {
				ids = append(ids, loc)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, loc := range ids {
			if e := o.deleteEventMonster(loc, q); e != nil {
				return e
			}
		}
	case "[DELETE MONSTER]":
		return o.deleteEventMonster(n(0), q)
	case "[MOVE MONSTER]":
		kind, ok := o.placements[n(0)]
		if !ok {
			return nil
		}
		allowed := false
		for _, l := range o.rules.Locations {
			allowed = allowed || uint32(l.Index) == n(1)
		}
		if !allowed {
			return fmt.Errorf("source monster move outside native route")
		}
		if _, busy := o.placements[n(1)]; busy {
			return nil
		}
		picked := o.runtime.selected[n(0)]
		if e := o.deleteEventMonster(n(0), q); e != nil {
			return e
		}
		o.placements[n(1)] = kind
		delete(o.defeatedLocations, n(1))
		o.runtime.selected[n(1)] = picked
		o.runtime.frames = append(o.runtime.frames, BakalFrame{"bakal_source_monster_moved", 2286, BakalMonstersFrame([]BakalMonster{o.eventMonsterRow(n(1))})})
		return o.eventPresence(kind, q)
	case "[CREATE RANDOM MONSTER]":
		total := int(a[2].Value + a[4].Value)
		if a[2].Value <= 0 || a[4].Value <= 0 {
			return fmt.Errorf("invalid source monster weights")
		}
		at, e := drawBakal(total)
		if e != nil {
			return e
		}
		kind := a[1].Text
		if at >= int(a[2].Value) {
			kind = a[3].Text
		}
		return o.createEventMonster(n(0), kind, q)
	case "[AWAKE RANDOM DRAGON]":
		var choices []catalog.BakalScriptInstruction
		for _, rule := range o.script.Events {
			if len(rule.Trigger) < 3 || rule.Trigger[0].Op != "[CHECK TIMER END]" || len(rule.Behavior) != 1 {
				continue
			}
			set := rule.Behavior[0]
			if set.Op != "[SET]" || len(set.Args) != 2 || set.Args[1].Value != 1 || !strings.HasSuffix(set.Args[0].Text, " AWAKEN]") || o.symbolValues[set.Args[0].Text] != 0 {
				continue
			}
			timer := rule.Trigger[0]
			key := [2]uint32{uint32(timer.Args[0].Value), uint32(timer.Args[1].Value)}
			if _, reserved := o.runtime.timers[key]; reserved {
				continue
			}
			hp := strings.TrimSuffix(set.Args[0].Text, " AWAKEN]") + " HP]"
			if o.symbolValues[hp] <= 0 {
				continue
			}
			choices = append(choices, catalog.BakalScriptInstruction{Op: "[SET TIMER]", Args: []pvf.Token{timer.Args[0], timer.Args[1], a[0]}})
		}
		if len(choices) > 0 {
			at, e := drawBakal(len(choices))
			if e != nil {
				return e
			}
			return o.executeEvent(choices[at], now, q)
		}
	case "[INCREASE BAKAL ANGER]":
		for _, l := range o.rules.Locations {
			if uint32(l.Index) == n(0) && l.Dungeon == o.current {
				return nil
			}
		}
		return o.setEventSymbol("[BAKAL ANGER]", o.symbolValues["[BAKAL ANGER]"]+a[1].Value, q)
	case "[SUB MAX]":
		value := int64(o.symbolValues[a[0].Text])
		first, last := o.rules.Symbols[a[2].Text], o.rules.Symbols[a[3].Text]
		if first == 0 || last < first {
			return fmt.Errorf("invalid native damage range")
		}
		for name, id := range o.rules.Symbols {
			if id >= first && id <= last {
				value -= int64(o.symbolValues[name])
			}
		}
		return o.setEventSymbol(a[1].Text, int32(max(0, value)), q)
	case "[SET DUNGEON STATE]":
		if _, ok := o.runtime.states[n(0)]; !ok {
			return fmt.Errorf("missing source dungeon state")
		}
		if o.runtime.states[n(0)] != a[1].Text {
			o.runtime.states[n(0)] = a[1].Text
			if a[1].Text == "clear" {
				o.cleared[n(0)] = true
			}
			o.runtime.frames = append(o.runtime.frames, BakalFrame{"bakal_script_dungeon_states", 572, BakalDungeonStateFrame(n(0))})
			*q = append(*q, bakalSignal{op: "[ON CHANGE DUNGEON STATE]", id: n(0)})
		}
	case "[ADD BAKAL RAID BUFF]":
		if n(0) >= 5 || a[1].Value < 0 || int(o.buffs[n(0)])+int(a[1].Value) > 255 {
			return fmt.Errorf("native buff inventory range")
		}
		o.buffs[n(0)] += byte(a[1].Value)
	case "[KICK OUT DUNGEON NO PENALTY]":
		if o.current == n(0) {
			old := o.current
			o.current = 0
			o.location = BakalCampLocation
			o.campReturnPending = true
			o.guaranteeCampBudget()
			*q = append(*q, bakalSignal{op: "[ON GIVEUP DUNGEON]", id: old})
		}
	case "[MOVE LAST BAKAL DUNGEON]":
		o.stage = BakalOpeningFinal
		o.settleAt = now.Add(time.Duration(o.script.SettlementTimer.Secs) * time.Second)
		return o.setEventSymbol("[BAKAL PHASE]", 2, q)
	case "[CLEAR PHASE]": // Durable settlement publishes the ended state, never source alone.
	case "[FAIL PHASE]":
		o.stage = BakalOpeningFailed
		o.runtime.frames = append(o.runtime.frames, BakalFrame{"bakal_script_phase_ended", 574, []byte(BakalRaidStateEnded)})
	default:
		return fmt.Errorf("unsupported compiled source action %s", ins.Op)
	}
	return nil
}
func (o *BakalOpening) advanceEventAnger(now time.Time) error {
	seconds := int64(now.Sub(o.runtime.clock) / time.Second)
	if seconds <= 0 {
		return nil
	}
	value := int64(o.anger) + seconds*int64(o.angerRate)
	value = min(value, int64(2147483647))
	q := []bakalSignal{}
	if e := o.setEventSymbol("[BAKAL ANGER]", int32(value), &q); e != nil {
		return e
	}
	o.runtime.clock = o.runtime.clock.Add(time.Duration(seconds) * time.Second)
	for _, s := range q {
		if e := o.dispatchEvent(s, now); e != nil {
			return e
		}
	}
	return nil
}
func (o *BakalOpening) tickEvents(now time.Time) ([]BakalFrame, error) {
	c := o.eventClone()
	for steps := 0; ; steps++ {
		if steps >= 8192 {
			return nil, fmt.Errorf("Bakal source backlog exceeded bound")
		}
		next := now.Add(time.Nanosecond)
		key := [2]uint32{}
		task := -1
		found := false
		for k, at := range c.runtime.timers {
			if at.Before(next) || at.Equal(next) && (k[0] < key[0] || k[0] == key[0] && k[1] < key[1]) {
				next, key, task, found = at, k, -1, true
			}
		}
		for i, t := range c.runtime.tasks {
			if t.at.Before(next) {
				next, task, found = t.at, i, true
			}
		}
		if !found || next.After(now) {
			break
		}
		if e := c.advanceEventAnger(next); e != nil {
			return nil, e
		}
		if c.stage == BakalOpeningFailed {
			break
		}
		var e error
		if task >= 0 {
			ins := c.runtime.tasks[task].op
			c.runtime.tasks = append(c.runtime.tasks[:task], c.runtime.tasks[task+1:]...)
			q := []bakalSignal{}
			e = c.executeEvent(ins, next, &q)
			if e == nil {
				for _, s := range q {
					if e = c.dispatchEvent(s, next); e != nil {
						break
					}
				}
			}
		} else {
			delete(c.runtime.timers, key)
			e = c.dispatchEvent(bakalSignal{op: "[CHECK TIMER END]", id: key[0], sub: key[1]}, next)
		}
		if e != nil {
			return nil, e
		}
		if c.stage == BakalOpeningFailed {
			break
		}
	}
	if c.stage == BakalOpeningActive {
		if e := c.advanceEventAnger(now); e != nil {
			return nil, e
		}
	}
	expired := false
	for name, at := range c.runtime.party {
		if !at.After(now) {
			delete(c.runtime.party, name)
			expired = true
		}
	}
	if expired {
		c.runtime.frames = append(c.runtime.frames, BakalFrame{"bakal_party_buff_expired", 2285, c.PartyFrame(c.location, now)})
	}
	*o = *c
	return o.eventFrames(), nil
}
func (o *BakalOpening) PartyFrame(loc uint32, now time.Time) []byte {
	if o.runtime == nil {
		return BakalPartyFrame(loc, nil)
	}
	var entries []BakalPartyLocation
	for name, at := range o.runtime.party {
		if at.After(now) {
			id, ok := BakalBuffType(name)
			if ok {
				entries = append(entries, BakalPartyLocation{Index: id, Counter: uint32(at.Unix())})
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Index < entries[j].Index })
	body := BakalPartyFrame(loc, entries)
	binary.LittleEndian.PutUint32(body[9:], uint32(o.runtime.coins))
	return body
}
func (o *BakalOpening) grantEventBuff(name string, now time.Time) error {
	d, ok := o.rules.BuffDefinitions[name]
	if !ok {
		return fmt.Errorf("missing source buff %s", name)
	}
	id, ok := BakalBuffType(name)
	if !ok {
		return fmt.Errorf("unbound native buff %s", name)
	}
	if d.Raid {
		if id >= 5 || o.buffs[id] == 255 {
			return fmt.Errorf("native raid buff inventory full")
		}
		o.buffs[id]++
		return nil
	}
	if name == "AddRandomRaidBuff" {
		var ids []uint32
		for name, d := range o.rules.BuffDefinitions {
			if d.Raid {
				id, ok := BakalBuffType(name)
				if ok {
					ids = append(ids, id)
				}
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		at, e := drawBakal(len(ids))
		if e != nil {
			return e
		}
		id = ids[at]
		if int(o.buffs[id])+d.ServerValue > 255 {
			return fmt.Errorf("raid buff overflow")
		}
		o.buffs[id] += byte(d.ServerValue)
		return nil
	}
	if name == "StackablePotion" {
		o.runtime.potions += d.ServerValue
	}
	duration := d.Duration
	if duration == 0 {
		duration = d.Cooltime
	}
	o.runtime.party[name] = now.Add(time.Duration(duration) * time.Second)
	return nil
}
func (o *BakalOpening) eventDefeat(dungeon, loc uint32, now time.Time) ([]BakalFrame, error) {
	c := o.eventClone()
	kind, exists := c.placements[loc]
	if !exists {
		return nil, nil
	}
	for _, name := range c.runtime.selected[loc] {
		if e := c.grantEventBuff(name, now); e != nil {
			return nil, e
		}
	}
	q := []bakalSignal{}
	if e := c.deleteEventMonster(loc, &q); e != nil {
		return nil, e
	}
	c.defeatedLocations[loc] = true
	if c.bossTypes[dungeon] != "" {
		q = append(q, bakalSignal{op: "[ON CHANGE DUNGEON STATE]", id: dungeon})
		c.runtime.states[dungeon] = "clear"
		c.cleared[dungeon] = true
		c.runtime.frames = append(c.runtime.frames, BakalFrame{"bakal_script_dungeon_states", 572, BakalDungeonStateFrame(dungeon)})
	}
	for _, s := range q {
		if e := c.dispatchEvent(s, now); e != nil {
			return nil, e
		}
	}
	if kind != "" {
		c.runtime.frames = append(c.runtime.frames, BakalFrame{"bakal_party_buff_granted", 2285, c.PartyFrame(c.location, now)}, BakalFrame{"bakal_script_buff_inventory", 2288, BakalBuffInventoryFrame(c.buffs, BakalBuffTailScript)})
	}
	*o = *c
	return o.eventFrames(), nil
}

func (o *BakalOpening) EventError() error { return o.eventErr }

func (o *BakalOpening) eventEnter(dungeon, loc uint32, now time.Time) ([]BakalFrame, error) {
	if o.stage == BakalOpeningActive && o.runtime.states[dungeon] != "open" {
		return nil, fmt.Errorf("source dungeon is not open")
	}
	c := o.eventClone()
	c.current, c.location, c.dungeonEnteredAt = dungeon, loc, now
	if e := c.dispatchEvent(bakalSignal{op: "[ON ENTER DUNGEON]", id: dungeon}, now); e != nil {
		return nil, e
	}
	delete(c.loaded, dungeon)
	frames := []BakalFrame{{"bakal_portal_ready", 2281, BakalPortalReadyFrame()}, {"bakal_room_party_location", 2285, c.PartyFrame(loc, now)}}
	frames = append(frames, c.eventFrames()...)
	*o = *c
	return frames, nil
}

// Source cooldowns and durations use server time; only the protocol adapter
// assigns the owned member index in N2288. No arbitrary target is accepted.
func (o *BakalOpening) UseRaidBuffAt(slot int, now time.Time) ([]BakalFrame, error) {
	if o.runtime == nil {
		return o.UseRaidBuff(slot)
	}
	if o.stage != BakalOpeningActive || slot < 0 || slot >= 5 || o.buffs[slot] == 0 {
		return nil, fmt.Errorf("commander unavailable outside active run")
	}
	name := ""
	var d catalog.BakalBuffDefinition
	for k, def := range o.rules.BuffDefinitions {
		id, ok := BakalBuffType(k)
		if ok && int(id) == slot && def.Raid {
			name, d = k, def
			break
		}
	}
	if name == "" || o.runtime.cooldown[name].After(now) || o.runtime.cooldown["*"].After(now) {
		return nil, fmt.Errorf("source commander cooling down or unbound")
	}
	c := o.eventClone()
	c.buffs[slot]--
	if name == "RaidBuffAddCoin" {
		c.runtime.coins += d.ServerValue
	}
	c.runtime.cooldown[name] = now.Add(time.Duration(d.Cooltime) * time.Second)
	c.runtime.cooldown["*"] = now.Add(time.Duration(c.rules.RaidBuffCooldownMillis) * time.Millisecond)
	if d.Duration > 0 {
		c.runtime.party[name] = now.Add(time.Duration(d.Duration) * time.Second)
		c.runtime.frames = append(c.runtime.frames, BakalFrame{"bakal_commander_effect", 2285, c.PartyFrame(c.location, now)})
	}
	if name == "RaidBuffCampTeleport" {
		q := []bakalSignal{}
		if e := c.executeEvent(catalog.BakalScriptInstruction{Op: "[KICK OUT DUNGEON NO PENALTY]", Args: []pvf.Token{{Type: 0, Value: int32(c.current)}}}, now, &q); e != nil {
			return nil, e
		}
		for _, s := range q {
			if e := c.dispatchEvent(s, now); e != nil {
				return nil, e
			}
		}
	}
	body := BakalBuffInventoryFrame(c.buffs, BakalBuffTailScript)
	body[5] = byte(slot)
	c.runtime.frames = append(c.runtime.frames, BakalFrame{"bakal_commander_activated", 2288, body})
	if name == "RaidBuffAddCoin" {
		c.runtime.frames = append(c.runtime.frames, BakalFrame{"bakal_commander_coin_budget", 2285, c.PartyFrame(c.location, now)})
	}
	*o = *c
	return o.eventFrames(), nil
}
