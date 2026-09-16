// questaudit records index discrepancies without assigning guessed quest IDs.
package main

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"hash/crc32"
	"log"
	"os"
	"strings"
)

func main() {
	a, e := pvf.LoadArchive(pvf.Options{Path: "runtime/pvf_source/Script.inner.pvf", MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	enc := json.NewEncoder(os.Stdout)
	for _, f := range a.Files() {
		if strings.EqualFold(f.Name, "quest.lst") {
			enc.Encode(f)
		}
	}
	for _, p := range []string{"list/quest.lst", "contents/2022/new_scenario_renewal/season_1/grandflores/quest/grandflores_01.qst"} {
		b, e := a.ReadRaw(p)
		if e != nil {
			log.Fatal(e)
		}
		ts, e := a.Tokens(p)
		if e != nil {
			log.Fatal(e)
		}
		refs := []pvf.Token{}
		for _, t := range ts {
			if t.Type == 8 {
				refs = append(refs, t)
			}
		}
		enc.Encode(map[string]any{"path": p, "crc32": crc32.ChecksumIEEE(b), "references": refs})
	}
}
