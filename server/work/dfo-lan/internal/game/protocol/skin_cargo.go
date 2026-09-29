package protocol

import (
	"encoding/binary"
	"fmt"
)

// SkinCargoDamageFontPage is the NOTI1545 owned page whose contents the
// damage-font panel renders. sub_1444ED900 reads a single u8 page selector and
// sub_1444EFF40 (0x1444EFF40) then rebuilds that page's owned map from the rest
// of the frame, so a page frame is absolute state: a frame naming only the
// newest skin deletes every skin the page already held.
//
// Page 2 is proven by the panel's own grid refill, sub_1441E3510, which
// enumerates page 2 through sub_1444EBDF0 and keeps the ids whose static
// skin-id->page registry record says page 2.
//
// The other pages each have a panel that enumerates one of 1, 3, 4, 7, 8 and 9,
// so a family is only pushed once its own page is proven like this.
const SkinCargoDamageFontPage = 2

// SkinCargoPartyFramePage and SkinCargoSkillCutscenePage are the owned pages the
// 边框 and 觉醒插图 panels enumerate. Each of the two has one panel and therefore one
// selection category, so the page number and the category number coincide — unlike
// the damage font, whose two tabs share page 2 under categories 2 and 6.
//
// The page is the same index the client's static registry stores as the record's
// family class: sub_1444EBD90(mgr, page, id) reads `mgr + 136 + 16*page`, the panel
// grid refill keeps only ids whose registry class equals its own page
// (df22_ui_1441E3F40 for 0, df22_ui_1441E4180 for 1), and each grid then singles out
// the built-in ids of the family it renders — 20000/50000/60000/80000 on page 0 and
// 30000/100000 on page 1.
const (
	SkinCargoPartyFramePage    = 0
	SkinCargoSkillCutscenePage = 1
)

// SkinCargoInstantEmoticonPage, SkinCargoSprayPage and SkinCargoAirshipEffectPage are
// the owned pages the 表情, 涂鸦 and 飞空艇特效 panels enumerate. Like the two list
// families above, each has one panel and therefore one selection category, so page and
// category coincide.
//
// The three numbers are the family class the client's registry loader gives each
// `[type]` label: sub_147C18830 compares the label text and stores the index at
// record+8 (analysis/dumps/CLIENT-MECHANICS.md 14.2.1, df40_loader_*.c), and each panel's
// grid refill keeps only the ids whose record class equals its own page
// (df41_grid_sub_1441E7BE0 for 3, df41_grid_sub_1441EA4F0 for 7,
// df41_grid_sub_1441EB0F0 + df41_grid_sub_1441E4600 for 8).
//
// None of the three has a client-shipped default row: the same five grid dumps hard-code
// no 5..8 digit skin id at all, unlike page 0 (20000/50000/60000/80000) and page 1
// (30000/100000). A page frame for these families therefore carries exactly the skins the
// account registered and nothing else.
const (
	SkinCargoInstantEmoticonPage = 3
	SkinCargoSprayPage           = 7
	SkinCargoAirshipEffectPage   = 8
)

// SkinCargoPartyFrameBuiltins are the three category-0 skins the client ships with:
// `Skin/PartyFrame/default.skn`, `Skin/PartyRequestFrame/default.skn` and
// `Skin/CharacterInfoBG/default.skn`. A NOTI1545 page frame rebuilds the whole page,
// so any page-0 push the server makes has to keep them alongside the skins the
// account bought, or it deletes them.
var SkinCargoPartyFrameBuiltins = []uint32{20000, 50000, 60000}

// SkinCargoPartyFrameDefaultList is the raid party list frame the client ships with,
// `Skin/PartyListFrame/Default.skn`. The border grid pulls it out of the owned page as
// that tab's own default row, so a page-0 push that wants the raid-list slot usable
// has to carry it: NOTI1545 rebuilds the page, and an id the page does not hold is
// invisible *and* unselectable (NOTI1546 case 0 validates every id against page 0
// before it stores it).
const SkinCargoPartyFrameDefaultList = 80000

// SkinCargoSkillCutsceneBuiltins are the two 觉醒插图 defaults the panel renders as
// its 默认 rows: `Skin/SkillCutscene/default.skn` (30000, the first awakening slot)
// and `Skin/SecondAwakeningCutscene/default.skn` (100000, the second).
// df22_ui_1441E4180.c:44-52 enumerates owned page 1 and singles those two ids out by
// value, so they are pulled out only when the page itself holds them.
var SkinCargoSkillCutsceneBuiltins = []uint32{30000, 100000}

