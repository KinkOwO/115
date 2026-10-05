package protocol

import (
	"encoding/hex"
	"testing"
)

// TestEventRequest115LiveWidths 钉住两种实机/夹具正文宽度：
// 662 的 680/681 是两包（事件号 + 关卡号），665 是 A 类三包。
// 8 字节那帧来自实机 2026-10-04 18:04:04（第一关对话后点「Get」）。
func TestEventRequest115LiveWidths(t *testing.T) {
	p, e := hex.DecodeString("9602000001000000")
	if e != nil || len(p) != 8 {
		t.Fatal(len(p), e)
	}
	r, e := DecodeEventRequest115(p, true)
	if e != nil || r.Event != 662 || r.Parameter != 1 || !r.HasParameter {
		t.Fatal(r, e)
	}
	if _, e := DecodeEventRequest115(p[:4], true); e == nil {
		t.Fatal("one-word body accepted")
	}
	wide := make([]byte, 12)
	live := append([]byte{}, p...)
	wide[0] = 0x99
	wide[1] = 0x02 // 事件号 665
	if q, e := DecodeEventRequest115(wide, false); e != nil || q.Event != 665 || q.Parameter != 0 || !q.HasParameter {
		t.Fatal(q, e)
	}
	// 三包正文仍按第三包取参数，两包口径不得回吞 665。
	three := append(live, 0, 0, 0, 0)
	three[8] = 7
	if q, e := DecodeEventRequest115(three, true); e != nil || q.Parameter != 7 {
		t.Fatal(q, e)
	}
}
