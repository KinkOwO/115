package catalog

import (
	"encoding/json"
	"fmt"
	"os"
)

const OdysseySource = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"

type OdysseyGrowth struct {
	Source      string                  `json:"source"`
	Definition  ScriptRecord            `json:"definition"`
	Items       map[uint32]ScriptRecord `json:"items"`
	ClearLevels map[uint32]byte         `json:"-"`
	EntryLevels map[uint32]byte         `json:"-"`
	Gifts       map[byte]uint32         `json:"-"`
}

func LoadOdysseyGrowth(path string) (*OdysseyGrowth, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var r OdysseyGrowth
	if e = json.Unmarshal(b, &r); e != nil {
		return nil, e
	}
	if r.Source != OdysseySource || r.Definition.SHA256 != "638e71ab8fdc84b4be28db8ca3302fd1dfe689a9b771907514297edee4b8c8e8" {
		return nil, fmt.Errorf("Odyssey source mismatch")
	}
	r.ClearLevels = map[uint32]byte{}
	r.EntryLevels = map[uint32]byte{}
	r.Gifts = map[byte]uint32{}
	for _, name := range []string{"[grow up level on dungeon clear]", "[dungeon list by level]", "[reward info]"} {
		c := sectionCells(r.Definition.Cells, name)
		if len(c) == 0 || len(c)%2 != 0 {
			return nil, fmt.Errorf("invalid Odyssey table %s", name)
		}
		for i := 0; i < len(c); i += 2 {
			a, b := c[i], c[i+1]
			if a.Type != 0 || b.Type != 0 || a.Value <= 0 || b.Value <= 0 {
				return nil, fmt.Errorf("invalid Odyssey pair")
			}
			if name == "[grow up level on dungeon clear]" {
				if b.Value > 115 || r.ClearLevels[uint32(a.Value)] != 0 {
					return nil, fmt.Errorf("invalid clear level")
				}
				r.ClearLevels[uint32(a.Value)] = byte(b.Value)
			} else {
				if a.Value > 115 {
					return nil, fmt.Errorf("invalid reward/entry level")
				}
				if name == "[reward info]" {
					if r.Gifts[byte(a.Value)] != 0 {
						return nil, fmt.Errorf("duplicate gift level")
					}
					r.Gifts[byte(a.Value)] = uint32(b.Value)
				} else {
					if r.EntryLevels[uint32(b.Value)] != 0 {
						return nil, fmt.Errorf("duplicate dungeon")
					}
					r.EntryLevels[uint32(b.Value)] = byte(a.Value)
				}
			}
		}
	}
	if len(r.ClearLevels) != 50 || len(r.EntryLevels) != 50 || len(r.Gifts) != 3 {
		return nil, fmt.Errorf("incomplete Odyssey tables")
	}
	for id, target := range r.ClearLevels {
		if r.EntryLevels[id] == 0 || target <= r.EntryLevels[id] {
			return nil, fmt.Errorf("invalid Odyssey level progression")
		}
	}
	for _, id := range r.Gifts {
		if r.Items[id].SHA256 == "" {
			return nil, fmt.Errorf("missing Odyssey gift script")
		}
	}
	return &r, nil
}
