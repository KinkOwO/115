package protocol

import "testing"

// 增幅券是**跳级**券：+0 的装备用 +10 券成功后，新等级直接就是 10，而不是 old+1。
// AmplifyUpgradeReply 强制 new == old+1，会把这种回包判为「结果不自洽」而整单拒绝
// —— 这正是「增幅券不能直接附加到装备上」的直接原因，所以这里要钉死两者的差别。
func TestAmplifyTicketReplyAllowsJump(t *testing.T) {
	r := ReinforcementRequest{Mode: 1, EquipmentSpace: 0, TicketSlot: 5, EquipmentSlot: 9}
	p, err := AmplifyTicketReply(r, 3, 0, 10, 0)
	if err != nil {
		t.Fatalf("+0 用 +10 增幅券成功应被接受，实际报错: %v", err)
	}
	if len(p) != 35 {
		t.Errorf("回包长度 = %d，期望 35（非摧毁布局）", len(p))
	}
	// 非摧毁结果 [1] 必须回显 mode=1：写成 11 会进不去分支并弹 dstr 1658。
	if p[1] != 1 {
		t.Errorf("回包 [1] = %d，非摧毁应回显 mode=1", p[1])
	}
	if p[0] != 1 {
		t.Errorf("回包 [0] = %d，状态字节必须是 1", p[0])
	}
	if p[11] != 0 || p[12] != 0 || p[13] != 10 {
		t.Errorf("回包等级字段错误：old=%d result=%d new=%d，期望 0/0/10", p[11], p[12], p[13])
	}
	// 同一组等级在「材料增幅」回包里必须被拒绝 —— 两条路径的差别就在这里。
	if _, err = AmplifyUpgradeReply(r, 3, 0, 10, 0); err == nil {
		t.Error("材料增幅回包不该接受跳级（它要求 new == old+1）")
	}
}

func TestAmplifyTicketReplyRejectsBadInput(t *testing.T) {
	r := ReinforcementRequest{Mode: 1, TicketSlot: 5, EquipmentSlot: 9}
	// mode 必须是 1（增幅）。
	if _, err := AmplifyTicketReply(ReinforcementRequest{Mode: 0}, 3, 0, 10, 0); err == nil {
		t.Error("mode=0 的回包应被拒绝")
	}
	// 成功但没有真的往上走。
	if _, err := AmplifyTicketReply(r, 3, 10, 10, 0); err == nil {
		t.Error("成功回包的新等级必须高于旧等级")
	}
	if _, err := AmplifyTicketReply(r, 3, 11, 10, 0); err == nil {
		t.Error("成功回包的新等级低于旧等级应被拒绝")
	}
	// 目标等级超出一个等级字节能表达的范围。
	if _, err := AmplifyTicketReply(r, 3, 0, 16, 0); err == nil {
		t.Error("目标等级 16 超出客户端支持范围，应被拒绝")
	}
	// 失败（只消耗券）必须等级不变。
	if _, err := AmplifyTicketReply(r, 3, 5, 6, 1); err == nil {
		t.Error("失败（等级不变）回包要求新旧等级相同")
	}
	if _, err := AmplifyTicketReply(r, 3, 5, 5, 1); err != nil {
		t.Errorf("失败且等级不变的回包应被接受，实际: %v", err)
	}
	// 固定券不降级也不摧毁。
	if _, err := AmplifyTicketReply(r, 3, 5, 4, 2); err == nil {
		t.Error("增幅券不降级，result=2 应被拒绝")
	}
	if _, err := AmplifyTicketReply(r, 3, 5, 4, 3); err == nil {
		t.Error("增幅券不摧毁，result=3 应被拒绝")
	}
}

// 券槽与剩余量要回显给客户端（客户端靠它决定要不要把最后一格移除）。
func TestAmplifyTicketReplyEchoesTicket(t *testing.T) {
	r := ReinforcementRequest{Mode: 1, TicketSlot: 0x1234, EquipmentSlot: 0x4321}
	p, err := AmplifyTicketReply(r, 7, 2, 10, 0)
	if err != nil {
		t.Fatalf("回包构造失败: %v", err)
	}
	if got := int(p[2]) | int(p[3])<<8; got != 0x1234 {
		t.Errorf("回包券槽 = %#x，期望 0x1234", got)
	}
	if got := uint32(p[4]) | uint32(p[5])<<8 | uint32(p[6])<<16 | uint32(p[7])<<24; got != 7 {
		t.Errorf("回包券剩余 = %d，期望 7", got)
	}
	if got := int(p[15]) | int(p[16])<<8; got != 0x4321 {
		t.Errorf("回包装备槽 = %#x，期望 0x4321", got)
	}
}
