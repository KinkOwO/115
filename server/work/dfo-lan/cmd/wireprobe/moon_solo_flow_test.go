package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/storage"
	"encoding/binary"
	"testing"
	"time"
)

func TestMoonSoloFeatureOffAndPreparationCancel(t *testing.T) {
	w := &worldSession{role: storage.Character{ID: 1, WireID: 7}}
	if handled, _, _ := w.moonHandle(2284, []byte{24, 0, 0, 0}, time.Now(), nil); handled {
		t.Fatal("feature default must be off")
	}
	w.moonConfig = &moonSoloConfig{Channel: 5, Remaining: 10}
	w.moon = moonSoloState{created: true, prepared: time.Now(), readySent: true, recovered: true}
	yes, packets, e := w.moonHandle(13, make([]byte, 8), time.Now(), nil)
	if !yes || e != nil || len(packets) != 2 || !w.moon.prepared.IsZero() || w.moon.created {
		t.Fatal("leave did not cancel countdown", e)
	}
	if p, e := w.moonTick(time.Now().Add(time.Minute)); e != nil || len(p) != 0 {
		t.Fatal("cancelled start fired", e)
	}
	clearSelectedWorld(w)
	if w.moon.owner != nil || w.moon.created || w.role.ID != 0 {
		t.Fatal("selection cleanup")
	}
}

func TestMoonSoloLoginAndCounterPacketShapes(t *testing.T) {
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	plain := make([]byte, 59)
	plain[0] = 1
	cipher, e := wire.EncryptPayload(keys, 1, plain)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := wire.ServerFrame(1, 1, cipher)
	if e != nil {
		t.Fatal(e)
	}
	updated, e := moonLoginResponse(raw, keys)
	if e != nil {
		t.Fatal(e)
	}
	decoded, e := wire.DecryptPayload(keys, 1, updated[wire.ServerHeaderSize:])
	if e != nil || decoded[3] != 101 {
		t.Fatal("wrong online type", e)
	}
	for i := range plain {
		if i != 3 && decoded[i] != plain[i] {
			t.Fatal("unrelated login byte changed")
		}
	}
	p, e := protocol.MoonSoloParty115(7, [2]byte{0, 5}, 10)
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 113 || p[29] != 27 || p[95] != 27 || p[106] != 41 || p[20] != 1 || p[69] != 1 || binary.LittleEndian.Uint16(p[71:]) != 7 || p[111] != 10 || p[112] != 10 {
		t.Fatal("Moon roster grammar")
	}
	refusal := moonRefusal(71, nil)
	if len(refusal) != 1 || len(refusal[0].Payload) != 33 {
		t.Fatal("short card refusal")
	}
	if got := moonRefusal(72, []byte{1, 3, 1}); len(got) != 1 || len(got[0].Payload) < 3 {
		t.Fatal("short exit refusal")
	}
}

