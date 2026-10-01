package catalog

import "testing"

// OdysseySource 在 2026-10-01（next146）由 const 改为 var + SetOdysseySource。
//
// 背景：PVF 直读模式下所有目录都从当次内层归档构建，角色的 ConfigVersion 也等于该
// 内层 checksum（character/service.go）。若奥德赛系的「源身份」令牌仍钉在历史常量
// `7ef2db59…`，整个奥德赛家族在直读启动时会被门禁拦下（实机首个撞墙点 =
// `Odyssey journal routes source mismatch`）。
//
// 本测试锁定：合法 64 位 hex 生效；非法值（空/短/非 hex）被忽略并保持原值 ——
// 因为它是**写入存档的运行时身份**，不能让空串污染 role.ConfigVersion。
func TestSetOdysseySourceValidatesAndIgnoresGarbage(t *testing.T) {
	original := OdysseySource
	t.Cleanup(func() { OdysseySource = original })

	const good = "b2b503b58ca12ed88befaa2d44b6d222cfb15e94aaff7a7dfa32f6fa0133b13e"
	SetOdysseySource(good)
	if OdysseySource != good {
		t.Fatalf("valid checksum not applied: got %q", OdysseySource)
	}

	for _, bad := range []string{"", "short", good[:63], good + "x", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"} {
		SetOdysseySource(bad)
		if OdysseySource != good {
			t.Fatalf("garbage %q must be ignored, got %q", bad, OdysseySource)
		}
	}

	// 大写 hex 也要接受（大小写等价）。
	const upper = "B2B503B58CA12ED88BEFAA2D44B6D222CFB15E94AAFF7A7DFA32F6FA0133B13E"
	SetOdysseySource(upper)
	if OdysseySource != upper {
		t.Fatalf("uppercase hex not applied: got %q", OdysseySource)
	}
}
