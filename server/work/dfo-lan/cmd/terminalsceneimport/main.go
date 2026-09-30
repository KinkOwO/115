// terminalsceneimport extracts final-layer [CHANGE MAP] scene bounds from the
// current PVF for single-map clear quests. Run after rebuilding the catalogs.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	archivePath := flag.String("pvf", "../client-build/Script.inner.pvf", "current 115 inner PVF")
	dungeonsPath := flag.String("dungeons", "configs/dungeons.full.json", "full dungeon catalog")
	questsPath := flag.String("quests", "configs/quests.generated.json", "quest catalog")
	output := flag.String("out", "configs/dungeons.terminal-scenes.json", "source scene metadata")
	flag.Parse()
	if err := run(*archivePath, *dungeonsPath, *questsPath, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(archivePath, dungeonsPath, questsPath, output string) error {
	d, err := catalog.LoadDungeons(dungeonsPath)
	if err != nil {
		return err
	}
	quests, err := catalog.LoadQuests(questsPath)
	if err != nil {
		return err
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: archivePath, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		return err
	}
	overlay, err := catalog.ImportTerminalScenes(a, d, quests)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(output, append(b, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("exported %d terminal scenes to %s\n", len(overlay.Scenes), output)
	return nil
}
