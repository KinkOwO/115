package main

import (
	"bytes"
	"dfolan/internal/game/wire"
	"os"
	"testing"
)

func TestActualLoginProjection(t *testing.T) {
	raw, err := os.ReadFile("../../runtime/login_ok.bin")
	if err != nil {
		t.Fatal(err)
	}
	out, old, err := project(raw, 22)
	if err != nil {
		t.Fatal(err)
	}
	if old != 0 || len(out) != len(raw) {
		t.Fatal("unexpected baseline or size")
	}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	a, _ := wire.DecryptPayload(keys, 1, raw[16:])
	b, _ := wire.DecryptPayload(keys, 1, out[16:])
	if b[3] != 22 {
		t.Fatal("channel")
	}
	b[3] = a[3]
	if !bytes.Equal(a, b) {
		t.Fatal("unrelated LOGIN fields changed")
	}
	restored, _, err := project(out, old)
	if err != nil || !bytes.Equal(restored, raw) {
		t.Fatal("rollback differs")
	}
	bad := bytes.Clone(raw)
	bad[16] ^= 1
	if _, _, err = project(bad, 22); err == nil {
		t.Fatal("bad checksum accepted")
	}
	t.Log("baseline=0 modified=22 rollback=0; only payload[3] changed; exact rollback; invalid checksum rejected")
}
