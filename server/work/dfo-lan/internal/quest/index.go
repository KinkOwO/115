package quest

import (
	"dfolan/internal/catalog"
	"sort"
)

// Entry holds one quest's static, source-derived properties. Everything here
// is decided by the imported catalog alone; nothing depends on a character.
type Entry struct {
	ID            uint32
	Kind          string
	Model         string
	Initial       uint32
	Implemented   bool
	RewardUsable  bool
	GrowUsable    bool
	MinimumLevel  uint32
	MaximumLevel  uint32
	Jobs          []string
	Prerequisites []uint32
	GrowTypes     []int32
	NPC           uint32
	Range         RangeObjective
	Seek          SeekObjective
}

// Index precomputes those properties once per catalog load. The hot paths —
// the available list, a run's map-clear settlement and the world proximity
// check — otherwise re-walk all 2844 quests and re-scan each quest's own
// script cells on every level-up, clear and submit.
type Index struct {
	Source     string
	Ordered    []uint32
	Entries    map[uint32]*Entry
	ByClearMap map[uint32][]uint16
	Positional []uint32
}

// rewardUsable reports whether Finish can settle this quest's reward today.
// Quests it rejects must also stay out of the available list: a character who
// accepts one can never submit it, which reads in game as a quest that will
// not complete no matter how often it is handed in.
func rewardUsable(d catalog.QuestDefinition) bool {
	if t := cells(d.Script.Cells, "[reward type]"); len(t) > 0 {
		if len(t) != 1 || t[0].Type != 6 || (t[0].Text != "[item]" && t[0].Text != "[none]") {
			return false
		}
	}
	return len(cells(d.Script.Cells, "[reward select int data]")) == 0 &&
		len(cells(d.Script.Cells, "[reward selection int data]")) == 0
}

func BuildIndex(c catalog.QuestCatalog) *Index {
	x := &Index{Source: c.Source.Checksum, Entries: map[uint32]*Entry{}, ByClearMap: map[uint32][]uint16{}}
	for id, d := range c.Quests {
		initial, model, err := InitialProgress(d)
		e := &Entry{
			ID: id, Kind: d.Kind, Model: model, Initial: initial, Implemented: err == nil,
			RewardUsable: rewardUsable(d), GrowUsable: true,
			MinimumLevel: d.MinimumLevel, MaximumLevel: d.MaximumLevel,
			Jobs: d.Jobs, Prerequisites: d.Prerequisites,
		}
		for _, g := range cells(d.Script.Cells, "[grow type]") {
			if g.Type != 0 {
				e.GrowUsable = false
				break
			}
			e.GrowTypes = append(e.GrowTypes, g.Value)
		}
		switch {
		case !e.Implemented:
		case model == SingleClearMap:
			m := uint32(d.ObjectiveCells[0].Value)
			if id < 65535 && id != 0 {
				x.ByClearMap[m] = append(x.ByClearMap[m], uint16(id))
			}
		case model == SingleMeetNPC:
			e.NPC = uint32(d.ObjectiveCells[0].Value)
			x.Positional = append(x.Positional, id)
		case model == SingleReachRange:
			e.Range, _ = ReachRange(d)
			x.Positional = append(x.Positional, id)
		case model == SeekAndMeetNPC:
			e.Seek, _ = SeekMeet(d)
			e.NPC = e.Seek.NPC
			x.Positional = append(x.Positional, id)
		}
		x.Entries[id] = e
		x.Ordered = append(x.Ordered, id)
	}
	sort.Slice(x.Ordered, func(i, j int) bool { return x.Ordered[i] < x.Ordered[j] })
	sort.Slice(x.Positional, func(i, j int) bool { return x.Positional[i] < x.Positional[j] })
	for m := range x.ByClearMap {
		sort.Slice(x.ByClearMap[m], func(i, j int) bool { return x.ByClearMap[m][i] < x.ByClearMap[m][j] })
	}
	return x
}

// Index returns the service's lazily built quest index.
func (s *Service) Index() *Index {
	if s.index == nil || s.index.Source != s.Catalog.Source.Checksum {
		s.index = BuildIndex(s.Catalog)
	}
	return s.index
}
