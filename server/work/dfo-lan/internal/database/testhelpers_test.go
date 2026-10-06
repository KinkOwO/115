package database

import (
	"encoding/json"
	"reflect"
	"testing"
)

// sameJSON reports whether two JSON documents are equal as values (key order and
// whitespace do not matter).
//
// It used to live in store_postgres_test.go, but it is engine-neutral and is used
// by tests that no longer have anything to do with PostgreSQL, so it moved here
// when the PostgreSQL-only test files were removed (owner decision 2026-10-05,
// see root AGENTS.md §0.6).
func sameJSON(t *testing.T, a, b json.RawMessage) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(av, bv)
}
