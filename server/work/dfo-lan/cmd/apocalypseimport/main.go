// apocalypseimport decodes the compiled `.ctp` tables that drive the legion /
// apocalypse subsystem and writes the truth the server consumes:
//
//	configs/apocalypse.generated.json
//
// The input is read straight out of the frozen client PVF, so the generated
// file can always be reproduced and diffed against the source hash. Nothing is
// invented: every field is a column read, and fields whose meaning is not yet
// proven keep their positional form plus a note.
package main

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	apocalypseTable = "contents/2026/apocalypse/etc/apocalypse.ctp"
	dutyTable       = "contents/2026/apocalypse/etc/dungeonskillinfo.ctp"
)

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

// config is the generated document.
type config struct {
	Source      string   `json:"source"`
	SHA256      string   `json:"sha256"`
	Bytes       int      `json:"bytes"`
	Version     uint32   `json:"version"`
	RecordCount uint32   `json:"recordCount"`
	PoolTags    []string `json:"poolTags"`

	// PhaseClock is the [phase info] table: one entry per phase, in file order.
	// Each entry is the (phase index, duration) pair the container stores.
	PhaseClock []phaseEntry `json:"phaseClock"`

	// Operations is the derived view of the four [operation data set] blocks.
	Operations []operation `json:"operations"`

	// Records and Trailer are the faithful dump, kept so the config is
	// self-describing and future phases do not need a regeneration.
	Records []pvf.CTPRecord `json:"records"`
	Trailer []pvf.CTPColumn `json:"trailer"`

	// Duties is the duty (skirmisher / guardian) definition from the sibling
	// DungeonSkillInfo.ctp, with its raw rows.
	Duties *dutyConfig `json:"duties,omitempty"`
}

type phaseEntry struct {
	Phase   int64   `json:"phase"`
	Seconds float64 `json:"seconds"`
}

// operation mirrors one [operation data set] block. Unknown-meaning tuples stay
// positional so nobody mistakes them for proven fields.
type operation struct {
	Row             int      `json:"row"`
	Index           *int64   `json:"index,omitempty"`
	Type            *int64   `json:"type,omitempty"`
	CardSymbolIndex *int64   `json:"cardSymbolIndex,omitempty"`
	MemberLimit     string   `json:"memberLimit,omitempty"`
	RecommendFame   *int64   `json:"recommendFame,omitempty"`
	TypeFixedValue  *int64   `json:"typeFixedValue,omitempty"`
	AllowCoin       []int64  `json:"allowCoin,omitempty"`
	GateSchedule    []int64  `json:"gateSchedule,omitempty"`
	GateFlow        []int64  `json:"gateFlow,omitempty"`
	GateCloseWarn   []value  `json:"gateCloseWarning,omitempty"`
	Reward          *reward  `json:"reward,omitempty"`
	TingReward      *reward  `json:"tingReward,omitempty"`
	StringData      []string `json:"stringData,omitempty"`
}

// reward keeps the label and the remaining tuple positionally: the leading
// value matches the item id range used elsewhere by the subsystem, the rest is
// not proven yet.
type reward struct {
	Label  string    `json:"label"`
	Values []float64 `json:"values"`
}

// value is one position of a mixed tuple.
type value struct {
	Float *float64 `json:"float,omitempty"`
	Text  *string  `json:"text,omitempty"`
}

type dutyConfig struct {
	Source  string          `json:"source"`
	SHA256  string          `json:"sha256"`
	Bytes   int             `json:"bytes"`
	Records []pvf.CTPRecord `json:"records"`
	Trailer []pvf.CTPColumn `json:"trailer"`
}

func main() {
	source := flag.String("source", "", "source PVF, opened read-only")
	out := flag.String("output", "", "output JSON path")
	flag.Parse()
	if *source == "" {
		log.Fatal("missing -source")
	}
	if *out == "" {
		log.Fatal("missing -output")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatal(err)
	}
	doc, err := build(a, apocalypseTable)
	if err != nil {
		log.Fatalf("apocalypse.ctp: %v", err)
	}
	duties, err := buildDuties(a)
	if err != nil {
		log.Fatalf("%s: %v", dutyTable, err)
	}
	doc.Duties = duties
	if err = verify(doc); err != nil {
		log.Fatalf("apocalypse.ctp shape check failed: %v", err)
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(*out), 0o700); err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*out, append(b, '\n'), 0o600); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("apocalypse.ctp sha256=%s bytes=%d records=%d poolTags=%d\n",
		doc.SHA256[:16], doc.Bytes, doc.RecordCount, len(doc.PoolTags))
	fmt.Printf("phase clock: %s\n", formatClock(doc.PhaseClock))
	for _, op := range doc.Operations {
		fmt.Printf("operation row=%-3d index=%s type=%s allowCoin=%v gateSchedule=%v reward=%s\n",
			op.Row, opt(op.Index), opt(op.Type), op.AllowCoin, op.GateSchedule, rewardLabel(op.Reward))
	}
	fmt.Printf("duties: %s sha256=%s rows=%d\n", dutyTable, duties.SHA256[:16], len(duties.Records))
	fmt.Printf("wrote %s\n", *out)
}

