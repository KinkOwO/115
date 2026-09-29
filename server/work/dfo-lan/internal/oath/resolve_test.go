package oath

import (
	"reflect"
	"testing"
)

func testCatalog() Catalog {
	return Catalog{
		Version:     "pvf-test-v1",
		UnlockLevel: 115,
		Cores: map[string]CoreDefinition{
			"core-a": {
				ID: "core-a", Slot: 47, DefaultOption: 1,
				Options: []OptionDefinition{
					{ID: 0, Effects: []Effect{{EffectKey: "atk", Target: "attack", Operation: "add", Value: 1}}},
					{ID: 1, Effects: []Effect{{EffectKey: "crit", Target: "critical", Operation: "add", Value: 2}}},
				},
			},
		},
		Crystals: map[string]CrystalDefinition{
			"crystal-a": {ID: "crystal-a", AllowedSlots: []uint16{36, 37}, Effects: []Effect{{EffectKey: "cdr", Target: "cooldown", Operation: "multiply", Value: 3}}},
		},
	}
}

func TestResolveUsesDefaultOnlyWhenSelectionIsMissing(t *testing.T) {
	catalog := testCatalog()
	snapshot := CharacterSnapshot{CharacterKey: "char-1", Revision: 7, Level: 115, Core: &CoreInstance{InstanceKey: "core-i1", DefinitionID: "core-a", Slot: 47}}
	got, err := Resolve(snapshot, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedOption == nil || *got.SelectedOption != 1 {
		t.Fatalf("selected option = %v, want configured default 1", got.SelectedOption)
	}
	if len(got.ResolvedEffects) != 1 || got.ResolvedEffects[0].EffectKey != "crit" || got.ResolvedEffects[0].SourceInstanceKey != "core-i1" {
		t.Fatalf("effects = %+v", got.ResolvedEffects)
	}
}

func TestResolveRejectsMismatchedOrUnknownSelection(t *testing.T) {
	catalog := testCatalog()
	snapshot := CharacterSnapshot{CharacterKey: "char-1", Revision: 1, Level: 115, Core: &CoreInstance{InstanceKey: "core-i1", DefinitionID: "core-a", Slot: 47}, BoundOption: &BoundOption{CoreInstanceKey: "other-core", SelectedOption: 0, Revision: 1}}
	if _, err := Resolve(snapshot, catalog); err != ErrInvalidSelection {
		t.Fatalf("mismatched core error = %v, want ErrInvalidSelection", err)
	}
	snapshot.BoundOption = &BoundOption{CoreInstanceKey: "core-i1", SelectedOption: 99, Revision: 1}
	if _, err := Resolve(snapshot, catalog); err == nil {
		t.Fatal("unknown option must be rejected")
	}
}

func TestResolveProducesDeterministicAbsoluteSnapshot(t *testing.T) {
	catalog := testCatalog()
	snapshot := CharacterSnapshot{
		CharacterKey: "char-1", Revision: 12, Level: 120,
		Core:        &CoreInstance{InstanceKey: "core-i1", DefinitionID: "core-a", Slot: 47},
		BoundOption: &BoundOption{CoreInstanceKey: "core-i1", SelectedOption: 0, Revision: 2},
		Crystals:    []CrystalInstance{{InstanceKey: "crystal-i1", DefinitionID: "crystal-a", Slot: 37}},
	}
	a, err := Resolve(snapshot, catalog)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(snapshot, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("non-deterministic snapshots: a=%+v b=%+v", a, b)
	}
	if len(a.ResolvedEffects) != 2 || a.ResolvedEffects[0].SourceInstanceKey != "core-i1" || a.ResolvedEffects[1].SourceInstanceKey != "crystal-i1" {
		t.Fatalf("effects = %+v", a.ResolvedEffects)
	}
}

func TestResolveReturnsEmptyEffectsWhenLockedOrUnequipped(t *testing.T) {
	catalog := testCatalog()
	locked, err := Resolve(CharacterSnapshot{CharacterKey: "char-1", Revision: 1, Level: 114, Core: &CoreInstance{InstanceKey: "core-i1", DefinitionID: "core-a", Slot: 47}}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if locked.Unlocked || len(locked.ResolvedEffects) != 0 {
		t.Fatalf("locked snapshot = %+v", locked)
	}
	unequipped, err := Resolve(CharacterSnapshot{CharacterKey: "char-1", Revision: 2, Level: 120}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !unequipped.Unlocked || len(unequipped.ResolvedEffects) != 0 {
		t.Fatalf("unequipped snapshot = %+v", unequipped)
	}
}
