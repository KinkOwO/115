package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"testing"
	"time"
)

func TestMoonSoloFeatureOffAndPreparationCancel(t *testing.T) {
	w := &worldSession{role: database.Character{ID: 1, WireID: 7}}
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
		role:  database.Character{ID: 1, WireID: 7},
		flags: [3]byte{1, 2, 4},
		state: database.WorldState{Position: database.WorldPosition{Town: 215, Area: 2, X: 311, Y: 907}},
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
	for _, pos := range []database.WorldPosition{
		{Town: 215, Area: 1, X: 1, Y: 1},
		{Town: 1, Area: 2, X: 1, Y: 1},
	} {
		w := &worldSession{role: database.Character{ID: 1, WireID: 7}, state: database.WorldState{Position: pos}}
		if _, e := w.moonFloorHandoff(); e == nil {
			t.Fatalf("handoff accepted %d/%d", pos.Town, pos.Area)
		}
	}
}

// 等候区红门（C15）：只有 215/2 那一扇门由月湖接管，且未满足条件时必须走**否定回执**，
// 而不是普通门那条「gate_ack + 空选图 N27」—— 后者实机表现是整屏黑屏后客户端强关连接
// （2026-10-02 18:23:50 的日志：C15 → dungeon_gate_ack → 空 N27 → close）。
func TestMoonPortalGateOnlyTakesTheWaitingArea(t *testing.T) {
	// 月湖未启用（普通频道）：这扇门与月湖无关，必须留给普通门路径。
	off := &worldSession{role: database.Character{ID: 1, WireID: 7}}
	off.state.Position = database.WorldPosition{Town: 215, Area: 2}
	if handled, _, _ := off.moonHandle(15, make([]byte, 8), time.Now(), nil); handled {
		t.Fatal("月湖未启用时红门必须留给普通门路径")
	}
	w := &worldSession{role: database.Character{ID: 1, WireID: 7}, moonConfig: &moonSoloConfig{Channel: 101}}
	// 招募区 215/1：不是等候区，不能被这条分支吃掉。
	w.state.Position = database.WorldPosition{Town: 215, Area: 1}
	if handled, _, _ := w.moonHandle(15, make([]byte, 8), time.Now(), nil); handled {
		t.Fatal("招募区的门不该由月湖接管")
	}
	// 等候区 215/2、未建队 → 必须拒绝（上层 main.go 会转成 N15 Refusal(4)）。
	w.state.Position = database.WorldPosition{Town: 215, Area: 2}
	handled, packets, e := w.moonHandle(15, make([]byte, 8), time.Now(), nil)
	if !handled || e == nil || len(packets) != 0 {
		t.Fatalf("未建队时必须拒绝：handled=%v packets=%v err=%v", handled, packets, e)
	}
	if !w.moon.prepared.IsZero() {
		t.Fatal("被拒的红门不能启动倒计时")
	}
	// 已建队但副本/掉落资源不可用 → 同样走否定通道，绝不发选图 N27。
	w.moon.created = true
	if handled, packets, e = w.moonHandle(15, make([]byte, 8), time.Now(), nil); !handled || e == nil || len(packets) != 0 {
		t.Fatalf("资源不可用时必须拒绝：handled=%v packets=%v err=%v", handled, packets, e)
	}
	if !w.moon.prepared.IsZero() {
		t.Fatal("资源校验没过就启动了倒计时")
	}
	// 回执形状：门应答的否定回执是 3 字节 Refusal(4)（0x00 + u16 4）。
	refusal := moonRefusal(15, make([]byte, 8))
	if len(refusal) != 1 || refusal[0].Kind != 1 || refusal[0].ID != 15 || len(refusal[0].Payload) != 3 {
		t.Fatalf("红门拒绝回执形状：%+v", refusal)
	}
	if refusal[0].Payload[0] != 0 || binary.LittleEndian.Uint16(refusal[0].Payload[1:]) != 4 {
		t.Fatalf("红门拒绝必须是 Refusal(4)，得到 %x", refusal[0].Payload)
	}
}

// 换层帧序：跨副本交接排在本人资料与 N27 之前，N27 恰好一次；
// 首次进场（floor=false）与普通同层移动的顺序保持不变。
func TestMoonEntryPlanFloorHandoffOrder(t *testing.T) {
	parts := moonEntryParts{
		Handoff: []outboundPacket{{"h_wait", 0, 23, []byte{1}}, {"h_users", 0, 24, []byte{2}}, {"h_off", 0, 23, []byte{3}}},
		Basic:   []byte{4}, Addition: []byte{5}, Worn: []byte{11}, State: []byte{6}, Roster: []byte{7},
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
	// 换层：N2281 → 区域交接 → 本人资料(2/2/14) → N27 → N3/N9 → N28/N29 → N2622。
	// N14 是穿戴窗口，与普通副本入图（2 → 2 → 14）同序，缺了它装备栏不刷新。
	want := []uint16{2281, 23, 24, 23, 2, 2, 14, 27, 3, 9, 28, 29, 2622}
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
	wantFirst := []uint16{2, 2, 14, 3, 9, 27, 28, 29, 2622}
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
