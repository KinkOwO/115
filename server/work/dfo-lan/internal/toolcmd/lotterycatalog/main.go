// lotterycatalog audits lottery definitions in the current inner PVF.
package lotterycatalog

import (
	"crypto/sha256"
	sourcecatalog "dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
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
type compactPool struct {
	SourceItem         uint32      `json:"source_item"`
	SourceScript       string      `json:"source_script"`
	SourceScriptSHA256 string      `json:"source_script_sha256"`
	Candidates         [][3]uint32 `json:"candidates"`
}
type compactCatalog struct {
	SourcePVFSHA256 string        `json:"source_pvf_sha256"`
	Pools           []compactPool `json:"pools"`
}

func Run() {
	source := flag.String("source", "", "current inner PVF")
	flag.String("index", "", "deprecated; native item index is always used")
	out := flag.String("out", "", "catalog output")
	outEquipment := flag.String("out-equipment", "", "equipment-containing lottery pools")
	equipmentFull := flag.String("equipment-full", "", "full equipment catalog prefix for grantability checks")
	equipmentCurrent := flag.String("equipment-current", "", "current equipment catalog for grantability checks")
	flag.Parse()
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		panic(err)
	}
	defer a.Close()
	nativeIndex, err := sourcecatalog.ImportItemIndex(a)
	if err != nil {
		panic(err)
	}
	var index itemIndex
	index.Source.Checksum = nativeIndex.Source.Checksum
	index.Items = make(map[string]itemInfo, len(nativeIndex.Items))
	for id, it := range nativeIndex.Items {
		index.Items[strconv.FormatUint(uint64(id), 10)] = itemInfo{Path: it.Path, Kind: it.Kind, StackableType: it.StackableType}
	}
	counts := map[string]int{}
	result := catalog{SourcePVFSHA256: a.Snapshot().Checksum}
	equipmentResult := catalog{SourcePVFSHA256: a.Snapshot().Checksum}
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
		values := sourcecatalog.ParseLotteryCells(tokens)
		if len(values) == 0 || len(values)%3 != 0 {
			counts["invalid triples"]++
			continue
		}
		p := pool{SourceItem: uint32(id), SourceScript: info.Path}
		if *outEquipment != "" {
			equipmentPool := pool{SourceItem: uint32(id), SourceScript: info.Path}
			hasEquipment := false
			eligible := true
			for j := 0; j < len(values); j += 3 {
				template, weight, amount := values[j], values[j+1], values[j+2]
				if template < 0 || weight <= 0 || amount <= 0 {
					eligible = false
					break
				}
				if template != 0 {
					reward := index.Items[strconv.Itoa(int(template))]
					switch reward.Kind {
					case "stackable":
					case "avatar":
						if amount != 1 {
							eligible = false
						}
						hasEquipment = true
					case "equipment":
						if amount != 1 || strings.Contains(reward.Path, "equipment/creature/") {
							eligible = false
						}
						hasEquipment = true
					default:
						eligible = false
					}
				}
				if !eligible {
					break
				}
				equipmentPool.Candidates = append(equipmentPool.Candidates, candidate{uint32(template), uint32(weight), uint32(amount)})
			}
			if eligible && hasEquipment {
				raw, err := a.ReadRaw(info.Path)
				if err != nil {
					panic(err)
				}
				equipmentPool.SourceScriptSHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
				equipmentResult.Pools = append(equipmentResult.Pools, equipmentPool)
				counts["equipment pools"]++
				counts["equipment pool reward rows"] += len(equipmentPool.Candidates)
			}
		}
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
	if *outEquipment != "" {
		if *equipmentFull != "" || *equipmentCurrent != "" {
			full, err := inventory.OpenPVFEquipmentCatalog(a, nativeIndex)
			if err != nil {
				panic(err)
			}
			defer full.Close()
			gear := &inventory.EquipmentCatalog{Full: full}
			grantable := make(map[uint32]bool)
			checked := make(map[uint32]bool)
			failure := make(map[uint32]string)
			avatarTypes := make(map[int32]int)
			var filtered []pool
			for _, p := range equipmentResult.Pools {
				valid := true
				for _, r := range p.Candidates {
					info := index.Items[strconv.Itoa(int(r.Template))]
					if info.Kind == "avatar" {
						if !checked[r.Template] {
							tokens, tokenErr := a.Tokens(info.Path)
							grantable[r.Template] = false
							if tokenErr == nil {
								for i := 0; i+2 < len(tokens); i++ {
									if tokens[i].Type == 3 && tokens[i].Text == "[equipment type]" && tokens[i+1].Type == 6 && tokens[i+2].Type == 0 {
										avatarTypes[tokens[i+2].Value]++
										grantable[r.Template] = tokens[i+2].Value >= 0 && tokens[i+2].Value <= 11
										break
									}
								}
							}
							if !grantable[r.Template] {
								failure[r.Template] = "avatar equipment type unresolved or outside 0..11"
							}
							checked[r.Template] = true
						}
					} else if info.Kind == "equipment" {
						if !checked[r.Template] {
							_, rewardErr := gear.Reward(r.Template)
							_, kindErr := gear.RewardType(r.Template)
							grantable[r.Template] = rewardErr == nil && kindErr == nil
							if !grantable[r.Template] {
								failure[r.Template] = fmt.Sprintf("reward=%v kind=%v", rewardErr, kindErr)
							}
							checked[r.Template] = true
						}
					} else {
						continue
					}
					if !grantable[r.Template] {
						if counts["equipment pools with unresolved grants"] < 8 || p.SourceItem == 7213 {
							fmt.Printf("unresolved source=%d reward=%d %s\n", p.SourceItem, r.Template, failure[r.Template])
						}
						valid = false
						break
					}
				}
				if valid {
					filtered = append(filtered, p)
				} else {
					counts["equipment pools with unresolved grants"]++
				}
			}
			equipmentResult.Pools = filtered
			fmt.Printf("equipment grantability checked %d distinct templates; 7213 retained=%v\n", len(checked), func() bool {
				for _, p := range filtered {
					if p.SourceItem == 7213 {
						return true
					}
				}
				return false
			}())
			fmt.Printf("avatar numeric equipment types: %v\n", avatarTypes)
		}
		compact := compactCatalog{SourcePVFSHA256: equipmentResult.SourcePVFSHA256}
		for _, p := range equipmentResult.Pools {
			row := compactPool{SourceItem: p.SourceItem, SourceScript: p.SourceScript, SourceScriptSHA256: p.SourceScriptSHA256}
			for _, c := range p.Candidates {
				row.Candidates = append(row.Candidates, [3]uint32{c.Template, c.Weight, c.Count})
			}
			compact.Pools = append(compact.Pools, row)
		}
		data, err := json.Marshal(compact)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(*outEquipment, data, 0644); err != nil {
			panic(err)
		}
		fmt.Printf("wrote %d equipment pools (%d bytes) to %s\n", len(equipmentResult.Pools), len(data), *outEquipment)
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
