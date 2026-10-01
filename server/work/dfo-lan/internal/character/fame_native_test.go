package character

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestFameInstallOwnsRulesWithoutEmbeddedRead(t *testing.T) {
	var source FameRules
	if err := json.Unmarshal(fameRulesJSON, &source); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFameRules(source); err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	restore, err := InstallFameRules(&source)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	old := loadEmbeddedFameRules
	loadEmbeddedFameRules = func() (*fameRules, error) { return nil, fmt.Errorf("embedded read forbidden") }
	defer func() { loadEmbeddedFameRules = old }()
	installed, err := CurrentFameRules()
	if err != nil {
		t.Fatal(err)
	}
	for id, entry := range source.Items {
		item := inventory.BagEquipment{Record: make([]byte, 181)}
		binary.LittleEndian.PutUint32(item.Record[14:18], id)
		expected := entry.Value
		if expected <= 0 {
			expected = source.Tables[entry.Table][entry.Index]
		}
		if got := installed.enchant(item); got != expected+entry.Additional {
			t.Fatalf("enchant %d: %d", id, got)
		}
	}
	// Mutating every owned family must leave the runtime snapshot intact.
	clear(source.Sources)
	for _, rows := range source.Tables {
		clear(rows)
	}
	clear(source.Refine)
	clear(source.Refine115)
	clear(source.Items)
	for _, rows := range source.Sets {
		for i := range rows {
			rows[i] = fameThreshold{}
		}
	}
	for _, rows := range source.ItemPoints {
		for i := range rows {
			rows[i] = fameSetPoint{}
		}
	}
	for i := range source.Expanded {
		source.Expanded[i] = fameThreshold{}
	}
	for _, rows := range source.Awakening {
		clear(rows)
	}
	clear(source.MemoryRestore)
	clear(source.MemoryActivate)
	clear(source.MemoryRuminations)
	for _, rows := range source.SoleQuality {
		clear(rows)
	}
	for _, rows := range source.SolePenalty {
		clear(rows)
	}
	got, err := json.Marshal(installed)
	if err != nil || string(got) != string(want) {
		t.Fatal("installed rules share mutable source data", err)
	}
	if _, err := InstallFameRules(&source); err == nil {
		t.Fatal("accepted incomplete rules")
	}
	current, err := CurrentFameRules()
	if err != nil || current != installed {
		t.Fatal("failed install replaced current rules", err)
	}
	if _, err := InstallFameRules(nil); err == nil {
		t.Fatal("accepted nil rules")
	}
}

func TestFameSourceLastFieldAndNumericBounds(t *testing.T) {
	rows := fameSourceSections([]pvf.Token{{Type: 3, Text: "[fame value]"}, {Type: 0, Value: 1}, {Type: 3, Text: "[fame value]"}, {Type: 0, Value: 2}})
	if got := fameSourceField(rows, "[fame value]"); len(got) != 1 || got[0].Value != 2 {
		t.Fatal("last source field lost", got)
	}
	cells := []pvf.Token{{Type: 0, Value: 1}, {Type: 0, Value: 3}, {Type: 0, Value: 1}, {Type: 0, Value: 4}}
	pairs, err := fameSourcePairs[byte, int64](cells)
	if err != nil || !reflect.DeepEqual(pairs, map[byte]int64{1: 4}) {
		t.Fatal("duplicate pair semantics", pairs, err)
	}
	for _, bad := range [][]pvf.Token{{{Type: 0, Value: 256}, {Type: 0, Value: 4}}, {{Type: 0, Value: -1}, {Type: 0, Value: 4}}, {{Type: 0, Value: 1}}} {
		if _, err := fameSourcePairs[byte, int64](bad); err == nil {
			t.Fatal("accepted overflow or partial pair", bad)
		}
	}
	if numbers, err := fameSourceNumbers([]pvf.Token{{Type: 6, Text: "171361456285"}}); err != nil || numbers[0] != 171361456285 {
		t.Fatal("lost wide source integer", numbers, err)
	}
}