// 沉月湖换层（第一层→第二层）的跨副本区域交接：三帧都必须用本人身份与本人坐标，
// 而区域 255 只出现在线路上，不能写进角色状态或存档。
func TestMoonFloorHandoffUsesOwnActorAndTransientArea(t *testing.T) {
	w := &worldSession{
		role:  storage.Character{ID: 1, WireID: 7},
		flags: [3]byte{1, 2, 4},
		state: storage.WorldState{Position: storage.WorldPosition{Town: 215, Area: 2, X: 311, Y: 907}},
	}
	before := w.state.Position
	handoff, e := w.moonFloorHandoff()
	if e != nil {
		t.Fatal(e)
	}
	if len(handoff) != 3 || handoff[0].ID != 23 || handoff[1].ID != 24 || handoff[2].ID != 23 {
		t.Fatalf("wrong handoff frame sequence: %+v", handoff)
	}
	for i, p := range handoff {
		if p.Kind != 0 {
			t.Fatalf("handoff frame %d must be a server notification", i)
		}
	}
	// N23 等候区：actor 是自己，城镇 215、区域 2，坐标是本人的坐标。
	if got := binary.LittleEndian.Uint16(handoff[0].Payload[0:]); got != 7 {
		t.Fatalf("waiting-area actor = %d, want the own wire id 7", got)
	}
	if got := binary.LittleEndian.Uint32(handoff[0].Payload[2:]); got != 215 {
		t.Fatalf("waiting-area town = %d", got)
	}
	if got := binary.LittleEndian.Uint32(handoff[0].Payload[6:]); got != 2 {
		t.Fatalf("waiting-area area = %d", got)
	}
	if x, y := binary.LittleEndian.Uint16(handoff[0].Payload[10:]), binary.LittleEndian.Uint16(handoff[0].Payload[12:]); x != 311 || y != 907 {
		t.Fatalf("waiting-area coordinate = %d/%d, want the own 311/907", x, y)
	}
	// N24 区域交接：同一等候区，名单里只有本人一条。
	if got := binary.LittleEndian.Uint32(handoff[1].Payload[0:]); got != 215 {
		t.Fatalf("area-users town = %d", got)
	}
	if got := binary.LittleEndian.Uint32(handoff[1].Payload[4:]); got != 2 {
		t.Fatalf("area-users area = %d", got)
	}
	if got := binary.LittleEndian.Uint16(handoff[1].Payload[8:]); got != 1 {
		t.Fatalf("area-users roster size = %d, want the single own actor", got)
	}
	if got := binary.LittleEndian.Uint16(handoff[1].Payload[10:]); got != 7 {
		t.Fatalf("area-users actor = %d", got)
	}
	// N23 临时出场：同一个人的同一个坐标，只把区域改成 255。
	if got := binary.LittleEndian.Uint32(handoff[2].Payload[6:]); got != 255 {
		t.Fatalf("offscreen area = %d, want the transient 255", got)
	}
	if x, y := binary.LittleEndian.Uint16(handoff[2].Payload[10:]), binary.LittleEndian.Uint16(handoff[2].Payload[12:]); x != 311 || y != 907 {
		t.Fatalf("offscreen coordinate = %d/%d", x, y)
	}
	// 255 不进角色状态；交接可重复调用且无副作用。
	if w.state.Position != before {
		t.Fatalf("handoff mutated the persisted world position: %+v", w.state.Position)
	}
	if again, e := w.moonFloorHandoff(); e != nil || len(again) != 3 {
		t.Fatal("second handoff must be repeatable and side-effect free", e)
	}
}

// 位置已经不在等候区就拒绝交接：区域 255 只是线路表示，位置漂了就报错，不猜。
func TestMoonFloorHandoffRefusesOutsideWaitingArea(t *testing.T) {
	for _, pos := range []storage.WorldPosition{
		{Town: 215, Area: 1, X: 1, Y: 1},
		{Town: 1, Area: 2, X: 1, Y: 1},
	} {
		w := &worldSession{role: storage.Character{ID: 1, WireID: 7}, state: storage.WorldState{Position: pos}}
		if _, e := w.moonFloorHandoff(); e == nil {
			t.Fatalf("handoff accepted %d/%d", pos.Town, pos.Area)
		}
	}
}

// 换层帧序：跨副本交接排在本人资料与 N27 之前，N27 恰好一次；
// 首次进场（floor=false）与普通同层移动的顺序保持不变。
func TestMoonEntryPlanFloorHandoffOrder(t *testing.T) {
	parts := moonEntryParts{
		Handoff: []outboundPacket{{"h_wait", 0, 23, []byte{1}}, {"h_users", 0, 24, []byte{2}}, {"h_off", 0, 23, []byte{3}}},
		Basic:   []byte{4}, Addition: []byte{5}, State: []byte{6}, Roster: []byte{7},
		Dungeon: []byte{8}, Map: []byte{9}, MoonInfo: outboundPacket{"moon_info", 0, 2622, []byte{10}},
	}
	ids := func(p []outboundPacket) []uint16 {
		out := make([]uint16, 0, len(p))
		for _, q := range p {
			out = append(out, q.ID)
		}
		return out
	}
	up := moonEntryPlan(true, parts)
	want := []uint16{2281, 23, 24, 23, 2, 2, 27, 3, 9, 28, 29, 2622}
	if got := ids(up); len(got) != len(want) {
		t.Fatalf("floor handoff plan = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("floor handoff plan = %v, want %v", got, want)
			}
		}
	}
	select27 := 0
	for i, q := range up {
		if q.ID != 27 {
			continue
		}
		select27++
		if i < 3 {
			t.Fatal("N27 must follow the cross-dungeon handoff")
		}
	}
	if select27 != 1 {
		t.Fatalf("N27 sent %d times on the floor handoff", select27)
	}
	first := moonEntryPlan(false, parts)
	wantFirst := []uint16{2, 2, 3, 9, 27, 28, 29, 2622}
	if got := ids(first); len(got) != len(wantFirst) {
		t.Fatalf("first-entry plan = %v, want %v", got, wantFirst)
	} else {
		for i := range wantFirst {
			if got[i] != wantFirst[i] {
				t.Fatalf("first-entry plan = %v, want %v", got, wantFirst)
			}
		}
	}
}
