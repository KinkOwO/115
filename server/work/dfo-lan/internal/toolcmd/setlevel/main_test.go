package setlevel

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/savecontract"
	"encoding/json"
	"path/filepath"
	"testing"
)

// levelTableFixture 是等级表的形状：Thresholds[i] 是「i+2 级的起点经验」。
func levelTableFixture() catalog.Progression {
	return catalog.Progression{Thresholds: []uint64{0, 200, 500, 900, 1400, 2000, 3000}}
}

// 只动 level 与 experience：其余字段（属性、技能点等）必须原样保留，
// 否则一个 GM 改等级会顺手把别的存档内容也改坏。
func TestApplyLevelWritesLevelAndExperienceOnly(t *testing.T) {
	// inventory 不在 character.State 里，整份结构体往返会把它清掉（副本实测出现过
	// 「改等级后金币 1000 → 0」）。所以夹具里必须带 inventory 并断言它存活。
	state := json.RawMessage(`{"level":3,"experience":77,"attributes":{"str":1.5},"skill_points":[9,9],` +
		`"inventory":{"gold":123,"items":[{"slot":65}]}}`)
	raw, receipt, err := applyLevel(database.Character{State: state}, 50, 123456)
	if err != nil {
		t.Fatalf("applyLevel: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if out["level"].(float64) != 50 || out["experience"].(float64) != 123456 {
		t.Fatalf("state = %v, want level 50 experience 123456", out)
	}
	if _, ok := out["attributes"]; !ok {
		t.Fatal("attributes were dropped by the level change")
	}
	points, ok := out["skill_points"].([]any)
	if !ok || len(points) != 2 || points[0].(float64) != 9 {
		t.Fatalf("skill_points changed: %v", out["skill_points"])
	}
	inventory, ok := out["inventory"].(map[string]any)
	if !ok {
		t.Fatalf("inventory was dropped by the level change: %v", out)
	}
	if inventory["gold"].(float64) != 123 {
		t.Fatalf("inventory.gold changed: %v", inventory["gold"])
	}
	if items, ok := inventory["items"].([]any); !ok || len(items) != 1 {
		t.Fatalf("inventory.items changed: %v", inventory["items"])
	}
	var decoded map[string]any
	if err := json.Unmarshal(receipt, &decoded); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if decoded["level_before"].(float64) != 3 || decoded["experience_before"].(float64) != 77 {
		t.Fatalf("receipt = %v, want the before values recorded", decoded)
	}
}

// 阈值越界必须拒绝（等级表是 PVF 定的，工具不能自己发明等级）。
func TestLevelThresholdRejectsOutOfRange(t *testing.T) {
	progression := levelTableFixture()
	if _, err := levelThreshold(progression, 1); err == nil {
		t.Fatal("level 1 must be rejected (no threshold below level 2)")
	}
	if _, err := levelThreshold(progression, 9); err == nil {
		t.Fatal("level beyond the table must be rejected")
	}
	got, err := levelThreshold(progression, 3)
	if err != nil {
		t.Fatalf("level 3: %v", err)
	}
	if got != 200 {
		t.Fatalf("threshold for level 3 = %d, want Thresholds[1] = 200", got)
	}
}

// 走真实的审计/幂等路径在 SQLite 上写等级，并证明重复同一个 grant-id 不重放。
func TestApplyGrantSetsLevelAndStaysIdempotentOnSQLite(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "setlevel.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(store.Close)
	account, err := store.DevelopmentAccount(ctx, "setlevel")
	if err != nil {
		t.Fatalf("DevelopmentAccount: %v", err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{
		AccountID:     account,
		WireID:        1,
		Name:          "Leveler",
		ConfigVersion: savecontract.Identity(),
		Request:       []byte{0},
		State:         json.RawMessage(`{"level":1}`),
	}, 8)
	if err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}

	grant := database.Grant{ID: "gm-level-1", AccountID: account, Character: role.ID, Reason: "test", Operator: "tester"}
	first, err := store.ApplyGrant(ctx, grant, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return applyLevel(current, 60, 999)
	})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if !first.Applied {
		t.Fatal("first grant reported applied=false")
	}
	view, err := readLevel(first.Character.State)
	if err != nil {
		t.Fatalf("readLevel: %v", err)
	}
	if view.Level != 60 || view.Experience != 999 {
		t.Fatalf("level/experience = %+v, want 60/999", view)
	}

	second, err := store.ApplyGrant(ctx, grant, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return applyLevel(current, 10, 5)
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if second.Applied {
		t.Fatal("replaying the same grant id re-applied the change")
	}
	// 幂等重放不会返回重读后的角色，所以直接回库里核对状态没被改动。
	roles, err := store.Characters(ctx, account)
	if err != nil {
		t.Fatalf("Characters: %v", err)
	}
	var stored database.Character
	for _, r := range roles {
		if r.ID == role.ID {
			stored = r
		}
	}
	if view, err = readLevel(stored.State); err != nil {
		t.Fatalf("readLevel after replay: %v", err)
	}
	if view.Level != 60 || view.Experience != 999 {
		t.Fatalf("replay changed the stored state: %+v", view)
	}
}
