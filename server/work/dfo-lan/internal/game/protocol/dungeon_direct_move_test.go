package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// 2026-09-21 实机（角色 test-jh，session roles_..._20260921_225338_199523_next37）：
// 清关后走进"下一个剧情关卡"门时，客户端发出的 CMD 2062
// （ENUM_CMDPACKET_DUNGEON_DIRECT_MOVE）明文，48 字节。两次的目标副本分别是
// 100004984 与 100004947（= 当前副本 100004946 的下一关）。
const (
	directMoveGate100004984 = "cf00000000000000ffffffffff78f4f50502000000000000000100000067000000cd000000140000000a000000000000"
	directMoveGate100004947 = "4e01000000000000ffffffffff53f4f505020000000000000001000000a0000000c8000000140000000a000000000000"
)

func TestDungeonDirectMoveCapturedBodies(t *testing.T) {
	for _, tc := range []struct {
		body       string
		id         uint32
		difficulty byte
		gate       [4]uint32
	}{
		{directMoveGate100004984, 100004984, 2, [4]uint32{103, 205, 20, 10}},
		{directMoveGate100004947, 100004947, 2, [4]uint32{160, 200, 20, 10}},
	} {
		raw, e := hex.DecodeString(tc.body)
		if e != nil {
			t.Fatal(e)
		}
		r, e := DecodeDungeonDirectMove(raw)
		if e != nil {
			t.Fatal(e)
		}
		if r.ID != tc.id {
			t.Fatalf("dungeon id: got %d want %d", r.ID, tc.id)
		}
		if r.Difficulty != tc.difficulty {
			t.Fatalf("difficulty: got %d want %d", r.Difficulty, tc.difficulty)
		}
		if r.Gate != tc.gate {
			t.Fatalf("gate: got %v want %v", r.Gate, tc.gate)
		}
		if !bytes.Equal(r.Record[:], raw) {
			t.Fatal("raw record not preserved")
		}
	}
}

func TestDungeonDirectMoveRejectsOtherLengths(t *testing.T) {
	for _, size := range []int{0, 32, 47, 49} {
		if _, e := DecodeDungeonDirectMove(make([]byte, size)); e == nil {
			t.Fatalf("%d-byte body accepted", size)
		}
	}
}
