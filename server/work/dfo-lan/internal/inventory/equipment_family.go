package inventory

import "strings"

// Native body getter admits slots0..47. Live primer moves use37 and45;
// the eleven surrounding slots36..46 are separate from oath47.
func EquipmentBodySlot(slot uint16) bool { return slot <= 47 }
func EquipmentBagSpace(kind string) byte {
	if strings.HasSuffix(kind, " avatar]") {
		return 1
	}
	switch kind {
	case "[creature]", "[creature skin]", "[artifact red]", "[artifact blue]", "[artifact green]":
		return 7
	}
	return 0
}
