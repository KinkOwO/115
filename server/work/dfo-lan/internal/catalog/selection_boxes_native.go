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

// SelectionBoxPolicy carries only the scope the source cannot express: a small
// whitelist of templates that are selection boxes but whose [stackable type] is
// not [booster selection]. The offered categories, items and counts are always
// archive facts discovered from the item index, not policy data.
type SelectionBoxPolicy struct {
	Version int `json:"version"`
	// Whitelist adds templates the [stackable type] scan cannot reach.
	Whitelist []uint32 `json:"whitelist,omitempty"`
	// Templates is the pre-discovery field. Entries are treated exactly like
	// whitelist entries so existing profiles keep loading during migration.
	Templates  []uint32 `json:"templates,omitempty"`
	Provenance string   `json:"provenance,omitempty"`
}

func (p SelectionBoxPolicy) namedTemplates() []uint32 {
	out := make([]uint32, 0, len(p.Whitelist)+len(p.Templates))
	out = append(out, p.Whitelist...)
	out = append(out, p.Templates...)
	return out
}

func ReadSelectionBoxPolicy(path string) (SelectionBoxPolicy, error) {
	if path == "" {
		return SelectionBoxPolicy{Version: 1}, nil
	}
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
	if p.Version != 1 {
		return p, fmt.Errorf("invalid selection box policy version %d", p.Version)
	}
	seen := map[uint32]bool{}
	for _, id := range p.namedTemplates() {
		if id == 0 || seen[id] {
			return p, fmt.Errorf("invalid or duplicate selection box policy template %d", id)
		}
		seen[id] = true
	}
	return p, nil
}

// ImportSelectionBoxes discovers every stackable the item index labels
// [booster selection], unions the policy's whitelist, and reads each script.
// A single malformed or unmodelled candidate is classified (fixed / unparsed /
// rejected) instead of aborting the whole catalog, so the source remains the
// sole content truth and one bad script cannot hide the rest.
func ImportSelectionBoxes(a *pvf.Archive, index ItemIndex, policy SelectionBoxPolicy) (*SelectionBoxes, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum || policy.Version != 1 {
		return nil, fmt.Errorf("selection boxes require matching source and version 1 policy")
	}
	if err := index.Validate(); err != nil {
		return nil, err
	}
	candidates, err := selectionBoxCandidates(index, policy)
	if err != nil {
		return nil, err
	}
	read := func(path string) (ScriptRecord, error) { return ReadScript(a, path) }
	return assembleSelectionBoxes(candidates, read, a.Snapshot())
}

// selectionBoxCandidates is the discovery half: every stackable the index
// labels [booster selection], plus any template the policy whitelists. A
// whitelisted template with no indexed binding is a hard error so a typo
// cannot silently drop a box.
func selectionBoxCandidates(index ItemIndex, policy SelectionBoxPolicy) (map[uint32]string, error) {
	candidates := map[uint32]string{}
	for id, item := range index.Items {
		if item.Kind == "stackable" && item.StackableType == "[booster selection]" {
			candidates[id] = item.Path
		}
	}
	for _, id := range policy.namedTemplates() {
		item, ok := index.Items[id]
		if id == 0 || !ok || item.ID != id {
			return nil, fmt.Errorf("selection box policy template %d has no indexed source binding", id)
		}
		candidates[id] = item.Path
	}
	return candidates, nil
}

type selectionScriptReader func(path string) (ScriptRecord, error)

// assembleSelectionBoxes is the archive-independent half of the import: it
// classifies candidate scripts and binds the runtime catalog. Keeping it
// separate lets the whitelist + discovery hybrid be tested without an archive.
func assembleSelectionBoxes(candidates map[uint32]string, read selectionScriptReader, source pvf.ArchiveSnapshot) (*SelectionBoxes, error) {
	ids := make([]uint32, 0, len(candidates))
	for id := range candidates {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	s := SelectionBoxes{Model: SelectionBoxModel, Bounded: true, Source: source, Boxes: map[string]SelectionBox{}}
	for _, id := range ids {
		script, err := read(candidates[id])
		if err != nil {
			s.Unparsed = append(s.Unparsed, id)
			continue
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
		box, err := normalizeSelectionBox(SelectionBox{Template: id, Path: script.Path, SHA256: script.SHA256, Categories: categories})
		if err != nil {
			s.Rejected = append(s.Rejected, id)
			continue
		}
		s.Boxes[strconv.FormatUint(uint64(id), 10)] = box
	}
	sort.Slice(s.Fixed, func(i, j int) bool { return s.Fixed[i] < s.Fixed[j] })
	sort.Slice(s.Unparsed, func(i, j int) bool { return s.Unparsed[i] < s.Unparsed[j] })
	sort.Slice(s.Rejected, func(i, j int) bool { return s.Rejected[i] < s.Rejected[j] })
	return NewSelectionBoxes(s)
}
