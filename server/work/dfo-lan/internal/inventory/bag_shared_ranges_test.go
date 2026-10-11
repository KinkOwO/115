package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBagRulesSharedRangesKeepBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name             string
		a, b             [2]uint16
		equipment, quick [2]uint16
		ok               bool
	}{
		{"identical", [2]uint16{65, 120}, [2]uint16{65, 120}, [2]uint16{9, 64}, [2]uint16{0, 8}, true},
		{"disjoint", [2]uint16{65, 80}, [2]uint16{81, 120}, [2]uint16{9, 64}, [2]uint16{0, 8}, true},
		{"partial", [2]uint16{65, 80}, [2]uint16{70, 90}, [2]uint16{}, [2]uint16{}, false},
		{"contained", [2]uint16{65, 120}, [2]uint16{70, 90}, [2]uint16{}, [2]uint16{}, false},
		{"shared-endpoint", [2]uint16{65, 80}, [2]uint16{80, 90}, [2]uint16{}, [2]uint16{}, false},
		{"equipment", [2]uint16{64, 120}, [2]uint16{64, 120}, [2]uint16{9, 64}, [2]uint16{}, false},
		{"quick", [2]uint16{1, 8}, [2]uint16{1, 8}, [2]uint16{}, [2]uint16{0, 8}, false},
		{"zero", [2]uint16{0, 5}, [2]uint16{0, 5}, [2]uint16{}, [2]uint16{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := BagRules{Model: "reference90-bag-v1", Source: strings.Repeat("a", 64), MissingStackLimit: 1, Slots: map[string][2]uint16{"[waste]": tc.a, "[etc]": tc.b}, EquipmentSlots: tc.equipment, QuickSlots: tc.quick}
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(t.TempDir(), "bag.json")
			if err = os.WriteFile(p, b, 0600); err != nil {
				t.Fatal(err)
			}
			_, err = LoadBagRules(p)
			if (err == nil) != tc.ok {
				t.Fatalf("accepted=%v want=%v err=%v", err == nil, tc.ok, err)
			}
		})
	}
}
