package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/storage"
	"testing"
)

// CMD 1418 = ENUM_CMDPACKET_PREV_VILLAGE：走出私有回程房（赛丽亚房间 38/1、
// 142/1）回到本次进房的那张图。实机 2026-09-23 客户端从 new_seria_room 连发 39 次
// 空体帧，服务端当时没有任何分支，default 把每一条都丢掉了，人出不去。
func TestPrevVillageUsesStampedOrigin(t *testing.T) {
	w := &worldSession{role: storage.Character{ID: 7, WireID: 10}}
	w.state.Position = storage.WorldPosition{
		Town: 38, Area: 1, X: 557, Y: 340,
		Return: &storage.WorldReturn{Town: 41, Area: 1, X: 404, Y: 197},
	}
	r, e := w.prevVillage(nil)
	if e != nil {
		t.Fatal(e)
	}
	if r.Town != 41 || r.Area != 1 || r.X != 404 || r.Y != 197 {
		t.Fatalf("destination is not the stamped origin: %+v", r)
	}
	if r.PreviousTown != 38 || r.PreviousArea != 1 {
		t.Fatalf("source area is not the room the character stands in: %+v", r)
	}
}

// 体必须为空、仅城镇角色、没有戳就拒绝：这三条都不许靠猜。
func TestPrevVillageRefusesWithoutEvidence(t *testing.T) {
	stamped := func() *worldSession {
		w := &worldSession{role: storage.Character{ID: 7, WireID: 10}}
		w.state.Position = storage.WorldPosition{
			Town: 38, Area: 1, X: 557, Y: 340,
			Return: &storage.WorldReturn{Town: 41, Area: 1, X: 404, Y: 197},
		}
		return w
	}

	w := stamped()
	if _, e := w.prevVillage([]byte{0}); e == nil {
		t.Fatal("non-empty body accepted")
	}

	w = &worldSession{role: storage.Character{ID: 7, WireID: 10}}
	w.state.Position = storage.WorldPosition{Town: 38, Area: 1, X: 557, Y: 340}
	if _, e := w.prevVillage(nil); e == nil {
		t.Fatal("prev village without a return stamp accepted")
	}

	w = stamped()
	w.activeDungeon = &dungeon.Session{}
	if _, e := w.prevVillage(nil); e == nil {
		t.Fatal("prev village inside a dungeon accepted")
	}
}
