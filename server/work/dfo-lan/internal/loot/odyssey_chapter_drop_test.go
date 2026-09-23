package loot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func chapterDropFixture(t *testing.T) *OdysseyChapterDrop {
	t.Helper()
	d, e := LoadOdysseyChapterDrop("../../configs/odyssey-chapter-drop-release.json")
	if e != nil {
		t.Fatal(e)
	}
	return d
}

// 出厂整表关闭：每章一条，第 1..6 章是自选盒，第 7 章 template=0（源里两盒都是普通 booster）。
func TestOdysseyChapterDropShipsDisabled(t *testing.T) {
	d := chapterDropFixture(t)
	if len(d.Drops) != 7 {
		t.Fatalf("drop 行数 %d，期望 7", len(d.Drops))
	}
	if d.Enabled() {
		t.Fatal("出厂就不该是启用状态")
	}
	for _, line := range d.Drops {
		if line.Enabled {
			t.Fatalf("第%d章出厂就是启用状态: %+v", line.Chapter, line)
		}
	}
	// 第 1..6 章各有自选盒，第 7 章没有
	for n := uint8(1); n <= 6; n++ {
		line, ok := d.DropFor(finalOf(t, d, n))
		if !ok || line.Template == 0 {
			t.Fatalf("第%d章缺少章节盒: %+v", n, line)
		}
	}
	line, ok := d.DropFor(finalOf(t, d, 7))
	if !ok || line.Template != 0 {
		t.Fatalf("第7章不该有章节盒: %+v", line)
	}
	t.Logf("7 章中 6 章有章节盒；第 7 章 template=0；出厂 enabled=false")
}

func finalOf(t *testing.T, d *OdysseyChapterDrop, number uint8) uint32 {
	t.Helper()
	for _, line := range d.Drops {
		if line.Chapter == number {
			return line.Final
		}
	}
	t.Fatalf("没有第%d章", number)
	return 0
}

// 禁用行必须完全惰性：连掷骰种子都不消耗，否则将来开启会挪动其它掉落。
func TestOdysseyChapterDropDisabledIsInert(t *testing.T) {
	d := chapterDropFixture(t)
	for _, seed := range []uint32{0, 1, 12345, 0xffffffff} {
		for _, line := range d.Drops {
			got, next, e := d.Roll(seed, line.Final)
			if e != nil || len(got) != 0 || next != seed {
				t.Fatalf("禁用行不惰性: seed=%d 章=%d got=%v next=%d err=%v", seed, line.Chapter, got, next, e)
			}
		}
		// 非 final 的副本也必须惰性
		if got, next, e := d.Roll(seed, 1); e != nil || len(got) != 0 || next != seed {
			t.Fatal("非 final 副本不该掷骰", got, next, e)
		}
	}
}

// 启用后必须真的掷骰，而且只在该章 final 上；掉率 100% 时必掉。
func TestOdysseyChapterDropEnabledRolls(t *testing.T) {
	d := chapterDropFixture(t)
	final := finalOf(t, d, 1)
	for i := range d.Drops {
		d.Drops[i].Enabled = true
		d.Drops[i].Rate = 10000
	}
	d.byDungeon = map[uint32]ChapterDropLine{}
	for _, line := range d.Drops {
		d.byDungeon[line.Final] = line
	}
	if !d.Enabled() {
		t.Fatal("启用后 Enabled() 仍为 false")
	}
	got, next, e := d.Roll(42, final)
	if e != nil || len(got) != 1 || got[0].Template != 10417792 || got[0].Amount != 1 {
		t.Fatalf("第1章最终领主必掉章节盒: got=%v err=%v", got, e)
	}
	if next == 42 {
		t.Fatal("启用的行必须消耗种子")
	}
	// 非 final 仍然惰性
	if got, next2, e := d.Roll(42, 100004934); e != nil || len(got) != 0 || next2 != 42 {
		t.Fatal("非 final 副本在启用后仍不该掷骰")
	}
	// 掉率 0 的启用行等价于关
	for i := range d.Drops {
		d.Drops[i].Rate = 0
	}
	d.byDungeon = map[uint32]ChapterDropLine{}
	for _, line := range d.Drops {
		d.byDungeon[line.Final] = line
	}
	if got, next, e := d.Roll(42, final); e != nil || len(got) != 0 || next != 42 {
		t.Fatal("rate=0 的启用行不该掷骰", got, next, e)
	}
}

// 装载必须拒绝形状不符的表（少一行 / enabled 却没有模板或掉率）。
func TestOdysseyChapterDropRejectsBadTables(t *testing.T) {
	base := chapterDropFixture(t)
	dir := t.TempDir()

	write := func(mutate func(m map[string]any)) string {
		raw, e := os.ReadFile("../../configs/odyssey-chapter-drop-release.json")
		if e != nil {
			t.Fatal(e)
		}
		var m map[string]any
		if e = json.Unmarshal(raw, &m); e != nil {
			t.Fatal(e)
		}
		mutate(m)
		b, e := json.Marshal(m)
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(dir, "t.json")
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		return p
	}

	// 少一章
	if _, e := LoadOdysseyChapterDrop(write(func(m map[string]any) {
		m["drops"] = m["drops"].([]any)[:6]
	})); e == nil {
		t.Fatal("6 行的表被接受")
	}
	// enabled 但没有模板
	if _, e := LoadOdysseyChapterDrop(write(func(m map[string]any) {
		d := m["drops"].([]any)[0].(map[string]any)
		d["enabled"] = true
		d["template"] = 0
	})); e == nil {
		t.Fatal("enabled 但 template=0 被接受")
	}
	// enabled 但掉率为 0
	if _, e := LoadOdysseyChapterDrop(write(func(m map[string]any) {
		d := m["drops"].([]any)[0].(map[string]any)
		d["enabled"] = true
		d["template"] = 10417792
		d["rate"] = 0
	})); e == nil {
		t.Fatal("enabled 但 rate=0 被接受")
	}
	// 校验和不对
	if _, e := LoadOdysseyChapterDrop(write(func(m map[string]any) {
		m["definition_sha256"] = "deadbeef"
	})); e == nil {
		t.Fatal("校验和不对的表被接受")
	}
	if base.Drops[0].Final == 0 {
		t.Fatal("fixture 异常")
	}
	t.Log("6 行 / enabled 无模板 / enabled rate=0 / 校验和错 四种坏表均被拒")
}