// SkinSelectionCutsceneDefault is the 觉醒插图 tab's own "nothing chosen" id. The
// client's CMD1565 composer inserts it into the request body when the selection vector
// it is handed is empty (analysis/dumps/skin-noti/df36_fn_f1090_0x1444f1090.c, case 1),
// exactly as the damage-font tabs insert 1 and 99999999, and sub_1444EA8A0 renders 30000
// whenever that vector is empty anyway. So it marks "nothing chosen" only while it is that
// list's single element: it is also the tab's own default row, and a player who ticks it
// beside real skins is putting it into the draw on purpose.
const SkinSelectionCutsceneDefault = 30000

// SkinSelectionSecondAwakeningDefault is the 二次觉醒 list's own "nothing chosen" id,
// the exact counterpart of 30000 for the 一觉 list: sub_1444F1BE0 assigns mgr+1176 from
// a vector and inserts 100000 whenever that assignment leaves it empty
// (analysis/dumps/skin-noti/df37_writer_0x1444f1be0.c:14-21), and sub_1444EBAD0 returns
// 100000 for an empty mgr+1152 (df37_fn_ebad0_0x1444ebad0.c:11-15). Same rule as 30000:
// alone in mgr+1176 it marks "nothing picked", beside a real row it is that tab's default
// row ticked, and mgr+1176 is what the panel reads back when it opens again.
const SkinSelectionSecondAwakeningDefault = 100000

// IsSkinSelectionFamilyBuiltin reports whether an id is one of the built-in rows of a
// list category's own family — the ids a 解除 click names rather than a skin the
// account bought. The command echo's case 0 / case 1 store them like any other id,
// and the panels render them as the default, so the server accepts and persists them
// without an ownership check. Damage-font categories answer false; they have their
// own per-tab sentinel in SkinSelectionResetID.
func IsSkinSelectionFamilyBuiltin(category, id uint32) bool {
	var builtins []uint32
	switch category {
	case SkinCategoryPartyFrame:
		builtins = append(append([]uint32{}, SkinCargoPartyFrameBuiltins...), SkinCargoPartyFrameDefaultList)
	case SkinCategorySkillCutscene:
		builtins = SkinCargoSkillCutsceneBuiltins
	default:
		return false
	}
	for _, b := range builtins {
		if b == id {
			return true
		}
	}
	return false
}

// SkinCargoPage encodes NOTI1545 for one owned page:
// `u8 page, u16 count, (u32 skin_id, u32 expires) x count, u16 0`. The trailing
// count is the reader's second list, whose ids it remaps through the client's
// type table; nothing in this server owns that list yet, so it stays empty.
//
// Every entry is written with expires=0. sub_1444EFF40 sets the entry's expiry
// flag, and sub_1444EC2C0 treats "flag set, expiry 0" as never lapsing, so a
// permanent grant needs no clock domain. A nonzero expiry would be compared
// against sub_145A11A50, whose encoding is not yet confirmed, so timed skins are
// deliberately not produced.
func SkinCargoPage(page byte, ids []uint32) ([]byte, error) {
	// The reader switches on the page: page 4 is the only one whose first list
	// holds bare u32 ids, so this builder cannot describe it.
	if page == 4 {
		return nil, fmt.Errorf("skin cargo page 4 has no expiry column")
	}
	if len(ids) > 65535 {
		return nil, fmt.Errorf("too many skin cargo entries")
	}
	p := []byte{page}
	p = add16(p, uint16(len(ids)))
	for _, id := range ids {
		p = add32(p, id)
		p = add32(p, SkinCargoPermanentExpiry)
	}
	return add16(p, 0), nil
}

// SkinCargoPermanentExpiry is the expiry cell of a grant that never lapses.
const SkinCargoPermanentExpiry = 0

