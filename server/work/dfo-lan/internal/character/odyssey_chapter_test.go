package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"

	"testing"
)

// 章节日志的真源形状：7 章 / 50 副本 / 各章 Final 是最后一个副本 / 无跨章重复。
func TestOdysseyChapterJournalShape(t *testing.T) {
	ch, e := catalog.LoadOdysseyChapters("../../configs/odyssey-chapters-release.json")
	if e != nil {
		t.Fatal(e)
	}
	if len(ch.Chapters) != 7 {
		t.Fatalf("章节数 %d，期望 7", len(ch.Chapters))
	}
	total := 0
	for _, c := range ch.Chapters {
		total += len(c.Dungeons)
		if c.Final != c.Dungeons[len(c.Dungeons)-1] {
			t.Fatalf("第%d章 final %d 不是最后一个副本 %d", c.Number, c.Final, c.Dungeons[len(c.Dungeons)-1])
		}
		for _, id := range c.Dungeons {
			if n, ok := ch.ChapterOf(id); !ok || n != c.Number {
				t.Fatalf("副本 %d 的章节归属错误: %d/%v", id, n, ok)
			}
		}
	}
	if total != 50 {
		t.Fatalf("副本总数 %d，期望 50", total)
	}
	// 前 6 章的最终领主是章节盒所在章的 Final；第 7 章没有章节盒。
	if !ch.IsFinal(100004945) || !ch.IsFinal(100004953) || !ch.IsFinal(100004990) {
		t.Fatal("Final 判定错误")
	}
	if ch.IsFinal(100004934) {
		t.Fatal("非最终领主被当成 final")
	}
	if n, ok := ch.ChapterOf(100004934); !ok || n != 1 {
		t.Fatal("第 1 章首个副本归属错误")
	}
	// 第 5 章的剧情顺序不是 id 升序，Final 必须跟着源顺序
	ch5, _ := ch.At(5)
	if ch5.Dungeons[0] != 100004968 || ch5.Final != 100004972 {
		t.Fatalf("第 5 章顺序被破坏: %v", ch5.Dungeons)
	}
	t.Logf("7 章 / %d 副本 / 15 模板；第 5 章源顺序 %v", total, ch5.Dungeons)
}

func odysseyChapterFixture(t *testing.T) (*ProgressionService, Character) {
	t.Helper()
	s, r := odysseyGrowthFixture(t)
	ch, e := catalog.LoadOdysseyChapters("../../configs/odyssey-chapters-release.json")
	if e != nil {
		t.Fatal(e)
	}
	s.Chapters = ch
	return s, r
}

// 逐行发放：数量为 2 的行按数量发放（Ch4 10419743 x2、Ch5 10419744 x2）。
func TestOdysseyChapterRewardGrant(t *testing.T) {
	s, r := odysseyChapterFixture(t)

	ch5, _ := s.Chapters.At(5)
	raw, receipt, e := s.ApplyOdysseyChapterReward(r, 5, 0, ch5.Rewards[0])
	if e != nil {
		t.Fatal(e)
	}
	bag, e := inventory.ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(bag.Items) != 1 || bag.Items[0].Template != 10419744 || bag.Items[0].Amount != 2 {
		t.Fatalf("Ch5 首行应为 10419744 x2 一格: %+v", bag.Items)
	}
	if len(receipt) == 0 {
		t.Fatal("receipt 为空")
	}

	// 普通角色（非奥德赛）拒绝
	normal := r
	normal.Request = nil
	if _, _, e := s.ApplyOdysseyChapterReward(normal, 5, 0, ch5.Rewards[0]); e == nil {
		t.Fatal("普通角色也发了章节奖励")
	}
	// 行号越界 / 章号不存在 / 行内容不匹配都拒绝
	if _, _, e := s.ApplyOdysseyChapterReward(r, 5, 99, ch5.Rewards[0]); e == nil {
		t.Fatal("行号越界被接受")
	}
	if _, _, e := s.ApplyOdysseyChapterReward(r, 8, 0, ch5.Rewards[0]); e == nil {
		t.Fatal("不存在的章被接受")
	}
	if _, _, e := s.ApplyOdysseyChapterReward(r, 5, 0, catalog.ChapterReward{Template: 999999, Count: 1}); e == nil {
		t.Fatal("与日志不符的行被接受")
	}
	if _, _, e := s.ApplyOdysseyChapterReward(r, 5, 0, ch5.Rewards[1]); e == nil {
		t.Fatal("行号与内容错配被接受")
	}
	t.Log("Ch5 首行按数量发 2 个；非奥德赛角色、越界行号、错配内容均被拒")
}
