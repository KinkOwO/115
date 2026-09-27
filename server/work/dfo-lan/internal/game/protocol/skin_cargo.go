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

// SelectSkinBodySize is the fixed CMD1565 body. The client's own receive path for
// the same opcode, sub_1444EE820, bulk-reads exactly this many bytes through
// sub_146EA0BE0 and lays them out as `u32 category, u32 result, u32 skin_ids[20]`.
const SelectSkinBodySize = 88

// SelectSkinRequest is one CMD1565 应用 or 解除 click. Live 2026-09-27 captured
// `2, 0, 12` and `6, 0, 18` from the panel's two tabs and `6, 0, 99999999` from
// 解除; sub_1444EE820 switches on the first u32 with the same case numbers
// NOTI1546 uses, so it is the selection category.
type SelectSkinRequest struct {
	Category uint32
	Result   uint32
	SkinID   uint32
}

// DecodeSelectSkin reads the fixed CMD1565 body. Only the first id slot is
// accepted: the categories whose reader consumes a list (case 1 and case 3) have
// no server-side meaning yet, so a request that fills any other byte is refused
// instead of being reinterpreted.
func DecodeSelectSkin(p []byte) (SelectSkinRequest, error) {
	if len(p) != SelectSkinBodySize {
		return SelectSkinRequest{}, fmt.Errorf("select skin body is %d bytes, want %d", len(p), SelectSkinBodySize)
	}
	for _, b := range p[12:] {
		if b != 0 {
			return SelectSkinRequest{}, fmt.Errorf("select skin body has unsupported trailing fields")
		}
	}
	return SelectSkinRequest{
		Category: binary.LittleEndian.Uint32(p),
		Result:   binary.LittleEndian.Uint32(p[4:]),
		SkinID:   binary.LittleEndian.Uint32(p[8:]),
	}, nil
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
