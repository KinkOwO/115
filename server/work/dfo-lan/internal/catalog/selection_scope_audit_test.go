package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"reflect"
	"testing"
)

func TestSelectionScopeAuditLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_SCOPE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_SCOPE_TEST_ARCHIVE for native discovery and parser coverage")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_SCOPE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	index, err := ImportItemIndex(a)
	if err != nil {
		t.Fatal(err)
	}
	before := make(map[uint32]ItemIndexEntry, len(index.Items))
	for id, item := range index.Items {
		before[id] = item
	}
	full, err := AuditSelectionScope(a, index, len(index.Items))
	if err != nil {
		t.Fatal(err)
	}
	if full.Candidates == 0 || full.Candidates != full.Parsed+full.Fixed+full.Unparsed+full.Rejected || len(full.Issues) != full.Unparsed+full.Rejected {
		t.Fatalf("incomplete coverage counts: %+v", full)
	}
	for i, issue := range full.Issues {
		if issue.Template == 0 || issue.Path != index.Items[issue.Template].Path || issue.Reason == "" || i > 0 && issue.Template <= full.Issues[i-1].Template {
			t.Fatal("missing or unordered source evidence", issue)
		}
	}
	limited, err := AuditSelectionScope(a, index, 0)
	if err != nil {
		t.Fatal(err)
	}
	full.Issues = nil
	if !reflect.DeepEqual(full, limited) || !reflect.DeepEqual(index.Items, before) {
		t.Fatal("issue limit changed counts or audit mutated source index")
	}
	foreign := index
	foreign.Source.Checksum = "different-source"
	if _, err := AuditSelectionScope(a, foreign, 1); err == nil {
		t.Fatal("accepted a foreign source index")
	}
	if _, err := AuditSelectionScope(a, index, -1); err == nil {
		t.Fatal("accepted negative report limit")
	}
	t.Logf("native discovery: candidates=%d parsed=%d fixed=%d unparsed=%d rejected=%d", full.Candidates, full.Parsed, full.Fixed, full.Unparsed, full.Rejected)
}
