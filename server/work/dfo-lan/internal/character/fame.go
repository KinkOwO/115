package character

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
)

//go:embed fame_rules.json
var fameRulesJSON []byte

type fameSourceValue struct {
	Value      int64  `json:"value"`
	Additional int64  `json:"additional,omitempty"`
	Table      string `json:"table,omitempty"`
	Index      int    `json:"index,omitempty"`
}

type fameThreshold struct {
	Point int   `json:"point"`
	Fame  int64 `json:"fame"`
}

type fameSetPoint struct {
	Set       int  `json:"set"`
	Point     int  `json:"point"`
	Awakening byte `json:"awakening"`
}

type fameRules struct {
	Sources           map[string]string          `json:"sources"`
	Version           int                        `json:"version"`
	Source            string                     `json:"source"`
	Tables            map[string]map[int]int64   `json:"tables"`
	Refine            map[int]int                `json:"refine"`
	Refine115         map[int]int                `json:"refine_115"`
	Items             map[uint32]fameSourceValue `json:"items"`
	Sets              map[int][]fameThreshold    `json:"sets"`
	ItemPoints        map[uint32][]fameSetPoint  `json:"item_points"`
	Expanded          []fameThreshold            `json:"expanded"`
	Awakening         map[uint32]map[byte]int64  `json:"awakening"`
	MemoryRestore     map[byte]int64             `json:"memory_restore"`
	MemoryActivate    map[byte]int64             `json:"memory_activate"`
	MemoryRuminations map[byte]int64             `json:"memory_ruminations"`
	SoleQuality       map[uint32]map[byte]int64  `json:"sole_quality"`
	SolePenalty       map[uint32]map[int]int64   `json:"sole_penalty"`
}

type FameRules = fameRules

var loadEmbeddedFameRules = sync.OnceValues(func() (*fameRules, error) {
	var r fameRules
	if err := json.Unmarshal(fameRulesJSON, &r); err != nil {
		return nil, err
	}
	return NewFameRules(r)
})

func NewFameRules(r FameRules) (*FameRules, error) {
	if b, err := hex.DecodeString(r.Source); err != nil || len(b) != 32 {
		return nil, fmt.Errorf("invalid fame source identity")
	}
	if r.Version != 1 || len(r.Source) != 64 || len(r.Tables) == 0 || len(r.Refine115) != 8 ||
		len(r.Awakening) == 0 || len(r.MemoryRestore) == 0 || len(r.MemoryActivate) == 0 ||
		len(r.MemoryRuminations) == 0 || len(r.SoleQuality) == 0 || len(r.SolePenalty) == 0 {
		return nil, fmt.Errorf("内置名望规则不完整")
	}
	return &r, nil
}

var installedFameRules atomic.Pointer[FameRules]

func EmbeddedFameRules() (*FameRules, error) { return loadEmbeddedFameRules() }

func currentFameRules() (*FameRules, error) {
	if r := installedFameRules.Load(); r != nil {
		return r, nil
	}
	return loadEmbeddedFameRules()
}

func CurrentFameRules() (*FameRules, error) { return currentFameRules() }

// FameItem保留每个实际穿戴物品的计算来源，便于核对而不向角色存档写缓存值。
type FameItem struct {
	Slot      uint16 `json:"slot"`
	Template  uint32 `json:"template"`
	Base      int64  `json:"base"`
	Upgrade   int64  `json:"upgrade"`
	Enchant   int64  `json:"enchant"`
	SetPoints int    `json:"set_points"`
	Awakening int64  `json:"awakening,omitempty"`
	Memory    int64  `json:"memory,omitempty"`
	Quality   int64  `json:"quality,omitempty"`
	Penalty   int64  `json:"penalty,omitempty"`
}

type FameBreakdown struct {
	Total uint32        `json:"total"`
	Items []FameItem    `json:"items"`
	Sets  map[int]int64 `json:"sets"`
}

func fameInt(d inventory.EquipmentDefinition, key string) (int, error) {
	v := d.Fields[key]
	if len(v) == 0 {
		return 0, nil
	}
	if len(v) != 1 || v[0].Type != 0 {
		return 0, fmt.Errorf("装备 %d 的名望字段 %s 格式无效", d.ID, key)
	}
	return int(v[0].Value), nil
}

func (r *fameRules) sourceValue(fields map[string][]pvf.Token) (int64, error) {
	v := fields["[fame value]"]
	if len(v) > 0 {
		if len(v) != 1 || v[0].Type != 0 || v[0].Value < 0 {
			return 0, fmt.Errorf("基础名望格式无效")
		}
		if v[0].Value > 0 {
			return int64(v[0].Value), nil
		}
	}
	v = fields["[fame table]"]
	if len(v) == 0 {
		return 0, nil
	}
	if len(v) != 2 || v[0].Type != 6 || v[1].Type != 0 {
		return 0, fmt.Errorf("名望查表格式无效")
	}
	// 原生1473A6D50对源表中不存在的条目返回零；例如旧护石的charm表。
	return r.Tables[v[0].Text][int(v[1].Value)], nil
}

