package character_test

import (
	"bytes"
	"context"
	. "dfolan/internal/character"

	"dfolan/internal/game/protocol"

	"dfolan/internal/database"
	"encoding/json"
	"fmt"

	"strings"
	"testing"
	"time"
)

func TestBuffEnhancementStorage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dbFixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := dbFixture.Close(); err != nil {
			t.Error(err)
		}
	}()
	store := dbFixture.Storage()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	s, fixture := BuffFixtureForTest(t)
	s.Store = store
	account, err := store.DevelopmentAccount(ctx, "buff-fixture")
	if err != nil {
		t.Fatal(err)
	}
	fixture.AccountID, fixture.ID, fixture.Name, fixture.Request = account, 0, "BuffFixture", []byte{0}
	role, err := store.CreateCharacter(ctx, fixture, 24)
	if err != nil {
		t.Fatal(err)
	}
	before := role.State
	role, _, err = s.SaveBuffEnhancement(ctx, role, "edit-1", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 57})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.SaveBuffEnhancement(ctx, role, "bad", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 58}); err == nil {
		t.Fatal("missing item accepted")
	}
	other := role
	other.AccountID++
	if _, _, err = s.SaveBuffEnhancement(ctx, other, "wrong-owner", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 46, Slot: 65535}); err == nil {
		t.Fatal("wrong owner accepted")
	}
	for i, skill := range []uint16{0, 306, 0, 306} {
		role, _, err = s.SaveBuffEnhancement(ctx, role, fmt.Sprintf("select-%d", i), protocol.BuffEnhancementRequest{Skill: skill, Kind: 48, List: 46, Slot: 65535})
		if err != nil {
			t.Fatal(err)
		}
	}
	second, err := dbFixture.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var restored Character
	rows, err := second.Characters(ctx, account)
	if err != nil || len(rows) != 1 {
		t.Fatalf("reconnect: %v", err)
	}
	restored = rows[0]
	got, err := s.BuffEnhancementRestore(restored)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatalf("reconnect restore %x %v", got, err)
	}
	var old, next map[string]json.RawMessage
	json.Unmarshal(before, &old)
	json.Unmarshal(restored.State, &next)
	for _, key := range []string{"inventory", "future", "quest_state", "learned_skills"} {
		var a, b any
		json.Unmarshal(old[key], &a)
		json.Unmarshal(next[key], &b)
		ar, _ := json.Marshal(a)
		br, _ := json.Marshal(b)
		if !bytes.Equal(ar, br) {
			t.Fatalf("changed %s", key)
		}
	}
	role, _, err = s.SaveBuffEnhancement(ctx, restored, "clear", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 46, Slot: 65535})
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 0}) {
		t.Fatal("database clear failed")
	}
	// Same event key must not reapply a conflicting request after a successful clear.
	role, changed, err := s.SaveBuffEnhancement(ctx, role, "clear", protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 57})
	if err != nil || changed {
		t.Fatal("receipt replay changed state")
	}
	if !strings.Contains(string(role.State), "buff_enhancement") {
		t.Fatal("save missing")
	}
}
