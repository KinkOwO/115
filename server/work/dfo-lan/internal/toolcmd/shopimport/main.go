package shopimport

import (
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
)

func writeAtomic(name string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(name), ".shop-import-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, name)
}

func Run() {
	source := flag.String("source", "", "read-only inner PVF")
	output := flag.String("output", ".tmp/shop-purchase-pilot.json", "output catalog (historical baseline export; never a runtime input)")
	report := flag.String("report", ".tmp/shop-purchase-report.json", "enabled and rejected ordinary products (historical baseline export)")
	flag.Parse()
	if filepath.Clean(*output) == filepath.Clean(*report) {
		log.Fatal("catalog and report must have distinct paths")
	}
	for _, dir := range []string{filepath.Dir(*output), filepath.Dir(*report)} {
		if e := os.MkdirAll(dir, 0o755); e != nil {
			log.Fatal(e)
		}
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := cashshop.ImportPilot(a)
	if e != nil {
		log.Fatal(e)
	}
	catalogBytes, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	rows := c.Report()
	b, e := json.MarshalIndent(rows, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = writeAtomic(*report, b); e != nil {
		log.Fatal(e)
	}
	if e = writeAtomic(*output, catalogBytes); e != nil {
		log.Fatal(e)
	}
	enabled := 0
	for _, r := range rows {
		if r.Enabled {
			enabled++
		}
	}
	log.Printf("PVF catalog: total%d enabled%d blocked%d source%s", len(rows), enabled, len(rows)-enabled, c.Source.Checksum)
}
