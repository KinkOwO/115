package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestCurrentExperienceNativeCursor(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    ExperienceUpdate
	}{
		{"experience", ExperienceUpdate{Level: 1, Total: 72}},
		{"experience_extended", ExperienceUpdate{Level: 2, Total: 0x100000048, SP: [2]uint16{30, 40}, TP: [2]uint16{2, 3}, CurrencySlot2: 11}},
	} {
		b, e := os.ReadFile("testdata/native_" + tc.name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		var f struct {
			Payload  string `json:"payload_hex"`
			Consumed int
			Effects  []struct {
				Kind       string
				Experience uint64
				Level      byte
			}
		}
		if e = json.Unmarshal(b, &f); e != nil {
			t.Fatal(e)
		}
		p, e := ExperienceState(tc.s)
		if e != nil || hex.EncodeToString(p) != f.Payload || len(p) != f.Consumed {
			t.Fatal("current native EXP layout mismatch", e)
		}
		if len(f.Effects) != 1 || f.Effects[0].Experience != tc.s.Total || f.Effects[0].Level != tc.s.Level {
			t.Fatal("native EXP/level setter mismatch")
		}
	}
}
