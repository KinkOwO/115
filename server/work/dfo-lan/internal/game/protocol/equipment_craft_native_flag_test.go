package protocol

import (
	"encoding/hex"
	"testing"
)

// Captured from the user's client at 2026-10-07 07:04:57 (Hong Kong).
// +12=185 is stack residue; +115=0 selects transform in the native sender.
const craftTutorialNativeFrame = "0000000000000000b22d6f29b9010000002e0000ffffffff2e0000ffffffff030e0055a9f6052e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff00010000000000000000000000"

func TestEquipmentCraftNativeFlagIgnoresStackPrefix(t *testing.T) {
	p, err := hex.DecodeString(craftTutorialNativeFrame)
	if err != nil {
		t.Fatal(err)
	}
	for _, residue := range []byte{0, 1, 36, 164, 185, 254} {
		for i := 0; i < 13; i++ {
			p[i] = residue
		}
		r, err := DecodeEquipmentCraftRequest(p)
		if err != nil {
			t.Fatal(err)
		}
		slots, templates := r.Wanted()
		if r.Action != 1 || r.PayOption != 1 || len(slots) != 1 || slots[0] != 14 || templates[0] != 100051285 {
			t.Fatalf("residue %d: decoded=%+v slots=%v templates=%v", residue, r, slots, templates)
		}
	}
	// The other native sender explicitly writes +115=1 for creation.
	p[115] = 1
	r, err := DecodeEquipmentCraftRequest(p)
	if err != nil || r.Action != 0 {
		t.Fatalf("create flag: %+v %v", r, err)
	}
	p[115] = 2
	if _, err := DecodeEquipmentCraftRequest(p); err == nil {
		t.Fatal("invalid native flag accepted")
	}
}
