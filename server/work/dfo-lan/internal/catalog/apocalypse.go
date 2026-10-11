package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// ApocalypseSource and ApocalypseChecksum pin the exact client build the
// generated table was decoded from. A mismatch is a shape change and must stop
// loading rather than fall back to defaults.
const (
	ApocalypseSource   = "contents/2026/apocalypse/etc/apocalypse.ctp"
	ApocalypseChecksum = "d560876f683d921ba8bab42ac268c51e2a704ba1dabaf1236ad7eeca78a8dba7"

	ApocalypseDutySource   = "contents/2026/apocalypse/etc/dungeonskillinfo.ctp"
	ApocalypseDutyChecksum = "4353c3fbf682aab24ac99652e0d9fbfb615f6faa8a75d1bba287674b7bf47251"
)

// ApocalypsePhase is one entry of the [phase info] clock: the container stores
// (phase index, duration) pairs, six of them in the release table.
type ApocalypsePhase struct {
	Phase   int64   `json:"phase"`
	Seconds float64 `json:"seconds"`
}

// ApocalypseValue is one position of a mixed tuple, used where the meaning of a
// column is still positional.
type ApocalypseValue struct {
	Float *float64 `json:"float,omitempty"`
	Text  *string  `json:"text,omitempty"`
}

// ApocalypseReward keeps the label and the remaining tuple positionally: the
// leading value matches the item id range used elsewhere by the subsystem, the
// rest is not proven yet and must not be renamed into a guessed field.
type ApocalypseReward struct {
	Label  string    `json:"label"`
	Values []float64 `json:"values"`
}

// ApocalypseOperation is one [operation data set] block (four in the release
// table). A nil slice means the column is absent for that block, which is how
// the source expresses "not configured" — for example only operation 1 carries
// [allow coin].
type ApocalypseOperation struct {
	Row             int               `json:"row"`
	Index           *int64            `json:"index,omitempty"`
	Type            *int64            `json:"type,omitempty"`
	CardSymbolIndex *int64            `json:"cardSymbolIndex,omitempty"`
	MemberLimit     string            `json:"memberLimit,omitempty"`
	RecommendFame   *int64            `json:"recommendFame,omitempty"`
	TypeFixedValue  *int64            `json:"typeFixedValue,omitempty"`
	AllowCoin       []int64           `json:"allowCoin,omitempty"`
	GateSchedule    []int64           `json:"gateSchedule,omitempty"`
	GateFlow        []int64           `json:"gateFlow,omitempty"`
	GateCloseWarn   []ApocalypseValue `json:"gateCloseWarning,omitempty"`
	Reward          *ApocalypseReward `json:"reward,omitempty"`
	TingReward      *ApocalypseReward `json:"tingReward,omitempty"`
	StringData      []string          `json:"stringData,omitempty"`
}

// ApocalypseDuties is the sibling DungeonSkillInfo table: the skirmisher and
// guardian role definitions.
type ApocalypseDuties struct {
	Source  string          `json:"source"`
	SHA256  string          `json:"sha256"`
	Bytes   int             `json:"bytes"`
	Records []pvf.CTPRecord `json:"records"`
	Trailer []pvf.CTPColumn `json:"trailer"`
}

// RecordsOf returns every row carrying the given name, in file order.
func (d *ApocalypseDuties) RecordsOf(name string) []pvf.CTPRecord {
	var out []pvf.CTPRecord
	for _, r := range d.Records {
		if r.Name == name {
			out = append(out, r)
		}
	}
	return out
}

// ApocalypseCatalog is the decoded contents of apocalypse.ctp plus the duty
// table. The raw records and trailer are kept so the catalog is self-describing
// and later phases do not need a regeneration to reach a column.
type ApocalypseCatalog struct {
	Source      string                `json:"source"`
	SHA256      string                `json:"sha256"`
	Bytes       int                   `json:"bytes"`
	Version     uint32                `json:"version"`
	RecordCount uint32                `json:"recordCount"`
	PoolTags    []string              `json:"poolTags"`
	PhaseClock  []ApocalypsePhase     `json:"phaseClock"`
	Operations  []ApocalypseOperation `json:"operations"`
	Duties      *ApocalypseDuties     `json:"duties"`
	Records     []pvf.CTPRecord       `json:"records"`
	Trailer     []pvf.CTPColumn       `json:"trailer"`
}

