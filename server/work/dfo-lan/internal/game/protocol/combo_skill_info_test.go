package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// Every body below is one observed on the wire (2026-09-26/27 sessions).
func TestDecodeComboSkillInfoReadsObservedBodies(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		cells []ComboCell
	}{
		{
			name: "six cells, no chains",
			body: "00067600007700007800007900007a00007b0000",
			cells: []ComboCell{
				{Skill: 118}, {Skill: 119}, {Skill: 120},
				{Skill: 121}, {Skill: 122}, {Skill: 123},
			},
		},
		{
			name: "cell 118 chains 108",
			body: "00067600016c007700007800007900007a00007b00000000",
			cells: []ComboCell{
				{Skill: 118, Chain: []uint16{108}}, {Skill: 119}, {Skill: 120},
				{Skill: 121}, {Skill: 122}, {Skill: 123},
			},
		},
		{
			name: "cell 118 chains 46 108 5 8 117",
			body: "00067600052e006c000500080075007700007800007900007a00007b00000000",
			cells: []ComboCell{
				{Skill: 118, Chain: []uint16{46, 108, 5, 8, 117}}, {Skill: 119}, {Skill: 120},
				{Skill: 121}, {Skill: 122}, {Skill: 123},
			},
		},
		{
			name: "cell 118 chains 24 73 74 81 252",
			body: "0006760005180049004a005100fc007700007800007900007a00007b00000000",
			cells: []ComboCell{
				{Skill: 118, Chain: []uint16{24, 73, 74, 81, 252}}, {Skill: 119}, {Skill: 120},
				{Skill: 121}, {Skill: 122}, {Skill: 123},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := hex.DecodeString(tc.body)
			if err != nil {
				t.Fatalf("bad fixture: %v", err)
			}
			info, err := DecodeComboSkillInfo(raw)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(info.Cells) != len(tc.cells) {
				t.Fatalf("cells = %d, want %d", len(info.Cells), len(tc.cells))
			}
			for i, want := range tc.cells {
				got := info.Cells[i]
				if got.Skill != want.Skill {
					t.Errorf("cell %d skill = %d, want %d", i, got.Skill, want.Skill)
				}
				if len(got.Chain) != len(want.Chain) {
					t.Fatalf("cell %d chain len = %d, want %d", i, len(got.Chain), len(want.Chain))
				}
				for j := range want.Chain {
					if got.Chain[j] != want.Chain[j] {
						t.Errorf("cell %d chain[%d] = %d, want %d", i, j, got.Chain[j], want.Chain[j])
					}
				}
			}
			// Raw stops at the last cell byte: the pad is not replayed.
			if pure, _ := EncodeComboSkillInfo(info); !bytes.Equal(info.Raw, pure) {
				t.Errorf("Raw = %x, want %x", info.Raw, pure)
			}
		})
	}
}

