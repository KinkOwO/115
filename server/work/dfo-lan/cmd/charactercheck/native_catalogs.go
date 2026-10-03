package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/managementdata"
	"fmt"
	"os"
)

func nativeSource() (*gamedata.Source, error) {
	path := os.Getenv("DFO_PVF_ARCHIVE")
	if path == "" {
		path = "../client-build/Script.inner.pvf"
	}
	return gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path})
}

func loadNativeQuestCatalog() (catalog.QuestCatalog, error) {
	source, err := nativeSource()
	if err != nil {
		return catalog.QuestCatalog{}, err
	}
	defer source.Close()
	return source.Quests("")
}

func loadNativeProgressionCatalog() (catalog.Progression, error) {
	source, err := nativeSource()
	if err != nil {
		return catalog.Progression{}, err
	}
	defer source.Close()
	return source.Progression("")
}

func loadNativeCharacterCatalog() (catalog.Characters, error) {
	source, err := nativeSource()
	if err != nil {
		return catalog.Characters{}, err
	}
	defer source.Close()
	return managementdata.Characters(source, "configs/pvf-character-policy.json")
}

func loadNativeLootCatalog() (catalog.LootCatalog, error) {
	source, err := nativeSource()
	if err != nil {
		return catalog.LootCatalog{}, err
	}
	defer source.Close()
	policy, err := inventory.ReadDropPolicy("configs/pvf-drop-policy.json")
	if err != nil {
		return catalog.LootCatalog{}, err
	}
	index, err := source.ItemIndex("")
	if err != nil {
		return catalog.LootCatalog{}, err
	}
	loot, err := source.Loot(policy.MaximumLootGrade)
	if err != nil {
		return loot, err
	}
	for _, id := range policy.ExcludedLootIDs {
		delete(loot.Items, id)
	}
	if err := loot.SupplementItemIndex(index); err != nil {
		return loot, err
	}
	return loot, nil
}

func loadNativeEquipmentCatalog(expectedSource string) (*inventory.EquipmentCatalog, error) {
	source, err := nativeSource()
	if err != nil {
		return nil, err
	}
	defer source.Close()
	index, err := source.ItemIndex("")
	if err != nil {
		return nil, err
	}
	quests := catalog.QuestCatalog{Source: source.Snapshot(), Quests: map[uint32]catalog.QuestDefinition{}}
	policy, err := inventory.ReadDropPolicy("configs/pvf-drop-policy.json")
	if err != nil {
		return nil, err
	}
	gear, err := source.EquipmentSelection(index, quests, policy)
	if err != nil {
		return nil, err
	}
	if expectedSource != "" && gear.Source.Checksum != expectedSource {
		return nil, fmt.Errorf("native equipment source %s does not match expected source %s", gear.Source.Checksum, expectedSource)
	}
	return gear, nil
}

func validateNativeEquipmentIdentity(source pvf.ArchiveSnapshot, characterConfigVersion, questSourceChecksum string) error {
	if source.SaveIdentity() != characterConfigVersion {
		return fmt.Errorf("native quest equipment save identity %s does not match character config version %s", source.SaveIdentity(), characterConfigVersion)
	}
	if source.Checksum != questSourceChecksum {
		return fmt.Errorf("native quest equipment source %s does not match quest source %s", source.Checksum, questSourceChecksum)
	}
	return nil
}

func loadNativeQuestEquipmentCatalog(characterConfigVersion, questSourceChecksum string) (*inventory.EquipmentCatalog, error) {
	source, err := nativeSource()
	if err != nil {
		return nil, err
	}
	defer source.Close()

	index, err := source.ItemIndex("")
	if err != nil {
		return nil, err
	}
	quests, err := source.Quests("")
	if err != nil {
		return nil, err
	}
	policy, err := inventory.ReadDropPolicy("configs/pvf-drop-policy.json")
	if err != nil {
		return nil, err
	}
	equipment, err := source.EquipmentSelection(index, quests, policy)
	if err != nil {
		return nil, err
	}
	if err := validateNativeEquipmentIdentity(equipment.Source, characterConfigVersion, questSourceChecksum); err != nil {
		return nil, err
	}
	return equipment, nil
}

func loadNativeWorldCatalog() (catalog.WorldCatalog, error) {
	source, err := nativeSource()
	if err != nil {
		return catalog.WorldCatalog{}, err
	}
	defer source.Close()
	return source.World("")
}

// The storage checks exercise dungeon 3; load its current native definition.
func loadNativeDungeonCatalog() (catalog.DungeonCatalog, error) {
	source, err := nativeSource()
	if err != nil {
		return catalog.DungeonCatalog{}, err
	}
	defer source.Close()
	return source.Dungeons([]uint32{3})
}
