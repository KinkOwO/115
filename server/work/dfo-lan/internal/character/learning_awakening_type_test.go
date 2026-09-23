package character

import (
	"dfolan/internal/catalog"
	"fmt"
	"sort"
	"testing"
)

// resolvePrereq marks every [pre required skill] of the queried skill (and its
// own prereq chain) as learned, so prerequisite refusals are excluded and only
// genuine gates surface.
func resolvePrereq(l *LearningCatalog, job byte, id uint16, acc map[uint16]byte) {
	if _, ok := acc[id]; ok {
		return
	}
	acc[id] = 250
	d, ok := l.index[job][id]
	if !ok {
		return
	}
	pre := d.Ints("[pre required skill]")
	for i := 0; i+1 < len(pre); i += 2 {
		pid := uint16(pre[i])
		if _, ok := acc[pid]; !ok {
			resolvePrereq(l, job, pid, acc)
		}
	}
}

func loadRealSkillCatalog(t *testing.T) (*Service, *LearningCatalog, catalog.Characters) {
	t.Helper()
	c, err := catalog.LoadCharacters("../../configs/characters.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	l, err := LoadLearningCatalog("../../configs/skills.next27.json", c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	return &Service{Catalog: c, Learning: l}, l, c
}

// Every skill the second-awakening matrix authorizes must be purchasable once
// the level gate is met and prerequisites are satisfied. Before the fix,
// skills whose source row carries no [type] (priest 253, at mage 138/139)
// were refused with "unsupported skill advancement/level" at every level —
// exactly the "二觉后学不了技能" report. The level gates themselves remain
// enforced (a 85-requirement skill is still refused below 85).
func TestSecondAwakeningMatrixSkillsLearnableWhenEligible(t *testing.T) {
	s, l, _ := loadRealSkillCatalog(t)
	type row struct {
		job, adv int
		id       uint16
		req, lvl int
		reason   string
	}
	var refused []row
	for job, defs := range l.index {
		for adv := 1; adv < 6; adv++ {
			for id, d := range defs {
				if !d.ForAwakening(adv, 2) {
					continue
				}
				reqs := d.Ints("[required level]")
				if len(reqs) != 1 {
					t.Fatalf("job=%d adv=%d id=%d: awakening skill missing required level", job, adv, id)
				}
				known := map[uint16]byte{}
				resolvePrereq(l, job, id, known)
				state := State{Level: byte(reqs[0]), Advancement: byte(adv), Awakening: 2}
				if _, err := d.costForLevel(state, reqs[0], 1, known); err != nil {
					refused = append(refused, row{int(job), adv, id, reqs[0], reqs[0], err.Error()})
				}
			}
		}
	}
	if len(refused) == 0 {
		return
	}
	sort.Slice(refused, func(i, j int) bool { return refused[i].reason < refused[j].reason })
	for _, r := range refused {
		t.Errorf("job=%d adv=%d id=%d req=%d: %s", r.job, r.adv, r.id, r.req, r.reason)
	}
	_ = s
}

// The [type] shape check now sits below the cap branch. Guard against a
// regression that lets a *non-awakening* row through: a row whose base caps are
// present and authorize adv while [type] is invalid would only exist if the
// source directory dropped the type row — the fix targets exactly the
// awakening-only rows the matrix authorizes.
func TestNoNonAwakeningRowReliesOnRelaxedTypeCheck(t *testing.T) {
	_, l, _ := loadRealSkillCatalog(t)
	var suspicious []string
	for _, d := range l.Rows {
		t := d.Fields["[type]"]
		typeOK := len(t) == 1 && (t[0].Text == "[active]" || t[0].Text == "[passive]")
		if typeOK {
			continue
		}
		cap := d.Ints("[growtype maximum level]")
		if len(cap) == 0 || allZeroCaps(cap) {
			continue
		}
		for adv := 0; adv < len(cap); adv++ {
			if cap[adv] > 0 {
				// Only rows the awakening matrix authorizes may lack [type].
				if !anyAwakeningCap(d, adv) {
					suspicious = append(suspicious, fmt.Sprintf("job=%d adv=%d id=%d", d.Job, adv, d.ID))
				}
			}
		}
	}
	if len(suspicious) > 0 {
		for _, s := range suspicious {
			t.Errorf("non-awakening row with invalid [type] now learnable: %s", s)
		}
	}
}

func anyAwakeningCap(d LearningDefinition, adv int) bool {
	cols := d.awakeningColumns()
	if cols == 0 || adv >= cols {
		return false
	}
	matrix := d.Ints("[awakening maximum level]")
	for stage := 1; stage <= 3; stage++ {
		if matrix[(stage-1)*cols+adv] > 0 {
			return true
		}
	}
	return false
}
