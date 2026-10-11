package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func slotTicketFixture(t *testing.T, count, base, amount, limit int) (*ItemService, database.Character, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "slots.sqlite3")
	store, err := database.Open(context.Background(), database.Config{Driver: database.DriverSQLite, SQLitePath: path, MaxConnections: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	account, err := store.DevelopmentAccount(context.Background(), "slot-owner")
	if err != nil {
		t.Fatal(err)
	}
	c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}, Items: map[uint32]catalog.LootItem{
		catalog.NativeCharacterSlotTicket: {ID: catalog.NativeCharacterSlotTicket, Kind: "stackable", Script: catalog.ScriptRecord{Cells: []pvf.Token{
			{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[town and dungeon]"}, {Type: 0}, {Type: 3, Text: "[int data]"}, {Type: 0, Value: int32(amount)}, {Type: 0, Value: int32(limit)},
		}}},
	}}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":1,"advancement":0,"opaque":{"future":17}}`), inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: catalog.NativeCharacterSlotTicket, Amount: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	var role database.Character
	for n := 0; n < count; n++ {
		created, e := store.CreateCharacter(context.Background(), database.Character{AccountID: account, Name: fmt.Sprintf("role%d", n), Request: []byte{1}, ConfigVersion: c.Source.SaveIdentity(), State: state}, base)
		if e != nil {
			t.Fatal(e)
		}
		if n == 0 {
			role = created
		}
	}
	return &ItemService{Store: store, Items: &inventory.ItemService{Catalog: c, Model: "slot-ticket-test-v1"}}, role, path
}

func TestCharacterSlotTicketAtomicReplayReopenAndCreate(t *testing.T) {
	ctx := context.Background()
	s, role, path := slotTicketFixture(t, 2, 2, 2, 5)
	r := protocol.UseStackableRequest{Slot: 65, Template: catalog.NativeCharacterSlotTicket, Instance: 123}
	saved, receipt, applied, err := s.ConsumeCharacterSlotTicket(ctx, role, r, 2)
	if err != nil || !applied || receipt.CharacterCapacity != 4 || receipt.Remaining != 1 {
		t.Fatalf("use: %+v %v %v", receipt, applied, err)
	}
	if !strings.Contains(string(saved.State), `"future":17`) {
		t.Fatal("unrelated save state lost")
	}
	_, receipt, applied, err = s.ConsumeCharacterSlotTicket(ctx, role, r, 2)
	if err != nil || applied || receipt.CharacterCapacity != 4 || receipt.Remaining != 1 {
		t.Fatalf("duplicate: %+v %v %v", receipt, applied, err)
	}
	second, e := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: path})
	if e != nil {
		t.Fatal(e)
	}
	defer second.Close()
	capacity, e := second.AccountCharacterCapacity(ctx, role.AccountID, 2)
	if e != nil || capacity != 4 {
		t.Fatalf("reopen capacity=%d %v", capacity, e)
	}
	cs := &character.Service{Store: second, Rules: character.Rules{MaxCharacters: 2}}
	list, e := cs.ListWithFatigue(ctx, role.AccountID, nil, time.Time{})
	if e != nil || len(list) < 5 || binary.LittleEndian.Uint16(list[3:]) != 4 {
		t.Fatalf("list capacity %x %v", list, e)
	}
	for n := 2; n < 4; n++ {
		_, e = second.CreateCharacter(ctx, database.Character{AccountID: role.AccountID, Name: fmt.Sprintf("new%d", n), Request: []byte{1}, ConfigVersion: role.ConfigVersion, State: role.State}, 2)
		if e != nil {
			t.Fatal(e)
		}
	}
	if _, e = second.CreateCharacter(ctx, database.Character{AccountID: role.AccountID, Name: "full", Request: []byte{1}, ConfigVersion: role.ConfigVersion, State: role.State}, 2); e == nil {
		t.Fatal("creation exceeded purchased capacity")
	}
	other, e := second.DevelopmentAccount(ctx, "other-account")
	if e != nil {
		t.Fatal(e)
	}
	capacity, e = second.AccountCharacterCapacity(ctx, other, 2)
	if e != nil || capacity != 2 {
		t.Fatal("capacity leaked across accounts")
	}
}

func TestCharacterSlotTicketRefusalKeepsCouponAndCapacity(t *testing.T) {
	for _, tc := range []struct {
		name               string
		count, base, limit int
		slot               uint16
	}{{"free-slot", 1, 2, 5, 65}, {"limit", 2, 2, 2, 65}, {"wrong-slot", 2, 2, 5, 66}} {
		t.Run(tc.name, func(t *testing.T) {
			s, role, _ := slotTicketFixture(t, tc.count, tc.base, 1, tc.limit)
			_, _, _, err := s.ConsumeCharacterSlotTicket(context.Background(), role, protocol.UseStackableRequest{Slot: tc.slot, Template: catalog.NativeCharacterSlotTicket, Instance: 123}, tc.base)
			if err == nil {
				t.Fatal("invalid use accepted")
			}
			roles, e := s.Store.Characters(context.Background(), role.AccountID)
			if e != nil || string(roles[0].State) != string(role.State) {
				t.Fatal("refusal changed coupon or state", e)
			}
			capacity, e := s.Store.AccountCharacterCapacity(context.Background(), role.AccountID, tc.base)
			if e != nil || capacity != tc.base {
				t.Fatal("refusal changed capacity", e)
			}
		})
	}
}

func TestCharacterSlotTicketConcurrentCharactersExpandOnce(t *testing.T) {
	s, role, _ := slotTicketFixture(t, 2, 2, 1, 5)
	roles, err := s.Store.Characters(context.Background(), role.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for _, r := range roles {
		wg.Add(1)
		go func(r database.Character) {
			defer wg.Done()
			_, _, applied, e := s.ConsumeCharacterSlotTicket(context.Background(), r, protocol.UseStackableRequest{Slot: 65, Template: catalog.NativeCharacterSlotTicket, Instance: uint32(r.ID)}, 2)
			results <- applied && e == nil
		}(r)
	}
	wg.Wait()
	close(results)
	success := 0
	for applied := range results {
		if applied {
			success++
		}
	}
	capacity, e := s.Store.AccountCharacterCapacity(context.Background(), role.AccountID, 2)
	if success != 1 || e != nil || capacity != 3 {
		t.Fatalf("concurrency: successful=%d capacity=%d %v", success, capacity, e)
	}
	roles, err = s.Store.Characters(context.Background(), role.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	remaining := uint32(0)
	for _, r := range roles {
		b, e := inventory.ReadBag(r.State)
		if e != nil {
			t.Fatal(e)
		}
		remaining += b.Items[0].Amount
	}
	if remaining != 3 {
		t.Fatalf("spent %d coupons", 4-remaining)
	}
}
