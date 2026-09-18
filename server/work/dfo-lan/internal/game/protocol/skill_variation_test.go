package protocol

import (
	"encoding/hex"
	"testing"
)

const autoSetCapture = "002572000002700000057100000902010012ef00000deb0000150900002961000017ec00001c5b000016490000240800000148000026690000016e00000a6d00002b070000016c00002e2600000a1f00000105010005ba00000a6b00000a43000001310000012100000a1b0000140f0000010e0000010d0000010c00000104000001aa00000141000001110000014400000262000008010101eb000100000000020101000000004800010000000001090001000000010048000100000001004900010000000100eb000100000001000201020000000100baeb9fb30000000000"

func TestCapturedAutoSetVariations(t *testing.T) {
	p, _ := hex.DecodeString(autoSetCapture)
	r, e := DecodeSkillPurchase(p)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Entries) != 37 || r.Mode != 1 || r.Preset != 1 || len(r.Intensions) != 3 || len(r.Options) != 5 {
		t.Fatal(r)
	}
	if r.Intensions[0].ID != 235 || r.Options[4].ID != 258 || r.Options[4].Choice != 2 {
		t.Fatal(r)
	}
	for n := 150; n < 215; n++ {
		if _, e := DecodeSkillPurchase(p[:n]); e == nil {
			t.Fatalf("truncated packet accepted %d", n)
		}
	}
	response, e := SkillPurchaseSuccess(0, 100, 0, nil)
	if e != nil {
		t.Fatal(e)
	}
	response, e = SkillPurchaseVariations(response, r.Mode, r.Intensions, r.Options)
	if e != nil || len(response) != 75 {
		t.Fatal(len(response), e)
	}
}
