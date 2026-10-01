package catalog

import (
	"container/list"
	"dfolan/internal/catalog/pvf"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

func TestMapCacheEvictionKeepsActiveReadsAndSourceOrder(t *testing.T) {
	c := &mapScriptCache{entries: map[string]*list.Element{}}
	reads := map[string]int{}
	load := func(path string) (ScriptRecord, error) {
		reads[path]++
		return ScriptRecord{Path: path, SHA256: path, Cells: []pvf.Token{{Type: 0, Value: 1}}}, nil
	}
	read := func(path string) ScriptRecord {
		s, err := c.readWith(ScriptRecord{Path: path, SHA256: path}, load)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	active := read("map0")
	for i := 1; i < 128; i++ {
		read(fmt.Sprintf("map%d", i))
	}
	read("map0") // Promote a map reused by an active run.
	read("map128")
	read("map0")
	if reads["map0"] != 1 {
		t.Fatal("active map was discarded by bulk cache reset", reads)
	}
	read("map1")
	if reads["map1"] != 2 || c.order.Len() != 128 || c.bytes > dungeonMapCacheBytes {
		t.Fatal("cache did not evict only LRU", reads["map1"], c.bytes)
	}
	if !reflect.DeepEqual(active.Cells, []pvf.Token{{Type: 0, Value: 1}}) {
		t.Fatal("eviction mutated a caller's script")
	}
}

func TestMapCacheByteBudgetAndClosedSource(t *testing.T) {
	c := &mapScriptCache{entries: map[string]*list.Element{}}
	load := func(path string) (ScriptRecord, error) {
		return ScriptRecord{Path: path, SHA256: path, Cells: []pvf.Token{{Text: strings.Repeat("x", 16*1024*1024)}}}, nil
	}
	first, err := c.readWith(ScriptRecord{Path: "first", SHA256: "first"}, load)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.readWith(ScriptRecord{Path: "second", SHA256: "second"}, load); err != nil {
		t.Fatal(err)
	}
	if c.order.Len() != 1 || c.bytes > dungeonMapCacheBytes {
		t.Fatal("byte budget did not evict large map")
	}
	d := DungeonCatalog{mapScripts: c, Maps: map[uint32]ScriptRecord{1: {Path: "second", SHA256: "second"}}}
	if err = d.CloseMapSource(); err != nil {
		t.Fatal(err)
	}
	if _, err = d.MapScript(1); err == nil {
		t.Fatal("cached map survived source close")
	}
	if len(first.Cells[0].Text) != 16*1024*1024 {
		t.Fatal("close mutated script held by a caller")
	}
}

func TestMapCacheRefusesCorruptOrUnavailableSourcesAndOversizedRetention(t *testing.T) {
	c := &mapScriptCache{entries: map[string]*list.Element{}}
	expected := ScriptRecord{Path: "map/test.map", SHA256: "expected"}
	failure := errors.New("source unavailable")
	if _, err := c.readWith(expected, func(string) (ScriptRecord, error) { return ScriptRecord{}, failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if _, err := c.readWith(expected, func(string) (ScriptRecord, error) { return ScriptRecord{SHA256: "different"}, nil }); err == nil {
		t.Fatal("accepted wrong map source hash")
	}
	if c.order.Len() != 0 {
		t.Fatal("cached a failed read")
	}
	large := ScriptRecord{Path: expected.Path, SHA256: expected.SHA256, Cells: make([]pvf.Token, dungeonMapCacheBytes/int64(unsafe.Sizeof(pvf.Token{}))+1)}
	for i := 0; i < 2; i++ {
		got, err := c.readWith(expected, func(string) (ScriptRecord, error) { return large, nil })
		if err != nil || len(got.Cells) != len(large.Cells) {
			t.Fatal("refused valid large map", err)
		}
	}
	if c.bytes != 0 || c.order.Len() != 0 {
		t.Fatal("retained oversized map")
	}
}
