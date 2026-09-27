package main

import (
	"dfolan/internal/catalog"
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
	groupsPath := "etc/dungeondroptablebygroup.etc"

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

	groups := loadSmartDropGroups(a, groupsPath)

	result := make(map[string]BoosterDefinition)
	count := 0
	substituted := 0
	var sealed []uint32   // structural containers the type test cannot see -> entry without pools
	var unresolved []uint32 // [booster]-typed bodies this build still cannot parse -> reported, never silent
	var unreadable []uint32
	t0 := time.Now()

	for idStr, item := range idx.Items {
		if item.Kind != "stackable" || !strings.HasPrefix(item.Path, "stackable/") {
			continue
		}
		cells, err := a.Tokens(item.Path)
		if err != nil {
			// An item the archive cannot hand over is worth knowing about, but it is
			// not a container decision; only count the ones we expected to be boxes.
			if strings.Contains(item.StackableType, "booster") {
				unreadable = append(unreadable, item.ID)
			}
			continue
		}
		// Structural test first: the section is what makes a box, not the type string.
		hasInfo := declaresSection(cells, "[booster info]")
		isBooster := strings.Contains(item.StackableType, "booster")
		isPkg := strings.Contains(item.StackableType, "package")
		if !hasInfo && !isBooster && !isPkg {
			continue
		}
		pools := parseBoosterInfo(cells)
		if len(pools) == 0 && isPkg {
			pools = parsePackageData(cells)
		}
		if len(pools) > 0 {
			pools = resolveSmartDrop(pools, sectionNumber(cells, "[smart drop group id]"), groups, &substituted)
			result[idStr] = BoosterDefinition{
				Template: item.ID,
				Type:     item.StackableType,
				Pools:    pools,
			}
			count++
			continue
		}
		switch {
		case hasInfo && !isBooster:
			// A container the type test misses. It must not reach the ground, and
			// the only way the drop layer can know that is to see it here.
			result[idStr] = BoosterDefinition{Template: item.ID, Type: item.StackableType}
			sealed = append(sealed, item.ID)
		case hasInfo:
			unresolved = append(unresolved, item.ID)
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
	fmt.Printf("  smart drop groups loaded : %d (%s)\n", len(groups), groupsPath)
	fmt.Printf("  smart drop substitutions : %d\n", substituted)
	fmt.Printf("  containers sealed (no payload, invisible to the type test): %d %v\n",
		len(sealed), headU32(sealed, 12))
	fmt.Printf("  [booster] bodies not parsed by this build : %d %v\n",
		len(unresolved), headU32(unresolved, 12))
	fmt.Printf("  [booster]-typed items the archive did not hand over: %d %v\n",
		len(unreadable), headU32(unreadable, 12))
	_ = filepath.Base("")
}

// declaresSection reports whether the script declares a section header verbatim.
// This is the structural half of "is this a box": the stackable type is a label,
// the [booster info] block is the payload.
func declaresSection(cells []pvf.Token, header string) bool {
	for _, c := range cells {
		if c.Type == 3 && c.Text == header {
			return true
		}
	}
	return false
}

// sectionNumber returns the first numeric cell that follows a section header, or
// 0 when the section is absent.
func sectionNumber(cells []pvf.Token, header string) uint32 {
	for i, c := range cells {
		if c.Type == 3 && c.Text == header && i+1 < len(cells) && cells[i+1].Type == 0 {
			return uint32(cells[i+1].Value)
		}
	}
	return 0
}

// reservedTemplateLow/High bracket the ids the source spends on "not a real
// item". 490000001 is the one the booster bodies use for "the real payload comes
// from elsewhere".
const (
	reservedTemplateLow  = 490000000
	reservedTemplateHigh = 490001000
)

func isReservedTemplate(t uint32) bool {
	return t >= reservedTemplateLow && t < reservedTemplateHigh
}

// loadSmartDropGroups reads etc/dungeondroptablebygroup.etc into
// group id -> candidate rows. A table that cannot be read is reported, not
// ignored: without it, every smart drop carrier would be sealed instead of paid.
func loadSmartDropGroups(a *pvf.Archive, path string) map[uint32][]RewardCandidate {
	cells, err := a.Tokens(path)
	if err != nil {
		log.Printf("smart drop groups: %s unreadable: %v", path, err)
		return nil
	}
	gs, unreadable, err := catalog.ParseDropGroups(cells)
	if err != nil {
		log.Printf("smart drop groups: %s unparsable: %v", path, err)
		return nil
	}
	if len(unreadable) > 0 {
		// Reported, not guessed: a carrier pointing at one of these will keep its
		// reserved placeholder and be paid as nothing, which is visible in the
		// export summary instead of becoming a silently wrong distribution.
		log.Printf("smart drop groups: %d group(s) the parser refused: %v", len(unreadable), headU32(unreadable, 8))
	}
	out := make(map[uint32][]RewardCandidate, len(gs))
	for _, g := range gs {
		cands := make([]RewardCandidate, 0, len(g.Explicit)+len(g.Smart))
		for _, w := range g.Explicit {
			cands = append(cands, RewardCandidate{Template: w.Template, Weight: w.Weight, Count: 1})
		}
		for _, w := range g.Smart {
			cands = append(cands, RewardCandidate{Template: w.Template, Weight: w.Weight, Count: 1})
		}
		if len(cands) > 0 {
			out[g.ID] = cands
		}
	}
	return out
}

// resolveSmartDrop replaces a pool that pays a reserved id with the rows of the
// group the item names. A pool that pays real templates is left alone: only the
// source's placeholder is resolved.
func resolveSmartDrop(pools []RewardPool, smartID uint32, groups map[uint32][]RewardCandidate, substituted *int) []RewardPool {
	if smartID == 0 || len(groups) == 0 {
		return pools
	}
	cands, ok := groups[smartID]
	if !ok {
		return pools
	}
	out := make([]RewardPool, 0, len(pools))
	for _, p := range pools {
		placeholder := false
		for _, c := range p.Candidates {
			if isReservedTemplate(c.Template) {
				placeholder = true
			}
		}
		if !placeholder {
			out = append(out, p)
			continue
		}
		*substituted++
		out = append(out, RewardPool{DrawCount: p.DrawCount, Candidates: cands})
	}
	return out
}

func headU32(v []uint32, n int) []uint32 {
	if len(v) <= n {
		return v
	}
	return v[:n]
}
