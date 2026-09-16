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
	for name, want := range map[string]bool{"upperslash": true, "hardattack": true, "backstep": true, "skillpresettype1": false, "quickstanding": false} {
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
	}
}
