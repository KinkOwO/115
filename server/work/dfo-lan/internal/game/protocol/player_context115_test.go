package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestPlayerContext115NativeVector(t *testing.T) {
	var v struct {
		Request  string `json:"request_payload"`
		Response string `json:"response_payload"`
		Consumed int    `json:"consumed"`
		Opened   bool   `json:"normal_menu_resolver_called"`
	}
	raw, err := os.ReadFile("testdata/native_player_context115.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	request, _ := hex.DecodeString(v.Request)
	for _, p := range [][]byte{request, append(bytes.Clone(request), 0, 0, 0)} {
		if actor, err := DecodePlayerContext115(p); err != nil || actor != 7 {
			t.Fatalf("native request: %d %v", actor, err)
		}
	}
	want, _ := hex.DecodeString(v.Response)
	got, err := PlayerContextReply115(7, 3, 65535)
	if err != nil || !bytes.Equal(got, want) || len(got) != v.Consumed || !v.Opened {
		t.Fatalf("native response %x %v", got, err)
	}
	for n := 0; n <= 17; n++ {
		if n == 13 || n == 16 {
			continue
		}
		p := make([]byte, n)
		copy(p, request)
		if _, err := DecodePlayerContext115(p); err == nil {
			t.Fatalf("accepted size %d", n)
		}
	}
	for _, offset := range []int{2, 3, 7, 9, 13, 15} {
		p := append(bytes.Clone(request), 0, 0, 0)
		p[offset]++
		if _, err := DecodePlayerContext115(p); err == nil {
			t.Fatalf("accepted unsupported field %d", offset)
		}
	}
	for _, ids := range [][3]uint16{{0, 3, 65535}, {65535, 3, 65535}, {7, 0, 65535}, {7, 65535, 65535}, {7, 7, 65535}, {7, 3, 0}} {
		if _, err := PlayerContextReply115(ids[0], ids[1], ids[2]); err == nil {
			t.Fatalf("accepted identities %v", ids)
		}
	}
}
