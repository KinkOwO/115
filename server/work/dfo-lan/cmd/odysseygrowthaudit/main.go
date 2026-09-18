// Read-only export of Odyssey growth, gift, and drop source evidence.
package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	a, e := pvf.LoadArchive(pvf.Options{Path: "../client-build/Script.inner.pvf", MaxBytes: 1024 * 1024 * 1024})
	must(e)
	dest := "docs/evidence/odyssey-growth-20260917/source"
	must(os.MkdirAll(dest, 0755))
	write := func(name string, v any) {
		b, e := json.MarshalIndent(v, "", "  ")
		must(e)
		must(os.WriteFile(filepath.Join(dest, name), b, 0644))
	}
	var paths []string
	for _, f := range a.Files() {
		p := strings.ToLower(f.ArchivePath)
		if strings.Contains(p, "odyssey") || strings.Contains(p, "itemdrop") {
			paths = append(paths, f.ArchivePath)
			if strings.Contains(p, ".etc") {
				s, e := catalog.ReadScript(a, f.ArchivePath)
				if e == nil {
					write(strings.ReplaceAll(p, "/", "_")+".json", s)
				}
			}
		}
	}
	write("paths.json", paths)
	wanted := map[uint32]bool{10419348: true, 10419349: true, 10419350: true, 10420561: true, 10418028: true, 10418036: true, 10418035: true}
	var drops []map[string]any
	for _, f := range a.Files() {
		p := strings.ToLower(f.ArchivePath)
		if !strings.Contains(p, "odyssey") || !(strings.HasSuffix(p, ".mob") || strings.HasSuffix(p, ".dgn") || strings.HasSuffix(p, ".shp")) {
			continue
		}
		s, e := catalog.ReadScript(a, f.ArchivePath)
		if e != nil {
			continue
		}
		if strings.HasSuffix(p, ".shp") {
			write(strings.ReplaceAll(p, "/", "_")+".json", s)
		}
		var picked []pvf.Token
		on := false
		for _, c := range s.Cells {
			if c.Type == 3 {
				on = strings.Contains(c.Text, "drop") || strings.Contains(c.Text, "item")
			}
			if on {
				picked = append(picked, c)
				if c.Type == 0 && c.Value > 10000000 && c.Value < 100000000 {
					wanted[uint32(c.Value)] = true
				}
			}
		}
		if len(picked) > 0 {
			drops = append(drops, map[string]any{"path": s.Path, "sha256": s.SHA256, "cells": picked})
		}
	}
	write("drop-sections.json", drops)
	idx, e := catalog.ReadScript(a, "list/stackable.lst")
	must(e)
	rows, e := catalog.ParseIndex(idx.Cells)
	must(e)
	var matches []map[string]any
	for _, r := range rows {
		s, e := catalog.ResolveScript(a, r.Path)
		if e != nil {
			continue
		}
		match := wanted[uint32(r.ID)]
		for _, c := range s.Cells {
			if strings.Contains(strings.ToLower(c.Text), "odyssey") {
				match = true
			}
		}
		if match {
			write(fmt.Sprintf("item-%d.json", r.ID), s)
			matches = append(matches, map[string]any{"id": r.ID, "path": r.Path})
		}
	}
	write("items.json", matches)
	var currencyRefs []string
	needle := func(id uint32) []byte { b := make([]byte, 5); binary.LittleEndian.PutUint32(b[1:], id); return b }
	for _, f := range a.Files() {
		p := strings.ToLower(f.ArchivePath)
		if f.DataType != 1 || !(strings.HasSuffix(p, ".etc") || strings.HasSuffix(p, ".cos") || strings.HasSuffix(p, ".tbl")) {
			continue
		}
		b, e := a.ReadRaw(f.ArchivePath)
		if e != nil {
			continue
		}
		if !bytes.Contains(b, needle(10418036)) && !bytes.Contains(b, needle(10418035)) {
			continue
		}
		s, e := catalog.ReadScript(a, f.ArchivePath)
		if e != nil {
			continue
		}
		currencyRefs = append(currencyRefs, s.Path)
		write("currency-ref-"+strings.ReplaceAll(p, "/", "_")+".json", s)
	}
	write("currency-refs.json", currencyRefs)
	fullLoot, e := catalog.ImportLoot(a, 150)
	must(e)
	b, e := json.MarshalIndent(fullLoot, "", "  ")
	must(e)
	must(os.WriteFile("configs/loot.level150.json", b, 0644))
	fmt.Printf("LOOT SOURCE PASS maximum_grade=150 items=%d\n", len(fullLoot.Items))
	fmt.Printf("SOURCE PASS checksum=%s paths=%d stackables=%d\n", a.Snapshot().Checksum, len(paths), len(matches))
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
