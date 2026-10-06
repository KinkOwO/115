package main

import (
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"encoding/json"
	"strconv"
	"testing"
)

// boostSkipCreateRequest 造一份普通模式（options[10]=0）或奥德赛（=2）的创建请求，
// 让门禁按角色自身的创建标记判定，不吃 DFO_ODYSSEY_MODE。
func boostSkipCreateRequest(odyssey bool) []byte {
	req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
	last := byte(0)
	if odyssey {
		last = 2
	}
	return append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, last, 0)
}

func boostSkipRole(t *testing.T, level int, activated, odyssey bool) database.Character {
	t.Helper()
	role := database.Character{Profession: 0, Name: "BoostSkip", Request: boostSkipCreateRequest(odyssey),
		State: json.RawMessage(`{"level":` + strconv.Itoa(level) + `,"advancement":1}`)}
	if !activated {
		return role
	}
	raw, e := boostup.WriteState(role.State, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 1, Phase: 1}})
	must115(t, e)
	role.State = raw
	return role
}

func TestBoostStorySkipEligibility(t *testing.T) {
	c := &boostup.Catalog{GoalLevel: 115}
	if !boostStorySkipEligible(c, boostSkipRole(t, 115, true, false)) {
		t.Fatal("胶囊直升角色被排除")
	}
	for _, tc := range []struct {
		name string
		role database.Character
	}{
		{"没用过胶囊", boostSkipRole(t, 115, false, false)},
		{"未到 goal level", boostSkipRole(t, 114, true, false)},
		{"奥德赛角色", boostSkipRole(t, 115, true, true)},
	} {
		if boostStorySkipEligible(c, tc.role) {
			t.Fatal(tc.name + " 走了主线清除")
		}
	}
	if boostStorySkipEligible(nil, boostSkipRole(t, 115, true, false)) {
		t.Fatal("活动目录缺失时照做")
	}
	if boostStorySkipEligible(&boostup.Catalog{}, boostSkipRole(t, 115, true, false)) {
		t.Fatal("goal 0 时照做")
	}
}
