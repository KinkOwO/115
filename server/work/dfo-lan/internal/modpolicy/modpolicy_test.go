package modpolicy

import (
	"strings"
	"testing"
)

// TestDefaultIsOff 是这套机制的安全前提：没有 mod 设置时，规则必须全关
// —— 装上 mod 之前/卸下之后，服务端行为要和基线一致。
func TestDefaultIsOff(t *testing.T) {
	defer Reset()
	Reset()
	if Odyssey().Enabled() {
		t.Fatal("零值必须是全关")
	}
	if s := Odyssey().String(); !strings.Contains(s, "关闭") {
		t.Fatalf("关闭态摘要应写清楚：%s", s)
	}
}

// TestConfigureRequiresSource 挡掉"无主规则"：开了规则却不说谁开的，
// 出问题时日志里会看不出是谁改的玩法。
func TestConfigureRequiresSource(t *testing.T) {
	defer Reset()
	if _, err := Configure(OdysseyRules{BanConsumables: true}); err == nil {
		t.Fatal("开启规则却没写 Source，应当报错")
	}
	if _, err := Configure(OdysseyRules{BanReviveCoin: true, Source: "   "}); err == nil {
		t.Fatal("只有空白 Source 也应当报错")
	}
	// 全关时允许不写来源（撤销路径）
	if got, err := Configure(OdysseyRules{}); err != nil || got.Enabled() {
		t.Fatalf("撤销不该报错：%+v %v", got, err)
	}
}

// TestConfigureRoundTrip 覆盖设置、回显、读取快照与撤销。
func TestConfigureRoundTrip(t *testing.T) {
	defer Reset()
	got, err := Configure(OdysseyRules{
		BanConsumables: true,
		BanReviveCoin:  true,
		Source:         "odyssey.hardcore",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Enabled() || Odyssey() != got {
		t.Fatalf("回显与读取不一致：%+v / %+v", got, Odyssey())
	}
	s := got.String()
	for _, want := range []string{"禁用消耗品", "禁用复活", "odyssey.hardcore"} {
		if !strings.Contains(s, want) {
			t.Errorf("摘要缺少 %q：%s", want, s)
		}
	}
	if _, err := Configure(OdysseyRules{}); err != nil {
		t.Fatal(err)
	}
	if Odyssey().Enabled() {
		t.Fatal("撤销后应当全关")
	}
}
