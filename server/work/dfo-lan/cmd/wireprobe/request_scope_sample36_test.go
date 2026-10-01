package main

import "testing"

// The body sampler is the evidence path for every feature this build does not
// implement: mail, avatars, the shop and item use all send commands whose
// payloads the old whitelist discarded outright.
func TestBodySamplerRetainsUnimplementedCommands(t *testing.T) {
	seen := map[uint16]int{}

	// An implemented command is always retained and never consumes a sample.
	for i := 0; i < BodySampleLimit+5; i++ {
		if !retainRequestBody(35, seen) {
			t.Fatal("implemented command body was dropped")
		}
	}
	if seen[35] != 0 {
		t.Fatalf("implemented command consumed %d samples", seen[35])
	}

	// An unimplemented command is sampled up to the cap, then counted only.
	kept := 0
	for i := 0; i < BodySampleLimit+20; i++ {
		if retainRequestBody(407, seen) {
			kept++
		}
	}
	if kept != BodySampleLimit {
		t.Fatalf("unimplemented command retained %d bodies, want %d", kept, BodySampleLimit)
	}

	// The cap is per command, so a second feature is not starved by the first.
	if !retainRequestBody(2377, seen) {
		t.Fatal("a second unimplemented command was starved by the first")
	}

	// A high-frequency telemetry command must not be able to flood the log.
	for i := 0; i < 5000; i++ {
		retainRequestBody(2127, seen)
	}
	if seen[2127] != BodySampleLimit {
		t.Fatalf("telemetry command retained %d bodies", seen[2127])
	}
	if retainRequestBody(2127, nil) {
		t.Fatal("a nil counter must not retain an unimplemented body")
	}
}

// 2026-09-27 回归：CMD2329 是定盘机关判死兜底唯一的状态源，却被 8 次采样上限截断，
// 此后每条都因为 `verified` 从未算过而被拒（理由还写成 checksum failed）—— 实测一场
// 20 条里 12 条这样丢掉，恰好让「血量峰值」判据失效。它必须不受采样限制。
func TestMonsterHistoryLogIsExemptFromTheBodySampleCap(t *testing.T) {
	seen := map[uint16]int{2329: BodySampleLimit}
	for i := 0; i < BodySampleLimit+5; i++ {
		if !retainRequestBody(2329, seen) {
			t.Fatal("the scale's status log stopped being verified after the cap")
		}
	}
	if seen[2329] != BodySampleLimit {
		t.Fatalf("the exemption consumed samples: %d", seen[2329])
	}
}

// 2026-10-02 回归：CMD2258（装备调适）漏了 `observedGameRequest` 豁免，于是同一会话第 8 次
// 之后（恰好 BodySampleLimit）`verified` 不再被计算，之后所有调适请求都在 wire 层被判
// "请求校验失败" → 客户端表现「调适 8 次后无法继续、重进客户端又能再来 8 次」，而同族的
// CMD2259 当时已在豁免列表里。
//
// 除了补豁免，`main.go` 里 `verified` 的计算已与 `retainRequestBody` **解耦**（业务判定不再
// 依赖日志采样配额），所以这类"漏登记就静默失效"的问题不会再出现；本测试锁住豁免本身。
func TestAwakeningPromoteIsExemptFromTheBodySampleCap(t *testing.T) {
	seen := map[uint16]int{2258: BodySampleLimit}
	for i := 0; i < BodySampleLimit+5; i++ {
		if !retainRequestBody(2258, seen) {
			t.Fatal("装备调适请求在采样上限之后必须仍然被保留并校验")
		}
	}
	if seen[2258] != BodySampleLimit {
		t.Fatalf("the exemption consumed samples: %d", seen[2258])
	}
	// 同族命令（CMD2259 装备转换）本来就在豁免里，钉住它别被顺手改掉。
	if !retainRequestBody(2259, nil) {
		t.Fatal("CMD2259 必须在 observedGameRequest 豁免列表里")
	}
}
