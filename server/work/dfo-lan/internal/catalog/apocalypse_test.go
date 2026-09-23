package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fixtureCatalog builds the smallest document the loader accepts, so each test
// can break exactly one invariant.
func fixtureCatalog() *ApocalypseCatalog {
	one := int64(1)
	return &ApocalypseCatalog{
		Source:      ApocalypseSource,
		SHA256:      ApocalypseChecksum,
		Bytes:       9280,
		Version:     1,
		RecordCount: 2,
		PoolTags:    []string{"[phase info]", "[operation data set]"},
		PhaseClock: []ApocalypsePhase{
			{Phase: 0, Seconds: 90},
			{Phase: 1, Seconds: 300},
			{Phase: 2, Seconds: 300},
			{Phase: 3, Seconds: 300},
			{Phase: 4, Seconds: 600},
			{Phase: 5, Seconds: 600},
		},
		Operations: []ApocalypseOperation{
			{Row: 1, Index: &one, Type: &one, AllowCoin: []int64{-1, 8}},
		},
		Duties: &ApocalypseDuties{
			Source: ApocalypseDutySource,
			SHA256: ApocalypseDutyChecksum,
			Bytes:  1636,
			Records: []pvf.CTPRecord{
				{Index: 0, Name: "[skirmisher info]", Parent: -1},
				{Index: 1, Name: "[gaurdian]", Parent: -1},
			},
		},
		Records: []pvf.CTPRecord{
			{Index: 0, Name: "[phase info]", Parent: -1},
			{Index: 1, Name: "[operation data set]", Parent: -1},
		},
	}
}

func writeFixture(t *testing.T, doc *ApocalypseCatalog) string {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "apocalypse.json")
	if err = os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestApocalypseCatalogLoads(t *testing.T) {
	cat, err := LoadApocalypseCatalog(writeFixture(t, fixtureCatalog()))
	if err != nil {
		t.Fatalf("fixture must load: %v", err)
	}
	if got := cat.PhaseDurations(); len(got) != 6 || got[0] != 90 || got[5] != 600 {
		t.Fatalf("phase durations %v", got)
	}
	if got := cat.TotalSeconds(); got != 90+300+300+300+600+600 {
		t.Fatalf("total seconds %g", got)
	}
	if s, ok := cat.PhaseSeconds(4); !ok || s != 600 {
		t.Fatalf("phase 4 = %g, %v", s, ok)
	}
	if _, ok := cat.PhaseSeconds(9); ok {
		t.Fatal("phase 9 must not exist")
	}
	op := cat.Operation(1)
	if op == nil {
		t.Fatal("operation 1 must exist")
	}
	if !op.AllowsCoin() {
		t.Fatal("operation 1 carries [allow coin]")
	}
	if cat.Operation(2) != nil {
		t.Fatal("operation 2 is not declared")
	}
	// Absence is not "deny": an undeclared block simply has no column.
	absent := ApocalypseOperation{}
	if absent.AllowsCoin() {
		t.Fatal("a block without the column must not report allow coin")
	}
	if names := cat.Duties.DutyNames(); len(names) != 2 {
		t.Fatalf("duty names %v", names)
	}
}

func TestApocalypseCatalogRejections(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ApocalypseCatalog)
	}{
		{"wrong source", func(c *ApocalypseCatalog) { c.Source = "contents/other.ctp" }},
		{"wrong checksum", func(c *ApocalypseCatalog) { c.SHA256 = "00" }},
		{"wrong version", func(c *ApocalypseCatalog) { c.Version = 2 }},
		{"record count lies", func(c *ApocalypseCatalog) { c.RecordCount = 5 }},
		{"no clock", func(c *ApocalypseCatalog) { c.PhaseClock = nil }},
		{"zero duration", func(c *ApocalypseCatalog) { c.PhaseClock[0].Seconds = 0 }},
		{"phases out of order", func(c *ApocalypseCatalog) { c.PhaseClock[1].Phase = 0 }},
		{"no operation", func(c *ApocalypseCatalog) { c.Operations = nil }},
		{"no duties", func(c *ApocalypseCatalog) { c.Duties = nil }},
		{"duty checksum", func(c *ApocalypseCatalog) { c.Duties.SHA256 = "00" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := fixtureCatalog()
			tc.mutate(doc)
			if _, err := LoadApocalypseCatalog(writeFixture(t, doc)); err == nil {
				t.Fatal("expected a load error")
			}
		})
	}
}

// TestCommittedApocalypseConfig pins the generated artifact: if the table is
// regenerated from a different client build the loader must refuse it, so this
// test fails loudly instead of letting the server run on a stale shape.
func TestCommittedApocalypseConfig(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "apocalypse.generated.json")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("generated config not present: %v", err)
	}
	cat, err := LoadApocalypseCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.PhaseClock) != 6 {
		t.Fatalf("committed clock has %d phases", len(cat.PhaseClock))
	}
	if len(cat.Operations) != 4 {
		t.Fatalf("committed table has %d operations", len(cat.Operations))
	}
	// Exactly one of the four release operations configures [allow coin]; the
	// other three must report absence rather than a zeroed column.
	configured := 0
	for i := range cat.Operations {
		if cat.Operations[i].AllowsCoin() {
			configured++
		}
	}
	if configured != 1 {
		t.Fatalf("%d operations configure [allow coin], want 1", configured)
	}
	if cat.Duties == nil || len(cat.Duties.Records) != 14 {
		t.Fatalf("committed duty table has %d records", len(cat.Duties.Records))
	}
}
