package catalog

import (
	"os"
	"testing"
)

func TestAsrahanRaidNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF archive required")
	}
	a, err := OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := importAsrahanRaidRules(a, "contents/2024/asrahanraid/etc/asrahan.etc")
	if err != nil {
		t.Fatal(err)
	}
	if r.Normal.TimeoutSeconds != 900 || r.Hard.TimeoutSeconds != 600 || len(r.Normal.Events) != 98 || len(r.Hard.Events) != 90 || len(r.Groups) != 9 || len(r.Normal.Dungeons) != 18 || len(r.Hard.Dungeons) != 18 || r.HardPartyMax != 3 || r.HardLimitFame != 30135 {
		t.Fatalf("source mode separation incomplete: normal=%+v hard=%+v groups=%d", r.Normal, r.Hard, len(r.Groups))
	}
	s, err := ResolveScript(a, r.Path)
	if err != nil {
		t.Fatal(err)
	}
	sym, err := ResolveScript(a, r.SymbolPath)
	if err != nil {
		t.Fatal(err)
	}
	// Source mutation must affect only the corresponding mode.
	for i, tok := range s.Cells {
		if tok.Text == "[HARD PHASE TIME OVER]" {
			s.Cells[i+1].Value = 777
			break
		}
	}
	changed, err := ParseAsrahanRaidRules(s, sym)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Hard.TimeoutSeconds != 777 || changed.Normal.TimeoutSeconds != r.Normal.TimeoutSeconds {
		t.Fatal("hard mode uses duplicated timeout or shared phase")
	}
	t.Logf("Asrahan source verified: symbols=%d groups=%d normal events=%d hard events=%d", len(r.Symbols), len(r.Groups), len(r.Normal.Events), len(r.Hard.Events))
}
