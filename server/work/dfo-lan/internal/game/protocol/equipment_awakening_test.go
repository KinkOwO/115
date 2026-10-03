package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 构造一帧"同口径"请求体：前 13 字节是客户端 writer 里的信封位置（服务端不解释），
// 字段区在 +13..+24（sub_141491D10），合计 25 字节。
func awakeningPayload(mode byte, group uint32, space byte, slot uint16, target uint32) []byte {
	p := make([]byte, EquipmentAwakeningPayloadSize)
	p[13] = mode
	binary.LittleEndian.PutUint32(p[14:], group)
	p[18] = space
	binary.LittleEndian.PutUint16(p[19:], slot)
	binary.LittleEndian.PutUint32(p[21:], target)
	return p
}

func TestDecodeEquipmentAwakening(t *testing.T) {
	p := awakeningPayload(0, 2, 3, 12, 101001150)
	r, err := DecodeEquipmentAwakening(p)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if r.PayloadOffset != equipmentAwakeningFields {
		t.Fatalf("payload offset = %d, want %d", r.PayloadOffset, equipmentAwakeningFields)
	}
	if r.Mode != 0 || r.MaterialGroup != 2 || r.Space != 3 || r.Slot != 12 || r.Target != 101001150 {
		t.Fatalf("decoded %+v", r)
	}

	// 初始化/返还路径：mode=1、材料组与目标都是 0xFFFFFFFF（sub_141491BE0）。
	init := awakeningPayload(1, EquipmentAwakeningNoTarget, 0, 64, EquipmentAwakeningNoTarget)
	got, err := DecodeEquipmentAwakening(init)
	if err != nil {
		t.Fatalf("decode init: %v", err)
	}
	if got.Mode != 1 || got.MaterialGroup != EquipmentAwakeningNoTarget || got.Target != EquipmentAwakeningNoTarget {
		t.Fatalf("decoded init %+v", got)
	}
}

// 带信封的帧（长度 38 = 13 + 25）必须整体后移 13 字节。
func TestDecodeEquipmentAwakeningWithEnvelope(t *testing.T) {
	body := awakeningPayload(0, 1, 0, 7, 0)
	frame := append([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, body...)
	binary.LittleEndian.PutUint16(frame[1:], EquipmentAwakeningOpcode)
	r, err := DecodeEquipmentAwakening(frame)
	if err != nil {
		t.Fatalf("decode framed: %v", err)
	}
	if r.PayloadOffset != equipmentAwakeningFields+equipmentAwakeningEnvelope {
		t.Fatalf("payload offset = %d, want %d", r.PayloadOffset, equipmentAwakeningFields+equipmentAwakeningEnvelope)
	}
	if r.MaterialGroup != 1 || r.Slot != 7 || r.Mode != 0 {
		t.Fatalf("decoded framed %+v", r)
	}
}

func TestDecodeEquipmentAwakeningRejects(t *testing.T) {
	short := awakeningPayload(0, 1, 0, 7, 0)[:24]
	if _, err := DecodeEquipmentAwakening(short); err == nil {
		t.Fatal("a 24-byte body must be rejected")
	}
	badMode := awakeningPayload(2, 1, 0, 7, 0)
	if _, err := DecodeEquipmentAwakening(badMode); err == nil {
		t.Fatal("mode 2 must be rejected")
	}
	badSpace := awakeningPayload(0, 1, 1, 7, 0)
	if _, err := DecodeEquipmentAwakening(badSpace); err == nil {
		t.Fatal("space 1 must be rejected")
	}
	// 前一帧恰好凑成 38 字节但首字节不是 1 ⇒ 不当作带信封帧，按同口径读，
	// 于是 +13 落在字段区里（模式 0 = 调适），这是有意的结构判据。
	notFramed := append(append([]byte(nil), awakeningPayload(0, 3, 3, 5, 0)...), make([]byte, 13)...)
	r, err := DecodeEquipmentAwakening(notFramed)
	if err != nil {
		t.Fatalf("decode long: %v", err)
	}
	if r.PayloadOffset != equipmentAwakeningFields || r.MaterialGroup != 3 || r.Slot != 5 {
		t.Fatalf("long body decoded %+v", r)
	}
}

func TestEquipmentAwakeningReplyShape(t *testing.T) {
	ok := EquipmentAwakeningSuccess()
	if !bytes.Equal(ok, []byte{1, 0, 0}) {
		t.Fatalf("success reply = %v, want 01 00 00（状态 1 = 成功，客户端据此刷新调适面板）", ok)
	}
	fail := EquipmentAwakeningFailure()
	if len(fail) != 3 || fail[0] != 0 {
		t.Fatalf("failure reply = %v, want 3 bytes with a zero status", fail)
	}
	coded := EquipmentAwakeningReply(0, 217)
	if coded[0] != 0 || coded[1] != 217 || coded[2] != 0 {
		t.Fatalf("coded reply = %v", coded)
	}
	if !bytes.Equal(EquipmentAwakeningReply(1, 0), ok) {
		t.Fatal("EquipmentAwakeningReply(1, 0) must match EquipmentAwakeningSuccess")
	}
}