// The damage-font panel has two tabs, 普通伤害 and 累计伤害. Both enumerate the
// same owned page (2), and both single-id selection cases validate against it,
// but each tab keeps its own selection: sub_1444EECA0 case 2 applies through
// sub_1447EF2F0 (holder+112, plus UI event 178) and case 6 through sub_142581F20
// (holder+232). Each case also assigns mgr+296[category], and the panel's pool
// filler marks a row 生效中 by asking sub_1444EC5B0(mgr, category, id) whether the
// id is in that vector — so answering with the wrong category lights the other
// tab. The live 2026-09-27 capture is what pins this down: the same skin id 18
// arrives once as `2, 0, 18` and once as `6, 0, 18`, so the first u32 names the
// tab's category, not a cargo page.
const (
	SkinSelectionDamageFontNormal     = 2
	SkinSelectionDamageFontCumulative = 6
	// SkinSelectionDefaultFont is the client's built-in font for the cumulative tab.
	// sub_1444EECA0 case 6 writes it itself when an id is not owned, and that tab's
	// 解除 button requests it by name.
	SkinSelectionDefaultFont = 99999999
	// SkinSelectionNormalDamageDefaultFont is the normal tab's own default: the value
	// the font holder's constructor puts in holder+112 before anything is applied
	// (df32_holder_ctor_sub_1447E41D0.c:186, `*(_DWORD *)(a1 + 112) = 1`), and the id
	// that tab's 解除 button sends. Live 2026-09-27 05:28: every 普通伤害 解除 arrived as
	// category 2 with id 1, never with the sentinel, so answering only the sentinel
	// left that tab stuck.
	SkinSelectionNormalDamageDefaultFont = 1
)

// SkinSelectionResetID returns the id one tab's 解除 button asks for, which is also
// the id the answer has to carry: each tab names its own default rather than one
// shared sentinel. Categories with no reversed meaning return false.
func SkinSelectionResetID(category uint32) (uint32, bool) {
	switch category {
	case SkinSelectionDamageFontNormal:
		return SkinSelectionNormalDamageDefaultFont, true
	case SkinSelectionDamageFontCumulative:
		return SkinSelectionDefaultFont, true
	}
	return 0, false
}

// IsSkinSelectionDamageFontCategory reports whether a selection category belongs
// to the damage-font panel.
func IsSkinSelectionDamageFontCategory(category uint32) bool {
	return category == SkinSelectionDamageFontNormal || category == SkinSelectionDamageFontCumulative
}

// SkinSelectionDamageFont encodes NOTI1546 `u8 category, u32 skin_id`.
func SkinSelectionDamageFont(category, id uint32) ([]byte, error) {
	if !IsSkinSelectionDamageFontCategory(category) {
		return nil, fmt.Errorf("damage font selection category %d is not reversed", category)
	}
	return add32([]byte{byte(category)}, id), nil
}

// SkinCategoryPartyFrame and SkinCategorySkillCutscene are the selection
// categories of the 边框 and 觉醒插图 panels. They equal the owned page numbers above.
// SkinCategoryInstantEmoticon, SkinCategorySpray and SkinCategoryAirshipEffect are the
// 表情, 涂鸦 and 飞空艇特效 categories, and they too equal their page number.
const (
	SkinCategoryPartyFrame      = SkinCargoPartyFramePage
	SkinCategorySkillCutscene   = SkinCargoSkillCutscenePage
	SkinCategoryInstantEmoticon = SkinCargoInstantEmoticonPage
	SkinCategorySpray           = SkinCargoSprayPage
	SkinCategoryAirshipEffect   = SkinCargoAirshipEffectPage
)

// SkinSelectionPartyFrame encodes NOTI1546 category 0:
// `u8 0, u32 single[3], u16 count, u32 ids[count]`.
//
// sub_1444EECA0 case 0 validates each of the three single ids against owned page 0
// and appends the survivors **in read order to one shared vector**, which then
// replaces mgr+296[0] whole; the trailing list instead refills the acquired set at
// mgr+312 from scratch, and an empty list inserts 80000
// (`Skin/PartyListFrame/Default.skn`). So the three singles are interchangeable as
// to position — each panel consumer scans the vector for the registry subtype it
// renders (df26_ebc10caller_sub_1458D0540.c:75-81 keeps only `rec+12 == 1`) — while
// the raid party list frames must be in the list, which is what
// catalog.SkinStorageEntry.IsRaidPartyListFrame decides.
func SkinSelectionPartyFrame(singles, raidList []uint32) ([]byte, error) {
	if len(singles) > 3 {
		return nil, fmt.Errorf("party frame selection has %d single slots, want at most 3", len(singles))
	}
	if len(raidList) > 65535 {
		return nil, fmt.Errorf("too many raid party list frames")
	}
	p := make([]byte, 1, 1+12+2+4*len(raidList))
	p[0] = SkinCategoryPartyFrame
	for i := 0; i < 3; i++ {
		var id uint32
		if i < len(singles) {
			id = singles[i]
		}
		p = add32(p, id)
	}
	p = add16(p, uint16(len(raidList)))
	for _, id := range raidList {
		p = add32(p, id)
	}
	return p, nil
}

