package main

import "testing"

// 2026-10-01（next146）：直读模式下抽奖目录的来源身份令牌必须能从编译期常量
// 切到当次内层 checksum，否则整族在直读启动时被拦下。这里锁定三件事：
//  1. 显式钉的旧版本仍被接受（保留手工钉版本意图）；
//  2. 空来源 = 接受并派生为当前令牌（直读派生路径）；
//  3. 别的 64 位哈希 = 拒绝（不能把任意客户端版本混进来）。
func TestLotterySourceAcceptsDerivedAndExplicit(t *testing.T) {
	original := lotterySourcePVFSHA256
	t.Cleanup(func() { lotterySourcePVFSHA256 = original })

	// 默认令牌下：空串与自身均接受，异构哈希拒绝。
	if !lotterySourceAccepts("") {
		t.Fatalf("empty source must be accepted (derive mode)")
	}
	if !lotterySourceAccepts(lotterySourcePVFSHA256) {
		t.Fatalf("current token must be accepted")
	}
	if lotterySourceAccepts("0000000000000000000000000000000000000000000000000000000000000000") {
		t.Fatalf("foreign pinned source must be refused")
	}

	// 切到当次内层 checksum 后，旧常量不再被接受、新值被接受。
	derived := "b2b503b58ca12ed88befaa2d44b6d222cfb15e94aaff7a7dfa32f6fa0133b13e"
	SetLotterySource(derived)
	if lotterySourcePVFSHA256 != derived {
		t.Fatalf("SetLotterySource did not switch the token: %s", lotterySourcePVFSHA256)
	}
	if !lotterySourceAccepts(derived) {
		t.Fatalf("derived token must be accepted after switch")
	}
	if lotterySourceAccepts(original) {
		t.Fatalf("stale pinned source must be refused after switch")
	}
}

// 非 64 位十六进制一律忽略，避免把垃圾值灌进来源令牌。
func TestSetLotterySourceIgnoresGarbage(t *testing.T) {
	original := lotterySourcePVFSHA256
	t.Cleanup(func() { lotterySourcePVFSHA256 = original })

	for _, bad := range []string{"", "abc", "b2b503b58ca12ed88befaa2d44b6d222cfb15e94aaff7a7dfa32f6fa0133b13", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"} {
		SetLotterySource(bad)
		if lotterySourcePVFSHA256 != original {
			t.Fatalf("garbage %q changed the token", bad)
		}
	}
}
