package adventure

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const EliteContentsPath = "etc/mycharactersapc/mycharacters_apc_contents.etc"

// EliteContents is the native content-key to dungeon binding. It does not
// override the client's channel/container gate or enable a new wire mode.
type EliteContents map[uint16][]uint32

func importEliteContents(a *pvf.Archive) (EliteContents, string, error) {
	script, err := catalog.ReadScript(a, EliteContentsPath)
	if err != nil {
		return nil, "", err
	}
	rules, err := readEliteContents(script.Cells)
	return rules, script.SHA256, err
}

func readEliteContents(cells []pvf.Token) (EliteContents, error) {
	out := EliteContents{}
	start := -1
	for i, cell := range cells {
		if cell.Type != 3 {
			continue
		}
		switch cell.Text {
		case "[contents]":
			if start >= 0 {
				return nil, fmt.Errorf("nested elite contents block")
			}
			start = i + 1
		case "[/contents]":
			if start < 0 {
				return nil, fmt.Errorf("unmatched elite contents closing tag")
			}
			block := cells[start:i]
			key, err := ruleUint(block, "[contents key]")
			if err != nil || key < 1 || key > 4 {
				return nil, fmt.Errorf("elite contents has unsupported native key: %d", key)
			}
			if _, exists := out[uint16(key)]; exists {
				return nil, fmt.Errorf("duplicate elite contents key: %d", key)
			}
			// Reject duplicated selectors instead of silently accepting the first.
			keys, opens, closes := 0, 0, 0
			openAt, closeAt := -1, -1
			for j, t := range block {
				if t.Type != 3 {
					continue
				}
				switch t.Text {
				case "[contents key]":
					keys++
				case "[dungeons]":
					opens++
					openAt = j
				case "[/dungeons]":
					closes++
					closeAt = j
				}
			}
			if keys != 1 || opens != 1 || closes != 1 || closeAt <= openAt {
				return nil, fmt.Errorf("elite contents key/dungeon sections are incomplete or repeated")
			}
			rows, err := ruleNumbers(block, "[dungeons]")
			if err != nil || len(rows) == 0 || closeAt-openAt-1 != len(rows) {
				return nil, fmt.Errorf("elite contents %d has no valid dungeon list", key)
			}
			for _, id := range rows {
				if id <= 0 || id > 0xffffffff {
					return nil, fmt.Errorf("elite contents %d has invalid dungeon: %d", key, id)
				}
				out[uint16(key)] = append(out[uint16(key)], uint32(id))
			}
			start = -1
		}
	}
	if start >= 0 {
		return nil, fmt.Errorf("unclosed elite contents block")
	}
	return out, validateEliteContents(out)
}

func validateEliteContents(contents EliteContents) error {
	seen := map[uint32]uint16{}
	for mode, dungeons := range contents {
		if mode < 1 || mode > 4 || len(dungeons) == 0 {
			return fmt.Errorf("invalid elite contents mode/dungeons: %d", mode)
		}
		for _, id := range dungeons {
			if old, exists := seen[id]; id == 0 || exists {
				return fmt.Errorf("invalid or ambiguous elite dungeon %d in modes %d/%d", id, old, mode)
			}
			seen[id] = mode
		}
	}
	return nil
}

func (r *Rules) EliteModeForDungeon(id uint32) (uint16, bool) {
	if r != nil {
		for mode, dungeons := range r.EliteContents {
			for _, candidate := range dungeons {
				if candidate == id {
					return mode, true
				}
			}
		}
	}
	return 0, false
}
