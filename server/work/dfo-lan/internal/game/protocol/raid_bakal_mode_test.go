package protocol

import "testing"

func TestBakalModeUsesNativeFifthOption(t *testing.T) {
	for _, mode := range []byte{0, 1} {
		r := RaidCreateRequest{Kind: 8, Options: [6]byte{0, 0, 0, 7, mode, 0}}
		got, err := r.BakalMode115()
		if err != nil || got != mode {
			t.Fatal(got, err)
		}
	}
	for _, r := range []RaidCreateRequest{{Kind: 8, Options: [6]byte{0, 0, 0, 0, 2, 0}}, {Kind: 8, Options: [6]byte{0, 0, 0, 0, 3, 0}}, {Kind: 9}} {
		if _, err := r.BakalMode115(); err == nil {
			t.Fatal("foreign raid variant accepted", r)
		}
	}
}
