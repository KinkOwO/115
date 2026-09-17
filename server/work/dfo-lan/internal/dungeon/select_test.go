package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"strings"
	"testing"
)

// protocolSelection 是普通进城选图请求：Party 65535 表示单人。
func protocolSelection(id, quest uint32) protocol.DungeonSelection {
	return protocol.DungeonSelection{ID: id, Party: 65535, Quest: quest}
}

func testCatalog(mazes ...catalog.DungeonMaze) catalog.DungeonCatalog {
	mapScript := func() catalog.ScriptRecord {
		return catalog.ScriptRecord{Path: "map.test", Cells: []pvf.Token{{Type: 3, Text: "[monster]"}}}
	}
	return catalog.DungeonCatalog{
		Source:   pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)},
		Dungeons: map[uint32]catalog.DungeonDefinition{7: {ID: 7, MinimumLevel: 1, Mazes: mazes}},
		Maps: map[uint32]catalog.ScriptRecord{
			100: mapScript(), 101: mapScript(), 200: mapScript(),
		},
	}
}

func room(x, y byte, id uint32, alts ...uint32) catalog.DungeonRoom {
	return catalog.DungeonRoom{X: x, Y: y, Map: id, Alternates: alts}
}

// 同一个 quest 对应多张 maze 时（实测 288 个副本的 quest==0 有多张）按 index
// 最小者确定性选取，而不是直接报 ambiguous source maze。
func TestSelectResolvesDuplicateQuestMaze(t *testing.T) {
	c := testCatalog(
		catalog.DungeonMaze{Index: 1, Quest: 0, Start: [2]byte{0, 0}, Boss: [2]byte{0, 0}, Rooms: []catalog.DungeonRoom{room(0, 0, 101)}},
		catalog.DungeonMaze{Index: 0, Quest: 0, Start: [2]byte{0, 0}, Boss: [2]byte{0, 0}, Rooms: []catalog.DungeonRoom{room(0, 0, 100)}},
	)
	s, e := Select(c, protocolSelection(7, 0), 10, nil)
	if e != nil {
		t.Fatal(e)
	}
	if s.Maze.Index != 0 {
		t.Fatalf("maze=%d want the lowest index", s.Maze.Index)
	}
}

// 解析不完整的 maze 不参与消歧：只剩一张可用 maze 时仍然要能进去。
func TestSelectSkipsPendingMaze(t *testing.T) {
	c := testCatalog(
		catalog.DungeonMaze{Index: 0, Quest: 0, Pending: []string{"unsupported room specification"}},
		catalog.DungeonMaze{Index: 1, Quest: 0, Start: [2]byte{0, 0}, Boss: [2]byte{0, 0}, Rooms: []catalog.DungeonRoom{room(0, 0, 100)}},
	)
	s, e := Select(c, protocolSelection(7, 0), 10, nil)
	if e != nil {
		t.Fatal(e)
	}
	if s.Maze.Index != 1 || s.Room.Map != 100 {
		t.Fatalf("maze=%d map=%d", s.Maze.Index, s.Room.Map)
	}
	if _, e = Select(testCatalog(
		catalog.DungeonMaze{Index: 0, Quest: 0, Pending: []string{"unsupported room specification"}},
	), protocolSelection(7, 0), 10, nil); e == nil {
		t.Fatal("pending-only dungeon accepted")
	}
}

// 起始房的首选地图不在目录里时，退到同一节点列出的备选地图。
func TestSelectFallsBackToAlternateStartMap(t *testing.T) {
	c := testCatalog(catalog.DungeonMaze{
		Index: 0, Quest: 0, Start: [2]byte{0, 0}, Boss: [2]byte{0, 0},
		Rooms: []catalog.DungeonRoom{room(0, 0, 999, 100)},
	})
	s, e := Select(c, protocolSelection(7, 0), 10, nil)
	if e != nil {
		t.Fatal(e)
	}
	if s.Room.Map != 100 {
		t.Fatalf("start map=%d want the imported alternate 100", s.Room.Map)
	}
}

// 同一坐标出现多次（源里给了多张候选地图）时取地图可用的一张。
func TestSelectResolvesDuplicateStartRoom(t *testing.T) {
	c := testCatalog(catalog.DungeonMaze{
		Index: 0, Quest: 0, Start: [2]byte{0, 0}, Boss: [2]byte{0, 0},
		Rooms: []catalog.DungeonRoom{room(0, 0, 999), room(0, 0, 100)},
	})
	s, e := Select(c, protocolSelection(7, 0), 10, nil)
	if e != nil {
		t.Fatal(e)
	}
	if s.Room.Map != 100 {
		t.Fatalf("start map=%d want 100", s.Room.Map)
	}
}
