package gamedata

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"testing"
)

// TestDerivedItemCacheSmoke covers the joint item cache "write + hit" loop
// without any PVF archive: a fixed key addresses a file in a temp directory,
// and a minimal JointItemCatalogs round-trips through encode/decode.
func TestDerivedItemCacheSmoke(t *testing.T) {
	dir := t.TempDir()
	key := [32]byte{0x11, 0x22, 0x33}
	path := derivedPath(dir, key)

	var out JointItemCatalogs
	if hit, err := loadDerived(path, key, func(d *json.Decoder) error { return decodeJointItems(d, &out) }); hit || err != nil {
		t.Fatalf("cold load: hit=%v err=%v", hit, err)
	}

	src := JointItemCatalogs{Basics: catalog.ItemBasics{
		Index: catalog.ItemIndex{Items: map[uint32]catalog.ItemIndexEntry{
			1001: {ID: 1001, Path: "list/equipment.lst", Kind: "equipment"},
			2002: {ID: 2002, Path: "list/stackable.lst", Kind: "stackable"},
		}},
	}}
	if err := saveDerived(path, key, func(e *json.Encoder) error { return encodeJointItems(e, src) }); err != nil {
		t.Fatalf("save: %v", err)
	}

	var got JointItemCatalogs
	hit, err := loadDerived(path, key, func(d *json.Decoder) error { return decodeJointItems(d, &got) })
	if !hit || err != nil {
		t.Fatalf("warm load: hit=%v err=%v", hit, err)
	}
	if len(got.Basics.Index.Items) != len(src.Basics.Index.Items) {
		t.Fatalf("items=%d want %d", len(got.Basics.Index.Items), len(src.Basics.Index.Items))
	}

	mismatch := key
	mismatch[0] = 2
	var rejected JointItemCatalogs
	if hit, err := loadDerived(path, mismatch, func(d *json.Decoder) error { return decodeJointItems(d, &rejected) }); hit || err == nil {
		t.Fatalf("identity mismatch accepted: hit=%v err=%v", hit, err)
	}
}
