package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
)

func assertSortSuccessAndRestore(t *testing.T, plan []outboundPacket, space byte) {
	t.Helper()
	// The current PE's CMD20 consumer reads a length-prefixed RETURN whose
	// field 1 selects the container. Keep this independent of the encoder.
	wantACK := []byte{1, 2, 0, 0, 0, 8, space}
	if len(plan) != 2 || plan[0].Kind != 1 || plan[0].ID != 20 || !bytes.Equal(plan[0].Payload, wantACK) {
		t.Fatalf("missing container %d ACK before restore: %+v", space, plan)
	}
	if plan[1].Kind != 0 || plan[1].ID != 13 || len(plan[1].Payload) == 0 || plan[1].Payload[0] != space {
		t.Fatalf("restored wrong container: %+v", plan[1])
	}
	keys := make([]byte, wire.SessionKeyBytes)
	prepared, err := preparePackets(keys, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 2 || prepared[0].Kind != 1 || prepared[0].ID != 20 || prepared[1].Kind != 0 || prepared[1].ID != 13 {
		t.Fatalf("preparation dropped or reordered ACK/restore: %+v", prepared)
	}
	for _, packet := range prepared {
		if err = wire.ValidateServer(packet.Raw); err != nil {
			t.Fatal(err)
		}
	}
	body, err := wire.DecryptPayload(keys, 20, prepared[0].Raw[wire.ServerHeaderSize:])
	if err != nil {
		t.Fatal(err)
	}
	if len(body) < len(wantACK) || !bytes.Equal(body[:len(wantACK)], wantACK) || !bytes.Equal(body[len(wantACK):], make([]byte, len(body)-len(wantACK))) {
		t.Fatalf("encrypted ACK lost its length-prefixed container: %x", body)
	}
}

func TestSortOrdinaryCapturePersistsInstancesAndReplaysRestore(t *testing.T) {
	// Reuse a real captured ordinary request instead of manufacturing the
	// permutation under test. It moves old slots 18/19 to slots 21/22.
	capture, err := os.ReadFile("../../internal/game/protocol/testdata/native_sort_item20.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtureRequest struct {
		PayloadHex string `json:"payload_hex"`
	}
	if err = json.Unmarshal(capture, &fixtureRequest); err != nil {
		t.Fatal(err)
	}
	request, err := hex.DecodeString(fixtureRequest.PayloadHex)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})
	store := fixture.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "sort-ordinary-capture")
	if err != nil {
		t.Fatal(err)
	}
	record := protocol.OrdinaryItem(18, 40601, 1)
	record[10], record[14], record[20] = 12, 9, 5
	seed := inventory.Bag{
		Version: "ordinary-bag-v1", Gold: 12345, Coin: 7, Expansion: 1,
		Items: []inventory.BagItem{{Slot: 3, Template: 60001, Amount: 9, ExpireTime: 13579}},
		Equipment: []inventory.BagEquipment{
			{Slot: 18, Template: 40601, Durability: 77, Record: record[:], Period: 12345, Refine: 6,
				ExtraFields: map[string]json.RawMessage{"future_instance": json.RawMessage(`{"keep":19}`)}},
			{Slot: 19, Template: 40602, Durability: 55},
		},
		Special: map[byte][]inventory.BagEquipment{1: {{Slot: 2, Template: 40603, Period: 24680}}},
	}
	raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"future_field":{"keep":true},"inventory":{"future_inventory":{"keep":23}}}`), seed)
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{
		AccountID: account, Name: "sortcapture", Profession: 0, Request: []byte{0},
		ConfigVersion: savecontract.Identity(), State: raw,
	}, 24)
	if err != nil {
		t.Fatal(err)
	}
	want, err := inventory.ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	want.Equipment[0].Slot, want.Equipment[1].Slot = 21, 22
	reader, err := fixture.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reader.Close)
	w := &worldSession{account: account, role: role, store: store}
	s := &sortSession{}
	wear := &workflow.WearService{WearService: inventory.WearService{BagRules: inventory.BagRules{
		QuickSlots: [2]uint16{0, 8}, EquipmentSlots: [2]uint16{9, 64},
		Slots: map[string][2]uint16{"[throw]": {65, 120}, "[material]": {121, 176}},
	}}}
	var previous json.RawMessage
	for attempt := 0; attempt < 2; attempt++ {
		plan, err := s.handle(wear, w, request, request)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		assertSortSuccessAndRestore(t, plan, 0)
		saved, err := reader.AdminCharacter(ctx, role.ID)
		if err != nil {
			t.Fatal(err)
		}
		got, err := inventory.ReadBag(saved.State)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("sort changed instances or unrelated containers: got=%+v want=%+v", got, want)
		}
		if !bytes.Contains(saved.State, []byte(`"future_field":{"keep":true}`)) || !bytes.Contains(saved.State, []byte(`"future_inventory":{"keep":23}`)) {
			t.Fatal("sort lost unrelated persisted fields")
		}
		if attempt > 0 && !bytes.Equal(saved.State, previous) {
			t.Fatal("replayed capture changed the persisted arrangement")
		}
		previous = append(json.RawMessage(nil), saved.State...)
	}
}

// This regression protects the already implemented vault transaction and
// restore behavior. The native NOTI13 consumer clears the vault's pending
// operations, so this test deliberately does not require an extra CMD20 ACK.
func TestSortVaultPersistsInstancesAndRestoresContainer(t *testing.T) {
	for _, space := range []byte{2, 45, 12} {
		t.Run(fmt.Sprint(space), func(t *testing.T) {
			ctx := context.Background()
			fixture, err := database.OpenTestFixture(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := fixture.Close(); err != nil {
					t.Error(err)
				}
			})
			store := fixture.Storage()
			if err = store.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			account, err := store.DevelopmentAccount(ctx, fmt.Sprintf("sort-vault-%d", space))
			if err != nil {
				t.Fatal(err)
			}
			version := strings.Repeat("b", 64)
			originalRoleState := json.RawMessage(`{"level":115,"inventory":{"version":"ordinary-bag-v1","gold":12345},"future_field":{"keep":true},"symbol":{"private":7}}`)
			role, err := store.CreateCharacter(ctx, database.Character{
				AccountID: account, Name: fmt.Sprintf("vault%d", space), Profession: 0,
				ConfigVersion: version, Request: []byte{0}, State: originalRoleState,
			}, 24)
			if err != nil {
				t.Fatal(err)
			}
			record := protocol.OrdinaryItem(6, 40001, 0)
			record[10], record[14], record[20] = 12, 9, 5
			equipment := inventory.BagEquipment{
				Slot: 6, Template: 40001, Durability: 77, Record: record[:],
				Period: 12345, Refine: 6,
				ExtraFields: map[string]json.RawMessage{"future_instance": json.RawMessage(`{"keep":19}`)},
			}
			items := []inventory.VaultItem{
				{Slot: 7, Template: 60001, Amount: 9, ExpireTime: 24680},
				{Slot: 6, Template: 40001, IsEquip: true, Equipment: &equipment},
				{Slot: 3, Template: 50001, Amount: 4, ExpireTime: 13579},
			}
			seed, err := json.Marshal(items)
			if err != nil {
				t.Fatal(err)
			}
			rules := inventory.VaultRules{
				SourceSHA256: version, InitialSlots: 8, InitialSecondarySlots: 8,
				VerifiedSlots: []uint16{8},
				Account:       &inventory.AccountVaultRules{RequiredLevel: 1, Upgrades: [][6]int64{{8, 100000}}},
			}
			if space == 12 {
				_, _, _, _, err = store.CommitAccountVault(ctx, account, role.ID, version, "sort-test-seed", 305,
					func(current database.Character, materials json.RawMessage, _ database.AccountVaultState) (json.RawMessage, json.RawMessage, database.AccountVaultState, error) {
						return current.State, materials, database.AccountVaultState{Slots: 8, Gold: 321, Items: seed}, nil
					})
			} else {
				if _, err = store.LoadVault(ctx, account, role.ID, 8, version, space); err != nil {
					t.Fatal(err)
				}
				_, _, err = store.CommitVaultMove(ctx, account, role.ID,
					func(current database.Character, _ database.VaultState) (json.RawMessage, json.RawMessage, error) {
						return current.State, seed, nil
					}, space)
			}
			if err != nil {
				t.Fatal(err)
			}
			w := &worldSession{store: store, account: account, role: role,
				vault: &workflow.VaultService{Store: store, VaultService: inventory.VaultService{Rules: rules}}}
			// Read from a separate connection to the same private SQLite file,
			// so assertions never rely on the in-memory return from the sorter.
			reader, err := fixture.Reopen(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(reader.Close)
			wantItems := []inventory.VaultItem{items[1], items[2], items[0]}
			for i := range wantItems {
				wantItems[i].Slot = uint16(i)
			}
			var previousItems json.RawMessage
			for attempt := 0; attempt < 2; attempt++ {
				var plan []outboundPacket
				if space == 12 {
					plan, err = w.sortAccountVaultCmd()
				} else {
					plan, err = w.sortVaultSpace(space)
				}
				if err != nil {
					t.Fatalf("attempt %d: %v", attempt, err)
				}
				var savedItems json.RawMessage
				if space == 12 {
					saved, e := reader.LoadAccountVault(ctx, account, role.ID)
					if e != nil {
						t.Fatal(e)
					}
					if saved.Slots != 8 || saved.Gold != 321 {
						t.Fatalf("sort changed account vault capacity/gold: %+v", saved)
					}
					savedItems = saved.Items
				} else {
					saved, e := reader.LoadVault(ctx, account, role.ID, 8, version, space)
					if e != nil {
						t.Fatal(e)
					}
					if saved.Slots != 8 || saved.ConfigVersion != version {
						t.Fatalf("sort changed personal vault identity: %+v", saved)
					}
					savedItems = saved.Items
				}
				var gotItems []inventory.VaultItem
				if err = json.Unmarshal(savedItems, &gotItems); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(gotItems, wantItems) {
					t.Fatalf("sort changed instance fields or arrangement: got=%+v want=%+v", gotItems, wantItems)
				}
				if attempt > 0 && !bytes.Equal(savedItems, previousItems) {
					t.Fatal("repeated sort changed the persisted arrangement")
				}
				previousItems = append(json.RawMessage(nil), savedItems...)
				savedRole, e := reader.AdminCharacter(ctx, role.ID)
				if e != nil {
					t.Fatal(e)
				}
				if !bytes.Equal(savedRole.State, originalRoleState) {
					t.Fatal("vault sort changed unrelated character state")
				}
				prepared, e := preparePackets(make([]byte, wire.SessionKeyBytes), plan)
				if e != nil {
					t.Fatal(e)
				}
				restores := 0
				for _, packet := range prepared {
					if packet.Kind != 0 || packet.ID != 13 {
						continue
					}
					restores++
					if len(packet.Payload) == 0 || packet.Payload[0] != space {
						t.Fatalf("restored wrong container: %x", packet.Payload)
					}
					if e := wire.ValidateServer(packet.Raw); e != nil {
						t.Fatal(e)
					}
				}
				if restores != 1 {
					t.Fatalf("prepared %d NOTI13 restores, want one", restores)
				}
			}
		})
	}
}
