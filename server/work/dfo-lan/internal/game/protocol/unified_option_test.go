package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type capturedUnifiedFrames struct {
	Frames []struct {
		Time       string `json:"time"`
		PayloadHex string `json:"payload_hex"`
	} `json:"frames"`
}

func loadCapturedUnifiedFrames(t *testing.T) [][]byte {
	t.Helper()
	b, e := os.ReadFile("testdata/native_unified_option_frames36.json")
	if e != nil {
		t.Fatal(e)
	}
	var doc capturedUnifiedFrames
	if e = json.Unmarshal(b, &doc); e != nil {
		t.Fatal(e)
	}
	out := make([][]byte, 0, len(doc.Frames))
	for _, f := range doc.Frames {
		p, e := hex.DecodeString(f.PayloadHex)
		if e != nil {
			t.Fatalf("%s: %v", f.Time, e)
		}
		out = append(out, p)
	}
	return out
}

// Every CMD2377 frame this client produced while playing conforms to the universal
// 19-byte header (+13 scope, +14 subtype, +15 u32 count, entries from +19).
func TestDecodeCapturedUnifiedOptionFrames(t *testing.T) {
	frames := loadCapturedUnifiedFrames(t)
	if len(frames) < 37 {
		t.Fatalf("captured frames = %d, want the 37 observed during play", len(frames))
	}
	locks := 0
	for i, p := range frames {
		opt, e := DecodeUnifiedOption(p)
		if e != nil {
			t.Fatalf("frame %d failed to decode: %v", i, e)
		}
		if opt.Subtype != UnifiedOptionSkillLock {
			continue
		}
		locks++
		if opt.Scope != 0 {
			t.Fatalf("captured skill lock frame scope = %d, want 0", opt.Scope)
		}
		if len(opt.Entries) == 0 || opt.Entries[0].Position != 0 {
			t.Fatalf("captured skill lock frame %d does not re-state page 0: %v", i, opt.Entries)
		}
	}
	if locks != 13 {
		t.Fatalf("captured skill lock frames = %d, want 13", locks)
	}
}

func TestDecodeUnifiedOptionRejectsShortAndForeignFrames(t *testing.T) {
	if _, e := DecodeUnifiedOption(make([]byte, 18)); e == nil {
		t.Fatal("decoded a short frame")
	}
	badPadding := make([]byte, 24)
	badPadding[14], badPadding[15] = UnifiedOptionSkillLock, 1
	badPadding[23] = 0xAA // non-zero padding
	if _, e := DecodeUnifiedOption(badPadding); e == nil {
		t.Fatal("decoded a frame with bad padding")
	}
	truncated := []byte{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, UnifiedOptionSkillLock, 2, 0, 0, 0, 1, 0, 0}
	if _, e := DecodeUnifiedOption(truncated); e == nil {
		t.Fatal("decoded a frame whose count exceeds its length")
	}
}

func TestDecodeHotkeyFrames(t *testing.T) {
	// Real in-game hotkey apply frame captured from gameplay:
	raw, err := hex.DecodeString("48091c4901000000000000000001030800000000004d00010037000200490003003a001400860015008600160086001b0086000000000000")
	if err != nil {
		t.Fatal(err)
	}
	opt, err := DecodeUnifiedOption(raw)
	if err != nil {
		t.Fatalf("decode hotkeys frame: %v", err)
	}
	if opt.Scope != UnifiedOptionScopeAccount || opt.Subtype != UnifiedOptionHotkeys {
		t.Fatalf("scope=%d subtype=%d, want scope=1 subtype=3", opt.Scope, opt.Subtype)
	}
	if len(opt.Entries) != 8 {
		t.Fatalf("entries=%d, want 8", len(opt.Entries))
	}
	if opt.Entries[0].Position != 0 || opt.Entries[0].Value != 0x4D {
		t.Fatalf("entry 0 = %+v, want pos=0 val=0x4D", opt.Entries[0])
	}
}

