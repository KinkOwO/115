package main

import (
	"bytes"
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"
)

func TestMergedSpecialSortPersistsInstanceAndReplaysRestore(t *testing.T) {
	ctx := context.Background()
	fixture, e := database.OpenTestFixture(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer fixture.Close()
	store := fixture.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if e = migrate(ctx); e != nil {
			t.Fatal(e)
		}
	}
	account, e := store.DevelopmentAccount(ctx, "sort-regression")
	if e != nil {
		t.Fatal(e)
	}
	for _, space := range []byte{1, 7} {
		t.Run(fmt.Sprint(space), func(t *testing.T) {
			record := protocol.OrdinaryItem(2, 40601, 1)
			record[10] = 12
			record[14] = 9
			item := inventory.BagEquipment{Slot: 2, Template: 40601, Record: record[:], Period: 12345}
			if space == 1 {
				item.AvatarOptions = []byte{4, 0, 0, 0, 0, 0}
			}
			raw, e := inventory.SaveBag(json.RawMessage(`{"level":1,"other":{"keep":7}}`), inventory.Bag{Version: "ordinary-bag-v1", Special: map[byte][]inventory.BagEquipment{space: {item}}})
			if e != nil {
				t.Fatal(e)
			}
			role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: fmt.Sprintf("sort%d", space), Profession: 0, Request: []byte{0}, ConfigVersion: savecontract.Identity(), State: raw}, 100)
			if e != nil {
				t.Fatal(e)
			}
			request := make([]byte, 8+2*432)
			request[0] = space
			request[1] = 1
			binary.LittleEndian.PutUint16(request[2:], 432)
			for i := 0; i < 432; i++ {
				n := i
				if i == 1 {
					n = 2
				}
				if i == 2 {
					n = 1
				}
				binary.LittleEndian.PutUint16(request[4+2*i:], uint16(n))
			}
			w := &worldSession{account: account, role: role, store: store}
			s := &sortSession{}
			wear := &workflow.WearService{}
			for attempt := 0; attempt < 2; attempt++ {
				plan, e := s.handle(wear, w, request, request)
				if e != nil {
					t.Fatal(e)
				}
				assertSortSuccessAndRestore(t, plan, space)
				b, e := inventory.ReadBag(w.role.State)
				if e != nil {
					t.Fatal(e)
				}
				got := b.Special[space]
				if len(got) != 1 || got[0].Slot != 1 || got[0].Period != item.Period || !bytes.Equal(got[0].Record, item.Record) || !bytes.Equal(got[0].AvatarOptions, item.AvatarOptions) {
					t.Fatalf("sort/replay changed instance: %+v", got)
				}
				if !bytes.Contains(w.role.State, []byte(`"keep":7`)) {
					t.Fatal("unrelated save lost")
				}
			}
			roles, e := store.Characters(ctx, account)
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, saved := range roles {
				if saved.ID == role.ID {
					found = true
					if !bytes.Equal(saved.State, w.role.State) {
						t.Fatal("not persisted")
					}
				}
			}
			if !found {
				t.Fatal("role lost")
			}
		})
	}
}
