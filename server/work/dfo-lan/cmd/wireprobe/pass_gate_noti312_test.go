package main

import (
	"encoding/binary"
	"reflect"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
)

// 奥德赛塞洛可（100004961）红柱路径门只认 NOTI312：源
// [boss room entrance condition] `[hunt monster] 1 109019125 0 1` 声明的猎杀目标
// 死亡确认后，服务端必须恰好补发一帧 `<u32 conditionID> <u8 completed>`；
// 非目标怪与重复死亡不发。静态取证 2026-10-04（attempt 1/3 命中，同日实机确认），布局见
// protocol.BossRoomPassGateCompleted 注释。
func TestPassGateNoti312OnBossEntranceHuntDeath(t *testing.T) {
	newBody := func(entity uint32) []byte {
		body := make([]byte, 64)
		binary.LittleEndian.PutUint32(body, entity)
		binary.LittleEndian.PutUint16(body[4:], 10) // killer = 本角色 WireID
		return body
	}
	run := &dungeon.Session{
		RunID:  "pass-gate-test",
		Loaded: true,
		Definition: catalog.DungeonDefinition{
			ID: 100004961, Odyssey: true,
			BossEntranceConditionIDs: []uint32{109019125},
		},
		Monsters: []protocol.DungeonMonster{
			{Entity: 4096, Template: 109019125},
			{Entity: 4097, Template: 111111},
		},
		Dead:    map[uint16]bool{},
		Unowned: map[uint16]bool{},
	}
	w := &worldSession{
		role:          database.Character{WireID: 10},
		activeDungeon: run,
		deathSent:     map[uint16]bool{},
	}
	event := func(map[string]any) {}

	var gates []packetShape
	collect := func(plan []outboundPacket) []packetShape {
		var out []packetShape
		for _, p := range plan {
			if p.ID == 312 {
				out = append(out, packetShape{Kind: p.Kind, ID: p.ID, Body: string(p.Payload)})
			}
		}
		return out
	}
	plan, err := w.monsterDeath(newBody(4096), event)
	if err != nil {
		t.Fatal(err)
	}
	gates = append(gates, collect(plan)...)
	if len(gates) != 1 {
		t.Fatalf("猎杀目标死亡应发恰好一帧 NOTI312，得到 %d 帧：%+v", len(gates), plan)
	}
	want := packetShape{Kind: 0, ID: 312, Body: string(protocol.BossRoomPassGateCompleted(109019125))}
	if !reflect.DeepEqual(gates[0], want) {
		t.Fatalf("NOTI312 包体不符：%+v，期望 %+v", gates[0], want)
	}

	// 重复死亡：ConfirmDeath 已记 Dead，confirmed=false，不再发。
	w.deathSent = map[uint16]bool{}
	plan, err = w.monsterDeath(newBody(4096), event)
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(plan); len(got) != 0 {
		t.Fatalf("重复死亡不应再发 NOTI312，得到 %+v", got)
	}

	// 非入场条件目标怪死亡：不发。
	plan, err = w.monsterDeath(newBody(4097), event)
	if err != nil {
		t.Fatal(err)
	}
	if got := collect(plan); len(got) != 0 {
		t.Fatalf("非条件目标死亡不应发 NOTI312，得到 %+v", got)
	}
}

type packetShape struct {
	Kind byte
	ID   uint16
	Body string
}
