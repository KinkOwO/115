package adventure

import (
	"dfolan/internal/catalog/pvf"
	"errors"
	"reflect"
	"testing"
)

func TestRecommendedWorldmapConsumesQuestCells(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[dungeon]"}, {Type: 0, Value: 100}, {Type: 0, Value: 900}, {Type: 0, Value: 101}, {Type: 3, Text: "[in progress]"}, {Type: 0, Value: 901}, {Type: 0, Value: 102}, {Type: 3, Text: "[is clear quest]"}, {Type: 0, Value: 902}, {Type: 3, Text: "[/dungeon]"}}
	got, err := recommendedWorldmapDungeons(cells)
	if err != nil || !reflect.DeepEqual(got, []uint32{100, 101, 102}) {
		t.Fatal(got, err)
	}
	for i := range cells {
		if cells[i].Type == 3 && cells[i].Text == "[in progress]" {
			cells[i].Text = "[unverified condition]"
			break
		}
	}
	if _, err := recommendedWorldmapDungeons(cells); err == nil {
		t.Fatal("unverified condition accepted")
	}
	cells = []pvf.Token{{Type: 3, Text: "[dungeon]"}, {Type: 0, Value: 100}, {Type: 3, Text: "[/dungeon]"}}
	if _, err := recommendedWorldmapDungeons(cells); err == nil {
		t.Fatal("missing quest accepted")
	}
}

func TestInstalledRecommendedAvoidsEmbeddedReadAndMutation(t *testing.T) {
	old, err := EmbeddedRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	restore, err := InstallRecommendedRules(old)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := CurrentRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	original := loadRecommended
	loadRecommended = func() (*RecommendedRules, error) { return nil, errors.New("embedded rules unavailable") }
	defer func() { loadRecommended = original }()
	for id, bounds := range old.Ranges {
		if current.Ranges[id] != bounds {
			t.Fatal("ranges changed")
		}
		// A separate installation must own its maps and private exclusion index.
		current.Ranges[id] = [2]uint32{1, 2}
		if old.Ranges[id] != bounds {
			t.Fatal("native mutation reached legacy source")
		}
		break
	}
	if _, err := RecommendedDungeonClear(old.Excluded[0], 255); err != nil {
		t.Fatal("native query fell back", err)
	}
	bad := *old
	bad.MinimumLevel = 0
	if _, err := InstallRecommendedRules(&bad); err == nil {
		t.Fatal("invalid source installed")
	}
	after, err := CurrentRecommendedRules()
	if err != nil || after != current {
		t.Fatal("failed install damaged native rules", err)
	}
}
