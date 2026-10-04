// Package sqliteconvert exposes the one-way PostgreSQL to SQLite conversion as a
// maintenance command. It exists so the migration can be run deliberately, by hand,
// against a chosen source and a fresh destination - never as a side effect of startup.
package sqliteconvert

import (
	"context"
	"flag"
	"log"
	"time"

	"dfolan/internal/database"
)

func Run() {
	config := flag.String("config", "runtime/storage/local.json", "storage config of the PostgreSQL source")
	out := flag.String("out", "", "destination SQLite file (must not exist yet)")
	migrate := flag.Bool("migrate", false, "apply pending PostgreSQL migrations before reading")
	flag.Parse()

	if *out == "" {
		log.Fatal("--out is required: the converter must be told which new file to write")
	}
	settings, err := database.LoadConfig(*config)
	if err != nil {
		log.Fatal(err)
	}
	// The source is PostgreSQL by definition; the destination is chosen by --out.
	settings.Driver = database.DriverPostgres

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	source, err := database.Open(ctx, settings)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	if *migrate {
		if err := source.Migrate(ctx); err != nil {
			log.Fatal(err)
		}
	}

	report, err := database.ConvertPostgresToSQLite(ctx, source, *out)
	if err != nil {
		log.Fatalf("convert: %v", err)
	}

	var rows int64
	for _, table := range report.Tables {
		rows += table.Rows
	}
	log.Printf("converted %d tables, %d rows into %s", len(report.Tables), rows, *out)
	for _, table := range report.Tables {
		log.Printf("  %-40s %8d rows  json-verified %d", table.Name, table.Rows, table.JSONChecked)
	}
	if len(report.Missing) > 0 {
		// Worth shouting about: it means the source is older than the destination schema,
		// so the conversion cannot claim to be complete.
		log.Printf("WARNING: %d destination tables are absent from the PostgreSQL schema and were left empty: %v",
			len(report.Missing), report.Missing)
	}
	log.Print("the destination passed row-count, JSON byte and foreign-key verification")
}
