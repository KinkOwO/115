package main

import (
	"dfolan/internal/catalog"
	"os"
	"testing"
)

func TestPVFSelectionBoxesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for full bounded selection box parity")
	}
	c, err := preparePVFCoreCatalogs("selection-boxes", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", selectionPolicyPath: "../../configs/pvf-selection-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	boxes, err := c.loadSelectionBoxes("missing-selection-boxes.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(boxes.Boxes) != 2975 || len(boxes.Fixed) != 2 || len(boxes.Unparsed) != 1 || !boxes.IsFixed(10307659) || !boxes.IsFixed(490022952) || boxes.Unparsed[0] != 10358468 {
		t.Fatal("selection source range changed")
	}
	old, err := catalog.LoadSelectionBoxes("../../configs/selection-boxes-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, box := range boxes.Boxes {
		for _, cat := range box.Categories {
			if len(cat.Items) == 0 {
				continue
			}
			picks := []uint32{cat.Items[0].Template, 1}
			items, missing, checked := boxes.Resolve(box.Template, cat.Category, picks)
			a, b, ok := old.Resolve(box.Template, cat.Category, picks)
			if checked != ok {
				t.Fatal("selection category lookup changed")
			}
			if err := verifyPVFCatalog(items, a); err != nil {
				t.Fatal(err)
			}
			if err := verifyPVFCatalog(missing, b); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Log("complete category/count/recommendation/hash parity and derived Resolve lookup verified for all modeled categories")
}
