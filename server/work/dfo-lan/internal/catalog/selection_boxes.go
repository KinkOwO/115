package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// SelectionBoxModel identifies the exported selection-box artifact.
const SelectionBoxModel = "source-selection-boxes-v1"

// SelectionItem is one entry of a box's [equipment] list. The source writes
// (id, count) pairs and the client hands the picked id back in its request.
type SelectionItem struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}

// SelectionCategory is one [booster select category] block: the (job, growtype)
// pair the client sends with its request, and the closed set of items the
// server validates that pick against. Sections records which content blocks the
// source carried — only [equipment] is modelled so far, the [avatar]/[etc]
// blocks still travel the generic destination path.
type SelectionCategory struct {
	Category  [2]byte         `json:"category"`
	Grade     uint32          `json:"grade,omitempty"`
	Recommend []uint32        `json:"recommend,omitempty"`
	Items     []SelectionItem `json:"items"`
	Sections  []string        `json:"sections,omitempty"`
}

// SelectionBox is a [booster selection] box whose contents the player picks.
type SelectionBox struct {
	Template   uint32              `json:"template"`
	Path       string              `json:"path"`
	SHA256     string              `json:"sha256"`
	Categories []SelectionCategory `json:"categories"`
}

// SelectionBoxes is the exported catalog. Fixed lists templates the item index
// labels [booster selection] but whose script only carries a fixed
// [booster info] block — they must not be treated as pick-a-item boxes.
type SelectionBoxes struct {
	Model    string                  `json:"model"`
	Bounded  bool                    `json:"bounded"`
	Source   pvf.ArchiveSnapshot     `json:"source"`
	Boxes    map[string]SelectionBox `json:"boxes"`
	Fixed    []uint32                `json:"fixed"`
	Unparsed []uint32                `json:"unparsed,omitempty"`

	byTemplate map[uint32]SelectionBox
	fixed      map[uint32]bool
}

// LoadSelectionBoxes reads and validates an exported catalog. A malformed
// artifact is refused outright: silently opening boxes from a half-loaded range
// would hand out items the source never offered.
func LoadSelectionBoxes(path string) (*SelectionBoxes, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s SelectionBoxes
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	if s.Model != SelectionBoxModel {
		return nil, fmt.Errorf("unexpected selection box model %q", s.Model)
	}
	if len(s.Source.Checksum) != 64 {
		return nil, fmt.Errorf("invalid selection box source")
	}
	if len(s.Boxes) == 0 {
		return nil, fmt.Errorf("empty selection box catalog")
	}
	s.byTemplate = make(map[uint32]SelectionBox, len(s.Boxes))
	s.fixed = make(map[uint32]bool, len(s.Fixed))
	for key, box := range s.Boxes {
		id, err := strconv.ParseUint(key, 10, 32)
		if err != nil || uint32(id) != box.Template || box.Template == 0 {
			return nil, fmt.Errorf("selection box key %q does not match template %d", key, box.Template)
		}
		if box.Path == "" {
			return nil, fmt.Errorf("selection box %d lacks a source path", box.Template)
		}
		if _, err := hex.DecodeString(box.SHA256); err != nil || len(box.SHA256) != 64 {
			return nil, fmt.Errorf("selection box %d has an invalid script hash", box.Template)
		}
		if len(box.Categories) == 0 {
			return nil, fmt.Errorf("selection box %d has no categories", box.Template)
		}
		seen := map[[2]byte]bool{}
		for _, cat := range box.Categories {
			if seen[cat.Category] {
				return nil, fmt.Errorf("selection box %d repeats category %v", box.Template, cat.Category)
			}
			seen[cat.Category] = true
			for _, it := range cat.Items {
				// 真源里数量可以是材料类的 1000/10000（盒子同时带装备与堆叠物），
				// 所以这里只要求非零：上限由发放路径按物品类别自己把关。
				if it.Template == 0 || it.Count == 0 {
					return nil, fmt.Errorf("selection box %d has an invalid item", box.Template)
				}
			}
		}
		s.byTemplate[box.Template] = box
	}
	for _, id := range s.Fixed {
		s.fixed[id] = true
	}
	return &s, nil
}

// ByTemplate reports the source definition of a pick-a-item box.
func (s *SelectionBoxes) ByTemplate(template uint32) (SelectionBox, bool) {
	if s == nil {
		return SelectionBox{}, false
	}
	box, ok := s.byTemplate[template]
	return box, ok
}

// IsFixed reports a template the item index mislabels as [booster selection]
// but whose script only carries a fixed [booster info] block.
func (s *SelectionBoxes) IsFixed(template uint32) bool {
	if s == nil {
		return false
	}
	return s.fixed[template]
}

// Resolve looks the player's picks up in the box's source range. It returns the
// matched items, the picks the source does not list, and whether the source
// carried an [equipment] set for that category at all.
//
// The caller must NOT reject a request because of missing picks: this table is
// exported from server/work/client-build/Script.inner.pvf (760,530,763 bytes)
// while the running client loads its own Script.pvf (761,337,702 bytes) — a
// different build. A pick the client's UI legitimately offered can therefore be
// absent from the exported list, and refusing it would break a working box. So
// the result is observation data for the log, not a gate.
//
// 2026-09-23 live check (test-jh, box 10417798): the client asked for 100051397
// — the first source entry — and the server granted exactly that.
func (s *SelectionBoxes) Resolve(template uint32, category [2]byte, picks []uint32) (items []SelectionItem, missing []uint32, checked bool) {
	box, ok := s.ByTemplate(template)
	if !ok {
		return nil, nil, false
	}
	var block *SelectionCategory
	for i := range box.Categories {
		if box.Categories[i].Category == category {
			block = &box.Categories[i]
			break
		}
	}
	if block == nil || len(block.Items) == 0 {
		return nil, nil, false
	}
	index := make(map[uint32]uint32, len(block.Items))
	for _, it := range block.Items {
		index[it.Template] = it.Count
	}
	for _, pick := range picks {
		if count, ok := index[pick]; ok {
			if count == 0 {
				count = 1
			}
			items = append(items, SelectionItem{Template: pick, Count: count})
			continue
		}
		missing = append(missing, pick)
	}
	return items, missing, true
}
