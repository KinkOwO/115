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

// Elvenmere 特殊副本（ID 100003126）支持选区（Zone 初始层数通过 Extra 传入，如 1、36、61、86）。
// 普通副本依然要求 Extra == 0。
func TestSelectElvenmereZone(t *testing.T) {
	mapScript := func() catalog.ScriptRecord {
		return catalog.ScriptRecord{Path: "elvenmere.map", Cells: []pvf.Token{{Type: 3, Text: "[dungeon start area]"}}}
	}
	c := catalog.DungeonCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)},
		Dungeons: map[uint32]catalog.DungeonDefinition{
			100003126: {
				ID:           100003126,
				MinimumLevel: 17,
				Mazes: []catalog.DungeonMaze{{
					Index: 0, Quest: 0, Start: [2]byte{0, 0}, Boss: [2]byte{0, 0},
					Rooms: []catalog.DungeonRoom{room(0, 0, 100007201)},
				}},
			},
			7: {
				ID:           7,
				MinimumLevel: 1,
				Mazes: []catalog.DungeonMaze{{
					Index: 0, Quest: 0, Start: [2]byte{0, 0}, Boss: [2]byte{0, 0},
					Rooms: []catalog.DungeonRoom{room(0, 0, 100)},
				}},
			},
		},
		Maps: map[uint32]catalog.ScriptRecord{
			100007201: mapScript(),
			100:       mapScript(),
		},
	}

	// 1. 普通副本 Extra != 0 必须被拒绝
	if _, err := Select(c, protocol.DungeonSelection{ID: 7, Difficulty: 1, Extra: 1, Party: 65535}, 20, nil); err == nil {
		t.Fatal("ordinary dungeon accepted Extra != 0")
	}

	// 2. Elvenmere 等级不足必须被拒绝
	if _, err := Select(c, protocol.DungeonSelection{ID: 100003126, Difficulty: 2, Extra: 1, Party: 65535}, 16, nil); err == nil {
		t.Fatal("elvenmere accepted under-level character")
	}

	// 3. Elvenmere 各合规 Zone 层数必须成功进入，并记录 Extra
	for _, zoneFloor := range []uint16{0, 1, 36, 61, 86, 100} {
		s, err := Select(c, protocol.DungeonSelection{ID: 100003126, Difficulty: 2, Extra: zoneFloor, Party: 65535}, 50, nil)
		if err != nil {
			t.Fatalf("elvenmere zone floor %d failed: %v", zoneFloor, err)
		}
		if s.Extra != zoneFloor {
			t.Fatalf("session Extra = %d, want %d", s.Extra, zoneFloor)
		}
		if s.Room.Map != 100007201 {
			t.Fatalf("session start map = %d, want 100007201", s.Room.Map)
		}
	}

	// 4. Elvenmere 超出 100 层非法 Extra 必须被拒绝
	if _, err := Select(c, protocol.DungeonSelection{ID: 100003126, Difficulty: 2, Extra: 101, Party: 65535}, 50, nil); err == nil {
		t.Fatal("elvenmere accepted Extra > 100")
	}
}

// 测试实机日志中的十六进制请求包解码后能正常通过 Select
func TestSelectElvenmereLivePacket(t *testing.T) {
	// 用户实机日志: plain_hex "36edf5050201000000ffff000000000000000000000000000000000000000000"
	p := []byte{
		0x36, 0xed, 0xf5, 0x05, // ID = 100003126
		0x02,       // Difficulty = 2
		0x01, 0x00, // Extra = 1 (Zone 1-35)
		0x00,       // Mode = 0
		0x00,       // Flag = 0
		0xff, 0xff, // Party = 65535
		0x00, 0x00, 0x00, 0x00, // Reserved = 0
		0x00,                   // Tail = 0
		0x00, 0x00, 0x00, 0x00, // Quest = 0
		0x00, 0x00, // Options = [0, 0]
		0x00, 0x00, 0x00, 0x00, // Event = 0
		// padding (6 bytes to 32)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	req, err := protocol.DecodeDungeonSelection(p)
	if err != nil {
		t.Fatalf("DecodeDungeonSelection failed: %v", err)
	}
	if req.ID != 100003126 || req.Difficulty != 2 || req.Extra != 1 || req.Party != 65535 {
		t.Fatalf("decoded request mismatch: %+v", req)
	}

	mapScript := func() catalog.ScriptRecord {
		return catalog.ScriptRecord{Path: "elvenmere.map", Cells: []pvf.Token{{Type: 3, Text: "[dungeon start area]"}}}
	}
	c := catalog.DungeonCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)},
		Dungeons: map[uint32]catalog.DungeonDefinition{
			100003126: {
				ID:           100003126,
				MinimumLevel: 17,
				Mazes: []catalog.DungeonMaze{{
					Index: 0, Quest: 0, Start: [2]byte{0, 0}, Boss: [2]byte{0, 0},
					Rooms: []catalog.DungeonRoom{room(0, 0, 100007201)},
				}},
			},
		},
		Maps: map[uint32]catalog.ScriptRecord{
			100007201: mapScript(),
		},
	}
	s, err := Select(c, req, 50, nil)
	if err != nil {
		t.Fatalf("Select with live packet failed: %v", err)
	}
	if s.Extra != 1 {
		t.Fatalf("session Extra = %d, want 1", s.Extra)
	}
	if s.Room.Map != 100007201 {
		t.Fatalf("session Room.Map = %d, want 100007201", s.Room.Map)
	}
}
