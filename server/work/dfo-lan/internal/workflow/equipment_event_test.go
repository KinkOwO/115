package workflow

import (
	"context"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"testing"
)

type equipmentEventFake struct {
	role                  storage.Character
	receipt               json.RawMessage
	replay                bool
	commitErr, receiptErr error
	reads, applies        int
}

func (s *equipmentEventFake) CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string,
	apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error) {
	if s.commitErr != nil {
		return storage.Character{}, false, s.commitErr
	}
	if !s.replay {
		s.applies++
		state, receipt, err := apply(s.role)
		if err != nil {
			return storage.Character{}, false, err
		}
		s.role.State, s.receipt = state, receipt
	}
	return s.role, !s.replay, nil
}
func (s *equipmentEventFake) CharacterEventReceipt(context.Context, int64, int64, string) (json.RawMessage, error) {
	s.reads++
	return s.receipt, s.receiptErr
}

func TestEquipmentEventRestoresPersistedReceipt(t *testing.T) {
	role := storage.Character{ID: 7, AccountID: 2, WireID: 19, State: json.RawMessage(`{"old":true}`)}
	store := &equipmentEventFake{role: role}
	apply := func(current storage.Character) (json.RawMessage, []int, error) {
		if string(current.State) != string(role.State) {
			t.Fatal("apply did not receive locked state")
		}
		return json.RawMessage(`{"new":true}`), []int{3, 8}, nil
	}
	saved, receipt, err := commitEquipmentEvent(context.Background(), store, role, "key", "model", apply)
	if err != nil || string(saved.State) != `{"new":true}` || len(receipt) != 2 || receipt[1] != 8 {
		t.Fatalf("first commit: %+v %v %v", saved, receipt, err)
	}
	store.replay = true
	store.role.WireID = 0
	saved, receipt, err = commitEquipmentEvent(context.Background(), store, role, "key", "model",
		func(storage.Character) (json.RawMessage, []int, error) {
			t.Fatal("replay reran equipment rules")
			return nil, nil, nil
		})
	if err != nil || saved.WireID != role.WireID || len(receipt) != 2 || receipt[0] != 3 || store.applies != 1 || store.reads != 2 {
		t.Fatalf("replay: %+v %v %v; applies=%d reads=%d", saved, receipt, err, store.applies, store.reads)
	}
}

func TestEquipmentEventFailures(t *testing.T) {
	failure := errors.New("failure")
	role := storage.Character{ID: 7, WireID: 19, State: json.RawMessage(`{}`)}
	for _, phase := range []string{"commit", "apply", "marshal", "receipt", "decode"} {
		t.Run(phase, func(t *testing.T) {
			store := &equipmentEventFake{role: role}
			switch phase {
			case "commit":
				store.commitErr = failure
			case "receipt":
				store.receiptErr = failure
			case "decode":
				store.replay, store.receipt = true, json.RawMessage(`{`)
			}
			saved, _, err := commitEquipmentEvent(context.Background(), store, role, "key", "model", func(storage.Character) (json.RawMessage, any, error) {
				if phase == "apply" {
					return nil, nil, failure
				}
				if phase == "marshal" {
					return nil, make(chan int), nil
				}
				return json.RawMessage(`{"new":true}`), 42, nil
			})
			if err == nil {
				t.Fatal("expected failure")
			}
			if phase == "commit" || phase == "apply" || phase == "marshal" {
				if store.reads != 0 || string(saved.State) != string(role.State) {
					t.Fatal("failed commit read receipt or replaced original role")
				}
			} else if store.reads != 1 || saved.WireID != role.WireID {
				t.Fatal("receipt failure lost committed role identity")
			}
		})
	}
}
