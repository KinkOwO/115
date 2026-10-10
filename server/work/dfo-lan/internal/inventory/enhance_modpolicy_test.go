package inventory

import (
	"testing"

	"dfolan/internal/modpolicy"
)

// 本文件钉的是"这条新能力有没有可能悄悄改掉既有玩法"：
// 没有 mod 设置规则时，`enhanceSuccessPercent` 必须逐位返回基值 ——
// 这样 `GoldSuccessPercent` / `SafeSuccessPercent` / `AmplifySuccessPercent` /
// `RefineSuccessPercent` 四个掷骰点的结果与加这条能力之前完全相同。

func TestEnhanceSuccessPercentPassthroughWithoutModPolicy(t *testing.T) {
	t.Cleanup(modpolicy.Reset)

	// 基线：没有任何 mod 规则（含"Servermod list 里根本没 mod"的默认态）
	for _, kind := range []string{enhanceKindReinforce, enhanceKindSafe, enhanceKindAmplify, enhanceKindRefine} {
		for level := 0; level <= 15; level++ {
			for _, base := range []int{0, 3, 8, 14, 20, 25, 30, 40, 50, 60, 70, 80, 100} {
				if got := enhanceSuccessPercent(kind, level, base); got != base {
					t.Fatalf("%s L%d base=%d → %d（必须原样）", kind, level, base, got)
				}
			}
		}
	}
}

func TestEnhanceSuccessPercentAppliesToAllFourLines(t *testing.T) {
	t.Cleanup(modpolicy.Reset)

	rules := modpolicy.EnhanceRules{
		ReinforcePercent: 200,
		SafePercent:      200,
		AmplifyPercent:   200,
		RefinePercent:    200,
		Override:         map[string]map[string]int{modpolicy.EnhanceKindSafe: {"11": 90}},
		Source:           "test",
	}
	if _, err := modpolicy.ConfigureEnhance(rules); err != nil {
		t.Fatalf("ConfigureEnhance 失败：%v", err)
	}

	cases := []struct {
		kind  string
		level int
		base  int
		want  int
	}{
		{enhanceKindReinforce, 7, 50, 100}, // 50×2 = 100（顶格）
		{enhanceKindReinforce, 12, 14, 28},
		{enhanceKindSafe, 10, 8, 16},
		{enhanceKindSafe, 11, 3, 90}, // 覆盖优先于倍率
		{enhanceKindAmplify, 4, 30, 60},
		{enhanceKindRefine, 1, 40, 80},
	}
	for _, c := range cases {
		if got := enhanceSuccessPercent(c.kind, c.level, c.base); got != c.want {
			t.Errorf("%s L%d base=%d 期望 %d，实际 %d", c.kind, c.level, c.base, c.want, got)
		}
	}
}

// 这四个常量必须与 modpolicy 的线别一致 —— 拼错一个字母，mod 配的东西会永远不生效，
// 而且日志里只显示"已生效"（正好是 AGENTS §0 最怕的那种静默半残）。
func TestEnhanceKindConstantsMatchModPolicy(t *testing.T) {
	pairs := [][2]string{
		{enhanceKindReinforce, modpolicy.EnhanceKindReinforce},
		{enhanceKindSafe, modpolicy.EnhanceKindSafe},
		{enhanceKindAmplify, modpolicy.EnhanceKindAmplify},
		{enhanceKindRefine, modpolicy.EnhanceKindRefine},
	}
	seen := map[string]bool{}
	for _, p := range pairs {
		if p[0] != p[1] {
			t.Errorf("线别常量不一致：inventory=%q modpolicy=%q", p[0], p[1])
		}
		if seen[p[0]] {
			t.Errorf("线别重复：%q", p[0])
		}
		seen[p[0]] = true
	}
}
