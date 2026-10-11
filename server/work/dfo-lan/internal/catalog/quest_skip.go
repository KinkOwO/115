package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strconv"
	"strings"
)

// QuestSkip is the source's own definition of what skipping a quest may do. Two
// files carry it, and nothing else in the archive does:
//
//	list/notclearquestbyticket.lst   quest ids that may not be skipped
//	list/questitembyticket.cos       key items a skip has to hand over anyway
//
// The blacklist is mostly quests that gate on being played: 135 [side], one
// [epic], one [invisible] and 86 ids this build's quest index no longer carries,
// typed [meet npc], [seeking], [raid phase clear], [clear dungeon index] and the
// rest. The story line is skippable; that content is not.
//
// The item list is the source's fix for what a skip loses. 10164787 is quest
// 3169's reward and quest 3173's objective, 10165043 is 3178's reward and 3181's
// objective, 10165054 is 3181's reward and 3193's objective: a character who
// skipped the prerequisite would otherwise strand forever on the quest that asks
// for the item, so the list travels with the skip.
type QuestSkip struct {
	Source      pvf.ArchiveSnapshot `json:"source"`
	Blocked     map[uint32]bool     `json:"blocked"`
	Items       []uint32            `json:"items"`
	IndexHashes map[string]string   `json:"index_hashes"`
}

// ImportQuestSkip reads both tables. The .lst is a token script of plain ids; the
// .cos is UTF-16 text with one [quest item list] section, which ReadText decodes.
func ImportQuestSkip(a *pvf.Archive) (QuestSkip, error) {
	x := QuestSkip{Source: a.Snapshot(), Blocked: map[uint32]bool{}, IndexHashes: map[string]string{}}
	record, e := ResolveScript(a, "list/notclearquestbyticket.lst")
	if e != nil {
		return x, e
	}
	x.IndexHashes[record.Path] = record.SHA256
	for _, c := range record.Cells {
		if c.Type != 0 {
			return x, fmt.Errorf("notclearquestbyticket.lst carries a non-id cell")
		}
		if c.Value <= 0 {
			return x, fmt.Errorf("notclearquestbyticket.lst carries quest %d", c.Value)
		}
		x.Blocked[uint32(c.Value)] = true
	}
	if len(x.Blocked) == 0 {
		return x, fmt.Errorf("notclearquestbyticket.lst is empty")
	}
	text, e := a.ReadText("list/questitembyticket.cos")
	if e != nil {
		return x, e
	}
	x.Items, e = questItemList(text)
	if e != nil {
		return x, e
	}
	return x, nil
}

// questItemList reads the [quest item list] section. The file pads every entry to
// a fixed width with NULs and spaces, so a line is only an id once both are gone.
func questItemList(text string) ([]uint32, error) {
	var out []uint32
	seen := map[uint32]bool{}
	on := false
	for _, line := range strings.Split(text, "\n") {
		s := strings.TrimSpace(strings.ReplaceAll(line, "\x00", ""))
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "[") {
			on = s == "[quest item list]"
			continue
		}
		if !on {
			continue
		}
		v, e := strconv.ParseUint(s, 10, 32)
		if e != nil {
			return nil, fmt.Errorf("quest item list entry %q is not a template id", s)
		}
		if v == 0 {
			return nil, fmt.Errorf("quest item list carries template 0")
		}
		if seen[uint32(v)] {
			continue
		}
		seen[uint32(v)] = true
		out = append(out, uint32(v))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("quest item list is empty")
	}
	return out, nil
}
