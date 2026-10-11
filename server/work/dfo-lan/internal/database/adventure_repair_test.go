package database

import (
	"context"
	"dfolan/internal/adventure"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func installAdventureTestRules(t *testing.T) {
	t.Helper()
	r, err := adventure.NewRules(adventure.Rules{SourceChecksum: strings.Repeat("a", 64), MaxLevel: 3, ExpRate: 0.3,
		Experience: map[uint32]uint64{1: 0, 2: 6, 3: 9}, Shops: map[byte]adventure.Shop{
			0: {MaxPoints: 10, ExpPoints: []uint64{3, 2}, Items: []adventure.ShopItem{{Template: 42, Level: 1, Price: 1, Limit: 5, Reset: 3}}},
			3: {MaxPoints: 20, ResetPoints: true},
		}, Items: map[uint32]adventure.Item{42: {Path: "fixture/42.stk", Type: "[throw]", Limit: 10}}})
	if err != nil {
		t.Fatal(err)
	}
	restore, err := adventure.InstallRules(r)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
}

func newAdventureTestRole(t *testing.T, s *Store, ctx context.Context, account int64, name string) Character {
	t.Helper()
	r, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: name, Profession: 0, Request: []byte{},
		ConfigVersion: strings.Repeat("a", 64), State: json.RawMessage(`{"level":100,"advancement":1,"unknown_future_field":{"keep":true}}`)}, 100)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAdventureExperienceAccountTransactionAndReplay(t *testing.T) {
	installAdventureTestRules(t)
	s, ctx := sqlcTestStore(t)
	if err := s.MigrateAdventure(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "adventure-exp")
	if err != nil {
		t.Fatal(err)
	}
	a := newAdventureTestRole(t, s, ctx, account, "A")
	b := newAdventureTestRole(t, s, ctx, account, "B")
	_, _, _, err = s.CommitAdventure(ctx, account, a.ID, "seed", func(role Character, p *AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		p.Experience = 5
		return role.State, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	applyGain := func(role Character, key string, gain uint64) {
		t.Helper()
		_, applied, err := s.CommitCharacterEvent(ctx, account, role.ID, role.ConfigVersion, key, "adventure-test-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(current.State, &fields); err != nil {
				return nil, nil, err
			}
			var total uint64
			if old := fields["adventure_earned_experience"]; old != nil {
				if err := json.Unmarshal(old, &total); err != nil {
					return nil, nil, err
				}
			}
			fields["adventure_earned_experience"], _ = json.Marshal(total + gain)
			state, err := json.Marshal(fields)
			return state, json.RawMessage(`{}`), err
		})
		if err != nil || !applied {
			t.Fatal("gain failed", key, applied, err)
		}
	}
	for i := 0; i < 4; i++ {
		applyGain(a, fmt.Sprintf("tiny-%d", i), 1)
	}
	p, err := s.LoadAdventure(ctx, account, a.ID, "unused")
	if err != nil || p.Level != 2 || p.Experience != 0 || p.Data.ExperienceRemainder != 2 {
		t.Fatal("fractional level boundary failed", p, err)
	}
	applyGain(a, "large", 30)
	p, err = s.LoadAdventure(ctx, account, b.ID, "unused")
	if err != nil || p.Level != 3 || p.Experience != 0 || p.Data.Points[0] != 6 || p.Data.PointExperience != 1 {
		t.Fatal("shared cap/points failed", p, err)
	}
	_, applied, err := s.CommitCharacterEvent(ctx, account, a.ID, a.ConfigVersion, "large", "adventure-test-v1", func(Character) (json.RawMessage, json.RawMessage, error) {
		t.Fatal("replay recalculated reward")
		return nil, nil, nil
	})
	if err != nil || applied {
		t.Fatal("replay applied", applied, err)
	}
	applyGain(b, "other-character", 10)
	p, err = s.LoadAdventure(ctx, account, a.ID, "unused")
	if err != nil || p.Data.Points[0] != 8 || p.Experience != 0 {
		t.Fatal("other character overwrote account", p, err)
	}
	if _, _, err := s.CommitCharacterEvent(ctx, account+1000, a.ID, a.ConfigVersion, "foreign", "adventure-test-v1", func(Character) (json.RawMessage, json.RawMessage, error) {
		t.Fatal("foreign account reached mutation callback")
		return nil, nil, nil
	}); err == nil {
		t.Fatal("foreign account accepted")
	}
	var state map[string]json.RawMessage
	roles, err := s.Characters(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(roles[0].State, &state); err != nil || string(state["unknown_future_field"]) != `{"keep":true}` {
		t.Fatal("unrelated character fields lost", state, err)
	}
}

func TestAdventureCalendarKeepsPermanentPointsAndResetsMonthlyLimits(t *testing.T) {
	installAdventureTestRules(t)
	s, ctx := sqlcTestStore(t)
	if err := s.MigrateAdventure(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "adventure-calendar")
	if err != nil {
		t.Fatal(err)
	}
	a := newAdventureTestRole(t, s, ctx, account, "Calendar")
	_, _, _, err = s.CommitAdventure(ctx, account, a.ID, "seed", func(role Character, p *AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		p.Data.Points[0], p.Data.Points[3] = 7, 19
		p.Data.Purchases[42] = 4
		p.Data.Period, p.Data.LastConnectionDay, p.Data.ConnectionDays = "2026-01", "2026-01-31", 9
		return role.State, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.PrepareAdventure(ctx, a, "2026-02-01")
	if err != nil || p.Data.Points[0] != 7 || p.Data.Points[3] != 0 || p.Data.Purchases[42] != 0 || p.Data.ConnectionDays != 10 {
		t.Fatal("monthly/daily reset failed", p, err)
	}
	p, err = s.PrepareAdventure(ctx, a, "2026-02-01")
	if err != nil || p.Data.ConnectionDays != 10 {
		t.Fatal("same day counted twice", p, err)
	}
	p, err = s.PrepareAdventure(ctx, a, "2026-02-03")
	if err != nil || p.Data.ConnectionDays != 1 {
		t.Fatal("broken streak not reset", p, err)
	}
}
