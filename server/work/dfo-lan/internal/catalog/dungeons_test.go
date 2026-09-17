package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func section(text string) pvf.Token { return pvf.Token{Type: 3, Text: text} }
func number(v int32) pvf.Token      { return pvf.Token{Type: 0, Value: v} }
func label(text string) pvf.Token   { return pvf.Token{Type: 6, Text: text} }

// parseOneMaze 用最小可解析脚本（含必填的等级段）解析出一张 maze。
func parseOneMaze(t *testing.T, body ...pvf.Token) DungeonMaze {
	t.Helper()
	cells := append([]pvf.Token{
		section("[minimum required level]"), number(1),
		section("[basis level]"), number(1),
	}, body...)
	d, e := ParseDungeon(1, ScriptRecord{Path: "test.dgn", Cells: cells})
	if e != nil {
		t.Fatal(e)
	}
	if len(d.Mazes) != 1 {
		t.Fatalf("want one maze, got %d", len(d.Mazes))
	}
	return d.Mazes[0]
}

// 一个房间节点可以在坐标后面列多张地图（实测 dungeon 86 的 boss 房有 4 张）。
// 旧解析器按固定 4 格分组，遇到这种写法 len%4!=0，把整张 maze 判成
// unsupported room specification、rooms 归零。
func TestParseDungeonKeepsMultiMapNode(t *testing.T) {
	m := parseOneMaze(t,
		section("[maze info]"),
		section("[size]"), number(9), number(1),
		section("[map specification]"),
		label("boss"), number(0), number(0), number(20314), number(20315), number(20316), number(20317),
		label("map"), number(1), number(0), number(20302),
		section("[/map specification]"),
		section("[start map]"), number(6), number(0), section("[/start map]"),
		section("[boss map]"), number(0), number(0), section("[/boss map]"),
	)
	if len(m.Pending) != 0 {
		t.Fatalf("unexpected pending: %v", m.Pending)
	}
	if len(m.Rooms) != 2 {
		t.Fatalf("rooms=%d want 2", len(m.Rooms))
	}
	boss := m.Rooms[0]
	if len(boss.Alternates) != 3 || boss.Map != 20314 {
		t.Fatalf("multi-map node lost candidates: %+v", boss)
	}
	if !boss.Boss || m.Rooms[1].Boss {
		t.Fatalf("boss flag wrong: %+v", m.Rooms)
	}
	if m.Start != [2]byte{6, 0} || m.Boss != [2]byte{0, 0} {
		t.Fatalf("start/boss wrong: %v %v", m.Start, m.Boss)
	}
}

// [start map] 可以列多组候选坐标（实测 dungeon 92 是 (1,4) 和 (3,4)）。
func TestParseDungeonAcceptsMultipleStartPairs(t *testing.T) {
	m := parseOneMaze(t,
		section("[maze info]"),
		section("[size]"), number(4), number(5),
		section("[map specification]"),
		label("map"), number(1), number(4), number(95149),
		label("boss"), number(0), number(1), number(95164),
		section("[/map specification]"),
		section("[start map]"), number(1), number(4), number(3), number(4), section("[/start map]"),
		section("[boss map]"), number(0), number(1), section("[/boss map]"),
	)
	if len(m.Pending) != 0 {
		t.Fatalf("unexpected pending: %v", m.Pending)
	}
	if m.Start != [2]byte{1, 4} {
		t.Fatalf("start=%v want {1 4}", m.Start)
	}
}

// [boss map] 的 -1 -1 表示"没有 boss 房"，不是解析失败（实测 dungeon 20000）。
func TestParseDungeonTreatsMinusOneBossAsAbsent(t *testing.T) {
	m := parseOneMaze(t,
		section("[maze info]"),
		section("[size]"), number(1), number(1),
		section("[map specification]"),
		label("map"), number(0), number(0), number(80512),
		section("[/map specification]"),
		section("[start map]"), number(0), number(0), section("[/start map]"),
		section("[boss map]"), number(-1), number(-1), section("[/boss map]"),
	)
	for _, p := range m.Pending {
		if p == "[boss map]: invalid coordinate" || p == "[boss map]: coordinate pair required" {
			t.Fatalf("absent boss treated as failure: %v", m.Pending)
		}
	}
	if m.Boss != absentCoordinate {
		t.Fatalf("boss=%v want %v", m.Boss, absentCoordinate)
	}
}

// boss_selection_probability 是 (地图 ID, 权重) 交替的随机 BOSS 房
// （实测 contents/2024/snk/dungeon/arcade/farming_1.dgn）。
func TestParseDungeonReadsBossSelectionProbability(t *testing.T) {
	m := parseOneMaze(t,
		section("[maze info]"),
		section("[size]"), number(6), number(1),
		section("[map specification]"),
		label("map"), number(0), number(0), number(100011700),
		label("boss_selection_probability"), number(5), number(0),
		number(100011720), number(33), number(100011789), number(33), number(100011790), number(34),
		section("[/map specification]"),
		section("[start map]"), number(0), number(0), section("[/start map]"),
		section("[boss map]"), number(5), number(0), section("[/boss map]"),
	)
	if len(m.Pending) != 0 {
		t.Fatalf("unexpected pending: %v", m.Pending)
	}
	if len(m.Rooms) != 2 {
		t.Fatalf("rooms=%d want 2", len(m.Rooms))
	}
	boss := m.Rooms[1]
	if !boss.Boss || boss.Map != 100011720 || len(boss.Alternates) != 2 {
		t.Fatalf("weighted boss room wrong: %+v", boss)
	}
}

// [quest connection] 的第三格是任务链接标记，实测有 -1 和 2 两种取值；
// 旧解析器只认 -1，把带 2 的完整 maze 判成 unsupported，Quest 留在 0，
// 反而制造出第二张 quest==0 的 maze（实测 dungeon 86 index5）。
func TestParseDungeonKeepsQuestConnectionFlag(t *testing.T) {
	m := parseOneMaze(t,
		section("[maze info]"),
		section("[quest connection]"), number(0), number(4906), number(2),
		section("[size]"), number(9), number(1),
		section("[map specification]"),
		label("map"), number(0), number(0), number(20314),
		section("[/map specification]"),
		section("[start map]"), number(0), number(0), section("[/start map]"),
		section("[boss map]"), number(0), number(0), section("[/boss map]"),
	)
	if len(m.Pending) != 0 {
		t.Fatalf("unexpected pending: %v", m.Pending)
	}
	if m.Quest != 4906 || m.QuestFlag != 2 {
		t.Fatalf("quest=%d flag=%d want 4906/2", m.Quest, m.QuestFlag)
	}
}

// 位置不认识的标签仍然要报错，不能悄悄当成房间。
func TestParseDungeonRejectsUnknownNodeLabel(t *testing.T) {
	m := parseOneMaze(t,
		section("[maze info]"),
		section("[size]"), number(2), number(1),
		section("[map specification]"),
		label("unknown node"), number(0), number(0), number(20314),
		section("[/map specification]"),
		section("[start map]"), number(0), number(0), section("[/start map]"),
		section("[boss map]"), number(0), number(0), section("[/boss map]"),
	)
	if len(m.Pending) == 0 {
		t.Fatal("unknown node label accepted")
	}
}
