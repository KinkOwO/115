package workflow

import (
	"dfolan/internal/cashshop"
	"dfolan/internal/loot"
)

func resolveLootContract(template uint32) (loot.PremiumActivation, bool) {
	contract, ok := cashshop.ResolveContractItem(template)
	return loot.PremiumActivation{Type: contract.Type, DurationSecond: contract.DurationSecond}, ok
}
