package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestAppearanceIncludesMandatoryBlobLength(t *testing.T) {
	rows := []Equipment{{12, 101000013}, {3, 40601}, {47, 100610096}}
	p, e := EquipmentAppearance(rows)
	if e != nil || len(p) != 106 {
		t.Fatalf("%d %v", len(p), e)
	}
	for i, r := range rows {
		off := 1 + i*35
		if p[off] != r.Slot || binary.LittleEndian.Uint32(p[off+1:]) != r.Item || !bytes.Equal(p[off+5:off+35], make([]byte, 30)) {
			t.Fatal("appearance boundary")
		}
	}
	p, e = UserInfoBasicProbe(EntryBasicProbe{ActorServerID: 503, Character: CharacterRow{Name: "normal_test", Level: 115, Equipment: rows}})
	if e != nil || len(p) != 307+11+105 {
		t.Fatalf("entry appearance %d %v", len(p), e)
	}
	if out := os.Getenv("APPEARANCE_FIXTURE_DIR"); out != "" {
		if e := os.WriteFile(filepath.Join(out, "basic.bin"), p, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = EquipmentAppearance([]Equipment{{12, 1}, {12, 2}}); e == nil {
		t.Fatal("duplicate")
	}
	if _, e = EquipmentAppearance([]Equipment{{48, 1}}); e == nil {
		t.Fatal("bounds")
	}
}

func TestSystemAwakeningCapture(t *testing.T) {
	p, _ := hex.DecodeString("042b74110000000058f18f11000100000000000000000000")
	for stage := byte(1); stage <= 3; stage++ {
		p[13] = stage
		got, e := DecodeSystemAwakening(p)
		if e != nil || got != stage {
			t.Fatal(got, e)
		}
	}
	for _, bad := range [][]byte{p[:16], append(append([]byte{}, p...), 0), make([]byte, 24)} {
		if _, e := DecodeSystemAwakening(bad); e == nil {
			t.Fatal("invalid awakening accepted")
		}
	}
}
