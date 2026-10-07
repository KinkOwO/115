package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
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
// Reinforce/Refine are the source's own crafted status for that block
// ([booster equipment upgrade] / [booster equipment separate]); a gift box that
// hands out ready-built gear declares them, and the grant path stamps them.
type SelectionCategory struct {
	Category  [2]byte         `json:"category"`
	Grade     uint32          `json:"grade,omitempty"`
	Reinforce uint32          `json:"reinforce,omitempty"`
	Refine    uint32          `json:"refine,omitempty"`
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
// Unparsed lists candidates with no modelled category or fixed block, and
// Rejected lists candidates the per-box validation refused (for example a
// category pair repeated with different contents). Both are recorded instead
// of aborting the whole catalog so one malformed script cannot hide the rest.
type SelectionBoxes struct {
	Model    string                  `json:"model"`
	Bounded  bool                    `json:"bounded"`
	Source   pvf.ArchiveSnapshot     `json:"source"`
	Boxes    map[string]SelectionBox `json:"boxes"`
	Fixed    []uint32                `json:"fixed"`
	Unparsed []uint32                `json:"unparsed,omitempty"`
	Rejected []uint32                `json:"rejected,omitempty"`

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
	return NewSelectionBoxes(s)
}

// NewSelectionBoxes validates and binds the same runtime indexes for either
// a native source projection or an exported baseline.
func NewSelectionBoxes(s SelectionBoxes) (*SelectionBoxes, error) {
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
		normalized, err := normalizeSelectionBox(box)
		if err != nil {
			return nil, fmt.Errorf("selection box %d %w", box.Template, err)
		}
		s.byTemplate[normalized.Template] = normalized
	}
	for _, id := range s.Fixed {
		s.fixed[id] = true
	}
	return &s, nil
}

// normalizeSelectionBox validates one box's bound content and drops a category
// pair the source repeats with identical, fully-modelled content: the client
// sees a single block there, so keeping both would only trip the duplicate
// guard. A repeated pair whose contents differ, or that carries a block this
// parser does not model (avatar/etc), is refused — which copy the client reads
// is not knowable from the script alone.
func normalizeSelectionBox(box SelectionBox) (SelectionBox, error) {
	if box.Path == "" {
		return box, fmt.Errorf("lacks a source path")
	}
	if _, err := hex.DecodeString(box.SHA256); err != nil || len(box.SHA256) != 64 {
		return box, fmt.Errorf("has an invalid script hash")
	}
	if len(box.Categories) == 0 {
		return box, fmt.Errorf("has no categories")
	}
	seen := map[[2]byte]int{}
	deduped := make([]SelectionCategory, 0, len(box.Categories))
	for _, cat := range box.Categories {
		if idx, ok := seen[cat.Category]; ok {
			// Only a fully-modelled, byte-identical repeat is safe to drop. An
			// [avatar]/[etc] block the parser does not model could differ while
			// looking empty here, so it stays a conflict rather than a guess.
			if hasUnmodelledSection(cat) || hasUnmodelledSection(deduped[idx]) || !sameSelectionCategory(deduped[idx], cat) {
				return box, fmt.Errorf("repeats category %v with contents this parser cannot compare", cat.Category)
			}
			continue
		}
		for _, it := range cat.Items {
			// 真源里数量可以是材料类的 1000/10000（盒子同时带装备与堆叠物），
			// 所以这里只要求非零：上限由发放路径按物品类别自己把关。
			if it.Template == 0 || it.Count == 0 {
				return box, fmt.Errorf("has an invalid item")
			}
		}
		seen[cat.Category] = len(deduped)
		deduped = append(deduped, cat)
	}
	box.Categories = deduped
	return box, nil
}

func sameSelectionCategory(a, b SelectionCategory) bool {
	return a.Category == b.Category && a.Grade == b.Grade &&
		a.Reinforce == b.Reinforce && a.Refine == b.Refine &&
		slices.Equal(a.Recommend, b.Recommend) &&
		slices.Equal(a.Items, b.Items) &&
		slices.Equal(a.Sections, b.Sections)
}

// hasUnmodelledSection reports a category carrying a content block whose items
// this parser does not read; two such blocks cannot be compared by value.
func hasUnmodelledSection(cat SelectionCategory) bool {
	for _, s := range cat.Sections {
		if s != "[equipment]" {
			return true
		}
	}
	return false
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

// CategoryStatus returns the crafted status the source declares for that
// category's equipment block: [booster equipment upgrade] (强化) and
// [booster equipment separate] (锻造). ok is false when the box, the category
// or the status is absent, so the caller leaves the granted gear uncrafted
// instead of inventing a level.
func (s *SelectionBoxes) CategoryStatus(template uint32, category [2]byte) (reinforce, refine uint32, ok bool) {
	box, found := s.ByTemplate(template)
	if !found {
		return 0, 0, false
	}
	for _, cat := range box.Categories {
		if cat.Category == category {
			if cat.Reinforce == 0 && cat.Refine == 0 {
				return 0, 0, false
			}
			return cat.Reinforce, cat.Refine, true
		}
	}
	return 0, 0, false
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
