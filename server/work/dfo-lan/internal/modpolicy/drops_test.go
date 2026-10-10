package modpolicy

import (
	"strings"
	"testing"
)

// TestDropsDefaultIsOff 是这套机制的安全前提：没有 mod 设置时倍率必须全关
// —— 装上 mod 之前/卸下之后，掉落行为要和基线一致（零值 = 1.00 倍）。
func TestDropsDefaultIsOff(t *testing.T) {
	defer Reset()
	Reset()
	if Drops().Enabled() {
		t.Fatal("零值必须是全关")
	}
	if s := Drops().String(); !strings.Contains(s, "关闭") {
		t.Fatalf("关闭态摘要应写清楚：%s", s)
	}
	if s := (DropRules{}).String(); !strings.Contains(s, "关闭") {
		t.Fatalf("零值 String() 不应 panic 且要写清楚：%s", s)
	}
}

// TestDropsConfigureRoundTrip 覆盖设置、回显、读取快照与撤销。
func TestDropsConfigureRoundTrip(t *testing.T) {
	defer Reset()
	Reset()
	ConfigureDrops(DropRules{WorldPercent: 500, MonsterItemPercent: 500, Source: "odyssey.hardcore"})
	got := Drops()
	if !got.Enabled() || got.WorldPercent != 500 || got.MonsterItemPercent != 500 || got.Source != "odyssey.hardcore" {
		t.Fatalf("设置与读取不一致：%+v", got)
	}
	s := got.String()
	for _, want := range []string{"世界掉落", "×5.00", "小怪专属物品池", "odyssey.hardcore"} {
		if !strings.Contains(s, want) {
			t.Errorf("摘要缺少 %q：%s", want, s)
		}
	}
	// Source 前后空白要清掉（照 Configure 的规则）。
	ConfigureDrops(DropRules{WorldPercent: 100, Source: "  odyssey.hardcore  "})
	if got := Drops().Source; got != "odyssey.hardcore" {
		t.Fatalf("Source 未 trim：%q", got)
	}
	// 撤销：回到全关。
	ConfigureDrops(DropRules{})
	if Drops().Enabled() {
		t.Fatal("撤销后应当全关")
	}
}

// TestDropsStringNeverPanics 覆盖各种取值下的 String()（启动日志无条件调用它）。
func TestDropsStringNeverPanics(t *testing.T) {
	for _, r := range []DropRules{
		{},
		{WorldPercent: 1},
		{MonsterItemPercent: 1},
		{WorldPercent: 4294967295, MonsterItemPercent: 4294967295},
		{WorldPercent: 100, MonsterItemPercent: 100, Source: "x"},
	} {
		if s := r.String(); s == "" {
			t.Fatalf("String() 不该为空：%+v", r)
		}
	}
	// 未署名必须显示出来：日志里要能看出"规则没写主人"。
	if s := (DropRules{WorldPercent: 200}).String(); !strings.Contains(s, "未署名") {
		t.Fatalf("无来源时应显示未署名：%s", s)
	}
}

// TestResetClearsDrops 钉住 runGateway 依赖的生命周期语义：
// 进出各 Reset 一次，掉落倍率也必须被清掉（否则"上一台服务端的倍率"会漏给下一次）。
func TestResetClearsDrops(t *testing.T) {
	ConfigureDrops(DropRules{WorldPercent: 500, MonsterItemPercent: 500, Source: "leak"})
	Reset()
	if Drops().Enabled() {
		t.Fatal("Reset 必须清空掉落倍率")
	}
}
