package character

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestCreationModeIsPreservedSeparatelyFromAdvancement(t *testing.T) {
	s := State{Level: 1, CreationOptions: []byte{0, 0, 0, 0, 0, 0, 0, 0, 1}, CreationMode: 1}
	b, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var g State
	if e = json.Unmarshal(b, &g); e != nil {
		t.Fatal(e)
	}
	if g.CreationMode != 1 || g.Advancement != 0 || len(g.CreationOptions) != 9 {
		t.Fatalf("%+v", g)
	}
}

func TestLiveCreationModes(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		mode          byte
	}{
		{"test01", "0006000000746573743031000000000000ff000100020000", 2},
		{"test02", "0006000000746573743032000000000000ff000100020000", 2},
		{"test03", "0006000000746573743033000000000000ff000100000000", 0},
		{"normal_test", "000b0000006e6f726d616c5f74657374000000000000ff000100000000000000", 0},
		{"odyssey_test", "000c0000006f6479737365795f74657374000000000000ff0001000200000000", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := hex.DecodeString(tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			req, err := protocol.DecodeCreateRequest(p)
			if err != nil {
				t.Fatal(err)
			}
			if req.Name != tc.name || req.Profession != 0 || req.Options[8] != 1 {
				t.Fatalf("request: %+v", req)
			}
			s := State{Level: 1}
			s.setCreationOptions(req.Options)
			b, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			var restored State
			if err := json.Unmarshal(b, &restored); err != nil {
				t.Fatal(err)
			}
			if restored.CreationMode != tc.mode || restored.Advancement != 0 || !bytes.Equal(restored.CreationOptions, req.Options) {
				t.Fatalf("state: %+v", restored)
			}
			req.Options[10] = 99
			if s.CreationOptions[10] != tc.mode {
				t.Fatal("creation options alias request")
			}
		})
	}
}

func TestCreationOptionsLegacyAndUnknown(t *testing.T) {
	for _, n := range []int{0, 9, 10, 11, 12} {
		opts := make([]byte, n)
		if n > 8 {
			opts[8] = 1
		}
		if n > 10 {
			opts[10] = 7
		}
		s := State{CreationMode: 2, Advancement: 3}
		s.setCreationOptions(opts)
		want := byte(0)
		if n == 12 {
			want = 7
		}
		if s.CreationMode != want || s.Advancement != 3 {
			t.Fatalf("length %d: %+v", n, s)
		}
	}
}
