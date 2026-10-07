package main

import (
	"bytes"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
)

// noblesseLayerSession 复现实机 2026-10-05 贵族机要 100004968 的前半段：
// (0,2) = base 100015973 + 层序列 [100015974, 100016356]，客户端用两次 CMD45
// layer 切换把序列播完，然后前进到 (0,1) 的战斗房。
func noblesseLayerSession(t *testing.T) (catalog.DungeonCatalog, *dungeon.Session, [2]byte, uint32, uint32) {
	t.Helper()
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
	var base uint32
	var count int
	found := false
	for _, mz := range d.Mazes {
		for _, l := range mz.Layers {
			if len(l.Maps) < 2 {
				continue
			}
			pos, last, count, found = l.Position, l.Maps[len(l.Maps)-1], len(l.Maps), true
			for _, r := range mz.Rooms {
				if [2]byte{r.X, r.Y} == pos {
					base = r.Map
				}
			}
		}
	}
	if !found || base == 0 || base == last {
		t.Fatal("需要一个带 base 图的多张层序列格子")
	}
	s, e := dungeon.Select(c, protocol.DungeonSelection{ID: 100004968, Difficulty: 2, Party: 65535}, 90, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	// 实机里客户端进层图的两包 CMD45（同位置、同一条源换图记录）。
	record := [18]byte{0, 0, 0, 0, 4, 5, 0x7f, 1, 0x14, 1, 0, 0, 3, 0, 2, 0, 0, 0}
	for i := 0; i < count; i++ {
		s, e = s.MoveScene(c, protocol.DungeonRoomTransition{Dungeon: 100004968, Position: pos, LayerChange: true, Record: record})
		if e != nil {
			t.Fatal(e)
		}
		s.Loaded = true
		if s.Room.Map == last {
			break
		}
	}
	if s.Room.Map != last {
		t.Fatalf("层序列没有走到末张: room=%d want=%d", s.Room.Map, last)
	}
	s, e = s.Move(c, [2]byte{pos[0], pos[1] - 1})
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	return c, s, pos, base, last
}

// [MERGE-20261005-LAYER-REVISIT-LAYER] 实机回归：打完后面几间房走回 (0,2)。
// 上一版这里发 flag 0 + mode 0 的 34 字节复用包，原生 flag 0 在 1452b787f..7886
// 绕过层序号清零，客户端留着末张层图的近景道具、却按 base 描述符装配远景
// [background animation] 层 —— 症状是「走回头路后房间背景全黑」；同局六个普通格
// 拿到逐字节同构的复用包且背景正常，坏的只有 layered 格。
//
// 期望形态是 flag 1 + mode 0：原生把同格层索引前进并钳在最后一张
// （1452b77ee..1452b788b），按有效序号选中缓存里那张**层图**房间，mode 0
// （1452b78f0）跳过建图与 ON START MAP 演出。attempt 1/3 的 flag 2 清序号后选的是
// base 缓存，实机背景正常但恢复成入场那间宫殿（内容选错房）。
func TestLayeredCellWalkBackKeepsLayer(t *testing.T) {
	c, s, pos, base, last := noblesseLayerSession(t)
	if s.Room.Map == base || s.Room.Map == last {
		t.Fatalf("前置条件: 应已经走到 (0,2) 之外的战斗房, room=%d", s.Room.Map)
	}
	w := &worldSession{dungeons: &c, activeDungeon: s}
	// 客户端回头那一包：普通 CMD45，落点记录不带覆盖。
	back := protocol.DungeonRoomTransition{Dungeon: 100004968, Position: pos,
		Record: [18]byte{0, 0, 0, 0, 3, 5, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0}}
	next, plan, e := w.moveDungeonRoomDecoded(back)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 2 || plan[1].ID != 29 {
		t.Fatalf("换图包: %+v", plan)
	}
	body := plan[1].Payload
	if len(body) != 34 || !bytes.Equal(body[:3], []byte{pos[0], pos[1], 1}) || body[31] != 0 {
		t.Fatalf("expected same-cell cached layer room with layer ordinal kept, got %x", body)
	}
	if !bytes.Equal(body[13:31], []byte{0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("walk-back must keep the native default landing record: %x", body[13:31])
	}
	if next.Room.X != pos[0] || next.Room.Y != pos[1] {
		t.Fatalf("回访落点格子不对: %+v", next.Room)
	}
	if next.Room.Map != last {
		t.Fatalf("服务端把回访房间换成了 %d，应停在末张层图 %d", next.Room.Map, last)
	}
	if next.IsResumedSceneBase() {
		t.Fatal("回访不该登记 resumed base，否则下一包又会回宫殿图")
	}

	// 序号还留在末张：从相邻格再走回这一格仍是同一形态，不回 base、不重播剧情。
	next.Loaded = true
	w.activeDungeon = next
	forward, e := next.Move(c, [2]byte{pos[0], pos[1] - 1})
	if e != nil {
		t.Fatal(e)
	}
	forward.Loaded = true
	w.activeDungeon = forward
	_, again, e := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: 100004968, Position: pos})
	if e != nil {
		t.Fatal(e)
	}
	if len(again) != 2 || len(again[1].Payload) != 34 || again[1].Payload[2] != 1 || again[1].Payload[31] != 0 {
		t.Fatalf("回访层图缓存应为 flag1+mode0: %+v", again)
	}
	if again[1].Payload[0] != pos[0] || again[1].Payload[1] != pos[1] {
		t.Fatalf("回访落点格子不对: %x", again[1].Payload[:3])
	}
}

// 层序列还没播完（末张未访问）时不能提前退出层图：那会让客户端在序列中途
// 收到 base 缓存并重播剧情（见 scene_transition 的 LAYER-SEQUENCE-EXIT 记录）。
func TestUnfinishedLayerSequenceDoesNotExit(t *testing.T) {
	c, s, pos, base, last := noblesseLayerSession(t)
	delete(s.Visited, last)
	w := &worldSession{dungeons: &c, activeDungeon: s}
	next, plan, e := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: 100004968, Position: pos})
	if e != nil {
		t.Fatal(e)
	}
	body := plan[1].Payload
	if len(body) == 34 && (body[2] == 1 || body[2] == 2) {
		t.Fatalf("未播完的层序列被当成已播完处理了: %x", body)
	}
	if next.Room.Map == base || next.Room.Map == last {
		t.Fatalf("未播完的层序列被换成了 %d（base=%d last=%d）", next.Room.Map, base, last)
	}
}
