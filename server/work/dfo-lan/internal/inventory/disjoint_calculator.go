package inventory

import (
	"dfolan/internal/catalog/pvf"
	"math"
	"math/rand"
	"time"
)

// Disjoint item IDs
const (
	DarkCubeFragmentID  uint32 = 3033
	LightCubeFragmentID uint32 = 3034
	FireCubeFragmentID  uint32 = 3035
	WaterCubeFragmentID uint32 = 3036
	GoldCubeFragmentID  uint32 = 3262

	LowGradeElementalCrystalID  uint32 = 3166
	HighGradeElementalCrystalID uint32 = 3167

	CommonSoulID    uint32 = 10100115
	UncommonSoulID  uint32 = 10100116
	RareSoulID      uint32 = 10099773
	UniqueSoulID    uint32 = 10099774
	EpicSoulID      uint32 = 10158124
	LegendarySoulID uint32 = 10099775
	MythicSoulID    uint32 = 10358207

	Soul115RareID      uint32 = 10361512
	Soul115UniqueID    uint32 = 10361513
	Soul115LegendaryID uint32 = 10361514
	Soul115EpicID      uint32 = 10361515
	Soul115SacredID    uint32 = 10361516
)

type DisjointEquipmentInfo struct {
	Template     uint32
	Rarity       int32
	MinimumLevel int32
	Value        int32
	Durability   int32
	Impossible   bool
}

func ExtractDisjointEquipmentInfo(d EquipmentDefinition) DisjointEquipmentInfo {
	info := DisjointEquipmentInfo{
		Template:     d.ID,
		Rarity:       0,
		MinimumLevel: 1,
		Value:        1000,
		Durability:   0,
		Impossible:   false,
	}
	if d.Fields == nil {
		return info
	}
	if _, ok := d.Fields["[impossible disjoint]"]; ok {
		info.Impossible = true
	}
	if toks, ok := d.Fields["[rarity]"]; ok && len(toks) > 0 && toks[0].Type == 0 {
		info.Rarity = toks[0].Value
	}
	if toks, ok := d.Fields["[minimum level]"]; ok && len(toks) > 0 && toks[0].Type == 0 {
		info.MinimumLevel = toks[0].Value
	} else if toks, ok := d.Fields["[grade]"]; ok && len(toks) > 0 && toks[0].Type == 0 {
		info.MinimumLevel = toks[0].Value
	}
	if toks, ok := d.Fields["[value]"]; ok && len(toks) > 0 && toks[0].Type == 0 && toks[0].Value > 0 {
		info.Value = toks[0].Value
	} else if toks, ok := d.Fields["[price]"]; ok && len(toks) > 0 && toks[0].Type == 0 && toks[0].Value > 0 {
		info.Value = toks[0].Value
	}
	if toks, ok := d.Fields["[durability]"]; ok && len(toks) > 0 && toks[0].Type == 0 {
		info.Durability = toks[0].Value
	}
	return info
}

type DisjointAdditionalConst struct {
	Divisor1 float64
	Divisor2 float64
	Chance1  float64
}

type DisjointExpandConst struct {
	Divisor      float64
	GreatChance  float64
	NormalChance float64
}

type DisjointRuleGroup struct {
	MinLevel              int32
	MaxLevel              int32
	CubeBase              float64
	CubeRarityMultipliers [9]float64
	AdditionalResult      [9][]uint32
	AdditionalConst       [9]DisjointAdditionalConst
	ExpandResult          [9][]uint32
	ExpandConst           [9]DisjointExpandConst
}

