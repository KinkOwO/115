package catalog

import (
	"dfolan/internal/catalog/pvf"
	"reflect"
	"runtime"
	"testing"
	"weak"
)

func TestJointSkinResultDoesNotRetainImportOwner(t *testing.T) {
	makeResult := func() (*SkinStorageCatalog, weak.Pointer[jointSkinImport]) {
		owner := &jointSkinImport{result: SkinStorageCatalog{Entries: []SkinStorageEntry{{Template: 7}}}}
		return owner.finish(), weak.Make(owner)
	}
	result, owner := makeResult()
	for i := 0; i < 5; i++ {
		runtime.GC()
		if owner.Value() == nil {
			break
		}
	}
	if owner.Value() != nil {
		t.Fatal("published skin catalog still retains import owner and its archive")
	}
	if len(result.Entries) != 1 || result.Entries[0].Template != 7 {
		t.Fatal("released published data")
	}
	runtime.KeepAlive(result)
}

func TestJointSkinConsumerKeepsFallbackAndMissingOrder(t *testing.T) {
	b := &jointSkinImport{skins: map[uint32]string{9: "skin/test.skn"}, types: map[uint32]skinLabels{9: {Type: "damage font"}}, result: SkinStorageCatalog{}}
	for _, row := range []struct{ template, skin uint32 }{{3, 12}, {1, 9}, {2, 11}} {
		// A non-exact STK is allowed by skin registration, unlike booster/fame.
		s := ItemScript{Item: ItemIndexEntry{ID: row.template}, Exact: false, Cells: []pvf.Token{{Type: 3, Text: "[action type]"}, {Type: 6, Text: AddSkinStorageLabel}, {Type: 0, Value: int32(row.skin)}}}
		if err := b.consume(s); err != nil {
			t.Fatal(err)
		}
	}
	got := b.finish()
	if len(got.Entries) != 1 || got.Entries[0].Template != 1 || got.Entries[0].SkinType != "damage font" || !reflect.DeepEqual(got.MissingSkins, []MissingSkin{{Template: 3, SkinID: 12}, {Template: 2, SkinID: 11}}) {
		t.Fatal(got)
	}
}

func TestJointBoosterConsumerRefusesFallbackAndKeepsSealedMarker(t *testing.T) {
	b := &jointBoosterImport{result: map[uint32]BoosterDefinition{}}
	s := ItemScript{Item: ItemIndexEntry{ID: 5, Kind: "stackable", Path: "stackable/test.stk", StackableType: "[material]"}, Cells: []pvf.Token{{Type: 3, Text: "[booster info]"}}}
	if err := b.consume(s); err == nil {
		t.Fatal("accepted namesake fallback even for non-booster")
	}
	s.Exact = true
	if err := b.consume(s); err != nil {
		t.Fatal(err)
	}
	if marker, ok := b.result[5]; !ok || len(marker.Pools) != 0 || b.sealed != 1 {
		t.Fatal("lost sealed marker", b)
	}
	s.Item.StackableType = "[booster]"
	s.Item.ID = 6
	if err := b.consume(s); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.result[6]; ok || b.unresolved != 1 {
		t.Fatal("unresolved body became payable", b)
	}
}
