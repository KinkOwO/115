package protocol

import (
	"encoding/binary"
	"testing"
)

// 增幅回包（CMD80 mode=1）与强化共用客户端同一个 handler（sub_14529B2F0），
// 所以布局一致、非摧毁时长度也是 35 字节。
// ⚠️ [1] 只在摧毁（result==3）时才写成 11，其余必须回显请求 mode=1。
func TestAmplifyUpgradeReplyShape(t *testing.T) {
	r := ReinforcementRequest{
		Mode:              1,
		EquipmentSpace:    0,
		EquipmentSlot:     19,
		EquipmentTemplate: 100051304,
		TicketSpace:       0,
		TicketSlot:        142,
	}
	p, err := AmplifyUpgradeReply(r, 41, 4, 5, 0)
	if err != nil {
		t.Fatalf("构造增幅回包失败: %v", err)
	}
	if len(p) != 35 {
		t.Fatalf("回包长度 = %d，期望 35", len(p))
	}
	if p[0] != 1 {
		t.Errorf("[0] 状态字节 = %d，期望 1", p[0])
	}
	// 非摧毁：回显 mode=1。写 11 会让客户端 dynamic_cast 到 off_14DCB71E8 失败，
	// 进不去结果分支，掉到公共落点弹 1658「No items are available.」。
	if p[1] != 1 {
		t.Errorf("[1] 升级种类 = %d，期望 1（非摧毁回显请求 mode）", p[1])
	}
	if binary.LittleEndian.Uint16(p[2:]) != 142 {
		t.Errorf("[2..3] 材料槽 = %d，期望 142", binary.LittleEndian.Uint16(p[2:]))
	}
	if binary.LittleEndian.Uint32(p[4:]) != 41 {
		t.Errorf("[4..7] 材料剩余 = %d，期望 41", binary.LittleEndian.Uint32(p[4:]))
	}
	if binary.LittleEndian.Uint16(p[8:]) != 0xffff {
		t.Errorf("[8..9] = %x，期望 0xffff", binary.LittleEndian.Uint16(p[8:]))
	}
	if p[10] != 1 {
		t.Errorf("[10] = %d，期望 1", p[10])
	}
	if p[11] != 4 {
		t.Errorf("[11] 旧增幅等级 = %d，期望 4", p[11])
	}
	if p[12] != 0 {
		t.Errorf("[12] 结果 = %d，期望 0（成功）", p[12])
	}
	if p[13] != 5 {
		t.Errorf("[13] 新增幅等级 = %d，期望 5", p[13])
	}
	if p[14] != 0 {
		t.Errorf("[14] 装备空间 = %d，期望 0", p[14])
	}
	if binary.LittleEndian.Uint16(p[15:]) != 19 {
		t.Errorf("[15..16] 装备槽 = %d，期望 19", binary.LittleEndian.Uint16(p[15:]))
	}
}

// 结果码必须与等级变化自洽 —— 不一致时客户端会判定异常并锁死增幅窗口：
// 0 成功(新=旧+1)、1 失败不变(新=旧)、2 失败降级(旧>新)、3 失败摧毁。
func TestAmplifyUpgradeReplyResultMatchesLevelChange(t *testing.T) {
	r := ReinforcementRequest{Mode: 1, TicketSlot: 142}
	if _, err := AmplifyUpgradeReply(r, 10, 5, 6, 0); err != nil {
		t.Errorf("成功回包 5→6 应通过: %v", err)
	}
	if _, err := AmplifyUpgradeReply(r, 10, 5, 5, 1); err != nil {
		t.Errorf("失败不变 5→5 应通过: %v", err)
	}
	if _, err := AmplifyUpgradeReply(r, 10, 9, 8, 2); err != nil {
		t.Errorf("失败降级 9→8 应通过: %v", err)
	}
	if _, err := AmplifyUpgradeReply(r, 10, 11, 0, 3); err != nil {
		t.Errorf("失败摧毁应通过: %v", err)
	}
	// 下面几种不一致必须被拒绝。
	if _, err := AmplifyUpgradeReply(r, 10, 5, 5, 0); err == nil {
		t.Error("成功却等级没涨，应被拒绝")
	}
	if _, err := AmplifyUpgradeReply(r, 10, 9, 8, 1); err == nil {
		t.Error("result=1（等级不变）却给了降级，应被拒绝")
	}
	if _, err := AmplifyUpgradeReply(r, 10, 5, 6, 2); err == nil {
		t.Error("result=2（降级）却是升级，应被拒绝")
	}
}

