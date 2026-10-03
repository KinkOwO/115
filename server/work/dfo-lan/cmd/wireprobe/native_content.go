package main

import (
	"dfolan/internal/gamedata"
	"fmt"
	"os"
)

// Validate content dependencies before storage, migrations or runtime globals.
// Historical path arguments never select a second content source.
func requireRuntimeContent(startup Config, c *gamedata.Catalogs) error {
	for _, need := range []struct {
		domain string
		used   bool
	}{
		{"town", startup.TownCatalog != ""},
		{"journal", startup.EquipmentJournalRules != ""},
		{"create-cost", startup.EquipmentCreateCost != ""},
		{"tutorial", startup.TutorialRoutes != ""},
		{"tutorial-dungeons", startup.TutorialDungeons != ""},
		{"training-dungeons", c.Dungeons != nil || os.Getenv("DFO_TRAINING_ROOM_CATALOG") != ""},
		{"dungeon-terminal", c.Dungeons != nil},
		{"layer-revisits", c.Dungeons != nil},
		{"dungeon-tournament", c.Dungeons != nil},
		{"dungeon-towers", c.Dungeons != nil},
		{"dungeon-maze", c.Dungeons != nil},
		{"dungeon-hell", c.Dungeons != nil},
		{"random-options", startup.RandomOptionCatalog != ""},
		{"shields", startup.QuestEquipmentCatalog != "" && startup.EquipmentWearRules != "" && startup.KnightShieldCatalog != ""},
		{"oath-grades", startup.OathGradesFromGear || startup.OathGradesTable != ""},
		{"vault", startup.VaultRules != ""},
		{"boxes", startup.Boxes != ""},
		{"item-shops", startup.ItemShop != ""},
		{"bleeding-mine", startup.BleedingMineRewards != ""},
		{"apocalypse", startup.ApocalypseCatalog != ""},
		{"attunement", startup.AttunementRewards != ""},
		{"odyssey-growth", os.Getenv("DFO_ODYSSEY_GROWTH") != ""},
		{"odyssey-chapters", os.Getenv("DFO_ODYSSEY_CHAPTERS") != ""},
		{"odyssey-weapons", os.Getenv("DFO_ODYSSEY_WEAPON_BOX") != ""},
		{"odyssey-drop", os.Getenv("DFO_ODYSSEY_CHAPTER_DROP") != ""},
		{"odyssey-currency", os.Getenv("DFO_ODYSSEY_COIN_RULES") != ""},
		{"clear-cube", os.Getenv("DFO_CLEAR_CUBE_SOURCE") != ""},
		{"characters", startup.CharacterStorage != ""},
	} {
		if need.used && (!c.Selected(need.domain) || !c.Prepared(need.domain)) {
			return fmt.Errorf("%s requires a prepared native PVF domain; JSON runtime catalogs are retired", need.domain)
		}
	}
	return nil
}
