package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestCharacterSlotTicketGatewayRefresh(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "slots.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	account, err := db.DevelopmentAccount(ctx, "coupon-owner")
	if err != nil {
		t.Fatal(err)
	}
	c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}, Items: map[uint32]catalog.LootItem{catalog.NativeCharacterSlotTicket: {Kind: "stackable", Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[town and dungeon]"}, {Type: 0}, {Type: 3, Text: "[int data]"}, {Type: 0, Value: 1}, {Type: 0, Value: 100}}}}}}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":1,"advancement":0}`), inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: catalog.NativeCharacterSlotTicket, Amount: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	var role database.Character
	for n := 0; n < 2; n++ {
		r, e := db.CreateCharacter(ctx, database.Character{AccountID: account, Name: fmt.Sprintf("role%d", n), Request: []byte{1}, ConfigVersion: c.Source.SaveIdentity(), State: state}, 2)
		if e != nil {
			t.Fatal(e)
		}
		if n == 0 {
			role = r
		}
	}
	w := &worldSession{role: role, account: account, store: db, items: &inventory.ItemService{Catalog: c, Model: "slot-test-v1"}, characters: &character.Service{Store: db, Rules: character.Rules{MaxCharacters: 2}}}
	body := make([]byte, 16)
	binary.LittleEndian.PutUint16(body, 65)
	binary.LittleEndian.PutUint32(body[3:], 123)
	binary.LittleEndian.PutUint32(body[7:], catalog.NativeCharacterSlotTicket)
	events := []map[string]any{}
	plan, err := w.useStackable(body, func(event map[string]any) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 3 || plan[0].ID != 44 || plan[1].ID != 14 || plan[2].ID != 66 || !bytes.Equal(plan[2].Payload, []byte{8, 0, 3}) {
		t.Fatalf("use plan: %+v", plan)
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || len(bag.Items) != 0 {
		t.Fatal("coupon was not spent", err)
	}
	if len(events) != 2 || events[1]["kind"] != "character_slot_expanded" {
		t.Fatalf("audit events: %+v", events)
	}
}
