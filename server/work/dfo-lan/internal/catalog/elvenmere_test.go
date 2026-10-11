package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"strings"
	"testing"
)

const elvenmereTestSource = `[max floor] 2
[weekly reward]
1 7 5
2 8 9
[/weekly reward]
[season reward]
2 10 1
[/season reward]
[floor clear exp]
1 0
2 23
[/floor clear exp]`

func TestElvenmereRewardsFollowSourceAndRejectIncompleteRules(t *testing.T) {
	r, err := parseElvenmereRules(elvenmereTestSource)
	if err != nil || r.Floors[2].Weekly.Count != 9 || r.Floors[1].Experience != 0 {
		t.Fatal(r, err)
	}
	changed, err := parseElvenmereRules(strings.ReplaceAll(elvenmereTestSource, "2 8 9", "2 81 19"))
	if err != nil || changed.Floors[2].Weekly.Template != 81 || changed.Floors[2].Weekly.Count != 19 {
		t.Fatal(changed, err)
	}
	for _, bad := range []string{
		strings.ReplaceAll(elvenmereTestSource, "2 8 9", ""),
		strings.ReplaceAll(elvenmereTestSource, "2 8 9", "1 8 9"),
		strings.ReplaceAll(elvenmereTestSource, "2 23", "2 -1"),
		strings.ReplaceAll(elvenmereTestSource, "[max floor] 2", "[max floor] 101"),
		strings.ReplaceAll(elvenmereTestSource, "[/weekly reward]", ""),
		strings.ReplaceAll(elvenmereTestSource, "2 10 1", "2 10 0"),
	} {
		if _, err := parseElvenmereRules(bad); err == nil {
			t.Fatal("invalid rules accepted", bad)
		}
	}
}

func TestElvenmereCurrentPVFRewardAudit(t *testing.T) {
	path := os.Getenv("DFO_FAME_AUDIT_PVF")
	if path == "" {
		t.Skip("requires current PVF audit archive")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 2 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	r, err := ImportElvenmereRules(a)
	if err != nil {
		t.Fatal(err)
	}
	if r.MaxFloor != 100 || len(r.Floors) != 100 || r.SHA256 == "" {
		t.Fatal("incomplete current source", r)
	}
	seasons := 0
	for floor, row := range r.Floors {
		if row.Experience != 0 {
			t.Fatalf("current source floor %d experience %d", floor, row.Experience)
		}
		if row.Season.Template != 0 {
			seasons++
		}
	}
	if seasons != 20 || r.Floors[35].Weekly.Count != 30 || r.Floors[100].Weekly.Count != 150 {
		t.Fatal("current source rewards mismatch")
	}
	t.Logf("pvf=%s rules=%s sha256=%s floors=%d season_rows=%d all_experience_zero=true", a.Snapshot().Checksum, r.Path, r.SHA256, len(r.Floors), seasons)
}
