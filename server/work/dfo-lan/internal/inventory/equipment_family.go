package inventory

import "strings"

var EggHatchOutputs = map[uint32]uint32{
	63006: 63000, // Pareas -> Faras
	63007: 63003, // Chaf -> Charp
	63013: 63008, // Botis -> Botis
	63014: 63009, // Marbas -> Marbas
	63015: 63010, // Flamba -> Balam
	63016: 63011, // Haagenti -> Haagenti
	63017: 63012, // Berith -> Berith
	63018: 63019, // Scythilid -> Zagan
	63022: 63023, // Bobo -> Bobo
	63024: 63025, // Amy -> Amy
	63028: 63027, // Aveas -> Elbon
	63031: 63032, // Belz -> Belz
}

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

