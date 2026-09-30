package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
)

// SelectionBoxPolicy contains only the bounded range the server loads. Paths,
// hashes and all offered item/category/count data remain archive facts.
type SelectionBoxPolicy struct {
	Version   int      `json:"version"`
	Templates []uint32 `json:"templates"`
}

func ReadSelectionBoxPolicy(path string) (SelectionBoxPolicy, error) {
	var p SelectionBoxPolicy
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("selection box policy has trailing data")
	}
	if p.Version != 1 || len(p.Templates) == 0 {
		return p, fmt.Errorf("empty or invalid selection box policy")
	}
	seen := map[uint32]bool{}
	for _, id := range p.Templates {
		if id == 0 || seen[id] {
			return p, fmt.Errorf("invalid or duplicate selection box policy template %d", id)
		}
		seen[id] = true
	}
	return p, nil
}

func ImportSelectionBoxes(a *pvf.Archive, index ItemIndex, policy SelectionBoxPolicy) (*SelectionBoxes, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum || policy.Version != 1 || len(policy.Templates) == 0 {
		return nil, fmt.Errorf("selection boxes require matching source and bounded policy")
	}
	s := SelectionBoxes{Model: SelectionBoxModel, Bounded: true, Source: a.Snapshot(), Boxes: map[string]SelectionBox{}}
	ids := append([]uint32(nil), policy.Templates...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		item, ok := index.Items[id]
		if id == 0 || i > 0 && ids[i-1] == id || !ok || item.ID != id || item.StackableType != "[booster selection]" {
			return nil, fmt.Errorf("selection box %d has no unique indexed source binding", id)
		}
		script, err := ReadScript(a, item.Path)
		if err != nil {
			return nil, fmt.Errorf("selection box %d: %w", id, err)
		}
		categories, fixed := ParseSelectionCells(script.Cells)
		if len(categories) == 0 {
			if fixed {
				s.Fixed = append(s.Fixed, id)
			} else {
				s.Unparsed = append(s.Unparsed, id)
			}
			continue
		}
		s.Boxes[strconv.FormatUint(uint64(id), 10)] = SelectionBox{Template: id, Path: script.Path, SHA256: script.SHA256, Categories: categories}
	}
	return NewSelectionBoxes(s)
}
