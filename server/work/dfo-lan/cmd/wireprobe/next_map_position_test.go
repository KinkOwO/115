package main

import (
	"dfolan/internal/testfixture"
	"encoding/binary"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
)

// [MERGE-20260928-LAYER-SEQUENCE-EXIT] 实机回归（2026-09-28「贵族机要」）：
// 100004968 的 (0,2) 挂两张层图 [100015974, 100016356]，序列走完时改用 Move 前进到
// 相邻房间 (0,1)。那一刻请求里带的 r.Position 还是层图所在格 (0,2)，而玩家实际去了
// (0,1) —— StartMap 把 Position 写成包的前两字节，客户端据此安放角色。用 r.Position
// 就等于把玩家放在地图外，实机表现是「角色不见了」。
// 这里锁住「发出的 next_map 里前两字节必须是目标房间自己的坐标」。
func TestNextMapPositionMatchesTargetRoom(t *testing.T) {
	c, e := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.odyssey-scenes-release.json"))
	if e != nil {
		t.Fatal(e)
	}
	d, ok := c.Dungeons[100004968]
	if !ok {
		t.Skip("100004968 不在 scenes 导出里")
	}
	var pos [2]byte
	var last uint32
	found := false
	for _, mz := range d.Mazes {
		for _, l := range mz.Layers {
			if len(l.Maps) > 1 {
				pos, last, found = l.Position, l.Maps[len(l.Maps)-1], true
			}
		}
	}
	if !found {
		t.Fatal("100004968 里找不到多张 layer")
	}

	s, e := dungeon.Select(c, protocol.DungeonSelection{ID: 100004968, Difficulty: 2, Party: 65535}, 90, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	// 把玩家放到层图序列的最后一张（模拟客户端播完序列后点门）。
	s.Room = catalog.DungeonRoom{X: pos[0], Y: pos[1], Map: last}
	s.Dead = map[uint16]bool{}
	for _, m := range s.Monsters {
		s.Dead[m.Entity] = true
	}
	w := &worldSession{dungeons: &c, activeDungeon: s}

	// 合成「层图序列走完」的门请求（interactDoor 内部正是这种形态）。
	req := make([]byte, 160)
	req[0], req[1] = pos[0], pos[1]
	req[10] = 1 // LayerChange
	binary.LittleEndian.PutUint32(req[151:155], 100004968)

	next, plan, err := w.moveDungeonRoom(req)
	if err != nil {
		t.Fatalf("moveDungeonRoom 失败: %v", err)
	}
	if [2]byte{next.Room.X, next.Room.Y} == pos {
		t.Fatalf("应前进到相邻房间，却仍在 %v", pos)
	}
	var body []byte
	for _, p := range plan {
		if p.Name == "dungeon_next_map_sent" {
			body = p.Payload
		}
	}
	if len(body) < 2 {
		t.Fatalf("没有发出可用的 next_map")
	}
	if body[0] != next.Room.X || body[1] != next.Room.Y {
		t.Fatalf("next_map 里的房间坐标 (%d,%d) 与目标房间 (%d,%d) 不一致 —— 客户端会把角色放在地图外",
			body[0], body[1], next.Room.X, next.Room.Y)
	}
	// [MERGE-20260928-TRANSITION-DEFAULT] 换图记录不能是零。StartMap 的默认记录是
	// `0000ffffffffffffffff000000000000`；发全零会覆盖它，客户端拿零落点安置角色
	// （实机 2026-09-28「无信草原」100004781 角色不显示）。这里用的 100004968 有源
	// 路由记录，所以应当是非零的路由记录而不是默认回落。
	if len(body) >= 31 {
		rec := [18]byte{}
		copy(rec[:], body[13:31])
		if rec == ([18]byte{}) {
			t.Fatal("换图记录全零 —— 客户端会拿零落点安置角色")
		}
		t.Logf("换图记录: %v", rec)
	}
	t.Logf("目标房间 (%d,%d) map=%d，next_map 坐标 (%d,%d)", next.Room.X, next.Room.Y, next.Room.Map, body[0], body[1])
}
