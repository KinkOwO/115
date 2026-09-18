package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"
)

func TestCeraPurchaseCancelledNativeLayout(t *testing.T) {
	p := CeraPurchaseCancelled()
	if len(p) != 24 || p[0] != 0 || binary.LittleEndian.Uint16(p[1:3]) != 0 {
		t.Fatalf("invalid dispatcher failure prefix: %x", p)
	}
	// Simulate all mandatory reads in145268248..145268282. Returning only
	// Refusal(0) would truncate the receiver after clearing the purchase UI.
	pos := 3
	_ = p[pos]
	pos++
	for i := 0; i < 5; i++ {
		if binary.LittleEndian.Uint32(p[pos:pos+4]) != 0 {
			t.Fatal("cancellation carries unintended product mutation")
		}
		pos += 4
	}
	if pos != len(p) {
		t.Fatal("unexpected cancellation tail")
	}
	p[0] = 1
	if CeraPurchaseCancelled()[0] != 0 {
		t.Fatal("response buffer is shared")
	}
}

func TestCeraPilotSuccessCompleteLayout(t *testing.T) {
	p, e := CeraPurchasePilotSuccess(3000118)
	if e != nil || len(p) != 49 || p[0] != 1 {
		t.Fatalf("%x %v", p, e)
	}
	pos := 2
	read := func() uint32 { v := binary.LittleEndian.Uint32(p[pos : pos+4]); pos += 4; return v }
	if read() != math.MaxUint32 || read() != 3000118 {
		t.Fatal("category/product")
	}
	c, d := read(), read()
	_ = read()
	n := binary.LittleEndian.Uint16(p[pos:])
	pos += 2
	for i := 0; i < int(n); i++ {
		read()
		read()
	}
	if c == d {
		if read() != math.MaxUint32 {
			t.Fatal("bonus sentinel")
		}
		read()
	}
	read()
	read()
	read()
	if read() != 1 {
		t.Fatal("pending quantity")
	}
	pos++
	if pos != len(p) {
		t.Fatalf("native parser consumed%d/%d", pos, len(p))
	}
	if _, e = CeraPurchasePilotSuccess(0); e == nil {
		t.Fatal("empty product accepted")
	}
	if _, e = CeraPurchasePilotSuccess(3999988); e != nil {
		t.Fatal("encoder still contains SKU whitelist", e)
	}
}

func TestCeraBalanceNativeLayout(t *testing.T) {
	p, e := CeraBalance(10000)
	if e != nil || hex.EncodeToString(p) != "011027000001000000" {
		t.Fatalf("%x %v", p, e)
	}
	if _, e = CeraBalance(math.MaxInt32); e != nil {
		t.Fatal(e)
	}
	if _, e = CeraBalance(math.MaxInt32 + 1); e == nil {
		t.Fatal("signed overflow")
	}
}
func TestCeraCartActualCapture(t *testing.T) {
	p, _ := hex.DecodeString("000001000029e3330001000000000000")
	items, e := DecodeCeraCart(p)
	if e != nil {
		t.Fatal(e)
	}
	if len(items) != 1 || items[0].Product != 3400489 || items[0].Quantity != 1 {
		t.Fatalf("%+v", items)
	}
	for _, index := range []int{1, 13, 14, 15} {
		bad := append([]byte(nil), p...)
		bad[index] = 1
		if _, e = DecodeCeraCart(bad); e == nil {
			t.Fatalf("accepted mutation %d", index)
		}
	}
	if _, e = DecodeCeraCart(p[:12]); e == nil {
		t.Fatal("accepted truncation")
	}
}
