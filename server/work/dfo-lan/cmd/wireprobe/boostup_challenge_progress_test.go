package main

import (
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func boostChallengeProgressCatalog() *boostup.Catalog {
	return &boostup.Catalog{GoalLevel: 115, Challenges: []boostup.ChallengeDefinition{{Index: 0, UnlockKind: "level", UnlockValue: 115, Kind: "clear endkeeper of order", GuideDungeon: 100005014, Goal: 10, Repeat: 1}}}
}

func boostChallengeProgressRole(t *testing.T, st boostup.State) database.Character {
	t.Helper()
	raw, e := boostup.WriteState(json.RawMessage(`{"level":115}`), st)
	if e != nil {
		t.Fatal(e)
	}
	return database.Character{ID: 9, State: raw}
}

func boostChallengeEnrolled(progress uint32) boostup.State {
	return boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 12, Phase: 3, Finished: true}, LevelBonusSent: true,
		Challenge: &boostup.ChallengeState{Version: 1, Enrolled: true, Rows: map[byte]boostup.ChallengeProgress{0: {Unlocked: true, UnlockClaimed: true, Progress: progress}}}}
}

// 通关计数与结算在同一笔事务里提交，面板只认 2722：这一帧缺失就是实机
// 2026-10-06 16:43 的「库里 progress=1、面板仍 0/10」。
func TestBoostChallengeClearProgressPush(t *testing.T) {
	c := boostChallengeProgressCatalog()
	before := boostChallengeProgressRole(t, boostChallengeEnrolled(0))
	after := boostChallengeProgressRole(t, boostChallengeEnrolled(1))

	plan := boostChallengeClearProgress(c, before, after)
	if len(plan) != 1 || plan[0].ID != 2722 {
		t.Fatalf("clear must push exactly one 2722: %+v", plan)
	}
	body := plan[0].Payload
	if len(body) != protocol.BoostChallengeBodyLen {
		t.Fatalf("body %d B, client reads %d B", len(body), protocol.BoostChallengeBodyLen)
	}
	if body[2] != 1 {
		t.Fatal("enrolled byte lost")
	}
	if got := binary.LittleEndian.Uint32(body[3+2:]); got != 1 {
		t.Fatalf("row0 progress=%d, want 1", got)
	}
	if binary.LittleEndian.Uint16(body) != uint16(c.GoalLevel) {
		t.Fatal("goal level marker lost")
	}

	if p := boostChallengeClearProgress(c, before, before); p != nil {
		t.Fatal("unchanged facts must not push (panel replays)")
	}
	other := after
	other.ID = 10
	if p := boostChallengeClearProgress(c, before, other); p != nil {
		t.Fatal("different character must not push")
	}
	if p := boostChallengeClearProgress(nil, before, after); p != nil {
		t.Fatal("activity off must not push")
	}
	if p := boostChallengeClearProgress(&boostup.Catalog{GoalLevel: 115}, before, after); p != nil {
		t.Fatal("catalog without challenges must not push")
	}
}

// 进城/重登录恢复：只有真登记了挑战的角色出帧，普通角色的进城序列逐字节不变。
func TestBoostChallengeEntryRestore(t *testing.T) {
	c := boostChallengeProgressCatalog()

	body, e := boostChallengeEntryRestore(c, boostChallengeProgressRole(t, boostChallengeEnrolled(3)))
	if e != nil {
		t.Fatal(e)
	}
	if len(body) != protocol.BoostChallengeBodyLen || binary.LittleEndian.Uint32(body[3+2:]) != 3 {
		t.Fatalf("enrolled restore wrong: len=%d", len(body))
	}

	plain := boostChallengeProgressRole(t, boostup.State{Version: 1})
	if body, e = boostChallengeEntryRestore(c, plain); e != nil || body != nil {
		t.Fatalf("unactivated role pushed: %v %v", body, e)
	}
	if body, e = boostChallengeEntryRestore(c, database.Character{ID: 9, State: json.RawMessage(`{"level":115}`)}); e != nil || body != nil {
		t.Fatalf("non-boost role pushed: %v %v", body, e)
	}
	if body, e = boostChallengeEntryRestore(nil, boostChallengeProgressRole(t, boostChallengeEnrolled(1))); e != nil || body != nil {
		t.Fatalf("activity off pushed: %v %v", body, e)
	}
}
