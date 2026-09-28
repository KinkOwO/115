package protocol

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// The two live confirmations both sent this body. op 4 and flag 0 are the
// hard-coded literals at 0x14407dca0, mode 1 comes from the window flag, and the
// trailing 9 is the selected index.
func TestDecodeMakeSkinMatchesCapture(t *testing.T) {
	raw, err := hex.DecodeString("0401000000000900")
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeMakeSkin(raw)
	if err != nil {
		t.Fatal(err)
	}
	if r.Op != 4 || r.Mode != 1 || r.Flag != 0 || r.Index != 9 {
		t.Fatalf("decoded %+v", r)
	}
}

func TestDecodeMakeSkinRejectsShortBody(t *testing.T) {
	if _, err := DecodeMakeSkin([]byte{4, 1, 0, 0, 0, 0, 9}); err == nil {
		t.Fatal("seven bytes must not decode")
	}
}

// 0x1444eff40 reads a u16 count, then two dwords per entry for every subtype but
// 4, which consumes the id alone.
func TestSkinCargoInfoEntryWidth(t *testing.T) {
	entry := []SkinCargoEntry{{SkinID: 101010438, Extra: 1}}
	got, err := SkinCargoInfo(0, entry, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 1, 0}
	want = add32(want, 101010438)
	want = add32(want, 1)
	want = add16(want, 0)
	if !bytes.Equal(got, want) {
		t.Fatalf("subtype 0 built %x, want %x", got, want)
	}

	got, err = SkinCargoInfo(4, entry, nil)
	if err != nil {
		t.Fatal(err)
	}
	want = []byte{4, 1, 0}
	want = add32(want, 101010438)
	want = add16(want, 0)
	if !bytes.Equal(got, want) {
		t.Fatalf("subtype 4 built %x, want %x", got, want)
	}
}

func TestSkinCargoInfoRefusesBadInput(t *testing.T) {
	if _, err := SkinCargoInfo(10, nil, nil); err == nil {
		t.Fatal("subtype 10 is out of range")
	}
	if _, err := SkinCargoInfo(0, []SkinCargoEntry{{SkinID: 0}}, nil); err == nil {
		t.Fatal("a zero skin id would be dropped by the client")
	}
}

// The selection push is five bytes because the subtype-4 core 0x1444eeca0 reads
// exactly one u32 and no length prefix - unlike the container push, which is
// count-delimited. A length prefix here would shift the id and the client would
// highlight nothing.
func TestSkinCargoSelectionInfoLayout(t *testing.T) {
	got := SkinCargoSelectionInfo(101010912)
	want := []byte{byte(SkinCargoWeaponShape)}
	want = add32(want, 101010912)
	if !bytes.Equal(got, want) {
		t.Fatalf("selection built %x, want %x", got, want)
	}
	if len(got) != 5 {
		t.Fatalf("selection is %d bytes, want 5", len(got))
	}
}

// A zero id is the unapply body: the reader clears the page's selection before it
// reads the id, so this frame leaves the page with nothing worn. The body still has
// to be five bytes - it is the same fixed shape, with a zero id in the last four.
func TestSkinCargoSelectionInfoClearsWithZero(t *testing.T) {
	got := SkinCargoSelectionInfo(0)
	want := []byte{byte(SkinCargoWeaponShape), 0, 0, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("unapply selection built %x, want %x", got, want)
	}
}

// 0x1444ed400 reads the u8 before the u32, so that is the order on the wire.
func TestRecentAddSkinListWireOrder(t *testing.T) {
	got, err := RecentAddSkinList([]RecentAddSkinEntry{{Kind: 3, SkinID: 101010438}})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{1, 3}
	want = add32(want, 101010438)
	if !bytes.Equal(got, want) {
		t.Fatalf("built %x, want %x", got, want)
	}
}

// The sender 0x1444f1090 always flushes the same 88 bytes: u32 subtype,
// u32 reserved, then 20 u32 id cells. Live capture 2026-09-26 12:33, taken on
// the weapon-shape tab seconds after the replication confirmation:
//
//	04000000 00000000 e04d0506 00 * 19
//
// i.e. subtype 4, reserved 0, ids[0] = 101010912 - the replicated katana's own
// template. The unused cells are padding and must not be reported as ids.
func TestDecodeSkinCargoSyncReadsLiveApplyFrame(t *testing.T) {
	body, e := hex.DecodeString("0400000000000000e04d0506" + strings.Repeat("00", 76))
	if e != nil {
		t.Fatal(e)
	}
	if len(body) != 8+4*SkinCargoSyncIDCount {
		t.Fatalf("frame is %d bytes, want %d", len(body), 8+4*SkinCargoSyncIDCount)
	}
	got, e := DecodeSkinCargoSync(body)
	if e != nil {
		t.Fatal(e)
	}
	if got.Subtype != SkinCargoWeaponShape {
		t.Fatalf("subtype=%d, want %d", got.Subtype, SkinCargoWeaponShape)
	}
	if got.Reserved != 0 {
		t.Fatalf("reserved=%d, want 0 on the apply sender", got.Reserved)
	}
	if len(got.IDs) != 1 || got.IDs[0] != 101010912 {
		t.Fatalf("ids=%v, want [101010912]", got.IDs)
	}
}

