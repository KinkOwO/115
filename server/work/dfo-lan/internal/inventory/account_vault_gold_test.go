package inventory

import (
	"encoding/json"
	"testing"
)

func TestAccountVaultGoldButtonsPreserveBalancesOnFailure(t *testing.T) {
	rules := AccountVaultRules{Upgrades: [][6]int64{{8, 100000000}}}
	role := Role{State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","gold":700}}`)}
	vault := AccountVaultState{Slots: 8, Gold: 100, Items: json.RawMessage(`[]`)}
	state, next, err := DepositAccountVaultGold(role, vault, rules, 500)
	if err != nil || next.Gold != 600 {
		t.Fatalf("deposit: %v %+v", err, next)
	}
	bag, err := ReadBag(state)
	if err != nil || bag.Gold != 200 {
		t.Fatalf("bag: %v %+v", err, bag)
	}
	role.State = state
	state, next, err = WithdrawAccountVaultGold(role, next, rules, 600)
	if err != nil || next.Gold != 0 {
		t.Fatalf("withdraw: %v %+v", err, next)
	}
	bag, err = ReadBag(state)
	if err != nil || bag.Gold != 800 {
		t.Fatalf("bag: %v %+v", err, bag)
	}
	if _, after, err := WithdrawAccountVaultGold(role, vault, rules, 101); err == nil || after.Gold != vault.Gold {
		t.Fatalf("failure mutated vault: %v %+v", err, after)
	}
}
