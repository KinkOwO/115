package wire

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeUDPDatagram115NativeVectors(t *testing.T) {
	data, e := os.ReadFile("testdata/native_udp_datagram115_vectors.json")
	if e != nil {
		t.Fatal(e)
	}
	var proof struct {
		SHA     string `json:"client_sha256"`
		Vectors []struct {
			Type, Slot    byte
			Key, Sequence uint32
			Payload       string `json:"payload_hex"`
			Raw           string `json:"raw_hex"`
		}
	}
	if e = json.Unmarshal(data, &proof); e != nil || proof.SHA != "515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb" || len(proof.Vectors) != 7 {
		t.Fatal("unbound native proof", e)
	}
	var combined []byte
	for _, v := range proof.Vectors {
		raw, _ := hex.DecodeString(v.Raw)
		want, _ := hex.DecodeString(v.Payload)
		got, e := DecodeNativeUDPDatagram115(raw, v.Key)
		if e != nil || len(got) != 1 || got[0].Type != v.Type || got[0].Slot != v.Slot || !bytes.Equal(got[0].Payload, want) {
			t.Fatal("native decode mismatch", e)
		}
		var encoded []byte
		switch v.Type {
		case 0:
			encoded, e = EncodeNativeUDPAck115(v.Slot, v.Key, v.Sequence)
		case 1:
			encoded, e = EncodeNativeUDPReliable115(v.Slot, v.Key, v.Sequence, want)
		case 2:
			encoded, e = EncodeNativeUDP115(v.Slot, v.Key, v.Sequence, want)
		}
		if e != nil || !bytes.Equal(encoded, raw) {
			t.Fatalf("native writer mismatch %d %x %v", v.Type, encoded, e)
		}
		for n := 0; n < len(raw); n++ {
			if _, e = DecodeNativeUDPDatagram115(raw[:n], v.Key); e == nil {
				t.Fatal("accepted truncated native frame", v.Type, n)
			}
		}
		bad := bytes.Clone(raw)
		bad[len(bad)-1] ^= 1
		if _, e = DecodeNativeUDPDatagram115(bad, v.Key); e == nil {
			t.Fatal("accepted corrupt native frame")
		}
		if v.Key == 0x11223344 {
			combined = append(combined, raw...)
		}
	}
	frames, e := DecodeNativeUDPDatagram115(combined, 0x11223344)
	if e != nil || len(frames) != 5 {
		t.Fatal("native aggregate rejected", e)
	}
	combined[len(combined)-1] ^= 1
	if frames, e = DecodeNativeUDPDatagram115(combined, 0x11223344); e == nil || frames != nil {
		t.Fatal("partial aggregate effects")
	}
	if _, e = DecodeNativeUDPDatagram115(make([]byte, 4097), 1); e == nil {
		t.Fatal("oversized UDP accepted")
	}
}
func TestNativeUDPReliable115BundleBounds(t *testing.T) {
	for _, p := range [][]byte{nil, {0}, {0, 0}, {2, 0, 1, 1}, {3, 0, 1, 1}, {4, 0, 1, 1, 0}} {
		if _, e := SplitNativeUDPReliable115(p); e == nil {
			t.Fatalf("invalid bundle accepted %x", p)
		}
	}
	p := []byte{3, 0, 1, 1, 0, 5, 0, 2, 195, 0, 0, 0}
	out, e := SplitNativeUDPReliable115(p)
	if e != nil || len(out) != 2 || !bytes.Equal(out[1], p[7:]) {
		t.Fatal("bundle split", e)
	}
	p[2] = 9
	if out[0][0] != 1 {
		t.Fatal("retained receive buffer alias")
	}
}
