package catalog

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func selectionBoxCells(pair [2]int32, item int32) []pvf.Token {
	return []pvf.Token{
		selTag("[booster select category]"), selNum(pair[0]), selNum(pair[1]),
		selTag("[equipment]"), selNum(item), selNum(1), selTag("[/equipment]"), selTag("[/booster select category]"),
	}
}

func TestSelectionBoxCandidatesUnionsDiscoveryAndWhitelist(t *testing.T) {
	index := ItemIndex{
		Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("b", 64)},
		Items: map[uint32]ItemIndexEntry{
			100: {ID: 100, Path: "stackable/a.stk", Kind: "stackable", StackableType: "[booster selection]"},
			200: {ID: 200, Path: "stackable/b.stk", Kind: "stackable", StackableType: "[material]"},
			300: {ID: 300, Path: "stackable/c.stk", Kind: "stackable", StackableType: "[booster]"},
		},
	}
	got, err := selectionBoxCandidates(index, SelectionBoxPolicy{Version: 1, Whitelist: []uint32{200}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[100] == "" || got[200] == "" {
		t.Fatalf("candidates = %v", got)
	}
	if _, ok := got[300]; ok {
		t.Fatal("a fixed [booster] box must not be discovered as a selection box")
	}
	if _, err := selectionBoxCandidates(index, SelectionBoxPolicy{Version: 1, Whitelist: []uint32{999}}); err == nil {
		t.Fatal("accepted a whitelist template with no indexed binding")
	}
}

func TestAssembleSelectionBoxesKeepsHybridScope(t *testing.T) {
	hash := strings.Repeat("a", 64)
	// 100 is reached by the [booster selection] scan, 200 only through the
	// whitelist; the assembled catalog must keep both.
	candidates := map[uint32]string{100: "stackable/a.stk", 200: "stackable/b.stk"}
	read := func(path string) (ScriptRecord, error) {
		switch path {
		case "stackable/a.stk":
			return ScriptRecord{Path: path, SHA256: hash, Cells: selectionBoxCells([2]int32{0, 0}, 11)}, nil
		case "stackable/b.stk":
			return ScriptRecord{Path: path, SHA256: hash, Cells: selectionBoxCells([2]int32{1, 0}, 22)}, nil
		}
		return ScriptRecord{}, pvf.ErrFileNotFound
	}
	boxes, err := assembleSelectionBoxes(candidates, read, pvf.ArchiveSnapshot{Checksum: hash})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := boxes.ByTemplate(100); !ok {
		t.Fatal("discovered box missing")
	}
	if _, ok := boxes.ByTemplate(200); !ok {
		t.Fatal("whitelisted box missing")
	}
	if len(boxes.Unparsed)+len(boxes.Rejected)+len(boxes.Fixed) != 0 {
		t.Fatalf("unexpected classification: %+v", boxes)
	}
}

func TestSelectionBoxesDropIdenticalDuplicateCategory(t *testing.T) {
	hash := strings.Repeat("c", 64)
	cat := SelectionCategory{Category: [2]byte{13, 0}, Items: []SelectionItem{{Template: 1, Count: 1}}}
	boxes, err := NewSelectionBoxes(SelectionBoxes{
		Model: SelectionBoxModel, Source: pvf.ArchiveSnapshot{Checksum: hash},
		Boxes: map[string]SelectionBox{"1": {Template: 1, Path: "p", SHA256: hash, Categories: []SelectionCategory{cat, cat}}},
	})
	if err != nil {
		t.Fatalf("identical duplicate category refused: %v", err)
	}
	box, _ := boxes.ByTemplate(1)
	if len(box.Categories) != 1 {
		t.Fatalf("duplicate not dropped: %+v", box.Categories)
	}
}

func TestSelectionBoxesRefuseConflictingDuplicateCategory(t *testing.T) {
	hash := strings.Repeat("d", 64)
	a := SelectionCategory{Category: [2]byte{1, 0}, Items: []SelectionItem{{Template: 1, Count: 1}}}
	b := SelectionCategory{Category: [2]byte{1, 0}, Items: []SelectionItem{{Template: 2, Count: 1}}}
	if _, err := NewSelectionBoxes(SelectionBoxes{
		Model: SelectionBoxModel, Source: pvf.ArchiveSnapshot{Checksum: hash},
		Boxes: map[string]SelectionBox{"1": {Template: 1, Path: "p", SHA256: hash, Categories: []SelectionCategory{a, b}}},
	}); err == nil {
		t.Fatal("conflicting duplicate category accepted; the client's block is unknown")
	}
}

func TestSelectionBoxesRefuseDuplicateUnmodelledCategory(t *testing.T) {
	hash := strings.Repeat("e", 64)
	// Both blocks look identical to the modelled parser (empty avatar section),
	// but their raw [avatar] contents could differ; do not silently drop one.
	cat := SelectionCategory{Category: [2]byte{1, 0}, Sections: []string{"[avatar]"}}
	if _, err := NewSelectionBoxes(SelectionBoxes{
		Model: SelectionBoxModel, Source: pvf.ArchiveSnapshot{Checksum: hash},
		Boxes: map[string]SelectionBox{"1": {Template: 1, Path: "p", SHA256: hash, Categories: []SelectionCategory{cat, cat}}},
	}); err == nil {
		t.Fatal("an unmodelled repeated category must stay a conflict, not be dropped")
	}
}
