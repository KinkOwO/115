package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 官服两份抓包在这一帧上（除副本 id 与尾部动态值）**逐字节相同**：
//
//	Normal  2026-10-02 21:42:18.606  op=28  dungeon 100003878
//	Extreme 2026-10-08 22:05:45.580  op=28  dungeon 100004079
//
// 本仓曾把 @19..26 写成 ff×8、官服常量 05 写在 @31，导致 @26 起整段错位一格；
// 这个测试把官方字节钉住，防止再次漂移。
func TestForestDungeonInfoMatchesOfficialCapture(t *testing.T) {
	const officialNormal = "26f0f505000000000000ffff0000000001000bffffffffffffff0001000005010001000100ffffffff6e2ce1c9360000"
	const officialExtreme = "eff0f505000000000000ffff0000000001000bffffffffffffff0001000005010001000100ffffffff7c465d5e420000"
	normal := mustHexBytes(t, officialNormal)
	extreme := mustHexBytes(t, officialExtreme)
	if len(normal) != 48 || len(extreme) != 48 {
		t.Fatalf("official forest NOTI28 must be 48 bytes: %d/%d", len(normal), len(extreme))
	}
	// 除 id 与尾部动态值外，两个难度必须一致 —— 这正是本函数不区分难度的依据。
	for i := 4; i < 41; i++ {
		if normal[i] != extreme[i] {
			t.Fatalf("official Normal/Extreme differ at @%d: %02x vs %02x", i, normal[i], extreme[i])
		}
	}
	got := ForestDungeonInfo(0x05f5f026, 0, [2]byte{}, 0xc9e12c6e)
	if len(got) != 48 {
		t.Fatalf("forest NOTI28 = %d bytes, want the official 48", len(got))
	}
	for i := 0; i < 45; i++ {
		if got[i] != normal[i] {
			t.Fatalf("forest NOTI28 @%d = %02x, want the official %02x（整帧 %x）", i, got[i], normal[i], got)
		}
	}
	// 客户端在这个偏移**单独读**这一字节（native 0x1452ada9e / offset=30）：
	// 官服两个难度都写 05，本仓旧实现把它写到了 @31（客户端不读的位置）。
	if got[30] != 0x05 {
		t.Fatalf("forest NOTI28 @30 = %02x, want the official 0x05", got[30])
	}
	if got[26] != 0x00 {
		t.Fatalf("forest NOTI28 @26 = %02x, want the official 0x00", got[26])
	}
	if !bytes.Equal(got[31:37], []byte{0x01, 0x00, 0x01, 0x00, 0x01, 0x00}) {
		t.Fatalf("forest NOTI28 @31..36 = %x, want 010001000100", got[31:37])
	}
	if !bytes.Equal(got[37:41], []byte{0xff, 0xff, 0xff, 0xff}) {
		t.Fatalf("forest NOTI28 @37..40 = %x, want ffffffff", got[37:41])
	}
	if seed := binary.LittleEndian.Uint32(got[41:45]); seed != 0xc9e12c6e {
		t.Fatalf("forest NOTI28 seed @41 = %#x", seed)
	}
	// Extreme 的强度：除 id 与尾部动态值，其余必须与官服 Extreme 一致。
	gotExtreme := ForestDungeonInfo(0x05f5f0ef, 0, [2]byte{}, 0x5e5d467c)
	for i := 4; i < 45; i++ {
		if gotExtreme[i] != extreme[i] {
			t.Fatalf("Extreme forest NOTI28 @%d = %02x, want %02x", i, gotExtreme[i], extreme[i])
		}
	}
}

// NOTI27 的三种形态各自对应官服抓包里的头四字节；极难度两个阶段各用一份。
func TestEnterDungeonSelectionHeads(t *testing.T) {
	cold := EnterDungeonSelection()
	if cold[0] != 0 || cold[1] != 0 || cold[2] != 0 || cold[3] != 1 {
		t.Fatalf("cold NOTI27 head = % x, want 00 00 00 01", cold[:4])
	}
	seamless := EnterDungeonSelectionSeamless()
	if seamless[0] != 1 || seamless[1] != 0 || seamless[2] != 0 || seamless[3] != 1 {
		t.Fatalf("seamless NOTI27 head = % x, want the official extreme-stage-0 01 00 00 01", seamless[:4])
	}
	stage := EnterDungeonSelectionStageContinue()
	if stage[0] != 0 || stage[1] != 1 || stage[2] != 1 || stage[3] != 1 {
		t.Fatalf("stage-continue NOTI27 head = % x, want the official extreme-stage-1+ 00 01 01 01", stage[:4])
	}
	// 三种形态只在头部这几个字节上有别，本体必须同长同形。
	if len(cold) != len(seamless) || len(cold) != len(stage) {
		t.Fatalf("NOTI27 variants differ in length: %d/%d/%d", len(cold), len(seamless), len(stage))
	}
	if !bytes.Equal(cold[4:], seamless[4:]) || !bytes.Equal(cold[4:], stage[4:]) {
		t.Fatal("NOTI27 variants must share the same body after the header bytes")
	}
}

func mustHexBytes(t *testing.T, text string) []byte {
	t.Helper()
	out := make([]byte, 0, len(text)/2)
	for i := 0; i+1 < len(text); i += 2 {
		var v byte
		for j := 0; j < 2; j++ {
			c := text[i+j]
			switch {
			case c >= '0' && c <= '9':
				v = v<<4 | (c - '0')
			case c >= 'a' && c <= 'f':
				v = v<<4 | (c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				v = v<<4 | (c - 'A' + 10)
			default:
				t.Fatalf("bad hex %q", text)
			}
		}
		out = append(out, v)
	}
	return out
}
