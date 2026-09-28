package protocol

import (
	"encoding/binary"
	"testing"
)

// 实机抓到的锻造请求：+17 Masterpiece Deadly Gold Sword，材料槽 141，穿戴武器槽 12。
func TestDecodeRefineRequest(t *testing.T) {
	name := "+17 Masterpiece Deadly Gold Sword"
	p := []byte{3, 0x0c, 0x00}
	p = binary.LittleEndian.AppendUint32(p, 100761210)
	p = binary.LittleEndian.AppendUint16(p, 141)
	p = binary.LittleEndian.AppendUint32(p, uint32(len(name)))
	p = append(p, []byte(name)...)
	p = binary.LittleEndian.AppendUint16(p, 0)

	r, err := DecodeRefine(p)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if r.EquipmentSpace != 3 {
		t.Errorf("容器 = %d，期望 3", r.EquipmentSpace)
	}
	if r.EquipmentSlot != 12 {
		t.Errorf("装备槽 = %d，期望 12", r.EquipmentSlot)
	}
	if r.EquipmentTemplate != 100761210 {
		t.Errorf("模板 = %d，期望 100761210", r.EquipmentTemplate)
	}
	if r.MaterialSlot != 141 {
		t.Errorf("材料槽 = %d，期望 141", r.MaterialSlot)
	}
	if r.Name != name {
		t.Errorf("名称 = %q，期望 %q", r.Name, name)
	}
}

func TestRefineReplyLayout(t *testing.T) {
	body, err := RefineReply(141, 83, 2, 3, 0, 3, 12)
	if err != nil {
		t.Fatalf("构造回包失败: %v", err)
	}
	if len(body) != 13 {
		t.Fatalf("回包长度 = %d，期望 13", len(body))
	}
	if body[0] != 1 {
		t.Errorf("[0] 状态字节 = %d，期望 1", body[0])
	}
	if binary.LittleEndian.Uint16(body[1:]) != 141 {
		t.Errorf("[1..2] 材料槽 = %d，期望 141", binary.LittleEndian.Uint16(body[1:]))
	}
	if binary.LittleEndian.Uint32(body[3:]) != 83 {
		t.Errorf("[3..6] 剩余 = %d，期望 83", binary.LittleEndian.Uint32(body[3:]))
	}
	if body[7] != 2 || body[8] != 0 || body[9] != 3 {
		t.Errorf("[7/8/9] = %d/%d/%d，期望 2/0/3", body[7], body[8], body[9])
	}
	if body[10] != 3 {
		t.Errorf("[10] 容器 = %d，期望 3", body[10])
	}
	if binary.LittleEndian.Uint16(body[11:]) != 12 {
		t.Errorf("[11..12] 装备槽 = %d，期望 12", binary.LittleEndian.Uint16(body[11:]))
	}
}

func TestRefineReplyRejectsBadResult(t *testing.T) {
	if _, err := RefineReply(141, 83, 2, 2, 0, 3, 12); err == nil {
		t.Fatal("成功但等级未涨应被拒绝")
	}
	if _, err := RefineReply(141, 83, 2, 3, 1, 3, 12); err == nil {
		t.Fatal("失败不变但新等级≠旧等级应被拒绝")
	}
	if _, err := RefineReply(141, 83, 2, 3, 5, 3, 12); err == nil {
		t.Fatal("非法结果码 5 应被拒绝")
	}
}