// build decodes one table and derives the server-facing view.
func build(a *pvf.Archive, path string) (*config, error) {
	table, err := a.CTP(path)
	if err != nil {
		return nil, err
	}
	doc := &config{
		Source:      table.Path,
		SHA256:      table.SHA256,
		Bytes:       table.Bytes,
		Version:     table.Version,
		RecordCount: table.RecordCount,
		PoolTags:    table.PoolTags,
		Records:     table.Records,
		Trailer:     table.Columns,
	}
	doc.PhaseClock = phaseClock(table)
	for _, block := range table.RecordsOf(colOperationData) {
		doc.Operations = append(doc.Operations, operationOf(table, block))
	}
	return doc, nil
}

// phaseClock reads [phase info] as (phase index, seconds) pairs.
func phaseClock(table *pvf.CTPTable) []phaseEntry {
	recs := table.RecordsOf(colPhaseInfo)
	if len(recs) == 0 {
		return nil
	}
	// Every [phase info] row carries the same clock; the first is authoritative
	// and verify() refuses a table where they disagree.
	vals := recs[0].Floats()
	var out []phaseEntry
	for i := 0; i+1 < len(vals); i += 2 {
		out = append(out, phaseEntry{Phase: int64(vals[i]), Seconds: vals[i+1]})
	}
	return out
}

// operationOf collects the child rows of one [operation data set] block.
func operationOf(table *pvf.CTPTable, block pvf.CTPRecord) operation {
	op := operation{Row: block.Index}
	child := map[string]pvf.CTPRecord{}
	for _, r := range table.Records {
		if r.Parent == block.Index {
			child[r.Name] = r
		}
	}
	if r, ok := child[colIndex]; ok {
		op.Index = firstInt(r)
	}
	if r, ok := child[colType]; ok {
		op.Type = firstInt(r)
	}
	if r, ok := child[colCardSymbol]; ok {
		op.CardSymbolIndex = firstInt(r)
	}
	if r, ok := child[colMemberLimit]; ok {
		if t := r.Texts(); len(t) > 0 {
			op.MemberLimit = t[0]
		}
	}
	if r, ok := child[colRecommendFame]; ok {
		op.RecommendFame = firstInt(r)
	}
	if r, ok := child["[type fixed value]"]; ok {
		op.TypeFixedValue = firstInt(r)
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
			v := value{}
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
		op.Reward = rewardOf(r)
	}
	if r, ok := child[colTingRewardData]; ok {
		op.TingReward = rewardOf(r)
	}
	if r, ok := child[colStringData]; ok {
		op.StringData = r.Texts()
	}
	return op
}

// rewardOf splits a reward row into its label and the remaining tuple.
func rewardOf(rec pvf.CTPRecord) *reward {
	out := &reward{}
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

func buildDuties(a *pvf.Archive) (*dutyConfig, error) {
	table, err := a.CTP(dutyTable)
	if err != nil {
		return nil, err
	}
	return &dutyConfig{
		Source:  table.Path,
		SHA256:  table.SHA256,
		Bytes:   table.Bytes,
		Records: table.Records,
		Trailer: table.Columns,
	}, nil
}

// verify refuses a table that no longer matches the shape the server relies on.
func verify(doc *config) error {
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
	col := findColumn(doc, colOperationData)
	if col == nil {
		return fmt.Errorf("trailer has no %s column", colOperationData)
	}
	if len(col.Records) != len(doc.Operations) {
		return fmt.Errorf("trailer lists %d operation blocks, derived %d", len(col.Records), len(doc.Operations))
	}
	return nil
}

func findColumn(doc *config, name string) *pvf.CTPColumn {
	for i := range doc.Trailer {
		if doc.Trailer[i].Name == name {
			return &doc.Trailer[i]
		}
	}
	return nil
}

func firstInt(r pvf.CTPRecord) *int64 {
	for _, c := range r.Cells {
		if c.Kind == "float" {
			v := int64(c.Float)
			return &v
		}
	}
	return nil
}

func formatClock(clock []phaseEntry) string {
	parts := make([]string, 0, len(clock))
	for _, e := range clock {
		parts = append(parts, fmt.Sprintf("phase%d=%gs", e.Phase, e.Seconds))
	}
	return strings.Join(parts, " ")
}

func opt(v *int64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}

func rewardLabel(r *reward) string {
	if r == nil {
		return "-"
	}
	return fmt.Sprintf("%s%v", r.Label, r.Values)
}
