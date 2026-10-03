package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

func favorSourceFixture() ScriptRecord {
	cells := []pvf.Token{}
	add := func(tag string, nums ...int32) {
		cells = append(cells, pvf.Token{Type: 3, Text: tag})
		for _, n := range nums {
			cells = append(cells, pvf.Token{Type: 0, Value: n})
		}
	}
	add("[favor condition level]", 20)
	add("[favor gift item count]", 100)
	add("[favor gift limit]", 5)
	add("[favor level point up]", 3037, 100, 300, 3033, 200, 600, 3034, 200, 600, 3035, 200, 600, 3036, 200, 600, 3262, 400, 900)
	add("[/favor level point up]")
	add("[favor level point down]", 0, 21, 500, 1, 14, 1000, 2, 7, 1500)
	add("[/favor level point down]")
	return ScriptRecord{Path: FavorRulesPath, Cells: cells}
}
func TestNPCFavorRulesFollowChangedSource(t *testing.T) {
	s := favorSourceFixture()
	r, e := ParseNPCFavorRules("source", s)
	if e != nil {
		t.Fatal(e)
	}
	if r.GiftCount != 100 || r.OpenLevel != 20 || r.DailyLimit != 5 || r.MaxPoint() != 1500 || r.Gifts[3034] != [2]int64{200, 600} {
		t.Fatal(r)
	}
	// Change the source input rather than copying a fixed gameplay table into Go.
	for i, c := range s.Cells {
		if c.Type == 3 {
			switch c.Text {
			case "[favor condition level]":
				s.Cells[i+1].Value = 27
			case "[favor gift item count]":
				s.Cells[i+1].Value = 13
			case "[favor gift limit]":
				s.Cells[i+1].Value = 9
			}
		}
	}
	for i, c := range s.Cells {
		if c.Type == 0 && c.Value == 3034 {
			s.Cells[i+1].Value = 700
			s.Cells[i+2].Value = 800
		}
		if c.Type == 0 && c.Value == 1500 {
			s.Cells[i].Value = 1900
		}
	}
	r, e = ParseNPCFavorRules("source", s)
	if e != nil {
		t.Fatal(e)
	}
	lo, hi := r.PointRange(3034)
	if r.OpenLevel != 27 || r.GiftCount != 13 || r.DailyLimit != 9 || r.MaxPoint() != 1900 || lo != 700 || hi != 800 {
		t.Fatalf("rules ignore source changes: %+v", r)
	}
	if lo, _ := r.PointRange(999); lo != 0 {
		t.Fatal("unknown gift enabled")
	}
}
func TestNPCFavorRulesRejectMalformedSource(t *testing.T) {
	for _, kind := range []string{"missing count", "negative range", "reversed range", "duplicate gift", "non-increasing level", "duplicate scalar"} {
		t.Run(kind, func(t *testing.T) {
			s := favorSourceFixture()
			switch kind {
			case "missing count":
				for i, c := range s.Cells {
					if c.Text == "[favor gift item count]" {
						s.Cells[i].Text = "[unknown]"
					}
				}
			case "negative range":
				for i, c := range s.Cells {
					if c.Value == 3034 {
						s.Cells[i+1].Value = -1
					}
				}
			case "reversed range":
				for i, c := range s.Cells {
					if c.Value == 3034 {
						s.Cells[i+2].Value = 10
					}
				}
			case "duplicate gift":
				for i, c := range s.Cells {
					if c.Value == 3034 {
						s.Cells[i].Value = 3033
					}
				}
			case "non-increasing level":
				for i, c := range s.Cells {
					if c.Value == 1500 {
						s.Cells[i].Value = 1000
					}
				}
			case "duplicate scalar":
				s.Cells = append(s.Cells, pvf.Token{Type: 3, Text: "[favor gift item count]"}, pvf.Token{Type: 0, Value: 1})
			}
			if _, e := ParseNPCFavorRules("source", s); e == nil {
				t.Fatal("malformed source accepted")
			}
		})
	}
}
func TestNPCFavorRulesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE")
	}
	a, e := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1073741824}, "")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	r, e := ImportNPCFavorRules(a)
	if e != nil {
		t.Fatal(e)
	}
	if r.Source != a.Snapshot().Checksum || r.Definition.Path != FavorRulesPath || len(r.Definition.SHA256) != 64 || len(r.Gifts) != 6 || len(r.Levels) != 3 || r.GiftCount != 100 || r.OpenLevel != 20 || r.MaxPoint() != 1500 {
		t.Fatal(r)
	}
	t.Logf("native favor: gifts=%d levels=%v count=%d open=%d hash=%s", len(r.Gifts), r.Levels, r.GiftCount, r.OpenLevel, r.Definition.SHA256)
}