func fameCeil(n float32) float32 { return float32(math.Ceil(float64(n))) }
func famePow(base float32, exponent int) float32 {
	return float32(math.Pow(float64(base), float64(exponent)))
}

// 1473A84F0：保持原生单精度乘法和各阶段取整；115与120级分支不同。
// 常量来自当前客户端14B278FA0、14B278EF8及14B278F0C等只读数据。
func fameUpgrade(level, rarity, rank int, weapon, amplify bool) int64 {
	if level < 1 || level > 255 || rarity < 0 || rarity > 8 || rank < 1 || rank > 31 {
		return 0
	}
	weights := [...]float32{0.5, 0.55, 0.6, 0.65, 1, 0.65, 0.72, 1.25, 1.25}
	tier := float32(1)
	switch {
	case rank <= 7:
		tier = 0.2
	case rank <= 9:
		tier = 0.3
	case rank == 10:
		tier = 0.38
	case rank == 11:
		tier = 0.68
	}
	mode, part := float32(0.9), float32(0.34)
	if amplify {
		mode = 1
	}
	if weapon {
		part = 1.06
	}
	var base float32
	if level >= 120 {
		base = fameCeil(weights[rarity] * 714)
	} else if level >= 115 {
		base = 59.5
		if weapon {
			base = 74.41667175292969
		}
	} else {
		adjusted := level
		if level >= 106 && level <= 110 {
			adjusted -= 5
		}
		base = fameCeil(famePow(0.65, 21-(adjusted/5+1)) * 464)
		base = fameCeil(base * weights[rarity])
	}
	n := float32(base * float32(rank))
	n = float32(n * tier)
	if level < 115 || level >= 120 {
		n = float32(n * float32(1.0/12.0))
	}
	n = float32(n * mode)
	n = fameCeil(float32(n * part))
	if level >= 115 {
		exponent := level - 105
		if level >= 120 {
			exponent = level*2 - 220
		}
		n = float32(n * famePow(1.01, exponent))
		return int64(math.Floor(float64(n)))
	}
	return int64(n)
}

func (r *fameRules) upgrade(item inventory.BagEquipment, level, rarity int) int64 {
	if item.Slot < 12 || item.Slot > 25 || item.Slot == 13 || item.Slot == 24 {
		return 0
	}
	var reinforcement int
	var amplify bool
	if len(item.Record) > 19 {
		reinforcement = int(item.Record[10] & 31)
		amplify = item.Record[19] != 0
	}
	value := fameUpgrade(level, rarity, reinforcement, item.Slot == 12, amplify)
	// 1473A8189..1473A81D3：仅武器计算锻造，转换后与强化/增幅取较高值。
	if item.Slot == 12 {
		// 与锻造事务保持同一真源；旧记录中的未校准字节不能反向覆盖服务端等级。
		refine := int(item.Refine)
		rank := r.Refine[refine]
		if level >= 115 {
			rank = r.Refine115[refine]
		}
		value = max(value, fameUpgrade(level, rarity, rank, true, false))
	}
	return value
}

func (r *fameRules) enchant(item inventory.BagEquipment) int64 {
	if len(item.Record) < 18 {
		return 0
	}
	id := binary.LittleEndian.Uint32(item.Record[14:18])
	v, ok := r.Items[id]
	if !ok {
		return 0
	}
	if v.Value > 0 {
		return v.Value + v.Additional
	}
	return r.Tables[v.Table][v.Index] + v.Additional
}

// 1473A7EEE与147398150：普通时装的默认名望来自装扮品级，不是装备稀有度。
func fameAvatarBase(slot uint16, grade int) int64 {
	if slot > 11 || slot == 9 || slot == 10 || slot == 11 {
		return 0
	}
	rarity := 2
	if slot == 8 {
		rarity = 1
	}
	if grade == 2 {
		rarity = 3
	} else if grade == 3 {
		rarity = 4
	}
	count := [...]float32{2, 2, 2, 4, 6, 9, 0, 0}
	weight := [...]float32{1.3, 1.3, 1.3, 1.3, 1.49, 1.57, 0, 0}
	n := float32(count[rarity] * float32(7.3))
	n = float32(n * weight[rarity])
	return int64(float32(n + 51))
}

func fameThresholdValue(rows []fameThreshold, points int) int64 {
	var value int64
	threshold := -1
	for _, row := range rows {
		if row.Point <= points && row.Point > threshold {
			threshold, value = row.Point, row.Fame
		}
	}
	return value
}

