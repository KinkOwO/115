package protocol

import (
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
//	+08 5B                   (context parameter / pointer / marker)
//	+13 u8  scope            (0 = character, 1 = account)
//	+14 u8  subtype          (0x01 account, 0x03 hotkeys A, 0x04 hotkeys B, 0x05 settings, 0x07 hotkey UI, 0x13 skill lock)
//	+15 u32 entry count      (little endian)
//	+19      count x (u16 position, u16 value)
//	tail    0..8 bytes of zero padding
//
// A forwarded repair note claims +04 is always 01000000; the captures falsify
// that, so +04 is never validated here.
const (
	unifiedOptionLen     = 19
	unifiedOptionMaxTail = 8

	UnifiedOptionScopeCharac  = 0x00
	UnifiedOptionScopeAccount = 0x01

	// UnifiedOptionAccount is the subtype of the account-level option block,
	// restored through NOTI2826.
	UnifiedOptionAccount = 0x01
	// UnifiedOptionHotkeys is keyboard scheme A (Type A).
	UnifiedOptionHotkeys = 0x03
	// UnifiedOptionHotkeysExt is keyboard scheme B (Type B).
	UnifiedOptionHotkeysExt = 0x04
	// UnifiedOptionSettings is the subtype of the ordinary settings block,
	// which shares the opcode but has its own index/value semantics.
	UnifiedOptionSettings = 0x05
	// UnifiedOptionHotkeyUI is the UI state reporting frame.
	UnifiedOptionHotkeyUI = 0x07
	// UnifiedOptionSkillLock is the subtype that carries the skill lock block.
	UnifiedOptionSkillLock = 0x13

	UnifiedHotkeysSlots     = 157
	UnifiedHotkeysBlockSize = 473
	UnifiedHotkeysSlotsAt   = 2
	UnifiedHotkeysExistAt   = 2 + 157*2 // 316

	UnifiedAccountHotkeysAt    = 1277 // Subtype 3 in NOTI2826 (offset 0x4fd)
	UnifiedAccountHotkeysExtAt = 1750 // Subtype 4 in NOTI2826 (offset 0x6d6)

	UnifiedCharacHotkeysAt    = 0   // Subtype 3 in NOTI2827
	UnifiedCharacHotkeysExtAt = 473 // Subtype 4 in NOTI2827
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
	r.Scope, r.Subtype = p[13], p[14]
	count := int(binary.LittleEndian.Uint32(p[15:19]))
	if count > 512 {
		return r, fmt.Errorf("unsupported unified option entry count %d", count)
	}
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
// inside the 3539-byte NOTI2827 block. The current client's sub_14757B0C0
// selects offset 946 for subtype 5 (subtype 6 starts at 1467). The consumer
// sub_147578C40 requires obj[0] == 1 and an exist byte at obj+348+position
// before reading the u16 at obj+2+2*position; there are 173 positions.
// This includes the skill cooldown alert settings saved by CMD2377.
const (
	UnifiedCharacSettingsAt    = 946
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
		block[obj] = 1
		slot := obj + 2 + int(position)*2
		binary.LittleEndian.PutUint16(block[slot:], value)
		exist := obj + 2 + 2*UnifiedCharacSettingsSlots + int(position)
		block[exist] = 1
	}
	return nil
}

// FillHotkeysBlock fills a 473-byte hotkey block (valid + version + 157*u16 + 157*exist)
// at the destination slice.
func FillHotkeysBlock(dst []byte, hotkeys map[uint16]uint16) error {
	if len(dst) < UnifiedHotkeysBlockSize {
		return fmt.Errorf("hotkey block buffer too short: %d < %d", len(dst), UnifiedHotkeysBlockSize)
	}
	if len(hotkeys) == 0 {
		return nil
	}
	dst[0] = 1
	dst[1] = 0
	for pos, keycode := range hotkeys {
		if pos >= UnifiedHotkeysSlots {
			continue
		}
		slot := UnifiedHotkeysSlotsAt + int(pos)*2
		binary.LittleEndian.PutUint16(dst[slot:], keycode)
		exist := UnifiedHotkeysExistAt + int(pos)
		dst[exist] = 1
	}
	return nil
}

// FillCharacHotkeys overlays character-specific hotkey schemes onto the 3539-byte NOTI2827 block.
// Subtype 3 (Scheme A) sits at offset 0, and Subtype 4 (Scheme B) sits at offset 473.
func FillCharacHotkeys(block []byte, hotkeys, hotkeysExt map[uint16]uint16) error {
	if len(block) != UnifiedCharacOptionSize {
		return fmt.Errorf("character option block size mismatch: %d != %d", len(block), UnifiedCharacOptionSize)
	}
	if len(hotkeys) > 0 {
		if err := FillHotkeysBlock(block[UnifiedCharacHotkeysAt:UnifiedCharacHotkeysAt+UnifiedHotkeysBlockSize], hotkeys); err != nil {
			return err
		}
	}
	if len(hotkeysExt) > 0 {
		if err := FillHotkeysBlock(block[UnifiedCharacHotkeysExtAt:UnifiedCharacHotkeysExtAt+UnifiedHotkeysBlockSize], hotkeysExt); err != nil {
			return err
		}
	}
	return nil
}

