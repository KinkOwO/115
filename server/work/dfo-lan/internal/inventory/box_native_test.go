package inventory

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
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

func TestBoxDiscoveryFollowsNativeGrammarWithoutPathOrTemplateLists(t *testing.T) {
	files := []pvf.File{{ArchivePath: "new/location/a.cos"}, {ArchivePath: "else/other.cos"}, {ArchivePath: "comment.cos"}, {ArchivePath: "ignored.txt"}}
	texts := map[string]string{"new/location/a.cos": nativeBoxFixture, "else/other.cos": "[material] 44 1 [upgrade material]", "comment.cos": "// [main lot group id] 1"}
	iterate := func(visit func(pvf.File) error) error {
		for _, f := range files {
			if err := visit(f); err != nil {
				return err
			}
		}
		return nil
	}
	read := func(p string) (string, error) {
		text, ok := texts[p]
		if !ok {
			return "", fmt.Errorf("unexpected read %s", p)
		}
		return text, nil
	}
	got, err := discoverBoxCOSFiles(iterate, read)
	if err != nil || len(got) != 1 || got[0].id != 800 || got[0].path != "new/location/a.cos" || got[0].table.Groups["1"][1].Count != 3 {
		t.Fatalf("native discovery: %+v %v", got, err)
	}
	texts["new/location/a.cos"] = strings.Replace(nativeBoxFixture, "[material] 800 1", "[material] 800 -1", 1)
	if _, err = discoverBoxCOSFiles(iterate, read); err == nil {
		t.Fatal("malformed candidate was silently ignored")
	}
	delete(texts, "new/location/a.cos")
	if _, err = discoverBoxCOSFiles(iterate, read); err == nil {
		t.Fatal("unreadable source was silently ignored")
	}
	files = []pvf.File{{ArchivePath: "comment.cos"}}
	if _, err = discoverBoxCOSFiles(iterate, read); err == nil {
		t.Fatal("comment enabled a box")
	}
}
