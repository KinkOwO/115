package activity

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

// HoneyNative is the grouped 115US source grammar, not CN's day-of-week
// interval grammar. Keeping the blocks is essential: 1473667c0 builds one
// nested reward vector per [reward data] block and sums its milliseconds.
// This read model deliberately does not infer claim eligibility or loop reset.
type HoneyNative struct {
	SourceChecksum, Path, Table, UI, WeeklyStart string
	Dates                                        Dates
	PCRoom                                       bool
	PostalType                                   uint32
	MaxOffsets                                   map[string]uint32
	Groups                                       map[string]HoneyNativeGroup
	// Retain auxiliary source fields without assigning unverified semantics.
	SourceTokens []pvf.Token
}

type HoneyNativeReward struct {
	Offset, Item, Count, Option, Milliseconds uint32
}

type HoneyNativeBlock struct {
	Rewards           []HoneyNativeReward
	CumulativeSeconds uint32
}

type HoneyNativeGroup struct {
	Loop, ApplyUnlimit uint32
	Blocks             []HoneyNativeBlock
	Unlimit            []HoneyNativeReward
}

// HoneyNativeEventPath follows the indexed path that the current native
// event313 actor actually loads. Weekly387 and daily365 have different,
// hardcoded path loaders and must not reuse this binding rule.
func HoneyNativeEventPath(index []pvf.Token) (string, error) {
	if len(index)%2 != 0 {
		return "", fmt.Errorf("invalid native event index width")
	}
	var found string
	for i := 0; i < len(index); i += 2 {
		id, e := num(index[i])
		if e != nil {
			return "", e
		}
		p, e := text(index[i+1])
		if e != nil {
			return "", e
		}
		if id == 313 {
			if found != "" {
				return "", fmt.Errorf("duplicate native event313 binding")
			}
			p = strings.ToLower(strings.ReplaceAll(p, "\\", "/"))
			if strings.HasPrefix(p, "/") || strings.Contains(p, "..") || !strings.HasSuffix(p, ".evt") {
				return "", fmt.Errorf("invalid native event313 path")
			}
			found = p
		}
	}
	if found == "" {
		return "", fmt.Errorf("native event313 not indexed")
	}
	return found, nil
}

func honeyNativeRows(t []pvf.Token) ([]HoneyNativeReward, error) {
	if len(t)%5 != 0 {
		return nil, fmt.Errorf("invalid native honey reward width")
	}
	var out []HoneyNativeReward
	for i := 0; i < len(t); i += 5 {
		v, e := numbers(t[i : i+5])
		if e != nil {
			return nil, e
		}
		// Native rows address sixteen byte counters and signed millisecond sums.
		if v[0] >= 16 || v[1] < 2 || v[2] == 0 || v[4] > 0x7fffffff {
			return nil, fmt.Errorf("native honey reward exceeds reader bounds")
		}
		out = append(out, HoneyNativeReward{v[0], v[1], v[2], v[3], v[4]})
	}
	return out, nil
}

func ParseHoneyNative(t []pvf.Token) (c HoneyNative, e error) {
	c.SourceTokens = append([]pvf.Token(nil), t...)
	if c.Table, c.Dates, _, e = control(t, 3); e != nil {
		return
	}
	if c.UI, e = label(t, "[ui file]"); e != nil {
		return
	}
	if c.WeeklyStart, e = label(t, "[explicit weekly start time]"); e != nil {
		return
	}
	if c.PCRoom, e = boolean(t, "[pc room event]"); e != nil {
		return
	}
	if c.PostalType, e = number(t, "[reward postal type]"); e != nil {
		return
	}
	// These nested layouts change the selection scope. Do not flatten them
	// into the main default groups and accidentally merge different calendars.
	for _, cell := range t {
		if cell.Type == 3 && (cell.Text == "[day of week reward info]" || cell.Text == "[day reward info]") {
			return c, fmt.Errorf("native honey calendar override requires separate binding")
		}
	}
	v, e := one(t, "[max reward offset count]")
	if e != nil || len(v)%2 != 0 {
		return c, fmt.Errorf("invalid native honey offset bounds")
	}
	c.MaxOffsets = map[string]uint32{}
	for i := 0; i < len(v); i += 2 {
		k, e := text(v[i])
		if e != nil {
			return c, e
		}
		n, e := num(v[i+1])
		if e != nil || n == 0 || n > 16 || c.MaxOffsets[k] != 0 {
			return c, fmt.Errorf("invalid native honey offset bound")
		}
		c.MaxOffsets[k] = n
	}
	groups, e := section(t, "[reward info]")
	if e != nil || len(groups) == 0 {
		return c, fmt.Errorf("native honey reward groups missing")
	}
	c.Groups = map[string]HoneyNativeGroup{}
	for _, tokens := range groups {
		k, e := label(tokens, "[reward type]")
		if e != nil {
			return c, e
		}
		if k != "daily" && k != "period" && k != "specified" && k != "extend_daily" {
			return c, fmt.Errorf("unknown native honey group %s", k)
		}
		if _, ok := c.Groups[k]; ok {
			return c, fmt.Errorf("duplicate native honey group %s", k)
		}
		var group HoneyNativeGroup
		if group.Loop, e = number(tokens, "[loop]"); e != nil {
			return c, e
		}
		blocks, e := section(tokens, "[reward data]")
		if e != nil || len(blocks) == 0 {
			return c, fmt.Errorf("native honey grouped rewards missing")
		}
		var seconds uint64
		var counts [16]uint32
		for _, block := range blocks {
			rows, e := honeyNativeRows(block)
			if e != nil || len(rows) == 0 {
				return c, fmt.Errorf("invalid native honey reward block")
			}
			var millis uint64
			for _, r := range rows {
				millis += uint64(r.Milliseconds)
				counts[r.Offset]++
				if millis > 0x7fffffff || counts[r.Offset] > 255 || (c.MaxOffsets[k] > 0 && r.Offset >= c.MaxOffsets[k]) {
					return c, fmt.Errorf("native honey grouped counter overflow")
				}
			}
			// The native parser divides each block before adding its prefix.
			seconds += millis / 1000
			if seconds > 0x7fffffff {
				return c, fmt.Errorf("native honey grouped timer overflow")
			}
			group.Blocks = append(group.Blocks, HoneyNativeBlock{rows, uint32(seconds)})
		}
		for _, cell := range tokens {
			if cell.Type == 3 && cell.Text == "[apply unlimit flag]" {
				if group.ApplyUnlimit, e = number(tokens, cell.Text); e != nil {
					return c, e
				}
			}
		}
		unlimit, e := section(tokens, "[unlimit reward data]")
		if e != nil || len(unlimit) > 1 {
			return c, fmt.Errorf("invalid native honey unlimit group")
		}
		if len(unlimit) == 1 {
			if group.Unlimit, e = honeyNativeRows(unlimit[0]); e != nil {
				return c, e
			}
		}
		c.Groups[k] = group
	}
	return c, nil
}
