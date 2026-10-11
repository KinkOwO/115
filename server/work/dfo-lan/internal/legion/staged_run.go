package legion

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
)

// StagedRun owns a Corridor/Dusky sequence. It never accepts a client clear
// flag as a server verdict or lets a reward-window request skip combat.
type StagedRun struct {
	content    string
	checksum   string
	mode       StagedOperationMode
	dungeons   []catalog.LegionDungeon
	stage      int
	cards      []catalog.NativeLegionCard
	selected   *catalog.NativeLegionCard
	activeRun  string
	clearedRun string
	settledRun string
	finished   bool
	usedRuns   map[string]bool
}

// These are separate source dimensions. Expert cards must never become normal
// cards simply because their type and numeric bounds happen to be the same.
type StagedOperationMode struct {
	Mode, Party, ParameterDifficulty string
}

func NewStagedRun(contents *catalog.LegionContents, name string, weeklyRow int, mode StagedOperationMode) (*StagedRun, error) {
	if contents == nil || contents.Source.Checksum == "" {
		return nil, fmt.Errorf("staged legion source missing")
	}
	if name != "Dimension Cloister" && name != "DuskyIsland" {
		return nil, fmt.Errorf("content has no native three-stage adapter")
	}
	c, ok := contents.Contents[name]
	if !ok {
		return nil, fmt.Errorf("native content absent")
	}
	if c.NativeOperations == nil || mode.Mode == "" || mode.Party == "" || mode.ParameterDifficulty == "" || len(c.NativeOperations.Modes[mode.Mode]) == 0 {
		return nil, fmt.Errorf("native operation dimensions missing")
	}
	dungeons, err := catalog.StagedLegionDungeons(c, weeklyRow)
	if err != nil {
		return nil, err
	}
	return &StagedRun{content: name, checksum: contents.Source.Checksum, mode: mode, dungeons: dungeons, usedRuns: map[string]bool{}}, nil
}

func (r *StagedRun) Stage() int      { return r.stage }
func (r *StagedRun) Finished() bool  { return r.finished }
func (r *StagedRun) Content() string { return r.content }

// Offer is separate from UI opening. Reopening a window reads Cards and keeps
// the original draw; replacing an offer requires an explicit reselect action.
func (r *StagedRun) Offer(source *catalog.LegionContents, cards []catalog.NativeLegionCard) error {
	if r == nil || r.finished || r.activeRun != "" {
		return fmt.Errorf("operation offer while run is unavailable")
	}
	if source == nil || source.Source.Checksum != r.checksum {
		return fmt.Errorf("operation offer source changed")
	}
	c, ok := source.Contents[r.content]
	if !ok || c.NativeOperations == nil {
		return fmt.Errorf("operation offer content absent")
	}
	if len(cards) == 0 {
		return fmt.Errorf("empty native operation offer")
	}
	seen := map[uint16]bool{}
	allowed := map[uint16]bool{}
	for _, index := range c.NativeOperations.Modes[r.mode.Mode] {
		allowed[index] = true
	}
	for _, card := range cards {
		d, ok := c.NativeOperations.Definitions[card.Index]
		if !ok || !allowed[card.Index] || d.MemberLimit != r.mode.Party || card.Index == 0 || seen[card.Index] || d.SelectPool != card.Pool || d.Probability == 0 {
			return fmt.Errorf("operation offer identity/pool mismatch")
		}
		valid := false
		for _, bounds := range d.Ranges {
			if bounds.Party != r.mode.Party || bounds.Difficulty != r.mode.ParameterDifficulty {
				continue
			}
			if card.Parameter < bounds.Minimum || card.Parameter > bounds.Maximum || math.IsNaN(card.Parameter) || math.IsInf(card.Parameter, 0) {
				continue
			}
			if bounds.Minimum == bounds.Maximum {
				valid = true
				break
			}
			if bounds.Step > 0 {
				position := (card.Parameter - bounds.Minimum) / bounds.Step
				if math.Abs(position-math.Round(position)) < 1e-9 {
					valid = true
					break
				}
			}
		}
		if !valid {
			return fmt.Errorf("operation offer operand outside source range")
		}
		if (card.FixedValue == nil) != (d.FixedValue == nil) || card.FixedValue != nil && *card.FixedValue != *d.FixedValue {
			return fmt.Errorf("operation offer fixed operand mismatch")
		}
		seen[card.Index] = true
	}
	r.cards = cloneNativeCards(cards)
	r.selected = nil
	return nil
}

func cloneNativeCards(cards []catalog.NativeLegionCard) []catalog.NativeLegionCard {
	out := append([]catalog.NativeLegionCard(nil), cards...)
	for i := range out {
		if out[i].FixedValue != nil {
			v := *out[i].FixedValue
			out[i].FixedValue = &v
		}
	}
	return out
}
func (r *StagedRun) Cards() []catalog.NativeLegionCard { return cloneNativeCards(r.cards) }
func (r *StagedRun) Select(index uint16) error {
	if r == nil || r.finished || r.activeRun != "" {
		return fmt.Errorf("operation selection while run is unavailable")
	}
	for _, card := range cloneNativeCards(r.cards) {
		if card.Index == index {
			r.selected = &card
			return nil
		}
	}
	return fmt.Errorf("operation was not offered for this stage")
}

func (r *StagedRun) Entry(stage int) (catalog.LegionDungeon, error) {
	if r == nil || r.finished || stage != r.stage || stage < 0 || stage >= len(r.dungeons) || r.activeRun != "" || r.selected == nil {
		return catalog.LegionDungeon{}, fmt.Errorf("stage entry is not owned/selected")
	}
	d := r.dungeons[stage]
	d.Tail = append([]int64(nil), d.Tail...)
	return d, nil
}
func (r *StagedRun) Enter(stage int, dungeon uint32, run string) error {
	d, err := r.Entry(stage)
	if err != nil {
		return err
	}
	if run == "" || d.Dungeon != dungeon || r.usedRuns[run] {
		return fmt.Errorf("dungeon run does not own source stage")
	}
	r.activeRun = run
	r.usedRuns[run] = true
	return nil
}

// Complete must be invoked by an owned completed dungeon session after all
// rewards were durably saved. A duplicate completion cannot advance a stage.
func (r *StagedRun) Complete(run string, dungeon uint32) error {
	if r == nil || run == "" || r.activeRun != run || r.stage >= len(r.dungeons) || r.dungeons[r.stage].Dungeon != dungeon {
		return fmt.Errorf("clear does not own active source stage")
	}
	r.clearedRun = run
	return nil
}
func (r *StagedRun) RewardEnd(run string) error {
	if r == nil || run == "" || r.activeRun != run || r.clearedRun != run {
		return fmt.Errorf("reward end before owned durable clear")
	}
	r.settledRun = run
	r.activeRun = ""
	r.stage++
	r.cards = nil
	r.selected = nil
	r.finished = r.stage == len(r.dungeons)
	return nil
}

// Retreat clears only the unfinished fight. Earlier committed stages remain
// finished, while stale entry/clear/reward requests lose their old run owner.
func (r *StagedRun) Retreat(run string) error {
	if r == nil || run == "" || r.activeRun != run || r.clearedRun == run {
		return fmt.Errorf("retreat does not own an unfinished fight")
	}
	r.activeRun = ""
	r.cards = nil
	r.selected = nil
	return nil
}
