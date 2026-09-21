package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/hex"
	"testing"
)

// 实机缺陷（2026-09-21，角色 test-jh）：奥德赛清关后场上出现两道门——"返回城镇"
// 与"下一个剧情关卡"。点后者时客户端发出 CMD 2062
// （ENUM_CMDPACKET_DUNGEON_DIRECT_MOVE，48 字节，目标副本 100004947），服务端只把
// 它当未实现请求采样，于是客户端一直停在传送黑屏。
const directMoveNextStoryDungeon = "4e01000000000000ffffffffff53f4f505020000000000000001000000a0000000c8000000140000000a000000000000"

func directMoveSession() (*worldSession, []byte) {
	body, e := hex.DecodeString(directMoveNextStoryDungeon)
	if e != nil {
		panic(e)
	}
	return &worldSession{
		role:          storage.Character{ID: 7, WireID: 10},
		dungeons:      &catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{}},
		activeDungeon: &dungeon.Session{RunID: "finished-run", Definition: catalog.DungeonDefinition{ID: 100004946}},
	}, body
}

func TestDirectMoveDungeonRequiresActiveRun(t *testing.T) {
	w, body := directMoveSession()
	w.activeDungeon = nil
	if _, _, e := w.directMoveDungeon(body); e == nil {
		t.Fatal("direct move without an active run accepted")
	}
}

func TestDirectMoveDungeonRequiresTargetInSource(t *testing.T) {
	w, body := directMoveSession()
	if _, _, e := w.directMoveDungeon(body); e == nil {
		t.Fatal("direct move to a dungeon absent from the source accepted")
	}
}

func TestDirectMoveDungeonRejectsMalformedBody(t *testing.T) {
	w, body := directMoveSession()
	if _, _, e := w.directMoveDungeon(body[:47]); e == nil {
		t.Fatal("48-byte direct move request expected")
	}
}

// 直达下一关复用城镇选图的整套进图序列，ack 2062 之后还要补一条 select ack(16)：
// 同一房间从 CMD 15/16 进入时首领房的传送阵正常，从 CMD 2062 进入时它不生效，
// 而两条进图序列除 ack id 外逐字节相同（2026-09-21 实机对照）。
func TestDirectMoveEntryPlanRepeatsSelectionAck(t *testing.T) {
	w := &worldSession{role: storage.Character{ID: 7, WireID: 10}}
	s := &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: 100004948},
		Maze:       catalog.DungeonMaze{Index: 0, Start: [2]byte{0, 0}, Boss: [2]byte{1, 1}},
		Room:       catalog.DungeonRoom{Map: 100016138},
	}
	sel := protocol.DungeonSelection{ID: 100004948, Difficulty: 2, Party: 65535}
	plan, e := w.directMoveEntryPlan(sel, s)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) < 3 {
		t.Fatalf("plan too short: %d packets", len(plan))
	}
	if plan[0].Name != "dungeon_direct_move_ack" || plan[0].ID != 2062 {
		t.Fatalf("first packet is not the CMD 2062 acknowledgement: %+v", plan[0])
	}
	if plan[1].Name != "dungeon_select_ack" || plan[1].ID != 16 || plan[1].Kind != 1 {
		t.Fatalf("missing selection acknowledgement: %+v", plan[1])
	}
	last := plan[len(plan)-1]
	if last.Name != "dungeon_start_map_sent" || last.ID != 29 {
		t.Fatalf("last packet is not NOTI 29 start map: %+v", last)
	}
}

// 直达下一关复用城镇选图的整套进图序列，只有 ack 的 id 不同。
func TestDungeonEntryPlanAcksDirectMove(t *testing.T) {
	w := &worldSession{role: storage.Character{ID: 7, WireID: 10}}
	s := &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: 100004947},
		Maze:       catalog.DungeonMaze{Index: 3, Start: [2]byte{0x40, 0x19}, Boss: [2]byte{1, 1}},
		Room:       catalog.DungeonRoom{Map: 100016119},
	}
	sel := protocol.DungeonSelection{ID: 100004947, Difficulty: 2, Party: 65535}
	plan, e := w.dungeonEntryPlan("dungeon_direct_move_ack", 2062, sel, s)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) < 2 {
		t.Fatalf("plan too short: %d packets", len(plan))
	}
	ack := plan[0]
	if ack.Name != "dungeon_direct_move_ack" || ack.ID != 2062 || ack.Kind != 1 {
		t.Fatalf("first packet is not the CMD 2062 acknowledgement: %+v", ack)
	}
	info := plan[len(plan)-2]
	if info.Name != "dungeon_info_sent" || info.ID != 28 {
		t.Fatalf("missing NOTI 28 dungeon info: %+v", info)
	}
	if !bytes.Contains(info.Payload, []byte{0x53, 0xf4, 0xf5, 0x05}) {
		t.Fatal("dungeon info does not carry the target dungeon 100004947")
	}
	start := plan[len(plan)-1]
	if start.Name != "dungeon_start_map_sent" || start.ID != 29 {
		t.Fatalf("last packet is not NOTI 29 start map: %+v", start)
	}
}
