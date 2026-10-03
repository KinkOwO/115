package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// ImportApocalypse follows the two pinned CTP source files. It retains every
// raw record and positional tuple used by the existing compiled catalog.
func ImportApocalypse(a *pvf.Archive) (*ApocalypseCatalog, error) {
	if a == nil {
		return nil, fmt.Errorf("nil apocalypse archive")
	}
	doc, err := importApocalypseTable(a, ApocalypseSource)
	if err != nil {
		return nil, err
	}
	doc.Duties, err = importApocalypseDuties(a)
	if err != nil {
		return nil, err
	}
	if err := verifyNativeApocalypseShape(doc); err != nil {
		return nil, err
	}
	return ValidateApocalypseCatalog(doc)
}

// Columns the server reads. Names are the literals the container stores.
const (
	colOperationData  = "[operation data set]"
	colPhaseInfo      = "[phase info]"
	colAllowCoin      = "[allow coin]"
	colGateSchedule   = "[gate schedule]"
	colGateFlow       = "[gateflow]"
	colGateCloseWarn  = "[gate close warning]"
	colIndex          = "[index]"
	colType           = "[type]"
	colCardSymbol     = "[card Symbol Index]"
	colMemberLimit    = "[member limit]"
	colRecommendFame  = "[recommend fame]"
	colRewardData     = "[reward data]"
	colTingRewardData = "[ting reward data]"
	colStringData     = "[string data]"
)

// build decodes one table and derives the server-facing view.
func importApocalypseTable(a *pvf.Archive, path string) (*ApocalypseCatalog, error) {
	table, err := a.CTP(path)
	if err != nil {
		return nil, err
	}
	doc := &ApocalypseCatalog{
		Source:      table.Path,
		SHA256:      table.SHA256,
		Bytes:       table.Bytes,
		Version:     table.Version,
		RecordCount: table.RecordCount,
		PoolTags:    table.PoolTags,
		Records:     table.Records,
		Trailer:     table.Columns,
	}
	doc.PhaseClock = apocalypsePhaseClock(table)
	for _, block := range table.RecordsOf(colOperationData) {
		doc.Operations = append(doc.Operations, apocalypseOperationOf(table, block))
	}
	return doc, nil
}

// phaseClock reads [phase info] as (phase index, seconds) pairs.
func apocalypsePhaseClock(table *pvf.CTPTable) []ApocalypsePhase {
	recs := table.RecordsOf(colPhaseInfo)
	if len(recs) == 0 {
		return nil
	}
	// Every [phase info] row carries the same clock; the first is authoritative
	// and verify() refuses a table where they disagree.
	vals := recs[0].Floats()
	var out []ApocalypsePhase
	for i := 0; i+1 < len(vals); i += 2 {
		out = append(out, ApocalypsePhase{Phase: int64(vals[i]), Seconds: vals[i+1]})
	}
	return out
}

// operationOf collects the child rows of one [operation data set] block.
func apocalypseOperationOf(table *pvf.CTPTable, block pvf.CTPRecord) ApocalypseOperation {
	op := ApocalypseOperation{Row: block.Index}
	child := map[string]pvf.CTPRecord{}
	for _, r := range table.Records {
		if r.Parent == block.Index {
			child[r.Name] = r
		}
	}
	if r, ok := child[colIndex]; ok {
		op.Index = apocalypseFirstInt(r)
	}
	if r, ok := child[colType]; ok {
		op.Type = apocalypseFirstInt(r)
	}
	if r, ok := child[colCardSymbol]; ok {
		op.CardSymbolIndex = apocalypseFirstInt(r)
	}
	if r, ok := child[colMemberLimit]; ok {
		if t := r.Texts(); len(t) > 0 {
			op.MemberLimit = t[0]
		}
	}
	if r, ok := child[colRecommendFame]; ok {
		op.RecommendFame = apocalypseFirstInt(r)
	}
	if r, ok := child["[type fixed value]"]; ok {
		op.TypeFixedValue = apocalypseFirstInt(r)
	}
	// [allow coin] only exists on some operation blocks; a missing column is
	// meaningful and stays null rather than defaulting to zero.
	if r, ok := child[colAllowCoin]; ok {
		op.AllowCoin = r.Ints()
	}
	if r, ok := child[colGateSchedule]; ok {
		op.GateSchedule = r.Ints()
	}
	if r, ok := child[colGateFlow]; ok {
		op.GateFlow = r.Ints()
	}
	if r, ok := child[colGateCloseWarn]; ok {
		for _, c := range r.Cells {
			v := ApocalypseValue{}
			switch c.Kind {
			case "float":
				f := c.Float
				v.Float = &f
			case "name", "name_indexed":
				t := c.Name
				v.Text = &t
			}
			if v.Float != nil || v.Text != nil {
				op.GateCloseWarn = append(op.GateCloseWarn, v)
			}
		}
	}
	if r, ok := child[colRewardData]; ok {
		op.Reward = apocalypseRewardOf(r)
	}
	if r, ok := child[colTingRewardData]; ok {
		op.TingReward = apocalypseRewardOf(r)
	}
	if r, ok := child[colStringData]; ok {
		op.StringData = r.Texts()
	}
	return op
}

