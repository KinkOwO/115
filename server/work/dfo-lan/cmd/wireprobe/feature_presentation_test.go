package main

import (
	"dfolan/internal/activity"
	"testing"
)

func TestFeaturePresentationUsesBoundSourceNames(t *testing.T) {
	r := activity.RoutineRules{Kind: "online", Table: "source", Dates: activity.Dates{Start: "a", End: "b"}, ItemNames: map[uint32]string{12: "源中材料甲"}}
	key := r.CampaignKey()
	reward := activity.Reward{Item: 12, Count: 7}
	if got := routineRewardLabel(r, "每日", reward); got != "每日：源中材料甲×7" {
		t.Fatal(got)
	}
	r.ItemNames[12] = "源中材料乙"
	if got := routineRewardLabel(r, "每日", reward); got != "每日：源中材料乙×7" {
		t.Fatal(got)
	}
	if r.CampaignKey() != key {
		t.Fatal("presentation changed campaign/save identity")
	}
	if got := membershipGradeLabel("当前我的等级·稀有 1阶段"); got != "稀有 1阶段" {
		t.Fatal(got)
	}
	if got := membershipGradeLabel("史诗"); got != "史诗" {
		t.Fatal(got)
	}
}
