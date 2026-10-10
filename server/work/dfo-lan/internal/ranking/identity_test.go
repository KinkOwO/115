package ranking

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestUserKeyNativeFilenameAndAccountIsolation(t *testing.T) {
	a, err := UserKey(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 33 || a[32] != 0 {
		t.Fatalf("native fixed string: %x", a)
	}
	if _, err := hex.DecodeString(string(a[:32])); err != nil {
		t.Fatal("identity is not a safe filename component", err)
	}
	again, _ := UserKey(1)
	other, _ := UserKey(2)
	if !bytes.Equal(a, again) || bytes.Equal(a, other) {
		t.Fatal("ranking identity lost account isolation or reconnect stability")
	}
	for _, account := range []int64{0, -1} {
		if _, err := UserKey(account); err == nil {
			t.Fatal("invalid account received a ranking identity")
		}
	}
}
