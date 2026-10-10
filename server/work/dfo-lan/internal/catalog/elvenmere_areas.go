package catalog

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ElvenmereArea struct {
	Index       uint32
	First, Last byte
	WaitingMap  uint32
}

func (r *ElvenmereRules) AreaForFloor(floor byte) (ElvenmereArea, error) {
	if r != nil {
		for _, area := range r.Areas {
			if floor >= area.First && floor <= area.Last {
				return area, nil
			}
		}
	}
	return ElvenmereArea{}, fmt.Errorf("elvenmere floor %d has no source area", floor)
}

func parseElvenmereAreas(text string, top byte) ([]ElvenmereArea, error) {
	inside, group := false, false
	var row ElvenmereArea
	var seen map[string]bool
	var areas []ElvenmereArea
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch line {
		case "[area group]":
			if inside || len(areas) > 0 {
				return nil, fmt.Errorf("duplicate elvenmere areas")
			}
			inside = true
			continue
		case "[/area group]":
			if group {
				return nil, fmt.Errorf("unclosed elvenmere area")
			}
			inside = false
			continue
		case "[group]":
			if inside {
				if group {
					return nil, fmt.Errorf("nested elvenmere area")
				}
				group = true
				row = ElvenmereArea{}
				seen = map[string]bool{}
			}
			continue
		case "[/group]":
			if inside {
				if !group || !seen["area index"] || !seen["floor"] || !seen["waiting map index"] {
					return nil, fmt.Errorf("incomplete elvenmere area")
				}
				areas = append(areas, row)
				group = false
			}
			continue
		}
		if !inside || !group || !strings.HasPrefix(line, "[") {
			continue
		}
		end := strings.IndexByte(line, ']')
		if end < 0 {
			return nil, fmt.Errorf("invalid elvenmere area field")
		}
		key := line[1:end]
		if key != "area index" && key != "floor" && key != "waiting map index" {
			continue
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicate elvenmere area field %s", key)
		}
		seen[key] = true
		values := strings.Fields(line[end+1:])
		width := 1
		if key == "floor" {
			width = 2
		}
		if len(values) != width {
			return nil, fmt.Errorf("invalid elvenmere area value")
		}
		v, err := strconv.ParseUint(values[0], 10, 32)
		if err != nil {
			return nil, err
		}
		switch key {
		case "area index":
			row.Index = uint32(v)
		case "waiting map index":
			row.WaitingMap = uint32(v)
		case "floor":
			last, err := strconv.ParseUint(values[1], 10, 8)
			if err != nil || v == 0 || v > last || last > uint64(top) {
				return nil, fmt.Errorf("invalid elvenmere area range")
			}
			row.First, row.Last = byte(v), byte(last)
		}
	}
	if inside || group {
		return nil, fmt.Errorf("unclosed elvenmere areas")
	}
	if len(areas) == 0 {
		return nil, nil
	} // pure reward reader fixtures omit routing
	sort.Slice(areas, func(i, j int) bool { return areas[i].First < areas[j].First })
	indices := map[uint32]bool{}
	next := byte(1)
	for _, a := range areas {
		if indices[a.Index] || a.WaitingMap == 0 || a.First != next {
			return nil, fmt.Errorf("invalid or overlapping elvenmere area coverage")
		}
		indices[a.Index] = true
		next = a.Last + 1
	}
	if next != top+1 {
		return nil, fmt.Errorf("incomplete elvenmere area coverage")
	}
	return areas, nil
}
