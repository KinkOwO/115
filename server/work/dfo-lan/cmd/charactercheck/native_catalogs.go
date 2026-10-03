package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
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
