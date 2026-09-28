package oath

import (
	"context"
	"testing"
)

func TestLoadOptionRejectsMissingStore(t *testing.T) {
	if _, err := LoadOption(context.Background(), nil, 1, "core"); err == nil {
		t.Fatal("expected nil-store error")
	}
}

func TestSaveOptionRejectsMissingStore(t *testing.T) {
	if _, err := SaveOption(context.Background(), nil, 1, "core", 0, 0); err == nil {
		t.Fatal("expected nil-store error")
	}
}

func TestResolveStoredWithoutCoreDoesNotTouchStore(t *testing.T) {
	catalog := Catalog{Version: "v1", UnlockLevel: 1, Cores: map[string]CoreDefinition{}, Crystals: map[string]CrystalDefinition{}}
	got, err := ResolveStored(context.Background(), nil, 1, CharacterSnapshot{CharacterKey: "c", Revision: 1, Level: 0}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if got.Unlocked || len(got.ResolvedEffects) != 0 {
		t.Fatalf("unexpected runtime snapshot: %+v", got)
	}
}
