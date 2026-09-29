package protocol

import (
	"bytes"
	"encoding/binary"
	"reflect"
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

// TestDecodeSelectSkinWeaponTabFrames replays the 武器外观 tab's captured bodies: the
// 2026-09-27 session recorded five category-4 frames naming the replicated skin 27694,
// three of them with a zero result and two with result 2 within a second of each other.
// That split is structural, not accidental — the tab's generic composer zeroes the
// result cell for every category (df39_sender_F1090.c: memset(&v24[1], 0, 84) and no
// later write to v24[1]), while the star toggle's composer writes (flag != 0) + 2, i.e.
// 2 or 3 (df39_sender_F0FE0.c). One body, two commands, so answering this category has
// to branch on Result: 0 is 应用, 2/3 is 收藏.
func TestDecodeSelectSkinWeaponTabFrames(t *testing.T) {
	for _, result := range []uint32{0, 2, 3} {
		p := make([]byte, SelectSkinBodySize)
		binary.LittleEndian.PutUint32(p, SkinCargoWeaponShape)
		binary.LittleEndian.PutUint32(p[4:], result)
		binary.LittleEndian.PutUint32(p[8:], 27694)
		got, e := DecodeSelectSkin(p)
		if e != nil {
			t.Fatal(e)
		}
		if got.Category != SkinCargoWeaponShape || got.Result != result ||
			got.SkinID != 27694 || len(got.SkinIDs) != 1 {
			t.Fatalf("request = %+v", got)
		}
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

// TestSkinSelectionInstantEmoticonIsPositional pins the 表情 frame's fixed width:
// sub_1444EECA0 case 3 reads four u32 with no count word anywhere, so a frame of three or
// five words would make the reader run off the payload or into the next field. The zeros
// are part of the state — they keep a cell's position — so they are written, not trimmed.
func TestSkinSelectionInstantEmoticonIsPositional(t *testing.T) {
	p, e := SkinSelectionInstantEmoticon([]uint32{40001, 0, 0, 40002})
	if e != nil {
		t.Fatal(e)
	}
	want := []byte{3}
	want = add32(want, 40001)
	want = add32(want, 0)
	want = add32(want, 0)
	want = add32(want, 40002)
	if !bytes.Equal(p, want) {
		t.Fatalf("emoticon selection = %x, want %x", p, want)
	}
	if len(p) != 1+4*SkinSelectionInstantEmoticonSlots {
		t.Fatalf("emoticon selection is %d bytes", len(p))
	}
	// A short list is refused rather than padded: the caller owns the cell layout.
	if _, e = SkinSelectionInstantEmoticon([]uint32{40001, 40002}); e == nil {
		t.Fatal("two-slot emoticon frame accepted")
	}
}

// TestSkinSelectionSingleCoversOnlyTheTwoSingletonFamilies pins the 涂鸦 / 飞空艇特效
// frame (u8 category, u32 id) and refuses every other category, because their readers
// consume a different body: 7 and 8 append one id as a one-element vector, while 2 and 6
// go through SkinSelectionDamageFont and 4 through the replication path.
func TestSkinSelectionSingleCoversOnlyTheTwoSingletonFamilies(t *testing.T) {
	for _, tc := range []struct {
		category, id uint32
		want         []byte
	}{
		{SkinCategorySpray, 90001, []byte{7, 0x91, 0x5f, 0x01, 0x00}},
		{SkinCategoryAirshipEffect, 90002, []byte{8, 0x92, 0x5f, 0x01, 0x00}},
	} {
		got, e := SkinSelectionSingle(tc.category, tc.id)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(got, tc.want) {
			t.Fatalf("category %d selection = %x, want %x", tc.category, got, tc.want)
		}
	}
	for _, category := range []uint32{SkinCategoryPartyFrame, SkinCategoryInstantEmoticon,
		SkinSelectionDamageFontNormal, SkinCargoWeaponShape} {
		if _, e := SkinSelectionSingle(category, 1); e == nil {
			t.Fatalf("category %d encoded as a single-value family", category)
		}
	}
}

// TestSkinFavoritesMatchesReader pins NOTI2641's body against sub_1444ED1B0: ten groups
// read as {u32 count, u32 ids[count]} with the group index taken from the loop counter —
// there is no page field on the wire, so group i is page i — followed by four trailing
// groups of the same shape. The reader frees the whole list before it parses, so a frame
// that omits a group is a group the player no longer has starred.
func TestSkinFavoritesMatchesReader(t *testing.T) {
	pages := make([][]uint32, SkinFavoritePages)
	pages[0] = []uint32{20001, 20002}
	pages[3] = []uint32{40001}
	p, e := SkinFavorites(pages)
	if e != nil {
		t.Fatal(e)
	}
	want := []byte{}
	for i := 0; i < SkinFavoritePages; i++ {
		want = add32(want, uint32(len(pages[i])))
		for _, id := range pages[i] {
			want = add32(want, id)
		}
	}
	for i := 0; i < SkinFavoriteCrossGroups; i++ {
		want = add32(want, 0)
	}
	if !bytes.Equal(p, want) {
		t.Fatalf("favourites = %x, want %x", p, want)
	}
	if len(p) != 4*(SkinFavoritePages+SkinFavoriteCrossGroups)+4*3 {
		t.Fatalf("favourites frame is %d bytes", len(p))
	}
	// A page count that does not match the reader's loop is refused: the groups are
	// positional, so a short slice would shift every later page onto a different key.
	if _, e = SkinFavorites(pages[:9]); e == nil {
		t.Fatal("nine-group favourite frame accepted")
	}
}

// TestDecodeSelectSkinEmoticonKeepsCellPositions replays the 表情 quick bar's flush body:
// four words, one per cell, with a hole in the middle. SkinIDs trims zeros because the set
// categories want a list, so the positional reader has to keep the untrimmed words.
func TestDecodeSelectSkinEmoticonKeepsCellPositions(t *testing.T) {
	body := make([]byte, SelectSkinBodySize)
	binary.LittleEndian.PutUint32(body, SkinCategoryInstantEmoticon)
	binary.LittleEndian.PutUint32(body[8:], 40001)
	binary.LittleEndian.PutUint32(body[8+4*2:], 40002)
	got, e := DecodeSelectSkin(body)
	if e != nil {
		t.Fatal(e)
	}
	if want := []uint32{40001, 0, 40002, 0}; !reflect.DeepEqual(got.EmoticonSlots, want) {
		t.Fatalf("slots = %v, want %v", got.EmoticonSlots, want)
	}
	if len(got.SkinIDs) != 2 {
		t.Fatalf("merged ids = %v", got.SkinIDs)
	}
	// Only the 表情 body fills the slot list; the other categories leave it nil so a
	// caller cannot mistake an absent selection for an empty bar.
	body = make([]byte, SelectSkinBodySize)
	binary.LittleEndian.PutUint32(body, SkinCategorySpray)
	binary.LittleEndian.PutUint32(body[8:], 90001)
	got, e = DecodeSelectSkin(body)
	if e != nil {
		t.Fatal(e)
	}
	if got.EmoticonSlots != nil {
		t.Fatalf("spray body filled the emoticon slots: %v", got.EmoticonSlots)
	}
}

// TestDecodeSelectSkinFavoriteFlagSplitsStarFromApply pins the one thing that keeps a star
// click from being answered as an 应用: the result word. The star composer writes
// (currently_starred != 0) + 2, so 2 and 3 are toggles and only 0 is an apply — the same
// split the 武器外观 capture showed on 2026-09-27, where one body shape carried both
// commands.
func TestDecodeSelectSkinFavoriteFlagSplitsStarFromApply(t *testing.T) {
	for _, tc := range []struct {
		result   uint32
		favorite bool
	}{
		{SkinSelectResultApply, false},
		{1, false},
		{SkinSelectResultFavoriteAdd, true},
		{SkinSelectResultFavoriteRemove, true},
		{SkinSelectResultAcquiredErase, false},
	} {
		body := make([]byte, SelectSkinBodySize)
		binary.LittleEndian.PutUint32(body, SkinCategoryPartyFrame)
		binary.LittleEndian.PutUint32(body[4:], tc.result)
		binary.LittleEndian.PutUint32(body[8:], 20001)
		got, e := DecodeSelectSkin(body)
		if e != nil {
			t.Fatal(e)
		}
		if got.Favorite != tc.favorite {
			t.Fatalf("result %d decoded as favorite=%v, want %v", tc.result, got.Favorite, tc.favorite)
		}
	}
}
