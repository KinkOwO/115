package storage

import (
	"testing"
	"time"
)

// PeriodStart 的周期切分。用固定时刻避免依赖真实的「今天」。
func TestPeriodStart(t *testing.T) {
	loc := time.UTC
	// 2026-09-29 是周二。
	now := time.Date(2026, 9, 29, 15, 4, 5, 0, loc)
	cases := []struct {
		period string
		want   time.Time
		note   string
	}{
		{"daily", time.Date(2026, 9, 29, 0, 0, 0, 0, loc), "当天 00:00"},
		{"weekly", time.Date(2026, 9, 28, 0, 0, 0, 0, loc), "本周一（9-29 是周二，故回退 1 天）"},
		{"monthly", time.Date(2026, 9, 1, 0, 0, 0, 0, loc), "本月 1 号"},
		{"accumulate", time.Time{}, "累计：零时间表示不限窗口"},
		{"version", time.Time{}, "version 语义未查证，按累计处理"},
		{"", time.Time{}, "空 = 一次性"},
	}
	for _, c := range cases {
		got := PeriodStart(c.period, now)
		if !got.Equal(c.want) {
			t.Errorf("%s：%v，期望 %v（%s）", c.period, got, c.want, c.note)
		}
	}
}

// 周日要归到「本周一」，不能归到下周一。
func TestPeriodStartSunday(t *testing.T) {
	loc := time.UTC
	sun := time.Date(2026, 9, 27, 12, 0, 0, 0, loc) // 周日
	got := PeriodStart("weekly", sun)
	want := time.Date(2026, 9, 21, 0, 0, 0, 0, loc) // 上周一
	if !got.Equal(want) {
		t.Fatalf("周日：%v，期望 %v", got, want)
	}
}

// 月末跨月：monthly 应回到本月 1 号，不是上月末。
func TestPeriodStartMonthEnd(t *testing.T) {
	loc := time.UTC
	end := time.Date(2026, 9, 30, 23, 59, 59, 0, loc)
	got := PeriodStart("monthly", end)
	want := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("月末：%v，期望 %v", got, want)
	}
}

// scope 常量与导出 JSON 里的字符串必须一致（catalog 侧写的是 "account"/"character"）。
func TestShopScopeStrings(t *testing.T) {
	if string(ShopScopeAccount) != "account" {
		t.Errorf("ShopScopeAccount = %q，期望 \"account\"", ShopScopeAccount)
	}
	if string(ShopScopeCharacter) != "character" {
		t.Errorf("ShopScopeCharacter = %q，期望 \"character\"", ShopScopeCharacter)
	}
}
