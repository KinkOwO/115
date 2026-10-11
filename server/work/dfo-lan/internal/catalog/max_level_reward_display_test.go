package catalog

import "testing"

func TestMaxLevelRewardTextFollowsConfiguredCap(t *testing.T) {
	r := &MaxLevelReward{Text: "您现在已经达到86级。盒子861，数量2。"}
	for _, tt := range []struct {
		cap  byte
		want string
	}{{115, "您现在已经达到115级。盒子861，数量2。"}, {120, "您现在已经达到120级。盒子861，数量2。"}, {0, "您现在已经达到86级。盒子861，数量2。"}} {
		if got := r.TextForLevelCap(tt.cap); got != tt.want {
			t.Fatalf("cap=%d got=%q", tt.cap, got)
		}
	}
	if r.Text != "您现在已经达到86级。盒子861，数量2。" {
		t.Fatal("source localization mutated")
	}
	r.Text = "Current max level reward."
	if got := r.TextForLevelCap(115); got != r.Text {
		t.Fatal("unrelated source wording changed")
	}
}
