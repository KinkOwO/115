package character

import (
	"fmt"
	"maps"
)

func fameCloneNested[K comparable, L comparable, V any](source map[K]map[L]V) map[K]map[L]V {
	out := make(map[K]map[L]V, len(source))
	for k, v := range source {
		out[k] = maps.Clone(v)
	}
	return out
}

func InstallFameRules(source *FameRules) (func(), error) {
	if source == nil {
		return nil, fmt.Errorf("nil fame rules")
	}
	r := *source
	r.Sources = maps.Clone(source.Sources)
	r.Tables = fameCloneNested(source.Tables)
	r.Refine = maps.Clone(source.Refine)
	r.Refine115 = maps.Clone(source.Refine115)
	r.Items = maps.Clone(source.Items)
	r.Sets = make(map[int][]fameThreshold, len(source.Sets))
	for k, v := range source.Sets {
		r.Sets[k] = append([]fameThreshold(nil), v...)
	}
	r.ItemPoints = make(map[uint32][]fameSetPoint, len(source.ItemPoints))
	for k, v := range source.ItemPoints {
		r.ItemPoints[k] = append([]fameSetPoint(nil), v...)
	}
	r.Expanded = append([]fameThreshold(nil), source.Expanded...)
	r.Awakening = fameCloneNested(source.Awakening)
	r.MemoryRestore = maps.Clone(source.MemoryRestore)
	r.MemoryActivate = maps.Clone(source.MemoryActivate)
	r.MemoryRuminations = maps.Clone(source.MemoryRuminations)
	r.SoleQuality = fameCloneNested(source.SoleQuality)
	r.SolePenalty = fameCloneNested(source.SolePenalty)
	validated, err := NewFameRules(r)
	if err != nil {
		return nil, err
	}
	previous := installedFameRules.Swap(validated)
	return func() { installedFameRules.Store(previous) }, nil
}
