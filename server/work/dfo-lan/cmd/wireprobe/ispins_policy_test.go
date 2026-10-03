package main

import (
	"bytes"
	"dfolan/internal/legion"
	"testing"
)

func TestIspinsModesValidateAndKeepUnlimitedLoginBytes(t *testing.T) {
	for _, m := range []string{"", "unlimited", "weekly", "typo"} {
		t.Setenv("DFO_ISPINS_MODE", m)
		limited, e := ispinsWeeklyLimited()
		if (m == "typo") != (e != nil) || limited != (m == "weekly") {
			t.Fatal(m, limited, e)
		}
	}
	t.Setenv("DFO_ISPINS_MODE", "unlimited")
	w := &worldSession{}
	got, e := w.ispinsLoginQuotaInfo()
	want, _ := legion.IspinsEntryCharacterInfo(true, [4]bool{}, [5]byte{})
	if e != nil || !bytes.Equal(got, want) {
		t.Fatal("unlimited login grammar changed", e)
	}
	t.Setenv("DFO_ISPINS_MODE", "weekly")
	if e := w.checkIspinsWeeklyAdmission(); e == nil {
		t.Fatal("weekly mode without persistent storage allowed entry")
	}
}
