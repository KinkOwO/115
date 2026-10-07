package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/json"
	"strings"
	"testing"
)

func maxLevelRewardFixture() *catalog.MaxLevelReward {
	return &catalog.MaxLevelReward{
		Source:     strings.Repeat("a", 64),
		Definition: catalog.ScriptRecord{Path: catalog.MaxLevelRewardPath, SHA256: strings.Repeat("b", 64)},
		Template:   10362946,
		Count:      1,
		Title:      "Max Level Reward",
		Text:       "You are now reached the Level 86. Take this box and I will let you ignite.",
	}
}

func TestApplyMaxLevelRewardMailGates(t *testing.T) {
	svc := &ProgressionService{MaxLevelReward: maxLevelRewardFixture(), Rules: GrowthRules{LevelCap: 115}}
	role := Character{ID: 7, AccountID: 3, ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"level":114}`)}
	if _, _, _, err := svc.ApplyMaxLevelRewardMail(role); err == nil {
		t.Fatal("below-cap character accepted")
	}
	role.State = json.RawMessage(`{"level":115,"inventory":{"sentinel":"preserve"}}`)
	state, proof, item, err := svc.ApplyMaxLevelRewardMail(role)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(state, role.State) {
		t.Fatalf("reward must not rewrite the save: %s", state)
	}
	var attachment inventory.MailItem
	if err = json.Unmarshal(item, &attachment); err != nil {
		t.Fatal(err)
	}
	if attachment.Stack == nil || attachment.Stack.Template != 10362946 || attachment.Stack.Amount != 1 || attachment.Stack.ExpireTime != inventory.GrantExpireTime {
		t.Fatalf("attachment differs from the source block: %+v", attachment.Stack)
	}
	if _, err = attachment.Row(); err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err = json.Unmarshal(proof, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt["template"] != float64(10362946) || receipt["count"] != float64(1) || receipt["level"] != float64(115) || receipt["level_cap"] != float64(115) || receipt["definition"] != strings.Repeat("b", 64) {
		t.Fatalf("receipt lost its source binding: %+v", receipt)
	}
	// 没有源规则、存档契约不符、等级上限未配置都不能发信。
	role.ConfigVersion = "another-generation"
	if _, _, _, err = svc.ApplyMaxLevelRewardMail(role); err == nil {
		t.Fatal("foreign save contract accepted")
	}
	role.ConfigVersion = savecontract.Identity()
	svc.MaxLevelReward = nil
	if _, _, _, err = svc.ApplyMaxLevelRewardMail(role); err == nil {
		t.Fatal("reward without a source rule accepted")
	}
	svc.MaxLevelReward = maxLevelRewardFixture()
	svc.Rules.LevelCap = 0
	if _, _, _, err = svc.ApplyMaxLevelRewardMail(role); err == nil {
		t.Fatal("reward without a configured cap accepted")
	}
}
