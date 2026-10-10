package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// The layout is pinned to the evidence recovered from this build's own sender
// (begin-command edx=44 at 146d746e0, then u16/u8/u32/u32/u32 through the
// calibrated writers) and cross-checked against the capture the supplied
// reference documents for the same 15-byte shape:
//
//	03 00 00 07 09 00 00 59 04 00 00 00 00 00 00 00
//	slot=3 list=0 instance=0x907 item=0x459 reserved=0
func TestDecodeUseStackableMatchesRecoveredLayout(t *testing.T) {
	p, _ := hex.DecodeString("03000007090000590400000000000000")
	r, e := DecodeUseStackable(p)
	if e != nil {
		t.Fatal(e)
	}
	if r.Slot != 3 || r.List != 0 || r.Instance != 0x907 || r.Template != 0x459 || r.Reserved != 0 {
		t.Fatalf("layout drift: %+v", r)
	}
}

func TestDecodeUseStackableRejectsMalformed(t *testing.T) {
	good, _ := hex.DecodeString("03000007090000590400000000000000")
	// A zero identity is never a real item.
	zero := append([]byte(nil), good...)
	for i := 7; i < 11; i++ {
		zero[i] = 0
	}
	if _, e := DecodeUseStackable(zero); e == nil {
		t.Fatal("accepted an item identity of zero")
	}
	if _, e := DecodeUseStackable(good[:15]); e == nil {
		t.Fatal("accepted an unpadded body")
	}
	// The trailing pad must stay zero: a non-zero byte there means the shape
	// is not the one that was recovered.
	dirty := append([]byte(nil), good...)
	dirty[15] = 1
	if _, e := DecodeUseStackable(dirty); e == nil {
		t.Fatal("accepted a dirty pad")
	}
}

// Handler 14529fd70 reads u16, u8, u32, u32 after the framework consumes the
// success flag, so a success body is the flag plus eleven bytes; the failure
// path reads u8, u32, u32, so nine.
func TestUseStackableAcknowledgementWidths(t *testing.T) {
	r := UseStackableRequest{Slot: 3, List: 0, Instance: 0x907, Template: 0x459}
	ok, e := UseStackableSuccess(r)
	if e != nil {
		t.Fatal(e)
	}
	if len(ok) != 12 {
		t.Fatalf("success body is %d bytes, want 1 flag + 11", len(ok))
	}
	if ok[0] != 1 {
		t.Fatal("success flag is not set")
	}
	// The slot echo is what the client matches its own pending use against.
	if ok[1] != 3 || ok[2] != 0 || ok[3] != 0 {
		t.Fatalf("slot/list echo drift: %x", ok[1:4])
	}
	bad := UseStackableRefused(r)
	if len(bad) != 12 {
		t.Fatalf("failure body is %d bytes, want flag/error/list/instance/template", len(bad))
	}
	if bad[0] != 0 {
		t.Fatal("failure flag is set")
	}
	if _, e = UseStackableSuccess(UseStackableRequest{Slot: 3}); e == nil {
		t.Fatal("acknowledged an item identity of zero")
	}
}

func TestUseStackableRefusalMatchesOfficialCurrentClient(t *testing.T) {
	request, _ := hex.DecodeString("4b0000e343000056c29d000000000000")
	r, e := DecodeUseStackable(request)
	if e != nil {
		t.Fatal(e)
	}
	got := UseStackableRefused(r)
	if hex.EncodeToString(got) != "00130000e343000056c29d00" {
		t.Fatalf("native failure drift: %x", got)
	}
	// The crash-causing real recovery request must retain bag0 after the
	// framework consumes flag/error; its template byte must not become list247.
	request, _ = hex.DecodeString("030000824d00006cf79e000000000000")
	r, e = DecodeUseStackable(request)
	if e != nil {
		t.Fatal(e)
	}
	got = UseStackableRefused(r)
	if got[3] != r.List || binary.LittleEndian.Uint32(got[4:8]) != r.Instance || binary.LittleEndian.Uint32(got[8:12]) != r.Template {
		t.Fatal("misaligned failed recovery identity")
	}
}
