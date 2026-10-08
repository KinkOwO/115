package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
)

type EmblemInlayRules struct {
	Source string
	Masks  map[uint32]uint16
}

// Native 14716AA90 maps source socket names to the masks used by the avatar
// extension. M excludes S (platinum); dual-color emblems accept either color.
// [S socket] 用 avatarPlatinumSocket 常量（与补默认孔共用同一真源）。
func emblemSocketMask(name string) (uint16, bool) {
	switch name {
	case "[A socket]":
		return 1, true
	case "[B socket]":
		return 2, true
	case "[C socket]":
		return 4, true
	case "[D socket]":
		return 8, true
	case "[S socket]":
		return avatarPlatinumSocket, true
	case "[M socket]":
		return avatarMultiSocket, true
	case "[All socket]":
		return 65535, true
	}
	return 0, false
}

func ImportEmblemInlayRules(a *pvf.Archive, index catalog.ItemIndex) (*EmblemInlayRules, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("avatar emblem source mismatch")
	}
	r := &EmblemInlayRules{Source: a.Snapshot().Checksum, Masks: map[uint32]uint16{}}
	for id, item := range index.Items {
		if item.Kind != "stackable" || item.StackableType != "[avatar emblem]" {
			continue
		}
		s, err := catalog.ResolveScript(a, item.Path)
		if err != nil {
			return nil, err
		}
		active := false
		var mask uint16
		for _, token := range s.Cells {
			if token.Type == 3 {
				active = token.Text == "[avatar emblem target type]"
				continue
			}
			if active {
				m, ok := emblemSocketMask(token.Text)
				if !ok {
					return nil, fmt.Errorf("unknown avatar emblem socket %q in %s", token.Text, item.Path)
				}
				mask |= m
			}
		}
		// Emblems without source target declarations are not eligible.
		if mask != 0 {
			r.Masks[id] = mask
		}
	}
	if len(r.Masks) == 0 {
		return nil, fmt.Errorf("empty source avatar emblem masks")
	}
	return r, nil
}
