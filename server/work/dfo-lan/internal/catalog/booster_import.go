package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
)

type BoosterRewardCandidate struct {
	Template uint32 `json:"template"`
	Weight   uint32 `json:"weight"`
	Count    uint32 `json:"count"`
}

type BoosterRewardPool struct {
	DrawCount  uint32                   `json:"draw_count"`
	Candidates []BoosterRewardCandidate `json:"candidates"`
}

func (p BoosterRewardPool) Pick(r *rand.Rand) []BoosterRewardCandidate {
	if len(p.Candidates) == 0 {
		return nil
	}
	// If only 1 candidate, or all weights 0 / 1000
	if len(p.Candidates) == 1 {
		return []BoosterRewardCandidate{p.Candidates[0]}
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
	var results []BoosterRewardCandidate
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
	Template uint32              `json:"template"`
	Type     string              `json:"type"` // "[booster]" or "[booster selection]" or package type
	Pools    []BoosterRewardPool `json:"pools,omitempty"`
	// InstantlyOpen 是源自己的 `[instantly open]` 段：**服务端代开**的标记。
	//
	// 它把 booster 分成两类，判据完全来自源：
	//   有标记  ⇒ 即开：玩家拿到的应当是内容物，所以掉落时由服务端展开（OpenRewardBoxes）
	//   没有    ⇒ 玩家自己开：应当**原样落地**，玩家在客户端打开（还能看到它自己的抽奖演出，
	//            这类模板常伴 `[oath item booster]` / `[lottery ani info]` / `[mulit open limit]`）
	// 见 next176 §19.3。零值 = 没有标记，与「目录由旧 JSON 生成」的情形一致。
	InstantlyOpen bool `json:"instantly_open,omitempty"`
}

func parseBoosterInfo(cells []pvf.Token) []BoosterRewardPool {
	var pools []BoosterRewardPool
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
						if cells[k].Type == 0 && cells[k].Value != 0 {
							nums = append(nums, boosterNumber(cells[k].Value))
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
						var cands []BoosterRewardCandidate
						if (len(nums)-start)%3 == 0 {
							for idx := start; idx+2 < len(nums); idx += 3 {
								cands = append(cands, BoosterRewardCandidate{
									Template: nums[idx],
									Weight:   nums[idx+1],
									Count:    nums[idx+2],
								})
							}
						} else if len(nums)%2 == 0 {
							for idx := 0; idx+1 < len(nums); idx += 2 {
								cands = append(cands, BoosterRewardCandidate{
									Template: nums[idx],
									Weight:   1000,
									Count:    nums[idx+1],
								})
							}
						}
						if len(cands) > 0 {
							pools = append(pools, BoosterRewardPool{
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

func parsePackageData(cells []pvf.Token) []BoosterRewardPool {
	var pools []BoosterRewardPool
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
					pools = append(pools, BoosterRewardPool{
						DrawCount: 1,
						Candidates: []BoosterRewardCandidate{
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

// BoosterNoDropTemplate 是 [booster info] 池里 `-1` 那一项在服务端的表示：
// 这次抽取**什么也不给**。
//
// 源里它的权重往往是主流（例：大深渊固定盒 10419725/10419728 的
// `[etc] 1 -1 911200 1 10420672 60000 1 …` 就是 91.12% 空），
// 所以它必须留在候选表里参与掷骰 —— 它占的正是「本次没有」那份概率。
//
// ⚠️ 这个值**必须是物品目录解析不出来的**：付费侧靠「目录不认识它」把它当空面丢掉
// （见 internal/loot/reward_box.go 的 Item 分支，以及 cmd/wireprobe 的开箱路径）。
const BoosterNoDropTemplate = uint32(0xFFFFFFFF)

// IsBoosterNoDrop 判断一次抽中的是不是源里的「本次没有」。
func IsBoosterNoDrop(t uint32) bool { return t == BoosterNoDropTemplate }

// boosterNumber 把 [booster info] 里的一个数值读成候选表用的 uint32。
//
// ⚠️ `-1` 必须**保留**，不能像以前那样被 `> 0` 过滤掉。池的正文是
//
//	<drawCount> [ <template> <weight> <count> ] …
//
// 丢掉 -1 会让三元组整体错位一格 ⇒ `drawCount` 被当成**模板**、权重被当成**数量**，
// 于是候选表里凭空长出 `1` / `6` / `12` 这类**根本不是奖励**的 id，而且它们拿的是
// 「本次没有」那份最高权重。实机表现：每次通关地上都多出
// 复活币(template 1 = stackable/coin.stk) 与 金库升级道具(template 6 =
// stackable/cash/store_silver.stk)，而且真正的 `[draw count]`（6/12 次）被静默降成 1 次，
// 誓约那条线大面积少发。2026-10-07 定位。
//
// 0 仍然过滤：源码只在段与段之间的分隔位置写 0，不属于池正文。
func boosterNumber(v int32) uint32 {
	if v < 0 {
		return BoosterNoDropTemplate
	}
	return uint32(v)
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
func loadSmartDropGroups(a *pvf.Archive, path string) map[uint32][]BoosterRewardCandidate {
	cells, err := a.Tokens(path)
	if err != nil {
		log.Printf("smart drop groups: %s unreadable: %v", path, err)
		return nil
	}
	gs, unreadable, err := ParseDropGroups(cells)
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
	out := make(map[uint32][]BoosterRewardCandidate, len(gs))
	for _, g := range gs {
		cands := make([]BoosterRewardCandidate, 0, len(g.Explicit)+len(g.Smart))
		for _, w := range g.Explicit {
			cands = append(cands, BoosterRewardCandidate{Template: w.Template, Weight: w.Weight, Count: 1})
		}
		for _, w := range g.Smart {
			cands = append(cands, BoosterRewardCandidate{Template: w.Template, Weight: w.Weight, Count: 1})
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
func resolveSmartDrop(pools []BoosterRewardPool, smartID uint32, groups map[uint32][]BoosterRewardCandidate, substituted *int) []BoosterRewardPool {
	if smartID == 0 || len(groups) == 0 {
		return pools
	}
	cands, ok := groups[smartID]
	if !ok {
		return pools
	}
	out := make([]BoosterRewardPool, 0, len(pools))
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
		out = append(out, BoosterRewardPool{DrawCount: p.DrawCount, Candidates: cands})
	}
	return out
}

func headU32(v []uint32, n int) []uint32 {
	if len(v) <= n {
		return v
	}
	return v[:n]
}

// ImportBoosters retains the exporter's fixed reward projection and sealed
// container markers. It does not change draw or empty-reward semantics.
func ImportBoosters(a *pvf.Archive, index ItemIndex) (map[uint32]BoosterDefinition, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("booster/PVF source mismatch")
	}
	const groupPath = "etc/dungeondroptablebygroup.etc"
	if _, err := a.Tokens(groupPath); err != nil {
		return nil, err
	}
	groups := loadSmartDropGroups(a, groupPath)
	result := map[uint32]BoosterDefinition{}
	substituted, sealed, unresolved := 0, 0, 0
	for i, id := range sortedItemIDs(index) {
		item := index.Items[id]
		if item.Kind != "stackable" || !strings.HasPrefix(item.Path, "stackable/") {
			continue
		}
		// Exported definitions bind the exact listed path, not a namesake variant.
		cells, err := a.Tokens(item.Path)
		if err != nil {
			return nil, fmt.Errorf("booster scan %d: %w", id, err)
		}
		hasInfo := declaresSection(cells, "[booster info]")
		isBooster := strings.Contains(item.StackableType, "booster")
		isPkg := strings.Contains(item.StackableType, "package")
		if hasInfo || isBooster || isPkg {
			pools := parseBoosterInfo(cells)
			if len(pools) == 0 && isPkg {
				pools = parsePackageData(cells)
			}
			if len(pools) > 0 {
				pools = resolveSmartDrop(pools, sectionNumber(cells, "[smart drop group id]"), groups, &substituted)
				result[id] = BoosterDefinition{
					Template:      id,
					Type:          item.StackableType,
					Pools:         pools,
					InstantlyOpen: declaresSection(cells, "[instantly open]"),
				}
			} else if hasInfo && !isBooster {
				result[id] = BoosterDefinition{Template: id, Type: item.StackableType}
				sealed++
			} else if hasInfo {
				unresolved++
			}
		}
		if i%4096 == 4095 {
			a.ReleaseReadCaches()
		}
	}
	log.Printf("PVF booster projection: %d definitions, %d smart substitutions, %d sealed markers, %d unparsed booster bodies", len(result), substituted, sealed, unresolved)
	return result, nil
}

// BoosterCatalog holds box definitions and their item metadata.
type BoosterCatalog struct {
	Definitions map[uint32]BoosterDefinition
	Items       map[uint32]ItemIndexEntry
}

func LoadBoosterCatalog(catPath, indexPath string) (*BoosterCatalog, error) {
	cat := &BoosterCatalog{
		Definitions: make(map[uint32]BoosterDefinition),
		Items:       make(map[uint32]ItemIndexEntry),
	}

	if catPath != "" {
		data, err := os.ReadFile(catPath)
		if err == nil {
			var raw map[string]BoosterDefinition
			if err = json.Unmarshal(data, &raw); err == nil {
				for _, def := range raw {
					cat.Definitions[def.Template] = def
				}
			}
		}
	}

	if indexPath != "" {
		data, err := os.ReadFile(indexPath)
		if err == nil {
			var raw struct {
				Items map[string]ItemIndexEntry `json:"items"`
			}
			if err = json.Unmarshal(data, &raw); err == nil {
				for _, it := range raw.Items {
					cat.Items[it.ID] = it
				}
			}
		}
	}

	return cat, nil
}
