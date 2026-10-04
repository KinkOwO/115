package database

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestSQLCQuestTransitionsAndAtomicReward(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func() error{func() error { return s.Migrate(ctx) }, func() error { return s.MigrateQuests(ctx) }, func() error { return s.MigrateQuestObjectives(ctx) }, func() error { return s.MigrateQuestRewards(ctx) }} {
		if err := migrate(); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "quests")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "Quest", Request: []byte{1}, ConfigVersion: version, State: json.RawMessage(`{"level":115,"unknown":{"retained":true},"gold":0}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := s.Quests(ctx, account, role.ID); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("empty quests: %+v %v", rows, err)
	}
	if _, err := s.Quests(ctx, account+100, role.ID); err == nil {
		t.Fatal("quest ownership bypass")
	}
	if n, err := s.ClearQuests(ctx, account, role.ID, version, []uint16{1, 2}); err != nil || n != 2 {
		t.Fatalf("clear seed: %d %v", n, err)
	}
	// One of the prerequisite groups may match; every member within it must match.
	q, err := s.AcceptQuestGroups(ctx, account, role.ID, 10, version, 1, 115, [][]uint32{{3}, {1, 2}}, math.MaxUint32, "quest-test-v1")
	if err != nil || q.Progress != math.MaxUint32 {
		t.Fatalf("accept or progress width: %+v %v", q, err)
	}
	if n, err := s.ClearQuests(ctx, account, role.ID, version, []uint16{10, 11}); err != nil || n != 1 {
		t.Fatalf("existing accepted overwritten: %d %v", n, err)
	}
	q, err = s.AcceptQuestGroups(ctx, account, role.ID, 10, version, 1, 115, nil, 1, "quest-test-v1")
	if err != nil || q.Progress != math.MaxUint32 {
		t.Fatalf("accept replay reset progress: %+v %v", q, err)
	}
	if _, err := s.AcceptQuestGroups(ctx, account, role.ID, 12, version, 1, 115, [][]uint32{{1, 3}}, 1, "quest-test-v1"); err == nil {
		t.Fatal("incomplete group accepted")
	}
	if _, err := s.AcceptQuest(ctx, account, role.ID, 12, version, 116, 120, nil, 1, "quest-test-v1"); err == nil {
		t.Fatal("level boundary bypass")
	}
	if n, err := s.ClearActQuests(ctx, account, role.ID, version, []uint16{10, 12}); err == nil || n != 0 {
		t.Fatalf("partial batch committed: %d %v", n, err)
	}
	if ok, err := s.CompleteQuestObjective(ctx, account, role.ID, 10, version, "quest-test-v1"); err != nil || !ok {
		t.Fatalf("batch failed to roll back: %v %v", ok, err)
	}
	if ok, err := s.CompleteQuestObjective(ctx, account, role.ID, 10, version, "quest-test-v1"); err != nil || ok {
		t.Fatalf("objective repeated: %v %v", ok, err)
	}
	before := append(json.RawMessage(nil), role.State...)
	boom := errors.New("reward failure")
	if _, err := s.CommitQuestReward(ctx, account, role.ID, 10, version, "quest-test-v1", "reward-test", func(Character) (json.RawMessage, json.RawMessage, error) { return nil, nil, boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	var receipts int
	if err := testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM character_quest_rewards`).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("failed reward recorded: %d %v", receipts, err)
	}
	count := 0
	apply := func(Character) (json.RawMessage, json.RawMessage, error) {
		count++
		return json.RawMessage(`{"level":115,"unknown":{"retained":true},"gold":10}`), json.RawMessage(`{"gold":10}`), nil
	}
	first, err := s.CommitQuestReward(ctx, account, role.ID, 10, version, "quest-test-v1", "reward-test", apply)
	if err != nil || !first.Applied || sameJSON(t, first.Character.State, before) {
		t.Fatalf("reward failed: %+v %v", first, err)
	}
	replay, err := s.CommitQuestReward(ctx, account, role.ID, 10, version, "quest-test-v1", "reward-test", apply)
	if err != nil || replay.Applied || count != 1 || !sameJSON(t, replay.Receipt, first.Receipt) {
		t.Fatalf("reward replay: %+v %v callback=%d", replay, err, count)
	}
	completed, err := s.CompletedQuestIDs(ctx, account, version, []uint16{1, 10, 12})
	if err != nil || len(completed[role.ID]) != 2 {
		t.Fatalf("completed projection: %+v %v", completed, err)
	}
	if err := s.AbandonQuest(ctx, account, role.ID, 10); err == nil {
		t.Fatal("completed quest abandoned")
	}
	if _, err := s.AcceptQuest(ctx, account, role.ID, 12, version, 1, 115, nil, 1, "map-test"); err != nil {
		t.Fatal(err)
	}
	run := strings.Repeat("1", 32)
	if ok, err := s.RecordQuestMapClear(ctx, account, role.ID, run, 1<<31, version, "map-test", []uint16{12}); err != nil || !ok {
		t.Fatalf("map clear: %v %v", ok, err)
	}
	if err := s.AbandonQuest(ctx, account, role.ID, 12); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptQuest(ctx, account, role.ID, 12, version, 1, 115, nil, 1, "map-test"); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.RecordQuestMapClear(ctx, account, role.ID, run, 1<<31, version, "map-test", []uint16{12}); err != nil || ok {
		t.Fatalf("old map evidence reused: %v %v", ok, err)
	}
	if ok, err := s.RecordQuestMapClear(ctx, account, role.ID, strings.Repeat("2", 32), 8, version, "map-test", []uint16{12, 0}); err == nil || ok {
		t.Fatalf("invalid batch accepted: %v %v", ok, err)
	}
	if err := s.MarkMeetNPCQuest(ctx, account+100, role.ID, 12, version, "map-test"); err == nil {
		t.Fatal("meet NPC ownership bypass")
	}
	rows, err := s.Quests(ctx, account, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == 12 && row.Progress != 1 {
			t.Fatalf("map replay or failed batch changed quest: %+v", row)
		}
	}
}

