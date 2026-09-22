package character

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
)

// LearningCatalog retains current PVF fields and provenance. No foreign-job
// skill IDs or reference-server SP table is merged into this projection.
type LearningDefinition struct {
	Job          byte
	ID           uint16
	Path, SHA256 string
	Fields       map[string][]pvf.Token
}
type LearningCatalog struct {
	Source pvf.ArchiveSnapshot  `json:"source"`
	Rows   []LearningDefinition `json:"rows"`
	index  map[byte]map[uint16]LearningDefinition
}

func LoadLearningCatalog(path string, source string) (*LearningCatalog, error) {
	p, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c LearningCatalog
	if e = json.Unmarshal(p, &c); e != nil {
		return nil, e
	}
	if c.Source.Checksum != source || len(c.Rows) == 0 {
		return nil, fmt.Errorf("learning source mismatch")
	}
	c.index = map[byte]map[uint16]LearningDefinition{}
	for _, d := range c.Rows {
		if d.ID == 0 || len(d.SHA256) != 64 || d.Path == "" {
			return nil, fmt.Errorf("invalid source skill")
		}
		if c.index[d.Job] == nil {
			c.index[d.Job] = map[uint16]LearningDefinition{}
		}
		if _, ok := c.index[d.Job][d.ID]; ok {
			return nil, fmt.Errorf("duplicate job skill")
		}
		c.index[d.Job][d.ID] = d
	}
	return &c, nil
}
func (d LearningDefinition) Ints(tag string) []int {
	var out []int
	for _, t := range d.Fields[tag] {
		if t.Type != 0 {
			return nil
		}
		out = append(out, int(t.Value))
	}
	return out
}
func (d LearningDefinition) Active() bool {
	ts := d.Fields["[type]"]
	return len(ts) == 1 && ts[0].Text == "[active]"
}

// allZeroCaps reports whether the source row is present but gives every
// growtype a zero cap. A row like [0 1 1 1 1 1] (archer latentability) is NOT
// all-zero: its zero is a real "this growtype cannot learn it" statement.
func allZeroCaps(v []int) bool {
	if len(v) == 0 {
		return false
	}
	for _, x := range v {
		if x != 0 {
			return false
		}
	}
	return true
}

// fitnessAllows answers "is this row owned by growtype adv?" from the two
// fitness rows. An empty [skill fitness growtype] with an empty
// [skill fitness second growtype] declares no ownership, which is how the
// current source writes job-wide rows; eligibility then falls to the level,
// prerequisite and cost checks.
func (d LearningDefinition) fitnessAllows(adv int) bool {
	fit := d.Ints("[skill fitness growtype]")
	for _, g := range fit {
		if g == adv {
			return true
		}
	}
	if sec := d.Ints("[skill fitness second growtype]"); len(sec) > 0 {
		for _, g := range sec {
			if g == adv {
				return true
			}
		}
		return false
	}
	return len(fit) == 0
}

// Explicit source profession eligibility for unlearned skill definitions.
// Existing .chr free grants do not depend on manual learning eligibility.
func (d LearningDefinition) ForAdvancement(adv int) bool {
	cap := d.Ints("[growtype maximum level]")
	if adv < 0 || len(cap) > 0 && adv >= len(cap) {
		return false
	}
	t := d.Fields["[type]"]
	if len(t) != 1 || (t[0].Text != "[active]" && t[0].Text != "[passive]") {
		return false
	}
	if len(cap) > 0 && !allZeroCaps(cap) {
		// The source gave a per-growtype cap row, so a zero in it means that
		// growtype cannot learn the skill at all.
		return cap[adv] > 0
	}
	if matrix := d.Ints("[awakening maximum level]"); len(matrix) > 0 && !allZeroCaps(matrix) {
		// An all-zero base row plus a live awakening matrix is an
		// awakening-only row: it stays refused here and is selected through
		// forState once the character actually awakened into this growtype.
		return false
	}
	// All-zero (or absent) base row with no live matrix: the row does not use
	// the base cap table at all, so ownership comes from the fitness rows.
	// atgunner/quartermaster ([0 0 0 0 0 0], matrix zero, second growtype 2)
	// and atfighter/lightenchantweapon (caps missing, fitness 1) are that
	// shape, and refusing them surfaced live as "觉醒后学不了觉醒技能".
	return d.fitnessAllows(adv)
}
func (d LearningDefinition) Cost(level, advancement, target int, known map[uint16]byte) (int, error) {
	// This argument is a growtype index, not the packed awakening wire byte.
	// Source fitness/caps decide eligibility for both base and advanced jobs.
	if advancement < 0 || advancement > 15 || target < 1 || target > 255 || !d.ForAdvancement(advancement) {
		return 0, fmt.Errorf("unsupported skill advancement/level")
	}
	required, max, cost := d.Ints("[required level]"), d.Ints("[maximum level]"), d.Ints("[purchase cost]")
	if len(required) != 1 || len(max) != 1 || len(cost) == 0 {
		return 0, fmt.Errorf("skill learning fields unavailable")
	}
	// [purchase cost] carries one value per rank band: the first purchase and
	// every later rank. Awakening skills use that shape (costForLevel selects
	// the band for them), but plain source skills do too - knight/perfectguard
	// and atgunner/twingunblade are [30 15] / [0 15] - and demanding a single
	// value refused them with "skill learning fields unavailable".
	costIdx := 0
	if target > 1 && len(cost) > 1 {
		costIdx = 1
	}
	if cost[costIdx] < 0 {
		return 0, fmt.Errorf("skill learning fields unavailable")
	}
	if len(d.Fields["[special purchase cost]"]) > 0 || len(d.Fields["[feature skill type]"]) > 0 {
		return 0, fmt.Errorf("technique skill learning pending")
	}
	limit := max[0]
	// An all-zero cap row means the source does not cap this skill per
	// growtype at all ([maximum level] is then the only bound) - see the
	// all-zero branch in ForAdvancement.
	if caps := d.Ints("[growtype maximum level]"); len(caps) > advancement && !allZeroCaps(caps) {
		if caps[advancement] < limit {
			limit = caps[advancement]
		}
	}
	step := d.Ints("[required level range]")
	// A skill with no [required level range] does not gate its higher ranks
	// behind extra character levels: every rank it allows (bounded by the
	// growtype cap and maximum level) is available at the base required level.
	// Refusing rank 2+ outright made archer 179 - a rank-10 mastery with no
	// range - stick at rank 1, which surfaced live as "skill rank interval
	// unavailable". Ranks stay bounded by target > limit below, so a skill like
	// 190, capped at rank 1 for the unadvanced growtype, is still refused there
	// (now with the correct advancement/level reason).
	need := required[0]
	if target > 1 && len(step) == 1 && step[0] > 0 {
		need += (target - 1) * step[0]
	}
	if target > limit || level < need {
		return 0, fmt.Errorf("character level does not allow skill rank")
	}
	pre := d.Ints("[pre required skill]")
	if len(pre)%2 != 0 {
		return 0, fmt.Errorf("invalid skill prerequisite")
	}
	for i := 0; i < len(pre); i += 2 {
		if int(known[uint16(pre[i])]) < pre[i+1] {
			return 0, fmt.Errorf("skill prerequisite not learned")
		}
	}
	return cost[costIdx], nil
}
