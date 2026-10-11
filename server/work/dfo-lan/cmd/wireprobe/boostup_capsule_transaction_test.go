package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/testfixture"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestBoostCapsuleSQLiteProjectionDoesNotNestTransactions(t *testing.T) {
	for _, connections := range []int{4, 1} {
		t.Run(fmt.Sprintf("pool%d", connections), func(t *testing.T) {
			svc, db, role := awakeningSQLiteFixture(t, connections)
			ctx := context.Background()
			if err := db.MigrateWorld(ctx); err != nil {
				t.Fatal(err)
			}
			growth, err := catalog.LoadProgression(testfixture.ProgressionPath(t))
			if err != nil {
				t.Fatal(err)
			}
			growth.Source = svc.Catalog.Source
			var fields map[string]json.RawMessage
			json.Unmarshal(role.State, &fields)
			fields["level"] = json.RawMessage(`110`)
			fields["experience"], _ = json.Marshal(growth.Thresholds[108])
			raw, _ := json.Marshal(fields)
			raw, err = inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Gold: 12345, Items: []inventory.BagItem{{Slot: 65, Template: 590015870, Amount: 1}}})
			if err != nil {
				t.Fatal(err)
			}
			role, _, err = db.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "fixture:capsule", "fixture", func(database.Character) (json.RawMessage, json.RawMessage, error) {
				return raw, json.RawMessage(`{}`), nil
			})
			if err != nil {
				t.Fatal(err)
			}
			wc := boostTestWorldCatalog(svc.Catalog.Source.Checksum)
			wc.Source = svc.Catalog.Source
			area := wc.Areas["38/1"]
			area.SeriaReturnWarp = true
			wc.Areas["38/1"] = area
			w := &worldSession{account: role.AccountID, role: role, level: 110, characters: svc, store: db,
				progression: &character.ProgressionService{Store: db, Catalog: growth, Professions: svc.Catalog, Rules: character.GrowthRules{LevelCap: 115}},
				service:     &world.Service{Store: db, Catalog: wc},
				loot:        &loot.Service{Catalog: catalog.LootCatalog{Source: svc.Catalog.Source, Items: map[uint32]catalog.LootItem{590015870: {ID: 590015870, Kind: "stackable", StackableType: "[waste]", StackLimit: 1}}}},
				boostup:     &boostup.Catalog{Town: 222, GoalLevel: 115, UsableLevel: 115, Capsules: map[uint32]boostup.Capsule{590015870: {Item: 590015870}}, Steps: []boostup.Step{{Number: 1, Area: 0, Guide: "none", Mission: "equip item"}}},
				state:       database.WorldState{Position: database.WorldPosition{Town: 38, Area: 1, X: 200, Y: 200}},
			}
			request := make([]byte, 64)
			binary.LittleEndian.PutUint16(request, 65)
			binary.LittleEndian.PutUint32(request[7:], 337)
			started := time.Now()
			plan, err := w.useBoostCapsule(request)
			if err != nil {
				t.Fatalf("capsule refused after %s: %v", time.Since(started), err)
			}
			if time.Since(started) > 2*time.Second {
				t.Fatal("capsule still waits on nested database operation")
			}
			ack := false
			for _, packet := range plan {
				if packet.Name == "boost_capsule_actor" && binary.LittleEndian.Uint32(packet.Payload[5+0x2c:]) != 20078 {
					t.Fatal("selected party frame lost")
				}
				if packet.Name == "boost_capsule_ack" {
					ack = true
				}
			}
			if !ack || w.level != 115 || w.state.Position.Town != 222 {
				t.Fatal("capsule completion/scene missing")
			}
			saved, err := db.AdminCharacter(ctx, role.ID)
			if err != nil {
				t.Fatal(err)
			}
			bag, err := inventory.ReadBag(saved.State)
			if err != nil || len(bag.Items) != 0 || bag.Gold != 12345 {
				t.Fatal("capsule consumption/assets", err)
			}
			json.Unmarshal(saved.State, &fields)
			if string(fields["future_field"]) != `{"keep":true}` || string(fields["awakening"]) != "3" {
				t.Fatal("unrelated field/awakening changed")
			}
			before := string(saved.State)
			if _, err = w.useBoostCapsule(request); err == nil {
				t.Fatal("consumed capsule accepted as a new use")
			}
			saved, err = db.AdminCharacter(ctx, role.ID)
			if err != nil || string(saved.State) != before {
				t.Fatal("retry changed character", err)
			}
			if _, err = svc.WithEntryProjectionSnapshot(role); err != nil {
				t.Fatal("shared service was corrupted", err)
			}
		})
	}
}
