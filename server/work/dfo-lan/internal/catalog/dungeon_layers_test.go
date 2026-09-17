package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"testing"
)

func TestDungeonLayerValidation(t *testing.T) {
	raw, err := os.ReadFile("../../docs/evidence/skycastle-auto-skills-20260917/skycastle.json")
	if err != nil {
		t.Fatal(err)
	}
	var c DungeonCatalog
	if err = json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	original := c.Dungeons[100004937].Script
	for _, mode := range []string{"unknown", "negative-map", "orphan", "duplicate-base"} {
		t.Run(mode, func(t *testing.T) {
			s := original
			s.Cells = append([]pvf.Token(nil), original.Cells...)
			for i, tok := range s.Cells {
				if tok.Type != 6 || tok.Text != "layered" {
					continue
				}
				switch mode {
				case "unknown":
					s.Cells[i].Text = "unsupported"
				case "negative-map":
					s.Cells[i+3].Value = -1
				case "orphan":
					s.Cells[i+1].Value = 1
					s.Cells[i+2].Value = 5
				case "duplicate-base":
					row := []pvf.Token{{Type: 6, Text: "map"}, {Type: 0, Value: 0}, {Type: 0, Value: 0}, {Type: 0, Value: 100015998}}
					s.Cells = append(s.Cells[:i], append(row, s.Cells[i:]...)...)
				}
				break
			}
			d, err := ParseDungeon(100004937, s)
			if err == nil && len(d.Mazes[0].Pending) == 0 {
				t.Fatal("invalid layout admitted")
			}
		})
	}
}
