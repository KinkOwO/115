package workflow

import (
	"context"
	"dfolan/internal/catalog"

	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestKnightShieldTransactionsIntegration(t *testing.T) {
	if os.Getenv("KNIGHT_SHIELD_INTEGRATION") != "1" {
		t.Skip("KNIGHT_SHIELD_INTEGRATION=1 uses isolated PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cfg, e := storage.LoadConfig("../../runtime/storage/local.json")
	if e != nil {
		t.Fatal(e)
	}
	live, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(live.Close)
	schema := fmt.Sprintf("knight_test_%d", time.Now().UnixNano())
	if _, e = live.DB.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if _, err := live.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg.PostgresSchema = schema
	store, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	var actualSchema string
	if e = store.DB.QueryRow(ctx, "SELECT current_schema()").Scan(&actualSchema); e != nil || actualSchema != schema {
		t.Fatal("integration schema isolation failed", actualSchema, e)
	}
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
	var events int
	if e = store.DB.QueryRow(ctx, "SELECT count(*) FROM character_events WHERE character_id=$1", role.ID).Scan(&events); e != nil || events != 1 {
		t.Fatal("failed upload left receipt", events, e)
	}
	var before json.RawMessage
	if e = store.DB.QueryRow(ctx, "SELECT state FROM characters WHERE id=$1", role.ID).Scan(&before); e != nil {
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
	if _, e = store.DB.Exec(ctx, `UPDATE characters SET state=jsonb_set(state,'{inventory,items}','[{"slot":65,"Template":1,"Amount":3},{"slot":66,"Template":590722921,"Amount":1}]'::jsonb) WHERE id=$1`, role.ID); e != nil {
		t.Fatal(e)
	}
	if e = store.MigrateCashShop(ctx); e != nil {
		t.Fatal(e)
	}
	var migrated json.RawMessage
	if e = store.DB.QueryRow(ctx, "SELECT state FROM characters WHERE id=$1", role.ID).Scan(&migrated); e != nil {
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

func knightRole(t *testing.T, s *inventory.WearService, deck [5]uint32) storage.Character {
	t.Helper()
	b := inventory.Bag{Version: "ordinary-bag-v1", KnightShieldDeck: append([]uint32(nil), deck[:]...)}
	if deck[0] != 0 {
		b.Worn = []inventory.BagEquipment{{Slot: 24, Template: deck[0]}}
	}
	raw, e := inventory.SaveBag(json.RawMessage(`{"level":90,"advancement":1}`), b)
	if e != nil {
		t.Fatal(e)
	}
	return storage.Character{Profession: s.Shields.Profession, ConfigVersion: s.Catalog.Source.SaveIdentity(), State: raw}
}
