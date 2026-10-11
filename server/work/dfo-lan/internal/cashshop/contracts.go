package cashshop

import "fmt"

// Premium contract identities are defined by premiumlist_new.etc in the 115
// client. Cera-shop rows often wrap these identities in a package item; the
// package data is resolved before delivery so the wrapper never reaches the bag.
const (
	PremiumConqueror uint8 = 22
	PremiumTactician uint8 = 27
	PremiumGabriel   uint8 = 73
	PremiumGrowth    uint8 = 79
	PremiumCube      uint8 = 92
	PremiumNeoBasic  uint8 = 117
	PremiumNeoPlus   uint8 = 118
)

type Contract struct {
	Type           uint8
	DurationSecond int64
}

// ResolveContract reads only the source-bound runtime registry installed at startup.
func ResolveContract(template uint32) (Contract, bool) {
	rules := installedContracts.Load()
	if rules == nil {
		return Contract{}, false
	}
	c, ok := rules.Items[template]
	return c, ok
}

// ResolveContractItem 读取当前 PVF 的契约规则，供非商城奖励使用。
func ResolveContractItem(template uint32) (Contract, bool) {
	return resolveContract(template)
}

// resolveContract is an internal alias kept for package compatibility.
func resolveContract(template uint32) (Contract, bool) {
	return ResolveContract(template)
}

// entryContract resolves both direct premium aliases and the package-data
// aliases used by the current Cera shop rows.
func entryContract(entry OrdinaryProduct) (Contract, bool, error) {
	resolve := resolveContract
	if entry.Contracts != nil {
		resolve = func(id uint32) (Contract, bool) { c, ok := entry.Contracts.Items[id]; return c, ok }
	}

	if c, ok := resolve(uint32(entry.Row[1].Value)); ok {
		return c, true, nil
	}
	items, ok := PackageItems(entry.Item)
	if !ok || len(items) != 1 || items[0].Count == 0 {
		return Contract{}, false, nil
	}
	c, ok := resolve(items[0].Template)
	if !ok {
		return Contract{}, false, nil
	}
	if items[0].Count != 1 {
		return Contract{}, false, fmt.Errorf("contract package has unsupported count")
	}
	return c, true, nil
}

// ContractForProduct is kept small and source-backed so callers can validate a
// product before charging. It is also useful to tests without exposing the
// PVF entry representation outside cashshop.
func ContractForProduct(entry OrdinaryProduct) (uint8, int64, bool, error) {
	c, ok, err := entryContract(entry)
	return c.Type, c.DurationSecond, ok, err
}
