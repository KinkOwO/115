package gamedata

import (
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"fmt"
)

func (s *Source) Membership(index catalog.ItemIndex) (*activity.MembershipRules, error) {
	if s == nil || s.archive == nil || index.Source.Checksum != s.Snapshot().Checksum {
		return nil, fmt.Errorf("membership archive/index mismatch")
	}
	tokens, e := s.archive.Tokens(activity.MembershipPath)
	if e != nil {
		return nil, e
	}
	rules, e := activity.ParseMembership(tokens)
	if e != nil {
		return nil, e
	}
	rules.SourceChecksum = s.Snapshot().Checksum
	rules.ItemNames = map[uint32]string{}
	if e = rules.Validate(); e != nil {
		return nil, e
	}
	dungeonScript, e := catalog.ReadScript(s.archive, "list/dungeon.lst")
	if e != nil {
		return nil, e
	}
	dungeonRows, e := catalog.ParseIndex(dungeonScript.Cells)
	if e != nil {
		return nil, e
	}
	dungeons := map[uint32]bool{}
	for _, row := range dungeonRows {
		dungeons[row.ID] = true
	}
	for _, benefit := range rules.Paybacks {
		if !dungeons[benefit.Dungeon] {
			return nil, fmt.Errorf("membership benefit dungeon %d absent from this source", benefit.Dungeon)
		}
	}
	for _, r := range rules.Rewards() {
		item, ok := index.Items[r.Item]
		if !ok || item.Kind != "stackable" || item.Path == "" || (item.StackLimit > 0 && r.Count > item.StackLimit) {
			return nil, fmt.Errorf("membership reward %d unavailable", r.Item)
		}
		if _, ok = s.archive.FindFile(item.Path); !ok {
			return nil, fmt.Errorf("membership reward script missing")
		}
		if _, exists := rules.ItemNames[r.Item]; !exists {
			name, found, err := s.archive.LocalizedNameByPath(item.Path)
			if err != nil || !found || name == "" {
				return nil, fmt.Errorf("membership item %d name missing: %v", r.Item, err)
			}
			rules.ItemNames[r.Item] = name
		}
	}
	return &rules, nil
}
