package main

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"testing"
)

// NOTI2634 必须排在入场帧的**最后**：handler 直接写角色实体对象的 `+1872/+1876`，
// 而 `actor_appearance_ready`（id 2）会重建实体 —— 排在它前面写会被覆盖，
// 用户看到的就是"数字没变"。见 docs/protocol/oath-set-points-20261004.md §7.1。
func TestEntryPacketsPutPartSetPointsLast(t *testing.T) {
	p := entryPayloads{
		Skills:    []byte{1},
		Basic:     []byte{2},
		OathPartSetPoints: []outboundPacket{
			{"oath_part_set_point", 0, protocol.PartSetPointOpcode, protocol.PartSetPoint(7, 0, 710)},
			{"oath_part_set_point", 0, protocol.PartSetPointOpcode, protocol.PartSetPoint(1, 0, 710)},
		},
	}
	packets := p.packets()
	if len(packets) < 2 {
		t.Fatalf("packets = %d, want the entry tail plus the point frames", len(packets))
	}
	actor, first, last := -1, -1, -1
	for i, packet := range packets {
		switch {
		case packet.Name == "actor_appearance_ready":
			actor = i
		case packet.Name == "oath_part_set_point":
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if actor < 0 || first < 0 {
		t.Fatalf("missing frames: actor=%d first point=%d", actor, first)
	}
	if first <= actor {
		t.Fatalf("part set points must follow the actor rebuild: actor=%d first=%d", actor, first)
	}
	if last != len(packets)-1 {
		t.Fatalf("part set points must be the last frames: last=%d total=%d", last, len(packets))
	}
	if got := binary.LittleEndian.Uint16(packets[first].Payload[0:2]); got != 7 {
		t.Fatalf("first point frame key = %d, want 7", got)
	}
	if got := binary.LittleEndian.Uint32(packets[first].Payload[6:10]); got != 710 {
		t.Fatalf("first point frame oath points = %d, want 710", got)
	}
}

// 没有积分帧时入场集与改动前完全一致（不凭空多帧）。
func TestEntryPacketsWithoutPartSetPoints(t *testing.T) {
	base := (entryPayloads{Skills: []byte{1}}).packets()
	withEmpty := (entryPayloads{Skills: []byte{1}, OathPartSetPoints: nil}).packets()
	if len(base) != len(withEmpty) {
		t.Fatalf("empty point list changed the plan: %d vs %d", len(base), len(withEmpty))
	}
	for i := range base {
		if base[i].Name != withEmpty[i].Name || base[i].ID != withEmpty[i].ID {
			t.Fatalf("frame %d changed: %s/%d vs %s/%d", i, base[i].Name, base[i].ID, withEmpty[i].Name, withEmpty[i].ID)
		}
	}
}
