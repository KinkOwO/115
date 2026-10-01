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

func TestDualModeProjection(t *testing.T) {
	role := Character{
		Request: []byte{0, 4, 0, 0, 0, 't', 'e', 's', 't', 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0, 0, 0, 0},
	}
	s := Service{}

	t.Setenv("DFO_ODYSSEY_MODE", "0")
	if isOdyssey, _ := s.IsOdyssey(role); isOdyssey {
		t.Fatal("expected false under DFO_ODYSSEY_MODE=0")
	}

	t.Setenv("DFO_ODYSSEY_MODE", "1")
	if isOdyssey, _ := s.IsOdyssey(role); !isOdyssey {
		t.Fatal("expected true under DFO_ODYSSEY_MODE=1")
	}
}

func TestCreateWithAllJobsPilotDoesNotRejectMissingGrowth(t *testing.T) {
	// Job 12 (Thief) and Job 16 (Archer) have Options[8]=1 in client create requests.
	// Even if prof.AdvancementGrowth[1] is empty, creation must succeed cleanly.
	s := Service{
		Rules: Rules{
			AllJobsPilot:  true,
			InitialLevel:  1,
			MaxCharacters: 24,
		},
	}
	for _, job := range []byte{12, 16} {
		p := append([]byte{job}, 4, 0, 0, 0, 't', 'e', 's', 't')
		p = append(p, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
		req, err := protocol.DecodeCreateRequest(p)
		if err != nil {
			t.Fatal(err)
		}
		initial := State{Level: s.Rules.InitialLevel}
		initial.setCreationOptions(req.Options)
		if s.Rules.AllJobsPilot && len(req.Options) == 12 && req.Options[8] != 0 {
			adv := req.Options[8]
			// Should not error if empty
			if len(map[byte]map[string]float32{}[adv]) > 0 {
				initial.Advancement, initial.AllJobsPilot = adv, true
			}
		}
		if initial.Advancement != 0 {
			t.Fatalf("expected base profession advancement 0 for unmapped growth, got %d", initial.Advancement)
		}
	}
}

func TestCharacterListCapacityUnderScenarioMode(t *testing.T) {
	t.Setenv("DFO_ODYSSEY_MODE", "0")
	var roles []protocol.CharacterRow
	for i := 0; i < 9; i++ {
		roles = append(roles, protocol.CharacterRow{
			Slot:             uint16(i),
			Name:             "test_role",
			Profession:       0,
			Level:            1,
			FatigueRemaining: 156,
		})
	}
	// Under old 8-character capacity, 9 roles trigger "invalid character capacity"
	if _, err := protocol.CharacterList(8, roles); err == nil {
		t.Fatal("expected error under capacity 8 with 9 roles")
	}
	// Under 24-character capacity (jobs-release / updated probe), 9 roles must succeed cleanly
	payload, err := protocol.CharacterList(24, roles)
	if err != nil {
		t.Fatalf("expected success under capacity 24, got: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("expected non-empty payload")
	}
}