// SkinSelectionSkillCutscene encodes NOTI1546 category 1:
// `u8 1, u16 count, u32 ids[count], u16 count2, u32 ids2[count2]`.
//
// The first list replaces mgr+296[1] whole after each id passes the page-1 ownership
// and expiry check (the reader also runs it through the per-job table at mgr+1328,
// which no code in this build ever inserts into, so it is the identity). That vector
// is the 一觉 render pool: sub_1444EA8A0 sets mgr+1120 to pool[RNG % size], falling
// back to 30000 when it is empty.
//
// The second list is the 二觉 render pool. sub_1444EECA0 assigns it to BOTH mgr+1152
// and mgr+1176 with no ownership check, only keeping ids > 0
// (analysis/dumps/skin-noti/df13_noti1546_body_1444eeca0.c:368-371), and mgr+1152's
// sole reader is sub_1444EBAD0 - called live by sub_145D451D0 every time a
// second-awakening cutscene plays, with the same pool[RNG % size] draw and 100000 as
// the empty fallback (df37_fn_ebad0_0x1444ebad0.c, df37.log section 1). An empty
// second list therefore does not mean "no 二觉 skin selected", it means the client can
// never leave the hardcoded default, which is why 二次觉醒 插图 used to do nothing in
// the dungeon.
func SkinSelectionSkillCutscene(awakening, secondAwakening []uint32) ([]byte, error) {
	if len(awakening) > 65535 || len(secondAwakening) > 65535 {
		return nil, fmt.Errorf("too many selected skill cutscenes")
	}
	p := []byte{SkinCategorySkillCutscene}
	p = add16(p, uint16(len(awakening)))
	for _, id := range awakening {
		p = add32(p, id)
	}
	p = add16(p, uint16(len(secondAwakening)))
	for _, id := range secondAwakening {
		p = add32(p, id)
	}
	return p, nil
}

// SkinSelectionInstantEmoticonSlots is the number of id words NOTI1546's category-3
// reader consumes: sub_1444EECA0 case 3 runs a fixed four-iteration loop
// (`v40 = 4; while (1) { read u32; ... if (!--v40) goto LABEL_155; }`,
// analysis/dumps/skin-noti/df13_noti1546_body_1444eeca0.c:477-553) and there is no count
// word anywhere in the body, so a short frame would make it read past the payload.
//
// The four words are **positional**: the tab is a four-cell quick bar, its sender
// sub_1441E03D0 fills the vector from four cells at panel+4176 with stride 120, and the
// one consumer sub_1444EC930 hands the whole vector to the chat channel unchanged
// (sub_14668C520(chat, 34, vector, 0), analysis/dumps/skin-noti/
// df41_reader_ebc10_sub_1444EC930_1444ec930.c:95-104). An empty cell is sent as a zero
// word and the reader appends that zero, so the position of every survivor is kept; only
// an id the owned page does not hold is dropped, which would shift the cells after it.
// That is why this takes the full four-slot array instead of a trimmed list.
const SkinSelectionInstantEmoticonSlots = 4

// SkinSelectionInstantEmoticon encodes NOTI1546 category 3: `u8 3, u32 slots[4]`.
func SkinSelectionInstantEmoticon(slots []uint32) ([]byte, error) {
	if len(slots) != SkinSelectionInstantEmoticonSlots {
		return nil, fmt.Errorf("emoticon selection has %d slots, want %d",
			len(slots), SkinSelectionInstantEmoticonSlots)
	}
	p := []byte{SkinCategoryInstantEmoticon}
	for _, id := range slots {
		p = add32(p, id)
	}
	return p, nil
}

