package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"strings"
)

// RaidEntrance contains entry facts from the native content script. It does
// not imply that phase, boss or reward handling has been implemented.
type RaidEntrance struct {
	Path                              string
	MemberMax, StartMinimum, PartyMax uint32
	Waiting                           TownArea
	// Recruiting is an explicit content-owned preparation area. A missing
	// value keeps older raids' existing channel-town policy unchanged.
	Recruiting                  *TownArea
	Bakal                       *BakalRaidRules
	BakalHard                   *BakalRaidRules
	HardPartyMax, HardLimitFame uint32
	Delezie                     *DelezieRaidRules
	Asrahan                     *AsrahanRaidRules
}

// Only canonical scripts with a confirmed channel town are bound. Variants
// must not silently substitute their base raid's mechanics.
func ImportRaidEntrances(a *pvf.Archive, directory *ChannelDirectory) (map[uint32]RaidEntrance, error) {
	if a == nil || directory == nil {
		return nil, fmt.Errorf("raid entrance source or channel directory missing")
	}
	out := map[uint32]RaidEntrance{}
	paths := []string{"etc/raid/siroco/siroco.etc", "contents/2021/ozma_raid/etc/ozma.etc", "contents/2022/bakalraid/etc/bakal.etc", "contents/2024/asrahanraid/etc/asrahan.etc", "contents/2025/artificialgodraid/etc/artificialgod.etc", "contents/2025/inaeduskwar/etc/inaeduskwar.etc", "contents/2025/delezie_of_plague/etc/delezieraid.normal.ctp"}
	for _, path := range paths {
		if _, ok := a.FindFile(path); !ok {
			continue
		}
		var ts []pvf.Token
		var explicitChannels []uint32
		var err error
		values := map[string][]uint32{}
		var compiled *pvf.CTPTable
		if strings.HasSuffix(path, ".ctp") {
			table, e := a.CTP(path)
			if e != nil {
				return nil, e
			}
			values, explicitChannels, err = raidCTPEntryValues(table)
			compiled = table
		} else {
			ts, err = a.Tokens(path)
		}
		if err != nil {
			return nil, err
		}
		for i, t := range ts {
			switch t.Text {
			case "[RAID MEMBER MAX]", "[RAID START MINIMUM MEMBER]", "[RAID PARTY MAX]", "[WAITING ROOM]", "[RECRUITING ROOM]":
				n := 1
				if t.Text == "[WAITING ROOM]" || t.Text == "[RECRUITING ROOM]" {
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
			explicit := false
			for _, channel := range explicitChannels {
				explicit = explicit || channel == kind
			}
			if (attr.IsRaid || explicit) && attr.Town == int(loc[0]) {
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
		if loc, exists := values["[RECRUITING ROOM]"]; exists {
			if len(loc) != 2 || loc[0] != waiting.TownID || loc[1] == waiting.AreaID {
				return nil, fmt.Errorf("%s: invalid recruiting/waiting room binding", path)
			}
			room, err := ImportTownArea(a, loc[0], loc[1])
			if err != nil || len(room.Walkable) == 0 {
				return nil, fmt.Errorf("%s: unavailable recruiting room: %v", path, err)
			}
			r.Recruiting = &room
		} else if compiled != nil {
			return nil, fmt.Errorf("%s: missing native recruiting room", path)
		}
		if path == "contents/2022/bakalraid/etc/bakal.etc" {
			r.Bakal, err = importBakalRaidMode(ts, BakalNormal)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			if err = loadBakalRaidTables(a, r.Bakal); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			r.BakalHard, err = importBakalRaidMode(ts, BakalHard)
			if err != nil {
				return nil, fmt.Errorf("%s hard: %w", path, err)
			}
			if err = loadBakalRaidTables(a, r.BakalHard); err != nil {
				return nil, fmt.Errorf("%s hard: %w", path, err)
			}
			for i, t := range ts {
				if t.Text != "[HARD RAID PARTY MAX]" && t.Text != "[HARD LIMIT FAME]" {
					continue
				}
				if i+1 >= len(ts) || ts[i+1].Type != 0 || ts[i+1].Value <= 0 {
					return nil, fmt.Errorf("invalid %s", t.Text)
				}
				if t.Text == "[HARD RAID PARTY MAX]" {
					r.HardPartyMax = uint32(ts[i+1].Value)
				} else {
					r.HardLimitFame = uint32(ts[i+1].Value)
				}
			}
			if r.HardPartyMax == 0 || r.HardLimitFame == 0 {
				return nil, fmt.Errorf("incomplete hard raid entry")
			}
		}
		if compiled != nil {
			r.Delezie, err = importDelezieRaidRules(a, compiled)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
		}
		if path == "contents/2024/asrahanraid/etc/asrahan.etc" {
			r.Asrahan, err = importAsrahanRaidRules(a, path)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			r.HardPartyMax, r.HardLimitFame = r.Asrahan.HardPartyMax, r.Asrahan.HardLimitFame
		}
		for _, kind := range matched {
			if _, dup := out[kind]; dup {
				return nil, fmt.Errorf("ambiguous raid entry for channel %d", kind)
			}
			bound := r
			// Both native Bakal difficulties share channel82. Channel83 is
			// Total War and must not borrow either Bakal executor.
			if kind != 82 {
				bound.Bakal = nil
				bound.BakalHard = nil
			}
			out[kind] = bound
		}
	}
	return out, nil
}

// Delezie's common entry header is in the base CTP, while single/match/hard
// tables contain separate phase trees. Import only the top-level entry facts;
// never merge those trees into Bakal rules. Channel IDs come from the base
// table's native raid-list difficulty records, not from the town alone.
func raidCTPEntryValues(table *pvf.CTPTable) (map[string][]uint32, []uint32, error) {
	values := map[string][]uint32{}
	var channels []uint32
	for _, row := range table.Records {
		n := 0
		if row.Parent == -1 {
			switch row.Name {
			case "[RAID MEMBER MAX]", "[RAID START MINIMUM MEMBER]", "[RAID PARTY MAX]":
				n = 1
			case "[WAITING ROOM]", "[RECRUITING ROOM]":
				n = 2
			}
		}
		channelRow := row.Name == "[CHANNEL TYPE]" && row.Parent >= 0 && row.Parent < len(table.Records)
		if channelRow {
			parent := table.Records[row.Parent]
			channelRow = parent.Name == "[INFO]" && parent.Parent >= 0 && parent.Parent < len(table.Records) && table.Records[parent.Parent].Name == "[RAID LIST WINDOW DIFFICULTY INFO]"
		}
		if channelRow {
			n = 1
		}
		if n == 0 {
			continue
		}
		if len(row.Cells) != n {
			return nil, nil, fmt.Errorf("%s: invalid %s cell count", table.Path, row.Name)
		}
		var numbers []uint32
		for _, cell := range row.Cells {
			v := cell.Float
			if cell.Kind != "float" || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > math.MaxUint32 || math.Trunc(v) != v {
				return nil, nil, fmt.Errorf("%s: invalid %s integer", table.Path, row.Name)
			}
			numbers = append(numbers, uint32(v))
		}
		if channelRow {
			if numbers[0] == 0 || numbers[0] > 255 {
				return nil, nil, fmt.Errorf("%s: invalid native channel type", table.Path)
			}
			channels = append(channels, numbers[0])
			continue
		}
		if _, dup := values[row.Name]; dup {
			return nil, nil, fmt.Errorf("%s: duplicate %s", table.Path, row.Name)
		}
		values[row.Name] = numbers
	}
	if len(channels) == 0 {
		return nil, nil, fmt.Errorf("%s: no native raid-list channel bindings", table.Path)
	}
	return values, channels, nil
}
