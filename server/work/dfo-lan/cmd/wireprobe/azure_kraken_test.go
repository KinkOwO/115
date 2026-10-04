package main

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

// [AZURE-KRAKEN-ECHO] N2621 的 [22:26] 是客户端 CMD2274 上报的海怪/弱点 id 的**回显**。
//
// 官服那一趟：c2s #426 上报 `e2797f06 0b1dee07 …`（第 2 个 u32 = 0x07ee1d0b），
// 随后 s2c #525 的 N2621 `[22:26]` 就出现同一个 0x07ee1d0b —— 服务端把它带回去了。
// 私服此前没接 CMD2274，客户端上报后拿不到回显 ⇒ 弱点状态错乱，
// 「打弱点反而把自己打死」（业主实机 2026-10-04）。
//
// 这条 golden 直接取自 F16-s2c.txt 的 s2c #525（只把尚未解出语义的 [116:121] 清零）。
const officialAzureMainInfoKrakenHex = "020000000000000000d60d00000000000000000000000b1dee07000000000000080000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

func TestAzureMainInfoEchoesKrakenID(t *testing.T) {
	want, err := hex.DecodeString(officialAzureMainInfoKrakenHex)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != 128 {
		t.Fatalf("golden must be 128 bytes, got %d", len(want))
	}
	// 官服 #525：阶段 2、倒计时 0x0dd6（= 已进行 58 秒）、已清房间 [(1,0)]、海怪 0x07ee1d0b。
	got := azureMainInfoBody(azureMainPhasePlaying, 58*time.Second, [][2]byte{{1, 0}}, 0x07ee1d0b, azureMainReviveLimit)
	if !bytes.Equal(got, want) {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("N2621 kraken frame byte %d: got 0x%02x want 0x%02x", i, got[i], want[i])
			}
		}
	}
	// 没有上报过海怪时，[22:26] 必须是 0（进本那一帧官服也是 0）。
	noKraken := azureMainInfoBody(azureMainPhasePlaying, 0, nil, 0, azureMainReviveLimit)
	if u32 := int(noKraken[22]) | int(noKraken[23])<<8 | int(noKraken[24])<<16 | int(noKraken[25])<<24; u32 != 0 {
		t.Fatalf("[22:26] = %d, want 0 before any CMD2274", u32)
	}
}