// SkinSelectionSingle encodes NOTI1546 for the two single-value families, 涂鸦 (7) and
// 飞空艇特效 (8): `u8 category, u32 skin_id`.
//
// sub_1444EECA0's case 7 and case 8 share one tail (`v55 = 7|8; LABEL_151:`), which
// validates the id against that page's owned map through sub_1444EBD90 plus the expiry
// test sub_1444EC2C0 and then appends it as a one-element vector
// (df13_noti1546_body_1444eeca0.c:610-623). An id the page does not hold jumps to
// LABEL_208, which frees the half-built vector and assigns **nothing** — so this frame can
// select but can never clear. Clearing is the CMD1565 echo's job: its own reader's case 7 /
// case 8 stores `v43[2]` verbatim with no ownership check at all
// (analysis/dumps/skin-noti/df40_core_sub_1444EE820.c:166-172), so the same word with id 0
// leaves that category's vector holding the zero, which is what no 生效中 row means.
func SkinSelectionSingle(category, id uint32) ([]byte, error) {
	if category != SkinCategorySpray && category != SkinCategoryAirshipEffect {
		return nil, fmt.Errorf("category %d is not a single-value skin family", category)
	}
	return add32([]byte{byte(category)}, id), nil
}

// The CMD1565 result word is the command kind, not a status code: the client's four
// composers each write their own value into that slot of the same 88-byte body.
//
//   - 0  sub_1444F1090 (apply / page flush) — its `memset(&v24[1], 0, 84)` leaves 0, and it
//     backfills a per-category empty selection (0→20000, 1→30000, 2→1, 3→four zero words,
//     4→one zero word, 6→99999999, 9→four zero words, 7 and 8→ nothing at all);
//     sub_1444F1310 also writes 0 or 1 for its category-9 flush.
//   - 2 / 3  sub_1444F0FE0(mgr, page, skin, currently_starred) writes `(starred != 0) + 2`,
//     so 2 is "this row is not starred and the player just turned it on" and 3 is the
//     removal. The same handler's cap test refuses an add whose group already holds
//     SkinFavoriteCapPerGroup entries (analysis/dumps/skin-noti/
//     df41_fav_flush_DD510_1441dd510.c:116, message 101037008).
//   - 4  sub_1444F1880(mgr, category, id), the acquired-set erase.
const (
	SkinSelectResultApply          = 0
	SkinSelectResultFavoriteAdd    = 2
	SkinSelectResultFavoriteRemove = 3
	SkinSelectResultAcquiredErase  = 4
)

// SkinFavoritePages and SkinFavoriteCrossGroups are the group counts NOTI2641's reader
// walks: `do { read count; read count ids } while (++page < 10)` and then
// `for (j = 0; j < 4; ++j) { same }` (analysis/dumps/skin-noti/
// df41_handler_NOTI_2641_1444ed1b0.c, restored as df39_fav_reader_ED1B0.c). The ten
// groups are inserted as {page, -1, id, group count} and the four trailing ones as
// {-1, j, id, group count}, and sub_1444EB840(mgr, page, slot) answers whichever comes
// first in the list — the per-page group by its first key, a trailing group by its second.
//
// What those four cross-page groups count is not named by any evidence in this workspace,
// so they are sent empty: the reader clears the whole list before it parses, so an empty
// group simply contributes no node, and the only consumer that could ever ask for one
// (the star handler's cap test) then sees 0 instead of a wrong number.
const (
	SkinFavoritePages       = 10
	SkinFavoriteCrossGroups = 4
	// SkinFavoriteCapPerGroup is the client's own limit: the star handler stops adding a
	// page's favourites once sub_1444EB840 reports ten, and tells the player so.
	SkinFavoriteCapPerGroup = 10
)

// SkinFavorites encodes NOTI2641, the only frame that fills the client's favourite table:
// `for page 0..9: u32 count, u32 ids[count]` then `for j 0..3: u32 0`.
//
// This push is not optional and the command echo is not a substitute: the CMD1565 reply
// core runs its whole category switch only `if ( !v43[1] )` — i.e. only for result 0 — so a
// star click is never applied client-side by its own echo
// (analysis/dumps/skin-noti/df40_core_sub_1444EE820.c:57), and the table's single writer is
// this handler (sub_1444E8DF0, called only from sub_1444ED1B0, df42.log section 5). The body
// is absolute state: the reader frees every node of the list before the first group.
func SkinFavorites(pages [][]uint32) ([]byte, error) {
	if len(pages) != SkinFavoritePages {
		return nil, fmt.Errorf("favourite push has %d page groups, want %d", len(pages), SkinFavoritePages)
	}
	p := make([]byte, 0, 4*(SkinFavoritePages+SkinFavoriteCrossGroups)+4*SkinFavoriteCapPerGroup)
	for _, ids := range pages {
		p = add32(p, uint32(len(ids)))
		for _, id := range ids {
			p = add32(p, id)
		}
	}
	for j := 0; j < SkinFavoriteCrossGroups; j++ {
		p = add32(p, 0)
	}
	return p, nil
}

