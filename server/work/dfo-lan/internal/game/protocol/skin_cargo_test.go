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
		if got.Category != tc.category || got.SkinID != tc.id ||
			len(got.SkinIDs) != 1 || got.SkinIDs[0] != tc.id {
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
		if back.Category != category || back.SkinID != SkinSelectionDefaultFont ||
			len(back.SkinIDs) != 1 || back.SkinIDs[0] != SkinSelectionDefaultFont {
			t.Fatalf("echo decodes back as %+v", back)
		}
	}
	if _, e := SelectSkinEcho(1, SkinSelectionDefaultFont); e == nil {
		t.Fatal("unreversed category encoded")
	}
}

// TestSkinSelectionPartyFrameLayout pins the two channels of the 边框 selection
// frame: three single-value slots, zero-padded, and the trailing list that refills
// the client's acquired set. sub_1444EECA0 case 0 appends the three singles in read
// order into one vector, so their position carries no meaning; the list is a
// different destination and has to stay a list.
func TestSkinSelectionPartyFrameLayout(t *testing.T) {
	p, e := SkinSelectionPartyFrame([]uint32{20001, 50002}, []uint32{80001})
	if e != nil {
		t.Fatal(e)
	}
	want := []byte{0}
	want = add32(want, 20001)
	want = add32(want, 50002)
	want = add32(want, 0)
	want = add16(want, 1)
	want = add32(want, 80001)
	if !bytes.Equal(p, want) {
		t.Fatalf("party frame selection = %x, want %x", p, want)
	}
	if _, e = SkinSelectionPartyFrame([]uint32{1, 2, 3, 4}, nil); e == nil {
		t.Fatal("fourth single slot accepted")
	}
}

// TestSkinSelectionSkillCutsceneLayout pins both lists of the 觉醒插图 frame. The first
// replaces mgr+296[1] (the 一觉 draw pool sub_1444EA8A0 renders), the second is assigned
// to mgr+1152 and mgr+1176 (df13_noti1546_body_1444eeca0.c:368-371) and mgr+1152 is what
// sub_1444EBAD0 draws the 二觉 cutscene with inside a dungeon, so it is not an optional
// tail an empty count can stand in for.
func TestSkinSelectionSkillCutsceneLayout(t *testing.T) {
	p, e := SkinSelectionSkillCutscene([]uint32{30001}, []uint32{100002, 100003})
	if e != nil {
		t.Fatal(e)
	}
	want := []byte{1}
	want = add16(want, 1)
	want = add32(want, 30001)
	want = add16(want, 2)
	want = add32(want, 100002)
	want = add32(want, 100003)
	if !bytes.Equal(p, want) {
		t.Fatalf("skill cutscene selection = %x, want %x", p, want)
	}
	p, e = SkinSelectionSkillCutscene([]uint32{30001}, nil)
	if e != nil {
		t.Fatal(e)
	}
	want = want[:7]
	want = add16(want, 0)
	if !bytes.Equal(p, want) {
		t.Fatalf("empty second list = %x, want %x", p, want)
	}
}

// TestDecodeSelectSkinSplitsTheCutsceneBodyByPosition replays the body the panel built on
// 2026-09-27 23:24: slot 0 holds the 二觉 list's 100000 marker and the 一觉 picks sit from
// slot 10 up, because sub_1444F1410 lays mgr+1176 into slots 0..9 and mgr+1128 into slots
// 10..19. The merged id list cannot express that, so the decode has to keep both halves.
func TestDecodeSelectSkinSplitsTheCutsceneBodyByPosition(t *testing.T) {
	body := make([]byte, SelectSkinBodySize)
	binary.LittleEndian.PutUint32(body, SkinCategorySkillCutscene)
	binary.LittleEndian.PutUint32(body[8:], SkinSelectionSecondAwakeningDefault)
	binary.LittleEndian.PutUint32(body[8+4*10:], SkinSelectionCutsceneDefault)
	binary.LittleEndian.PutUint32(body[8+4*11:], 30117)
	binary.LittleEndian.PutUint32(body[8+4*12:], 100001)
	got, e := DecodeSelectSkin(body)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Awakening) != 3 || got.Awakening[0] != SkinSelectionCutsceneDefault ||
		got.Awakening[2] != 100001 {
		t.Fatalf("一觉 list = %v", got.Awakening)
	}
	if len(got.SecondAwakening) != 1 || got.SecondAwakening[0] != SkinSelectionSecondAwakeningDefault {
		t.Fatalf("二觉 list = %v", got.SecondAwakening)
	}
	if len(got.SkinIDs) != 4 {
		t.Fatalf("merged ids = %v", got.SkinIDs)
	}
	// The 边框 body keeps its single merged list; only the cutscene tab is positional.
	body = make([]byte, SelectSkinBodySize)
	binary.LittleEndian.PutUint32(body, SkinCategoryPartyFrame)
	binary.LittleEndian.PutUint32(body[8:], 20078)
	got, e = DecodeSelectSkin(body)
	if e != nil {
		t.Fatal(e)
	}
	if got.Awakening != nil || got.SecondAwakening != nil {
		t.Fatalf("party frame body filled the cutscene lists: %v / %v", got.Awakening, got.SecondAwakening)
	}
}

