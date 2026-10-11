//go:build sqlite

package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func skinSlotFixture(t *testing.T) *worldSession {
	t.Helper()
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "skin.sqlite")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateGrants, store.MigratePremiums} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "skin-fixture")
	if err != nil {
		t.Fatal(err)
	}
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":84,"source_sha256":"`+source.Checksum+`","attributes":{"[hp max]":100,"[mp max]":50},"opaque":{"keep":17}}`), inventory.Bag{Version: "ordinary-bag-v1", ExpandEquipFlags: 19, Items: []inventory.BagItem{{Slot: 65, Template: inventory.AuraSkinTicket, Amount: 2}, {Slot: 66, Template: inventory.AuraSkinLicense, Amount: 1}, {Slot: 67, Template: inventory.CreatureSkinTicket, Amount: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "SkinFixture", ConfigVersion: source.SaveIdentity(), State: state, Request: []byte{0, 255}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 3
	professions, err := catalog.LoadCharacters("../../configs/characters.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	items := map[uint32]catalog.LootItem{}
	for _, id := range []uint32{inventory.AuraSkinTicket, inventory.AuraSkinLicense, inventory.CreatureSkinTicket} {
		action := "[open aura skin slot]"
		if id == inventory.CreatureSkinTicket {
			action = "[open creature skin slot]"
		}
		items[id] = catalog.LootItem{ID: id, Kind: "stackable", StackableType: "[etc]", StackLimit: 99999,
			Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[action type]"}, {Type: 6, Text: action}, {Type: 3, Text: "[/action type]"}}}}
	}
	return &worldSession{store: store, role: role, characters: &character.Service{Store: store, Catalog: professions}, items: &inventory.ItemService{Model: "skin-fixture", Catalog: catalog.LootCatalog{Source: source, Items: items}}}
}

func assertAuraSlot(t *testing.T, w *worldSession, plan []outboundPacket) {
	t.Helper()
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || bag.ExpandEquipFlags != 19|inventory.ExpandAuraSkin {
		t.Fatalf("unlock bits: %v %v", bag.ExpandEquipFlags, err)
	}
	if !strings.Contains(string(w.role.State), `"keep":17`) {
		t.Fatal("unrelated save state lost")
	}
	var refresh bool
	for _, p := range plan {
		if p.Name == "reward_actor_addition_sent" && len(p.Payload) > 360 && p.Payload[360] == bag.ExpandEquipFlags {
			refresh = true
		}
	}
	if !refresh {
		t.Fatal("native unlock byte not refreshed")
	}
}

func TestAuraLicenseOrdinaryUseSQLiteUnlockAndReplay(t *testing.T) {
	w := skinSlotFixture(t)
	body := make([]byte, 16)
	binary.LittleEndian.PutUint16(body, 66)
	binary.LittleEndian.PutUint32(body[3:], 17)
	binary.LittleEndian.PutUint32(body[7:], inventory.AuraSkinLicense)
	plan, err := w.useStackable(body, func(map[string]any) {})
	if err != nil {
		t.Fatal(err)
	}
	assertAuraSlot(t, w, plan)
	state := string(w.role.State)
	plan, err = w.useStackable(body, func(map[string]any) {})
	if err != nil {
		t.Fatal(err)
	}
	assertAuraSlot(t, w, plan)
	if string(w.role.State) != state {
		t.Fatal("retry spent another ticket")
	}
	// Clicking the padlock after the ticket is spent still answers and refreshes.
	plan, err = w.openSkinSlot([]byte{66, 0, 11, 0, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	assertAuraSlot(t, w, plan)
	if p := plan[len(plan)-1]; p.ID != 857 || string(p.Payload) != string([]byte{1, 11, 0}) {
		t.Fatal("native confirmation answer missing")
	}
}

func TestAuraPadlockSQLiteConsumesRequestedTicketOnly(t *testing.T) {
	w := skinSlotFixture(t)
	plan, err := w.openSkinSlot([]byte{66, 0, 11, 0, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	assertAuraSlot(t, w, plan)
	bag, _ := inventory.ReadBag(w.role.State)
	if len(bag.Items) != 2 || bag.Items[0].Slot != 65 || bag.Items[0].Amount != 2 {
		t.Fatal("spent a different ticket", bag.Items)
	}
	state := string(w.role.State)
	if _, err = w.openSkinSlot([]byte{66, 0, 11, 0}); err != nil || string(w.role.State) != state {
		t.Fatal("confirmation retry changed state", err)
	}
	// A second refresh from a vanished action ticket must not charge again.
	body := make([]byte, 64)
	binary.LittleEndian.PutUint16(body, 66)
	binary.LittleEndian.PutUint32(body[7:], 101)
	plan, err = w.stackableAction(body)
	if err != nil {
		t.Fatal(err)
	}
	assertAuraSlot(t, w, plan)
	if string(w.role.State) != state {
		t.Fatal("action retry changed state")
	}
}

func TestSkinSlotWrongTicketAndContainerSQLiteRollback(t *testing.T) {
	for _, test := range []string{"wrong-window", "wrong-action", "wrong-container", "missing-slot"} {
		t.Run(test, func(t *testing.T) {
			w := skinSlotFixture(t)
			before := string(w.role.State)
			var err error
			switch test {
			case "wrong-window":
				_, err = w.openSkinSlot([]byte{67, 0, 11, 0})
			case "missing-slot":
				_, err = w.openSkinSlot([]byte{99, 0, 11, 0})
			default:
				body := make([]byte, 64)
				binary.LittleEndian.PutUint16(body, 67)
				binary.LittleEndian.PutUint32(body[7:], 101)
				if test == "wrong-container" {
					body[2] = 7
					binary.LittleEndian.PutUint16(body, 65)
				}
				_, err = w.stackableAction(body)
			}
			if err == nil || string(w.role.State) != before {
				t.Fatal("invalid request consumed/unlocked", err)
			}
		})
	}
}

func TestAuraActionSQLiteUnlockAndReconnect(t *testing.T) {
	w := skinSlotFixture(t)
	body := make([]byte, 64)
	binary.LittleEndian.PutUint16(body, 65)
	binary.LittleEndian.PutUint32(body[7:], 101)
	plan, err := w.stackableAction(body)
	if err != nil {
		t.Fatal(err)
	}
	assertAuraSlot(t, w, plan)
	saved, _, err := w.characters.ExpandSkinSlot(context.Background(), w.role, "invalid-second-unlock", inventory.ExpandAuraSkin, w.items.Catalog, 65)
	if err == nil {
		t.Fatal("service spent another ticket for an open slot", saved)
	}
	roles, err := w.characters.Store.Characters(context.Background(), w.role.AccountID)
	if err != nil || len(roles) != 1 {
		t.Fatal("reload", err)
	}
	w.role = roles[0]
	w.role.WireID = 3
	plan, err = w.openSkinSlot([]byte{255, 255, 11, 0})
	if err != nil {
		t.Fatal(err)
	}
	assertAuraSlot(t, w, plan)
}
