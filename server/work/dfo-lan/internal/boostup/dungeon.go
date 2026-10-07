package boostup

import "fmt"

func (c *Catalog) GuideDungeon(s Training, job string, grow byte, variant uint32) (uint32, error) {
	row, e := c.current(s)
	if e != nil {
		return 0, e
	}
	if row.Guide != "dungeon" || row.Dungeon == 0 || s.Phase > 1 || variant > 1 {
		return 0, fmt.Errorf("current boost step has no pending guide dungeon")
	}
	id := row.Dungeon
	v := values(row.GuideCells, "[specific dungeon index]")
	if len(v)%4 != 0 {
		return 0, fmt.Errorf("invalid profession guide routes")
	}
	matches := 0
	for i := 0; i < len(v); i += 4 {
		if v[i].Type != 6 || v[i+1].Type != 0 || v[i+2].Type != 0 || v[i+3].Type != 0 || v[i+3].Value <= 0 {
			return 0, fmt.Errorf("invalid guide route row")
		}
		if v[i].Text == job && v[i+1].Value == int32(grow) && v[i+2].Value == int32(variant) {
			id = uint32(v[i+3].Value)
			matches++
		}
	}
	if matches > 1 {
		return 0, fmt.Errorf("ambiguous profession guide route")
	}
	return id, nil
}
