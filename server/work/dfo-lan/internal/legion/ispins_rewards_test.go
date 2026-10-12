package legion

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func TestIspinsSourceNativeRewardFields(t *testing.T) {
	// Official stage0 decompressed record: bc b1 9d 00 14 00 00 00.
	// Current fixed reader/consumer audit establishes identical row scalar widths.
	b, a, err := IspinsSourceRewardPayloads([]IspinsRewardItem{{Template: 0x9db1bc, Amount: 20}}, []IspinsRewardItem{{Template: 0x9db1bc, Amount: 28}}, 35987*time.Millisecond, true)
	if err != nil || len(b) != 7772 || len(a) != 2405 {
		t.Fatal(err)
	}
	captured, _ := hex.DecodeString("bcb19d0014000000")
	if !bytes.Equal(b[1600:1608], captured) {
		t.Fatal("native basic capture fields differ")
	}
	captured, _ = hex.DecodeString("01bcb19d001c000000")
	if !bytes.Equal(a[:9], captured) {
		t.Fatal("native additional capture fields differ")
	}
	if binary.LittleEndian.Uint64(b[7760:]) != 35987 || !bytes.Equal(b[7768:], []byte{0, 1, 1, 1}) || !bytes.Equal(a[2401:], []byte{0, 1, 1, 1}) {
		t.Fatal("elapsed/owner layout")
	}
	_, a, err = IspinsSourceRewardPayloads(nil, []IspinsRewardItem{{Template: 123, Amount: 700}}, time.Second, true)
	if err != nil || binary.LittleEndian.Uint32(a[5:]) != 700 {
		t.Fatal("quantity truncated", err)
	}
	b, a, err = IspinsSourceRewardPayloads(nil, nil, 0, false)
	if err != nil || a[0] != 0 || binary.LittleEndian.Uint32(b[1600:]) != 0 {
		t.Fatal("ineligible display invents awards")
	}
}
