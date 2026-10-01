package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
)

// Device modes are the original [action type] [add avatar socket] argument.
// Content is read from the active PVF rather than a fixed device ID list.
type AvatarSocketRules struct {
	Source  string
	Devices map[uint32]int32
}

func ImportAvatarSocketRules(a *pvf.Archive, index catalog.ItemIndex) (*AvatarSocketRules, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("avatar socket source mismatch")
	}
	r := &AvatarSocketRules{Source: a.Snapshot().Checksum, Devices: map[uint32]int32{}}
	var ids []uint32
	for id, item := range index.Items {
		if item.Kind == "stackable" && item.StackableType == "[waste]" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		s, e := catalog.ResolveScript(a, index.Items[id].Path)
		if e != nil {
			return nil, e
		}
		cells := avatarDisjointSection(s.Cells, "[action type]")
		if len(cells) == 0 || cells[0].Text != "[add avatar socket]" {
			continue
		}
		if len(cells) != 2 || cells[0].Type != 6 || cells[1].Type != 0 || cells[1].Value < 1 || cells[1].Value > 3 {
			return nil, fmt.Errorf("invalid source avatar socket device %d", id)
		}
		r.Devices[id] = cells[1].Value
	}
	return r, nil
}
