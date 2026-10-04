package protocol

import (
	"encoding/hex"
	"testing"
)

// N537（DUNGEON_ENTER_COUNT_INFO）的载荷几何：u32 副本 id + 两个相同字节的计数。
//
// 钉住蔚蓝号那次修复：**照抄官服「进本后」的 0 会被客户端判为「本周入场次数已用完」**
// （DSTR 100088500，sub_14165B950 的第四道门），必须发 >0。
// 面板上「Weekly Entry Count: x/1」的分母 1 说明该值域就是 0..1。
func TestDungeonTestRemainingShape115(t *testing.T) {
	p, err := DungeonTestRemaining(100004131, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(p); got != "23f1f5050101" {
		t.Fatalf("100004131 remaining=1 -> %s, want 23f1f5050101", got)
	}
	// 0 是本次踩到的坑：函数明确拒绝，调用方必须给正值。
	if _, err := DungeonTestRemaining(100004131, 0); err == nil {
		t.Fatal("configured=0 必须报错（照抄官服进本后的 0 正是那次实机卡点）")
	}
	// 副本 id 为 0 同样拒绝。
	if _, err := DungeonTestRemaining(0, 1); err == nil {
		t.Fatal("dungeon=0 必须报错")
	}
	// 上限截断到 u8。
	p, err = DungeonTestRemaining(100004131, 300)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(p[4:]); got != "ffff" {
		t.Fatalf("300 应截断为 255，得到 %s", got)
	}
}
