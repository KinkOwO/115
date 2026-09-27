package protocol

import (
	"encoding/binary"
	"fmt"
)

// MakeSkinRequest is the body of CMD1592 (ENUM_CMDPACKET_MAKE_SKIN), the
// weapon-replication confirmation the blacksmith window sends.
//
// The sender at 0x14407dca0 writes the fields one at a time and the layout is
// fixed by that sequence, not guessed:
//
//	WriteOpcode(0x638)                    -> CMD 1592
//	u8  4                                 -> hard-coded literal
//	u32 1 or 2                            -> lea edx,[rbx+2] with ebx = -1 when
//	                                         [window+0x758] is non-zero, so 1 for
//	                                         a set flag and 2 for a clear one
//	u8  0                                 -> hard-coded literal
//	u16 [window+0x5ec]                    -> the selected entry index
//
// Live captures of two confirmations both produced 04 01000000 00 0900: op 4,
// mode 1, flag 0, index 9.
type MakeSkinRequest struct {
	Op    byte
	Mode  uint32
	Flag  byte
	Index uint16
}

// DecodeMakeSkin reads the eight bytes the sender flushes. The body carries no
// template at all, so the server has to resolve the item from Index itself.
func DecodeMakeSkin(p []byte) (MakeSkinRequest, error) {
	var r MakeSkinRequest
	if len(p) < 8 {
		return r, fmt.Errorf("make skin request is shorter than 8 bytes")
	}
	r.Op = p[0]
	r.Mode = binary.LittleEndian.Uint32(p[1:])
	r.Flag = p[5]
	r.Index = binary.LittleEndian.Uint16(p[6:])
	return r, nil
}

// SkinCargoEntry is one row of NOTI1545's first list. The reader 0x1444eff40
// takes edx = dword@0 as the skin id and skips the row when that is zero; the
// second dword becomes the node's extra field. Subtype 4 is the exception: the
// reader consumes only the four id bytes there and leaves the extra field zero.
type SkinCargoEntry struct {
	SkinID uint32
	Extra  uint32
}

// SkinCargoInfo builds NOTI1545 (ENUM_NOTIPACKET_SKIN_CARGO_INFO), the push that
// fills one skin cargo container.
//
// Layout read back by 0x1444eff40, in order:
//
//	u8   subtype (0..9; anything above returns immediately)
//	u16  first-list count, then that many skin entries
//	u16  second-list count, then that many 8-byte rows (subtype, value)
//
// Entries with skin id zero are dropped by the client, so this refuses to build
// one rather than silently sending a shorter list than the count promises.
func SkinCargoInfo(subtype byte, entries []SkinCargoEntry, rows [][2]uint32) ([]byte, error) {
	if subtype > 9 {
		return nil, fmt.Errorf("skin cargo subtype %d is out of range", subtype)
	}
	p := []byte{subtype}
	p = add16(p, uint16(len(entries)))
	for _, e := range entries {
		if e.SkinID == 0 {
			return nil, fmt.Errorf("skin cargo entry has no skin id")
		}
		p = add32(p, e.SkinID)
		// Subtype 4 reads four bytes, every other subtype reads eight; writing
		// the extra word where the client does not expect it desynchronises the
		// rest of the body.
		if subtype != 4 {
			p = add32(p, e.Extra)
		}
	}
	p = add16(p, uint16(len(rows)))
	for _, r := range rows {
		p = add32(p, r[0])
		p = add32(p, r[1])
	}
	return p, nil
}

// SkinCargoSelectionInfo builds NOTI1546 (ENUM_NOTIPACKET_SELECT_SKIN_LIST), the
// push that tells one skin page which entry is currently worn.
//
// The window highlights the worn row from its own copy of that selection, and
// closing the storage throws that copy away: the Apply handler writes only the
// window-local detail cell (0x1441d7b70 -> SetDetail(win, 4, rowId, 0)) and never
// reports the choice back to the manager, so reopening the window rebuilt the
// highlight from an empty table and the worn row lost its frame (live 2026-09-27).
//
// Wire layout read by the dispatcher 0x1444ed4f0 and its subtype-4 core
// 0x1444eeca0: a u8 subtype, then the ids for that page. The weapon-shape page is
// subtype 4 - the one Apply hard-codes - and that core consumes exactly one u32 and
// no length prefix, so the body is these five bytes. Other pages lay their payload
// out differently (alternating single-word and length-prefixed forms), and nothing
// here needs them, so this does not pretend to build them. The reader guards every
// read against a short body (0x146ea0340 charges the length and a truncated read
// yields zero), and an id the client cannot resolve makes the core skip the write
// entirely, so a bad id is a no-op rather than a crash.
//
// That skip is what makes a zero id mean "nothing is worn": the reader prunes the
// page's selection before it reads the id (0x1444eecd8 finds the manager's map node
// for this subtype and does node[+0x30] = node[+0x28], vector::clear()), and only
// the write-back path refills it. So a zero id leaves the page empty instead of
// keeping the last id, which is how an unapply clears the worn row.
func SkinCargoSelectionInfo(skin uint32) []byte {
	return add32([]byte{byte(SkinCargoWeaponShape)}, skin)
}

