package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOdysseyCompletionMappingAndNativeValidation(t *testing.T) {
	c, err := LoadOdysseyCompletionRewards("../../configs/odyssey-completion-rewards.json")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]uint32{{10417792, 10419741, 10419342, 10419346}, {10419742, 10419745, 10419346}, {10417793, 10417796, 10419745, 10419346}, {10417794, 10419743, 10419745, 10419347}, {10419744, 10419745, 10419347}, {10417795, 10419744, 10419745, 10419347}, {10419538, 10419539, 10419745}}
	index := ItemIndex{Items: map[uint32]ItemIndexEntry{10420561: {ID: 10420561, Kind: "stackable", StackableType: "[booster]", StackLimit: 1}}}
	for i, ids := range want {
		got := c.At(uint8(i + 1))
		if len(got) != len(ids) {
			t.Fatal(got)
		}
		for j, id := range ids {
			if got[j].Template != id {
				t.Fatal(i, j, got)
			}
			index.Items[id] = ItemIndexEntry{ID: id, Kind: "stackable", StackableType: "[booster selection]"}
		}
	}
	if c.At(4)[1].Count != 2 || c.At(5)[0].Count != 2 || c.Honor.Template != 10420561 {
		t.Fatal("native quantities/honor drift")
	}
	if err = c.ValidateItems(index); err != nil {
		t.Fatal(err)
	}
	delete(index.Items, 10419342)
	if err = c.ValidateItems(index); err == nil {
		t.Fatal("missing native ID accepted")
	}
	file := filepath.Join(t.TempDir(), "invalid.json")
	os.WriteFile(file, []byte(`{"version":1,"unexpected":true}`), 0600)
	if _, err = LoadOdysseyCompletionRewards(file); err == nil {
		t.Fatal("unknown config field accepted")
	}
}
