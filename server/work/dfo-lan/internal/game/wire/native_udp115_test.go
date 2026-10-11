package wire

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeUDP115CurrentPEVectors(t *testing.T) {
	raw, e := os.ReadFile("testdata/native_udp115_vectors.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		ClientSHA string `json:"client_sha256"`
		Vectors   []struct {
			Slot     byte
			Key      uint32
			Sequence uint32
			Payload  string `json:"payload_hex"`
			Raw      string `json:"raw_hex"`
		}
	}
	if e = json.Unmarshal(raw, &f); e != nil {
		t.Fatal(e)
	}
	if f.ClientSHA != "515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb" || len(f.Vectors) != 3 {
		t.Fatal("native vector identity")
	}
	for _, v := range f.Vectors {
		payload, _ := hex.DecodeString(v.Payload)
		expected, _ := hex.DecodeString(v.Raw)
		got, e := EncodeNativeUDP115(v.Slot, v.Key, v.Sequence, payload)
		if e != nil || !bytes.Equal(got, expected) {
			t.Fatalf("native encoder mismatch: %x != %x: %v", got, expected, e)
		}
		slot, sequence, p, e := DecodeNativeUDP115(expected, v.Key)
		if e != nil || slot != v.Slot || sequence != v.Sequence || !bytes.Equal(p, payload) {
			t.Fatal("native sender decode", e)
		}
		for i := 0; i < len(expected); i++ {
			bad := append([]byte(nil), expected...)
			bad[i] ^= 0x80
			if _, _, _, e = DecodeNativeUDP115(bad, v.Key); e == nil {
				t.Fatalf("accepted corrupt byte%d", i)
			}
		}
		for i := 0; i < len(expected); i++ {
			if _, _, _, e = DecodeNativeUDP115(expected[:i], v.Key); e == nil {
				t.Fatal("accepted truncated datagram")
			}
		}
	}
}
