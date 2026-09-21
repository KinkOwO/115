package protocol

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"sort"
)

// CMD2377 SET_UNIFIED_OPTION carries one option block per frame. The layout
// below is verified against every CMD2377 frame this client produced while
// playing (see testdata/native_unified_option_frames36.json for the raw
// plaintexts):
//
//	+00 u32 stamp            (varies per frame)
//	+04 u32                  (0, 1 and 0xFFFFFFFF all observed)
//	+08 5B FE FF FF FF FF    (marker)
//	+13 u8  scope
//	+14 u8  subtype          (0x13 skill lock, 0x05 system settings)
//	+15 u8  entry count
//	+16 3B  zero
//	+19      count x (u16 position, u16 value)
//	tail    0..5 bytes of zero padding
//
// A forwarded repair note claims +04 is always 01000000; the captures falsify
// that, so +04 is never validated here.
const (
	unifiedOptionLen      = 19
	unifiedOptionMarkerAt = 8
	unifiedOptionMaxTail  = 8

	// UnifiedOptionSkillLock is the subtype that carries the skill lock block.
	UnifiedOptionSkillLock = 0x13
	// UnifiedOptionSettings is the subtype of the ordinary settings block,
	// which shares the opcode but has its own index/value semantics.
	UnifiedOptionSettings = 0x05
	// UnifiedOptionAccount is the subtype of the account-level option block,
	// restored through NOTI2826.
	UnifiedOptionAccount = 0x01
)

var unifiedOptionMarker = []byte{0xFE, 0xFF, 0xFF, 0xFF, 0xFF}

type UnifiedOptionEntry struct {
	Position uint16
	Value    uint16
}

type UnifiedOption struct {
	Scope   byte
	Subtype byte
	Entries []UnifiedOptionEntry
	// Tail counts the zero padding after the last entry.
	Tail int
}

func DecodeUnifiedOption(p []byte) (UnifiedOption, error) {
	var r UnifiedOption
	if len(p) < unifiedOptionLen {
		return r, fmt.Errorf("short unified option frame")
	}
	if !bytes.Equal(p[unifiedOptionMarkerAt:unifiedOptionMarkerAt+len(unifiedOptionMarker)], unifiedOptionMarker) {
		return r, fmt.Errorf("unified option marker mismatch")
	}
	r.Scope, r.Subtype = p[13], p[14]
	count := int(p[15])
	end := unifiedOptionLen + count*4
	if len(p) < end || len(p) > end+unifiedOptionMaxTail {
		return r, fmt.Errorf("unsupported unified option entry count")
	}
	for _, b := range p[end:] {
		if b != 0 {
			return r, fmt.Errorf("unsupported unified option padding")
		}
	}
	r.Tail = len(p) - end
	r.Entries = make([]UnifiedOptionEntry, 0, count)
	for i := 0; i < count; i++ {
		off := unifiedOptionLen + i*4
		r.Entries = append(r.Entries, UnifiedOptionEntry{
			Position: binary.LittleEndian.Uint16(p[off:]),
			Value:    binary.LittleEndian.Uint16(p[off+2:]),
		})
	}
	return r, nil
}

// Locked skills travel as two compact, ascending pages of the character option
// table: page 0 holds ids below 512 as-is, page 1 holds ids 512..1023 reduced
// by UnifiedSkillPageStride. A block therefore covers at most
// UnifiedSkillSlots distinct skills.
const (
	UnifiedSkillSlots      = 128
	UnifiedSkillPageSize   = 64
	UnifiedSkillPageStride = 512
	unifiedSkillEmpty      = 0xFFFF
)

