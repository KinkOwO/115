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

func TestAvatarOptionPaidRareCloneCapture(t *testing.T) {
	p, err := hex.DecodeString("460064982800000001002757d91e0200")
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeAvatarOption(p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Paid || r.ConsumableSlot != 70 || r.ConsumableTemplate != 2660452 || r.Location != 0 || r.Slot != 1 || r.Template != 517560103 || r.Option != 2 {
		t.Fatalf("decoded paid rare-clone request = %+v", r)
	}
	if got := hex.EncodeToString(AvatarOptionSuccess(r)); got != "010000010002" {
		t.Fatalf("success payload = %s", got)
	}

	for _, body := range []string{
		"ffff64982800000001002757d91e0200", // mismatched free sentinel
		"460064982800020001002757d91e0200", // paid request targets unsupported location
	} {
		q, decodeErr := hex.DecodeString(body)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if _, err := DecodeAvatarOption(q); err == nil {
			t.Fatalf("invalid paid request accepted: %s", body)
		}
	}
}
