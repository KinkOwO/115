package character

import (
	"dfolan/internal/boostup"
	"encoding/json"
	"testing"
)

func TestBoostChallengeClearSourceFinalsOnly(t *testing.T) {
	c := &boostup.Catalog{Challenges: []boostup.ChallengeDefinition{
		{Index: 0, Kind: "clear endkeeper of order", GuideDungeon: 100005014, Goal: 10, Repeat: 1},
		{Index: 1, Kind: "clear higher or legion", Conditions: []uint32{26, 31, 35, 36}, Goal: 1, Repeat: 1},
		{Index: 2, Kind: "clear raid", Conditions: []uint32{14}, GuideDungeon: 100004604, Goal: 1, Repeat: 1},
	}}
	s := &ProgressionService{Boost: c}
	st := boostup.State{Version: 1, Activated: true, Training: boostup.Training{Finished: true}, Challenge: &boostup.ChallengeState{Version: 1, Enrolled: true, Rows: map[byte]boostup.ChallengeProgress{0: {Unlocked: true}, 1: {Unlocked: true}, 2: {Unlocked: true}}}}
	raw, e := boostup.WriteState(json.RawMessage(`{"untouched":42}`), st)
	if e != nil {
		t.Fatal(e)
	}
	// 本树契约（applyBoostChallengeClear 的注释）：只有 `[go contents dungeon index]`
	// 直读的「清理秩序终结者」由副本号判定；「高阶/军团」与「团队」的内容号
	// (26/31/35/36、14) 缺 internal/conquest 真源，通关事件不计数。
	// donor 基线在这里期望 100004134/100004520/… 落 row=1、100004604 落 row=2，
	// 那三行仍是 [GAP]，真源到位后把这些用例补回。
	for _, tc := range []struct {
		id  uint32
		row int
	}{
		{100004132, -1}, {100004133, -1}, {100004134, -1},
		{100004520, -1}, {100004521, -1}, {100005013, -1}, {100003630, -1},
		{100005014, 0}, {100004604, -1}, {100004918, -1}, {1, -1},
	} {
		next, e := s.applyBoostChallengeClear(raw, tc.id)
		if e != nil {
			t.Fatal(e)
		}
		result, e := boostup.ReadState(next)
		if e != nil {
			t.Fatal(e)
		}
		for row, p := range result.Challenge.Rows {
			want := uint32(0)
			if int(row) == tc.row {
				want = 1
			}
			if p.Progress != want {
				t.Fatalf("dungeon=%d row=%d progress=%d", tc.id, row, p.Progress)
			}
		}
		var obj map[string]json.RawMessage
		_ = json.Unmarshal(next, &obj)
		if string(obj["untouched"]) != "42" {
			t.Fatal("lost unrelated state")
		}
	}
}
