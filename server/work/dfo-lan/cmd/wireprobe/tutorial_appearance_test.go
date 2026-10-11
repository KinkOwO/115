package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exercise the actual owned tutorial route with a temporary database. The
// historical priest dungeon is a regression fixture, not a runtime route table.
func TestTutorialEntryRestoresOwnedAppearanceBeforeMap(t *testing.T) {
	professions, err := catalog.LoadCharacters("../../configs/characters.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	dungeons, err := catalog.LoadDungeons("../../configs/tutorial-dungeons.current36.json")
	if err != nil {
		t.Fatal(err)
	}
	var profession byte
	for id, p := range professions.Professions {
		if p.Job == "[priest]" {
			profession = id
			break
		}
	}
	p := professions.Professions[profession]
	if p.Job != "[priest]" {
		t.Fatal("priest fixture absent")
	}
	for _, tc := range []struct {
		name  string
		actor uint16
		worn  []inventory.BagEquipment
	}{
		{"local_worn", 1, []inventory.BagEquipment{{Slot: 3, Template: 40601}, {Slot: 14, Template: 400070177}}},
		{"cloud_worn", 503, []inventory.BagEquipment{{Slot: 3, Template: 40601}, {Slot: 14, Template: 400070177}}},
		{"cloud_empty", 504, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "tutorial.sqlite3")})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			account, err := store.DevelopmentAccount(ctx, "tutorial-appearance")
			if err != nil {
				t.Fatal(err)
			}
			state, err := json.Marshal(character.State{Level: 1, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills, SourceSHA256: p.RawSHA256})
			if err != nil {
				t.Fatal(err)
			}
			state, err = inventory.SaveBag(state, inventory.Bag{Version: "ordinary-bag-v1", Worn: tc.worn})
			if err != nil {
				t.Fatal(err)
			}
			stored, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "TutorialLook", Profession: profession, State: state, Request: []byte{0}, ConfigVersion: savecontract.Identity()}, 8)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.StartBirth(ctx, account, stored.ID); err != nil {
				t.Fatal(err)
			}
			role := stored
			role.WireID = tc.actor
			w := &worldSession{role: role, account: account, store: store, service: &world.Service{}, characters: &character.Service{Catalog: professions}, professions: professions, dungeons: &dungeons, tutorialDungeons: &dungeons, tutorials: &catalog.TutorialCatalog{Flows: []catalog.TutorialFlow{{Job: p.Job, Dungeon: 7113}}}}
			session, plan, err := w.selectTutorial(7113)
			if err != nil {
				t.Fatal(err)
			}
			if session.Definition.ID != 7113 || !w.inTutorial {
				t.Fatal("tutorial route changed")
			}
			if len(plan) < 5 || plan[0].Kind != 1 || plan[0].ID != 16 || plan[len(plan)-2].ID != 28 || plan[len(plan)-1].ID != 29 {
				t.Fatalf("appearance must precede map: %+v", plan)
			}
			var basic, addition, worn []byte
			for _, packet := range plan {
				switch packet.Name {
				case "dungeon_actor_appearance_sent":
					basic = packet.Payload
				case "dungeon_actor_addition_sent":
					addition = packet.Payload
				case "dungeon_worn_visuals_sent":
					worn = packet.Payload
				}
				if packet.ID == 9 || packet.ID == 27 {
					t.Fatal("ordinary party/select UI leaked into tutorial")
				}
			}
			// USERINFO0: u8 mode + u16 count + 2 context bytes + 160 header.
			if len(basic) < 177+len(role.Name) || binary.LittleEndian.Uint16(basic[165:167]) != tc.actor {
				t.Fatalf("appearance does not address this session actor: %x", basic)
			}
			if count := int(basic[176+len(role.Name)]); count != len(tc.worn) {
				t.Fatalf("worn appearance rows=%d want=%d", count, len(tc.worn))
			}
			wantAddition, err := w.characters.EntryAddition(role)
			if err != nil || !bytes.Equal(addition, wantAddition) {
				t.Fatalf("authoritative actor details missing: %v", err)
			}
			wantWorn, err := inventory.WornSpaceUpdate(role.State)
			if err != nil || !bytes.Equal(worn, wantWorn) {
				t.Fatalf("authoritative worn visuals missing: %v", err)
			}
			after, err := store.Characters(ctx, account)
			if err != nil || len(after) != 1 || after[0].WireID != stored.WireID || !bytes.Equal(after[0].State, stored.State) {
				t.Fatalf("tutorial appearance rewrote saved identity/inventory: %v", err)
			}
			stage, route, err := store.BirthStage(ctx, account, stored.ID)
			if err != nil || stage != database.BirthEntered || route != 7113 {
				t.Fatalf("birth progress changed: %d %d %v", stage, route, err)
			}
		})
	}
}
