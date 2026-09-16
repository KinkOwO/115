package character

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestArcherBaseSkillsRespectBothFitnessAndGrowCap(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.next27.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []uint16{3, 7, 511} {
		if !l.index[16][id].ForAdvancement(0) {
			t.Fatalf("base Archer skill%d filtered", id)
		}
	}
	// The original latent-ability script lists grow0 in fitness but caps it
	// at zero. Its advancement-only passive must not leak into the base page.
	if l.index[16][501].ForAdvancement(0) {
		t.Fatal("grow0 zero-cap skill leaked")
	}
	if !l.index[16][501].ForAdvancement(1) {
		t.Fatal("legitimate advanced eligibility lost")
	}
	if _, e = l.index[16][7].Cost(4, 0, 1, map[uint16]byte{}); e == nil {
		t.Fatal("level4 learned level10 RisingMoon")
	}
}
