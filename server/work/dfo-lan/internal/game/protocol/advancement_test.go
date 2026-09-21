package protocol

import "testing"

func TestDecodeChangeGrowType(t *testing.T) {
	body := make([]byte, 14)
	body[13] = 0x04 // knight -> dragon knight (advancement branch 4)
	adv, err := DecodeChangeGrowType(body)
	if err != nil || adv != 4 {
		t.Fatalf("got adv=%d err=%v, want 4/nil", adv, err)
	}
}

func TestDecodeChangeGrowTypePadded(t *testing.T) {
	body := make([]byte, 16)
	body[13] = 0x04
	adv, err := DecodeChangeGrowType(body)
	if err != nil || adv != 4 {
		t.Fatalf("got adv=%d err=%v, want 4/nil", adv, err)
	}
}

func TestDecodeChangeGrowTypeMasksAwakeningBits(t *testing.T) {
	body := make([]byte, 14)
	body[13] = 0x24 // awakening stage 2 packed above advancement branch 4
	adv, err := DecodeChangeGrowType(body)
	if err != nil || adv != 4 {
		t.Fatalf("got adv=%d err=%v, want 4/nil", adv, err)
	}
}

func TestDecodeChangeGrowTypeRejects(t *testing.T) {
	if _, err := DecodeChangeGrowType(make([]byte, 13)); err == nil {
		t.Fatal("short body must be rejected")
	}
	base := make([]byte, 14) // p[13] == 0 targets the base profession
	if _, err := DecodeChangeGrowType(base); err == nil {
		t.Fatal("advancement 0 must be rejected")
	}
	bad := make([]byte, 16)
	bad[13], bad[15] = 0x04, 0x01 // nonzero padding
	if _, err := DecodeChangeGrowType(bad); err == nil {
		t.Fatal("nonzero padding must be rejected")
	}
}
