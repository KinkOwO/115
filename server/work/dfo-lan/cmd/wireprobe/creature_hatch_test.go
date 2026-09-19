package main

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

type mockHatchStore struct {
	events map[string]bool
	state  json.RawMessage
}

func (m *mockHatchStore) CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error) {
	if m.events == nil {
		m.events = map[string]bool{}
	}
	m.events[key] = true
	raw, _, err := apply(storage.Character{ID: id, AccountID: account, ConfigVersion: version, State: m.state})
	if err != nil {
		return storage.Character{}, false, err
	}
	m.state = raw
	return storage.Character{ID: id, AccountID: account, ConfigVersion: version, State: raw}, true, nil
}

func TestCreatureHatchFlow(t *testing.T) {
	initialBag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Special: map[byte][]inventory.BagEquipment{
			7: {
				{Slot: 0, Template: 63006}, // Pareas egg
				{Slot: 1, Template: 63007}, // Chaf egg
			},
		},
	}
	rawState, err := inventory.SaveBag(json.RawMessage(`{}`), initialBag)
	if err != nil {
		t.Fatal(err)
	}

	w := &worldSession{
		role: storage.Character{
			ID:            101,
			AccountID:     1,
			ConfigVersion: "test-ver",
			State:         rawState,
		},
	}
	store := &mockHatchStore{state: rawState}

	// Hatch slot 0 (Pareas egg 63006 -> Faras 63000)
	req := []byte{7, 0, 0}
	rawFrame := []byte{1, 0, 102, 0, 7, 0, 0}
	packets, err := w.hatchCreature(context.Background(), store, 102, req, rawFrame)
	if err != nil {
		t.Fatal("hatch error:", err)
	}
	if len(packets) != 3 {
		t.Fatalf("expected 3 packets, got %d", len(packets))
	}
	if packets[0].Kind != 1 || packets[0].ID != 102 || packets[0].Payload[0] != 1 {
		t.Fatalf("unexpected ACK: %+v", packets[0])
	}
	if packets[1].Kind != 0 || packets[1].ID != 14 || packets[1].Payload[0] != 7 {
		t.Fatalf("unexpected NOTI14: %+v", packets[1])
	}
	if packets[2].Kind != 0 || packets[2].ID != 105 {
		t.Fatalf("unexpected NOTI105: %+v", packets[2])
	}

	updatedBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if updatedBag.Special[7][0].Template != 63000 {
		t.Fatalf("expected template 63000, got %d", updatedBag.Special[7][0].Template)
	}
	if updatedBag.Special[7][1].Template != 63007 {
		t.Fatalf("second egg corrupted: %+v", updatedBag.Special[7][1])
	}

	// Test CMD 173 on slot 1 (Chaf egg 63007 -> Charp 63003)
	req173 := []byte{7, 1, 0}
	rawFrame173 := []byte{1, 0, 173, 0, 7, 1, 0}
	packets173, err := w.hatchCreature(context.Background(), store, 173, req173, rawFrame173)
	if err != nil {
		t.Fatal("hatch 173 error:", err)
	}
	if packets173[0].ID != 173 || packets173[0].Payload[0] != 1 {
		t.Fatalf("unexpected 173 ACK: %+v", packets173[0])
	}
	updatedBag2, _ := inventory.ReadBag(w.role.State)
	if updatedBag2.Special[7][1].Template != 63003 {
		t.Fatalf("expected template 63003, got %d", updatedBag2.Special[7][1].Template)
	}
}
