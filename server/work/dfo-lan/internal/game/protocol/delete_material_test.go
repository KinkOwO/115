package protocol

import (
	"encoding/hex"
	"testing"
)

func TestMaterialDeleteCaptured(t *testing.T) {
	p, _ := hex.DecodeString("0f00000010001a090802107918dd17200120000000000000000000000000000000")
	rows, e := DecodeMaterialDelete(p)
	if e != nil || len(rows) != 1 || rows[0] != (MaterialDelete{121, 3037, 1}) {
		t.Fatal(rows, e)
	}
	got := hex.EncodeToString(MaterialDeleteReply(rows, true))
	if got != "010c000000080018001206087910011802" {
		t.Fatal(got)
	}
	for _, at := range []int{0, 4, 6, 8, 10, 12, 15, 18, 31} {
		bad := append([]byte{}, p...)
		bad[at] = 255
		if _, e := DecodeMaterialDelete(bad); e == nil {
			t.Fatalf("mutation %d accepted", at)
		}
	}
}

func TestFatigueActionCaptured(t *testing.T) {
	p := make([]byte, 64)
	p[0] = 66
	p[7] = 54
	if slot, e := DecodeFatigueAction(p); e != nil || slot != 66 {
		t.Fatal(slot, e)
	}
	for _, at := range []int{2, 3, 7, 8, 11, 19, 63} {
		q := append([]byte{}, p...)
		q[at] = 255
		if _, e := DecodeFatigueAction(q); e == nil {
			t.Fatal(at)
		}
	}
}
