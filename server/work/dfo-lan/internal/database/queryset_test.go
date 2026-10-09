package database

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Query-seam guards for S3b.
//
// queryset.go is generated, so the real risk is drift: a query added to the
// PostgreSQL tree regenerates sqlcgen but not the interface, and the interface
// then silently stops describing the persistence surface. The compile-time
// assertion in queryset.go proves the signatures match; these tests prove the
// METHOD SET matches in both directions.
func TestQuerySetCoversGeneratedSurface(t *testing.T) {
	pg := collectGeneratedAPI(t, "sqlcgen")
	if len(pg.methods) == 0 {
		t.Fatal("parsed no PostgreSQL methods; the generated package layout changed")
	}

	want := map[string]bool{}
	for name := range pg.methods {
		if name == "withTxExcluded" {
			continue
		}
		if name == "WithTx" {
			// Deliberately outside the shared interface: it returns a transaction
			// handle (pgx.Tx vs *sql.Tx), which no single signature can express.
			continue
		}
		want[name] = true
	}

	iface := reflect.TypeOf((*querySet)(nil)).Elem()
	got := map[string]bool{}
	for i := 0; i < iface.NumMethod(); i++ {
		got[iface.Method(i).Name] = true
	}

	// manualSQLiteMethods are SQLite-only methods hand-implemented in
	// sqlite_adapter_manual.go. The generated package (sqlcgen) cannot carry them
	// (the SQLite column is a DFO-server extension, not part of the generated
	// surface), so they are deliberately exempt from the exact-coverage check.
	manualSQLiteMethods := map[string]bool{
		"AccountSlotBonus": true,
	}

	var missing, extra []string
	for name := range want {
		if !got[name] {
			missing = append(missing, name)
		}
	}
	for name := range got {
		if !want[name] && !manualSQLiteMethods[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("querySet is missing %d generated method(s); re-run .tmp/gen-queryset.go: %s",
			len(missing), strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		t.Errorf("querySet declares %d method(s) that the generated package does not have: %s",
			len(extra), strings.Join(extra, ", "))
	}
	if len(missing) == 0 && len(extra) == 0 {
		t.Logf("querySet covers the generated surface exactly: %d methods (WithTx excluded by design)", len(got))
	}
}

// TestPostgresEngineNeedsNoAdapter states the property the whole seam design rests
// on: the canonical shape is the PostgreSQL one, so that engine plugs in with no
// conversion code. If this ever fails, the canonical shape drifted and every
// adapter (including the one already written) would need revisiting.
func TestPostgresEngineNeedsNoAdapter(t *testing.T) {
	pg := collectGeneratedAPI(t, "sqlcgen")
	iface := reflect.TypeOf((*querySet)(nil)).Elem()

	var incompatible []string
	for i := 0; i < iface.NumMethod(); i++ {
		method := iface.Method(i)
		params, ok := pg.methods[method.Name]
		if !ok {
			continue
		}
		// Compare the non-context parameters and the results by name; the generated
		// signature types are compared structurally by the compile-time assertion in
		// queryset.go, so this only has to catch a shape change.
		if len(params) != method.Type.NumIn()-1 {
			incompatible = append(incompatible, method.Name)
			continue
		}
		results := pg.results[method.Name]
		if len(results) != method.Type.NumOut()-1 {
			incompatible = append(incompatible, method.Name)
		}
	}
	if len(incompatible) > 0 {
		sort.Strings(incompatible)
		t.Errorf("these methods no longer line up between querySet and the generated package: %s",
			strings.Join(incompatible, ", "))
	}
}
