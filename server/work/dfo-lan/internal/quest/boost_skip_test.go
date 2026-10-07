package quest_test

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/quest"
	"dfolan/internal/savecontract"
	"dfolan/internal/testfixture"
	"encoding/json"
	"slices"
	"testing"
	"time"
)

const boostSkipGoal = byte(115)

// boostSkipFixture 与奥德赛毕业用同一份任务/职业源，但**不**挂 Odyssey：
// 普通模式直升不读 aradodyssey.etc 的 [quest clear] 表，挂上就会把两条链的判据混在一起。
// boostSkipCreateRequest 是普通模式（非奥德赛）的创建请求：12 个选项、
// options[3:8] 固定后缀、options[10] != 2。CreateCharacter 对 create_request 有
// NOT NULL 约束，而这条链的角色本来就来自胶囊直升的普通角色。
func boostSkipCreateRequest() []byte {
	req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
	return append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 0, 0)
}

func boostSkipFixture(t *testing.T) (*quest.Service, database.Character) {
	t.Helper()
	q, err := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := catalog.LoadCharacters("../../configs/characters.alljobs-pilot.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &quest.Service{Catalog: q, Professions: p}
	r := database.Character{Profession: 0, Request: boostSkipCreateRequest(), Name: "BoostSkipFixture",
		ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"level":115,"advancement":1}`)}
	return s, r
}

func TestBoostStorySkipPlanSortedAndValid(t *testing.T) {
	s, role := boostSkipFixture(t)
	ids, err := s.BoostStorySkipPlan(role, boostSkipGoal)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) < 500 || !slices.Contains(ids, 22790) {
		t.Fatalf("source epics missing: %d", len(ids))
	}
	// 业主 2026-10-06 实机反馈「还剩 115 级的任务」＝角色 1 库里剩下的这两条
	// `[epic]` min=115（[[project-boost662-story-skip]]）；含等于目标等级这条界必须钉住。
	if !slices.Contains(ids, 22841) || !slices.Contains(ids, 23028) {
		t.Fatal("goal-level epics are not cleared")
	}
	goalLevel := 0
	for i, id := range ids {
		if i > 0 && ids[i-1] >= id {
			t.Fatal("plan not sorted and unique")
		}
		q, ok := s.Catalog.Quests[uint32(id)]
		if !ok || q.ID != uint32(id) {
			t.Fatalf("plan quest %d absent from source catalog", id)
		}
		if q.MinimumLevel == 0 || q.MinimumLevel > uint32(boostSkipGoal) {
			t.Fatalf("plan quest %d outside (0, goal]: %d", id, q.MinimumLevel)
		}
		if q.MinimumLevel == uint32(boostSkipGoal) {
			goalLevel++
		}
	}
	if goalLevel == 0 {
		t.Fatal("no goal-level epic in the plan")
	}
	// 提交层的校验就是这三条（0/65535/有序），这里先把计划侧对齐。
	if _, err = s.BoostStorySkipPlan(role, 0); err == nil {
		t.Fatal("goal 0 accepted")
	}
	role.ConfigVersion = "not-the-source-identity"
	if _, err = s.BoostStorySkipPlan(role, boostSkipGoal); err == nil {
		t.Fatal("mismatched source identity accepted")
	}
}

// TestBoostStorySkipPlanChangesWithSource 是 §0.2.5 要求的「改源字段证明结果随源变」：
// 最低等级、职业限制、目标等级三条各证一次，不复述常量。
func TestBoostStorySkipPlanChangesWithSource(t *testing.T) {
	s, role := boostSkipFixture(t)
	const id = uint32(22790)
	base, err := s.BoostStorySkipPlan(role, boostSkipGoal)
	if err != nil || !slices.Contains(base, uint16(id)) {
		t.Fatalf("fixture epic missing: %v", err)
	}
	original := s.Catalog.Quests[id]

	q := original
	q.MinimumLevel = uint32(boostSkipGoal) + 1 // 推到目标等级之上 ⇒ 不再是「直升该清的主线」
	s.Catalog.Quests[id] = q
	if ids, err := s.BoostStorySkipPlan(role, boostSkipGoal); err != nil || slices.Contains(ids, uint16(id)) {
		t.Fatalf("quest above the goal still cleared: %v", err)
	}
	// 含等于：min 正好落在目标等级必须被清（业主 2026-10-06「还剩 115 级的任务」）。
	q.MinimumLevel = uint32(boostSkipGoal)
	s.Catalog.Quests[id] = q
	if ids, err := s.BoostStorySkipPlan(role, boostSkipGoal); err != nil || !slices.Contains(ids, uint16(id)) {
		t.Fatalf("goal-level quest omitted: %v", err)
	}
	q.MinimumLevel = 60
	q.Jobs = []string{"[unrelated job]"}
	s.Catalog.Quests[id] = q
	if ids, err := s.BoostStorySkipPlan(role, boostSkipGoal); err != nil || slices.Contains(ids, uint16(id)) {
		t.Fatalf("foreign profession auto-cleared: %v", err)
	}
	q.Jobs = nil // 源里没有 [job] 限制的任务对全部职业生效
	s.Catalog.Quests[id] = q
	if ids, err := s.BoostStorySkipPlan(role, boostSkipGoal); err != nil || !slices.Contains(ids, uint16(id)) {
		t.Fatalf("unrestricted epic omitted: %v", err)
	}
	s.Catalog.Quests[id] = original

	// goal 就是活动自己的 [goal level]：调低它，计划必须随之变小且不越界。
	low, err := s.BoostStorySkipPlan(role, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(low) >= len(base) {
		t.Fatalf("goal bound ignored: %d >= %d", len(low), len(base))
	}
	for _, got := range low {
		if s.Catalog.Quests[uint32(got)].MinimumLevel > 60 {
			t.Fatalf("quest %d above the lowered goal", got)
		}
	}
	// 没到目标等级的角色不做任何清除（胶囊路径本身也不会走到这里）。
	role.State = json.RawMessage(`{"level":114,"advancement":1}`)
	if _, err = s.BoostStorySkipPlan(role, boostSkipGoal); err == nil {
		t.Fatal("below-goal plan accepted")
	}
}

func TestBoostStorySkipCommitIsIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})
	db := fixture.Storage()
	for _, migrate := range []func(context.Context) error{db.Migrate, db.MigrateQuests, db.MigrateCharacterEvents} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s, role := boostSkipFixture(t)
	s.Store = db
	account, err := db.DevelopmentAccount(ctx, "boost-skip-test")
	if err != nil {
		t.Fatal(err)
	}
	role.AccountID = account
	role, err = db.CreateCharacter(ctx, role, 24)
	if err != nil {
		t.Fatal(err)
	}
	if character.CreatedAsOdyssey(role) {
		t.Fatal("boost skip fixture became an Odyssey character")
	}
	ids, err := s.BoostStorySkipPlan(role, boostSkipGoal)
	if err != nil {
		t.Fatal(err)
	}
	// 一条已接受（该被改写成完成）、一条已完成且带别的 progress_model（不该被动）。
	if err = fixture.SeedQuestRecord(ctx, role.ID, int32(ids[0]), "accepted", role.ConfigVersion, "prior-active"); err != nil {
		t.Fatal(err)
	}
	if err = fixture.SeedQuestRecord(ctx, role.ID, int32(ids[1]), "completed", role.ConfigVersion, "prior-completed"); err != nil {
		t.Fatal(err)
	}
	count, applied, err := s.BoostStorySkip(ctx, role, boostSkipGoal)
	if err != nil || !applied || count != len(ids) {
		t.Fatalf("first pass: count=%d applied=%v err=%v", count, applied, err)
	}
	for i := 0; i < 2; i++ {
		count, applied, err = s.BoostStorySkip(ctx, role, boostSkipGoal)
		if err != nil || applied || count != 0 {
			t.Fatalf("replay %d: count=%d applied=%v err=%v", i, count, applied, err)
		}
	}
	status, model, err := fixture.QuestRecord(ctx, role.ID, int32(ids[0]))
	if err != nil || status != "completed" || model != database.BoostStorySkipModel {
		t.Fatalf("accepted row not skipped: %s %s %v", status, model, err)
	}
	status, model, err = fixture.QuestRecord(ctx, role.ID, int32(ids[1]))
	if err != nil || status != "completed" || model != "prior-completed" {
		t.Fatalf("completed history replaced: %s %s %v", status, model, err)
	}
	rows, err := fixture.EventCountByKey(ctx, role.ID, database.BoostStorySkipModel)
	if err != nil || rows != 1 {
		t.Fatalf("duplicate receipts: %d %v", rows, err)
	}
	total, err := fixture.QuestCount(ctx, role.ID)
	if err != nil || total != int64(len(ids)) {
		t.Fatalf("quest rows written: %d of %d (%v)", total, len(ids), err)
	}
}

// TestBoostStorySkipCompensatesLegacyReceipt：v1（min < goal）已经跑过的角色，
// 登录/进城镇时必须按 v2（min <= goal）补一次，只补缺失行、不重写已有历史，
// 并且只留一把 v2 收据（存档兼容，§0 铁律 4；与 2026-10-05 奥德赛毕业 v2→v3 同法）。
func TestBoostStorySkipCompensatesLegacyReceipt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})
	db := fixture.Storage()
	for _, migrate := range []func(context.Context) error{db.Migrate, db.MigrateQuests, db.MigrateCharacterEvents} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s, role := boostSkipFixture(t)
	s.Store = db
	account, err := db.DevelopmentAccount(ctx, "boost-skip-compensate")
	if err != nil {
		t.Fatal(err)
	}
	role.AccountID = account
	role, err = db.CreateCharacter(ctx, role, 24)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := s.BoostStorySkipPlan(role, boostSkipGoal)
	if err != nil {
		t.Fatal(err)
	}
	goalID, belowID := uint16(0), uint16(0)
	for _, id := range ids {
		switch s.Catalog.Quests[uint32(id)].MinimumLevel {
		case uint32(boostSkipGoal):
			if goalID == 0 {
				goalID = id
			}
		default:
			if belowID == 0 {
				belowID = id
			}
		}
	}
	if goalID == 0 || belowID == 0 {
		t.Fatal("fixture needs both a goal-level and a below-goal epic")
	}
	// 上一版收据 + 它当时已经写过的一行（不该被改）；goalID 是 v1 漏掉、至今仍 accepted 的那条。
	if _, _, err = db.CommitCharacterEvent(ctx, account, role.ID, role.ConfigVersion,
		database.BoostStorySkipLegacyModel, database.BoostStorySkipLegacyModel,
		func(current character.Character) (json.RawMessage, json.RawMessage, error) {
			return current.State, json.RawMessage(`{"count":1}`), nil
		}); err != nil {
		t.Fatal(err)
	}
	if err = fixture.SeedQuestRecord(ctx, role.ID, int32(belowID), "completed", role.ConfigVersion, database.BoostStorySkipLegacyModel); err != nil {
		t.Fatal(err)
	}
	if err = fixture.SeedQuestRecord(ctx, role.ID, int32(goalID), "accepted", role.ConfigVersion, "prior-active"); err != nil {
		t.Fatal(err)
	}

	count, applied, err := s.BoostStorySkip(ctx, role, boostSkipGoal)
	if err != nil || !applied || count != len(ids) {
		t.Fatalf("compensation pass: count=%d applied=%v err=%v", count, applied, err)
	}
	status, model, err := fixture.QuestRecord(ctx, role.ID, int32(goalID))
	if err != nil || status != "completed" || model != database.BoostStorySkipModel {
		t.Fatalf("goal-level row not compensated: %s %s %v", status, model, err)
	}
	status, model, err = fixture.QuestRecord(ctx, role.ID, int32(belowID))
	if err != nil || status != "completed" || model != database.BoostStorySkipLegacyModel {
		t.Fatalf("v1 history rewritten: %s %s %v", status, model, err)
	}
	if rows, e := fixture.EventCountByKey(ctx, role.ID, database.BoostStorySkipModel); e != nil || rows != 1 {
		t.Fatalf("v2 receipts: %d %v", rows, e)
	}
	if rows, e := fixture.EventCountByKey(ctx, role.ID, database.BoostStorySkipLegacyModel); e != nil || rows != 1 {
		t.Fatalf("v1 receipts: %d %v", rows, e)
	}
	receipt, err := db.CharacterEventReceipt(ctx, account, role.ID, database.BoostStorySkipModel)
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		CompensatedFrom string `json:"compensated_from"`
	}
	if err = json.Unmarshal(receipt, &proof); err != nil || proof.CompensatedFrom != database.BoostStorySkipLegacyModel {
		t.Fatalf("receipt does not record the compensation: %s %v", receipt, err)
	}
	if count, applied, err = s.BoostStorySkip(ctx, role, boostSkipGoal); err != nil || applied || count != 0 {
		t.Fatalf("v2 replay: count=%d applied=%v err=%v", count, applied, err)
	}
}
