package protocol

import (
	"encoding/hex"
	"testing"
)

func TestAreaChangeNativeWriterSequence(t *testing.T) {
	// Native writer field widths, deliberately distinct values for every field.
	p, _ := hex.DecodeString("26000000010000003102ea00002700000003000405000000")
	r, e := DecodeAreaChangeRequest(p)
	if e != nil || r.Town != 38 || r.Area != 1 || r.X != 561 || r.Y != 234 || r.Flag != 0 || r.PreviousTown != 39 || r.PreviousArea != 3 || r.TailFlags != [2]byte{4, 5} {
		t.Fatalf("decode native sequence: %+v %v", r, e)
	}
	p[len(p)-1] = 1
	if _, e = DecodeAreaChangeRequest(p); e == nil {
		t.Fatal("accepted nonzero tail")
	}
	if _, e = DecodeAreaChangeRequest(p[:20]); e == nil {
		t.Fatal("accepted truncated request")
	}
}

func TestUserAreaNativeReaderSequence(t *testing.T) {
	p, e := UserArea(38, 1, AreaUser{ActorServerID: 3, X: 561, Y: 234, Flags: [3]byte{0, 1, 1}})
	if e != nil || hex.EncodeToString(p) != "030026000000010000003102ea000001" {
		t.Fatalf("user area %x %v", p, e)
	}
	p, e = AreaChangeFailure(7, 38, 3)
	if e != nil || hex.EncodeToString(p) != "0007002600000003000000" {
		t.Fatalf("area refusal %x %v", p, e)
	}
}
