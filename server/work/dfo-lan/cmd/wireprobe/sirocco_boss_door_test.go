package main

import (
	"encoding/binary"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
)

// 塞洛可红柱由 NOTI312 真实点亮、interactDoor 合成旁路撤除后，进 Boss 房的
// 唯一路径应当是客户端门开后自己发的 CMD45 换房请求：服务端必须接受 (3,1)→(4,1)。
// 2026-10-04 实机同轮验证 NOTI312 生效；本测试锁住撤除旁路后的服务端侧契约。
func TestSiroccoBossDoorCMD45AfterBypassRemoval(t *testing.T) {
	c, e := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.odyssey-scenes-release.json"))
	if e != nil {
		t.Fatal(e)
	}
	s, e := dungeon.Select(c, protocol.DungeonSelection{ID: 100004961, Difficulty: 2, Party: 65535}, 80, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	s.Room = catalog.DungeonRoom{X: 3, Y: 1, Map: 100016294}
	w := &worldSession{dungeons: &c, activeDungeon: s}

	req := make([]byte, 160)
	req[0], req[1] = 4, 1
	binary.LittleEndian.PutUint32(req[151:155], 100004961)
	next, plan, err := w.moveDungeonRoom(req)
	if err != nil {
		t.Fatalf("(3,1)→(4,1) 的 CMD45 应被接受（门由客户端把关，服务端一律放行奥德赛换房）: %v", err)
	}
	if next == nil || next.Room.Map != 100016295 || len(plan) == 0 {
		t.Fatalf("未进入 Boss 房 100016295: next=%+v plan=%d", next, len(plan))
	}
}
