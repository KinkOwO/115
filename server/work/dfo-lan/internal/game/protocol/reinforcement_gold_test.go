package protocol

import (
	"encoding/binary"
	"testing"
)

// 金币强化的成功体与固定券共用 reader 布局，但有两处必须放宽：
// 等级上限 31（行内五位）与「失败可以降级」。
func TestReinforcementGoldReplyShape(t *testing.T) {
	r := ReinforcementRequest{Mode: 0, EquipmentSpace: 3, EquipmentSlot: 12, EquipmentTemplate: 101011322,
		TicketSpace: 0, TicketSlot: 367, MaterialSlot: 0xffff, NPC: 0xffff, ProtectionSlot: 0xffff}
	body, err := ReinforcementGoldReply(r, 68371, 14, 15, 0)
	if err != nil {
		t.Fatalf("构造金币强化成功体失败: %v", err)
	}
	if len(body) != 35 {
		t.Fatalf("金币强化成功体长度 = %d，期望 35", len(body))
	}
	// 金币强化不会摧毁，[1] 回显请求 mode=0（写 10 会让客户端 dynamic_cast
	// 失败、进不去结果分支，弹 1658「No items are available.」）。
	if body[0] != 1 || body[1] != 0 {
		t.Errorf("头部 = %v，期望 [1 0]", body[:2])
	}
	if slot := binary.LittleEndian.Uint16(body[2:]); slot != 367 {
		t.Errorf("材料槽 = %d，期望 367", slot)
	}
	if remaining := binary.LittleEndian.Uint32(body[4:]); remaining != 68371 {
		t.Errorf("材料剩余 = %d，期望 68371", remaining)
	}
	// 布局：0 头 / 1 mode / 2 slot / 4 remaining / 8 0xffff / 10 保留 1 / 11 old / 12 result / 13 level / 14 space
	if body[10] != 1 || body[11] != 14 || body[12] != 0 || body[13] != 15 {
		t.Errorf("保留/old/result/level = %d/%d/%d/%d，期望 1/14/0/15", body[10], body[11], body[12], body[13])
	}
	if body[14] != r.EquipmentSpace {
		t.Errorf("装备空间 = %d，期望 %d", body[14], r.EquipmentSpace)
	}
	if slot := binary.LittleEndian.Uint16(body[15:]); slot != r.EquipmentSlot {
		t.Errorf("装备槽 = %d，期望 %d", slot, r.EquipmentSlot)
	}
}

func TestReinforcementGoldReplyRules(t *testing.T) {
	r := ReinforcementRequest{Mode: 0, EquipmentSpace: 0, EquipmentSlot: 9, TicketSlot: 130}
	// 失败必须 old == level：实机把降级(8→7)写进回包会触发客户端 ADD_HACKTYPE_CNT 并锁死窗口。
	if _, err := ReinforcementGoldReply(r, 10, 8, 8, 1); err != nil {
		t.Fatalf("失败的 old==level 回执被拒: %v", err)
	}
	if _, err := ReinforcementGoldReply(r, 10, 8, 7, 1); err == nil {
		t.Fatal("失败却改变等级的回执必须被拒绝（客户端要求 old==level）")
	}
	// 成功分支的结果等级上限 15。
	if _, err := ReinforcementGoldReply(r, 10, 14, 15, 0); err != nil {
		t.Fatalf("15 级成功回执被拒: %v", err)
	}
	if _, err := ReinforcementGoldReply(r, 10, 16, 17, 0); err == nil {
		t.Fatal("超过 15 级的成功回执必须被拒绝")
	}
	// 回包字段本身仍能表达到 31 级（实例行五位），只是客户端实机只认到 15。
	if _, err := ReinforcementGoldReply(r, 10, 31, 31, 1); err != nil {
		t.Fatalf("31 级失败回执被拒: %v", err)
	}
	// result 只有 0/1。
	if _, err := ReinforcementGoldReply(r, 10, 5, 6, 2); err == nil {
		t.Fatal("非法 result 应被拒绝")
	}
}
