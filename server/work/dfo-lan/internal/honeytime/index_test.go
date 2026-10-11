package honeytime

import (
	"strings"
	"testing"
)

// fakeSource 是一份内存里的 PVF 视图：键是大写不敏感的路径，值是脚本文本。
// 只实现 TextSource 需要的那一件事，让索引解析能在没有 760 MB 归档的环境里被钉住。
type fakeSource map[string]string

func (f fakeSource) Text(path string) (string, error) {
	if v, ok := f[strings.ToLower(path)]; ok {
		return v, nil
	}
	return "", errTextNotFound{path}
}

type errTextNotFound struct{ path string }

func (e errTextNotFound) Error() string { return "pvf file not found: " + e.path }

// indexFixture 是 `list/event.lst` 的**形态复制**（一条长行、`id `path`` 两两成对）。
// 刻意包含：313 之前有别的条目、一条用反斜杠分隔的路径、以及 313 之后的条目。
const indexFixture = "89 `Live/Event/Kor/2022/0618_ThreeBuffEvent/ThreeBuffEvent.evt` " +
	"93 `Event/LevelUpRewardItem.evt` " +
	"331 `Live/Event/Kor/2026/0326_AttendanceDailyEvent/AttendanceDailyEvent.evt` " +
	"313 `Live/Event/Kor/2023/1026_HoneyTime/HoneyTime.evt` " +
	"4163 `Live\\Event\\Kor\\2024\\0801_DnfMichelin\\DnfMichelin.evt` " +
	"516 `Live/Else/Univ/2026/0804_2026_AradPass_season3/NewAradPassEvent.evt` "

// realPeriodFixture 是索引版（2023 期）`HoneyTime.evt` 的逐字导出（节选到 period 段为止）。
// 物品号与本仓 2026 那一期完全不同 —— 这正是"发错物品"那件事的形状。
const realPeriodFixture = `
[control panel]
` + "`event_1703_honeytime_info` `2026-07-09 09:00:00` `2026-08-04 09:00:00`" + `
[/control panel]

[reward postal type]
1 
[pc room event]
` + "`false`" + `
[max reward offset count]
` + "`daily` 4 `period` 4" + `
[/max reward offset count]

[reward info]

[reward type]
` + "`daily`" + `
[loop]
1 
[reward data]
0 590015969 1 0 600000 1 590015970 1 0 1200000 2 590015971 1 0 1800000 3 590015972 1 0 3600000 
[/reward data]

[/reward info]

[period reward weekly loop increase value]
0 1 
[reward info]

[reward type]
` + "`period`" + `
[loop]
1 
[reward data]
0 590015973 1 0 18000000 1 590015974 1 0 54000000 2 590015976 1 0 108000000 3 590015977 3 0 108000000 
[/reward data]

[/reward info]

[ui file]
` + "`UI/Event/PCRoomTimePoint/upgradeDailyEventWindow_20221229.xui`" + `
`

func TestResolveScriptPathFollowsEventIndex(t *testing.T) {
	src := fakeSource{EventListPath: indexFixture}
	got, err := ResolveScriptPath(src)
	if err != nil {
		t.Fatalf("ResolveScriptPath: %v", err)
	}
	want := "live/event/kor/2023/1026_honeytime/honeytime.evt"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestResolveScriptPathNormalizesBackslashesAndCase(t *testing.T) {
	src := fakeSource{EventListPath: "313 `Live\\Event\\Kor\\2023\\1026_HoneyTime\\HoneyTime.evt` "}
	got, err := ResolveScriptPath(src)
	if err != nil {
		t.Fatalf("ResolveScriptPath: %v", err)
	}
	if strings.Contains(got, `\`) || got != strings.ToLower(got) {
		t.Fatalf("path not normalized: %q", got)
	}
}

func TestResolveScriptPathRejectsBadIndex(t *testing.T) {
	cases := []struct {
		name  string
		index string
	}{
		{"缺少 313 绑定", "331 `Live/Event/Kor/x/a.evt` "},
		{"重复绑定", "313 `a/b.evt` 313 `c/d.evt` "},
		{"token 不成对", "313 `a/b.evt` 331 "},
		{"id 不是数字", "three `a/b.evt` "},
		{"绝对路径", "313 `/abs/b.evt` "},
		{"含 ..", "313 `../b.evt` "},
		{"后缀不是 .evt", "313 `a/b.cos` "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolveScriptPath(fakeSource{EventListPath: tc.index}); err == nil {
				t.Fatalf("expected error for %v", tc.name)
			}
		})
	}
}

func TestResolveScriptPathRequiresIndexFile(t *testing.T) {
	if _, err := ResolveScriptPath(fakeSource{}); err == nil {
		t.Fatal("expected error when " + EventListPath + " is missing")
	}
}

// TestLoadFollowsIndexAndParsesIndexedScript 是这次修复的回归钉：
// 目录必须**按索引**读脚本（而不是写死某一份），并且把实际路径记进 Catalog.ScriptPath。
func TestLoadFollowsIndexAndParsesIndexedScript(t *testing.T) {
	src := fakeSource{
		EventListPath:                            indexFixture,
		"live/event/kor/2023/1026_honeytime/honeytime.evt": realPeriodFixture,
	}
	cat, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cat.ScriptPath != "live/event/kor/2023/1026_honeytime/honeytime.evt" {
		t.Fatalf("ScriptPath = %q", cat.ScriptPath)
	}
	// 索引版期间轨是 4 档，物品 590015973/74/76/77。
	if len(cat.Period) != 4 {
		t.Fatalf("period tiers = %d, want 4", len(cat.Period))
	}
	got := []uint32{cat.Period[0].Reward.Template, cat.Period[1].Reward.Template,
		cat.Period[2].Reward.Template, cat.Period[3].Reward.Template}
	want := []uint32{590015973, 590015974, 590015976, 590015977}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("period[%d] template = %d, want %d", i, got[i], want[i])
		}
	}
	if cat.Period[3].Reward.Count != 3 {
		t.Fatalf("period[3] count = %d, want 3", cat.Period[3].Reward.Count)
	}
	// 累计门槛 5 / 20 / 50 / 80 小时。
	if got, want := cat.PeriodCumulativeSeconds(0), uint32(5*3600); got != want {
		t.Fatalf("cumulative[0] = %d, want %d", got, want)
	}
	if got, want := cat.PeriodCumulativeSeconds(3), uint32(80*3600); got != want {
		t.Fatalf("cumulative[3] = %d, want %d", got, want)
	}
	if cat.PeriodOffsetCount != 4 {
		t.Fatalf("period offset count = %d, want 4", cat.PeriodOffsetCount)
	}
	if cat.PCRoom {
		t.Fatal("[pc room event] false must not become true")
	}
	// 面板日档（索引版）也不是 10414136 一族。
	if cat.Daily[0].Reward.Template != 590015969 {
		t.Fatalf("daily[0] template = %d, want 590015969", cat.Daily[0].Reward.Template)
	}
	if cat.Loop != 1 {
		t.Fatalf("daily loop = %d, want 1 (indexed script)", cat.Loop)
	}
}

// TestLoadPathIsExplicit 直读路必须原样使用给定路径（不做任何猜测）。
func TestLoadPathIsExplicit(t *testing.T) {
	src := fakeSource{"live/else/x/y.evt": realPeriodFixture}
	cat, err := LoadPath(src, "live/else/x/y.evt")
	if err != nil {
		t.Fatalf("LoadPath: %v", err)
	}
	if cat.ScriptPath != "live/else/x/y.evt" {
		t.Fatalf("ScriptPath = %q", cat.ScriptPath)
	}
}
