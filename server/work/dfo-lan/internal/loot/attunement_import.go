package loot

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

const (
	dungeonIndexColumn = "[dungeon index]"
	fixedTableColumn   = "[fixed drop table]"
	additionalColumn   = "[additional drop table]"
	hiddenTableColumn  = "[hidden drop table]"
	couponTableColumn  = "[coupon drop table]"
	obtainProbColumn   = "[obtain prob]"
	dropProbColumn     = "[drop prob]"
	mazeColumn         = "[maze]"
	dropListColumn     = "[drop list]"
	effectIndexColumn  = "[effect index]"
	selectProbColumn   = "[select prob]"
	dropCountColumn    = "[drop count]"

	// weightSpace is the denominator every [drop list] and the whole
	// [additional drop table] set add up to. It is a fact about the data, not a
	// tunable: both sums are checked before anything is written.
)

// readTable decodes one difficulty table and checks every invariant the reading
// depends on before returning. A table that breaks one is not written at all:
// a half-believed reward table would pay the wrong items silently.
func ReadAttunementTable(a *pvf.Archive, entry string) (attunementDungeon, error) {
	var out attunementDungeon
	t, err := a.CTP(entry)
	if err != nil {
		return out, err
	}
	out.Path = t.Path
	out.SHA256 = t.SHA256
	out.Bytes = t.Bytes
	out.Version = t.Version
	out.RecordCount = t.RecordCount

	children := map[int][]int{}
	for i, r := range t.Records {
		if r.Parent >= 0 {
			children[r.Parent] = append(children[r.Parent], i)
		}
	}
	childNamed := func(parent int, name string) (int, bool) {
		for _, i := range children[parent] {
			if t.Records[i].Name == name {
				return i, true
			}
		}
		return 0, false
	}

	var dungeonSet bool
	for i, r := range t.Records {
		if r.Parent >= 0 {
			continue
		}
		switch r.Name {
		case dungeonIndexColumn:
			v, err := attunementSingleNumber(r)
			if err != nil {
				return out, fmt.Errorf("%s: %w", dungeonIndexColumn, err)
			}
			if dungeonSet {
				return out, fmt.Errorf("repeated %s", dungeonIndexColumn)
			}
			out.Dungeon = v
			dungeonSet = true
		case fixedTableColumn:
			var ft attunementFixed
			if j, ok := childNamed(i, mazeColumn); ok {
				if ft.Maze, err = attunementSingleNumber(t.Records[j]); err != nil {
					return out, err
				}
			}
			j, ok := childNamed(i, dropListColumn)
			if !ok {
				return out, fmt.Errorf("%s at %d carries no %s", fixedTableColumn, i, dropListColumn)
			}
			if ft.Entries, err = attunementEntries(t.Records[j]); err != nil {
				return out, err
			}
			if err := checkAttunementEntries(ft.Entries, fmt.Sprintf("%s maze %d", fixedTableColumn, ft.Maze)); err != nil {
				return out, err
			}
			out.Fixed = append(out.Fixed, ft)
		case additionalColumn:
			var at attunementAdditional
			for _, col := range []struct {
				name string
				dst  *uint32
			}{{effectIndexColumn, &at.EffectIndex}, {selectProbColumn, &at.SelectProb}, {dropCountColumn, &at.DropCount}} {
				j, ok := childNamed(i, col.name)
				if !ok {
					return out, fmt.Errorf("%s at %d carries no %s", additionalColumn, i, col.name)
				}
				if *col.dst, err = attunementSingleNumber(t.Records[j]); err != nil {
					return out, err
				}
			}
			j, ok := childNamed(i, dropListColumn)
			if !ok {
				return out, fmt.Errorf("%s at %d carries no %s", additionalColumn, i, dropListColumn)
			}
			if at.Entries, err = attunementEntries(t.Records[j]); err != nil {
				return out, err
			}
			if err := checkAttunementEntries(at.Entries, fmt.Sprintf("%s effect %d", additionalColumn, at.EffectIndex)); err != nil {
				return out, err
			}
			out.Additional = append(out.Additional, at)
		case couponTableColumn:
			var ct attunementCoupon
			for _, col := range []struct {
				name string
				dst  *uint32
			}{{obtainProbColumn, &ct.ObtainProb}, {dropProbColumn, &ct.DropProb}} {
				j, ok := childNamed(i, col.name)
				if !ok {
					return out, fmt.Errorf("%s at %d carries no %s", couponTableColumn, i, col.name)
				}
				if *col.dst, err = attunementSingleNumber(t.Records[j]); err != nil {
					return out, err
				}
			}
			j, ok := childNamed(i, dropListColumn)
			if !ok {
				return out, fmt.Errorf("%s at %d carries no %s", couponTableColumn, i, dropListColumn)
			}
			// An empty drop list is real here: one row spends its cells on
			// obtain/drop prob alone, and its drop prob is zero, so it pays
			// nothing. Refusing it would drop the row and hide that.
			if ct.Entries, err = attunementCouponEntries(t.Records[j]); err != nil {
				return out, err
			}
			out.Coupons = append(out.Coupons, ct)
		case hiddenTableColumn:
			var ht attunementHidden
			if j, ok := childNamed(i, mazeColumn); ok {
				if ht.Maze, err = attunementSingleNumber(t.Records[j]); err != nil {
					return out, err
				}
			}
			for _, j := range children[i] {
				if t.Records[j].Name != dropListColumn {
					continue
				}
				row, err := attunementReadHiddenRow(t.Records[j])
				if err != nil {
					return out, err
				}
				ht.Index = row.Index
				ht.Entries = row.Entries
				out.Hidden = append(out.Hidden, ht)
			}
		}
	}
	if !dungeonSet {
		return out, fmt.Errorf("missing %s", dungeonIndexColumn)
	}
	if len(out.Fixed) == 0 && len(out.Additional) == 0 {
		return out, fmt.Errorf("table carries no reward list")
	}

	// Coupon rows are structural only. Both probabilities live in the same
	// million-space the rest of the file uses, so anything outside it is a
	// misread column rather than data - that much is checkable. Whether the two
	// are independent, sequential or indexed by difficulty is not, which is why
	// no roll reads them (see couponTable).
	for i, ct := range out.Coupons {
		if ct.ObtainProb > attunementWeightSpace || ct.DropProb > attunementWeightSpace {
			return out, fmt.Errorf("%s %d prob (%d/%d) is outside the %d space",
				couponTableColumn, i, ct.ObtainProb, ct.DropProb, attunementWeightSpace)
		}
		if len(ct.Entries) > 0 {
			if err := checkAttunementEntries(ct.Entries, fmt.Sprintf("%s %d", couponTableColumn, i)); err != nil {
				return out, err
			}
		}
	}

	// The additional tables are a single draw from the same million-space: their
	// [select prob] values are exclusive weights, not independent chances.
	var prob uint32
	for _, at := range out.Additional {
		prob += at.SelectProb
	}
	if len(out.Additional) > 0 && prob != attunementWeightSpace {
		return out, fmt.Errorf("%s select prob sums to %d, want %d", additionalColumn, prob, attunementWeightSpace)
	}
	return out, nil
}

