package gamedata

import (
	"dfolan/internal/catalog"
	"math"
	"os"
	"strings"
	"testing"
)

// Verify the built client/server archive, including list registration and every
// native enchant value, against the imported CN CTP rules in the same archive.
func TestSymbolNativeOptionSourceParity(t *testing.T) {
	p := os.Getenv("US115_SYMBOL_PVF")
	if p == "" {
		t.Skip("set US115_SYMBOL_PVF to the paired inner archive")
	}
	s, err := Open(Options{Mode: PVF, ArchivePath: p})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	l, err := s.SymbolSocketLayout()
	if err != nil {
		t.Fatal(err)
	}
	if l == nil || l.Rules == nil {
		t.Fatal("missing symbol rules")
	}
	indexScript, err := catalog.ResolveScript(s.archive, "list/stackable.lst")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := catalog.ParseIndex(indexScript.Cells)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[uint32]string{}
	for _, r := range rows {
		paths[r.ID] = r.Path
	}
	checked := 0
	for category, templates := range []map[uint32][2]uint32{l.FixedTemplates, l.ResonanceTemplates} {
		for key, pair := range templates {
			options := l.Rules.Fixed[key]
			if category == 1 {
				options = l.Rules.Resonance[key]
			}
			for mode, id := range pair {
				path := paths[id]
				if path == "" {
					t.Fatalf("unregistered option %d", id)
				}
				if !strings.HasPrefix(path, "stackable/") {
					path = "stackable/" + path
				}
				script, err := catalog.ResolveScript(s.archive, path)
				if err != nil {
					t.Fatal(err)
				}
				want := options.Dealer
				if mode == 1 {
					want = options.Buffer
				}
				seen := map[string]bool{}
				enchant := false
				for i, c := range script.Cells {
					if c.Text == "[enchant]" {
						enchant = true
						continue
					}
					if c.Text == "[/enchant]" {
						enchant = false
						continue
					}
					if !enchant || c.Type != 3 {
						continue
					}
					value, ok := want[c.Text]
					if !ok || i+1 >= len(script.Cells) {
						t.Fatalf("unexpected native option %d %s", id, c.Text)
					}
					n := script.Cells[i+1]
					if n.Type != 2 || uint32(n.Value) != math.Float32bits(float32(value)) {
						t.Fatalf("native value mismatch %d %s: %+v != %g", id, c.Text, n, value)
					}
					seen[c.Text] = true
				}
				if len(seen) != len(want) {
					t.Fatalf("missing native options %d", id)
				}
				checked++
			}
		}
	}
	if checked != 170 {
		t.Fatalf("unexpected source option coverage %d", checked)
	}
}
