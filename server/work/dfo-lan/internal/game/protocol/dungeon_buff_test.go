package protocol

import "testing"

// TestCharacterBuffDungeonMatchesBothOfficialCaptures 钉住 NOTI475 的官服字节。
//
// 两处**独立**官服抓包给出同一串 0000a51f257d3f00：
//   - 小深渊 official_20261008-012831（3 次）
//   - 伊斯大陆 replay 夹具 internal/legion/ispins_replay_frames.generated.go
//
// 各字段语义尚未取证，所以这里只钉「原样发官服那 8 字节」，不解释、不构造。
func TestCharacterBuffDungeonMatchesBothOfficialCaptures(t *testing.T) {
	got := CharacterBuffDungeon()
	want := []byte{0x00, 0x00, 0xa5, 0x1f, 0x25, 0x7d, 0x3f, 0x00}
	if len(got) != len(want) {
		t.Fatalf("payload = %d bytes, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d = %#x, want %#x（% x）", i, got[i], want[i], want)
		}
	}
	// 调用方拿到的是副本：改它不该污染下一次发送。
	got[0] = 0xff
	if again := CharacterBuffDungeon(); again[0] != 0x00 {
		t.Fatal("CharacterBuffDungeon 返回了共享底层数组")
	}
}
