package character

import "testing"

func TestIspinsRosterEligibilityUsesCompletedPrerequisite(t *testing.T) {
	for _, ids := range [][]uint16{nil, {13714}, {13762}} {
		if f := contentClearFlagsForQuests(ids); f[11] != 0 {
			t.Fatal("unfinished prerequisite granted entry", ids)
		}
	}
	f := contentClearFlagsForQuests([]uint16{13763})
	for i, v := range f {
		want := byte(0)
		if i == 11 {
			want = 1
		}
		if v != want {
			t.Fatalf("flag%d=%d want%d", i, v, want)
		}
	}
	all := contentClearFlagsForQuests([]uint16{12167, 12312, 12392, 12422, 13763})
	for _, i := range []int{5, 6, 8, 9, 11} {
		if all[i] != 1 {
			t.Fatalf("existing gate%d lost", i)
		}
	}
}
