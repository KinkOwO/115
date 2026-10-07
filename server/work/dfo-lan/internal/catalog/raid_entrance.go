package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// RaidEntrance contains entry facts from the native content script. It does
// not imply that phase, boss or reward handling has been implemented.
type RaidEntrance struct {
	Path                              string
	MemberMax, StartMinimum, PartyMax uint32
	Waiting                           TownArea
	Bakal                             *BakalRaidRules
}

// Only canonical scripts with a confirmed channel town are bound. Variants
// must not silently substitute their base raid's mechanics.
func ImportRaidEntrances(a *pvf.Archive, directory *ChannelDirectory) (map[uint32]RaidEntrance, error) {
	out := map[uint32]RaidEntrance{}
	paths := []string{"etc/raid/siroco/siroco.etc", "contents/2021/ozma_raid/etc/ozma.etc", "contents/2022/bakalraid/etc/bakal.etc", "contents/2024/asrahanraid/etc/asrahan.etc", "contents/2025/artificialgodraid/etc/artificialgod.etc"}
	for _, path := range paths {
		if _, ok := a.FindFile(path); !ok {
			continue
		}
		ts, err := a.Tokens(path)
		if err != nil {
			return nil, err
		}
		values := map[string][]uint32{}
		for i, t := range ts {
			switch t.Text {
			case "[RAID MEMBER MAX]", "[RAID START MINIMUM MEMBER]", "[RAID PARTY MAX]", "[WAITING ROOM]":
				n := 1
				if t.Text == "[WAITING ROOM]" {
					n = 2
				}
				if _, dup := values[t.Text]; dup {
					return nil, fmt.Errorf("%s: duplicate %s", path, t.Text)
				}
				for j := 1; j <= n; j++ {
					if i+j >= len(ts) || ts[i+j].Type != 0 || ts[i+j].Value < 0 {
						return nil, fmt.Errorf("%s: invalid %s", path, t.Text)
					}
					values[t.Text] = append(values[t.Text], uint32(ts[i+j].Value))
				}
			}
		}
		if len(values["[WAITING ROOM]"]) != 2 || len(values["[RAID MEMBER MAX]"]) != 1 || len(values["[RAID START MINIMUM MEMBER]"]) != 1 || len(values["[RAID PARTY MAX]"]) != 1 {
			return nil, fmt.Errorf("%s: incomplete raid entry rules", path)
		}
		r := RaidEntrance{Path: path, MemberMax: values["[RAID MEMBER MAX]"][0], StartMinimum: values["[RAID START MINIMUM MEMBER]"][0], PartyMax: values["[RAID PARTY MAX]"][0]}
		if r.MemberMax == 0 || r.MemberMax > 255 || r.StartMinimum == 0 || r.StartMinimum > r.MemberMax || r.PartyMax == 0 {
			return nil, fmt.Errorf("%s: invalid raid limits", path)
		}
		loc := values["[WAITING ROOM]"]
		var matched []uint32
		for _, kind := range directory.Types() {
			attr, _ := directory.Attributes(kind)
			if attr.IsRaid && attr.Town == int(loc[0]) {
				matched = append(matched, kind)
			}
		}
		if len(matched) == 0 {
			continue
		}
		waiting, err := ImportTownArea(a, loc[0], loc[1])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if len(waiting.Walkable) == 0 {
			return nil, fmt.Errorf("%s: empty waiting room", path)
		}
		r.Waiting = waiting
		if path == "contents/2022/bakalraid/etc/bakal.etc" {
			r.Bakal, err = importBakalRaidRules(ts)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			if err = loadBakalRaidTables(a, r.Bakal); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
		}
		for _, kind := range matched {
			if _, dup := out[kind]; dup {
				return nil, fmt.Errorf("ambiguous raid entry for channel %d", kind)
			}
			bound := r
			// Only the normal channel uses the imported opening behavior.
			// Hard mode has a separate initialization in the same script.
			if kind != 82 {
				bound.Bakal = nil
			}
			out[kind] = bound
		}
	}
	return out, nil
}
