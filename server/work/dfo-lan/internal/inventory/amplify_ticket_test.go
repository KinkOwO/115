package inventory

// 增幅券规则的回归测试：数据来自 configs/amplify-tickets.json
// （scripts/export_amplify_tickets.py 从 PVF 的 [equipment amplify reinforcement ticket] 只读导出）。

import (
	"path/filepath"
	"testing"
)

func loadAmplifyTicketsForTest(t *testing.T) {
	t.Helper()
	if len(amplifyTickets) > 0 {
		return
	}
	path := filepath.Join("..", "..", "configs", "amplify-tickets.json")
	if err := LoadAmplifyTickets(path); err != nil {
		t.Fatalf("装载增幅券规则失败: %v", err)
	}
	if len(amplifyTickets) == 0 {
		t.Skip("configs/amplify-tickets.json 不存在，跳过增幅券用例")
	}
}

// 段值格式：[目标等级, 成功率百分比, 'fixed', -1]。
// 实机锚点：10_amplifying_ticket_probability90 = [10, 90, 'fixed', -1]，
// amplify_ticket_7（epic_road）= [7, 100, 'fixed', -1]。
func TestAmplifyTicketSpec(t *testing.T) {
	loadAmplifyTicketsForTest(t)
	if !IsAmplifyTicket(50022659) {
		t.Fatal("50022659（10_amplifying_ticket_probability90）应被识别为增幅券")
	}
	level, percent, ok := amplifyTicketSpec(50022659)
	if !ok || level != 10 || percent != 90 {
		t.Errorf("+10/90%% 券解析 = (%d,%d,%v)，期望 (10,90,true)", level, percent, ok)
	}
	level, percent, ok = amplifyTicketSpec(50024309)
	if !ok || level != 7 || percent != 100 {
		t.Errorf("+7/100%% 券解析 = (%d,%d,%v)，期望 (7,100,true)", level, percent, ok)
	}
	// 增幅材料不是券：分流就靠这个区别（券 → 跳级，材料 → 每级 +1）。
	if IsAmplifyTicket(3242) {
		t.Error("3242（矛盾结晶体）是增幅材料，不应当增幅券")
	}
	if IsAmplifyMaterial(50022659) {
		t.Error("增幅券不应被当成增幅材料")
	}
}

// 库里存在少量把目标等级写到 16..20 的异常券。它们必须：
//  1. 在装载阶段被放行（LoadAmplifyTickets 只校验结构，否则 1629 张券会整体失效）；
//  2. 在使用阶段被 amplifyTicketSpec 单独拒绝（等级字节只有低五位，客户端最高 15）。
func TestAmplifyTicketRejectsOutOfRangeLevel(t *testing.T) {
	loadAmplifyTicketsForTest(t)
	accepted, rejected := 0, 0
	for template := range amplifyTickets {
		level, _, ok := amplifyTicketSpec(template)
		if !ok {
			rejected++
			continue
		}
		accepted++
		if level < 1 || level > 15 {
			t.Errorf("券 %d 的等级 %d 越界却被接受", template, level)
		}
	}
	if accepted == 0 {
		t.Fatal("整张券表没有一张可用，装载或段值格式有问题")
	}
	t.Logf("增幅券：可用 %d 张，越界/不支持 %d 张", accepted, rejected)
}

// 增幅券与「普通强化券」是两套道具（段名不同），不能互相串：
// 前者 [equipment amplify reinforcement ticket]，后者 [equipment reinforcement ticket]。
func TestAmplifyTicketSectionIsDistinct(t *testing.T) {
	loadAmplifyTicketsForTest(t)
	for template, rule := range amplifyTickets {
		if len(rule.Fields[amplifyTicketSection]) != 4 {
			t.Fatalf("券 %d 的增幅券段应有 4 个 token", template)
		}
		if _, isReinforce := rule.Fields["[equipment reinforcement ticket]"]; isReinforce {
			t.Errorf("券 %d 同时带普通强化券段，分流会歧义", template)
		}
	}
}
