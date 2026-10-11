package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func oathSelectionFixture(t *testing.T) (*worldSession, *database.TestFixture) {
	t.Helper()
	ctx := context.Background()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	s := f.Storage()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "oath-selection-refresh")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("b", 64)
	eq, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Rows: []inventory.EquipmentDefinition{
		{ID: 100313757, Path: "gold.epic.equ", SHA256: version, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[oath]"}}}},
		{ID: 100313756, Path: "gold.representative.equ", SHA256: version, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[oath]"}}}},
		{ID: 100610079, Path: "gold.primeval.equ", SHA256: version, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[oath]"}}}},
		{ID: 100610060, Path: "valkyrie.equ", SHA256: version, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[oath]"}}}},
		{ID: 100401604, Path: "gold.crystal.equ", SHA256: version, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[primer]"}}}},
	}}, version)
	if err != nil {
		t.Fatal(err)
	}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":115,"future":{"keep":true}}`), inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{{Slot: 47, Template: 100313757}, {Slot: 36, Template: 100401604}}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, database.Character{AccountID: account, Name: "oath-refresh", Profession: 11, ConfigVersion: version, Request: []byte{0}, State: state}, 24)
	if err != nil {
		t.Fatal(err)
	}
	points := &catalog.PointRules{Oath: catalog.OathPointTable{Rules: []catalog.OathPointRule{
		{Template: 100313757, PartSetIndex: 16203, Value: 455},
		{Template: 100313756, PartSetIndex: 16203, Value: 265},
		{Template: 100610079, PartSetIndex: 16203, Value: 655},
		{Template: 100610060, PartSetIndex: 16209, Value: 265},
		{Template: 100401604, PartSetIndex: 16203, Value: 85},
	}}}
	return &worldSession{store: s, account: account, role: role, characters: &character.Service{}, items: &inventory.ItemService{Equipment: eq, Points: points}}, f
}

func oathSelectRequest(item uint32, option byte) []byte {
	body := make([]byte, 32)
	body[4] = 1 // Existing observed explicit-selection envelope.
	binary.LittleEndian.PutUint32(body[13:], item)
	body[17] = option
	return body
}

func TestOathRepresentativeSelectionPersistsActualCore(t *testing.T) {
	w, f := oathSelectionFixture(t)
	ctx := context.Background()
	before, err := f.CharacterState(ctx, w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range []byte{2, 3, 1, 2} {
		plan, err := w.oathSelectionPackets(ctx, oathSelectRequest(100610079, option))
		if err != nil {
			t.Fatal(err)
		}
		if len(plan) != 3 || plan[0].ID != 2382 || plan[1].ID != protocol.PartSetPointOpcode || plan[2].ID != 2839 || plan[2].Payload[0] != option {
			t.Fatalf("wrong selection refresh: %+v", plan)
		}
		selection, err := w.store.EquippedOathSelection(ctx, w.account, w.role.ID)
		if err != nil || selection.ItemID != 100313757 || selection.Option != int(option) {
			t.Fatalf("selection=%+v err=%v", selection, err)
		}
		if _, exists, err := w.store.OathOption(ctx, w.role.ID, "oath:100610079"); err != nil || exists {
			t.Fatal("representative must not get a separate save row", err)
		}
	}
	after, err := f.CharacterState(ctx, w.role.ID)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("selection rewrote character inventory/state", err)
	}
	for _, item := range []uint32{100610060, 100401604, 999} {
		if _, err := w.oathSelectionPackets(ctx, oathSelectRequest(item, 3)); err == nil {
			t.Fatalf("accepted unrelated/non-core %d", item)
		}
	}
	w.activeDungeon = &dungeon.Session{}
	if _, err := w.oathSelectionPackets(ctx, oathSelectRequest(100610079, 3)); err == nil {
		t.Fatal("dungeon selection accepted")
	}
}

func TestOathBootstrapRestoresDespiteStaleCachedItem(t *testing.T) {
	w, _ := oathSelectionFixture(t)
	ctx := context.Background()
	if _, err := w.store.SelectEquippedOathOption(ctx, w.account, w.role.ID, 100313757, 3); err != nil {
		t.Fatal(err)
	}
	// Actual client vector from the 2026-10-08 session: the UI's cached
	// Valkyrie representative remains after the player equips Golden Land.
	body, err := hex.DecodeString("a8adfb050000000000080000000c30ff05010000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := w.oathSelectionPackets(ctx, body)
	if err != nil || len(plan) != 2 || plan[0].ID != 2839 || plan[0].Payload[0] != 3 || plan[1].ID != 2382 {
		t.Fatalf("bootstrap=%+v err=%v", plan, err)
	}
	selection, err := w.store.EquippedOathSelection(ctx, w.account, w.role.ID)
	if err != nil || selection.Option != 3 {
		t.Fatal("bootstrap overwrote saved option", selection, err)
	}
}

func TestOathRefreshAfterCrystalPoints(t *testing.T) {
	w, _ := oathSelectionFixture(t)
	ctx := context.Background()
	if _, err := w.store.SelectEquippedOathOption(ctx, w.account, w.role.ID, 100313757, 2); err != nil {
		t.Fatal(err)
	}
	plan := w.primerTransformRefresh(ctx)
	worn, points, info := -1, -1, -1
	for i, p := range plan {
		if p.Name == "primer_transform_worn_refreshed" {
			worn = i
		}
		if p.ID == protocol.PartSetPointOpcode {
			points = i
			if binary.LittleEndian.Uint32(p.Payload[2:]) != 540 {
				t.Fatalf("points=%x", p.Payload)
			}
		}
		if p.ID == 2839 {
			info = i
			if p.Payload[0] != 2 {
				t.Fatal("lost saved option")
			}
		}
	}
	if worn < 0 || points <= worn || info <= points {
		t.Fatalf("required worn -> points -> display order, got %d %d %d", worn, points, info)
	}
}

func TestOathGroupRejectsAmbiguousSource(t *testing.T) {
	w, _ := oathSelectionFixture(t)
	w.items.Points.Oath.Rules = append(w.items.Points.Oath.Rules, catalog.OathPointRule{Template: 100610079, PartSetIndex: 16209})
	if w.sameOathSelectionGroup(100313757, 100610079) {
		t.Fatal("ambiguous source group accepted")
	}
	if !w.sameOathSelectionGroup(100313757, 100313757) {
		t.Fatal("existing exact-core request rejected")
	}
}

func TestOathDisplayMoveBoundary(t *testing.T) {
	for _, slot := range []uint16{35, 36, 47, 48} {
		want := slot >= 36 && slot <= 47
		if got := moveTouchesOath(protocol.ItemMoveRequest{SourceList: 3, SourceSlot: slot}); got != want {
			t.Fatalf("source slot %d", slot)
		}
		if got := moveTouchesOath(protocol.ItemMoveRequest{DestinationList: 3, DestinationSlot: slot}); got != want {
			t.Fatalf("destination slot %d", slot)
		}
		if moveTouchesOath(protocol.ItemMoveRequest{SourceList: 0, SourceSlot: slot}) {
			t.Fatal("bag-only move redraws oath")
		}
	}
}

func TestOathEquipmentRefreshFollowsActorRebuild(t *testing.T) {
	w, _ := oathSelectionFixture(t)
	plan := append(w.oathPointPackets(), outboundPacket{"clone_restored", 0, 2, nil}, outboundPacket{"worn_restored", 0, 14, nil})
	plan = w.refreshOathAfterEquipment(context.Background(), plan, protocol.ItemMoveRequest{DestinationList: 3, DestinationSlot: 36})
	if len(plan) != 4 || plan[0].ID != 2 || plan[1].ID != 14 || plan[2].ID != protocol.PartSetPointOpcode || plan[3].ID != 2839 {
		t.Fatalf("expected one point update after actor/worn rebuild: %+v", plan)
	}
}

func TestOathSelectionCurrentNativeSource(t *testing.T) {
	path := os.Getenv("DFO_OATH_SELECTION_TEST_PVF")
	if path == "" {
		t.Skip("native source test requires explicit archive")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 2 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	points, err := catalog.ImportPointRules(a)
	if err != nil {
		t.Fatal(err)
	}
	w, _ := oathSelectionFixture(t)
	w.items.Points = &points
	for _, id := range []uint32{100313757, 100610079, 100610060} {
		p := "equipment/character/common/oath/" + fmt.Sprint(id) + ".equ"
		script, err := catalog.ResolveScript(a, p)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for i, cell := range script.Cells {
			if cell.Type == 3 && cell.Text == "[equipment type]" && i+1 < len(script.Cells) && script.Cells[i+1].Text == "[oath]" {
				found = true
			}
		}
		if !found {
			t.Fatal("current source item is not an oath core", id)
		}
	}
	if !w.sameOathSelectionGroup(100313757, 100610079) || w.sameOathSelectionGroup(100313757, 100610060) {
		t.Fatal("current native group identity mismatch")
	}
	t.Logf("current source=%s point script=%s; golden group representative validated", a.Snapshot().Checksum, points.OathSHA256)
}

func TestOathActualFeatureSessionConfirmVector(t *testing.T) {
	w, _ := oathSelectionFixture(t)
	body, err := hex.DecodeString("a88758400100000000000000009caafa05020000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := w.oathSelectionPackets(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 3 || plan[0].ID != 2382 || plan[2].ID != 2839 || plan[2].Payload[0] != 2 {
		t.Fatalf("confirm plan=%+v", plan)
	}
	saved, err := w.store.EquippedOathSelection(context.Background(), w.account, w.role.ID)
	if err != nil || saved.ItemID != 100313757 || saved.Option != 2 {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
}
