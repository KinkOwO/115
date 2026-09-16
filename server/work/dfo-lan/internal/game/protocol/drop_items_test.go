package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestCurrentDropRecordsAgainstNativeReader(t *testing.T) {
	b, e := os.ReadFile("testdata/native_drop_items.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Payload  string `json:"payload_hex"`
		Consumed int
		Fields   struct {
			Scene, Template uint32
			Owner           uint16
		} `json:"native_item_fields"`
	}
	if e = json.Unmarshal(b, &cases); e != nil {
		t.Fatal(e)
	}
	if len(cases) != 2 {
		t.Fatal("missing ordinary/gold native rows")
	}
	for i, tc := range cases {
		d := SceneDrop{Object: 12345, Owner: 3, Sentinel: 65535}
		binary.LittleEndian.PutUint16(d.Item[:], 120)
		binary.LittleEndian.PutUint32(d.Item[2:], tc.Fields.Template)
		amount := uint32(34)
		if i == 1 {
			amount = 2
		}
		binary.LittleEndian.PutUint32(d.Item[6:], amount)
		p, e := OrdinarySceneDropRecord(d)
		if e != nil || len(p) != tc.Consumed || hex.EncodeToString(p) != tc.Payload {
			t.Fatal("native drop row mismatch", e)
		}
	}
}
