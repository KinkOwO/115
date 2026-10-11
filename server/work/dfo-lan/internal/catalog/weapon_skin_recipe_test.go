package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"testing"
)

func TestWeaponShapeRecipeFieldsDriveProductAndCost(t *testing.T) {
	for _, n := range []int32{1, 7} {
		cells := []pvf.Token{{Type: 3, Text: "[make enchanted skin]"}, {Type: 3, Text: "[recipe]"}, {Type: 0, Value: 9}, {Type: 3, Text: "[enchanted result]"}, {Type: 0, Value: 22 + n}, {Type: 3, Text: "[material]"}, {Type: 0, Value: 11 + n}, {Type: 6, Text: "fixed"}, {Type: 0, Value: n}, {Type: 3, Text: "[/material]"}, {Type: 3, Text: "[/recipe]"}, {Type: 3, Text: "[/make enchanted skin]"}}
		read := func(id uint32) ([]pvf.Token, error) {
			if id != uint32(22+n) {
				return nil, fmt.Errorf("unexpected product %d", id)
			}
			return []pvf.Token{{Type: 3, Text: "[action type]"}, {Type: 6, Text: AddSkinStorageLabel}, {Type: 0, Value: 40}, {Type: 3, Text: "[/action type]"}, {Type: 3, Text: "[attach type]"}, {Type: 6, Text: "[account]"}, {Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[waste]"}, {Type: 0, Value: 0}}, nil
		}
		got, e := parseWeaponSkinRecipes(cells, read)
		if e != nil || got[9].Material != uint32(11+n) || got[9].Product != uint32(22+n) || got[9].Count != uint32(n) || got[9].Attach != "[account]" {
			t.Fatalf("source fields ignored: %+v %v", got, e)
		}
		cells[7].Text = "random"
		if _, e := parseWeaponSkinRecipes(cells, read); e == nil {
			t.Fatal("unsupported cost rule accepted")
		}
	}
}
