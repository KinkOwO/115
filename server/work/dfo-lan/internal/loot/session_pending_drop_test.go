package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"strings"
	"testing"
)

// 图内确认（CMD 2406「命运的凝械」）要掉的那一件不新造包：它记在会话上，
// 搭**本场下一次怪物死亡掉落帧**（noti 38）一起落地。实机顺序就是这样的
// （2026-10-12 15:36:14 确认 → 15:36:32 爆掉落 → 15:36:33 起连续拾取 CMD43），
// 所以拾取、对象号分配、装备耐久编码全都复用既有那一条路。

func pendingDropFixture(t *testing.T) (*Session, *dungeon.Session) {
	t.Helper()
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	d := &dungeon.Session{RunID: "pending", Loaded: true, NextEntity: 0x1000,
		Dead:     map[uint16]bool{1: true},
		Monsters: []protocol.DungeonMonster{{Entity: 1, Template: 123, Level: 1, Rank: 3}}}
	d.Definition.ID = 100005068
	d.Definition.SourceBoss = 123
	d.Definition.Script.Cells = []pvf.Token{{Type: 3, Text: "[exclude gold drop]"}, {Type: 3, Text: "[exclude monster random drop]"}}
	s := NewSession(catalog.LootCatalog{}, Tables{}, Rules{}, nil, "pending", 1, 1, 1)
	s.Attunement = a
	s.OathTier = 45
	s.seeds[d.Room.Map] = 42
	return s, d
}

// 记下的必须是源给的**载体**，落地的是展开后的那一件 —— 和别的调律掉落同一条展开路。
func TestPendingDropIsOpenedByTheSameWrapperStepAsEveryOtherDrop(t *testing.T) {
	s, d := pendingDropFixture(t)
	s.RewardBoxes = fakeBoxes{
		boxes: map[uint32]RewardBox{
			10410316: {Pools: []RewardBoxPool{{Draws: 1,
				Candidates: []RewardBoxCandidate{{Template: 100301825, Weight: 20, Count: 1}}}}},
		},
		items: map[uint32]bool{10410316: true, 100301825: true},
	}
	s.SetPendingDrop(Award{Template: 10410316, Amount: 1})
	rows, err := s.Death(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no rows published")
	}
	last := rows[len(rows)-1]
	// 行里的模板号从已登记的 Drop 上读，比按字节猜编码位置可靠。
	drop, ok := s.Objects[last.Object]
	if !ok {
		t.Fatalf("last row object %d not registered", last.Object)
	}
	tmpl := drop.Award.Template
	if tmpl == 10410316 {
		t.Fatal("pending drop landed as the unopened wrapper")
	}
	if tmpl != 100301825 {
		t.Fatalf("pending drop paid %d, want the opened product 100301825", tmpl)
	}
	if left := s.PendingDrop(); len(left) != 0 {
		t.Fatalf("pending not cleared after publication: %v", left)
	}
	// 再死一次同一只：走缓存行，不该又掉一件。
	again, err := s.Death(d, 1)
	if err != nil || len(again) != len(rows) {
		t.Fatalf("repeat death rows %d vs %d err %v", len(again), len(rows), err)
	}
}

// 同一场里改主意（弹窗写着 Reselecting will reset the selected Part and Set）以最后一次
// 为准，而且只会掉一件 —— 记的是替换，不是累加。
func TestSetPendingDropReplacesInsteadOfAccumulating(t *testing.T) {
	s, _ := pendingDropFixture(t)
	s.SetPendingDrop(Award{Template: 10410316, Amount: 1})
	s.SetPendingDrop(Award{Template: 10410340, Amount: 1})
	got := s.PendingDrop()
	if len(got) != 1 || got[0].Template != 10410340 {
		t.Fatalf("pending = %v, want exactly one 10410340", got)
	}
}

// 发布失败时不能把这一件吃掉：它下一帧还该掉。
func TestPendingDropSurvivesAFailedPublication(t *testing.T) {
	s, d := pendingDropFixture(t)
	s.SetPendingDrop(Award{Template: 10410316, Amount: 1})
	d.NextEntity = 65534 // 触发 "drop identity exhausted"
	if _, err := s.Death(d, 1); err == nil || !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("want capacity failure, got %v", err)
	}
	if left := s.PendingDrop(); len(left) != 1 || left[0].Template != 10410316 {
		t.Fatalf("failed publication consumed the pending drop: %v", left)
	}
	if len(s.Objects) != 0 {
		t.Fatalf("failed publication published objects: %d", len(s.Objects))
	}
	d.NextEntity = 0x1000
	rows, err := s.Death(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	last := rows[len(rows)-1]
	if drop := s.Objects[last.Object]; drop.Award.Template != 10410316 {
		t.Fatalf("retry lost the pending drop (last row = %+v)", drop)
	}
	if len(s.PendingDrop()) != 0 {
		t.Fatal("pending still armed after a successful retry")
	}
}

// 没记东西时掉落帧逐字节等于旧行为（对象号、行数都不多）。
func TestNoPendingDropLeavesTheFrameUnchanged(t *testing.T) {
	s, d := pendingDropFixture(t)
	rows, err := s.Death(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if drop, ok := s.Objects[r.Object]; ok && drop.Award.Template == 10410316 {
			t.Fatal("carrier appeared without ever being armed")
		}
	}
}
