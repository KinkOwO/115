package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestSourceAttributesMatchNativeLoader(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile("testdata/native_source_stats.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Profession byte
		SHA        string `json:"source_sha256"`
		Wire       string `json:"wire_hex"`
	}
	if e = json.Unmarshal(raw, &rows); e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		p := c.Professions[r.Profession]
		if p.RawSHA256 != r.SHA {
			t.Fatal("fixture source changed")
		}
		state, _ := json.Marshal(State{Level: 1, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills, SourceSHA256: p.RawSHA256})
		got, e := (&Service{Catalog: c}).EntryAddition(storage.Character{WireID: 3, Profession: r.Profession, State: state})
		if e != nil {
			t.Fatal(e)
		}
		want, _ := hex.DecodeString(r.Wire)
		if !bytes.Equal(got[269:360], want) {
			t.Fatalf("profession %d differs from native PVF loader\ngot %x\nwant%x", r.Profession, got[269:360], want)
		}
	}
}
