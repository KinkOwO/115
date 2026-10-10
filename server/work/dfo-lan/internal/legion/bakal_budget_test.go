package legion

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestBakalBudgetUsesSourceAndPersistsBetweenRooms(t *testing.T) {
	o, at := runtimeNative(t, false)
	coins, potions := o.Budget()
	if coins != o.rules.PartyCoinLimit || potions != o.rules.GuaranteePartyPotion {
		t.Fatal("opening budgets differ from source")
	}
	for i := 0; i < coins; i++ {
		if e := o.CheckCoinBudget(); e != nil {
			t.Fatal(e)
		}
		o.SpendCoinBudget()
	}
	if o.CheckCoinBudget() == nil {
		t.Fatal("exhausted coins accepted")
	}
	for i := 0; i < potions; i++ {
		o.SpendPotionBudget()
	}
	if o.CheckPotionBudget() == nil {
		t.Fatal("exhausted potions accepted")
	}
	if binary.LittleEndian.Uint32(o.PartyFrame(24, at)[9:]) != 0 {
		t.Fatal("wrong native remaining coins")
	}
	// A position publication must never refill the budget.
	o.StageWarp(24)
	if c, p := o.Budget(); c != 0 || p != 0 {
		t.Fatal("room movement reset budget")
	}
	o.guaranteeCampBudget()
	if c, p := o.Budget(); c != o.rules.GuaranteePartyCoin || p != o.rules.GuaranteePartyPotion {
		t.Fatal("camp guarantees not applied")
	}
	if _, e := o.UseRaidBuffAt(1, at.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	if c, _ := o.Budget(); c != o.rules.GuaranteePartyCoin+o.rules.BuffDefinitions["RaidBuffAddCoin"].ServerValue {
		t.Fatal("commander did not replenish source count")
	}
	d := o.rules.BuffDefinitions["StackablePotion"]
	d.ServerValue = 9
	o.rules.BuffDefinitions[d.Kind] = d
	if e := o.grantEventBuff(d.Kind, at); e != nil {
		t.Fatal(e)
	}
	if _, p := o.Budget(); p != o.rules.GuaranteePartyPotion+9 {
		t.Fatal("modified source potion grant ignored")
	}
}

func TestBakalBudgetEventCloneRollsBack(t *testing.T) {
	o, at := runtimeNative(t, false)
	c := o.eventClone()
	c.SpendCoinBudget()
	c.SpendPotionBudget()
	if coins, potions := o.Budget(); coins != o.rules.PartyCoinLimit || potions != o.rules.GuaranteePartyPotion {
		t.Fatal("uncommitted event mutated original")
	}
	o.stage = BakalOpeningFailed
	if o.CheckCoinBudget() == nil || o.CheckPotionBudget() == nil {
		t.Fatal("failed raid admitted use")
	}
	_ = at
}
