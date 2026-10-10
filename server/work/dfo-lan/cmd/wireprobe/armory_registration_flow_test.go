package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// Reproduces the player's coat/slot and pre-existing permanent receipt using a
// private database, never the player's save. All operations use real commits.
func TestArmoryRegistrationReusedSlotAndLegacyReceipt(t *testing.T) {
	ctx := context.Background()
	f, e := database.OpenTestFixture(ctx)
	must115(t, e)
	defer f.Close()
	store := f.Storage()
	for _, m := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateAccountMaterials, store.MigrateAccountVault} {
		must115(t, m(ctx))
	}
	account, e := store.DevelopmentAccount(ctx, "armory-registration-fixture")
	must115(t, e)
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}
	const target = uint32(100051285)
	def := primerDef(target, 6)
	def.Fields["[equipment type]"] = []pvf.Token{{Type: 6, Text: "[coat]"}}
	def.Fields["[durability]"] = []pvf.Token{{Type: 0, Value: 40}}
	eq, e := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: source, Rows: []inventory.EquipmentDefinition{def}}, source.Checksum)
	must115(t, e)
	items := &inventory.ItemService{Model: "reference90-gold-stack-v1", Catalog: catalog.LootCatalog{Source: source}, Equipment: eq, Journal: &catalog.EquipmentJournalRules{Maximum: 99}, BagRules: inventory.BagRules{Source: source.Checksum, EquipmentSlots: [2]uint16{9, 64}, Slots: map[string][2]uint16{"[material]": {121, 176}}, MissingStackLimit: math.MaxUint32}}
	bag := inventory.Bag{Version: "ordinary-bag-v1", Equipment: []inventory.BagEquipment{{Slot: 15, Template: target, TutorialLocked: true}, {Slot: 16, Template: 42}}, Worn: []inventory.BagEquipment{{Slot: 14, Template: target}}}
	raw, e := inventory.SaveBag(json.RawMessage(`{"level":115,"unrelated":42}`), bag)
	must115(t, e)
	role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "armoryfixture", Profession: 0, ConfigVersion: source.SaveIdentity(), Request: []byte{0}, State: raw}, 24)
	must115(t, e)
	request := protocol.DisjointItemRequest{ToolSlot: 65535, Items: []protocol.DisjointItemEntry{{Slot: 15, Template: target}}}
	svc := &workflow.ItemService{Store: store, Items: items}
	// The legacy key is the exact one which blocked the player's new item.
	role, _, e = store.CommitCharacterEvent(ctx, account, role.ID, source.SaveIdentity(), "disjoint:bbb3497207b169e3596d2a78", items.Model, func(c database.Character) (json.RawMessage, json.RawMessage, error) {
		return c.State, json.RawMessage(`{"deleted_slots":[15],"tool_slot":65535}`), nil
	})
	must115(t, e)
	stale := role
	saved, receipt, applied, e := svc.Disjoint(ctx, role, request)
	must115(t, e)
	if !applied || receipt.Sequence != 0 || len(receipt.JournalAdded) != 1 || len(receipt.Rewards) == 0 {
		t.Fatalf("first applied=%v receipt=%+v", applied, receipt)
	}
	check := func(c database.Character, sequence uint64, count uint32, present bool) {
		t.Helper()
		b, e := inventory.ReadBag(c.State)
		must115(t, e)
		j, e := inventory.ReadEquipmentJournal(c.State)
		must115(t, e)
		found := false
		for _, row := range b.Equipment {
			if row.Slot == 15 {
				found = true
			}
		}
		if found != present || b.DisjointSeq != sequence || j.Counts[target] != count || len(b.Worn) != 1 || b.Worn[0].Template != target {
			t.Fatalf("bag=%+v journal=%v", b, j.Counts)
		}
		other := false
		for _, row := range b.Equipment {
			if row.Slot == 16 && row.Template == 42 {
				other = true
			}
		}
		if !other {
			t.Fatal("unselected item removed")
		}
		var state map[string]json.RawMessage
		must115(t, json.Unmarshal(c.State, &state))
		if string(state["unrelated"]) != "42" {
			t.Fatal("unrelated save field changed")
		}
	}
	check(saved, 1, 1, false)
	firstRewards := append([]protocol.DisjointRewardEntry(nil), receipt.Rewards...)
	before := string(saved.State)
	replay, _, applied, e := svc.Disjoint(ctx, stale, request)
	must115(t, e)
	if applied || string(replay.State) != before {
		t.Fatal("stale snapshot replay changed assets")
	}
	// Reacquire a new instance of the same template in exactly the same bag slot.
	fresh, _, e := store.CommitCharacterEvent(ctx, account, role.ID, source.SaveIdentity(), "fixture-reacquire", items.Model, func(c database.Character) (json.RawMessage, json.RawMessage, error) {
		b, e := inventory.ReadBag(c.State)
		if e != nil {
			return nil, nil, e
		}
		b.Equipment = append(b.Equipment, inventory.BagEquipment{Slot: 15, Template: target, TutorialLocked: true})
		next, e := inventory.SaveBag(c.State, b)
		return next, json.RawMessage(`{}`), e
	})
	must115(t, e)
	replay, _, applied, e = svc.Disjoint(ctx, stale, request)
	must115(t, e)
	if applied {
		t.Fatal("old receipt consumed newly acquired equipment")
	}
	check(replay, 1, 1, true)
	second, receipt, applied, e := svc.Disjoint(ctx, fresh, request)
	must115(t, e)
	if !applied || receipt.Sequence != 1 {
		t.Fatalf("second applied=%v receipt=%+v", applied, receipt)
	}
	check(second, 2, 2, false)
	firstBag, e := inventory.ReadBag(saved.State)
	must115(t, e)
	secondBag, e := inventory.ReadBag(second.State)
	must115(t, e)
	totals := func(b inventory.Bag) map[uint32]uint32 {
		m := map[uint32]uint32{}
		for _, it := range b.Items {
			m[it.Template] += it.Amount
		}
		return m
	}
	// Some soul rewards are random. Compare committed stacks to both durable
	// receipts, rather than incorrectly requiring two rolls to be identical.
	expected := map[uint32]uint32{}
	for _, reward := range firstRewards {
		expected[reward.Template] += reward.Count
	}
	for template, amount := range expected {
		if totals(firstBag)[template] != amount {
			t.Fatalf("first reward %d mismatch", template)
		}
	}
	for _, reward := range receipt.Rewards {
		expected[reward.Template] += reward.Count
	}
	actual := totals(secondBag)
	if len(actual) != len(expected) {
		t.Fatalf("reward templates=%v expected=%v", actual, expected)
	}
	for template, amount := range expected {
		if actual[template] != amount {
			t.Fatalf("reward %d duplicated/lost: got %d want %d", template, actual[template], amount)
		}
	}

	// A new request for the now empty slot must fail without creating a receipt.
	_, _, _, e = svc.Disjoint(ctx, second, request)
	if e == nil {
		t.Fatal("empty slot accepted")
	}
	plan, e := items.PlanDisjoint(workflow.InventoryRole(second), request)
	must115(t, e)
	if _, e = store.CharacterEventReceipt(ctx, account, role.ID, plan.Key); e == nil {
		t.Fatal("failed operation persisted a receipt")
	}
	// Verify the gateway emits deletion + authoritative inventory/journal packets,
	// and logs a stale replay accurately rather than claiming a new registration.
	w := &worldSession{account: account, store: store, role: stale, items: items}
	body, e := hex.DecodeString("00ffff010f0055a9f605000000000000")
	must115(t, e)
	var kinds []string
	packets, e := w.disjointItem(body, func(ev map[string]any) { kinds = append(kinds, ev["kind"].(string)) })
	must115(t, e)
	if !containsString(kinds, "disjoint_item_replayed") || containsString(kinds, "disjoint_journal_added") {
		t.Fatalf("events=%v", kinds)
	}
	have := map[uint16]bool{}
	for _, p := range packets {
		have[p.ID] = true
	}
	for _, id := range []uint16{26, 13, 2610} {
		if !have[id] {
			t.Fatalf("missing packet %d", id)
		}
	}
	check(w.role, 2, 2, false)
	// A fresh native CMD26 must really apply and refresh the same authoritative state.
	w.role, _, e = store.CommitCharacterEvent(ctx, account, role.ID, source.SaveIdentity(), "fixture-third-instance", items.Model, func(c database.Character) (json.RawMessage, json.RawMessage, error) {
		b, e := inventory.ReadBag(c.State)
		if e != nil {
			return nil, nil, e
		}
		b.Equipment = append(b.Equipment, inventory.BagEquipment{Slot: 15, Template: target, TutorialLocked: true})
		next, e := inventory.SaveBag(c.State, b)
		return next, json.RawMessage(`{}`), e
	})
	must115(t, e)
	kinds = nil
	packets, e = w.disjointItem(body, func(ev map[string]any) { kinds = append(kinds, ev["kind"].(string)) })
	must115(t, e)
	if !containsString(kinds, "disjoint_item_applied") || !containsString(kinds, "disjoint_journal_added") {
		t.Fatalf("events=%v", kinds)
	}
	have = map[uint16]bool{}
	for _, p := range packets {
		have[p.ID] = true
	}
	for _, id := range []uint16{26, 13, 2610} {
		if !have[id] {
			t.Fatalf("missing applied packet %d", id)
		}
	}
	check(w.role, 3, 3, false)
	// The current persisted sequence survives normal JSON save reloads.
	reloaded, e := inventory.ReadBag(w.role.State)
	must115(t, e)
	if reloaded.DisjointSeq != 3 {
		t.Fatal("sequence lost on reload")
	}
}

