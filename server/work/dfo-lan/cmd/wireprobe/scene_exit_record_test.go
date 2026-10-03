package main

import (
	"dfolan/internal/testfixture"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
)

// [MERGE-20260928-LAYER-SEQUENCE-EXIT] 实机回归（2026-09-28「贵族机要」）：
// 客户端在层图里点门只发 id=38，不带换图记录。而 StartMap 的 Transition 记录
// （record[6:10]）里带落点，客户端靠它安置角色 —— 100004968 的路由记录是
// [0,0,0,0,4,5,127,1,20,1,0,0,3,0,2,0,0,0]。留全零会让客户端用默认落点，角色
// 卡在场景左上角。这里锁住 interactDoor 合成的请求确实带上了源路由的记录。
func TestSceneExitCarriesTransitionRecord(t *testing.T) {
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
	// 该位置的路由记录必须本身非零，否则本用例没有意义。
	var want [18]byte
	hasRecord := false
	for _, r := range c.SceneRoutes {
		if r.Dungeon == 100004968 && r.Position == pos {
			want, hasRecord = r.Record, true
		}
	}
	if !hasRecord {
		t.Fatalf("位置 %v 没有源路由记录", pos)
	}
	if want == ([18]byte{}) {
		t.Skip("该图的路由记录本来就是全零（自带出生点），不需要补")
	}

	s, e := dungeon.Select(c, protocol.DungeonSelection{ID: 100004968, Difficulty: 2, Party: 65535}, 90, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	s.Room = catalog.DungeonRoom{X: pos[0], Y: pos[1], Map: last}
	w := &worldSession{dungeons: &c, activeDungeon: s}
	t.Logf("room=%+v monsters=%d cinematic=%v", s.Room, len(s.Monsters), s.LayerRoomIsCinematic())
	req, ok := w.sceneExitRequest()
	if !ok {
		t.Fatal("合成请求失败")
	}
	t.Logf("合成请求 record=%v", req[132:150])
	r, de := protocol.DecodeDungeonRoomTransition(req)
	if de != nil {
		t.Fatalf("解码失败: %v", de)
	}
	if _, me := s.MoveScene(c, r); me != nil {
		t.Fatalf("MoveScene 失败: %v", me)
	}

	_, plan, err := w.interactDoor(nil)
	if err != nil {
		t.Fatalf("interactDoor 失败: %v", err)
	}
	// 找 move 请求被解码出来的 Transition：它由 next_map 的 payload 携带。
	var body []byte
	for _, p := range plan {
		if p.Name == "dungeon_next_map_sent" {
			body = p.Payload
		}
	}
	if len(body) < 31 {
		t.Fatalf("没有发出可用的 next_map（len=%d）", len(body))
	}
	got := [18]byte{}
	copy(got[:], body[13:31])
	if got != want {
		t.Fatalf("next_map 里的换图记录 %v 与源路由记录 %v 不一致 —— 客户端会用默认落点，角色卡在场景角落", got, want)
	}
	t.Logf("换图记录已带上: %v", got)
}
