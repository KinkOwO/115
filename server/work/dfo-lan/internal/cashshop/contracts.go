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

var contractDurations = map[uint32]Contract{
	30: {PremiumConqueror, 1 * 86400}, 2660704: {PremiumConqueror, 1 * 86400}, 741: {PremiumConqueror, 2 * 86400}, 31: {PremiumConqueror, 3 * 86400}, 32: {PremiumConqueror, 5 * 86400}, 33: {PremiumConqueror, 7 * 86400}, 2660050: {PremiumConqueror, 10 * 86400}, 34: {PremiumConqueror, 15 * 86400}, 2660012: {PremiumConqueror, 30 * 86400}, 10096109: {PremiumConqueror, 3600},
	43: {PremiumTactician, 1 * 86400}, 2660705: {PremiumTactician, 1 * 86400}, 742: {PremiumTactician, 2 * 86400}, 44: {PremiumTactician, 3 * 86400}, 200: {PremiumTactician, 5 * 86400}, 45: {PremiumTactician, 7 * 86400}, 2660051: {PremiumTactician, 10 * 86400}, 46: {PremiumTactician, 15 * 86400}, 2660013: {PremiumTactician, 30 * 86400}, 10096110: {PremiumTactician, 3600},
	2660354: {PremiumGabriel, 30 * 86400}, 10092497: {PremiumGabriel, 5 * 86400}, 10096111: {PremiumGabriel, 3600}, 10151653: {PremiumGabriel, 15 * 86400}, 10157955: {PremiumGabriel, 1 * 86400}, 590004603: {PremiumGabriel, 7 * 86400}, 590712185: {PremiumGabriel, 3 * 86400}, 590712539: {PremiumGabriel, 30 * 86400}, 590004904: {PremiumGabriel, 15 * 86400}, 590717046: {PremiumGabriel, 3 * 86400}, 590717459: {PremiumGabriel, 15 * 86400}, 590717589: {PremiumGabriel, 3 * 86400}, 590721465: {PremiumGabriel, 1 * 86400},
	2660703: {PremiumGrowth, 1 * 86400}, 2660409: {PremiumGrowth, 3 * 86400}, 2660410: {PremiumGrowth, 7 * 86400}, 2660411: {PremiumGrowth, 15 * 86400}, 10327725: {PremiumGrowth, 30 * 86400}, 10096112: {PremiumGrowth, 3600},
	10000391: {PremiumCube, 1 * 86400}, 10000388: {PremiumCube, 3 * 86400}, 10000389: {PremiumCube, 7 * 86400}, 10000390: {PremiumCube, 15 * 86400}, 10327726: {PremiumCube, 30 * 86400}, 10096113: {PremiumCube, 3600},
	50002526: {PremiumNeoBasic, 1 * 86400}, 50002527: {PremiumNeoBasic, 3 * 86400}, 50002528: {PremiumNeoBasic, 5 * 86400}, 50002529: {PremiumNeoBasic, 11 * 86400}, 50002917: {PremiumNeoBasic, 23 * 86400}, 50002530: {PremiumNeoBasic, 3 * 60},
	50002532: {PremiumNeoPlus, 1 * 86400}, 50002533: {PremiumNeoPlus, 3 * 86400}, 50002534: {PremiumNeoPlus, 5 * 86400}, 590005745: {PremiumNeoPlus, 3 * 86400}, 590005746: {PremiumNeoPlus, 5 * 86400}, 590005747: {PremiumNeoPlus, 10 * 86400}, 590705236: {PremiumNeoPlus, 10 * 86400}, 50002535: {PremiumNeoPlus, 11 * 86400}, 50002918: {PremiumNeoPlus, 23 * 86400}, 590005748: {PremiumNeoPlus, 23 * 86400}, 590005782: {PremiumNeoPlus, 23 * 86400}, 590008445: {PremiumNeoPlus, 23 * 86400}, 590009297: {PremiumNeoPlus, 23 * 86400}, 50002536: {PremiumNeoPlus, 3 * 60}, 590005749: {PremiumNeoPlus, 3 * 86400}, 50042664: {PremiumNeoPlus, 5 * 86400}, 590005750: {PremiumNeoPlus, 5 * 86400}, 50042665: {PremiumNeoPlus, 10 * 86400}, 590005751: {PremiumNeoPlus, 10 * 86400}, 590705237: {PremiumNeoPlus, 10 * 86400}, 590005752: {PremiumNeoPlus, 23 * 86400}, 50051259: {PremiumNeoPlus, 1 * 86400}, 50051260: {PremiumNeoPlus, 3 * 86400}, 590714871: {PremiumNeoPlus, 7 * 86400}, 590714872: {PremiumNeoPlus, 3 * 86400}, 590714873: {PremiumNeoPlus, 11 * 86400}, 590714876: {PremiumNeoPlus, 3 * 86400}, 590714877: {PremiumNeoPlus, 7 * 86400}, 590714297: {PremiumNeoPlus, 3 * 86400}, 590714298: {PremiumNeoPlus, 7 * 86400}, 590714301: {PremiumNeoPlus, 14 * 86400}, 590714299: {PremiumNeoPlus, 15 * 86400}, 590714300: {PremiumNeoPlus, 30 * 86400}, 590717455: {PremiumNeoPlus, 3 * 86400},
	// Additional contracts from Cera Shop & in-game packages:
	590714880: {PremiumConqueror, 15 * 86400}, 590709319: {PremiumConqueror, 15 * 86400}, 590005436: {PremiumConqueror, 15 * 86400}, 590005211: {PremiumConqueror, 15 * 86400},
	590714881: {PremiumTactician, 15 * 86400}, 590709321: {PremiumTactician, 15 * 86400}, 590005437: {PremiumTactician, 15 * 86400}, 590005213: {PremiumTactician, 15 * 86400},
	590714882: {PremiumGabriel, 15 * 86400}, 590004900: {PremiumGabriel, 7 * 86400}, 590004901: {PremiumGabriel, 15 * 86400}, 590004902: {PremiumGabriel, 30 * 86400}, 590709311: {PremiumGabriel, 15 * 86400}, 590005422: {PremiumGabriel, 15 * 86400}, 590005426: {PremiumGabriel, 30 * 86400}, 590005203: {PremiumGabriel, 15 * 86400},
	590714879: {PremiumGrowth, 15 * 86400}, 590709317: {PremiumGrowth, 15 * 86400}, 590005209: {PremiumGrowth, 15 * 86400},
	590714878: {PremiumCube, 15 * 86400}, 590709315: {PremiumCube, 15 * 86400}, 590005207: {PremiumCube, 15 * 86400},
	590720003: {PremiumNeoPlus, 30 * 86400}, 590720004: {PremiumNeoPlus, 15 * 86400}, 590714885: {PremiumNeoPlus, 30 * 86400}, 590714886: {PremiumNeoPlus, 14 * 86400}, 590709307: {PremiumNeoPlus, 14 * 86400}, 590709309: {PremiumNeoPlus, 30 * 86400}, 590005420: {PremiumNeoPlus, 14 * 86400}, 590005199: {PremiumNeoPlus, 14 * 86400}, 590005201: {PremiumNeoPlus, 30 * 86400}, 590701428: {PremiumNeoPlus, 10 * 86400}, 50002921: {PremiumNeoPlus, 3 * 86400}, 50002922: {PremiumNeoPlus, 7 * 86400},
}

func ResolveContract(template uint32) (Contract, bool) {
	c, ok := contractDurations[template]
	return c, ok
}

// ResolveContractItem 复用当前客户端的契约别名映射，供非商城奖励使用。
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
	if c, ok := resolveContract(uint32(entry.Row[1].Value)); ok {
		return c, true, nil
	}
	items, ok := PackageItems(entry.Item)
	if !ok || len(items) != 1 || items[0].Count == 0 {
		return Contract{}, false, nil
	}
	c, ok := resolveContract(items[0].Template)
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
