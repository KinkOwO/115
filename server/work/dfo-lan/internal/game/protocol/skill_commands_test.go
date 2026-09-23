package protocol

import (
	"encoding/hex"
	"reflect"
	"testing"
)

func TestDecodeNativeSkillCommandSnapshots(t *testing.T) {
	tests := []struct {
		hex  string
		want map[uint16][]uint32
	}{
		{"00000000000000000000000000000000", map[uint16][]uint32{}},
		{"01460001080000000000000000000000", map[uint16][]uint32{70: {8}}},
		{"02080001044600010800000000000000", map[uint16][]uint32{8: {4}, 70: {8}}},
		{"01460003000004000000000000000000", map[uint16][]uint32{70: {0, 0, 4}}},
	}
	for _, tt := range tests {
		p, err := hex.DecodeString(tt.hex)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeSkillCommands(p)
		if err != nil {
			t.Fatalf("%s: %v", tt.hex, err)
		}
		if !reflect.DeepEqual(got.Entries, tt.want) {
			t.Fatalf("%s: got %v, want %v", tt.hex, got.Entries, tt.want)
		}
		if len(got.Raw) != 1+len(tt.want)*4 && tt.hex != "01460003000004000000000000000000" {
			t.Fatalf("%s: unexpected normalized length %d", tt.hex, len(got.Raw))
		}
	}
}

func TestDecodeSkillCommandsRejectsBrokenEntries(t *testing.T) {
	for _, p := range [][]byte{
		{}, {1, 70}, {1, 70, 0, 0}, {1, 70, 0, 6, 8},
		{1, 70, 0, 1, 7}, {1, 70, 0, 1, 0},
		{2, 70, 0, 1, 8, 70, 0, 1, 8},
		{1, 70, 0, 1, 8, 1},
	} {
		if _, err := DecodeSkillCommands(p); err == nil {
			t.Fatalf("accepted invalid snapshot %x", p)
		}
	}
}
