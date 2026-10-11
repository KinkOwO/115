package character

import "testing"

// 巴卡尔频道门禁与伊斯同族：客户端读角色资料 28 字节内容解锁区的第 12 字节
// （伊斯是第 11，已由 13763 实证）。官服角色列表样本：完成 21019「小小的齿轮」
// 的角色该位为 1，未过伊斯的角色 [5..9] 之后全为 0；缺位时客户端弹「需 110 级
// 且完成小小的齿轮」并自动打回原频道（2026-10-05 实机 trace）。
func TestBakalChannelGateUsesSmallCogwheelQuest(t *testing.T) {
	for _, ids := range [][]uint16{nil, {13763}, {21018}} {
		if f := contentClearFlagsForQuests(ids); f[12] != 0 {
			t.Fatal("unfinished prerequisite granted entry", ids)
		}
	}
	f := contentClearFlagsForQuests([]uint16{21019})
	for i, v := range f {
		want := byte(0)
		if i == 12 {
			want = 1
		}
		if v != want {
			t.Fatalf("flag%d=%d want%d", i, v, want)
		}
	}
	all := contentClearFlagsForQuests([]uint16{12167, 12312, 12392, 12422, 13763, 21019})
	for _, i := range []int{5, 6, 8, 9, 11, 12} {
		if all[i] != 1 {
			t.Fatalf("gate%d lost", i)
		}
	}
}
