package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestCreateReceiptUsesZeroBasedPosition(t *testing.T) {
	// The live fourth-character crash used ID 4. The native map contains
	// positions 0..3 and dereferences a null result for 4 (0x14444c724).
	if got := CreateSuccess(3, "monv"); hex.EncodeToString(got) != "010300040000006d6f6e76" {
		t.Fatalf("fourth character receipt: %x", got)
	}
	rows := []CharacterRow{{Slot: 0, Name: "LanTest01", Level: 1}}
	packet, err := CharacterList(8, rows)
	if err != nil || binary.LittleEndian.Uint16(packet[15:17]) != 0 {
		t.Fatalf("first roster position must be zero: %x %v", packet, err)
	}
	rows[0].Slot = 1
	if _, err = CharacterList(8, rows); err == nil {
		t.Fatal("accepted a storage ID in place of the first roster position")
	}
}

func TestNormalCreateSenderLayout(t *testing.T) {
	// Independently transcribed order/constants from native 0x1402394b4.
	p, _ := hex.DecodeString("00090000004c616e546573743031000000000000ff000000")
	r, e := DecodeCreateRequest(p)
	if e != nil || r.Name != "LanTest01" || r.Profession != 0 {
		t.Fatalf("%+v %v", r, e)
	}
	for i := 0; i < len(p); i++ {
		if _, e := DecodeCreateRequest(p[:i]); e == nil {
			t.Fatalf("accepted truncation at %d", i)
		}
	}
	for _, bad := range []string{"00ffffffff", "00090000004c616e00546573743031000000000000ff000000"} {
		b, _ := hex.DecodeString(bad)
		if _, e := DecodeCreateRequest(b); e == nil {
			t.Fatal("accepted invalid name")
		}
	}
	p[20] = 1
	if _, e := DecodeCreateRequest(p); e == nil {
		t.Fatal("accepted changed structural suffix")
	}
}

func TestCurrentNamingWindowRegression(t *testing.T) {
	// Actual checksum-verified client payload from roles_persist_live_01,
	// 2026-09-10 23:02:23 UTC. The previous validator disconnected here.
	p, _ := hex.DecodeString("100400000036363636000000000000ff0001000000000000")
	r, e := DecodeCreateRequest(p)
	if e != nil {
		t.Fatal(e)
	}
	if r.Profession != 16 || r.Name != "6666" || len(r.Options) != 12 || r.Options[8] != 1 {
		t.Fatalf("%+v", r)
	}
}
func TestEmptyListMatchesNativeAcceptedFixture(t *testing.T) {
	p, e := CharacterList(8, nil)
	if e != nil {
		t.Fatal(e)
	}
	if hex.EncodeToString(p) != "02000008000000000000000000000001000000000000000000" {
		t.Fatalf("%x", p)
	}
}
func TestNamePacketStrictPadding(t *testing.T) {
	p := addName(nil, "LanTest01")
	p = append(p, 0, 0, 0)
	if n, e := DecodeNameRequest(p); e != nil || n != "LanTest01" {
		t.Fatal(n, e)
	}
	p[len(p)-1] = 1
	if _, e := DecodeNameRequest(p); e == nil {
		t.Fatal("nonzero padding accepted")
	}
}
