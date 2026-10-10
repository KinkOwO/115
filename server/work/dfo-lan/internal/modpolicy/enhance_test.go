package modpolicy

import (
	"strings"
	"testing"
)

// 本文件钉两件事：① 零值规则**逐位等价**（没有 mod 时服务端行为不许变）；
// ② 倍率与覆盖表的优先级与边界。前者比后者重要 —— 后者写错只是功能不对，
// 前者写错会让"装了能力的那天"悄悄改掉所有人的强化结果。

func TestEnhanceZeroValueIsIdentity(t *testing.T) {
	t.Cleanup(Reset)

	var zero EnhanceRules
	for _, kind := range enhanceKinds {
		for level := 0; level <= 15; level++ {
			for _, base := range []int{0, 1, 3, 8, 14, 30, 50, 70, 80, 100} {
				if got := zero.SuccessPercent(kind, level, base); got != base {
					t.Fatalf("零值规则改变了结果：%s L%d base=%d → %d", kind, level, base, got)
				}
			}
		}
	}
	if zero.Enabled() {
		t.Fatal("零值规则不应报告开启")
	}
	// 未 Known 线别也不能偷偷动手
	if got := zero.SuccessPercent("unknown-kind", 1, 42); got != 42 {
		t.Fatalf("未知线别应当原样返回，得到 %d", got)
	}
}

func TestEnhanceSuccessPercentPriority(t *testing.T) {
	t.Cleanup(Reset)

	rules := EnhanceRules{
		ReinforcePercent: 200, // ×2
		AmplifyPercent:   50,  // ×0.5
		Override: map[string]map[string]int{
			EnhanceKindReinforce: {"10": 66},
			EnhanceKindRefine:    {"0": 100},
		},
		Source: "test",
	}
	if got := rules.SuccessPercent(EnhanceKindReinforce, 9, 25); got != 50 {
		t.Fatalf("×2 应得 50，实际 %d", got)
	}
	// 覆盖优先于倍率：10 级被钉成 66，不是 25×2=50
	if got := rules.SuccessPercent(EnhanceKindReinforce, 10, 25); got != 66 {
		t.Fatalf("覆盖应得 66，实际 %d", got)
	}
	if got := rules.SuccessPercent(EnhanceKindAmplify, 3, 80); got != 40 {
		t.Fatalf("×0.5 应得 40，实际 %d", got)
	}
	// 只有 Refine 的覆盖生效（它没有倍率），别处不动
	if got := rules.SuccessPercent(EnhanceKindRefine, 0, 7); got != 100 {
		t.Fatalf("锻造覆盖应得 100，实际 %d", got)
	}
	// Safe 既没倍率也没覆盖 → 原样
	if got := rules.SuccessPercent(EnhanceKindSafe, 11, 3); got != 3 {
		t.Fatalf("安全强化应原样返回 3，实际 %d", got)
	}
}

func TestEnhanceSuccessPercentClamps(t *testing.T) {
	t.Cleanup(Reset)

	rules := EnhanceRules{
		ReinforcePercent: 1000, // ×10
		Override:         map[string]map[string]int{EnhanceKindAmplify: {"1": 120, "2": -5}},
		Source:           "test",
	}
	if got := rules.SuccessPercent(EnhanceKindReinforce, 5, 70); got != 100 {
		t.Fatalf("倍率结果必须 clamp 到 100，实际 %d", got)
	}
	if got := rules.SuccessPercent(EnhanceKindAmplify, 1, 10); got != 100 {
		t.Fatalf("覆盖 120 必须 clamp 到 100，实际 %d", got)
	}
	if got := rules.SuccessPercent(EnhanceKindAmplify, 2, 10); got != 0 {
		t.Fatalf("覆盖 -5 必须 clamp 到 0，实际 %d", got)
	}
	// 基值为 0 时任何倍率都不能凭空造出成功率
	if got := rules.SuccessPercent(EnhanceKindReinforce, 0, 0); got != 0 {
		t.Fatalf("基值 0 必须仍是 0，实际 %d", got)
	}
}

func TestConfigureEnhanceRejectsAuthorErrors(t *testing.T) {
	t.Cleanup(Reset)

	cases := []struct {
		name string
		r    EnhanceRules
		want string
	}{
		{"开启了却没署名", EnhanceRules{ReinforcePercent: 200}, "Source"},
		{"未知线别", EnhanceRules{Override: map[string]map[string]int{"reinforc": {"1": 50}}, Source: "t"}, "未知的装备成长线别"},
		{"显式值越界", EnhanceRules{Override: map[string]map[string]int{EnhanceKindReinforce: {"1": 101}}, Source: "t"}, "超出 0..100"},
		{"显式值负数", EnhanceRules{Override: map[string]map[string]int{EnhanceKindRefine: {"1": -1}}, Source: "t"}, "超出 0..100"},
		{"倍率越界", EnhanceRules{AmplifyPercent: 1001, Source: "t"}, "超出上限"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ConfigureEnhance(c.r); err == nil {
				t.Fatalf("应当拒绝，但没有：%+v", c.r)
			} else if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("错误信息应当包含 %q，实际：%v", c.want, err)
			}
			if Enhance().Enabled() {
				t.Fatal("校验失败后不得留下半成品规则")
			}
		})
	}
}

func TestConfigureEnhanceRoundTripAndReset(t *testing.T) {
	t.Cleanup(Reset)

	rules := EnhanceRules{SafePercent: 150, Source: "  enhance.rate  "}
	applied, err := ConfigureEnhance(rules)
	if err != nil {
		t.Fatalf("配置失败：%v", err)
	}
	if applied.Source != "enhance.rate" {
		t.Fatalf("Source 应当被 trim，实际 %q", applied.Source)
	}
	if got := Enhance().SuccessPercent(EnhanceKindSafe, 10, 8); got != 12 {
		t.Fatalf("安全强化 ×1.5 应得 12，实际 %d", got)
	}
	desc := Enhance().String()
	if !strings.Contains(desc, "安全强化") || !strings.Contains(desc, "enhance.rate") {
		t.Fatalf("摘要信息不完整：%s", desc)
	}
	Reset()
	if Enhance().Enabled() {
		t.Fatal("Reset 没能清干净")
	}
	if Enhance().SuccessPercent(EnhanceKindSafe, 10, 8) != 8 {
		t.Fatal("Reset 之后必须回到原样")
	}
}

func TestEnhanceDisabledString(t *testing.T) {
	t.Cleanup(Reset)
	if got := (EnhanceRules{}).String(); !strings.Contains(got, "关闭") {
		t.Fatalf("零值摘要应当写明关闭，实际 %q", got)
	}
	if got := (EnhanceRules{RefinePercent: 300, Source: "t"}).String(); !strings.Contains(got, "锻造") || !strings.Contains(got, "×3.00") {
		t.Fatalf("摘要应当写明锻造 ×3.00，实际 %q", got)
	}
}
