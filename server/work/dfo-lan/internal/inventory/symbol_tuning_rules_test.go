package inventory

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func loadCurrentSymbolTables(t *testing.T) (string, map[string]*pvf.CTPTable) {
	t.Helper()
	root := os.Getenv("US115_CTP_EVIDENCE")
	if root == "" {
		t.Skip("explicit read-only current CN evidence required")
	}
	raw, e := os.ReadFile(filepath.Join(root, "manifest.json"))
	require.NoError(t, e)
	var manifest struct {
		Source string `json:"source_sha256"`
	}
	require.NoError(t, json.Unmarshal(raw, &manifest))
	tables := map[string]*pvf.CTPTable{}
	for _, name := range []string{"symbolemblemsystem.ctp", "symboloptioninfoscript.ctp", "symbolemblemresonancesystem.ctp"} {
		path := SymbolEmblemSourceDirectory + name
		raw, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, e)
		tables[name], e = pvf.ReadCTP(path, raw)
		require.NoError(t, e)
	}
	return manifest.Source, tables
}

func TestSymbolTuningSourceChangesAndRejectsIncompleteCompensation(t *testing.T) {
	source, tables := loadCurrentSymbolTables(t)
	rules, e := ParseSymbolEmblemRules(source, tables)
	require.NoError(t, e)
	require.EqualValues(t, 100, rules.Tuning.DuplicateButtonBonusRate)
	require.EqualValues(t, 300, rules.Tuning.MultiColorCost)
	require.EqualValues(t, 100, rules.Tuning.MultiColorAppearanceRate)
	require.Equal(t, map[uint32]uint32{1: 3500, 2: 3000, 3: 2500, 4: 2000}, rules.Tuning.ComboRates)
	require.EqualValues(t, 490719431, rules.Tuning.FailRewardItem)
	require.EqualValues(t, 1, rules.Tuning.FailRewardCount)
	require.Equal(t, "New_EmblemRemodel_mail_title_00", rules.Tuning.FailMailTitleKey)
	require.Equal(t, "New_EmblemRemodel_mail_msg_00", rules.Tuning.FailMailTextKey)
	table := tables["symbolemblemresonancesystem.ctp"]
	for i := range table.Records {
		row := &table.Records[i]
		switch row.Name {
		case "[multi color cost]":
			row.Cells[0].Float = 407
		case "[fail reward]":
			row.Cells[1].Float = 3
		}
	}
	changed, e := ParseSymbolEmblemRules(source, tables)
	require.NoError(t, e)
	require.EqualValues(t, 407, changed.Tuning.MultiColorCost)
	require.EqualValues(t, 3, changed.Tuning.FailRewardCount)
	require.EqualValues(t, 300, rules.Tuning.MultiColorCost, "prior source projection is independent")

	for _, tc := range []struct {
		name    string
		mutate  func(*pvf.CTPTable)
		message string
	}{
		{"missing cost", func(x *pvf.CTPTable) {
			for i := range x.Records {
				if x.Records[i].Name == "[multi color cost]" {
					x.Records[i].Cells = nil
				}
			}
		}, "invalid multi color cost"},
		{"zero reward", func(x *pvf.CTPTable) {
			for i := range x.Records {
				if x.Records[i].Name == "[fail reward]" {
					x.Records[i].Cells[1].Float = 0
				}
			}
		}, "invalid resonance fail reward"},
		{"mail contains NUL", func(x *pvf.CTPTable) {
			for i := range x.Records {
				if x.Records[i].Name == "[fail reward mail]" {
					x.Records[i].Cells[0].Name = "title\x00other"
				}
			}
		}, "invalid resonance fail mail key"},
		{"duplicate combo step", func(x *pvf.CTPTable) {
			for i := range x.Records {
				if x.Records[i].Name == "[step]" && x.Records[i].Cells[0].Float == 2 {
					x.Records[i].Cells[0].Float = 1
					break
				}
			}
		}, "duplicate resonance combo step"},
		{"missing interior step", func(x *pvf.CTPTable) {
			for i := range x.Records {
				if x.Records[i].Name == "[step]" && x.Records[i].Cells[0].Float == 2 {
					x.Records[i].Cells[0].Float = 8
					break
				}
			}
		}, "noncontiguous resonance combo steps"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, tables := loadCurrentSymbolTables(t)
			tc.mutate(tables["symbolemblemresonancesystem.ctp"])
			_, e := ParseSymbolEmblemRules(source, tables)
			require.ErrorContains(t, e, tc.message)
		})
	}
	t.Log("native tuning fields and compensation preserved; source changes, malformed cost/reward/mail and combo gaps checked; no executor enabled")
}

func TestSymbolIntegerCellsRejectNonFiniteAndFractionalValues(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 0.5, float64(math.MaxUint32) + 1} {
		_, e := symbolIntegers(pvf.CTPRecord{Name: "test", Cells: []pvf.CTPCell{{Kind: "float", Float: v}}})
		require.Error(t, e)
	}
}
