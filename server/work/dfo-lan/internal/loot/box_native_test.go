package loot

import (
	"strings"
	"testing"
)

const nativeBoxFixture = "[rate] 100 [main lot group id] 1 [special lot group id] 2 [material] 800 1 [change box stack] 75\n" +
	"[point stack] [type] `bonus` [gain] 1 [max] 10 [reward] [type] `draw` [param] 1 [/reward] [/point stack]\n" +
	"[point stack] [type] `section` [gain] 1 [max] 100 [section reward] 3 25 901 2 [/section reward] [/point stack]\n" +
	"[lot group] [id] 1 [item] 1 900 1 20 2 900 3 80 [/item] [/lot group]\n" +
	"[lot group] [id] 2 [item] 1 902 1 100 [/item] [/lot group]\n" +
	"[postal tag] `mail_title` `mail_message` [/postal tag]"

func TestNativeBoxMaterialAndDuplicateRewardOrder(t *testing.T) {
	id, table, err := parseNativeBoxCOS(nativeBoxFixture, "source.cos.txt")
	if err != nil || id != 800 || table.MaterialCount != 1 || len(table.Groups["1"]) != 2 || table.Groups["1"][0].Count != 1 || table.Groups["1"][1].Count != 3 {
		t.Fatal("source lot rows changed", id, table, err)
	}
	if len(table.PointStacks) != 2 || table.PointStacks[0].RewardParam != 1 || table.PointStacks[1].SectionReward[0].Template != 901 || table.PointStacks[1].SectionReward[0].Count != 2 || len(table.PostalTag) != 2 {
		t.Fatal("counter/postal source changed")
	}
	c, err := NewBoxCatalog(BoxCatalog{Tables: map[string]BoxTable{"800": table}})
	if err != nil || c.TableCount() != 1 {
		t.Fatal(err)
	}
}
func TestNativeBoxRefusesAmbiguousOrMalformedSource(t *testing.T) {
	for name, input := range map[string]string{
		"duplicate material": nativeBoxFixture + " [material] 800 1",
		"duplicate rate":     nativeBoxFixture + " [rate] 100",
		"incomplete lot":     strings.Replace(nativeBoxFixture, "1 900 1 20", "1 900 20", 1),
		"negative amount":    strings.Replace(nativeBoxFixture, "1 900 1 20", "1 900 -1 20", 1),
		"unclosed group":     strings.Replace(nativeBoxFixture, "[/lot group]", "", 1),
		"unknown counter":    strings.Replace(nativeBoxFixture, "`bonus`", "`unknown`", 1),
		"counter overflow":   strings.Replace(nativeBoxFixture, "3 25 901 2", "3 101 901 2", 1),
		"weight overflow":    strings.Replace(nativeBoxFixture, "1 900 1 20", "1 900 1 4294967295", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := parseNativeBoxCOS(input, "source.cos.txt"); err == nil {
				t.Fatal("invalid source accepted")
			}
		})
	}
}
