package legion

import (
	"dfolan/internal/catalog"
	"encoding/binary"
	"fmt"
	"math"
)

// ClearReward is the single native normal reward row. Unknown/branching
// categories are refused, rather than interpreting a positional CTP field as
// currency or borrowing the Ispins reward table.
type ClearReward struct {
	Template, Amount uint32
	Label            string
	Values           [5]uint32
}

func NormalClearReward(cat *catalog.ApocalypseCatalog, operation uint32) (ClearReward, error) {
	return guaranteedClearReward(cat, operation, "normal", 1)
}

// Expert uses the same native mode107 standard-item reader. Only the closed
// guaranteed normal/expert/master tuples are supported; no optional reward is rolled.
func GuaranteedClearReward(cat *catalog.ApocalypseCatalog, operation uint32) (ClearReward, error) {
	if cat == nil {
		return ClearReward{}, fmt.Errorf("native apocalypse rewards unavailable")
	}
	op := cat.Operation(int64(operation))
	// Master's current CTP tuple has tier 1, independently of operation type 3.
	if op != nil && op.Type != nil && *op.Type == 3 && op.Reward != nil && op.Reward.Label == "master" {
		return guaranteedClearReward(cat, operation, "master", 1)
	}
	if op != nil && op.Type != nil && *op.Type == 2 && op.Reward != nil && op.Reward.Label == "expert" {
		return guaranteedClearReward(cat, operation, op.Reward.Label, uint32(*op.Type))
	}
	return NormalClearReward(cat, operation)
}

// The legion COS owns the basic bundle; the Apocalypse CTP owns the
// separate star-stone box. Neither table substitutes for the other.
func GuaranteedBasicClearReward(contents catalog.LegionContents, source, difficulty string) (ClearReward, error) {
	if contents.Source.Checksum != source || len(contents.SHA256) != 64 {
		return ClearReward{}, fmt.Errorf("apocalypse basic reward source mismatch")
	}
	c, ok := contents.Contents["Apocalypse"]
	if !ok {
		return ClearReward{}, fmt.Errorf("apocalypse basic reward content missing")
	}
	var out ClearReward
	// The COS reader now preserves every repeated reward block, including
	// party/pass/PC variants. This executor owns a solo run. An untagged row
	// remains compatible with existing typed fixtures; another party's row
	// must never become either a second payout or a duplicate-row refusal.
	for _, row := range c.Rewards {
		if row.PartyType != "" && row.PartyType != "solo" || row.Difficulty != difficulty || row.Kind != "basic" {
			continue
		}
		if out.Template != 0 || len(row.Values) != 3 || row.Values[0] != 1000000 || row.Values[1] <= 0 || row.Values[1] > math.MaxUint32 || row.Values[2] <= 0 || row.Values[2] > math.MaxUint32 {
			return ClearReward{}, fmt.Errorf("unsupported apocalypse basic reward row")
		}
		out = ClearReward{Template: uint32(row.Values[1]), Amount: uint32(row.Values[2]), Label: difficulty}
	}
	if out.Template == 0 {
		return out, fmt.Errorf("apocalypse basic reward missing for %s", difficulty)
	}
	return out, nil
}

func guaranteedClearReward(cat *catalog.ApocalypseCatalog, operation uint32, label string, positionalType uint32) (ClearReward, error) {
	var out ClearReward
	if cat == nil {
		return out, fmt.Errorf("native apocalypse rewards unavailable")
	}
	op := cat.Operation(int64(operation))
	if op == nil || op.Reward == nil || op.Reward.Label != label || len(op.Reward.Values) != 5 {
		return out, fmt.Errorf("normal source reward row required")
	}
	for i, v := range op.Reward.Values {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > math.MaxUint32 || math.Trunc(v) != v {
			return out, fmt.Errorf("invalid native reward position %d", i)
		}
		out.Values[i] = uint32(v)
	}
	// 1476E7370 imports item/amount/weight and two u16 fields. This executor
	// supports the guaranteed solo rows only; the two trailing fields
	// retain their positional meaning until their other branches are closed.
	if out.Values[0] == 0 || out.Values[1] == 0 || out.Values[2] != 1000000 || out.Values[3] != positionalType || out.Values[4] != 1 {
		return ClearReward{}, fmt.Errorf("unsupported normal reward weight or positional flags")
	}
	out.Template, out.Amount, out.Label = out.Values[0], out.Values[1], op.Reward.Label
	return out, nil
}

// Native14250FBE0 reads item/count at 1600/1604, then advances44 bytes,
// 35 times per seat. Record+40==0 takes14250FF2B's ordinary item branch.
// The separate CTP star box stays in the first40B record; COS bundle products
// occupy the basic item list. Inventory has already committed every product.
func ApocalypseClearRewardItems(reward ClearReward, items []ClearReward, elapsedMillis uint64) ([]byte, error) {
	if reward.Template == 0 || reward.Amount == 0 || elapsedMillis > math.MaxInt64 {
		return nil, fmt.Errorf("invalid apocalypse reward projection")
	}
	p := make([]byte, 7772)
	binary.LittleEndian.PutUint32(p, reward.Template)
	binary.LittleEndian.PutUint32(p[4:], reward.Amount)
	if len(items) > 35 {
		return nil, fmt.Errorf("apocalypse basic item list exceeds native capacity")
	}
	for i, item := range items {
		if item.Template == 0 || item.Amount == 0 {
			return nil, fmt.Errorf("invalid apocalypse basic item")
		}
		at := 1600 + i*44
		binary.LittleEndian.PutUint32(p[at:], item.Template)
		binary.LittleEndian.PutUint32(p[at+4:], item.Amount)
	}
	binary.LittleEndian.PutUint64(p[7760:], elapsedMillis)
	return p, nil
}

// 1424FDB60 reads2405B. Byte0==0 is14250EDF0's explicit no-additional-
// reward branch; it renders/opens the already populated result window282.
func ApocalypseNoAdditionalReward() []byte { return make([]byte, 2405) }
