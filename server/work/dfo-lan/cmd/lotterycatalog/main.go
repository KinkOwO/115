// lotterycatalog audits lottery definitions in the current inner PVF.
package main

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
)

type itemInfo struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	StackableType string `json:"stackable_type"`
}

type itemIndex struct {
	Source struct {
		Checksum string `json:"checksum"`
	} `json:"source"`
	Items map[string]itemInfo `json:"items"`
}

type candidate struct{ Template, Weight, Count uint32 }
type pool struct {
	SourceItem         uint32      `json:"source_item"`
	SourceScript       string      `json:"source_script"`
	SourceScriptSHA256 string      `json:"source_script_sha256"`
	Candidates         []candidate `json:"candidates"`
}
type catalog struct {
	SourcePVFSHA256 string `json:"source_pvf_sha256"`
	Pools           []pool `json:"pools"`
}

func main() {
	source := flag.String("source", "", "current inner PVF")
	indexPath := flag.String("index", "", "item index")
	out := flag.String("out", "", "catalog output")
	flag.Parse()
	b, err := os.ReadFile(*indexPath)
	if err != nil {
		panic(err)
	}
	var index itemIndex
	if err := json.Unmarshal(b, &index); err != nil {
		panic(err)
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		panic(err)
	}
	if a.Snapshot().Checksum != index.Source.Checksum {
		panic("item index and PVF checksum differ")
	}
	counts := map[string]int{}
	result := catalog{SourcePVFSHA256: a.Snapshot().Checksum}
	var ids []int
	for id, info := range index.Items {
		if info.StackableType != "[upgradable legacy]" {
			continue
		}
		n, err := strconv.Atoi(id)
		if err != nil {
			panic(err)
		}
		ids = append(ids, n)
	}
	sort.Ints(ids)
	for _, id := range ids {
		info := index.Items[strconv.Itoa(id)]
		tokens, err := a.Tokens(info.Path)
		if err != nil {
			counts["read error"]++
			continue
		}
		var values []int32
		inside := false
		for _, token := range tokens {
			if token.Type == 3 && token.Text == "[int data]" {
				inside = true
				continue
			}
			if token.Type == 3 && token.Text == "[/int data]" {
				break
			}
			if inside {
				if token.Type != 0 {
					values = nil
					break
				}
				values = append(values, token.Value)
			}
		}
		if len(values) == 0 || len(values)%3 != 0 {
			counts["invalid triples"]++
			continue
		}
		p := pool{SourceItem: uint32(id), SourceScript: info.Path}
		valid := true
		hasGold := false
		hasStackable := false
		for j := 0; j < len(values); j += 3 {
			id, weight, amount := values[j], values[j+1], values[j+2]
			if id < 0 || weight <= 0 || amount <= 0 {
				valid = false
				break
			}
			if id == 0 {
				hasGold = true
			} else if index.Items[strconv.Itoa(int(id))].Kind == "stackable" {
				hasStackable = true
			} else {
				valid = false
				break
			}
			p.Candidates = append(p.Candidates, candidate{uint32(id), uint32(weight), uint32(amount)})
		}
		if !valid {
			counts["unsupported reward or nonpositive triple"]++
			continue
		}
		raw, err := a.ReadRaw(info.Path)
		if err != nil {
			panic(err)
		}
		p.SourceScriptSHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
		result.Pools = append(result.Pools, p)
		kind := "stackable"
		if hasGold && hasStackable {
			kind = "mixed"
		} else if hasGold {
			kind = "gold"
		}
		counts[kind]++
		counts["reward rows"] += len(p.Candidates)
		if id == 7772 || id == 10306598 {
			fmt.Printf("example %d %s: %v\n", id, info.Path, values[:min(len(values), 9)])
		}
	}
	if *out != "" {
		data, err := json.Marshal(result)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(*out, data, 0644); err != nil {
			panic(err)
		}
		fmt.Printf("wrote %d pools (%d bytes) to %s\n", len(result.Pools), len(data), *out)
	}
	fmt.Printf("PVF %s, upgradable legacy %d\n", a.Snapshot().Checksum, len(ids))
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%s: %d\n", k, counts[k])
	}
}
