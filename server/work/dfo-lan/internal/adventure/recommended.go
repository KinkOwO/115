package adventure

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

//go:embed recommended_rules.json
var recommendedRulesJSON []byte

type RecommendedRules struct {
	SourceChecksum       string               `json:"pvf_sha256"`
	Sources              map[string]string    `json:"sources"`
	AmbiguousDungeons    []uint32             `json:"ambiguous_dungeons"`
	UnavailableWorldmaps []uint32             `json:"unavailable_worldmaps"`
	MinimumLevel         uint32               `json:"minimum_level"`
	Excluded             []uint32             `json:"excluded"`
	Ranges               map[uint32][2]uint32 `json:"ranges"`
	excluded             map[uint32]bool
}

var loadRecommended = sync.OnceValues(func() (*RecommendedRules, error) {
	var rules RecommendedRules
	if err := json.Unmarshal(recommendedRulesJSON, &rules); err != nil {
		return nil, err
	}
	return NewRecommendedRules(rules)
})

func NewRecommendedRules(rules RecommendedRules) (*RecommendedRules, error) {
	if b, err := hex.DecodeString(rules.SourceChecksum); err != nil || len(b) != 32 {
		return nil, fmt.Errorf("invalid recommended dungeon source identity")
	}
	if rules.MinimumLevel == 0 || len(rules.Ranges) == 0 || len(rules.Excluded) == 0 {
		return nil, fmt.Errorf("推荐地下城源规则不完整")
	}
	for id, bounds := range rules.Ranges {
		if id == 0 || bounds[0] == 0 || bounds[1] < bounds[0] {
			return nil, fmt.Errorf("推荐地下城 %d 的等级范围无效", id)
		}
	}
	rules.excluded = make(map[uint32]bool, len(rules.Excluded))
	for _, id := range rules.Excluded {
		rules.excluded[id] = true
	}
	return &rules, nil
}

var currentRecommended atomic.Pointer[RecommendedRules]

func EmbeddedRecommendedRules() (*RecommendedRules, error) { return loadRecommended() }

func CurrentRecommendedRules() (*RecommendedRules, error) {
	if r := currentRecommended.Load(); r != nil {
		return r, nil
	}
	return loadRecommended()
}

func InstallRecommendedRules(source *RecommendedRules) (func(), error) {
	if source == nil {
		return nil, fmt.Errorf("nil recommended rules")
	}
	r := *source
	r.Excluded = append([]uint32(nil), source.Excluded...)
	r.AmbiguousDungeons = append([]uint32(nil), source.AmbiguousDungeons...)
	r.UnavailableWorldmaps = append([]uint32(nil), source.UnavailableWorldmaps...)
	r.Ranges = make(map[uint32][2]uint32, len(source.Ranges))
	for k, v := range source.Ranges {
		r.Ranges[k] = v
	}
	r.Sources = make(map[string]string, len(source.Sources))
	for k, v := range source.Sources {
		r.Sources[k] = v
	}
	validated, err := NewRecommendedRules(r)
	if err != nil {
		return nil, err
	}
	previous := currentRecommended.Swap(validated)
	return func() { currentRecommended.Store(previous) }, nil
}

// 145A28720：先检查排除表及最低角色等级，再查副本专属范围，缺省时查区域范围。
// 导出器已展开无歧义区域映射；上下界均包含，不把强制显示推荐标记当成计数资格。
func RecommendedDungeonClear(dungeon uint32, level byte) (bool, error) {
	rules, err := CurrentRecommendedRules()
	if err != nil {
		return false, err
	}
	if uint32(level) < rules.MinimumLevel || rules.excluded[dungeon] {
		return false, nil
	}
	bounds, ok := rules.Ranges[dungeon]
	return ok && uint32(level) >= bounds[0] && uint32(level) <= bounds[1], nil
}
