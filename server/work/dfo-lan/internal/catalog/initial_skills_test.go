package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"testing"
)

func TestInitialShortcutsFromOriginalScripts(t *testing.T) {
	// Original full typed PVF scripts. UpperSlash and HardAttack repeat
	// [type] in nested damage groups, reproducing the live24 missing slots.
	for name, want := range map[string]bool{"upperslash": true, "hardattack": true, "backstep": true, "skillpresettype1": false, "quickstanding": true} {
		b, e := os.ReadFile("testdata/initial_skills/" + name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		var cells []pvf.Token
		if e = json.Unmarshal(b, &cells); e != nil {
			t.Fatal(e)
		}
		if got := initialShortcutEligible(cells); got != want {
			t.Errorf("%s shortcut eligible=%v, want %v", name, got, want)
		}
		if name == "backstep" || name == "quickstanding" {
			if got := defaultShortcutEligible("skill/knight/"+name+".skl", cells); got {
				t.Errorf("%s should remain learned but not receive a default shortcut", name)
			}
		}
	}
}

func TestSourceCommandVectorMatchesCurrentKnightRows(t *testing.T) {
	const (
		backstepFamily int32 = 0x11b6c657
		dragonFamily   int32 = 0x0337ea25
	)
	tests := []struct {
		name  string
		cells []pvf.Token
		want  []uint32
	}{
		{
			name:  "backstep",
			cells: []pvf.Token{{Type: 3, Text: "[command]"}, {Type: 5, Value: 11}, {Type: 7, Value: backstepFamily}, {Type: 5, Value: 89}, {Type: 3, Text: "[/command]"}},
			want:  []uint32{10, 6},
		},
		{
			name:  "finish",
			cells: []pvf.Token{{Type: 3, Text: "[command]"}, {Type: 5, Value: 39}, {Type: 7, Value: dragonFamily}, {Type: 5, Value: 11}, {Type: 7, Value: dragonFamily}, {Type: 5, Value: 55}, {Type: 3, Text: "[/command]"}},
			want:  []uint32{3, 1, 4},
		},
		{
			name:  "dragon wing",
			cells: []pvf.Token{{Type: 3, Text: "[command]"}, {Type: 5, Value: 125}, {Type: 3, Text: "[/command]"}},
			want:  []uint32{8},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := sourceCommandVector(tt.cells)
			if !ok {
				t.Fatal("source command vector was not decoded")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("vector=%v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("vector=%v, want %v", got, tt.want)
				}
			}
		})
	}
}
