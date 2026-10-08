package legion

import "fmt"

// These are run budgets, separate from the character's owned coins/items.
// Native N2285 @9 is remaining coins (official paired C41: 5 -> 4).
func (o *BakalOpening) CheckCoinBudget() error {
	if o.runtime == nil || o.stage != BakalOpeningActive || o.runtime.coins <= 0 {
		return fmt.Errorf("Bakal party revival allowance exhausted or inactive")
	}
	return nil
}

func (o *BakalOpening) SpendCoinBudget() {
	if o.runtime != nil && o.runtime.coins > 0 {
		o.runtime.coins--
	}
}

func (o *BakalOpening) CheckPotionBudget() error {
	if o.runtime == nil || o.stage != BakalOpeningActive || o.runtime.potions <= 0 {
		return fmt.Errorf("Bakal limited consumable allowance exhausted or inactive")
	}
	return nil
}

func (o *BakalOpening) SpendPotionBudget() {
	if o.runtime != nil && o.runtime.potions > 0 {
		o.runtime.potions--
	}
}

func (o *BakalOpening) guaranteeCampBudget() {
	if o.runtime == nil {
		return
	}
	if o.runtime.coins < o.rules.GuaranteePartyCoin {
		o.runtime.coins = o.rules.GuaranteePartyCoin
	}
	if o.runtime.potions < o.rules.GuaranteePartyPotion {
		o.runtime.potions = o.rules.GuaranteePartyPotion
	}
}

func (o *BakalOpening) Budget() (coins, potions int) {
	if o.runtime != nil {
		return o.runtime.coins, o.runtime.potions
	}
	return 0, 0
}
