package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/modcompanion"
)

// 副本同伴的落地契约：加进当前房间 → 与原生同伴同形 → 能过 StartMap 编码 →
// 下一间房自动带过去 → 不进清房判定。
func TestModCompanionsJoinAndCarry(t *testing.T) {
	emptyMap := catalog.ScriptRecord{}
	maze := catalog.DungeonMaze{Index: 0, Start: [2]byte{0, 0}, Rooms: []catalog.DungeonRoom{
		{X: 0, Y: 0, Map: 1}, {X: 1, Y: 0, Map: 2},
	}}
	d := catalog.DungeonDefinition{ID: 99, BasisLevel: 10, Mazes: []catalog.DungeonMaze{maze}}
	c := catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{99: d}, Maps: map[uint32]catalog.ScriptRecord{1: emptyMap, 2: emptyMap}}
	s, err := newSession(c, d, maze)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Monsters) != 0 {
		t.Fatalf("空地图不该有怪物：%+v", s.Monsters)
	}
	if err := s.AddCompanions([]protocol.DungeonMonster{{Template: 201}}); err != nil {
		t.Fatal(err)
	}
	if len(s.Monsters) != 1 {
		t.Fatalf("同伴没有进当前房间：%+v", s.Monsters)
	}
	got := s.Monsters[0]
	if !got.APC || got.Team != 0 || !got.NonCombat || got.Template != 201 ||
		got.SourceIndex != dynamicAPCSourceIndex || got.Rank != 5 || got.Entity == 0 {
		t.Fatalf("同伴行形状不对：%+v", got)
	}
	if s.Visited[s.Room.Map][0] != got {
		t.Fatal("当前房间的 Visited 没有同步")
	}
	// 同伴不是敌人：房间不该因为有同伴而"没清"（RoomCleared 要先 Loaded）。
	s.Loaded = true
	if !s.RoomCleared() {
		t.Fatal("同伴把房间判成未清")
	}
	body, err := protocol.StartMap(protocol.StartMapState{Map: s.Room.Map, Monsters: s.Monsters})
	if err != nil {
		t.Fatalf("同伴行被 StartMap 拒绝：%v", err)
	}
	if len(body) == 0 {
		t.Fatal("StartMap 输出为空")
	}
	next, err := s.Move(c, [2]byte{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Monsters) != 1 {
		t.Fatalf("同伴没有跟到下一间房：%+v", next.Monsters)
	}
	if carried := next.Monsters[0]; carried.Template != 201 || carried.SourceIndex != dynamicAPCSourceIndex ||
		carried.Entity == got.Entity {
		t.Fatalf("携带后的同伴行不对：%+v", carried)
	}
}

func TestAddCompanionsRejectsBadRows(t *testing.T) {
	s := &Session{Room: catalog.DungeonRoom{Map: 1}, Dead: map[uint16]bool{}, Visited: map[uint32][]protocol.DungeonMonster{}}
	if err := s.AddCompanions([]protocol.DungeonMonster{{Template: 0}}); err == nil {
		t.Fatal("模板 0 被接受了")
	}
	if err := s.AddCompanions([]protocol.DungeonMonster{{Template: 201, Rank: 4}}); err == nil {
		t.Fatal("档位 4 被接受了")
	}
	if len(s.Monsters) != 0 {
		t.Fatalf("被拒绝的行不该落进怪物表：%+v", s.Monsters)
	}
}

// 没有 mod 注册时 ApplyModCompanions 必须是空操作：这是"零值不改变服务端行为"的落点。
func TestApplyModCompanionsWithoutRegistrationIsNoop(t *testing.T) {
	modcompanion.Reset()
	defer modcompanion.Reset()
	maze := catalog.DungeonMaze{Index: 0, Start: [2]byte{0, 0}, Rooms: []catalog.DungeonRoom{{X: 0, Y: 0, Map: 1}}}
	d := catalog.DungeonDefinition{ID: 99, BasisLevel: 10, Mazes: []catalog.DungeonMaze{maze}}
	c := catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{99: d}, Maps: map[uint32]catalog.ScriptRecord{1: {}}}
	s, err := newSession(c, d, maze)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyModCompanions()
	if len(s.Monsters) != 0 {
		t.Fatalf("没有注册时不该改动怪物表：%+v", s.Monsters)
	}
}

// 注册后按副本号筛选：被排除的副本一行都不加。
func TestApplyModCompanionsHonoursDungeonFilter(t *testing.T) {
	modcompanion.Reset()
	defer modcompanion.Reset()
	if _, err := modcompanion.Configure(modcompanion.Rules{
		Enabled:    true,
		Source:     "test",
		Companions: []modcompanion.Companion{{Template: 201}},
		// 只给副本 99 加；100 不在名单里。
		IncludeDungeons: []uint32{99},
	}); err != nil {
		t.Fatal(err)
	}
	build := func(id uint32) *Session {
		maze := catalog.DungeonMaze{Index: 0, Start: [2]byte{0, 0}, Rooms: []catalog.DungeonRoom{{X: 0, Y: 0, Map: 1}}}
		d := catalog.DungeonDefinition{ID: id, BasisLevel: 10, Mazes: []catalog.DungeonMaze{maze}}
		c := catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{id: d}, Maps: map[uint32]catalog.ScriptRecord{1: {}}}
		s, err := newSession(c, d, maze)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	included := build(99)
	included.ApplyModCompanions()
	if len(included.Monsters) != 1 {
		t.Fatalf("名单内的副本没有带上同伴：%+v", included.Monsters)
	}
	excluded := build(100)
	excluded.ApplyModCompanions()
	if len(excluded.Monsters) != 0 {
		t.Fatalf("名单外的副本不该带同伴：%+v", excluded.Monsters)
	}
}

// 同伴与地图原生 AIC 同模板时不该重复：携带逻辑按模板去重。
func TestAddCompanionsDeduplicatesAgainstNativeAPC(t *testing.T) {
	apcMap := catalog.ScriptRecord{Cells: []pvf.Token{
		{Type: 3, Text: "[ai character]"},
		{Type: 0, Value: 201}, {Type: 0, Value: 100}, {Type: 0, Value: 200}, {Type: 0, Value: 0},
		{Type: 6, Text: "[character]"}, {Type: 6, Text: "[normal]"},
		{Type: 0, Value: 0}, {Type: 0, Value: 0},
		{Type: 3, Text: "[/ai character]"},
	}}
	maze := catalog.DungeonMaze{Index: 0, Start: [2]byte{0, 0}, Rooms: []catalog.DungeonRoom{{X: 0, Y: 0, Map: 1}}}
	d := catalog.DungeonDefinition{ID: 99, BasisLevel: 10, Mazes: []catalog.DungeonMaze{maze}}
	c := catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{99: d}, Maps: map[uint32]catalog.ScriptRecord{1: apcMap}}
	s, err := newSession(c, d, maze)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Monsters) != 1 {
		t.Fatalf("原生同伴没有落地：%+v", s.Monsters)
	}
	nativeEntity := s.Monsters[0].Entity
	if err := s.AddCompanions([]protocol.DungeonMonster{{Template: 201}}); err != nil {
		t.Fatal(err)
	}
	if len(s.Monsters) != 1 || s.Monsters[0].Entity != nativeEntity {
		t.Fatalf("同模板的同伴被重复添加：%+v", s.Monsters)
	}
}