// An earlier capture, taken before any skin was replicated, reported subtype 1
// with id 100000 - the client's default entry for the tab it opened. It decodes
// the same way; the subtype is what tells the two apart.
func TestDecodeSkinCargoSyncReadsBrowseFrame(t *testing.T) {
	body, e := hex.DecodeString("0100000000000000a0860100" + strings.Repeat("00", 76))
	if e != nil {
		t.Fatal(e)
	}
	got, e := DecodeSkinCargoSync(body)
	if e != nil {
		t.Fatal(e)
	}
	if got.Subtype == SkinCargoWeaponShape {
		t.Fatalf("subtype=%d, want the pre-replication browse subtype", got.Subtype)
	}
	if len(got.IDs) != 1 || got.IDs[0] != 100000 {
		t.Fatalf("ids=%v, want [100000]", got.IDs)
	}
}

// A short body must be refused rather than read past its end: the fixed width is
// what makes every cell resolvable.
func TestDecodeSkinCargoSyncRejectsShortBody(t *testing.T) {
	if _, e := DecodeSkinCargoSync(make([]byte, 8+4*SkinCargoSyncIDCount-1)); e == nil {
		t.Fatal("short skin cargo sync accepted")
	}
}

// 实机 2026-09-26 三次 CMD857（幻化栏窗口点 OK 开栏）：第一个 u16 恒为 0xffff ——
// 客户端根本不报券在背包哪一格；第二个 u16 才是窗口类型，0x0b(11) 光环、0x20(32)
// 宠物。body 会补齐到 16 字节。
func TestDecodeOpenSkinSlotMatchesCapture(t *testing.T) {
	raw, e := hex.DecodeString("ffff0b0000000000000000000000000000")
	if e != nil {
		t.Fatal(e)
	}
	r, e := DecodeOpenSkinSlot(raw)
	if e != nil || r.Slot != 0xffff || r.Window != SkinSlotWindowAura {
		t.Fatalf("aura window -> %+v %v", r, e)
	}
	if SkinSlotWindowAura != 11 || SkinSlotWindowCreature != 32 {
		t.Fatalf("windows %d %d", SkinSlotWindowAura, SkinSlotWindowCreature)
	}
	raw2, e := hex.DecodeString("ffff200000000000000000000000000000")
	if e != nil {
		t.Fatal(e)
	}
	if r2, e := DecodeOpenSkinSlot(raw2); e != nil || r2.Window != SkinSlotWindowCreature {
		t.Fatalf("creature window -> %+v %v", r2, e)
	}
}

func TestDecodeOpenSkinSlotRejectsShortBody(t *testing.T) {
	if _, e := DecodeOpenSkinSlot([]byte{0xff, 0xff, 0x0b}); e == nil {
		t.Fatal("3-byte body accepted")
	}
}

// 回包必须**恰好 3 字节**：收包分发器把 body[0] 当成功标志并推进游标，handler
// 0x145286a50 再从偏移 1 读回窗口类型。多一个字节就把 handler 之后的读取整体错位。
func TestOpenSkinSlotReplyLayout(t *testing.T) {
	for _, w := range []uint16{SkinSlotWindowAura, SkinSlotWindowCreature} {
		got := OpenSkinSlotReply(w)
		if len(got) != 3 || got[0] != 1 || got[1] != byte(w) || got[2] != byte(w>>8) {
			t.Fatalf("window %d -> % x", w, got)
		}
	}
	if got := OpenSkinSlotReply(SkinSlotWindowAura); !bytes.Equal(got, []byte{1, 0x0b, 0x00}) {
		t.Fatalf("aura reply -> % x", got)
	}
	if got := OpenSkinSlotReply(SkinSlotWindowCreature); !bytes.Equal(got, []byte{1, 0x20, 0x00}) {
		t.Fatalf("creature reply -> % x", got)
	}
}