// LoadApocalypseCatalog reads the generated table and refuses anything that no
// longer matches the pinned source shape.
func LoadApocalypseCatalog(path string) (*ApocalypseCatalog, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c ApocalypseCatalog
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	return ValidateApocalypseCatalog(&c)
}

func ValidateApocalypseCatalog(c *ApocalypseCatalog) (*ApocalypseCatalog, error) {
	if c == nil {
		return nil, fmt.Errorf("nil apocalypse catalog")
	}
	if c.Source != ApocalypseSource || c.SHA256 != ApocalypseChecksum {
		return nil, fmt.Errorf("apocalypse source mismatch: %s %s", c.Source, c.SHA256)
	}
	if c.Version != 1 {
		return nil, fmt.Errorf("apocalypse table version %d", c.Version)
	}
	if int(c.RecordCount) != len(c.Records) {
		return nil, fmt.Errorf("apocalypse declares %d records but carries %d", c.RecordCount, len(c.Records))
	}
	if len(c.PhaseClock) == 0 {
		return nil, fmt.Errorf("apocalypse table has no phase clock")
	}
	for i, p := range c.PhaseClock {
		if p.Seconds <= 0 {
			return nil, fmt.Errorf("phase %d has a non-positive duration %g", p.Phase, p.Seconds)
		}
		if i > 0 && p.Phase <= c.PhaseClock[i-1].Phase {
			return nil, fmt.Errorf("phase indices are not increasing at %d", i)
		}
	}
	if len(c.Operations) == 0 {
		return nil, fmt.Errorf("apocalypse table has no operation block")
	}
	if c.Duties == nil {
		return nil, fmt.Errorf("apocalypse table is missing the duty table")
	}
	if c.Duties.Source != ApocalypseDutySource || c.Duties.SHA256 != ApocalypseDutyChecksum {
		return nil, fmt.Errorf("duty source mismatch: %s %s", c.Duties.Source, c.Duties.SHA256)
	}
	return c, nil
}

// PhaseDurations returns the phase durations in phase order.
func (c *ApocalypseCatalog) PhaseDurations() []float64 {
	out := make([]float64, 0, len(c.PhaseClock))
	for _, p := range c.PhaseClock {
		out = append(out, p.Seconds)
	}
	return out
}

// PhaseSeconds returns the duration of one phase.
func (c *ApocalypseCatalog) PhaseSeconds(phase int64) (float64, bool) {
	for _, p := range c.PhaseClock {
		if p.Phase == phase {
			return p.Seconds, true
		}
	}
	return 0, false
}

// TotalSeconds is the sum of every phase duration.
func (c *ApocalypseCatalog) TotalSeconds() float64 {
	var total float64
	for _, p := range c.PhaseClock {
		total += p.Seconds
	}
	return total
}

// Operation returns the block with the given [index] value.
func (c *ApocalypseCatalog) Operation(index int64) *ApocalypseOperation {
	for i := range c.Operations {
		if c.Operations[i].Index != nil && *c.Operations[i].Index == index {
			return &c.Operations[i]
		}
	}
	return nil
}

// AllowsCoin reports whether the block carries an [allow coin] column at all.
// Absence is meaningful: only one of the four release operations configures it.
func (o *ApocalypseOperation) AllowsCoin() bool {
	return len(o.AllowCoin) > 0
}

// DutyNames lists the role names the duty table defines, in file order.
func (d *ApocalypseDuties) DutyNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range d.Records {
		if r.Parent != -1 || seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		out = append(out, r.Name)
	}
	sort.Strings(out)
	return out
}
