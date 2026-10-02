package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"dfolan/internal/catalog/pvf"
)

func TestCurrentClearRewardSourceKeepsProfilesAndNestedBonuses(t *testing.T) {
	p := os.Getenv("DFO_LOOT_PVF")
	if p == "" {
		p = filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip("current PVF absent")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: p, MaxBytes: 900 * 1024 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s, err := ResolveScript(a, "etc/itemdropinfo_clearreward.etc")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := ParseClearRewardTable(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules.Profiles) != 7 || len(rules.GradeReference) != 200 || len(rules.GoldCardCosts) != 200 || rules.SHA256 != s.SHA256 {
		t.Fatalf("source table boundary changed: %+v", rules)
	}
	p90, ok := rules.Profile("event")
	row, covered := p90.Row(90)
	if !ok || !covered || row != (ClearRewardLevelRow{90, 94, 10000, 100}) {
		t.Fatalf("fourth profile column lost: %+v", row)
	}
	row, covered = p90.Row(95)
	if !covered || row != (ClearRewardLevelRow{95, 200, 9000, 0}) {
		t.Fatalf("next profile row shifted: %+v", row)
	}
	if _, ok := p90.Row(201); ok {
		t.Fatal("uncovered level used nearest row")
	}
	if _, ok := rules.Profile("unknown"); ok {
		t.Fatal("unknown profile used default")
	}
	if len(rules.PartyBonus) != 3 || len(rules.GoldDifficultyBonus) != 3 || rules.PartyBonus[0].DungeonType != -1 || len(rules.PartyBonus[0].Values) != 4 || rules.GoldDifficultyBonus[2].DungeonType != 100 || len(rules.GoldDifficultyBonus[2].Values) != 5 {
		t.Fatal("nested dungeon type bonuses lost")
	}
	if rules.MapCountRates[4] != ([2]int32{5, 6000}) || rules.GoldCardCosts[114] != ([2]int32{115, 13780}) || rules.GradeReference[101] != ([3]int32{102, 7, -1}) || rules.RarityControl != ([5]int32{2, 13, 17, 0, -5}) {
		t.Fatal("source integers or signed grade offsets changed")
	}
	if rules.ItemTypeProbability != ([4]int32{0, 9500, 0, 500}) || rules.Rarity[4] != 1000001 || rules.GoldCardCreateRate != 1 {
		t.Fatal("source thresholds clamped or changed")
	}
	for _, mutation := range []struct {
		name   string
		change func(*ScriptRecord)
	}{
		{"missing fourth profile column", func(b *ScriptRecord) {
			for i, c := range b.Cells {
				if c.Type == 6 && c.Text == "default" {
					b.Cells = append(b.Cells[:i+4], b.Cells[i+5:]...)
					return
				}
			}
		}},
		{"missing profile closing tag", func(b *ScriptRecord) {
			for i, c := range b.Cells {
				if c.Type == 3 && c.Text == "[/drop prob]" {
					b.Cells = append(b.Cells[:i], b.Cells[i+1:]...)
					return
				}
			}
		}},
		{"short nested row", func(b *ScriptRecord) {
			for i, c := range b.Cells {
				if c.Type == 3 && c.Text == "[party member drop bonusrate]" {
					b.Cells = append(b.Cells[:i+3], b.Cells[i+4:]...)
					return
				}
			}
		}},
		{"duplicate core section", func(b *ScriptRecord) { b.Cells = append(b.Cells, pvf.Token{Type: 3, Text: "[drop kind prob]"}) }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			broken := s
			broken.Cells = append([]pvf.Token(nil), s.Cells...)
			mutation.change(&broken)
			if _, err := ParseClearRewardTable(broken); err == nil {
				t.Fatal("malformed core rule accepted")
			}
		})
	}
}
