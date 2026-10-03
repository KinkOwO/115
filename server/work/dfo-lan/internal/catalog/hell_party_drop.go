package catalog

import "fmt"

type HellPartyDropTable struct {
	Path, SHA256 string
	Probability  [][7]uint32 // min level, max level, five dungeon difficulties
	Rarity       [][9]uint32 // source rows A/B; retain unreachable sentinel columns
}

func ParseHellPartyDropTable(s ScriptRecord) (*HellPartyDropTable, error) {
	t := &HellPartyDropTable{Path: s.Path, SHA256: s.SHA256}
	if s.Path != "etc/itemdropinfo_monster_hell.etc" || len(s.SHA256) != 64 {
		return nil, fmt.Errorf("Hell drop table without provenance")
	}
	count := sectionCells(s.Cells, "[drop prob count]")
	cells := sectionCells(s.Cells, "[dungeon difficulty drop prob]")
	if len(count) != 1 || count[0].Type != 0 || count[0].Value < 1 || count[0].Value > 200 || len(cells) != int(count[0].Value)*7 {
		return nil, fmt.Errorf("invalid Hell drop probability rows")
	}
	var previous uint32
	for i := 0; i < len(cells); i += 7 {
		var row [7]uint32
		for j := range row {
			if cells[i+j].Type != 0 || cells[i+j].Value < 0 {
				return nil, fmt.Errorf("invalid Hell probability cell")
			}
			row[j] = uint32(cells[i+j].Value)
		}
		if row[0] == 0 || row[1] < row[0] || row[1] > 200 || row[0] <= previous {
			return nil, fmt.Errorf("invalid Hell level range")
		}
		previous = row[1]
		t.Probability = append(t.Probability, row)
	}
	cells = sectionCells(s.Cells, "[basis of rarity dicision]")
	if len(cells) < 1 || cells[0].Type != 0 || cells[0].Value != 2 || len(cells) != 19 {
		return nil, fmt.Errorf("Hell rarity requires A/B rows of nine columns")
	}
	for i := 1; i < len(cells); i += 9 {
		var row [9]uint32
		var last uint32
		covered := false
		for j := range row {
			c := cells[i+j]
			if c.Type != 0 || c.Value < 0 {
				return nil, fmt.Errorf("invalid Hell rarity cell")
			}
			row[j] = uint32(c.Value)
			if row[j] <= 1000000 && row[j] < last {
				return nil, fmt.Errorf("decreasing reachable Hell rarity threshold")
			}
			if row[j] >= 1000000 {
				covered = true
			}
			last = row[j]
		}
		if !covered {
			return nil, fmt.Errorf("Hell rarity does not cover compatibility denominator")
		}
		t.Rarity = append(t.Rarity, row)
	}
	return t, nil
}