func TestHotkeysBlockFillAndRestore(t *testing.T) {
	hotkeysA := map[uint16]uint16{0: 0x4D, 1: 0x37, 2: 0x49}
	hotkeysB := map[uint16]uint16{4: 0x38, 58: 0x86}

	// 1. Account Options (3648 bytes, Subtype 3 at 1277, Subtype 4 at 1750)
	accBlock, err := AccountOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = FillAccountHotkeys(accBlock, hotkeysA, hotkeysB); err != nil {
		t.Fatal(err)
	}
	// Verify Scheme A at 1277
	if accBlock[UnifiedAccountHotkeysAt] != 1 {
		t.Fatal("account hotkey A valid flag not set")
	}
	if binary.LittleEndian.Uint16(accBlock[UnifiedAccountHotkeysAt+2:]) != 0x4D {
		t.Fatal("slot 0 keycode mismatch")
	}
	if accBlock[UnifiedAccountHotkeysAt+UnifiedHotkeysExistAt] != 1 {
		t.Fatal("slot 0 exist flag not set")
	}
	// Verify Scheme B at 1750
	if accBlock[UnifiedAccountHotkeysExtAt] != 1 {
		t.Fatal("account hotkey B valid flag not set")
	}
	if binary.LittleEndian.Uint16(accBlock[UnifiedAccountHotkeysExtAt+2+4*2:]) != 0x38 {
		t.Fatal("slot 4 keycode mismatch")
	}
	if accBlock[UnifiedAccountHotkeysExtAt+UnifiedHotkeysExistAt+4] != 1 {
		t.Fatal("slot 4 exist flag not set")
	}

	// 2. Character Options (3539 bytes, Subtype 3 at 0, Subtype 4 at 473)
	charBlock := CharacOptionsTemplate()
	if err = FillCharacHotkeys(charBlock, hotkeysA, hotkeysB); err != nil {
		t.Fatal(err)
	}
	if charBlock[UnifiedCharacHotkeysAt] != 1 {
		t.Fatal("charac hotkey A valid flag not set")
	}
	if binary.LittleEndian.Uint16(charBlock[UnifiedCharacHotkeysAt+2:]) != 0x4D {
		t.Fatal("charac slot 0 keycode mismatch")
	}
	if charBlock[UnifiedCharacHotkeysAt+UnifiedHotkeysExistAt] != 1 {
		t.Fatal("charac slot 0 exist flag not set")
	}
	if charBlock[UnifiedCharacHotkeysExtAt] != 1 {
		t.Fatal("charac hotkey B valid flag not set")
	}
	if binary.LittleEndian.Uint16(charBlock[UnifiedCharacHotkeysExtAt+2+4*2:]) != 0x38 {
		t.Fatal("charac slot 4 keycode mismatch")
	}
}

// A frame only lists the slots that changed since the last baseline, so it may
// never replace the stored set.
func TestMergeSkillLocksAppliesIncrementalFrames(t *testing.T) {
	got := MergeSkillLocks([]uint16{9, 72}, []UnifiedOptionEntry{{Position: 2, Value: 73}})
	want := []uint16{9, 72, 73}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %v, want %v", got, want)
	}
}

func TestMergeSkillLocksTreatsZeroAsDelete(t *testing.T) {
	got := MergeSkillLocks([]uint16{9, 72, 73}, []UnifiedOptionEntry{{Position: 1, Value: 0}})
	want := []uint16{9, 73}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged = %v, want %v", got, want)
	}
}

// A frame that starts on a page boundary rebuilds that whole page before its
// entries are applied, so page 0 skills disappear while page 1 survives.
func TestMergeSkillLocksRebuildsTouchedPage(t *testing.T) {
	got := MergeSkillLocks([]uint16{9, 517}, []UnifiedOptionEntry{{Position: 0, Value: 40}, {Position: 1, Value: 0}})
	if !reflect.DeepEqual(got, []uint16{40, 517}) {
		t.Fatalf("page 0 rebuild = %v, want [40 517]", got)
	}
	second := MergeSkillLocks([]uint16{9, 517}, []UnifiedOptionEntry{{Position: 64, Value: 6}})
	if !reflect.DeepEqual(second, []uint16{9, 518}) {
		t.Fatalf("page 1 rebuild = %v, want [9 518]", second)
	}
}

func TestMergeSkillLocksUsesPageTwoStride(t *testing.T) {
	got := MergeSkillLocks(nil, []UnifiedOptionEntry{{Position: 64, Value: 5}})
	if !reflect.DeepEqual(got, []uint16{517}) {
		t.Fatalf("page two merge = %v, want [517]", got)
	}
}

func TestMergeSkillLocksIgnoresSlotsPastTheBlock(t *testing.T) {
	got := MergeSkillLocks([]uint16{9}, []UnifiedOptionEntry{{Position: 200, Value: 3}})
	if !reflect.DeepEqual(got, []uint16{9}) {
		t.Fatalf("out of range merge = %v, want [9]", got)
	}
}

