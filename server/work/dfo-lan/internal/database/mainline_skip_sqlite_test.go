package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/quest"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// A conflicting audit event makes the final write fail after the quests and
// rewards were written. The whole skip must roll back, then concurrent retries
// must pay only once and preserve fields outside this operation.
func TestSQLiteMainlineSkipRollbackAndConcurrentReplay(t *testing.T) {
	s, account, role, version := sqliteAbsenceStore(t)
	ctx := context.Background()
	raw := json.RawMessage(`{"level":115,"gold":20,"unknown":{"keep":true}}`)
	if err := s.queries.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: role.ID, State: raw}); err != nil {
		t.Fatal(err)
	}
	if err := s.queries.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: role.ID, EventKey: "gm-quest-sweep:fixture", ConfigVersion: version, Model: "old", Outcome: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	apply := func(current Character, states []quest.QuestState, ticket json.RawMessage, prior bool) (quest.SkipBatch, error) {
		if len(states) > 0 {
			return quest.SkipBatch{State: current.State}, nil
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(current.State, &fields); err != nil {
			return quest.SkipBatch{}, err
		}
		fields["gold"] = json.RawMessage(`40`)
		state, err := json.Marshal(fields)
		return quest.SkipBatch{State: state, EventKey: "gm-quest-sweep:fixture", Outcome: json.RawMessage(`{}`), Rows: []quest.SkipRow{{Quest: 101, Model: "fixture", Receipt: json.RawMessage(`{}`)}, {Quest: 102, Model: "fixture", Receipt: json.RawMessage(`{}`)}}}, err
	}
	skip := func() (quest.SkipCommit, error) {
		return s.CommitMainlineSkip(ctx, account, role.ID, version, "fixture-reward", apply)
	}
	if _, err := skip(); err == nil {
		t.Fatal("conflicting reward receipt must abort")
	}
	rows, err := s.queries.Quests(ctx, role.ID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("partial quests committed: %v %v", rows, err)
	}
	db := s.engine.(*sqliteEngine).db
	var after string
	if err = db.QueryRowContext(ctx, `SELECT state FROM characters WHERE id=?`, role.ID).Scan(&after); err != nil || after != string(raw) {
		t.Fatalf("rollback lost state: %s %v", after, err)
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM character_events WHERE character_id=?`, role.ID); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, e := skip()
			if e == nil && !json.Valid(result.Character.State) {
				e = fmt.Errorf("invalid replay state")
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM character_quest_rewards WHERE character_id=?`, role.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("reward count %d %v", count, err)
	}
	if err = db.QueryRowContext(ctx, `SELECT state FROM characters WHERE id=?`, role.ID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal([]byte(after), &fields)
	if string(fields["gold"]) != "40" || string(fields["unknown"]) != "{\"keep\":true}" {
		t.Fatalf("repeat paid or unrelated field changed: %s", after)
	}
}
