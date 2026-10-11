package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestOrdinaryParty115NativeVectors(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		p, err := os.ReadFile(fmt.Sprintf("testdata/native_ordinary_party_nested_%d_115.json", n))
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			PayloadHex string   `json:"payload_hex"`
			Actors     []uint16 `json:"actors"`
		}
		if err = json.Unmarshal(p, &v); err != nil {
			t.Fatal(err)
		}
		want, err := hex.DecodeString(v.PayloadHex)
		if err != nil {
			t.Fatal(err)
		}
		got, err := OrdinaryPartyRoster115(1, [2]byte{3, 86}, v.Actors)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%d native roster differs: %x != %x", n, got, want)
		}
	}
	for _, actors := range [][]uint16{nil, {0}, {65535}, {3, 3}, {1, 2, 3, 4, 5}} {
		if _, e := OrdinaryPartyRoster115(1, [2]byte{}, actors); e == nil {
			t.Fatal("invalid party accepted", actors)
		}
	}
	if _, e := OrdinaryPartyRoster115(65535, [2]byte{}, []uint16{3}); e == nil {
		t.Fatal("invalid party id accepted")
	}
	if output := os.Getenv("DFO_TOWN_ROBOT_PARTY_VECTOR_OUTPUT"); output != "" {
		p, e := OrdinaryPartyRoster115(7, [2]byte{3, 86}, []uint16{3, 65534})
		if e != nil {
			t.Fatal(e)
		}
		r, _ := json.Marshal(map[string]any{"payload_hex": hex.EncodeToString(p), "actors": []uint16{3, 65534}, "context": [2]byte{3, 86}})
		if e = os.WriteFile(output, r, 0600); e != nil {
			t.Fatal(e)
		}
	}
}

func TestOrdinaryParty115InviteLiveRequest(t *testing.T) {
	p, _ := hex.DecodeString("feff0093000000fa3e02000000000000")
	r, e := DecodeOrdinaryPartyInvite115(p)
	if e != nil || r.Actor != 65534 || r.Metrics != [3]uint32{147, 147194, 0} || r.Option != 0 {
		t.Fatalf("%+v %v", r, e)
	}
	for _, bad := range [][]byte{p[:15], append(append([]byte{}, p...), 0), {0, 0, 0}} {
		if _, e := DecodeOrdinaryPartyInvite115(bad); e == nil {
			t.Fatal("bad invitation accepted")
		}
	}
	p[2] = 15
	if _, e := DecodeOrdinaryPartyInvite115(p); e == nil {
		t.Fatal("context action captured as invitation")
	}
}
