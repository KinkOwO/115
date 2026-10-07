package catalog

import (
	"os"
	"path/filepath"
	"testing"

)

// [maxlevel reward] 是「任何满级角色都发这一份」的源规则，不属于奥德赛分支。
// 测试钉住模板、数量与邮件文案，并证明缺件模板会被拒绝而不是发出悬空附件。
func TestMaxLevelRewardFollowsSourceBlock(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		for _, cand := range []string{
			filepath.Join("..", "..", "client-build", "Script.inner.pvf"),
			filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf"),
		} {
			if _, e := os.Stat(cand); e == nil {
				p = cand
				break
			}
		}
	}
	if p == "" {
		t.Skip("current PVF absent")
	}
	a, err := OpenTestArchiveCached(p, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Skip("cannot open PVF:", err)
	}
	index, err := ImportItemIndex(a)
	if err != nil {
		t.Fatal(err)
	}
	reward, err := ImportMaxLevelReward(a, index)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("maxlevel reward template=%d count=%d title=%q text=%q definition=%s@%s",
		reward.Template, reward.Count, reward.Title, reward.Text, reward.Definition.Path, reward.Definition.SHA256[:12])
	if reward.Definition.Path != MaxLevelRewardPath || reward.Source != a.Snapshot().Checksum {
		t.Fatalf("reward does not carry its own source identity: %+v", reward)
	}
	if reward.Count == 0 || reward.Title == "" || reward.Text == "" {
		t.Fatalf("source block or wording lost: %+v", reward)
	}
	// 收件人拿到的必须是源索引里真实存在的可叠加模板。
	if entry, ok := index.Items[reward.Template]; !ok || entry.Kind != "stackable" {
		t.Fatalf("reward template %d is not a source stackable: %+v", reward.Template, entry)
	}
	sparse := ItemIndex{Items: map[uint32]ItemIndexEntry{reward.Template - 1: {ID: reward.Template - 1, Kind: "stackable"}}}
	if _, err = ImportMaxLevelReward(a, sparse); err == nil {
		t.Fatal("absent reward template accepted")
	}
}
