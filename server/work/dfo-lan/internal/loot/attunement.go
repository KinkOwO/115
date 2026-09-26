package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
)

// AttunementModel names the document shape LoadAttunementRewards accepts. The
// reader refuses anything else: a table that says it is something else may have
// moved a column, and paying the wrong column is worse than paying nothing.
const AttunementModel = "rewardboostinfo-ctp-v1"

// attunementWeightSpace is the denominator both the per-list weights and the
// whole [additional drop table] set add up to in the source CTPs. It is the one
// invariant that makes the reading falsifiable, so it is enforced here rather
// than trusted from the file.
const attunementWeightSpace = 1000000

type attunementEntry struct {
	Tier   string `json:"tier"`
	Weight uint32 `json:"weight"`
	Item   uint32 `json:"item"`
}

type attunementFixed struct {
	Maze    uint32            `json:"maze"`
	Entries []attunementEntry `json:"entries"`
}

type attunementAdditional struct {
	EffectIndex uint32            `json:"effectIndex"`
	SelectProb  uint32            `json:"selectProb"`
	DropCount   uint32            `json:"dropCount"`
	Entries     []attunementEntry `json:"entries"`
}

// attunementHidden keeps the hidden tables verbatim. Key is the middle number of
// a (tier, key, item) triple: in the fixed and additional tables that slot is a
// weight, but here it runs 0,1,2,.. in step with the item ids, which no weight
// can do. Nothing rolls these yet - see the note on Roll.
type attunementHiddenEntry struct {
	Tier string `json:"tier"`
	Key  uint32 `json:"key"`
	Item uint32 `json:"item"`
}

type attunementHidden struct {
	Maze    uint32                  `json:"maze"`
	Index   uint32                  `json:"index"`
	Entries []attunementHiddenEntry `json:"entries"`
}

type attunementDungeon struct {
	Path        string                 `json:"path"`
	SHA256      string                 `json:"sha256"`
	Bytes       int                    `json:"bytes"`
	Version     uint32                 `json:"version"`
	RecordCount uint32                 `json:"recordCount"`
	Dungeon     uint32                 `json:"dungeon"`
	Fixed       []attunementFixed      `json:"fixed"`
	Additional  []attunementAdditional `json:"additional"`
	Hidden      []attunementHidden     `json:"hidden"`
}

// AttunementRewards is the decoded reward table of the "boundary of attunement"
// (调律之边界) abyss dungeons: one entry per difficulty file, keyed by the
// dungeon the file declares.
type AttunementRewards struct {
	Model   string              `json:"model"`
	Archive pvf.ArchiveSnapshot `json:"archive"`
	Tables  []attunementDungeon `json:"tables"`

	byDungeon map[uint32]*attunementDungeon
}

