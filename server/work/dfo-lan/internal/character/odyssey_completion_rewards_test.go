package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func completionFixture(t *testing.T) (*ProgressionService, Character) {
	t.Helper()
	s, role := odysseyChapterFixture(t)
	r, err := catalog.LoadOdysseyCompletionRewards("../../configs/odyssey-completion-rewards.json")
	if err != nil {
		t.Fatal(err)
	}
	s.CompletionRewards = r
	items := map[uint32]catalog.LootItem{}
	for _, ch := range r.Chapters {
		for _, item := range ch.Rewards {
			items[item.Template] = catalog.LootItem{ID: item.Template, Kind: "stackable", StackableType: "[booster]"}
		}
	}
	items[r.Honor.Template] = catalog.LootItem{ID: r.Honor.Template, Kind: "stackable", StackableType: "[booster]", StackLimit: 1}
	c := catalog.LootCatalog{Items: items}
	c.Source.Checksum = catalog.OdysseySource
	s.CompletionAwarder = &inventory.Awarder{Catalog: c, Rules: inventory.BagRules{Source: catalog.OdysseySource, Slots: map[string][2]uint16{"[booster]": {65, 120}}, MissingStackLimit: 1000}}
	return s, role
}

func completionState(t *testing.T, role Character, fields map[string]any) Character {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &doc); err != nil {
		t.Fatal(err)
	}
	for k, v := range fields {
		doc[k], _ = json.Marshal(v)
	}
	role.State, _ = json.Marshal(doc)
	return role
}

type completionStore struct {
	ProgressionStore
	role Character
	paid map[string]bool
}

func (f *completionStore) CommitCharacterEvent(_ context.Context, _, _ int64, _, key, _ string, apply func(Character) (json.RawMessage, json.RawMessage, error)) (Character, bool, error) {
	if f.paid[key] {
		return f.role, false, nil
	}
	raw, _, err := apply(f.role)
	if err != nil {
		return f.role, false, err
	}
	f.role.State = raw
	f.paid[key] = true
	return f.role, true, nil
}

func TestOdysseyCompletionRetryAfterFullBagAndGraduation(t *testing.T) {
	s, role := completionFixture(t)
	role = completionState(t, role, map[string]any{"odyssey_completed_dungeons": []uint32{100004945}, "unrelated": "keep"})
	f := &completionStore{role: role, paid: map[string]bool{}}
	s.Store = f
	s.CompletionAwarder.Rules.Slots["[booster]"] = [2]uint16{65, 65}
	next, changed, pending := s.OdysseyChapterRewards(context.Background(), role)
	if !changed || len(pending) != 3 || len(f.paid) != 1 {
		t.Fatal(changed, pending, f.paid)
	}
	// Graduation must not lose three unpaid lines. A larger bag retries only
	// those lines and never repeats the already-paid weapon box.
	f.role = completionState(t, next, map[string]any{"odyssey_graduated": true})
	s.CompletionAwarder.Rules.Slots["[booster]"] = [2]uint16{65, 120}
	next, changed, pending = s.OdysseyChapterRewards(context.Background(), f.role)
	if !changed || len(pending) != 0 || len(f.paid) != 4 {
		t.Fatal(changed, pending, f.paid)
	}
	bag, err := inventory.ReadBag(next.State)
	if err != nil || len(bag.Items) != 4 {
		t.Fatal(bag, err)
	}
	for _, it := range bag.Items {
		if it.Amount != 1 {
			t.Fatal("duplicate payment", it)
		}
	}
	if _, changed, pending = s.OdysseyChapterRewards(context.Background(), next); changed || len(pending) != 0 {
		t.Fatal("replay", changed, pending)
	}
	var doc map[string]any
	json.Unmarshal(next.State, &doc)
	if doc["unrelated"] != "keep" {
		t.Fatal("save field lost")
	}
}

func TestOdysseyCompletionLockedEligibility(t *testing.T) {
	s, role := completionFixture(t)
	r := s.CompletionRewards.At(1)[0]
	if _, _, err := s.ApplyOdysseyChapterReward(role, 1, 0, r); err == nil {
		t.Fatal("uncompleted chapter paid")
	}
	// The unlocked snapshot claims completion, but the locked row does not.
	f := &completionStore{role: role, paid: map[string]bool{}}
	s.Store = f
	snapshot := completionState(t, role, map[string]any{"odyssey_completed_dungeons": []uint32{100004945}})
	if _, changed, pending := s.OdysseyChapterRewards(context.Background(), snapshot); changed || len(pending) != 4 {
		t.Fatal(changed, pending)
	}
	role.Request = nil
	t.Setenv("DFO_ODYSSEY_MODE", "1")
	if _, _, err := s.ApplyOdysseyChapterReward(role, 1, 0, r); err == nil {
		t.Fatal("debug mode paid normal character")
	}
}

func TestOdysseyHonorMailEligibilityAndPreservation(t *testing.T) {
	s, role := completionFixture(t)
	if _, _, _, err := s.ApplyOdysseyHonorMail(role, false); err == nil {
		t.Fatal("premature honor")
	}
	role = completionState(t, role, map[string]any{"level": 115, "odyssey_graduated": true, "odyssey_graduation_reward_owed": 10420561, "unrelated": "keep"})
	for _, paid := range []bool{false, true} {
		raw, _, item, err := s.ApplyOdysseyHonorMail(role, paid)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		json.Unmarshal(raw, &doc)
		if doc["unrelated"] != "keep" || doc["odyssey_graduation_reward_owed"] != float64(0) {
			t.Fatal(doc)
		}
		if paid {
			if len(item) != 0 {
				t.Fatal("legacy bag receipt repaid")
			}
			continue
		}
		var attachment inventory.MailItem
		if err := json.Unmarshal(item, &attachment); err != nil {
			t.Fatal(err)
		}
		if attachment.Stack.Template != 10420561 || attachment.Stack.Amount != 1 || attachment.Stack.ExpireTime != inventory.GrantExpireTime {
			t.Fatal(attachment)
		}
		if _, err = attachment.Row(); err != nil {
			t.Fatal(err)
		}
	}
	// A full bag is immaterial to constructing the mailbox attachment.
	full := inventory.Bag{Version: "ordinary-bag-v1"}
	for n := uint16(65); n <= 120; n++ {
		full.Items = append(full.Items, inventory.BagItem{Slot: n, Template: 10420561, Amount: 1})
	}
	role.State, _ = inventory.SaveBag(role.State, full)
	if _, _, _, err := s.ApplyOdysseyHonorMail(role, false); err != nil {
		t.Fatal(err)
	}
	role.Request = nil
	if _, _, _, err := s.ApplyOdysseyHonorMail(role, false); err == nil {
		t.Fatal("normal character honor")
	}
}
