package activity

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const DailyNativePath = "event/dailymissionevent.evt"

// ParseDailyNative reads the current 115US physical source path and dated
// seven-cell reward rows. It deliberately does not reinterpret CN's five-cell
// integrated attendance grammar as this older native calendar protocol.
func ParseDailyNative(t []pvf.Token) (c Daily, e error) {
	if c.Table, c.Dates, c.Mode, e = control(t, 3); e != nil {
		return
	}
	c.Mode = "calendar"
	if c.UI, e = label(t, "[ui file]"); e != nil {
		return
	}
	if c.MinimumLevel, e = number(t, "[level limit]"); e != nil {
		return
	}
	if c.TodayBonus, e = number(t, "[today bonus]"); e != nil {
		return
	}
	if c.MissionType, e = number(t, "[mission type]"); e != nil {
		return
	}
	v, e := value(t, "[proper dungeon count]")
	if e != nil {
		return c, e
	}
	if c.ProperDungeonCounts, e = numbers(v); e != nil {
		return
	}
	if len(c.ProperDungeonCounts) != 2 {
		return c, fmt.Errorf("invalid native proper clear counts")
	}
	v, e = one(t, "[daily reward]")
	if e != nil {
		return c, e
	}
	if len(v) == 0 || len(v)%7 != 0 {
		return c, fmt.Errorf("invalid dated daily rewards")
	}
	for i := 0; i < len(v); i += 7 {
		var row Milestone
		row.Index, e = num(v[i])
		if e != nil || row.Index != uint32(len(c.Days)+1) {
			return c, fmt.Errorf("invalid native daily sequence")
		}
		row.Required = row.Index
		row.Dates, e = dates(v[i+1 : i+3])
		if e != nil {
			return c, e
		}
		row.Reward, e = reward(v[i+3:i+7], true)
		if e != nil {
			return c, e
		}
		c.Days = append(c.Days, row)
	}
	if c.Cumulative, e = milestones(t, "[accumulated reward]", false); e != nil {
		return
	}
	mode, e := number(t, "[accumulated reward type]")
	if e != nil {
		return c, e
	}
	if mode != 0 {
		return c, fmt.Errorf("unverified native accumulated reward mode")
	}
	c.UseCumulative = true
	return c, nil
}