func TestSkillLockBlockEncoding(t *testing.T) {
	block, e := EncodeSkillLockBlock([]uint16{9, 517})
	if e != nil {
		t.Fatal(e)
	}
	if len(block) != UnifiedSkillLockBlockSize || UnifiedSkillLockBlockSize != 386 {
		t.Fatalf("block size = %d, want 386", len(block))
	}
	if block[0] != 1 {
		t.Fatalf("valid byte = %d, want 1", block[0])
	}
	if block[UnifiedSkillLockSlotsAt] != 9 || block[UnifiedSkillLockSlotsAt+1] != 0 {
		t.Fatalf("page 0 first slot = % x, want 09 00", block[UnifiedSkillLockSlotsAt:UnifiedSkillLockSlotsAt+2])
	}
	if block[UnifiedSkillLockSlotsAt+64*2] != 5 {
		t.Fatalf("page 1 first slot = %d, want 5", block[UnifiedSkillLockSlotsAt+64*2])
	}
	if block[UnifiedSkillLockSlotsAt+2] != 0xff || block[UnifiedSkillLockSlotsAt+3] != 0xff {
		t.Fatal("empty slot is not 0xFFFF")
	}
	if block[UnifiedSkillLockExistAt] != 1 || block[UnifiedSkillLockExistAt+64] != 1 {
		t.Fatal("exist flags missing for locked skills")
	}
	if block[UnifiedSkillLockExistAt+1] != 0 {
		t.Fatal("exist flag set for an empty slot")
	}
}

func TestSkillLockBlockRejectsMoreThanTheBlockHolds(t *testing.T) {
	ids := make([]uint16, UnifiedSkillSlots+1)
	for i := range ids {
		ids[i] = uint16(i)
	}
	if _, e := EncodeSkillLockBlock(ids); e == nil {
		t.Fatal("encoded more locked skills than the block holds")
	}
}

// The server stores only the id set and re-derives the layout, so encoding and
// compacting must round trip.
func TestSkillLockBlockRoundTrips(t *testing.T) {
	ids := []uint16{3, 9, 517, 518, 1023}
	block, e := EncodeSkillLockBlock(ids)
	if e != nil {
		t.Fatal(e)
	}
	slots := make([]uint16, UnifiedSkillSlots)
	for i := range slots {
		slots[i] = uint16(block[UnifiedSkillLockSlotsAt+i*2]) | uint16(block[UnifiedSkillLockSlotsAt+i*2+1])<<8
	}
	if got := SkillIDsFromSlots(slots); !reflect.DeepEqual(got, ids) {
		t.Fatalf("round trip = %v, want %v", got, ids)
	}
}

// One real session captured in play: eight locks added one at a time and then
// four removed, each step in the client's own frame. Every frame re-states the
// whole page (first position 0, entry count equal to the locked set) in compact
// ascending order, so the merge must follow it exactly - including the insert
// of skill 1, which re-orders the whole page, and the removals, which shift the
// remaining slots forward.
func TestCapturedSkillLockSession(t *testing.T) {
	want := [][]uint16{
		{3}, {3, 68}, {3, 31, 68}, {3, 9, 31, 68}, {3, 9, 31, 68, 109},
		{1, 3, 9, 31, 68, 109}, {1, 3, 9, 31, 58, 68, 109}, {1, 3, 9, 20, 31, 58, 68, 109},
		{1, 3, 20, 31, 58, 68, 109}, {1, 3, 20, 58, 68, 109}, {1, 3, 20, 58, 109}, {1, 20, 58, 109},
	}
	var locks []uint16
	step := 0
	for _, p := range capturedFramesFrom(t, "2026-09-20T18:09:") {
		opt, e := DecodeUnifiedOption(p)
		if e != nil {
			t.Fatal(e)
		}
		if opt.Subtype != UnifiedOptionSkillLock {
			continue
		}
		if len(opt.Entries) == 0 || opt.Entries[0].Position != 0 {
			t.Fatalf("skill lock frame %d does not re-state page 0: %v", step, opt.Entries)
		}
		locks = MergeSkillLocks(locks, opt.Entries)
		if step >= len(want) {
			t.Fatalf("more lock frames than expected")
		}
		if !reflect.DeepEqual(locks, want[step]) {
			t.Fatalf("frame %d locks = %v, want %v", step, locks, want[step])
		}
		step++
	}
	if step != len(want) {
		t.Fatalf("replayed %d lock frames, want %d", step, len(want))
	}
}

