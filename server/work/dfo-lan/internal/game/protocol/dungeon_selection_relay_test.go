package protocol

import (
	"bytes"
	"testing"
)

// TestEnterDungeonSelectionRelayOnlyFlipsTheRelayByte 钉住 NOTI27 的「继续挑战」形态。
//
// 官服取证（analysis/tasks/next178 §3，2026-10-08 抓包）：同一个副本冷进场的 NOTI27 头字节
// `relay` 是 0x00，两次「继续挑战」（CMD72 选项 5）都是 0x01；两次之间其余字段逐帧相同。
// 本仓客户端在 0x145303307 单独读这一个字节（见 EnterDungeonSelection 的字段表）。
func TestEnterDungeonSelectionRelayOnlyFlipsTheRelayByte(t *testing.T) {
	plain := EnterDungeonSelection()
	relay := EnterDungeonSelectionRelay()

	// 36 字节是 native 读序的长度（EnterDungeonSelection 的注释：full 36-byte reader sequence）。
	if len(plain) != 36 {
		t.Fatalf("NOTI27 payload = %d bytes, want 36", len(plain))
	}
	if len(relay) != len(plain) {
		t.Fatalf("relay 形态长度 %d != %d", len(relay), len(plain))
	}
	// 头 4 字节必须与官服逐字节一致（next178 §18）：
	//   冷进   = 00 00 00 01（u32@3 = 1）
	//   继续   = 00 01 c3 01（relay=01、relay-extra=c3、u32@3 仍是 1）
	plainHead := []byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	relayHead := []byte{0x00, 0x01, 0xc3, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if !bytes.Equal(plain[:12], plainHead) {
		t.Fatalf("普通进本头 12 字节 = % x, want % x", plain[:12], plainHead)
	}
	if !bytes.Equal(relay[:12], relayHead) {
		t.Fatalf("继续挑战头 12 字节 = % x, want % x", relay[:12], relayHead)
	}
	// 只有 relay / relay-extra 这两个头字节变：其余位移都被客户端逐个读，顺手改动就是动别的字段。
	for i := range plain {
		if plain[i] != relay[i] && i != 1 && i != 2 {
			t.Fatalf("offset %d 也被改了：%#x -> %#x", i, plain[i], relay[i])
		}
	}
}