// The client aligns the body to a 4-byte multiple; the pad must be tolerated
// and must not survive into Raw.
func TestDecodeComboSkillInfoTrimsAlignmentPad(t *testing.T) {
	raw, _ := hex.DecodeString("00067600007700007800007900007a00007b0000000000")
	info, err := DecodeComboSkillInfo(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := raw[:20]; !bytes.Equal(info.Raw, want) {
		t.Fatalf("Raw = %x, want %x", info.Raw, want)
	}
}

func TestEncodeComboSkillInfoRoundTrips(t *testing.T) {
	in := ComboSkillInfo{Cells: []ComboCell{
		{Skill: 118, Chain: []uint16{46, 108, 5, 8, 117}},
		{Skill: 119}, {Skill: 120}, {Skill: 121}, {Skill: 122}, {Skill: 123},
	}}
	out, err := EncodeComboSkillInfo(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	back, err := DecodeComboSkillInfo(out)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(back.Cells) != len(in.Cells) {
		t.Fatalf("cells = %d, want %d", len(back.Cells), len(in.Cells))
	}
	for i := range in.Cells {
		if back.Cells[i].Skill != in.Cells[i].Skill {
			t.Errorf("cell %d skill = %d, want %d", i, back.Cells[i].Skill, in.Cells[i].Skill)
		}
		if len(back.Cells[i].Chain) != len(in.Cells[i].Chain) {
			t.Fatalf("cell %d chain len = %d, want %d", i, len(back.Cells[i].Chain), len(in.Cells[i].Chain))
		}
		for j := range in.Cells[i].Chain {
			if back.Cells[i].Chain[j] != in.Cells[i].Chain[j] {
				t.Errorf("cell %d chain[%d] = %d, want %d", i, j, back.Cells[i].Chain[j], in.Cells[i].Chain[j])
			}
		}
	}
}

func TestDecodeComboSkillInfoRejectsBrokenBodies(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"header only":      "00",
		"zero cells":       "0000",
		"nonzero header":   "01067600007700007800007900007a00007b0000",
		"truncated entry":  "00067600",
		"zero combo skill": "0006000000",
		"duplicate cell":   "0002760000760000",
		"short chain":      "0001760005760000770000780000790000",
		"zero chain skill": "0001760001000000",
		"nonzero pad":      "00067600007700007800007900007a00007b0001",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			raw, err := hex.DecodeString(body)
			if err != nil {
				t.Fatalf("bad fixture: %v", err)
			}
			if _, err := DecodeComboSkillInfo(raw); err == nil {
				t.Fatalf("decode(%s) succeeded, want error", body)
			}
		})
	}
}

// NOTI433 is the S2C arm, and the two directions do not share a layout: the
// notify body is a three byte header (T1, A1, PAGE) followed by the C2S cell
// list with the C2S leading '0' dropped. See §18 of
// docs/protocol/dark-knight-comboset-quickbar-20260927.md.
func TestEncodeComboSkillInfoNotifyPrependsTheS2CHeader(t *testing.T) {
	in := ComboSkillInfo{Cells: []ComboCell{
		{Skill: 118}, {Skill: 119}, {Skill: 120},
		{Skill: 121}, {Skill: 122}, {Skill: 123},
	}}
	body, err := EncodeComboSkillInfoNotify(in)
	if err != nil {
		t.Fatalf("encode notify: %v", err)
	}
	want, _ := hex.DecodeString("000100" + "06" + "7600007700007800007900007a00007b0000")
	if !bytes.Equal(body, want) {
		t.Fatalf("notify body = %x, want %x", body, want)
	}
	if body[0] != 0x00 || body[1] != 0x01 || body[2] != 0x00 {
		t.Fatalf("notify header = %x, want 00 01 00", body[:3])
	}
}

// Echoing the C2S body back is the exact bug that crashed the client: the
// client reads byte[1] as the block count and byte[2] as the page index, so a
// verbatim echo makes PAGE = 0x76 and dereferences far past the object.
func TestEncodeComboSkillInfoNotifyNeverEqualsTheC2SBody(t *testing.T) {
	cases := []string{
		"00067600007700007800007900007a00007b0000",
		"00067600016c007700007800007900007a00007b00000000",
		"00067600052e006c000500080075007700007800007900007a00007b00000000",
	}
	for _, hexBody := range cases {
		raw, err := hex.DecodeString(hexBody)
		if err != nil {
			t.Fatalf("bad fixture: %v", err)
		}
		info, err := DecodeComboSkillInfo(raw)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		notify, err := EncodeComboSkillInfoNotify(info)
		if err != nil {
			t.Fatalf("encode notify: %v", err)
		}
		if bytes.Equal(notify, raw) {
			t.Errorf("notify body equals the C2S body %s", hexBody)
		}
		// PAGE must stay 0 no matter what the arrangement is; the C2S body's
		// third byte is the first combo skill id (0x76).
		if notify[2] != 0x00 {
			t.Errorf("PAGE = %#x, want 0x00 (body %s)", notify[2], hexBody)
		}
		// Dropping the S2C prefix and re-adding the C2S one round-trips.
		back := append([]byte{0x00}, notify[3:]...)
		if !bytes.Equal(back, info.Raw) {
			t.Errorf("strip prefix = %x, want %x", back, info.Raw)
		}
	}
}
