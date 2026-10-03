package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
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
