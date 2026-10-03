package character

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// Exercise the transaction owners, including receipt replay, rather than only
// the formula helpers (which already accepted a difficulty table index).
type difficultyExperienceStore struct {
	role     Character
	receipts map[string]json.RawMessage
}

func (s *difficultyExperienceStore) CommitCharacterEvent(_ context.Context, _, _ int64, _, key, _ string, apply func(Character) (json.RawMessage, json.RawMessage, error)) (Character, bool, error) {
	if _, ok := s.receipts[key]; ok {
		return s.role, false, nil
	}
	state, receipt, err := apply(s.role)
	if err != nil {
		return s.role, false, err
	}
	s.role.State = state
	s.receipts[key] = receipt
	return s.role, true, nil
}
func (s *difficultyExperienceStore) CharacterEventReceipt(_ context.Context, _, _ int64, key string) (json.RawMessage, error) {
	return s.receipts[key], nil
}
func (s *difficultyExperienceStore) HasGrowthPremium(context.Context, int64, time.Time) (bool, error) {
	return false, nil
}
func (s *difficultyExperienceStore) HasTacticianPremium(context.Context, int64, time.Time) (bool, error) {
	return false, nil
}
func (s *difficultyExperienceStore) RunFatigueLedger(context.Context, int64, int64, string) (int64, int64, error) {
	return 0, 0, nil
}
func (s *difficultyExperienceStore) RunMonsterExperience(_ context.Context, _, _ int64, run string) (uint64, error) {
	var receipt struct{ Gain uint64 }
	if raw := s.receipts["monster:"+run+":10:1"]; len(raw) != 0 {
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return 0, err
		}
	}
	return receipt.Gain, nil
}

func TestDungeonExperienceUsesSelectedDifficulty(t *testing.T) {
	for _, tc := range []struct {
		difficulty byte
		want       uint64
	}{
		{0, 130}, {1, 130}, {2, 200}, {3, 250}, {4, 300}, {5, 400}, {6, 0}, {255, 0},
	} {
		t.Run(fmt.Sprintf("difficulty%d", tc.difficulty), func(t *testing.T) {
			s := advancementProgressionService(t)
			role := ordinaryAdvancedRole(t, s, 12, 0)
			before := decodeState(t, role.State)
			store := &difficultyExperienceStore{role: role, receipts: map[string]json.RawMessage{}}
			s.Store = store
			// Rates taken from the current PVF etc/(r)serverparameter.etc.
			s.Catalog.DifficultyRates = []float32{1.3, 2, 2.5, 3, 4}
			s.Catalog.MonsterExperience = map[uint16]uint64{15: 100}
			s.Catalog.MonsterRates = []float32{1, 1, 1, 1}
			s.Catalog.RankThresholds = []byte{50}
			s.Catalog.ClearRankRates = []float32{0.1}
			s.Catalog.Scripts["n_quest/questparameter.etc"] = catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[exp reward table]"}, {Type: 0, Value: 100}, {Type: 0, Value: -1}}}
			s.Rules.Penalty = map[int]float32{0: 1}
			s.Rules.NamedMultiplier = 1
			monster := protocol.DungeonMonster{Entity: 1, Template: 100, Level: 15, Rank: 3, Team: 100}
			room := catalog.DungeonRoom{Map: 10, Boss: true}
			run := &dungeon.Session{RunID: "00112233445566778899aabbccddeeff", StartedAt: time.Now().Add(-time.Second), Difficulty: tc.difficulty,
				Definition: catalog.DungeonDefinition{ID: 3, BasisLevel: 1}, Room: room, Maze: catalog.DungeonMaze{Rooms: []catalog.DungeonRoom{room}},
				Loaded: true, Monsters: []protocol.DungeonMonster{monster}, Dead: map[uint16]bool{1: true}, Visited: map[uint32][]protocol.DungeonMonster{10: {monster}}}
			ctx := context.Background()
			next, applied, err := s.Monster(ctx, role, run, 1)
			if tc.want == 0 {
				if err == nil || applied || !bytes.Equal(next.State, role.State) || len(store.receipts) != 0 {
					t.Fatal("unsupported monster difficulty changed save")
				}
				if err = run.BossCheck(protocol.BossCheckRequest{Actor: role.WireID, Target: 1}, role.WireID); err != nil {
					t.Fatal(err)
				}
				next, _, applied, err = s.Clear(ctx, role, run, 50, time.Now())
				if err == nil || applied || !bytes.Equal(next.State, role.State) || len(store.receipts) != 0 {
					t.Fatal("unsupported clear difficulty changed save")
				}
				return
			}
			if err != nil || !applied {
				t.Fatalf("monster: applied=%v err=%v", applied, err)
			}
			if got := decodeState(t, next.State).Experience - before.Experience; got != tc.want {
				t.Fatalf("monster gain=%d want=%d", got, tc.want)
			}
			monsterState := bytes.Clone(next.State)
			next, applied, err = s.Monster(ctx, next, run, 1)
			if err != nil || applied || !bytes.Equal(next.State, monsterState) {
				t.Fatalf("monster replay changed save: %v", err)
			}
			if err = run.BossCheck(protocol.BossCheckRequest{Actor: role.WireID, Target: 1}, role.WireID); err != nil || !run.Completed() {
				t.Fatalf("completion: %v", err)
			}
			now := time.Now()
			next, receipt, applied, err := s.Clear(ctx, next, run, 50, now)
			if err != nil || !applied {
				t.Fatalf("clear: applied=%v err=%v", applied, err)
			}
			if receipt.Base != uint32(tc.want) || receipt.Score != uint32(tc.want/10) || receipt.MonsterExperience != uint32(tc.want) {
				t.Fatalf("incorrect clear receipt: %+v", receipt)
			}
			if got := decodeState(t, next.State).Experience - before.Experience; got != tc.want*2+tc.want/10 {
				t.Fatalf("total gain=%d", got)
			}
			clearState := bytes.Clone(next.State)
			next, _, applied, err = s.Clear(ctx, next, run, 50, now)
			if err != nil || applied || !bytes.Equal(next.State, clearState) || len(store.receipts) != 2 {
				t.Fatalf("clear replay changed save: %v", err)
			}
			var saved, original map[string]json.RawMessage
			json.Unmarshal(next.State, &saved)
			json.Unmarshal(role.State, &original)
			for _, key := range []string{"inventory", "future_progression_field"} {
				if !bytes.Equal(saved[key], original[key]) {
					t.Fatalf("lost existing save field %s", key)
				}
			}
		})
	}
}
