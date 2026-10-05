package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Exercise the real CMD19 dispatcher, transaction, encoder and output. The
// previous condition/helper tests could not detect its missing final restore.
func cloneTownClient(t *testing.T) (*gameConnection, *recordingConnection, *[]map[string]any, inventory.Bag) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "storage.sqlite")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	version := strings.Repeat("a", 64)
	defs := inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Rows: []inventory.EquipmentDefinition{
		{ID: 506560010, Path: "hair-clone.equ", SHA256: version, Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[hair avatar]"}}, "[item category]": {{Type: 6, Text: "clear avatar"}},
		}},
		{ID: 506520010, Path: "breast-clone.equ", SHA256: version, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[breast avatar]"}, {Type: 0, Value: 0}}, "[item category]": {{Type: 6, Text: "clear avatar"}}}},
		{ID: 506522889, Path: "breast-look.equ", SHA256: version, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[breast avatar]"}, {Type: 0, Value: 0}}, "[usable job]": {{Type: 6, Text: "[all]"}}}},
	}}
	path := filepath.Join(t.TempDir(), "equipment.json")
	data, err := json.Marshal(defs)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))
	equipment, err := inventory.LoadEquipmentCatalog(path, version)
	require.NoError(t, err)
	bag := inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{{Slot: 1, Template: 506560010}}}
	for _, slot := range []uint16{12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47} {
		record := make([]byte, protocol.CurrentItemRecordSize)
		binary.LittleEndian.PutUint32(record[2:], 100000000+uint32(slot))
		record[85] = 1 // Preserve instance metadata, including the sort lock.
		bag.Worn = append(bag.Worn, inventory.BagEquipment{Slot: slot, Template: 100000000 + uint32(slot), Record: record, Durability: 40, Period: 123})
	}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":115,"advancement":0,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"unknown_future_field":{"keep":true}}`), bag)
	require.NoError(t, err)
	account, err := store.DevelopmentAccount(ctx, "clone-town-sync")
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "clone-town-sync", Profession: 0, ConfigVersion: equipment.Source.SaveIdentity(), Request: []byte{}, State: state}, 24)
	require.NoError(t, err)
	role.WireID = 14
	client, conn, events := newDispatchTestClient()
	client.worldState.role = role
	client.characters = &character.Service{DetailedWornCandidate: true}
	client.worldState.characters = client.characters
	client.wearService = &workflow.WearService{Store: store, WearService: inventory.WearService{
		Catalog: equipment, Rules: inventory.WearRules{Source: version, Special: true, Slots: map[string]uint16{"[breast avatar]": 6}},
		Professions: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: version}, Professions: map[byte]catalog.Profession{0: {ID: 0, Job: "[mage]"}}},
	}}
	return client, conn, events, bag
}
func TestTownCloneRemovalRestoresEveryNonAvatarSlotAfterMode1(t *testing.T) {
	client, conn, events, bag := cloneTownClient(t)
	// Current-client vector: 2026-10-05 12:58:18.418, log line 525.
	p, err := hex.DecodeString("01000000000000010000000301000a7e311e00000000ffffffff000000000000")
	require.NoError(t, err)
	request := &clientRequest{frame: wire.Frame{Type: 1, ID: 19, Raw: p}, plaintext: p, verified: true}
	if got := client.dispatchEquipmentSkillsAndMoves(request); got != dispatchHandled {
		t.Fatalf("dispatch=%v", got)
	}
	if conn.Len() == 0 {
		t.Fatal("dispatcher sent no frames")
	}
	var mode1, restore int = -1, -1
	var body []byte
	for i, event := range *events {
		switch event["kind"] {
		case "equipment_avatar_addition_refreshed":
			mode1 = i
		case "equipment_nonavatar_worn_restored":
			restore = i
			body, err = hex.DecodeString(event["plain_hex"].(string))
			require.NoError(t, err)
		case "equipment_move_refused", "equipment_encode_error", "equipment_avatar_addition_error":
			t.Fatalf("rejected: %v", event)
		}
	}
	if mode1 < 0 || restore <= mode1 {
		t.Fatalf("mode1=%d restore=%d; omitted slots would stay cleared", mode1, restore)
	}
	want, err := inventory.EquipmentPayload(3, bag.Worn[1:], false)
	require.NoError(t, err)
	if !bytes.Equal(body, want) {
		t.Fatal("final restore lost an ordinary/Oath slot or instance fields")
	}
	saved, err := inventory.ReadBag(client.worldState.role.State)
	require.NoError(t, err)
	if !reflect.DeepEqual(saved.Worn, bag.Worn[1:]) {
		t.Fatal("ordinary/Oath saved instances changed")
	}
	if len(saved.Special[1]) != 1 || saved.Special[1][0].Template != 506560010 {
		t.Fatal("removed Clone was not retained in avatar inventory")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(client.worldState.role.State, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["unknown_future_field"]) != `{"keep":true}` {
		t.Fatal("unknown character state was lost")
	}
}

func TestNativeCloneBindingAndRevocationReachRealDispatcher(t *testing.T) {
	client, _, events, bag := cloneTownClient(t)
	bag.Worn[0].Slot, bag.Worn[0].Template = 6, 506520010
	bag.Special = map[byte][]inventory.BagEquipment{1: {{Slot: 9, Template: 506522889, Period: 4321, AvatarOptions: []byte{1, 2}, AvatarSockets: []byte{3, 4}}}}
	ctx := context.Background()
	role, _, err := client.wearService.Store.CommitCharacterEvent(ctx, client.worldState.role.AccountID, client.worldState.role.ID, client.worldState.role.ConfigVersion, "native-clone-fixture", "test-clone-fixture", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		raw, err := inventory.SaveBag(current.State, bag)
		return raw, json.RawMessage(`{}`), err
	})
	require.NoError(t, err)
	role.WireID = 14
	client.worldState.role = role
	body, _ := hex.DecodeString("01090009ed301e01000000030600cae1301e00000000ffffffff000000000000")
	invoke := func(body []byte) {
		t.Helper()
		if action := client.dispatchEquipmentSkillsAndMoves(&clientRequest{frame: wire.Frame{Type: 1, ID: 19, Raw: body}, plaintext: body, verified: true}); action != dispatchHandled {
			t.Fatalf("action: %v", action)
		}
	}
	invoke(body)
	bound, err := inventory.ReadBag(client.worldState.role.State)
	require.NoError(t, err)
	if len(bound.Worn) != len(bag.Worn) || bound.CloneAvatarLook(bound.Worn[0]) != 506522889 || !reflect.DeepEqual(bound.Special[1], bag.Special[1]) {
		t.Fatal("real binding dispatcher consumed source or replaced Clone")
	}
	*events = nil
	// Derived from 145ADF2B0's current writer (not a captured native vector):
	// occupied source slot with identity0, target copied-template, flag1.
	unbind := append([]byte(nil), body...)
	binary.LittleEndian.PutUint32(unbind[3:], 0)
	binary.LittleEndian.PutUint32(unbind[14:], 506522889)
	unbind[28] = 1
	invoke(unbind)
	var ackIndex, sourceIndex int = -1, -1
	for i, event := range *events {
		if event["kind"] == "equipment_move_committed" {
			ackIndex = i
			ack, _ := hex.DecodeString(event["plain_hex"].(string))
			if len(ack) != 12 || ack[11] != 1 {
				t.Fatal("revocation did not use ACK mode1")
			}
		}
		if event["kind"] == "clone_avatar_sources_replaced" && sourceIndex == -1 {
			sourceIndex = i
		}
		if event["kind"] == "equipment_move_refused" {
			t.Fatalf("unbind refused: %v", event)
		}
	}
	if ackIndex < 0 || sourceIndex < 0 || sourceIndex >= ackIndex {
		t.Fatal("revocation must clear source relationship before its native ACK")
	}
	cleared, err := inventory.ReadBag(client.worldState.role.State)
	require.NoError(t, err)
	if len(cleared.Worn) != len(bound.Worn) || cleared.Worn[0].CloneSource != nil || !reflect.DeepEqual(cleared.Special, bound.Special) {
		t.Fatal("revocation physically moved or rewrote items")
	}
}
