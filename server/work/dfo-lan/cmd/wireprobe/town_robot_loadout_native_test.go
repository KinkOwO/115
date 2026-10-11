package main

import (
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Explicit offline audit only: no player store, network or live archive writes.
func TestTownRobotCurrentMallAudit(t *testing.T) {
	out := os.Getenv("DFO_ROBOT_LOADOUT_AUDIT")
	if out == "" {
		t.Skip("explicit offline current mall audit")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: `D:\115us\server\work\client-build\Script.inner.pvf`, MaxBytes: 1 << 30}, "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err = os.MkdirAll(out, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, v any) {
		raw, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(out, name), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	c, err := cashshop.ImportPilot(a)
	if err != nil {
		t.Fatal(err)
	}
	write("mall.json", c)
	refsByID := map[uint32]string{}
	for _, kind := range []string{"stackable", "equipment"} {
		idx, e := catalog.ResolveScript(a, "list/"+kind+".lst")
		if e != nil {
			t.Fatal(e)
		}
		refs, e := catalog.ParseIndex(idx.Cells)
		if e != nil {
			t.Fatal(e)
		}
		for _, ref := range refs {
			refsByID[ref.ID] = ref.Path
			if ref.ID < 930000000 {
				continue
			}
			s, e := catalog.ResolveScript(a, ref.Path)
			if e != nil {
				t.Fatal(e)
			}
			write(strconv.FormatUint(uint64(ref.ID), 10)+".json", s)
		}
	}
	queue := []uint32{930440001, 752000004, 752000005, 752000006, 750000403, 590005188, 590005189}
	seen := map[uint32]bool{}
	for depth := 0; depth < 4; depth++ {
		next := []uint32{}
		for _, id := range queue {
			if seen[id] {
				continue
			}
			seen[id] = true
			s, e := catalog.ResolveScript(a, refsByID[id])
			if e != nil {
				t.Fatal(e)
			}
			write(strconv.FormatUint(uint64(id), 10)+".json", s)
			for _, cell := range s.Cells {
				if cell.Type == 0 && cell.Value > 1000000 {
					n := uint32(cell.Value)
					if refsByID[n] != "" && !seen[n] {
						next = append(next, n)
					}
				}
			}
		}
		queue = next
	}
	chr, err := catalog.ResolveScript(a, "character/swordman/swordman.chr")
	if err != nil {
		t.Fatal(err)
	}
	write("swordman.chr.json", chr)
	err = a.IterateFilesUnder("etc/mycharactersapc", func(f pvf.File) error {
		if !strings.HasSuffix(f.ArchivePath, ".etc") {
			return nil
		}
		s, e := catalog.ResolveScript(a, f.ArchivePath)
		if e != nil {
			return e
		}
		write(filepath.Base(f.ArchivePath)+".json", s)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
