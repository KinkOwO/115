// Repair an explicitly selected development character's legacy zero progress.
package questrepair

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/managementdata"
	"dfolan/internal/quest"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"
)

func Run() {
	sourceFlags := managementdata.Register(flag.CommandLine)
	config := flag.String("storage", "runtime/storage/local.json", "storage configuration")
	flag.String("catalog", "", "deprecated; quests are read from native PVF")
	id := flag.Int64("character", 0, "exact development character ID")
	apply := flag.Bool("apply", false, "apply audited repair; default previews character changes after schema migration")
	flag.Parse()
	if sourceFlags.ArchivePath == "" {
		sourceFlags.ArchivePath = "../client-build/Script.inner.pvf"
	}
	native, e := sourceFlags.Open()
	if e != nil {
		log.Fatal(e)
	}
	defer native.Close()
	cat, e := native.Quests("")
	if e != nil {
		log.Fatal(e)
	}
	if sourceFlags.CheckOnly {
		if e := managementdata.Report(map[string]any{"source": cat.Source.Checksum, "quests": len(cat.Quests), "storage_accessed": false}); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *id <= 0 {
		log.Fatal("an exact character ID is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conf, e := database.LoadConfig(*config)
	if e != nil {
		log.Fatal(e)
	}
	s, e := database.Open(ctx, conf)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	if e = s.MigrateQuests(ctx); e != nil {
		log.Fatal(e)
	}
	var account int64
	if account, e = s.DevelopmentCharacterAccount(ctx, *id); e != nil {
		log.Fatal("development character not found")
	}
	quests, e := s.Quests(ctx, account, *id)
	if e != nil {
		log.Fatal(e)
	}
	changes := []map[string]any{}
	for _, q := range quests {
		if q.ProgressModel != "legacy-zero" {
			continue
		}
		d, ok := cat.Quests[uint32(q.ID)]
		if !ok || q.ConfigVersion != cat.Source.SaveIdentity() {
			log.Fatal("quest source mismatch")
		}
		initial, model, e := quest.InitialProgress(d)
		if e != nil {
			log.Fatal(e)
		}
		if q.Status != "accepted" || q.Progress != 0 {
			log.Fatal("unexpected legacy state; no repair applied")
		}
		changes = append(changes, map[string]any{"character": *id, "quest": q.ID, "before": q, "remaining": initial, "progress_model": model, "objective": d.ObjectiveCells})
	}
	// Finish validation before applying the explicitly selected records.
	if *apply {
		for _, q := range quests {
			if q.ProgressModel == "legacy-zero" {
				initial, model, _ := quest.InitialProgress(cat.Quests[uint32(q.ID)])
				if e = s.RepairLegacyQuest(ctx, account, *id, q, initial, model); e != nil {
					log.Fatal(e)
				}
			}
		}
	}
	b, _ := json.MarshalIndent(map[string]any{"applied": *apply, "changes": changes}, "", "  ")
	fmt.Println(string(b))
}
