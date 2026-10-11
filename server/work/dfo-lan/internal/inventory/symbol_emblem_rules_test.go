package inventory

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSymbolEmblemRulesRejectMissingOrUnboundSource(t *testing.T) {
	_, e := ImportSymbolEmblemRules(nil)
	require.Error(t, e)
	_, e = ParseSymbolEmblemRules("", nil)
	require.Error(t, e)
	_, e = ParseSymbolEmblemRules(strings.Repeat("a", 64), nil)
	require.Error(t, e)
	var nilRules *SymbolEmblemRules
	_, e = nilRules.FixedAt(0, 0, 1)
	require.Error(t, e)
	_, e = nilRules.ResonanceAt(0, 2)
	require.Error(t, e)
}

func TestSymbolEmblemCurrentSourceAndSourceChanges(t *testing.T) {
	root := os.Getenv("US115_CTP_EVIDENCE")
	if root == "" {
		t.Skip("explicit read-only current CN evidence required")
	}
	manifestRaw, e := os.ReadFile(filepath.Join(root, "manifest.json"))
	require.NoError(t, e)
	var manifest struct {
		Source string `json:"source_sha256"`
	}
	require.NoError(t, json.Unmarshal(manifestRaw, &manifest))
	tables := map[string]*pvf.CTPTable{}
	for _, name := range []string{"symbolemblemsystem.ctp", "symboloptioninfoscript.ctp", "symbolemblemresonancesystem.ctp"} {
		path := SymbolEmblemSourceDirectory + name
		raw, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, e)
		table, e := pvf.ReadCTP(path, raw)
		require.NoError(t, e)
		tables[name] = table
	}
	r, e := ParseSymbolEmblemRules(manifest.Source, tables)
	require.NoError(t, e)
	require.Len(t, r.Tiers, 20)
	require.Len(t, r.Fixed, 80)
	require.Len(t, r.Resonance, 5)
	require.Len(t, r.ResonanceIndices, 16)
	require.Len(t, r.Steps, 15)
	fixed, e := r.FixedAt(0, 0, 1)
	require.NoError(t, e)
	require.Equal(t, 10.0, fixed.Buffer["[equipment buff]"])
	resonance, e := r.ResonanceAt(0, 2)
	require.NoError(t, e)
	require.Equal(t, 10.0, resonance.Buffer["[equipment buff]"])
	require.EqualValues(t, 0, r.ResonanceIndices[SymbolResonanceKey{0, 2}], "index zero is a real source row")
	_, e = r.FixedAt(0, 0, 99)
	require.Error(t, e)
	_, e = r.ResonanceAt(0, 99)
	require.Error(t, e)
	fixed.Buffer["[equipment buff]"] = 999
	next, e := r.FixedAt(0, 0, 1)
	require.NoError(t, e)
	require.Equal(t, 10.0, next.Buffer["[equipment buff]"], "caller must not mutate source projection")
	option := tables["symboloptioninfoscript.ctp"]
	changed := false
	for i := range option.Records {
		row := &option.Records[i]
		if !changed && row.Name == "[equipment buff]" {
			row.Cells[0].Float = 17
			changed = true
		}
	}
	require.True(t, changed)
	updated, e := ParseSymbolEmblemRules(manifest.Source, tables)
	require.NoError(t, e)
	next, e = updated.FixedAt(0, 0, 1)
	require.NoError(t, e)
	require.Equal(t, 17.0, next.Buffer["[equipment buff]"])
	seen := false
	for i := range option.Records {
		row := &option.Records[i]
		if row.Name == "[index]" {
			if seen {
				row.Cells[0].Float = 1
				break
			}
			seen = true
		}
	}
	_, e = ParseSymbolEmblemRules(manifest.Source, tables)
	require.ErrorContains(t, e, "duplicate symbol option")
	t.Logf("source tiers=20 fixed=80 resonance=5 indices=16 steps=15; zero index and source-change checks passed")
}