// 增幅回包必须带 mode=1，否则会退化成强化语义。
func TestAmplifyUpgradeReplyRejectsOtherMode(t *testing.T) {
	r := ReinforcementRequest{Mode: 0}
	if _, err := AmplifyUpgradeReply(r, 1, 0, 1, 0); err == nil {
		t.Fatal("mode != 1 的增幅回包应被拒绝")
	}
	r.Mode = 10
	if _, err := AmplifyUpgradeReply(r, 1, 0, 1, 0); err == nil {
		t.Fatal("mode 10 不是增幅，应被拒绝")
	}
}

// 摧毁（result=3）必须换成 36 字节布局：客户端 sub_14529B2F0 的摧毁分支
// （dumps/cmd205-reply/cmd205_handler80.c 的 LABEL_149）先读 [17] 当「额外条目数」，
// 再按 (u16, u32, u32) 循环读那么多条。留成非摧毁布局的 0xffff 会让客户端
// 去读 255 条（2500 字节），把 35 字节的包直接读爆 —— 这就是装备碎掉后客户端
// 发 CMD217（wire overflow）并卡死的根因。
func TestAmplifyUpgradeReplyDestroyLayout(t *testing.T) {
	r := ReinforcementRequest{Mode: 1, EquipmentSpace: 0, EquipmentSlot: 49,
		EquipmentTemplate: 100251132, TicketSlot: 142}
	p, err := AmplifyUpgradeReply(r, 4228, 11, 11, 3)
	if err != nil {
		t.Fatalf("构造摧毁回包失败: %v", err)
	}
	if len(p) != 36 {
		t.Fatalf("摧毁回包长度 = %d，期望 36", len(p))
	}
	// 摧毁必须写 11：客户端 `if (v52 || v149 == 3)` 里 v149==3 是「或」条件，
	// 保证能进 11 分支，摧毁对话框和 36 字节尾部布局都在分支里面。
	if p[1] != cmd80KindAmplify {
		t.Errorf("[1] 升级种类 = %d，期望 %d（摧毁时才硬闯增幅分支）", p[1], cmd80KindAmplify)
	}
	if p[17] != 0 {
		t.Errorf("[17] 额外条目数 = %d，期望 0（否则客户端会按条数继续读爆包）", p[17])
	}
	if binary.LittleEndian.Uint16(p[18:]) != 0xffff {
		t.Errorf("[18..19] 槽位 = %x，期望 0xffff", binary.LittleEndian.Uint16(p[18:]))
	}
	// 成功/失败/降级仍是 35 字节，[17..18] 直接就是槽位。
	for _, result := range []byte{0, 1, 2} {
		old, neu := byte(4), byte(5)
		if result == 1 {
			neu = old
		}
		if result == 2 {
			old, neu = 8, 7
		}
		q, err := AmplifyUpgradeReply(r, 10, old, neu, result)
		if err != nil {
			t.Fatalf("result=%d 构造失败: %v", result, err)
		}
		if len(q) != 35 {
			t.Errorf("result=%d 回包长度 = %d，期望 35", result, len(q))
		}
		if binary.LittleEndian.Uint16(q[17:]) != 0xffff {
			t.Errorf("result=%d [17..18] = %x，期望 0xffff", result, binary.LittleEndian.Uint16(q[17:]))
		}
		if q[1] != 1 {
			t.Errorf("result=%d [1] = %d，期望 1（非摧毁回显 mode）", result, q[1])
		}
	}
}

// cmd80UpgradeKind 的完整真值表。这是 2026-09-28「强化和增幅都被修坏了」的回归护栏：
// 摧毁才升级成 10/11，其余一律回显 mode。
func TestCmd80UpgradeKind(t *testing.T) {
	cases := []struct {
		mode, result, want byte
	}{
		{0, 0, 0},  // 强化成功：回显 mode
		{0, 1, 0},  // 强化失败不变
		{0, 2, 0},  // 强化降级
		{0, 3, 10}, // 强化摧毁
		{1, 0, 1},  // 增幅成功：回显 mode
		{1, 1, 1},  // 增幅失败不变
		{1, 2, 1},  // 增幅降级
		{1, 3, 11}, // 增幅摧毁
	}
	for _, c := range cases {
		if got := cmd80UpgradeKind(c.mode, c.result); got != c.want {
			t.Errorf("cmd80UpgradeKind(mode=%d, result=%d) = %d，期望 %d",
				c.mode, c.result, got, c.want)
		}
	}
}
