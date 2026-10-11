package main

import (
	"encoding/binary"
	"testing"
)

// TestConstructedActivityRowShapes 钉住**构造行**（战令 516/10005）的形状。
//
// 构造行没有官服字节可对照（抓包里没有这几个 id），所以只能守两件事：
//  ① 行长/字段顺序与官方行**完全一致**（用同一个解析器走一遍，且每条都必须恰好吃掉整行）；
//  ② 我们填进去的值就是打算填的那些（id/名/窗口/四个串/flag）。
// 「客户端认不认」这件事只能靠实机（`[event on]` 里出现该 id）—— 用例不假装能证明它。
func TestConstructedActivityRowShapes(t *testing.T) {
	rows := constructedActivityEventRows()
	if len(rows) == 0 {
		t.Fatal("构造行是空的")
	}
	want := map[uint16]struct {
		name       string
		start, end uint32
	}{
		aradPassEventID:     {"Arad Pass", aradPassEventStart, aradPassEventEnd},
		aradPassAltEventID:  {"Arad Pass", aradPassEventStart, aradPassEventEnd},
	}
	if len(want) != len(constructedActivityEventIDs) {
		t.Fatalf("期望表 %d 条与 constructedActivityEventIDs %d 条不一致",
			len(want), len(constructedActivityEventIDs))
	}
	o := 0
	u8 := func() byte { v := rows[o]; o++; return v }
	u16 := func() uint16 { v := binary.LittleEndian.Uint16(rows[o:]); o += 2; return v }
	u32 := func() uint32 { v := binary.LittleEndian.Uint32(rows[o:]); o += 4; return v }
	str := func() string { n := int(u32()); s := string(rows[o : o+n]); o += n; return s }

	seen := make(map[uint16]bool, len(want))
	for i := 0; i < len(constructedActivityEventIDs); i++ {
		id := u16()
		exp, ok := want[id]
		if !ok {
			t.Fatalf("第 %d 行的 id=%d 不在期望表里", i, id)
		}
		if seen[id] {
			t.Fatalf("id=%d 出现两次", id)
		}
		seen[id] = true
		if present := u8(); present != 1 {
			t.Fatalf("id=%d present=%d, want 1", id, present)
		}
		if a, b := u8(), u8(); a != 2 || b != 4 {
			t.Fatalf("id=%d a/b=%d/%d, want 2/4（与官服活动行同形）", id, a, b)
		}
		if name := str(); name != exp.name {
			t.Fatalf("id=%d name=%q, want %q", id, name, exp.name)
		}
		if s2 := str(); s2 != "" {
			t.Fatalf("id=%d s2=%q, want 空", id, s2)
		}
		if url := str(); url != "" {
			t.Fatalf("id=%d url=%q, want 空（带 url 的记录是选角屏崩溃风险面）", id, url)
		}
		if start, end := u32(), u32(); start != exp.start || end != exp.end {
			t.Fatalf("id=%d 窗口 %d~%d, want %d~%d", id, start, end, exp.start, exp.end)
		}
		if cal := str(); cal != "" {
			t.Fatalf("id=%d calendar=%q, want 空（探针阶段不引资源）", id, cal)
		}
		if extra := str(); extra != "" {
			t.Fatalf("id=%d extra=%q, want 空", id, extra)
		}
		if flag := u8(); flag != 1 {
			t.Fatalf("id=%d flag=%d, want 1", id, flag)
		}
	}
	if o != len(rows) {
		t.Fatalf("解析只吃掉 %d / %d 字节 —— 行长与官方行不一致", o, len(rows))
	}
	// 表头条数必须把构造行算进去。
	if got := activityEventInfoCount(); got != uint16(len(eventInfoActivityIDs)+len(constructedActivityEventIDs)) {
		t.Fatalf("activityEventInfoCount=%d", got)
	}
}
