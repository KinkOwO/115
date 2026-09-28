package catalog

import (
	"testing"
)

// 钉死已导出的军团内容目录：末世录那一块的每个值都是 legionsystem.cos 的读字段，
// 是入口/计数/X10（阶段→副本）的唯一真源。数值来自运行时源 PVF。
func TestLegionContentsGeneratedCatalog(t *testing.T) {
	const source = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	c, e := LoadLegionContents("../../configs/legion-contents.generated.json", source)
	if e != nil {
		t.Fatal(e)
	}
	if c.Path != LegionContentsPath || len(c.SHA256) != 64 {
		t.Fatalf("provenance %q %q", c.Path, c.SHA256)
	}
	a, ok := c.Contents["Apocalypse"]
	if !ok {
		t.Fatalf("Apocalypse block missing; contents=%d", len(c.Contents))
	}
	if !a.Complete {
		t.Fatalf("Apocalypse block is incomplete: %v", a.Missing)
	}
	if a.LimitLevel != 115 || a.LastPhase != 5 {
		t.Fatalf("level/last phase = %d/%d, want 115/5", a.LimitLevel, a.LastPhase)
	}
	wantDungeons := []uint32{100005112, 100004995, 100005057, 100004918, 100005111, 100004994}
	wantBosses := []uint32{0, 109019062, 109019064, 109019065, 109019066, 109019061}
	if len(a.Dungeons) != len(wantDungeons) {
		t.Fatalf("dungeon rows = %d, want %d", len(a.Dungeons), len(wantDungeons))
	}
	for i, w := range wantDungeons {
		if a.Dungeons[i].Dungeon != w || a.Dungeons[i].Boss != wantBosses[i] {
			t.Fatalf("phase %d = %+v, want dungeon %d boss %d", i, a.Dungeons[i], w, wantBosses[i])
		}
	}
	if a.WeeklyEnter != 3 || a.PhaseReward != 1 {
		t.Fatalf("counts = %d/%d, want 3/1", a.WeeklyEnter, a.PhaseReward)
	}
	// 计数键：与 [ui data] 的 REWARD_ITEM_DUNGEON_INDEX 同值（同一格的两种用途）。
	if a.InCountIndex != 100004994 || a.UIInts["REWARD_ITEM_DUNGEON_INDEX"] != 100004994 {
		t.Fatalf("incount=%d rewardItemDungeon=%d", a.InCountIndex, a.UIInts["REWARD_ITEM_DUNGEON_INDEX"])
	}
	if a.Recruiting != (LegionAreaPoint{Town: 239, Area: 1, X: 1110, Y: 317}) {
		t.Fatalf("recruiting %+v", a.Recruiting)
	}
	if a.Waiting != (LegionAreaPoint{Town: 239, Area: 2, X: 727, Y: 310}) {
		t.Fatalf("waiting %+v", a.Waiting)
	}
	// 1007 = ENABLE_CHANNEL_TAB_EVENT_ID，就是分享版频道配置里的 OpenEvent 来源。
	if a.UIInts["ENABLE_CHANNEL_TAB_EVENT_ID"] != 1007 {
		t.Fatalf("channel tab event = %d, want 1007", a.UIInts["ENABLE_CHANNEL_TAB_EVENT_ID"])
	}
	if len(a.EnterData) != 6 {
		t.Fatalf("enter data rows = %d, want 6", len(a.EnterData))
	}
	for _, row := range a.EnterData {
		if row.FameLow != 73993 || row.FameHigh != 105881 {
			t.Fatalf("fame gate %+v", row)
		}
	}
	v, ok := c.Contents["Venus Legion"]
	if !ok || v.InCountIndex != 100003921 || v.Waiting.Town != 204 || !v.Complete {
		t.Fatalf("Venus block %+v", v)
	}
	// 不是每块都齐全：源里 ForestOfAwakeningNormal 没有 [dungeon info data]，
	// 目录必须如实报告而不是补零冒充。
	if foa, ok := c.Contents["ForestOfAwakeningNormal"]; !ok || foa.Complete || len(foa.Missing) == 0 {
		t.Fatalf("ForestOfAwakeningNormal completeness %+v", foa)
	}
}
