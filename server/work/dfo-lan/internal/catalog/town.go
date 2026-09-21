package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TownArea contains source facts only. The server's chosen spawn point is
// separate policy, not a claim about the official new-character tutorial.
type TownArea struct {
	Source       pvf.ArchiveSnapshot `json:"source"`
	TownID       uint32              `json:"town_id"`
	AreaID       uint32              `json:"area_id"`
	TownPath     string              `json:"town_path"`
	TownSHA256   string              `json:"town_sha256"`
	MapPath      string              `json:"map_path"`
	MapSHA256    string              `json:"map_sha256"`
	MinimumLevel uint32              `json:"minimum_level"`
	Walkable     [][4]int32          `json:"walkable_rectangles"`
}

func ImportTownArea(a *pvf.Archive, townID, areaID uint32) (TownArea, error) {
	c := TownArea{Source: a.Snapshot(), TownID: townID, AreaID: areaID}
	list, err := a.Tokens("list/town.lst")
	if err != nil {
		return c, err
	}
	if len(list)%2 != 0 {
		return c, fmt.Errorf("invalid town list")
	}
	for i := 0; i < len(list); i += 2 {
		if list[i].Type != 0 || list[i+1].Type != 6 {
			return c, fmt.Errorf("invalid town list pair")
		}
		if list[i].Value >= 0 && uint32(list[i].Value) == townID {
			c.TownPath = strings.ToLower(list[i+1].Text)
		}
	}
	if c.TownPath == "" {
		return c, fmt.Errorf("town %d absent from PVF", townID)
	}
	ts, err := a.Tokens(c.TownPath)
	if err != nil {
		return c, err
	}
	active := false
	for i, t := range ts {
		if t.Type != 3 {
			continue
		}
		if t.Text == "[area]" {
			if i+2 >= len(ts) || ts[i+1].Type != 0 || ts[i+2].Type != 6 {
				return c, fmt.Errorf("invalid town area")
			}
			active = ts[i+1].Value >= 0 && uint32(ts[i+1].Value) == areaID
			if active {
				c.MapPath = "map/" + strings.ToLower(strings.ReplaceAll(ts[i+2].Text, "\\", "/"))
			}
		} else if t.Text == "[/area]" {
			active = false
		} else if active && t.Text == "[need level]" {
			if i+1 >= len(ts) || ts[i+1].Type != 0 || ts[i+1].Value < 0 {
				return c, fmt.Errorf("invalid area level")
			}
			c.MinimumLevel = uint32(ts[i+1].Value)
		}
	}
	if c.MapPath == "" {
		return c, fmt.Errorf("area %d absent from town %d", areaID, townID)
	}
	ts, err = a.Tokens(c.MapPath)
	if err != nil {
		return c, err
	}
	var cells []int32
	active = false
	for _, t := range ts {
		if t.Type == 3 {
			active = t.Text == "[virtual movable area]"
			continue
		}
		if active {
			if t.Type != 0 {
				return c, fmt.Errorf("non-numeric walkable area")
			}
			cells = append(cells, t.Value)
		}
	}
	if len(cells) == 0 || len(cells)%4 != 0 {
		return c, fmt.Errorf("map walkable rectangles not recovered")
	}
	for i := 0; i < len(cells); i += 4 {
		if cells[i+2] <= 0 || cells[i+3] <= 0 {
			return c, fmt.Errorf("invalid walkable rectangle")
		}
		c.Walkable = append(c.Walkable, [4]int32{cells[i], cells[i+1], cells[i+2], cells[i+3]})
	}
	for path, dst := range map[string]*string{c.TownPath: &c.TownSHA256, c.MapPath: &c.MapSHA256} {
		raw, e := a.ReadRaw(path)
		if e != nil {
			return c, e
		}
		*dst = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	return c, nil
}

func (c TownArea) Allows(level byte, x, y uint16) bool {
	if uint32(level) < c.MinimumLevel {
		return false
	}
	for _, r := range c.Walkable {
		px, py := int64(x), int64(y)
		if r[0] < 0 && x >= 0x8000 {
			px = int64(int16(x))
		}
		if r[1] < 0 && y >= 0x8000 {
			py = int64(int16(y))
		}
		if px >= int64(r[0]) && px < int64(r[0])+int64(r[2]) && py >= int64(r[1]) && py < int64(r[1])+int64(r[3]) {
			return true
		}
	}
	return false
}

func LoadTownArea(path string) (TownArea, error) {
	var c TownArea
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if len(c.Source.Checksum) != 64 || len(c.TownSHA256) != 64 || len(c.MapSHA256) != 64 || len(c.Walkable) == 0 {
		return c, fmt.Errorf("incomplete town catalog")
	}
	return c, nil
}
