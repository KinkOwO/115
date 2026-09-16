package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNonzeroGoldNoticeUsesNativeOverheadBranch(t *testing.T) {
	p, e := GoldPickupConfirmed(0x11223344, 3, 31)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(p[6:], nativeInventoryFixture(t, "gold_pickup30")) {
		t.Fatal("gold branch cursor mismatch")
	}
	if _, e = GoldPickupConfirmed(1, 3, 0); e == nil {
		t.Fatal("zero award accepted")
	}
}

func nativeInventoryFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/native_" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Payload string `json:"payload_hex"`
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	p, e := hex.DecodeString(f.Payload)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestCurrentInventoryNativeFixtures(t *testing.T) {
	refused, e := PickupRefused(0x11223344)
	wantRefusal := append([]byte{0, 4, 0}, nativeInventoryFixture(t, "pickup_ack_0")...)
	if e != nil || string(refused) != string(wantRefusal) {
		t.Fatal("pickup refusal mismatch", e)
	}
	if len(nativeInventoryFixture(t, "pickup_ack_1")) != 0 {
		t.Fatal("success ACK unexpectedly consumes a body")
	}
	for n := 0; n <= 2; n++ {
		var rows [][CurrentItemRecordSize]byte
		if n > 0 {
			rows = append(rows, OrdinaryItem(0, 0, 123))
		}
		if n > 1 {
			rows = append(rows, OrdinaryItem(65, 3327, 123))
		}
		got, e := InventoryRestore(rows)
		want := nativeInventoryFixture(t, fmt.Sprintf("inventory_restore_%d", n))
		if e != nil || string(got) != string(want) {
			t.Fatalf("native inventory count%d: %v", n, e)
		}
	}
	for _, id := range []uint32{0, 3327} {
		slot := uint16(0)
		if id != 0 {
			slot = 57
		}
		got, e := InventoryUpdate([][CurrentItemRecordSize]byte{OrdinaryItem(slot, id, 123)})
		want := nativeInventoryFixture(t, fmt.Sprintf("item_update_%d", id))
		if e != nil || string(got) != string(want) {
			t.Fatalf("native item update%d: %v", id, e)
		}
	}
	for _, gold := range []bool{false, true} {
		slot := uint16(57)
		n := 0
		if gold {
			slot = 0
			n = 1
		}
		got, e := PickupConfirmed(0x11223344, 3, slot, gold)
		want := append([]byte{0x44, 0x33, 0x22, 0x11, 3, 0}, nativeInventoryFixture(t, fmt.Sprintf("pickup_tail_%d", n))...)
		if e != nil || string(got) != string(want) {
			t.Fatal("pickup native branch mismatch", e)
		}
	}
	p := nativeInventoryFixture(t, "pickup_request")
	r, e := DecodePickup(p)
	if e != nil || r.Object != 0x11223344 || r.ActorX != 500 || r.ActorY != 250 || r.DropX != 500 || r.DropY != 250 {
		t.Fatal(r, e)
	}
	if _, e = DecodePickup(append(p, make([]byte, 32-len(p))...)); e != nil {
		t.Fatal(e)
	}
	if _, e = DecodePickup(p[:20]); e == nil {
		t.Fatal("short pickup accepted")
	}
	p[4] = 1
	if _, e = DecodePickup(p); e == nil {
		t.Fatal("unsupported pickup mode accepted")
	}
}