func TestSQLCLegacyQuestRepairPreservesAudit(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateQuests(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "legacyquest")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("b", 64)
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "LegacyQuest", Request: []byte{1}, ConfigVersion: version, State: json.RawMessage(`{"level":115}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	_, err = testPool(t, s).Exec(ctx, `INSERT INTO character_quests(character_id,quest_id,status,progress,config_version,progress_model,accepted_at,completed_at)
 VALUES($1,30,'accepted',0,$2,'legacy-zero','2026-10-01',NULL),
 ($1,31,'completed',0,$2,'act-clear-v1','2026-10-01','2026-10-01'),
 ($1,32,'completed',0,$2,'act-clear-v1','2026-10-01','2026-10-02')`, role.ID, version)
	if err != nil {
		t.Fatal(err)
	}
	before := QuestState{ID: 30, Status: "accepted", Progress: 0, ConfigVersion: version, ProgressModel: "legacy-zero"}
	if err := s.RepairLegacyQuest(ctx, account, role.ID, before, math.MaxUint32, "source-model"); err != nil {
		t.Fatal(err)
	}
	if err := s.RepairLegacyQuest(ctx, account, role.ID, before, 1, "source-model"); err == nil {
		t.Fatal("stale legacy repair overwrote progress")
	}
	for i := 0; i < 2; i++ {
		if err := s.MigrateQuests(ctx); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.Quests(ctx, account, role.ID)
	if err != nil || len(rows) != 2 || rows[0].ID != 30 || rows[0].Progress != math.MaxUint32 || rows[1].ID != 32 {
		t.Fatalf("legacy repair lost real progress: %+v %v", rows, err)
	}
	var n int
	if err := testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM character_quest_repairs WHERE character_id=$1`, role.ID).Scan(&n); err != nil || n != 2 {
		t.Fatalf("audit duplicated or missing: %d %v", n, err)
	}
}
