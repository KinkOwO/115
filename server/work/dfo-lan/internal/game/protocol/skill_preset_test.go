package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestDecodeNativeSkillPresetSave(t *testing.T) {
	semantic, _ := hex.DecodeString("00000000000000004020f22c0100000d0009002e00050000000000")
	wire := append(append([]byte(nil), semantic...), make([]byte, 5)...)
	preset, err := DecodeSkillPresetSave(wire)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString("00000d0009002e00050000000000")
	if !bytes.Equal(preset.Config[:], want) {
		t.Fatalf("config=%x want=%x", preset.Config, want)
	}
	info := SkillPresetInfo(preset)
	if len(info) != 28 || !bytes.Equal(info[:14], want) || !bytes.Equal(info[14:], want) {
		t.Fatalf("NOTI2758=%x", info)
	}
}

func TestDecodeSkillPresetSaveRejectsTruncationAndDigestDamage(t *testing.T) {
	if _, err := DecodeSkillPresetSave(make([]byte, 26)); err == nil {
		t.Fatal("truncated save accepted")
	}
	semantic := make([]byte, 27)
	wire := append(append([]byte(nil), semantic...), make([]byte, 5)...)
	wire[27] ^= 1
	if _, err := DecodeSkillPresetSave(wire); err == nil {
		t.Fatal("nonzero padding accepted")
	}
}
