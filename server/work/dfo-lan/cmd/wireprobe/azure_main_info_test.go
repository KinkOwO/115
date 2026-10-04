package main

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

// 官服蔚蓝号 N2621（AZURE_MAIN_INFO）的逐字节基线，直接取自
// E:/迅雷下载/20261003-214424/decoded/F16-s2c.txt（仅把 [116:121] 那个尚未
// 解出语义的 5B 值清零，本实现也留 0）。

// s2c #416：进本首帧 —— 帧序里紧跟 N28(#414)、在首张 N29(#417) 之前。
// 已清房间表 [36:68] 为空、倒计时 [9:11] = 3600。
const officialAzureMainInfoEntryHex = "020000000000000000100e000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

// s2c #488：房间 (1,0) 的最后一怪被确认打死之后、下一张 N29(#495) 之前。
// [36:68] 出现第一个 (x,y) = (1,0)，倒计时 3584（= 3600 - 16s）。
const officialAzureMainInfoFirstClearHex = "020000000000000000000e000000000000000000000000000000000000000000080000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

func decodeGolden(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 128 {
		t.Fatalf("N2621 golden must be 128 bytes, got %d", len(b))
	}
	return b
}

func assertBody(t *testing.T, got, want []byte, label string) {
	t.Helper()
	if bytes.Equal(got, want) {
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: byte %d got 0x%02x want 0x%02x", label, i, got[i], want[i])
		}
	}
}

func TestAzureMainInfoMatchesOfficialEntryFrame(t *testing.T) {
	want := decodeGolden(t, officialAzureMainInfoEntryHex)
	assertBody(t, azureMainInfoBody(azureMainPhasePlaying, 0, nil, 0, azureMainReviveLimit), want, "entry")
}

func TestAzureMainInfoMatchesOfficialFirstClearFrame(t *testing.T) {
	want := decodeGolden(t, officialAzureMainInfoFirstClearHex)
	assertBody(t, azureMainInfoBody(azureMainPhasePlaying, 16*time.Second, [][2]byte{{1, 0}}, 0, azureMainReviveLimit), want, "first clear")
}

// [36:68] 是客户端判定「本房间打完了、门可以走」的表：官服每清空一个有怪的房间就追加
// 一个 (x,y)，最多 4 个；0 怪的起始房与 (1,2) 不进去。
func TestAzureMainInfoClearedPairsAreCappedAtFour(t *testing.T) {
	body := azureMainInfoBody(azureMainPhasePlaying, 0, [][2]byte{{1, 0}, {1, 1}, {1, 3}, {2, 2}, {9, 9}}, 0, azureMainReviveLimit)
	want := []byte{1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 3, 0, 0, 0, 2, 0, 0, 0, 2, 0, 0, 0}
	if !bytes.Equal(body[36:68], want) {
		t.Fatalf("cleared pairs = %x, want %x", body[36:68], want)
	}
	if body[68] != 0 || body[69] != 0 || body[70] != 0 || body[71] != 0 {
		t.Fatal("the fifth room must be dropped, not written")
	}
}

func TestAzureMainInfoCountdownClamps(t *testing.T) {
	entry := azureMainInfoBody(azureMainPhasePlaying, 0, nil, 0, azureMainReviveLimit)
	if left := int(entry[9]) | int(entry[10])<<8; left != azureMainCountdownSeconds {
		t.Fatalf("entry countdown = %d, want %d", left, azureMainCountdownSeconds)
	}
	late := azureMainInfoBody(azureMainPhasePlaying, 4000*time.Second, nil, 0, azureMainReviveLimit)
	if left := int(late[9]) | int(late[10])<<8; left != 0 {
		t.Fatalf("exhausted countdown = %d, want 0 (must clamp, not wrap)", left)
	}
	if !bytes.Equal(entry[:9], late[:9]) || !bytes.Equal(entry[11:], late[11:]) {
		t.Fatal("only the countdown field may vary with elapsed time")
	}
}

func TestAzureMainInfoPacketShape(t *testing.T) {
	w := &worldSession{}
	p := w.azureMainInfo(time.Now())
	if p.Kind != 0 || p.ID != 2621 {
		t.Fatalf("N2621 must be a server NOTI: kind=%d id=%d", p.Kind, p.ID)
	}
	if len(p.Payload) != 128 {
		t.Fatalf("N2621 body must be 128 bytes, got %d", len(p.Payload))
	}
	// 未开启的 run（runStarted 零值）按 0 秒算，不该下溢成 65535。
	if left := int(p.Payload[9]) | int(p.Payload[10])<<8; left != azureMainCountdownSeconds {
		t.Fatalf("zero-value run countdown = %d, want %d", left, azureMainCountdownSeconds)
	}
}
