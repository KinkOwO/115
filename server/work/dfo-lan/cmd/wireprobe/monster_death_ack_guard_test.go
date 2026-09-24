package main

import "testing"

// NOTI38 的确认体是裸小端 u16。短包必须被拒并上报，而不是裸下标越界 ——
// 同一个失效模式（宽度只由别处保证）已经让网关崩过一次。
func TestMonsterDeathEntityGuardsShortPayload(t *testing.T) {
	for _, p := range [][]byte{nil, {}, {0x15}} {
		if _, ok := monsterDeathEntity(p); ok {
			t.Fatalf("short payload accepted: %v", p)
		}
	}
	if e, ok := monsterDeathEntity([]byte{0x15, 0x10}); !ok || e != 0x1015 {
		t.Fatalf("two-byte entity read wrong: %x %v", e, ok)
	}
	// 实机里这一族的 id 会带尾随字节；只取前两个。
	if e, ok := monsterDeathEntity([]byte{0x1a, 0x10, 9, 9, 9, 9, 9, 9}); !ok || e != 0x101a {
		t.Fatalf("entity read from a padded payload: %x %v", e, ok)
	}
}
