package loot

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
)

func TestBlackPurgatoryRewardFreezesWithSaveIdentityAndReplays(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	store := f.Storage()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "black-reward-identity")
	if err != nil {
		t.Fatal(err)
	}
	s := Service{}
	s.Catalog.Source.Checksum = strings.Repeat("a", 64)
	version := s.Catalog.Source.SaveIdentity()
	if version == s.Catalog.Source.Checksum {
		t.Fatal("fixture must distinguish archive and save identities")
	}
	state := json.RawMessage(`{"level":115,"future_field":42}`)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "BlackReward", Request: []byte{}, ConfigVersion: version, State: state}, 24)
	if err != nil {
		t.Fatal(err)
	}
	s.BlackPurgatory = &BlackPurgatoryRewards{
		Source:         s.Catalog.Source.Checksum,
		Cards:          []RewardBoxCandidate{{Template: 701, Weight: 10000}},
		Boss:           blackPurgatoryBossRules{Model: blackPurgatoryBossModel, Denominator: 1000000, Rates: map[string]uint32{}},
		bossDurability: map[uint32]uint16{702: 50},
		boxes:          fakeBoxes{items: map[uint32]bool{701: true}},
	}
	d := &dungeon.Session{RunID: "0123456789abcdef0123456789abcdef", Loaded: true,
		Definition: catalog.DungeonDefinition{ID: BlackPurgatorySquadDungeon}, Room: catalog.DungeonRoom{Map: 100003073},
		Monsters: []protocol.DungeonMonster{{Entity: 4202, Template: 109012760, Rank: 3}}, Dead: map[uint16]bool{4202: true}}
	day := time.Now().UTC().Truncate(24 * time.Hour)
	week := day.AddDate(0, 0, -3)
	if _, err = store.BlackPurgatoryQuota(ctx, account, role.ID, d.RunID, "enter", day, week); err != nil {
		t.Fatal(err)
	}
	p, err := s.PlanBlackPurgatoryCards(Role{ConfigVersion: role.ConfigVersion}, d, 42)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	freeze := func(seed uint32) (json.RawMessage, error) {
		return store.FreezeBlackPurgatoryReward(ctx, account, role.ID, p.Source, p.Run, p.Model, func() (json.RawMessage, error) {
			calls++
			return s.PrepareBlackPurgatoryCards(p, seed)
		})
	}
	raw, err := freeze(42)
	if err != nil {
		t.Fatalf("final boss reward freeze blocked: %v", err)
	}
	var frozen CardPlan
	if err = json.Unmarshal(raw, &frozen); err != nil {
		t.Fatal(err)
	}
	if frozen.Source != version || frozen.Items[0] != (Award{Template: 701, Amount: 1}) {
		t.Fatalf("invalid frozen plan: %+v", frozen)
	}
	if err = s.ValidateBlackPurgatoryCards(frozen, d); err != nil {
		t.Fatal(err)
	}
	if err = s.ValidateBlackPurgatoryBossPlan(Role{ConfigVersion: version}, CardPlan{Run: p.Run, Source: frozen.Source, Model: p.Model, BossModel: blackPurgatoryBossModel, BossItems: [3]Award{{Template: 702, Amount: 1}}}, p.Run, 1, Award{Template: 702, Amount: 1}); err != nil {
		t.Fatal(err)
	}
	replay, err := freeze(999)
	if err != nil || !bytes.Equal(raw, replay) || calls != 1 {
		t.Fatalf("replay rerolled reward: calls=%d err=%v", calls, err)
	}
	quota, err := store.BlackPurgatoryQuota(ctx, account, role.ID, "", "read", day, week)
	if err != nil || quota.Daily != 0 || quota.Weekly != 1 {
		t.Fatalf("clear quota changed: %+v %v", quota, err)
	}
	if _, err = store.FreezeBlackPurgatoryReward(ctx, account, role.ID, s.Catalog.Source.Checksum, p.Run, p.Model, func() (json.RawMessage, error) { t.Fatal("wrong version reached draw"); return nil, nil }); err == nil {
		t.Fatal("archive hash accepted as character version")
	}
	frozen.Source = s.Catalog.Source.Checksum
	if err = s.ValidateBlackPurgatoryCards(frozen, d); err == nil {
		t.Fatal("archive hash accepted as saved reward identity")
	}
	roles, err := store.Characters(ctx, account)
	if err != nil || len(roles) != 1 || !bytes.Equal(roles[0].State, role.State) {
		t.Fatalf("reward freeze altered character state: %v", err)
	}
	other := s.Catalog
	other.Source.Checksum = strings.Repeat("b", 64)
	if _, err = s.BlackPurgatory.StorageCatalog(other); err == nil {
		t.Fatal("different content archive accepted")
	}
}