// capturedFramesFrom returns every decodable frame captured at or after the
// given timestamp prefix, in capture order.
func capturedFramesFrom(t *testing.T, prefix string) [][]byte {
	t.Helper()
	b, e := os.ReadFile("testdata/native_unified_option_frames36.json")
	if e != nil {
		t.Fatal(e)
	}
	var doc capturedUnifiedFrames
	if e = json.Unmarshal(b, &doc); e != nil {
		t.Fatal(e)
	}
	var out [][]byte
	for _, f := range doc.Frames {
		if !strings.HasPrefix(f.Time, prefix) {
			continue
		}
		p, e := hex.DecodeString(f.PayloadHex)
		if e != nil {
			t.Fatalf("%s: %v", f.Time, e)
		}
		if _, e = DecodeUnifiedOption(p); e != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

// The embedded block is the client's own default: an empty lock object at 2736
// (subtype 19) and another at 3122 (subtype 20), each 386 bytes with an
// all-0xFFFF slot table and no exist flags. Subtype 18 sits at 2716 and is only
// 20 bytes wide, so it must not be touched.
func TestEmbeddedCharacOptionsCarryEmptyLockObjects(t *testing.T) {
	block := CharacOptionsTemplate()
	if len(block) != UnifiedCharacOptionSize {
		t.Fatalf("template size = %d, want %d", len(block), UnifiedCharacOptionSize)
	}
	if UnifiedCharacSkillLockAt+UnifiedSkillLockBlockSize != UnifiedCharacSkillLockSecondAt {
		t.Fatalf("lock objects are not adjacent: %d + %d != %d",
			UnifiedCharacSkillLockAt, UnifiedSkillLockBlockSize, UnifiedCharacSkillLockSecondAt)
	}
	for _, at := range []int{UnifiedCharacSkillLockAt, UnifiedCharacSkillLockSecondAt} {
		if block[at] != 0 || block[at+1] != 0 {
			t.Fatalf("object at %d is not the empty default", at)
		}
		for i := 0; i < UnifiedSkillSlots; i++ {
			if v := binary.LittleEndian.Uint16(block[at+UnifiedSkillLockSlotsAt+i*2:]); v != unifiedSkillEmpty {
				t.Fatalf("slot %d of object %d = %#x, want 0xFFFF", i, at, v)
			}
			if block[at+UnifiedSkillLockExistAt+i] != 0 {
				t.Fatalf("exist flag %d of object %d set in the default block", i, at)
			}
		}
	}
}

// Both lock objects get the same data, and the subtype 18 object in front of
// them stays at its default.
func TestUnifiedCharacOptionsFillsBothLockObjects(t *testing.T) {
	payload, e := UnifiedCharacOptions([]uint16{58, 517})
	if e != nil {
		t.Fatal(e)
	}
	template := CharacOptionsTemplate()
	block, e := EncodeSkillLockBlock([]uint16{58, 517})
	if e != nil {
		t.Fatal(e)
	}
	if len(payload) != len(template) {
		t.Fatalf("payload length = %d, want %d", len(payload), len(template))
	}
	for _, at := range []int{UnifiedCharacSkillLockAt, UnifiedCharacSkillLockSecondAt} {
		if !bytes.Equal(payload[at:at+len(block)], block) {
			t.Fatalf("lock object at %d was not filled", at)
		}
		if payload[at] != 1 {
			t.Fatalf("object at %d is not marked valid", at)
		}
	}
	for i := 2716; i < UnifiedCharacSkillLockAt; i++ {
		if payload[i] != template[i] {
			t.Fatalf("subtype 18 byte %d changed", i)
		}
	}
}

func TestFillCharacEffectsOnlyTouchesSubtype18Slots(t *testing.T) {
	block := CharacOptionsTemplate()
	before := append([]byte(nil), block...)
	settings := map[uint16]uint16{0: 12, 2: 63, 5: 100}
	if err := FillCharacEffects(block, settings); err != nil {
		t.Fatal(err)
	}
	obj := UnifiedCharacEffectsAt
	if block[obj] != 1 {
		t.Fatal("subtype 18 object was not marked valid")
	}
	for position, want := range map[uint16]uint16{0: 12, 2: 63, 5: 100} {
		slot := obj + 2 + int(position)*2
		if got := binary.LittleEndian.Uint16(block[slot:]); got != want {
			t.Errorf("position %d value = %d, want %d", position, got, want)
		}
		exist := obj + 2 + 2*UnifiedCharacEffectsSlots + int(position)
		if block[exist] != 1 {
			t.Errorf("position %d exist flag was not set", position)
		}
	}
	withOutOfRange := CharacOptionsTemplate()
	if err := FillCharacEffects(withOutOfRange, map[uint16]uint16{0: 12, 2: 63, 5: 100, 6: 999}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(block[obj:UnifiedCharacSkillLockAt], withOutOfRange[obj:UnifiedCharacSkillLockAt]) {
		t.Fatal("out-of-range position changed the subtype 18 object")
	}
	if !bytes.Equal(block[:obj], before[:obj]) || !bytes.Equal(block[UnifiedCharacSkillLockAt:], before[UnifiedCharacSkillLockAt:]) {
		t.Fatal("filling subtype 18 changed unrelated settings or skill locks")
	}
}
