package gamedata_test

import (
	"dfolan/internal/gamedata"
	"os"
	"testing"
)

func TestBoostTrainingSourceSkills(t *testing.T) {
	path := os.Getenv("US115_TEST_BOOST_PVF")
	if path == "" {
		t.Skip("explicit source")
	}
	s, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, MaxBytes: gamedata.DefaultMaxBytes})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	chars, e := s.Characters("")
	if e != nil {
		t.Fatal(e)
	}
	learning, e := s.Learning(chars)
	if e != nil {
		t.Fatal(e)
	}
	idx, e := s.ItemIndex("")
	if e != nil {
		t.Fatal(e)
	}
	src, content, e := s.BoostUp(idx)
	if e != nil {
		t.Fatal(e)
	}
	if e = content.BindTrainingAPCs(src); e != nil {
		t.Fatal(e)
	}
	if len(content.TrainingAPCs) != 1 || content.TrainingAPCs[0].Template != 2504 || content.TrainingAPCs[0].SourceIndex != 1 {
		t.Fatal("unexpected current training APC binding", content.TrainingAPCs)
	}
	cells, e := src.Tokens("aicharacter/mycharacters_apc_custom/2025_boostup_forbuffer/swordman/custommyapcswordman.aic")
	if e != nil {
		t.Fatal(e)
	}
	var active []uint16
	inSkill := false
	for i := 0; i < len(cells); i++ {
		c := cells[i]
		if c.Type == 3 {
			inSkill = c.Text == "[skill]"
			continue
		}
		if !inSkill {
			continue
		}
		if c.Type != 0 || i+1 >= len(cells) || cells[i+1].Type != 0 {
			t.Fatal("bad skill pair")
		}
		d, ok, e := learning.Definition(0, uint16(c.Value))
		if e != nil || !ok {
			t.Fatalf("skill %d: %v %v", c.Value, ok, e)
		}
		if d.ShortcutCapable() {
			active = append(active, uint16(c.Value))
		}
		i++
	}
	t.Logf("active=%v count=%d", active, len(active))
}
