// equipmentfullimport builds a separately indexed, compressed wear catalog.
// The quest/drop catalog is never modified or used as an import filter.
package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
)

func main() {
	source := flag.String("source", "", "read-only PVF")
	output := flag.String("output", "configs/equipment-full", "output prefix")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	idx, e := catalog.ResolveScript(a, "list/equipment.lst")
	if e != nil {
		log.Fatal(e)
	}
	rows, e := catalog.ParseIndex(idx.Cells)
	if e != nil {
		log.Fatal(e)
	}
	type location struct {
		Offset int64
		Size   int
		SHA256 string
	}
	index := struct {
		Source      pvf.ArchiveSnapshot
		IndexSHA256 string
		Records     map[uint32]location
		Errors      map[uint32]string
	}{a.Snapshot(), idx.SHA256, map[uint32]location{}, map[uint32]string{}}
	f, e := os.Create(*output + ".data.tmp")
	if e != nil {
		log.Fatal(e)
	}
	var offset int64
	for i, r := range rows {
		name := r.Path
		if !strings.HasPrefix(name, "equipment/") {
			name = path.Join("equipment", name)
		}
		s, err := catalog.ResolveScript(a, name)
		if err != nil {
			index.Errors[r.ID] = err.Error()
			continue
		}
		raw, err := json.Marshal(s)
		if err != nil {
			log.Fatal(err)
		}
		var packed bytes.Buffer
		z := zlib.NewWriter(&packed)
		if _, err = z.Write(raw); err != nil {
			log.Fatal(err)
		}
		if err = z.Close(); err != nil {
			log.Fatal(err)
		}
		data := packed.Bytes()
		if _, err = f.Write(data); err != nil {
			log.Fatal(err)
		}
		index.Records[r.ID] = location{offset, len(data), fmt.Sprintf("%x", sha256.Sum256(data))}
		offset += int64(len(data))
		if i%10000 == 0 {
			log.Printf("equipment %d/%d exported%d missing%d", i, len(rows), len(index.Records), len(index.Errors))
		}
	}
	if e = f.Sync(); e != nil {
		log.Fatal(e)
	}
	if e = f.Close(); e != nil {
		log.Fatal(e)
	}
	b, e := json.Marshal(index)
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*output+".index.tmp", b, 0600); e != nil {
		log.Fatal(e)
	}
	if e = os.Rename(*output+".data.tmp", *output+".data"); e != nil {
		log.Fatal(e)
	}
	if e = os.Rename(*output+".index.tmp", *output+".index.json"); e != nil {
		log.Fatal(e)
	}
	log.Printf("DONE equipment indexed%d missing%d bytes%d", len(index.Records), len(index.Errors), offset)
}