// LoadAttunementRewards reads the generated table and checks every invariant the
// reading depends on. It is deliberately strict: a reward table that is only
// half understood would pay the wrong items, and the failure would look like
// "the boss dropped nothing", which is indistinguishable from the gap this
// feature exists to close.
func LoadAttunementRewards(path string) (*AttunementRewards, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var a AttunementRewards
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	if a.Model != AttunementModel {
		return nil, fmt.Errorf("attunement rewards model %q, want %q", a.Model, AttunementModel)
	}
	if len(a.Tables) == 0 {
		return nil, fmt.Errorf("attunement rewards carry no table")
	}
	a.byDungeon = make(map[uint32]*attunementDungeon, len(a.Tables))
	for i := range a.Tables {
		t := &a.Tables[i]
		if t.Dungeon == 0 {
			return nil, fmt.Errorf("attunement table %s declares no dungeon", t.Path)
		}
		if len(t.SHA256) != 64 {
			return nil, fmt.Errorf("attunement table %s has no source hash", t.Path)
		}
		if _, dup := a.byDungeon[t.Dungeon]; dup {
			return nil, fmt.Errorf("attunement table duplicates dungeon %d", t.Dungeon)
		}
		if len(t.Fixed) == 0 && len(t.Additional) == 0 {
			return nil, fmt.Errorf("attunement table %d carries no reward list", t.Dungeon)
		}
		seenMaze := map[uint32]bool{}
		for _, f := range t.Fixed {
			if seenMaze[f.Maze] {
				return nil, fmt.Errorf("attunement table %d repeats maze %d", t.Dungeon, f.Maze)
			}
			seenMaze[f.Maze] = true
			if err := checkAttunementEntries(f.Entries, fmt.Sprintf("dungeon %d fixed maze %d", t.Dungeon, f.Maze)); err != nil {
				return nil, err
			}
		}
		var prob uint32
		for _, at := range t.Additional {
			if at.DropCount == 0 {
				return nil, fmt.Errorf("dungeon %d additional effect %d draws nothing", t.Dungeon, at.EffectIndex)
			}
			// One [effect index] may legitimately appear more than once (the
			// source repeats 1001 for three different voidsoul amounts), but a
			// zero-probability branch would be dead data, so it is refused.
			if at.SelectProb == 0 {
				return nil, fmt.Errorf("dungeon %d additional effect %d has no select prob", t.Dungeon, at.EffectIndex)
			}
			if err := checkAttunementEntries(at.Entries, fmt.Sprintf("dungeon %d additional effect %d", t.Dungeon, at.EffectIndex)); err != nil {
				return nil, err
			}
			prob += at.SelectProb
		}
		if len(t.Additional) > 0 && prob != attunementWeightSpace {
			return nil, fmt.Errorf("dungeon %d select prob sums to %d, want %d", t.Dungeon, prob, attunementWeightSpace)
		}
		for _, h := range t.Hidden {
			for _, e := range h.Entries {
				if e.Item == 0 {
					return nil, fmt.Errorf("dungeon %d hidden maze %d holds an empty item", t.Dungeon, h.Maze)
				}
			}
		}
		a.byDungeon[t.Dungeon] = t
	}
	return &a, nil
}

func checkAttunementEntries(list []attunementEntry, what string) error {
	if len(list) == 0 {
		return fmt.Errorf("%s has an empty drop list", what)
	}
	var sum uint64
	for _, e := range list {
		if e.Item == 0 {
			return fmt.Errorf("%s holds an empty item", what)
		}
		if e.Weight == 0 {
			return fmt.Errorf("%s holds a zero weight for item %d", what, e.Item)
		}
		sum += uint64(e.Weight)
	}
	if sum != attunementWeightSpace {
		return fmt.Errorf("%s weights sum to %d, want %d", what, sum, attunementWeightSpace)
	}
	return nil
}

// Enabled reports whether any dungeon carries a reward table.
func (a *AttunementRewards) Enabled() bool { return a != nil && len(a.Tables) > 0 }

// Dungeons lists the dungeons the table pays, in file order.
func (a *AttunementRewards) Dungeons() []uint32 {
	if a == nil {
		return nil
	}
	out := make([]uint32, 0, len(a.Tables))
	for _, t := range a.Tables {
		out = append(out, t.Dungeon)
	}
	return out
}