// SkinCargoSyncRequest is the body of CMD1565, the one command the skin storage
// window sends both when a tab is opened and when the player presses Apply.
//
// The sender 0x1444f1090 always emits the same 88 bytes:
//
//	u32 subtype   0..9 - which skin cargo container the window has open
//	u32 reserved  always 0 from this sender; the variant 0x1444f0fe0 writes 2
//	              or 3 and is reached from another window path
//	u32 ids[20]   the container's skin ids, unused cells zero
//
// Live capture 2026-09-26 12:33 (weapon shape tab, seconds after the
// replication confirmation): 04 000000 00000000 e04d0506 00*19 - subtype 4,
// reserved 0, ids[0] = 101010912, the replicated weapon's own template.
//
// The client keeps one container per subtype (manager +0x88 + 16*subtype), and
// its Apply handler hard-codes the weapon-shape subtype 4, so a subtype-4 sync
// is the client saying "this is the weapon skin I am showing/wearing". Because
// Apply and the tab refresh share this frame, the body alone cannot separate
// them; the server treats the reported id as the applied weapon skin, which is
// what makes the world model follow.
type SkinCargoSyncRequest struct {
	Subtype  uint32
	Reserved uint32
	IDs      []uint32
}

// SkinCargoSyncIDCount is how many id cells the frame carries. It is fixed by
// the sender's 88-byte write, not by the container size; the tail is padding.
const SkinCargoSyncIDCount = 20

// SkinCargoWeaponShape is the subtype the weapon-replication tab uses. The
// Apply handler at 0x1441d7b70 loads edx = 4 before calling 0x1444f1090, and
// live captures show the replicated katana in the tab that reports subtype 4.
const SkinCargoWeaponShape uint32 = 4

// DecodeSkinCargoSync reads the fixed 88-byte frame. Zero cells are padding and
// are dropped, so IDs carries only the ids the client actually reported.
func DecodeSkinCargoSync(p []byte) (SkinCargoSyncRequest, error) {
	const size = 8 + 4*SkinCargoSyncIDCount
	if len(p) < size {
		return SkinCargoSyncRequest{}, fmt.Errorf("skin cargo sync is shorter than %d bytes", size)
	}
	r := SkinCargoSyncRequest{
		Subtype:  binary.LittleEndian.Uint32(p),
		Reserved: binary.LittleEndian.Uint32(p[4:]),
	}
	r.IDs = make([]uint32, 0, SkinCargoSyncIDCount)
	for i := 0; i < SkinCargoSyncIDCount; i++ {
		if id := binary.LittleEndian.Uint32(p[8+4*i:]); id != 0 {
			r.IDs = append(r.IDs, id)
		}
	}
	return r, nil
}

// RecentAddSkinEntry is one row of NOTI1547 (ENUM_NOTIPACKET_RECENT_ADD_SKIN_LIST).
// The reader 0x1444ed400 takes a u8 first and a u32 second, then stores the pair
// into the manager vector at +0x448 as {u32 second, u32 first}, so the wire order
// is the reverse of the memory order.
type RecentAddSkinEntry struct {
	Kind   byte
	SkinID uint32
}

// RecentAddSkinList builds NOTI1547: a u8 count followed by count {u8, u32}
// pairs. The reader clears the vector before the loop, so this body is the whole
// "recently added" list rather than an append.
func RecentAddSkinList(entries []RecentAddSkinEntry) ([]byte, error) {
	if len(entries) > 255 {
		return nil, fmt.Errorf("recent skin list is longer than the count byte")
	}
	p := []byte{byte(len(entries))}
	for _, e := range entries {
		if e.SkinID == 0 {
			return nil, fmt.Errorf("recent skin entry has no skin id")
		}
		p = append(p, e.Kind)
		p = add32(p, e.SkinID)
	}
	return p, nil
}
