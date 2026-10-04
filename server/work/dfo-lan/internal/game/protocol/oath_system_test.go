package protocol

import (
	"bytes"
	"testing"
)

// 2839 载荷几何（2026-10-04 补）：客户端解析器只读 15 字节，我们发 24 字节且只有
// `[0:4) = option` 是已确认字段。这条用例把"当前口径"钉住 —— 将来往 `[4:15)` 填
// 誓约积分时必须同时改这里，避免无声改协议。
func TestOathSystemInfoPayloadGeometry(t *testing.T) {
	body, err := OathSystemInfo(115, 2)
	if err != nil {
		t.Fatalf("OathSystemInfo: %v", err)
	}
	if len(body) != OathSystemInfoSize {
		t.Fatalf("payload = %d bytes, want %d", len(body), OathSystemInfoSize)
	}
	if OathSystemInfoSize < OathSystemInfoWireSize {
		t.Fatalf("发送长度 %d 小于客户端读取的 %d", OathSystemInfoSize, OathSystemInfoWireSize)
	}
	if got := int(body[0]); got != 2 {
		t.Fatalf("option = %d, want 2", got)
	}
	if !bytes.Equal(body[4:], make([]byte, OathSystemInfoSize-4)) {
		t.Fatalf("tail must stay zero until the oath-point fields are pinned: % x", body[4:])
	}
	// 等级门槛：<115 一律 0；>=115 且未选时给 1（既有实机口径）。
	if body, err := OathSystemInfo(114, 3); err != nil || body[0] != 0 {
		t.Fatalf("level<115 -> option 0, got %v err=%v", body, err)
	}
	if body, err := OathSystemInfo(115, 0); err != nil || body[0] != 1 {
		t.Fatalf("level>=115 option 0 -> 1, got %v err=%v", body, err)
	}
	if _, err := OathSystemInfo(115, 4); err == nil {
		t.Fatal("option 4 must be refused")
	}
}