// EquipmentFameBreakdown统一供选角、进城、换装与编队使用，不修改角色或物品数据。
func (s *Service) EquipmentFameBreakdown(raw json.RawMessage) (FameBreakdown, error) {
	result := FameBreakdown{Sets: map[int]int64{}}
	if s.Equipment == nil {
		return result, nil
	}
	rules, err := currentFameRules()
	if err != nil {
		return result, err
	}
	bag, err := inventory.ReadBag(raw)
	if err != nil {
		return result, err
	}
	var state struct {
		Level byte `json:"level"`
	}
	if err = json.Unmarshal(raw, &state); err != nil {
		return result, err
	}
	points := map[int]int{}
	soleItems := map[int]bool{}
	var total int64
	for _, item := range bag.WornBaseItems() {
		// 142758AC0排除副手24、30；11和32为光环/宠物幻化，不叠加本体名望。
		if item.Template == 0 || item.Template == math.MaxUint32 || item.Slot == 24 || item.Slot == 30 || item.Slot == 11 || item.Slot == 32 {
			continue
		}
		d, err := s.Equipment.FameDefinition(item.Template, state.Level)
		if err != nil {
			return result, err
		}
		if err := item.ValidateRecord(); err != nil {
			return result, err
		}
		var awakening byte
		var memory int64
		if len(item.Record) != 0 {
			// 14576D8B0：181字节记录的170映射到实例285；162/163/169
			// 分别映射到记忆激活275、还原276、追溯282。
			awakening = item.Record[170]
			memory = rules.MemoryActivate[item.Record[162]] +
				rules.MemoryRestore[item.Record[163]] + rules.MemoryRuminations[item.Record[169]]
		}
		base, err := rules.sourceValue(d.Fields)
		if err != nil {
			return result, fmt.Errorf("装备 %d：%w", item.Template, err)
		}
		level, err := fameInt(d, "[minimum level]")
		if err != nil {
			return result, err
		}
		rarity, err := fameInt(d, "[rarity]")
		if err != nil {
			return result, err
		}
		// 1473A7B10：记忆养成已有名望时不再进入旧装备/装扮的默认值分支。
		if base == 0 && memory == 0 && item.Slot <= 8 {
			grade, err := fameInt(d, "[grade]")
			if err != nil {
				return result, err
			}
			base = fameAvatarBase(item.Slot, grade)
		}
		if base == 0 && memory == 0 && item.Slot >= 12 && item.Slot <= 25 && item.Slot != 13 && level > 0 && level <= 255 && rarity >= 0 && rarity <= 8 {
			// 1473A7DCE：没有直值/查表值的旧装备，按原生等级及品质公式回退。
			weights := [...]float32{0.5, 0.55, 0.6, 0.65, 1, 0.65, 0.72, 1.25, 1.25}
			n := fameCeil(famePow(0.65, 21-(level/5+1)) * 464)
			base = int64(fameCeil(n * weights[rarity]))
		}
		additional, err := fameInt(d, "[add fame value]")
		if err != nil {
			return result, err
		}
		entry := FameItem{Slot: item.Slot, Template: item.Template, Base: base + int64(additional),
			Upgrade: rules.upgrade(item, level, rarity), Enchant: rules.enchant(item),
			Awakening: rules.Awakening[item.Template][awakening], Memory: memory}
		sole, err := fameInt(d, "[sole equipment]")
		if err != nil {
			return result, err
		}
		if sole != 0 {
			soleItems[len(result.Items)] = true
		}
		if sole != 0 && len(item.Record) != 0 {
			entry.Quality = rules.SoleQuality[item.Template][item.Record[172]]
		}
		set, err := fameInt(d, "[part set index]")
		if err != nil {
			return result, err
		}
		for _, p := range rules.ItemPoints[item.Template] {
			if p.Awakening != awakening {
				continue
			}
			id := p.Set
			if id == -1 {
				id = set
			}
			if id > 0 {
				points[id] += p.Point
				entry.SetPoints += p.Point
			}
		}
		total += entry.Base + entry.Upgrade + entry.Enchant + entry.Awakening + entry.Memory + entry.Quality
		result.Items = append(result.Items, entry)
	}
	// 145CDE7B0取最高套装积分；1473A6740按不小于积分的最小上界扣分。
	// 这是上界表，不能复用套装奖励的下界查找，也不能合并不同套装的积分。
	highestPoints := 0
	for _, point := range points {
		highestPoints = max(highestPoints, point)
	}
	for index := range soleItems {
		entry := &result.Items[index]
		upper := math.MaxInt
		for point, penalty := range rules.SolePenalty[entry.Template] {
			if point >= highestPoints && point < upper {
				upper, entry.Penalty = point, penalty
			}
		}
		total -= entry.Penalty
	}
	for set, point := range points {
		value := fameThresholdValue(rules.Sets[set], point)
		// 源SetPointInfo.cos仅为这12个常规115套装配置超过2550分的额外档位。
		if set >= 16201 && set <= 16212 {
			value += fameThresholdValue(rules.Expanded, point)
		}
		result.Sets[set] = value
		total += value
	}
	if total < 0 || total > math.MaxInt32 {
		return result, fmt.Errorf("角色名望超出客户端范围")
	}
	result.Total = uint32(total)
	return result, nil
}