// SelectSkinBodySize is the fixed CMD1565 body. The client's own receive path for
// the same opcode, sub_1444EE820, bulk-reads exactly this many bytes through
// sub_146EA0BE0 and lays them out as `u32 category, u32 result, u32 skin_ids[20]`.
const SelectSkinBodySize = 88

// SelectSkinIDSlots is the width of that id array: (88-8)/4. sub_1444EE820 walks all
// of them for the list categories, skipping the zero ones, so a smaller selection is
// sent zero-padded rather than length-prefixed.
const SelectSkinIDSlots = 20

// SelectSkinRequest is one CMD1565 应用 or 解除 click. Live 2026-09-27 captured
// `2, 0, 12` and `6, 0, 18` from the panel's two tabs and `6, 0, 99999999` from
// 解除; sub_1444EE820 switches on the first u32 with the same case numbers
// NOTI1546 uses, so it is the selection category.
//
// SkinIDs holds the request's id slots in order with the trailing zeros trimmed:
// the two list categories (0 边框, 1 觉醒插图) fill more than one slot, and the
// client's own receive path iterates all 20 slots appending every nonzero one, so
// the whole list is the client's intended selection. SkinID stays the first slot
// for the single-id categories.
type SelectSkinRequest struct {
	Category uint32
	Result   uint32
	SkinID   uint32
	SkinIDs  []uint32

	// Awakening and SecondAwakening are the 觉醒插图 body's two lists, read by
	// position rather than merged: sub_1444F1410 lays mgr+1176 (the panel's 二觉 rows)
	// into slots 0..9 and mgr+1128 (its 一觉 rows) into slots 10..19 before handing the
	// vector to the composer (analysis/dumps/skin-noti/df36_fn_f1410_0x1444f1410.c:103-131),
	// and each list is capped at ten pushes by sub_1444E9240 / sub_1444E9290. Only
	// category 1 fills these two fields.
	Awakening       []uint32
	SecondAwakening []uint32

	// EmoticonSlots is the 表情 body read **by position**, all four words with the zeros
	// kept, because the four cells are the client's four quick-bar slots and its consumer
	// forwards the vector unchanged. Only category 3 fills it.
	EmoticonSlots []uint32

	// Favorite is one star toggle rather than an apply. The result word carries the
	// direction: sub_1444F0FE0(mgr, page, skin, currently_starred) writes
	// `(currently_starred != 0) + 2`, so 2 is the player turning a row on and 3 turning it
	// off, and the body's id vector is exactly that one skin
	// (analysis/dumps/skin-noti/df41_composer_F0FE0_ref.c:18-29). The client applies nothing
	// itself — the CMD1565 reply core only runs its switch for result 0 — so the server owns
	// the answer for this kind.
	Favorite bool
}

