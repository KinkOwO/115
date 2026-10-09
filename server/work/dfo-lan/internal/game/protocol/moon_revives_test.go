package protocol

import (
	"encoding/binary"
	"testing"
	"time"
)

// TestMoonLakeRevives115 守 N2622 里「剩余复活币次数」那一格（p[16]）。
//
// 背景（next188）：协议原注释就写着 "the host supplies the actual upstream-owned
// revival balance"，此前恒为 0 ⇒ 客户端看不到上限。上限来自副本脚本 [coin limit]（8）。
func TestMoonLakeRevives115(t *testing.T) {
	base := MoonLakeBootstrap115(2, time.Unix(1800000000, 0))
	if got := binary.LittleEndian.Uint32(base[16:]); got != 0 {
		t.Fatalf("基线 p[16] 应为 0，得到 %d", got)
	}

	out, err := MoonLakeRevives115(base, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(out[16:]); got != 7 {
		t.Fatalf("p[16] = %d，期望 7", got)
	}
	if got := binary.LittleEndian.Uint32(base[16:]); got != 0 {
		t.Fatalf("不应改动入参（基线被写成 %d）", got)
	}
	if len(out) != 126 {
		t.Fatalf("长度 = %d，期望 126", len(out))
	}

	// 关闭态（phase 14）那一格是 255 哨兵，不能被覆盖。
	closed := MoonLakeBootstrap115(14, time.Unix(1800000000, 0))
	kept, err := MoonLakeRevives115(closed, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(kept[16:]); got != 255 {
		t.Fatalf("phase 14 的 p[16] 应保持 255 哨兵，得到 %d", got)
	}
}
