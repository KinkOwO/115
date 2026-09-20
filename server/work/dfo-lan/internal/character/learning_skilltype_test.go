package character

import (
	"dfolan/internal/catalog"
	"testing"
)

func loadLearningForTest(t *testing.T) *LearningCatalog {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.next27.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	return l
}

// Root cause of the live "level 35 skills cannot be learned" report: the
// projection kept the first [type] it met, while the current PVF repeats
// [type] inside [vp explain], [preset info] and the damage groups to label the
// default/vp1/vp2 variant a block belongs to. Every Swordman skill whose
// required level is at least 35 declares its own type as [active], so a row
// projected as vp1 was refused as an unsupported skill type.
func TestLevel35SwordmanSkillsCarryTheirSourceType(t *testing.T) {
	l := loadLearningForTest(t)
	d := l.index[0][9]
	if d.Path != "skill/swordman/momentaryslash.skl" {
		t.Fatalf("unexpected skill 9 source %q", d.Path)
	}
	if !d.Active() {
		t.Fatal("skill 9 does not carry its source [active] type")
	}
	cost, e := d.Cost(35, 1, 1, map[uint16]byte{})
	if e != nil || cost != 40 {
		t.Fatalf("level 35 Swordman skill 9 rank 1: cost=%d err=%v", cost, e)
	}
	// The other growtype 1 rows the client offers at or above level 35.
	for _, id := range []uint16{72, 73, 98} {
		if !l.index[0][id].Active() {
			t.Fatalf("skill %d does not carry its source [active] type", id)
		}
	}
}

// Widening a projection must never weaken the level, rank interval or growtype
// rules the row still carries.
func TestSkillLearningRespectsLevelAndGrowtype(t *testing.T) {
	l := loadLearningForTest(t)
	known := map[uint16]byte{}
	// rank 2 needs required level 35 + 2 = 37.
	if _, e := l.index[0][9].Cost(35, 1, 2, known); e == nil {
		t.Fatal("level 35 learned rank 2 of a required level 35 skill")
	}
	// skill 72 rapidmoveslash requires character level 40.
	if _, e := l.index[0][72].Cost(35, 1, 1, known); e == nil {
		t.Fatal("level 35 learned a required level 40 skill")
	}
	// skill 9 belongs to growtype 1, not to the unadvanced base column.
	if _, e := l.index[0][9].Cost(35, 0, 1, known); e == nil {
		t.Fatal("unadvanced Swordman learned a growtype 1 skill")
	}
}

// default/vp1/vp2 are variant labels, never a skill's own type. Only the five
// source skills that declare no own [type] may keep one, and all five have a
// zero growtype cap, so no learning request can reach them.
func TestSkillCatalogCarriesSourceSkillType(t *testing.T) {
	l := loadLearningForTest(t)
	allowed := map[string]bool{
		"skill/atmage/spiralpress.skl":  true,
		"skill/atmage/violentstorm.skl": true,
		"skill/priest/doomcrush.skl":    true,
		"skill/priest/direstream.skl":   true,
		"skill/priest/mortalgospel.skl": true,
	}
	seen := 0
	for _, d := range l.Rows {
		ts := d.Fields["[type]"]
		if len(ts) == 1 && (ts[0].Text == "[active]" || ts[0].Text == "[passive]") {
			continue
		}
		if !allowed[d.Path] {
			t.Fatalf("%s carries variant [type] %v instead of its source type", d.Path, ts)
		}
		seen++
		for _, adv := range []int{0, 1, 2, 3, 4, 5} {
			if d.ForAdvancement(adv) {
				t.Fatalf("%s without a source [type] must stay unlearnable (growtype %d)", d.Path, adv)
			}
		}
	}
	if seen != len(allowed) {
		t.Fatalf("rows without a source [type] = %d, want %d", seen, len(allowed))
	}
}