// DecodeSelectSkin reads the fixed CMD1565 body. The damage-font categories are
// still single-id: their reader (case 2, case 6) only ever looks at slot 0, so a
// request that fills another slot is refused instead of being reinterpreted, which
// keeps the round-7 behaviour byte-identical.
func DecodeSelectSkin(p []byte) (SelectSkinRequest, error) {
	if len(p) != SelectSkinBodySize {
		return SelectSkinRequest{}, fmt.Errorf("select skin body is %d bytes, want %d", len(p), SelectSkinBodySize)
	}
	category := binary.LittleEndian.Uint32(p)
	if IsSkinSelectionDamageFontCategory(category) {
		for _, b := range p[12:] {
			if b != 0 {
				return SelectSkinRequest{}, fmt.Errorf("select skin body has unsupported trailing fields")
			}
		}
	}
	ids := make([]uint32, 0, SelectSkinIDSlots)
	for i := 0; i < SelectSkinIDSlots; i++ {
		if id := binary.LittleEndian.Uint32(p[8+4*i:]); id != 0 {
			ids = append(ids, id)
		}
	}
	var first uint32
	if len(ids) > 0 {
		first = ids[0]
	}
	req := SelectSkinRequest{
		Category: category,
		Result:   binary.LittleEndian.Uint32(p[4:]),
		SkinID:   first,
		SkinIDs:  ids,
	}
	if category == SkinCategorySkillCutscene {
		for i := 10; i < SelectSkinIDSlots; i++ {
			if id := binary.LittleEndian.Uint32(p[8+4*i:]); id != 0 {
				req.Awakening = append(req.Awakening, id)
			}
		}
		for i := 0; i < 10; i++ {
			if id := binary.LittleEndian.Uint32(p[8+4*i:]); id != 0 {
				req.SecondAwakening = append(req.SecondAwakening, id)
			}
		}
	}
	if category == SkinCategoryInstantEmoticon {
		req.EmoticonSlots = make([]uint32, SkinSelectionInstantEmoticonSlots)
		for i := range req.EmoticonSlots {
			req.EmoticonSlots[i] = binary.LittleEndian.Uint32(p[8+4*i:])
		}
	}
	req.Favorite = req.Result == SkinSelectResultFavoriteAdd ||
		req.Result == SkinSelectResultFavoriteRemove
	return req, nil
}

// SelectSkinEcho rebuilds that 88-byte body as the answer the client waits for,
// behind the one status byte every command reply carries. sub_1444EE820 reads it
// and switches on the category exactly like NOTI1546 does, but its case 2 calls
// sub_1447EF2F0(holder, ids[0]) with **no ownership check**
// (df25_caller_sub_1444EE820_0x1444ee820.c:102-107), so it is the only proven
// frame that rewrites holder+112 — which is what the normal-damage tab renders.
// NOTI1546's case 2 cannot unequip because its unowned branch jumps to
// LABEL_208, which only empties the selection vector.
//
// The status byte and the frame kind are not decoration (df33/df33b/df33c): the
// receive router sub_1459A1BB0 branches on the envelope's first byte — 0 goes to
// the notification bus sub_1456B8AB0, only 1 goes to sub_14599D200, which
// consumes this status byte and dispatches through the **CMD registry**
// qword_14E6836F8 via sub_1459A2D70(table, id, status, code). sub_1444E8D00 —
// the only handler of 1565 in that registry — is then called as
// (obj, status, code) and short-circuits into the failure toast sub_1444EC790
// when status is 0. A kind-0 echo lands on the notification registry's own 1565
// entry, sub_14399DD70, which belongs to an unrelated subsystem.
//
// The echo then runs the tab's own refresh (LABEL_58 → sub_1444F1EE0 →
// sub_1441ECDB0 → sub_1441E0820 case 2), which rebuilds the tab's font object
// from the id it just stored. That refresh tail is not a sender, and the client
// already runs it for every category-2 NOTI1546 this server pushes, so the echo
// adds no new round-trip.
func SelectSkinEcho(category, id uint32) ([]byte, error) {
	if !IsSkinSelectionDamageFontCategory(category) {
		return nil, fmt.Errorf("skin selection category %d has no reversed echo", category)
	}
	p := make([]byte, 1+SelectSkinBodySize)
	p[0] = 1
	binary.LittleEndian.PutUint32(p[1:], category)
	binary.LittleEndian.PutUint32(p[9:], id)
	return p, nil
}

// SelectSkinEchoRaw is the same reply for a request the server does not
// reinterpret: the client's own 88-byte body with the success status byte in front.
// This is the only echo shape the list categories can use, because their reader
// derives everything else from that body — sub_1444EE820 case 0 walks all 20 slots
// and routes a slot whose registry subtype is 2 into the acquired set instead of the
// vector, and its `result == 4 && category == 0` branch erases an id from that set.
// Rebuilding the body from a filtered id list would lose both of those intents, so
// the request goes back exactly as it arrived and the server's own NOTI1546 follows
// it (the echo writes the vector first, then the notification replaces it with the
// validated selection, so the two frames cannot leave the panel half-applied).
func SelectSkinEchoRaw(body []byte) ([]byte, error) {
	if len(body) != SelectSkinBodySize {
		return nil, fmt.Errorf("select skin echo body is %d bytes, want %d", len(body), SelectSkinBodySize)
	}
	return append([]byte{1}, body...), nil
}