// DisjointRuleGroups 提取自 115 级客户端 Script.pvf / etc/disjoint.etc。
var DisjointRuleGroups = []DisjointRuleGroup{
	// Group 0: Level 0..114
	{
		MinLevel: 0,
		MaxLevel: 115,
		CubeBase: 150.0,
		CubeRarityMultipliers: [9]float64{
			1.12, // 0: Common
			1.35, // 1: Uncommon
			0.07, // 2: Rare
			1.48, // 3: Unique
			1.80, // 4: Epic
			0.70, // 5: Chronicle
			1.53, // 6: Legendary
			1.80, // 7: 7
			1.80, // 8: Mythic
		},
		AdditionalResult: [9][]uint32{
			0: nil,
			1: {DarkCubeFragmentID, LightCubeFragmentID, FireCubeFragmentID, WaterCubeFragmentID},
			2: {LowGradeElementalCrystalID},
			3: {HighGradeElementalCrystalID},
			4: nil,
			5: {3229},
			6: {3332},
			7: nil,
			8: nil,
		},
		AdditionalConst: [9]DisjointAdditionalConst{
			0: {},
			1: {Divisor1: 1.2, Divisor2: 34.8, Chance1: 8.0},
			2: {Divisor1: 200.0, Divisor2: 200.0, Chance1: 8.0},
			3: {Divisor1: 0.38, Divisor2: 11.3, Chance1: 8.0},
			4: {},
			5: {Divisor1: 1.7, Divisor2: 7.3, Chance1: 6.0},
			6: {Divisor1: 0.25, Divisor2: 7.5, Chance1: 8.0},
			7: {},
			8: {},
		},
		ExpandResult: [9][]uint32{
			0: {CommonSoulID},
			1: {UncommonSoulID},
			2: {RareSoulID},
			3: {UniqueSoulID},
			4: {EpicSoulID},
			5: nil,
			6: {LegendarySoulID},
			7: {490023249},
			8: {MythicSoulID},
		},
		ExpandConst: [9]DisjointExpandConst{
			0: {Divisor: 25.0, GreatChance: 8.7, NormalChance: 100.0},
			1: {Divisor: 25.0, GreatChance: 8.7, NormalChance: 100.0},
			2: {Divisor: 20.0, GreatChance: 50.0, NormalChance: 100.0},
			3: {Divisor: 17.0, GreatChance: 0.19, NormalChance: 100.0},
			4: {Divisor: 9999.0, GreatChance: 0.19, NormalChance: 100.0},
			5: {},
			6: {Divisor: 16.0, GreatChance: 0.19, NormalChance: 100.0},
			7: {Divisor: 9999.0, GreatChance: 0.19, NormalChance: 100.0},
			8: {Divisor: 9999.0, GreatChance: 0.19, NormalChance: 100.0},
		},
	},
	// Group 1: Level 115..1000
	{
		MinLevel: 115,
		MaxLevel: 1000,
		CubeBase: 150.0,
		CubeRarityMultipliers: [9]float64{
			1.12, 1.08, 0.96, 1.18, 1.44, 0.56, 1.22, 1.44, 1.44,
		},
		AdditionalResult: [9][]uint32{
			0: nil,
			1: {DarkCubeFragmentID, LightCubeFragmentID, FireCubeFragmentID, WaterCubeFragmentID},
			2: {LowGradeElementalCrystalID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID, ClearCubeFragmentID},
			3: {HighGradeElementalCrystalID, DarkCubeFragmentID, LightCubeFragmentID, FireCubeFragmentID, WaterCubeFragmentID},
			4: nil,
			5: nil,
			6: nil,
			7: nil,
			8: nil,
		},
		AdditionalConst: [9]DisjointAdditionalConst{
			0: {},
			1: {Divisor1: 0.3, Divisor2: 8.7, Chance1: 8.0},
			2: {Divisor1: 999.0, Divisor2: 999.0, Chance1: 8.0},
			3: {Divisor1: 999.0, Divisor2: 999.0, Chance1: 8.0},
			4: {},
			5: {},
			6: {},
			7: {},
			8: {},
		},
		ExpandResult: [9][]uint32{
			0: nil,
			1: nil,
			2: {10362396, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID, Soul115RareID},
			3: {10362397, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID, Soul115UniqueID},
			4: {10362399, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID, Soul115EpicID},
			5: nil,
			6: {10362398, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID, Soul115LegendaryID},
			7: nil,
			8: {10362400, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID, Soul115SacredID},
		},
		ExpandConst: [9]DisjointExpandConst{
			0: {},
			1: {},
			2: {Divisor: 9999.0, GreatChance: 0.19, NormalChance: 100.0},
			3: {Divisor: 9999.0, GreatChance: 0.19, NormalChance: 100.0},
			4: {Divisor: 9999.0, GreatChance: 0.19, NormalChance: 100.0},
			5: {},
			6: {Divisor: 9999.0, GreatChance: 0.19, NormalChance: 100.0},
			7: {},
			8: {Divisor: 9999.0, GreatChance: 0.19, NormalChance: 100.0},
		},
	},
}

