package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestCapturedQuestSubmit(t *testing.T) {
	p, _ := hex.DecodeString("2200490cffff0100")
	q, e := DecodeQuestSubmit(p)
	if e != nil || q.ID != 3145 || q.RewardSelection != 65535 || q.Option != 1 {
		t.Fatalf("capture mismatch: %+v %v", q, e)
	}
	if _, e = DecodeQuestSubmit(p[:6]); e == nil {
		t.Fatal("truncated quest submit accepted")
	}
	if !bytes.Equal(QuestSubmitRefused(), []byte{0, 19, 0}) {
		t.Fatal("refusal reached reward branch")
	}
}
func TestVaultNativeBoundary(t *testing.T) {
	p, e := EmptyPersonalVault(8)
	if e != nil || !bytes.Equal(p, []byte{2, 8, 0, 0, 0, 0}) {
		t.Fatalf("vault native empty row boundary: %x %v", p, e)
	}
	if _, e = EmptyPersonalVault(0); e == nil {
		t.Fatal("uninitialized vault grade accepted")
	}
}