// TestDecodeSelectSkinListCategory replays what the 边框 and 觉醒插图 panels send:
// one category, one result, and a selection spread over the body's id slots. The
// reader walks all 20 of them and skips the zero ones, so a padded body has to
// decode into exactly the ids that were set.
func TestDecodeSelectSkinListCategory(t *testing.T) {
	for _, tc := range []struct {
		category uint32
		ids      []uint32
	}{
		{SkinCategoryPartyFrame, []uint32{20001, 50002, 80001}},
		{SkinCategorySkillCutscene, []uint32{30001}},
		{SkinCategorySkillCutscene, nil},
	} {
		body := make([]byte, SelectSkinBodySize)
		binary.LittleEndian.PutUint32(body, tc.category)
		for i, id := range tc.ids {
			binary.LittleEndian.PutUint32(body[8+4*i:], id)
		}
		got, e := DecodeSelectSkin(body)
		if e != nil {
			t.Fatal(e)
		}
		if got.Category != tc.category {
			t.Fatalf("category = %d, want %d", got.Category, tc.category)
		}
		if len(got.SkinIDs) != len(tc.ids) {
			t.Fatalf("request ids = %v, want %v", got.SkinIDs, tc.ids)
		}
		var first uint32
		if len(tc.ids) > 0 {
			first = tc.ids[0]
		}
		if got.SkinID != first {
			t.Fatalf("SkinID = %d, want %d", got.SkinID, first)
		}
	}
	// The damage-font tabs stay single-value: their reader only looks at slot 0, so
	// a body that fills another slot is refused rather than reinterpreted.
	multi := make([]byte, SelectSkinBodySize)
	binary.LittleEndian.PutUint32(multi, SkinSelectionDamageFontNormal)
	binary.LittleEndian.PutUint32(multi[8:], 18)
	binary.LittleEndian.PutUint32(multi[24:], 19)
	if _, e := DecodeSelectSkin(multi); e == nil {
		t.Fatal("damage-font body with a second id slot accepted")
	}
}

// TestSelectSkinEchoRawKeepsTheRequestBody pins the only echo shape the list
// categories can use. sub_1444EE820 branches on the body's second u32 before it
// reaches its category switch, and its result==4 case erases one id from the acquired
// set, so rebuilding the body from a filtered id list would drop the client's intent.
func TestSelectSkinEchoRawKeepsTheRequestBody(t *testing.T) {
	body := make([]byte, SelectSkinBodySize)
	binary.LittleEndian.PutUint32(body, SkinCategoryPartyFrame)
	binary.LittleEndian.PutUint32(body[4:], 4)
	binary.LittleEndian.PutUint32(body[8:], 80001)
	p, e := SelectSkinEchoRaw(body)
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 1+SelectSkinBodySize || p[0] != 1 {
		t.Fatalf("echo = kind byte %d, %d bytes", p[0], len(p))
	}
	if !bytes.Equal(p[1:], body) {
		t.Fatalf("echo body = %x, want %x", p[1:], body)
	}
	if _, e = SelectSkinEchoRaw(body[:80]); e == nil {
		t.Fatal("short echo body accepted")
	}
}

// TestIsSkinSelectionFamilyBuiltin pins which ids a 解除 click may name without the
// account owning anything: each list family's own default rows. The damage-font tabs
// answer false because their sentinel lives in SkinSelectionResetID instead.
func TestIsSkinSelectionFamilyBuiltin(t *testing.T) {
	for _, tc := range []struct {
		category, id uint32
		want         bool
	}{
		{SkinCategoryPartyFrame, 20000, true},
		{SkinCategoryPartyFrame, 80000, true},
		{SkinCategorySkillCutscene, 100000, true},
		{SkinCategoryPartyFrame, 20001, false},
		{SkinSelectionDamageFontNormal, SkinSelectionNormalDamageDefaultFont, false},
	} {
		if got := IsSkinSelectionFamilyBuiltin(tc.category, tc.id); got != tc.want {
			t.Fatalf("IsSkinSelectionFamilyBuiltin(%d, %d) = %v, want %v", tc.category, tc.id, got, tc.want)
		}
	}
}
