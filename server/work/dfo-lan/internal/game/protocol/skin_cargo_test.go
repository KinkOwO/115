package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestSkinCargoPageMatchesReader pins the byte layout sub_1444EFF40 consumes: one
// u8 page, the first owned count, {u32 id, u32 expiry} per entry, then the second
// (remapped) list count. Page 2 is the damage-font page.
func TestSkinCargoPageMatchesReader(t *testing.T) {
	p, e := SkinCargoPage(SkinCargoDamageFontPage, []uint32{12, 59})
	if e != nil {
		t.Fatal(e)
	}
	want := []byte{2, 2, 0,
		12, 0, 0, 0, 0, 0, 0, 0,
		59, 0, 0, 0, 0, 0, 0, 0,
		0, 0}
	if !bytes.Equal(p, want) {
		t.Fatalf("page = %x, want %x", p, want)
	}

	empty, e := SkinCargoPage(SkinCargoDamageFontPage, nil)
	if e != nil {
		t.Fatal(e)
	}
	if want := []byte{2, 0, 0, 0, 0}; !bytes.Equal(empty, want) {
		t.Fatalf("empty page = %x, want %x", empty, want)
	}

	// Page 4 is read as bare u32 ids, so this builder must refuse it rather
	// emit a frame the client consumes as half the entries.
	if _, e = SkinCargoPage(4, []uint32{12}); e == nil {
		t.Fatal("page 4 accepted")
	}
}

// TestSkinSelectionDamageFontMatchesReader pins the 5-byte NOTI1546 frame the
// damage-font cases of sub_1444EECA0 consume: the u8 category the handler
// dispatches on, then the one u32 skin id it looks up in owned page 2. A category
// outside the panel's pair is refused rather than encoded.
func TestSkinSelectionDamageFontMatchesReader(t *testing.T) {
	for _, tc := range []struct {
		category, id uint32
		want         []byte
	}{
		{SkinSelectionDamageFontNormal, 12, []byte{2, 12, 0, 0, 0}},
		{SkinSelectionDamageFontCumulative, 12, []byte{6, 12, 0, 0, 0}},
		{SkinSelectionDamageFontNormal, SkinSelectionNormalDamageDefaultFont, []byte{2, 1, 0, 0, 0}},
		{SkinSelectionDamageFontCumulative, SkinSelectionDefaultFont, []byte{6, 0xff, 0xe0, 0xf5, 0x05}},
	} {
		got, e := SkinSelectionDamageFont(tc.category, tc.id)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(got, tc.want) {
			t.Fatalf("category %d selection = %x, want %x", tc.category, got, tc.want)
		}
	}
	if _, e := SkinSelectionDamageFont(1, 12); e == nil {
		t.Fatal("unreversed category encoded")
	}
}

// TestSkinSelectionResetIDsArePerTab pins that each tab's 解除 names its own
// built-in default instead of one shared sentinel: the holder constructor starts
// holder+112 at 1 and holder+232 at -1 (df32_holder_ctor_sub_1447E41D0.c:186 and
// :123), and the live 2026-09-27 05:28 capture shows category 2 解除 arriving with
// id 1 while category 6 解除 arrives with 99999999.
func TestSkinSelectionResetIDsArePerTab(t *testing.T) {
	for _, tc := range []struct {
		category, want uint32
	}{
		{SkinSelectionDamageFontNormal, SkinSelectionNormalDamageDefaultFont},
		{SkinSelectionDamageFontCumulative, SkinSelectionDefaultFont},
	} {
		got, ok := SkinSelectionResetID(tc.category)
		if !ok || got != tc.want {
			t.Fatalf("category %d reset id = %d (%v), want %d", tc.category, got, ok, tc.want)
		}
	}
	for _, category := range []uint32{0, 1, 3, 4, 5, 7} {
		if _, ok := SkinSelectionResetID(category); ok {
			t.Fatalf("category %d claims a reset id", category)
		}
	}
}

// TestDecodeSelectSkinLiveVectors replays the bodies the 应用 and 解除 buttons sent
// on 2026-09-27: 88 plain bytes naming a selection category and a skin id. Skin id
// 18 arrived under both category 2 and category 6, which is what proves the first
// u32 names the panel tab's selection rather than a cargo page. The `2, 0, 1` body
// is that tab's 解除, recorded seven times in the 05:28 run.
func TestDecodeSelectSkinLiveVectors(t *testing.T) {
	for _, tc := range []struct{ category, id uint32 }{
		{2, 12}, {2, 18}, {6, 18}, {6, SkinSelectionDefaultFont},
		{2, SkinSelectionNormalDamageDefaultFont},
	} {
		p := make([]byte, SelectSkinBodySize)
		binary.LittleEndian.PutUint32(p, tc.category)
		binary.LittleEndian.PutUint32(p[8:], tc.id)
		got, e := DecodeSelectSkin(p)
		if e != nil {
			t.Fatal(e)
		}
		if got != (SelectSkinRequest{Category: tc.category, SkinID: tc.id}) {
			t.Fatalf("request = %+v", got)
		}
	}
	p := make([]byte, SelectSkinBodySize)
	if _, e := DecodeSelectSkin(p[:80]); e == nil {
		t.Fatal("short body accepted")
	}
	trailing := make([]byte, SelectSkinBodySize)
	trailing[0], trailing[16] = 2, 9
	if _, e := DecodeSelectSkin(trailing); e == nil {
		t.Fatal("second id slot accepted")
	}
}

// TestSelectSkinEchoMatchesReader pins the body sub_1444EE820 consumes on the way
// back in: the command reply's one status byte (the router consumes it before it
// dispatches, and the handler treats 0 as "failure, show the toast") in front of
// the 88-byte payload. The reader bulk-reads that block and only reaches its
// category switch when the second u32 is zero, so the echo has to leave the
// result cell empty; the id goes in the first slot because every damage-font case
// reads ids[0]. Decoding the payload with the request decoder is the check that
// both directions really use this one layout.
func TestSelectSkinEchoMatchesReader(t *testing.T) {
	for _, category := range []uint32{SkinSelectionDamageFontNormal, SkinSelectionDamageFontCumulative} {
		p, e := SelectSkinEcho(category, SkinSelectionDefaultFont)
		if e != nil {
			t.Fatal(e)
		}
		if len(p) != 1+SelectSkinBodySize {
			t.Fatalf("echo is %d bytes, want %d", len(p), 1+SelectSkinBodySize)
		}
		if p[0] == 0 {
			t.Fatalf("echo status byte is 0, which routes to the failure toast")
		}
		body := p[1:]
		if got := binary.LittleEndian.Uint32(body[4:]); got != 0 {
			t.Fatalf("echo result = %d, the reader skips its switch unless it is 0", got)
		}
		for _, b := range body[12:] {
			if b != 0 {
				t.Fatalf("echo set a byte outside the first id slot: %x", p)
			}
		}
		back, e := DecodeSelectSkin(body)
		if e != nil {
			t.Fatal(e)
		}
		if back != (SelectSkinRequest{Category: category, SkinID: SkinSelectionDefaultFont}) {
			t.Fatalf("echo decodes back as %+v", back)
		}
	}
	if _, e := SelectSkinEcho(1, SkinSelectionDefaultFont); e == nil {
		t.Fatal("unreversed category encoded")
	}
}
