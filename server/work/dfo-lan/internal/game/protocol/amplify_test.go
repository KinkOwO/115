package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// 实机抓包回放：2026-09-27 19:39:34，玩家用自己那本增幅书 10356325 对装备试打红字。
// 原始 plaintext（45 字节）：
//
//	000000000219004e2ff8054d0065069e00030000000000000000000000000000
func TestDecodeAmplifyOptionLiveSample(t *testing.T) {
	raw, err := hex.DecodeString("000000000219004e2ff8054d0065069e00030000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeAmplifyOption(raw)
	if err != nil {
		t.Fatalf("解析增幅选项请求失败: %v", err)
	}
	if r.Reserved != 0 || r.Aux != 2 {
		t.Errorf("保留位/辅助 = %d/%d，期望 0/2", r.Reserved, r.Aux)
	}
	if r.EquipmentSlot != 25 || r.EquipmentTemplate != 100151118 {
		t.Errorf("目标装备 = 槽%d 模板%d，期望 槽25 模板100151118", r.EquipmentSlot, r.EquipmentTemplate)
	}
	if r.BookSlot != 77 || r.BookTemplate != 10356325 {
		t.Errorf("增幅书 = 槽%d 模板%d，期望 槽77 模板10356325", r.BookSlot, r.BookTemplate)
	}
	if r.Type != 3 {
		t.Errorf("次元属性类型 = %d，期望 3", r.Type)
	}
}

func TestDecodeAmplifyOptionRejectsShortPayload(t *testing.T) {
	if _, err := DecodeAmplifyOption(make([]byte, 8)); err == nil {
		t.Fatal("过短的增幅选项请求应被拒绝")
	}
}

// 回包按客户端 CMD205 handler sub_145282510 的真实读取顺序构造（18 字节）。
// 注意它不是强化回包那套 35 字节布局 —— 两者是不同的 handler。
func TestAmplifyOptionReplyShape(t *testing.T) {
	// 成功首打：类型 3，数值 +4，背包空间 0，书槽 77 剩余 4，装备槽 25。
	p := AmplifyOptionReply(0, 25, 77, 4, 3, 4)
	if len(p) != 18 {
		t.Fatalf("回包长度 = %d，期望 18", len(p))
	}
	if p[0] != 1 {
		t.Errorf("状态字节 = %d，期望 1", p[0])
	}
	if p[1] != 2 {
		t.Errorf("结果码 = %d，期望 2（播放增幅动画 + 特效）", p[1])
	}
	if binary.LittleEndian.Uint16(p[2:]) != 77 {
		t.Errorf("书槽 = %d，期望 77", binary.LittleEndian.Uint16(p[2:]))
	}
	if binary.LittleEndian.Uint32(p[4:]) != 4 {
		t.Errorf("书剩余 = %d，期望 4", binary.LittleEndian.Uint32(p[4:]))
	}
	if binary.LittleEndian.Uint32(p[8:]) != 0 {
		t.Errorf("装备容器 = %d，期望 0（背包）", binary.LittleEndian.Uint32(p[8:]))
	}
	if binary.LittleEndian.Uint16(p[12:]) != 25 {
		t.Errorf("装备槽 = %d，期望 25", binary.LittleEndian.Uint16(p[12:]))
	}
	if p[14] != 3 {
		t.Errorf("次元属性类型 = %d，期望 3", p[14])
	}
	if binary.LittleEndian.Uint16(p[15:]) != 4 {
		t.Errorf("次元属性数值 = %d，期望 4", binary.LittleEndian.Uint16(p[15:]))
	}
	if p[17] != 4 {
		t.Errorf("特效文案数值 = %d，期望 4", p[17])
	}
}

// 已穿戴的装备必须把容器 3 写进 [8..11]，否则客户端查不到装备对象、动画不播。
func TestAmplifyOptionReplyWornEquipment(t *testing.T) {
	p := AmplifyOptionReply(3, 13, 9, 1, 1, 5)
	if binary.LittleEndian.Uint32(p[8:]) != 3 {
		t.Errorf("装备容器 = %d，期望 3（已穿戴）", binary.LittleEndian.Uint32(p[8:]))
	}
	if binary.LittleEndian.Uint16(p[12:]) != 13 {
		t.Errorf("装备槽 = %d，期望 13", binary.LittleEndian.Uint16(p[12:]))
	}
	if p[14] != 1 {
		t.Errorf("次元属性类型 = %d，期望 1", p[14])
	}
	if binary.LittleEndian.Uint16(p[15:]) != 5 {
		t.Errorf("次元属性数值 = %d，期望 5", binary.LittleEndian.Uint16(p[15:]))
	}
}

// [14] 的类型一旦为 0，客户端会整段跳过应用逻辑 —— 这正是红字停在 +0 的原因。
func TestAmplifyOptionReplyTypeMustBeNonZero(t *testing.T) {
	for _, typ := range []byte{1, 2, 3, 4} {
		if p := AmplifyOptionReply(0, 25, 77, 4, typ, 4); p[14] == 0 {
			t.Errorf("类型 %d 被写成 0，客户端会跳过整段应用逻辑", typ)
		}
	}
}
