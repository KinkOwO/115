package loot

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func TestBoxContractRepairPreservesStateAndCannotPayTwice(t *testing.T) {
	s := &Service{Boxes: &BoxCatalog{Rewards: map[string]BoxReward{"101": {}, "102": {}}}}
	resolver := func(id uint32) (PremiumActivation, bool) {
		return PremiumActivation{Type: 79, DurationSecond: 3600}, id == 101
	}
	bag := inventory.Bag{Version: "ordinary-bag-v1", Gold: 19, Items: []inventory.BagItem{
		{Slot: 121, Template: 101, Amount: 2, ExpireTime: protocol.MaxItemPeriod},
		{Slot: 122, Template: 102, Amount: 1},
		{Slot: 123, Template: 999, Amount: 1},
	}}
	raw, err := inventory.SaveBag(json.RawMessage(`{"other":{"keep":7}}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role := Role{State: raw}
	original := append([]byte(nil), raw...)
	needed, err := s.NeedsBoxRewardRepair(role, resolver)
	if err != nil || !needed {
		t.Fatalf("repair eligibility: %t %v", needed, err)
	}
	after, receipt, premiums, err := s.PrepareBoxRewardRepair(role, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, original) {
		t.Fatal("input state mutated")
	}
	if len(premiums) != 1 || premiums[0].Type != 79 || premiums[0].DurationSecond != 7200 {
		t.Fatalf("activation: %+v", premiums)
	}
	updated, err := inventory.ReadBag(after)
	if err != nil || len(updated.Items) != 2 || updated.Gold != 19 || updated.Items[0].Template != 102 || updated.Items[0].ExpireTime != protocol.MaxItemPeriod || updated.Items[1].Template != 999 || updated.Items[1].ExpireTime != 0 {
		t.Fatalf("bag: %+v err=%v", updated, err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(after, &fields); err != nil || !bytes.Contains(fields["other"], []byte("7")) {
		t.Fatal("unrelated field lost", err)
	}
	if !json.Valid(receipt) {
		t.Fatal("invalid durable receipt")
	}
	role.State = after
	needed, err = s.NeedsBoxRewardRepair(role, resolver)
	if err != nil || needed {
		t.Fatalf("repaired contract can pay twice: %t %v", needed, err)
	}
}
