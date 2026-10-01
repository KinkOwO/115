// Command attunementimport decodes the per-dungeon reward tables that drive the
// "boundary of attunement" (调律之边界) abyss dungeons and writes the truth the
// server consumes:
//
//	configs/attunement-rewards.generated.json
//
// The input is read straight out of the frozen client PVF, so the generated
// file can always be reproduced and diffed against the source hash. Nothing is
// invented: every field is a cell read, and the one field whose meaning is not
// yet proven ([hidden drop table]'s middle number, see hiddenEntry.Key) keeps
// its positional name plus a note.
//
// The container stores its values as positional triples, not as a nested
// structure, so the shape below is the shape the boundary-of-attunement tables
// actually use (verified on unique/legendary/epic alike):
//
//	[dungeon index]          1 float        which dungeon owns this table
//	[fixed drop table]       per maze       [maze] + [drop list]
//	[additional drop table]  x N            [effect index] + [select prob]
//	                                         + [drop count] + [drop list]
//	[hidden drop table]      per maze       [maze] + N x [drop list]
//	[coupon drop table]      x N            [obtain prob] + [drop prob] + [drop list]
//
// [drop list] inside a fixed/additional table is (tier, weight, item) repeated,
// and the weights always sum to exactly one million. That invariant is the
// reason a decoder can be shipped at all: it makes the reading falsifiable, so
// the importer refuses to emit a table that breaks it.
package main

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/loot"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
)

func main() {
	source := flag.String("source", "../client-build/Script.inner.pvf", "source PVF")
	baseDir := flag.String("base", "etc/rewardboostinfo/skyofathousandseasofborder", "directory holding the difficulty tables")
	extra := flag.String("extra", "", "comma separated extra archive entries to import alongside the base directory (e.g. the endkeeperoforder table)")
	output := flag.String("output", "configs/attunement-rewards.generated.json", "generated config")
	flag.Parse()

	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatalf("load %s: %v", *source, err)
	}

	doc := loot.AttunementRewards{Model: loot.AttunementModel, Archive: a.Snapshot()}
	sources := make([]string, 0, 4)
	for _, name := range []string{"unique.ctp", "legendary.ctp", "epic.ctp"} {
		sources = append(sources, path.Join(*baseDir, name))
	}
	// -extra lets one generated file carry other dungeons' rewardboostinfo
	// tables. Every table names its own dungeon in [dungeon index], so the
	// document keeps its one-table-per-dungeon shape no matter how many are
	// imported together; a duplicate dungeon index is refused by the loader.
	for _, e := range strings.Split(*extra, ",") {
		if e = strings.TrimSpace(e); e != "" {
			sources = append(sources, e)
		}
	}
	for _, entry := range sources {
		table, err := loot.ReadAttunementTable(a, entry)
		if err != nil {
			log.Fatalf("read %s: %v", entry, err)
		}
		fmt.Printf("%-58s dungeon=%-10d fixed=%d additional=%d hidden=%d coupon=%d\n",
			path.Base(entry), table.Dungeon, len(table.Fixed), len(table.Additional),
			len(table.Hidden), len(table.Coupons))
		doc.Tables = append(doc.Tables, table)
	}
	if len(doc.Tables) == 0 {
		log.Fatal("no attunement reward table found")
	}

	if _, err := loot.NewAttunementRewards(doc); err != nil {
		log.Fatal(err)
	}
	b, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(*output, b, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nwrote %s (%d bytes, %d tables)\n", *output, len(b), len(doc.Tables))
}
