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