func containsString(items []string, wanted string) bool {
	for _, s := range items {
		if s == wanted {
			return true
		}
	}
	return false
}

func TestArmoryRegistrationRejectsStaleIdentityAndSequence(t *testing.T) {
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("b", 64)}
	items := &inventory.ItemService{Catalog: catalog.LootCatalog{Source: source}}
	request := protocol.DisjointItemRequest{ToolSlot: 65535, Items: []protocol.DisjointItemEntry{{Slot: 15, Template: 100051285}}}
	raw, e := inventory.SaveBag(json.RawMessage(`{}`), inventory.Bag{Version: "ordinary-bag-v1", Equipment: []inventory.BagEquipment{{Slot: 15, Template: 100051284}}})
	must115(t, e)
	role := inventory.Role{ConfigVersion: source.SaveIdentity(), State: raw}
	plan, e := items.PlanDisjoint(role, request)
	must115(t, e)
	if _, _, e = items.PrepareDisjoint(role, request, plan); e == nil || !strings.Contains(e.Error(), "identity") {
		t.Fatalf("wrong identity error=%v", e)
	}
	b, e := inventory.ReadBag(raw)
	must115(t, e)
	b.DisjointSeq = 1
	role.State, e = inventory.SaveBag(raw, b)
	must115(t, e)
	if _, _, e = items.PrepareDisjoint(role, request, plan); e == nil || !strings.Contains(e.Error(), "sequence") {
		t.Fatalf("stale sequence error=%v", e)
	}
	b.DisjointSeq = math.MaxUint64
	role.State, e = inventory.SaveBag(raw, b)
	must115(t, e)
	if _, e = items.PlanDisjoint(role, request); e == nil {
		t.Fatal("sequence overflow accepted")
	}
}
