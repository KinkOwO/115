package workflow

import (
	"context"
	"dfolan/internal/catalog"

	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestKnightShieldTransactionsIntegration(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("requires explicit DFO_TEST_POSTGRES_DSN for isolated PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fixture, e := database.OpenTestFixture(ctx)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})
	store := fixture.Storage()
	for _, fn := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateCashShop} {
		if e = fn(ctx); e != nil {
			t.Fatal(e)
		}
	}
	s := knightDeckTestService(t)
	w := &WearService{WearService: *s, Store: store}
	account, e := store.DevelopmentAccount(ctx, "knight-fixture")
	if e != nil {
		t.Fatal(e)
	}
	role := knightRole(t, s, [5]uint32{113370003, 113370008})
	bag, _ := inventory.ReadBag(role.State)
	bag.Gold = 122
	bag.Coin = 9
	bag.ExpandEquipFlags = 63
	bag.CreatureExperience = map[uint32]uint32{3: 42}
	// Exercise the old coin/package migrations after deck persistence too.
	bag.Items = []inventory.BagItem{{Slot: 65, Template: 1, Amount: 3}, {Slot: 66, Template: 590722921, Amount: 1}}
	role.State, e = inventory.SaveBag(role.State, bag)
	if e != nil {
		t.Fatal(e)
	}
	role.AccountID = account
	role.Name = "KnightFixture"
	role.Request = []byte{0}
	role, e = store.CreateCharacter(ctx, role, 24)
	if e != nil {
		t.Fatal(e)
	}
	deck := [5]uint32{113370008, 113370003}
	saved, applied, e := w.CommitKnightDeck(ctx, role, "knight-fixture-upload-1", deck)
	if e != nil || !applied {
		t.Fatal(applied, e)
	}
	again, applied, e := w.CommitKnightDeck(ctx, role, "knight-fixture-upload-1", deck)
	if e != nil || applied {
		t.Fatal("replay applied twice", e)
	}
	b, _ := inventory.ReadBag(again.State)
	if b.KnightDeck() != deck {
		t.Fatal("replay changed deck")
	}
	if _, _, e = w.CommitKnightDeck(ctx, role, "knight-fixture-invalid-1", [5]uint32{113370007}); e == nil {
		t.Fatal("quest upload was accepted")
	}
	events, e := fixture.EventCount(ctx, role.ID)
	if e != nil || events != 1 {
		t.Fatal("failed upload left receipt", events, e)
	}
	before, e := fixture.CharacterState(ctx, role.ID)
	if e != nil {
		t.Fatal(e)
	}
	var aState, bState any
	json.Unmarshal(saved.State, &aState)
	json.Unmarshal(before, &bState)
	if !reflect.DeepEqual(aState, bState) {
		t.Fatal("refusal changed saved state")
	}
	// Reintroduce the legacy coin row after normal bag canonicalization, so
	// both SQL-selected cash migrations really run against the fixture.
	if e = fixture.SeedInventoryItems(ctx, role.ID, json.RawMessage(`[{"slot":65,"Template":1,"Amount":3},{"slot":66,"Template":590722921,"Amount":1}]`)); e != nil {
		t.Fatal(e)
	}
	if e = store.MigrateCashShop(ctx); e != nil {
		t.Fatal(e)
	}
	migrated, e := fixture.CharacterState(ctx, role.ID)
	if e != nil {
		t.Fatal(e)
	}
	b, e = inventory.ReadBag(migrated)
	if e != nil {
		t.Fatal(e)
	}
	if b.KnightDeck() != deck || b.Coin != 15 || b.ExpandEquipFlags != 63 || b.CreatureExperience[3] != 42 || b.Gold != 122 {
		t.Fatal("cash migrations lost shield/pet/expansion assets", b)
	}
}

func knightDeckTestService(t *testing.T) *inventory.WearService {
	t.Helper()
	jobs, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	shields, e := inventory.LoadKnightShields("../../configs/equipment-knight-shield.full-candidate.json", jobs.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	full, e := inventory.OpenFullEquipmentCatalog("../inventory/testdata/equipment-flow", jobs.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { full.Close() })
	rules, e := inventory.LoadWearRules("../../configs/equipment-wear.current35.json", jobs.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	return &inventory.WearService{Catalog: &inventory.EquipmentCatalog{Source: jobs.Source, Full: full}, Professions: jobs, Rules: rules, Shields: shields}
}

func knightRole(t *testing.T, s *inventory.WearService, deck [5]uint32) database.Character {
	t.Helper()
	b := inventory.Bag{Version: "ordinary-bag-v1", KnightShieldDeck: append([]uint32(nil), deck[:]...)}
	if deck[0] != 0 {
		b.Worn = []inventory.BagEquipment{{Slot: 24, Template: deck[0]}}
	}
	raw, e := inventory.SaveBag(json.RawMessage(`{"level":90,"advancement":1}`), b)
	if e != nil {
		t.Fatal(e)
	}
	return database.Character{Profession: s.Shields.Profession, ConfigVersion: s.Catalog.Source.SaveIdentity(), State: raw}
}
