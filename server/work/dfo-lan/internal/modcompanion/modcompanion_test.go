package modcompanion

import "testing"

func TestConfigureRejectsAuthorMistakes(t *testing.T) {
	Reset()
	defer Reset()
	cases := []struct {
		name string
		in   Rules
	}{
		{"no source", Rules{Enabled: true, Companions: []Companion{{Template: 201}}}},
		{"no companions", Rules{Enabled: true, Source: "m"}},
		{"zero template", Rules{Enabled: true, Source: "m", Companions: []Companion{{Template: 0}}}},
		{"duplicate template", Rules{Enabled: true, Source: "m", Companions: []Companion{{Template: 201}, {Template: 201}}}},
		{"rank too low", Rules{Enabled: true, Source: "m", Companions: []Companion{{Template: 201, Rank: 4}}}},
		{"rank too high", Rules{Enabled: true, Source: "m", Companions: []Companion{{Template: 201, Rank: 9}}}},
	}
	for _, tc := range cases {
		if _, err := Configure(tc.in); err == nil {
			t.Fatalf("%s: 非法配置被接受了", tc.name)
		}
		if Enabled() {
			t.Fatalf("%s: 非法配置留下了生效状态", tc.name)
		}
	}
	tooMany := Rules{Enabled: true, Source: "m"}
	for i := 0; i <= maxCompanions; i++ {
		tooMany.Companions = append(tooMany.Companions, Companion{Template: uint32(200 + i)})
	}
	if _, err := Configure(tooMany); err == nil {
		t.Fatal("超出同伴上限被接受了")
	}
}

func TestConfigureDefaultsRankAndNormalizesIDs(t *testing.T) {
	Reset()
	defer Reset()
	got, err := Configure(Rules{
		Enabled:         true,
		Source:          "  town.companion  ",
		Companions:      []Companion{{Template: 201}, {Template: 202, Rank: 7}},
		IncludeDungeons: []uint32{30, 10, 30},
		ExcludeDungeons: []uint32{99, 99},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "town.companion" {
		t.Fatalf("来源没有去掉空白：%q", got.Source)
	}
	if got.Companions[0].Rank != minRank {
		t.Fatalf("档位 0 没有兜底成 %d：%d", minRank, got.Companions[0].Rank)
	}
	if len(got.IncludeDungeons) != 2 || got.IncludeDungeons[0] != 10 || got.IncludeDungeons[1] != 30 {
		t.Fatalf("副本号没有排序去重：%v", got.IncludeDungeons)
	}
	if len(got.ExcludeDungeons) != 1 || got.ExcludeDungeons[0] != 99 {
		t.Fatalf("排除号没有去重：%v", got.ExcludeDungeons)
	}
	if !Enabled() {
		t.Fatal("注册后 Enabled 仍为假")
	}
}

func TestAppliesTo(t *testing.T) {
	Reset()
	defer Reset()
	if (Rules{}).AppliesTo(1) {
		t.Fatal("零值规则不该对任何副本生效")
	}
	all, err := Configure(Rules{Enabled: true, Source: "m", Companions: []Companion{{Template: 201}}})
	if err != nil {
		t.Fatal(err)
	}
	if !all.AppliesTo(12345) {
		t.Fatal("没写 include 时应当对所有副本生效")
	}
	only, err := Configure(Rules{
		Enabled:         true,
		Source:          "m",
		Companions:      []Companion{{Template: 201}},
		IncludeDungeons: []uint32{7},
		ExcludeDungeons: []uint32{8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !only.AppliesTo(7) || only.AppliesTo(8) || only.AppliesTo(9) {
		t.Fatalf("include/exclude 判定不对：%v", only)
	}
	// 同一张副本既在 include 又在 exclude：排除优先（作者写重了应当是"别进"）。
	both, err := Configure(Rules{
		Enabled:         true,
		Source:          "m",
		Companions:      []Companion{{Template: 201}},
		IncludeDungeons: []uint32{7},
		ExcludeDungeons: []uint32{7},
	})
	if err != nil {
		t.Fatal(err)
	}
	if both.AppliesTo(7) {
		t.Fatal("排除应当优先于包含")
	}
}

func TestConfigureZeroValueDisables(t *testing.T) {
	Reset()
	defer Reset()
	if _, err := Configure(Rules{Enabled: true, Source: "m", Companions: []Companion{{Template: 201}}}); err != nil {
		t.Fatal(err)
	}
	got, err := Configure(Rules{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled || len(got.Companions) != 0 || Enabled() {
		t.Fatalf("零值没有完全撤销：%+v", got)
	}
	if (Current()).AppliesTo(1) {
		t.Fatal("撤销后仍对副本生效")
	}
}