// rewardOf splits a reward row into its label and the remaining tuple.
func apocalypseRewardOf(rec pvf.CTPRecord) *ApocalypseReward {
	out := &ApocalypseReward{}
	for _, c := range rec.Cells {
		switch c.Kind {
		case "name", "name_indexed":
			if out.Label == "" {
				out.Label = c.Name
				continue
			}
		case "float":
			out.Values = append(out.Values, c.Float)
		}
	}
	return out
}

func importApocalypseDuties(a *pvf.Archive) (*ApocalypseDuties, error) {
	table, err := a.CTP(ApocalypseDutySource)
	if err != nil {
		return nil, err
	}
	return &ApocalypseDuties{
		Source:  table.Path,
		SHA256:  table.SHA256,
		Bytes:   table.Bytes,
		Records: table.Records,
		Trailer: table.Columns,
	}, nil
}

// verify refuses a table that no longer matches the shape the server relies on.
func verifyNativeApocalypseShape(doc *ApocalypseCatalog) error {
	if doc.Version != 1 {
		return fmt.Errorf("unsupported version %d", doc.Version)
	}
	if int(doc.RecordCount) != len(doc.Records) {
		return fmt.Errorf("declared %d records but walked %d", doc.RecordCount, len(doc.Records))
	}
	for i, r := range doc.Records {
		if r.Name == "" {
			return fmt.Errorf("record %d at %#x has no resolvable name", i, r.Offset)
		}
	}
	if len(doc.PhaseClock) == 0 {
		return fmt.Errorf("no [phase info] clock")
	}
	// Every [phase info] row must agree, otherwise the clock is not a constant.
	for _, r := range doc.Records {
		if r.Name != colPhaseInfo {
			continue
		}
		vals := r.Floats()
		if len(vals) != len(doc.PhaseClock)*2 {
			return fmt.Errorf("record %d carries %d values, want %d", r.Index, len(vals), len(doc.PhaseClock)*2)
		}
		for i, e := range doc.PhaseClock {
			if vals[2*i] != float64(e.Phase) || vals[2*i+1] != e.Seconds {
				return fmt.Errorf("record %d disagrees on phase %d: got %v %v", r.Index, e.Phase, vals[2*i], vals[2*i+1])
			}
		}
	}
	if len(doc.Operations) == 0 {
		return fmt.Errorf("no %s block", colOperationData)
	}
	// The trailer must map every operation block, which is how the four blocks
	// are declared in the first place.
	col := apocalypseFindColumn(doc, colOperationData)
	if col == nil {
		return fmt.Errorf("trailer has no %s column", colOperationData)
	}
	if len(col.Records) != len(doc.Operations) {
		return fmt.Errorf("trailer lists %d operation blocks, derived %d", len(col.Records), len(doc.Operations))
	}
	return nil
}

func apocalypseFindColumn(doc *ApocalypseCatalog, name string) *pvf.CTPColumn {
	for i := range doc.Trailer {
		if doc.Trailer[i].Name == name {
			return &doc.Trailer[i]
		}
	}
	return nil
}

func apocalypseFirstInt(r pvf.CTPRecord) *int64 {
	for _, c := range r.Cells {
		if c.Kind == "float" {
			v := int64(c.Float)
			return &v
		}
	}
	return nil
}