type attunementHiddenRow struct {
	Index   uint32
	Entries []attunementHiddenEntry
}

// readHiddenRow decodes one [drop list] of a hidden table: a leading index
// followed by (tier, key, item) triples.
func attunementReadHiddenRow(r pvf.CTPRecord) (attunementHiddenRow, error) {
	var out attunementHiddenRow
	var cells []pvf.CTPCell
	for _, c := range r.Cells {
		if c.Kind != "" {
			cells = append(cells, c)
		}
	}
	if len(cells) == 0 {
		return out, fmt.Errorf("empty hidden %s", dropListColumn)
	}
	v, err := attunementNumber(cells[0])
	if err != nil {
		return out, fmt.Errorf("hidden %s index: %w", dropListColumn, err)
	}
	out.Index = v
	rest := cells[1:]
	if len(rest)%3 != 0 {
		return out, fmt.Errorf("hidden %s holds %d cells, want a triple count", dropListColumn, len(rest)+1)
	}
	for i := 0; i < len(rest); i += 3 {
		tier, err := attunementText(rest[i])
		if err != nil {
			return out, err
		}
		key, err := attunementNumber(rest[i+1])
		if err != nil {
			return out, err
		}
		item, err := attunementNumber(rest[i+2])
		if err != nil {
			return out, err
		}
		out.Entries = append(out.Entries, attunementHiddenEntry{Tier: tier, Key: key, Item: item})
	}
	return out, nil
}

// entries decodes a fixed/additional [drop list]: (tier, weight, item) triples.
func attunementEntries(r pvf.CTPRecord) ([]attunementEntry, error) {
	var cells []pvf.CTPCell
	for _, c := range r.Cells {
		if c.Kind != "" {
			cells = append(cells, c)
		}
	}
	if len(cells)%3 != 0 {
		return nil, fmt.Errorf("%s holds %d cells, want a triple count", dropListColumn, len(cells))
	}
	var out []attunementEntry
	for i := 0; i < len(cells); i += 3 {
		tier, err := attunementText(cells[i])
		if err != nil {
			return nil, err
		}
		weight, err := attunementNumber(cells[i+1])
		if err != nil {
			return nil, err
		}
		item, err := attunementNumber(cells[i+2])
		if err != nil {
			return nil, err
		}
		if item == 0 || item > 0x2ffffff {
			return nil, fmt.Errorf("%s item %d is outside the item id space", dropListColumn, item)
		}
		out = append(out, attunementEntry{Tier: tier, Weight: weight, Item: item})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s is empty", dropListColumn)
	}
	return out, nil
}

// couponEntries reads a [coupon drop table]'s drop list, tolerating the empty
// list the source uses for a row that pays nothing.
func attunementCouponEntries(r pvf.CTPRecord) ([]attunementEntry, error) {
	var cells []pvf.CTPCell
	for _, c := range r.Cells {
		if c.Kind != "" {
			cells = append(cells, c)
		}
	}
	if len(cells) == 0 {
		return nil, nil
	}
	return attunementEntries(r)
}

func attunementSingleNumber(r pvf.CTPRecord) (uint32, error) {
	var n uint32
	for _, c := range r.Cells {
		if c.Kind == "" {
			continue
		}
		v, err := attunementNumber(c)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", r.Name, err)
		}
		n = v
	}
	return n, nil
}

func attunementNumber(c pvf.CTPCell) (uint32, error) {
	switch c.Kind {
	case "float":
		if c.Float < 0 || c.Float > float64(^uint32(0)) {
			return 0, fmt.Errorf("value %g does not fit a u32", c.Float)
		}
		return uint32(c.Float), nil
	case "byte":
		return uint32(c.Byte), nil
	default:
		return 0, fmt.Errorf("cell kind %q is not numeric", c.Kind)
	}
}

func attunementText(c pvf.CTPCell) (string, error) {
	switch c.Kind {
	case "name", "name_indexed":
		if strings.TrimSpace(c.Name) == "" {
			return "", fmt.Errorf("empty name cell")
		}
		return c.Name, nil
	default:
		return "", fmt.Errorf("cell kind %q is not a name", c.Kind)
	}
}
