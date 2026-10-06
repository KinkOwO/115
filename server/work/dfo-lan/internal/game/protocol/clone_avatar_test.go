package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestCloneAvatarSourceClearReaderConsumesElevenRowsAndSlot11Delta(t *testing.T) {
	full, last, err := CloneAvatarSources(map[byte]uint16{0: 0, 6: 9, 11: AvatarInventorySlots(MaxAvatarInventoryExpansion) - 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(full) != 34 || full[0] != 11 || len(last) != 4 || last[0] != 1 || last[1] != 11 {
		t.Fatal("count11 is a clear AND eleven-row read, not a sentinel-only body")
	}
	for i := byte(0); i < 11; i++ {
		p := 1 + 3*int(i)
		want := uint16(0xffff)
		if i == 0 {
			want = 12
		}
		if i == 6 {
			want = 21
		}
		if full[p] != i || binary.LittleEndian.Uint16(full[p+1:]) != want {
			t.Fatalf("row %d: %x", i, full[p:p+3])
		}
	}
	if binary.LittleEndian.Uint16(last[2:]) != AvatarInventorySlots(MaxAvatarInventoryExpansion)+11 {
		t.Fatal("slot11 source uses same +12 namespace")
	}
	if _, _, err := CloneAvatarSources(map[byte]uint16{12: 0}); err == nil {
		t.Fatal("out-of-range worn slot")
	}
	if _, _, err := CloneAvatarSources(map[byte]uint16{0: AvatarInventorySlots(MaxAvatarInventoryExpansion)}); err == nil {
		t.Fatal("fabricated bag source")
	}
}

func TestCloneNativeBindingCurrentClientVector(t *testing.T) {
	// Actual current-client CMD19, 2026-10-05 13:51:50, log line4582.
	p, _ := hex.DecodeString("01090009ed301e01000000030600cae1301e00000000ffffffff000000000000")
	r, err := DecodeItemMove(p)
	if err != nil {
		t.Fatal(err)
	}
	if r.SourceList != 1 || r.SourceSlot != 9 || r.SourceItem != 506522889 || r.DestinationList != 3 || r.DestinationSlot != 6 || r.DestinationItem != 506520010 || r.Flags != [3]byte{} {
		t.Fatalf("native vector: %+v", r)
	}
	ack, err := ItemMoveSuccessMode(r, 1, 1)
	if err != nil || len(ack) != 12 || ack[11] != 1 {
		t.Fatal("mode is the twelfth business byte; frame padding is separate")
	}
	if _, err := ItemMoveSuccessMode(r, 1, 2); err == nil {
		t.Fatal("unsupported native mode2")
	}
}

func TestCloneNativeRevocationCurrentClientVectors(t *testing.T) {
	// Actual current-client input after NOTI1437 made the source discoverable.
	// Fresh role1, 2026-10-05 14:44:53/58, session143838 lines742/770.
	for _, v := range []struct {
		request, ack string
		source, worn uint16
	}{
		{"010d000000000001000000030600bda1e41d00000000ffffffff000001000000", "01010d000100000003060001", 13, 6},
		{"010a0000000000010000000300003717e51d00000000ffffffff000001000000", "01010a000100000003000001", 10, 0},
	} {
		p, _ := hex.DecodeString(v.request)
		r, err := DecodeItemMove(p)
		if err != nil {
			t.Fatal(err)
		}
		if r.SourceList != 1 || r.SourceSlot != v.source || r.SourceItem != 0 || r.DestinationList != 3 || r.DestinationSlot != v.worn || r.Flags != [3]byte{0, 0, 1} || r.DestinationItem == 0 {
			t.Fatalf("native revocation: %+v", r)
		}
		ack, err := ItemMoveSuccessMode(r, 1, 1)
		if err != nil || hex.EncodeToString(ack) != v.ack {
			t.Fatalf("native ACK: %x %v", ack, err)
		}
	}
}
