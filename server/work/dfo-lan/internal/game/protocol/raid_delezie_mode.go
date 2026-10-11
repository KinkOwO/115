package protocol

import "fmt"

// CMD656's fifth option comes from 14506F4B0. Kind14 resolves through
// 142513D98 -> 1406BAA80 -> 1420C0C70, whose vtable+468 returns true.
// The selection branch therefore emits normal0/hard1/single2/match3,
// matching the native difficulty enum at 14759DC80.
func (r RaidCreateRequest) DelezieMode115() (byte, error) {
	if r.Kind != 14 || r.Options[4] > 3 {
		return 0, fmt.Errorf("unsupported native Delezie difficulty")
	}
	return r.Options[4], nil
}