// Templates lists every item the table can pay, hidden tables included. The
// hidden lists are never rolled, but they are part of the same reward set, so
// they are validated alongside it instead of being left unchecked.
func (a *AttunementRewards) Templates() []uint32 {
	if a == nil {
		return nil
	}
	seen := map[uint32]bool{}
	var out []uint32
	add := func(id uint32) {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, t := range a.Tables {
		for _, f := range t.Fixed {
			for _, e := range f.Entries {
				add(e.Item)
			}
		}
		for _, at := range t.Additional {
			for _, e := range at.Entries {
				add(e.Item)
			}
		}
		for _, h := range t.Hidden {
			for _, e := range h.Entries {
				add(e.Item)
			}
		}
	}
	return out
}

// ValidateTemplates refuses a reward table whose items the running item catalog
// does not know as stackables. Every source entry is a [booster] box, and the
// bag can only hold a box it can identify - an unknown template would be routed
// down the equipment path at pickup and rejected there, after the player had
// already seen it drop.
func (a *AttunementRewards) ValidateTemplates(c catalog.LootCatalog) error {
	if !a.Enabled() {
		return nil
	}
	for _, id := range a.Templates() {
		item, ok := c.Items[id]
		if !ok {
			return fmt.Errorf("attunement reward %d is absent from the item catalog; is the item index still supplemented?", id)
		}
		if item.Kind != "stackable" {
			return fmt.Errorf("attunement reward %d is %q, want a stackable box", id, item.Kind)
		}
	}
	return nil
}

// Roll pays one cleared attunement maze.
//
// The draw is: the fixed table of that maze (one weighted pick), then one branch
// of the additional tables, then [drop count] weighted picks inside that branch.
// One branch - not fourteen independent chances - because the whole
// [select prob] set sums to exactly a million, the same exclusive-weight
// convention every [drop list] uses. LoadAttunementRewards enforces that sum, so
// the reading cannot drift silently.
//
// A dungeon with no table, or a maze its fixed tables do not cover, leaves the
// seed untouched: the line stays inert for every other dungeon in the game, the
// same way a disabled Odyssey chapter drop does.
//
// The hidden tables are parsed and validated but not rolled. Their middle number
// is not a weight (it steps 0,1,2,.. with the item ids), so the trigger they
// belong to is not established, and a guess there would pay items on clears the
// source never pays them on. Which dungeon even has them is recorded: only the
// epic table does, and only for mazes 1 and 2, which a run never reaches.
func (a *AttunementRewards) Roll(seed, dungeon, maze uint32) ([]Award, uint32, error) {
	if !a.Enabled() {
		return nil, seed, nil
	}
	t, ok := a.byDungeon[dungeon]
	if !ok {
		return nil, seed, nil
	}
	rng := RNG{seed}
	var out []Award
	if fixed, ok := t.fixedFor(maze); ok {
		e, err := pickAttunement(&rng, fixed.Entries)
		if err != nil {
			return nil, seed, err
		}
		out = append(out, Award{Template: e.Item, Amount: 1})
	}
	if len(t.Additional) > 0 {
		branch, err := pickAttunementBranch(&rng, t.Additional)
		if err != nil {
			return nil, seed, err
		}
		for i := uint32(0); i < branch.DropCount; i++ {
			e, err := pickAttunement(&rng, branch.Entries)
			if err != nil {
				return nil, seed, err
			}
			out = append(out, Award{Template: e.Item, Amount: 1})
		}
	}
	return out, rng.Seed, nil
}

func (t *attunementDungeon) fixedFor(maze uint32) (attunementFixed, bool) {
	for _, f := range t.Fixed {
		if f.Maze == maze {
			return f, true
		}
	}
	return attunementFixed{}, false
}

// pickAttunement draws one entry out of a million-weight list.
func pickAttunement(rng *RNG, list []attunementEntry) (attunementEntry, error) {
	if len(list) == 0 {
		return attunementEntry{}, fmt.Errorf("attunement drop list is empty")
	}
	roll := rng.Next(attunementWeightSpace)
	var acc uint32
	for _, e := range list {
		acc += e.Weight
		if roll < acc {
			return e, nil
		}
	}
	// Only reachable if the weights do not sum to the space, which load time
	// already refuses; keep the last entry rather than dropping the award.
	return list[len(list)-1], nil
}

// pickAttunementBranch draws the one additional branch this clear takes.
func pickAttunementBranch(rng *RNG, list []attunementAdditional) (attunementAdditional, error) {
	if len(list) == 0 {
		return attunementAdditional{}, fmt.Errorf("attunement additional table is empty")
	}
	roll := rng.Next(attunementWeightSpace)
	var acc uint32
	for _, at := range list {
		acc += at.SelectProb
		if roll < acc {
			return at, nil
		}
	}
	return list[len(list)-1], nil
}
