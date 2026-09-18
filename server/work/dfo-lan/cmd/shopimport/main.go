package main

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

func main() {
	source := flag.String("source", "", "read-only inner PVF")
	output := flag.String("output", "configs/shop-purchase-pilot.json", "output catalog")
	report := flag.String("report", "configs/shop-purchase-report.json", "enabled and rejected ordinary products")
	flag.Parse()
	if filepath.Clean(*output) == filepath.Clean(*report) {
		log.Fatal("catalog and report must have distinct paths")
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