type DisjointRewardItem struct {
	Template uint32
	Count    uint32
}

func CalculateDisjointRewards(info DisjointEquipmentInfo, rng *rand.Rand) []DisjointRewardItem {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	var group *DisjointRuleGroup
	for i := range DisjointRuleGroups {
		g := &DisjointRuleGroups[i]
		if info.MinimumLevel >= g.MinLevel && info.MinimumLevel < g.MaxLevel {
			group = g
			break
		}
	}
	if group == nil {
		group = &DisjointRuleGroups[0]
	}

	rarity := info.Rarity
	if rarity < 0 {
		rarity = 0
	} else if rarity > 8 {
		rarity = 8
	}

	// 1. 基础无色小晶块产出 (sell_rate = 200/1000 = 20%)
	sellGold := (int64(info.Value) * 200) / 1000
	if sellGold < 1 {
		sellGold = 1
	}

	cubeBase := group.CubeBase
	if cubeBase <= 0 {
		cubeBase = 150.0
	}
	multiplier := group.CubeRarityMultipliers[rarity]
	cubeCount := uint32(math.Max(1, math.Floor(float64(sellGold)*multiplier/cubeBase)))

	var rewards []DisjointRewardItem
	rewards = append(rewards, DisjointRewardItem{
		Template: ClearCubeFragmentID,
		Count:    cubeCount,
	})

	// 2. 附加产物 (彩色小晶块、元素结晶等)
	candidates := group.AdditionalResult[rarity]
	if len(candidates) > 0 {
		consts := group.AdditionalConst[rarity]
		divisor := consts.Divisor2
		if rng.Float64()*100.0 < consts.Chance1 && consts.Divisor1 > 0 {
			divisor = consts.Divisor1
		}
		if divisor <= 0 {
			divisor = 1.0
		}
		count := uint32(math.Max(1, math.Trunc(float64(info.MinimumLevel)/divisor)))
		pickedItem := candidates[rng.Intn(len(candidates))]
		rewards = append(rewards, DisjointRewardItem{
			Template: pickedItem,
			Count:    count,
		})
	}

	// 3. 灵魂产物 (Common/Uncommon/Rare/Unique/Epic/Legendary/Mythic 灵魂)
	expandCandidates := group.ExpandResult[rarity]
	if len(expandCandidates) > 0 {
		consts := group.ExpandConst[rarity]
		if rng.Float64()*100.0 < consts.NormalChance {
			divisor := consts.Divisor
			count := uint32(1)
			if divisor > 0 {
				exact := float64(max(1, int(info.MinimumLevel))) / divisor
				base := math.Floor(exact)
				c := uint32(base)
				if rng.Float64() < exact-base {
					c++
				}
				if c > 1 {
					count = c
				}
			}
			if rng.Float64()*100.0 < consts.GreatChance {
				count++
			}
			pickedSoul := expandCandidates[rng.Intn(len(expandCandidates))]
			rewards = append(rewards, DisjointRewardItem{
				Template: pickedSoul,
				Count:    count,
			})
		}
	}

	return rewards
}

// 保证引入 pvf 包用于类型系统
var _ pvf.Token
