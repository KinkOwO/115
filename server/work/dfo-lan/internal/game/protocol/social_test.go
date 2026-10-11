package protocol

import (
	"bytes"
	"dfolan/internal/game/wire"
	"encoding/hex"
	"testing"
)

func TestSocialAddNativeRequest(t *testing.T) {
	raw, _ := hex.DecodeString("09000000e8b0b7e69ca8e8afad06000000313233313233000000000001050000004465656d6f730010038f3802000000")
	r, err := DecodeSocialAddRequest(raw)
	if err != nil || r.Adventure != "谷木语" || r.Memo != "123123" || r.Context != "" || r.Character != "Deemo" || r.Group != 0 || r.Server != 1 || r.Level != 115 || r.Profession != 16 || r.Growth != 3 || r.Extra != 0x2388f {
		t.Fatalf("native social request: %+v %v", r, err)
	}
	for _, end := range []int{0, 3, 12, 22, 28, 38, 45} {
		if _, err := DecodeSocialAddRequest(raw[:end]); err == nil {
			t.Errorf("accepted truncated request at %d", end)
		}
	}
	bad := append([]byte(nil), raw...)
	bad[0] = 0xff
	if _, err := DecodeSocialAddRequest(bad); err == nil {
		t.Fatal("accepted invalid string length")
	}
	bad = append([]byte(nil), raw...)
	bad[len(bad)-1] = 1
	if _, err := DecodeSocialAddRequest(bad); err == nil {
		t.Fatal("accepted nonzero padding")
	}
}

func TestSocialRefreshNativeFrame(t *testing.T) {
	raw, _ := hex.DecodeString("018d060d000000633735000a01")
	f, err := wire.ReadClient(bytes.NewReader(raw))
	if err != nil || f.ID != 1677 || len(raw) != wire.ClientHeaderSize {
		t.Fatalf("native refresh: %+v %v", f, err)
	}
}
