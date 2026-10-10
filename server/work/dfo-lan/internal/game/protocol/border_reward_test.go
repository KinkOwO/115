package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestBorderRewardMatchesNativeReaderVectors(t *testing.T) {
	b, err := os.ReadFile("testdata/border-reward-native.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vectors []struct {
			Name           string
			Primary, Bonus uint32
			Payload        string
			Consumed       int
			NativeMaximum  uint32 `json:"native_maximum"`
		}
	}
	if err = json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, v := range doc.Vectors {
		if v.Bonus != 40 {
			continue
		}
		p, err := BorderRewardInfo(v.Primary)
		if err != nil || hex.EncodeToString(p) != v.Payload || v.Consumed != len(p) || v.NativeMaximum != v.Primary {
			t.Fatalf("native %s: %x %v", v.Name, p, err)
		}
		n++
	}
	if n != 4 {
		t.Fatal("native vectors missing")
	}
	for _, invalid := range []uint32{0, 39, 46, 72, 0xffffffff} {
		if _, err := BorderRewardInfo(invalid); err == nil {
			t.Fatal("invalid grade emitted")
		}
	}
}
