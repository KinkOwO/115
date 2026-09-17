package protocol

import (
	"encoding/hex"
	"testing"
)

func TestAvatarOptionCaptured(t *testing.T) {
	p, _ := hex.DecodeString("ffffffffffff02000400999e00000f00")
	r, e := DecodeAvatarOption(p)
	if e != nil || r.Slot != 4 || r.Template != 40601 || r.Option != 15 {
		t.Fatal(r, e)
	}
	if got := hex.EncodeToString(AvatarOptionSuccess(r)); got != "01020004000f" {
		t.Fatal(got)
	}
	for _, n := range []int{0, 6, 10, 14, 15} {
		q := append([]byte(nil), p...)
		q[n] = 0xfe
		if _, e := DecodeAvatarOption(q); e == nil && n != 10 && n != 14 {
			t.Fatal("invalid accepted", n)
		}
	}
	if _, e := DecodeAvatarOption(p[:14]); e == nil {
		t.Fatal("short request")
	}
}