// CompactSkillSlots rebuilds the 128 slot block from a skill id set. The client
// only ever sends compact ascending pages, so the server never stores slot
// positions and re-derives them the same way on every frame.
func CompactSkillSlots(ids []uint16) []uint16 {
	slots := make([]uint16, UnifiedSkillSlots)
	for i := range slots {
		slots[i] = unifiedSkillEmpty
	}
	sorted := append([]uint16(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	page0, page1 := 0, 0
	for _, id := range sorted {
		switch {
		case id < UnifiedSkillPageStride && page0 < UnifiedSkillPageSize:
			slots[page0] = id
			page0++
		case id >= UnifiedSkillPageStride && id < 2*UnifiedSkillPageStride && page1 < UnifiedSkillPageSize:
			slots[UnifiedSkillPageSize+page1] = id - UnifiedSkillPageStride
			page1++
		}
	}
	return slots
}

// SkillIDsFromSlots is the inverse of CompactSkillSlots. A zero value is the
// client's delete signal and never becomes a skill.
func SkillIDsFromSlots(slots []uint16) []uint16 {
	out := make([]uint16, 0, len(slots))
	for pos, v := range slots {
		if v == 0 || v == unifiedSkillEmpty {
			continue
		}
		id := v
		if pos >= UnifiedSkillPageSize {
			id += UnifiedSkillPageStride
		}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// MergeSkillLocks applies one incremental frame onto the stored set.
//
// Two properties of the client's frames drive this function: the frame lists
// only the slots that differ from the last baseline, and a slot value of zero
// means "clear this slot" rather than "no change" (dropping those entries is
// what makes an unlock appear to fail). A frame whose first position sits on a
// page boundary rebuilds that whole page first.
func MergeSkillLocks(current []uint16, entries []UnifiedOptionEntry) []uint16 {
	slots := CompactSkillSlots(current)
	if len(entries) > 0 {
		switch entries[0].Position {
		case 0:
			for i := 0; i < UnifiedSkillPageSize; i++ {
				slots[i] = unifiedSkillEmpty
			}
		case UnifiedSkillPageSize:
			for i := UnifiedSkillPageSize; i < UnifiedSkillSlots; i++ {
				slots[i] = unifiedSkillEmpty
			}
		}
	}
	for _, e := range entries {
		if int(e.Position) >= UnifiedSkillSlots {
			// The client never addresses past the block; ignore rather than
			// corrupt a neighbouring slot.
			continue
		}
		if e.Value == 0 {
			slots[e.Position] = unifiedSkillEmpty
			continue
		}
		slots[e.Position] = e.Value
	}
	return SkillIDsFromSlots(slots)
}

// EncodeSkillLockBlock builds the 386 byte skill lock block:
//
//	[0]        valid (1)
//	[1]        0
//	[2..257]   128 u16 slots (page 0 then page 1, compact ascending)
//	[258..385] 128 exist bytes, one per slot
//
// The group shape was cross-checked against the client's own account option
// template (protocol/templates/account-options-current.bin), whose 286 and 157
// entry groups use exactly this valid + 1 + N*u16 + N*exist layout.
const (
	UnifiedSkillLockBlockSize = 2 + 2*UnifiedSkillSlots + UnifiedSkillSlots
	UnifiedSkillLockSlotsAt   = 2
	UnifiedSkillLockExistAt   = 2 + 2*UnifiedSkillSlots
)

func EncodeSkillLockBlock(ids []uint16) ([]byte, error) {
	if len(ids) > UnifiedSkillSlots {
		return nil, fmt.Errorf("too many locked skills")
	}
	block := make([]byte, UnifiedSkillLockBlockSize)
	block[0] = 1
	slots := CompactSkillSlots(ids)
	for i, v := range slots {
		binary.LittleEndian.PutUint16(block[UnifiedSkillLockSlotsAt+i*2:], v)
		if v != unifiedSkillEmpty {
			block[UnifiedSkillLockExistAt+i] = 1
		}
	}
	return block, nil
}

// NOTI2827 (UNIFIED_OPTION_CHARAC) is a single 3539 byte block made of fixed,
// subtype ordered objects. The client's own default block is embedded verbatim:
// the server fills only the two skill lock objects and leaves every other byte
// at the client default, which the client consumes normally.
//
// The offsets come from the client's "locate object by subtype" switch
// (sub_14757B0C0 in the 115 US build): subtype 19 at 2736 and subtype 20 at
// 3122, both 386 bytes and served by the same lock handler. Writing only the
// first object restores the locks incompletely after a character reselect, so
// both get the same data. Subtype 18 sits at 2716 and is only 20 bytes wide, so
// it is not a lock object and must not be written.
//
//go:embed templates/unified-charac-options-current.bin
var nativeCharacOptions []byte

const (
	// UnifiedCharacOptionSize is the whole NOTI2827 payload.
	UnifiedCharacOptionSize = 3539
	// UnifiedCharacSkillLockAt is the subtype 19 lock object.
	UnifiedCharacSkillLockAt = 2736
	// UnifiedCharacSkillLockSecondAt is the subtype 20 lock object.
	UnifiedCharacSkillLockSecondAt = 3122
)

// CharacOptionsTemplate returns a copy of the embedded client block, for
// callers that override only the lock object offset.
func CharacOptionsTemplate() []byte {
	return append([]byte(nil), nativeCharacOptions...)
}

// UnifiedCharacOptions returns the NOTI2827 payload that restores the locked
// skills. Send it as the final frame of the town entry batch: this client
// crashes about 0.3~1 second after town entry when 2827 arrives early, whatever
// the payload contains.
func UnifiedCharacOptions(locks []uint16) ([]byte, error) {
	if len(nativeCharacOptions) != UnifiedCharacOptionSize {
		return nil, fmt.Errorf("character option template size mismatch")
	}
	return UnifiedCharacOptionsFrom(nativeCharacOptions, locks, UnifiedCharacSkillLockAt)
}

// UnifiedCharacOptionsFrom is UnifiedCharacOptions with an explicit client
// block and subtype 19 offset, for a build whose layout differs. The subtype 20
// object is expected to follow the first one directly.
func UnifiedCharacOptionsFrom(template []byte, locks []uint16, skillLockAt int) ([]byte, error) {
	block, e := EncodeSkillLockBlock(locks)
	if e != nil {
		return nil, e
	}
	p := append([]byte(nil), template...)
	for _, at := range []int{skillLockAt, skillLockAt + UnifiedSkillLockBlockSize} {
		if at < 0 || at+UnifiedSkillLockBlockSize > len(p) {
			return nil, fmt.Errorf("skill lock object does not fit the character option block")
		}
		copy(p[at:], block)
	}
	return p, nil
}

// UnifiedCharacSettingsAt is the subtype 5 (character system settings) object
// inside the 3539-byte NOTI2827 block, located by walking the client sub_14757B0C0
// subtype switch backwards from the anchored skill-lock objects (subtype 18 @
// 2716, subtype 19 @ 2736). It holds N=173 u16 slots at obj+2, with one exist
// byte per slot at obj+2+2*N. A captured CMD2377 subtype-0x05 frame (entries
// Position=94/101/103/137/138) confirms entries map 1:1 onto these slots.
const (
	UnifiedCharacSettingsAt    = 1558
	UnifiedCharacSettingsSlots = 173
)

// FillCharacSettings overlays the stored per-character settings onto a fresh
// 3539-byte NOTI2827 block. Unknown or out-of-range positions are ignored so a
// malformed save can never corrupt the block.
func FillCharacSettings(block []byte, settings map[uint16]uint16) error {
	if len(block) != UnifiedCharacOptionSize {
		return fmt.Errorf("character option block size mismatch")
	}
	obj := UnifiedCharacSettingsAt
	if obj+2+3*UnifiedCharacSettingsSlots > len(block) {
		return fmt.Errorf("settings object does not fit the block")
	}
	for position, value := range settings {
		if int(position) >= UnifiedCharacSettingsSlots {
			continue
		}
		slot := obj + 2 + int(position)*2
		binary.LittleEndian.PutUint16(block[slot:], value)
		exist := obj + 2 + 2*UnifiedCharacSettingsSlots + int(position)
		block[exist] = 1
	}
	return nil
}

