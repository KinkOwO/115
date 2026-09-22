package main

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ItemIndexEntry struct {
	ID            uint32 `json:"id"`
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	StackableType string `json:"stackable_type"`
}

type ItemIndex struct {
	Items map[string]ItemIndexEntry `json:"items"`
}

type RewardCandidate struct {
	Template uint32 `json:"template"`
	Weight   uint32 `json:"weight"`
	Count    uint32 `json:"count"`
}

type RewardPool struct {
	DrawCount  uint32            `json:"draw_count"`
	Candidates []RewardCandidate `json:"candidates"`
}

func (p RewardPool) Pick(r *rand.Rand) []RewardCandidate {
	if len(p.Candidates) == 0 {
		return nil
	}
	// If only 1 candidate, or all weights 0 / 1000
	if len(p.Candidates) == 1 {
		return []RewardCandidate{p.Candidates[0]}
	}
	var totalWeight uint32
	for _, c := range p.Candidates {
		totalWeight += c.Weight
	}
	if totalWeight == 0 {
		totalWeight = uint32(len(p.Candidates))
	}
	draws := p.DrawCount
	if draws == 0 {
		draws = 1
	}
	var results []RewardCandidate
	for d := uint32(0); d < draws; d++ {
		roll := r.Uint32() % totalWeight
		var acc uint32
		picked := false
		for _, c := range p.Candidates {
			w := c.Weight
			if c.Weight == 0 {
				w = 1
			}
			acc += w
			if roll < acc {
				results = append(results, c)
				picked = true
				break
			}
		}
		if !picked {
			results = append(results, p.Candidates[len(p.Candidates)-1])
		}
	}
	return results
}

type BoosterDefinition struct {
	Template uint32       `json:"template"`
	Type     string       `json:"type"` // "[booster]" or "[booster selection]" or package type
	Pools    []RewardPool `json:"pools,omitempty"`
}

func parseBoosterInfo(cells []pvf.Token) []RewardPool {
	var pools []RewardPool
	for i := 0; i < len(cells); i++ {
		c := cells[i]
		if c.Type == 3 && c.Text == "[booster info]" {
			j := i + 1
			for j < len(cells) && !(cells[j].Type == 3 && cells[j].Text == "[/booster info]") {
				tag := cells[j]
				if tag.Type == 3 && (tag.Text == "[etc]" || tag.Text == "[equipment]" || tag.Text == "[avatar]" ||
					tag.Text == "[stackable]" || tag.Text == "[creature]" || tag.Text == "[cera]" ||
					tag.Text == "[grow type]" || tag.Text == "[dungeon and life items]" || tag.Text == "[special avatar]" ||
					tag.Text == "[equipment random option]" || tag.Text == "[emblem]" || tag.Text == "[card upgrade]" ||
					tag.Text == "[enchant card]") {
					endTag := "[/" + tag.Text[1:]
					k := j + 1
					var nums []uint32
					for k < len(cells) && !(cells[k].Type == 3 && cells[k].Text == endTag) {
						if cells[k].Type == 0 && cells[k].Value > 0 {
							nums = append(nums, uint32(cells[k].Value))
						}
						k++
					}
					if len(nums) > 0 {
						drawCount := uint32(1)
						start := 0
						if len(nums) >= 4 && (len(nums)-1)%3 == 0 {
							drawCount = nums[0]
							start = 1
						}
						var cands []RewardCandidate
						if (len(nums)-start)%3 == 0 {
							for idx := start; idx+2 < len(nums); idx += 3 {
								cands = append(cands, RewardCandidate{
									Template: nums[idx],
									Weight:   nums[idx+1],
									Count:    nums[idx+2],
								})
							}
						} else if len(nums)%2 == 0 {
							for idx := 0; idx+1 < len(nums); idx += 2 {
								cands = append(cands, RewardCandidate{
									Template: nums[idx],
									Weight:   1000,
									Count:    nums[idx+1],
								})
							}
						}
						if len(cands) > 0 {
							pools = append(pools, RewardPool{
								DrawCount:  drawCount,
								Candidates: cands,
							})
						}
					}
					j = k
				} else {
					j++
				}
			}
			break
		}
	}
	return pools
}

func parsePackageData(cells []pvf.Token) []RewardPool {
	var pools []RewardPool
	for i := 0; i < len(cells); i++ {
		c := cells[i]
		if c.Type == 3 && c.Text == "[package data]" {
			j := i + 1
			var nums []uint32
			for j < len(cells) && !(cells[j].Type == 3 && cells[j].Text == "[/package data]") {
				if cells[j].Type == 0 && cells[j].Value > 0 {
					nums = append(nums, uint32(cells[j].Value))
				}
				j++
			}
			if len(nums) >= 2 && len(nums)%2 == 0 {
				for idx := 0; idx+1 < len(nums); idx += 2 {
					pools = append(pools, RewardPool{
						DrawCount: 1,
						Candidates: []RewardCandidate{
							{
								Template: nums[idx],
								Weight:   1000,
								Count:    nums[idx+1],
							},
						},
					})
				}
			}
			break
		}
	}
	return pools
}

func main() {
	pvfPath := "runtime/pvf_source/Script.inner.pvf"
	indexPath := "configs/items.index.json"
	outPath := "configs/booster-catalog.json"

	data, err := os.ReadFile(indexPath)
	if err != nil {
		log.Fatalf("read item index: %v", err)
	}
	var idx ItemIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		log.Fatalf("unmarshal item index: %v", err)
	}

	a, err := pvf.LoadArchive(pvf.Options{Path: pvfPath, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		a, err = pvf.LoadArchive(pvf.Options{Path: "../client-build/Script.inner.pvf", MaxBytes: 1024 * 1024 * 1024})
		if err != nil {
			log.Fatalf("load pvf: %v", err)
		}
	}

	result := make(map[string]BoosterDefinition)
	count := 0
	t0 := time.Now()

	for idStr, item := range idx.Items {
		isBooster := strings.Contains(item.StackableType, "booster")
		isPkg := strings.Contains(item.StackableType, "package")
		if !isBooster && !isPkg {
			continue
		}
		cells, err := a.Tokens(item.Path)
		if err != nil {
			continue
		}
		pools := parseBoosterInfo(cells)
		if len(pools) == 0 && isPkg {
			pools = parsePackageData(cells)
		}
		if len(pools) > 0 {
			result[idStr] = BoosterDefinition{
				Template: item.ID,
				Type:     item.StackableType,
				Pools:    pools,
			}
			count++
		}
	}

	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(outPath, b, 0644); err != nil {
		log.Fatalf("write: %v", err)
	}

	fmt.Printf("Exported %d booster definitions to %s in %v\n", count, outPath, time.Since(t0))
	_ = filepath.Base("")
}
