package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestOathTruthNativeVectors(t *testing.T) {
	var fixture struct {
		Requests []struct {
			Opcode uint16 `json:"opcode"`
			Hex    string `json:"body_hex"`
		} `json:"request_writer_vectors"`
		States []struct {
			Level, Option byte
			Hex           string `json:"noti2839_hex"`
		} `json:"state_and_attribute_vectors"`
	}
	raw, err := os.ReadFile("testdata/native_oath_truth.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, v := range fixture.Requests {
		raw, _ := hex.DecodeString(v.Hex)
		if v.Opcode == 2420 {
			body := append(raw, make([]byte, 16-len(raw))...)
			r, err := DecodeOathTruthUpgrade(body)
			if err != nil || r.Level != raw[0] || byte(r.Payment) != raw[1] {
				t.Fatalf("native upgrade %x %v", body, err)
			}
			ack := OathTruthUpgradeReply(r.Level, r.Payment)
			if len(ack) != 10 || ack[5] != r.Level {
				t.Fatal("native 9-byte reply geometry")
			}
		} else {
			body := append(raw, make([]byte, 8-len(raw))...)
			r, err := DecodeOathTruthSelect(body)
			if err != nil || r.Slot != raw[0] || r.Option != raw[1] {
				t.Fatalf("native selection %x %v", body, err)
			}
			ack := OathTruthSelectReply(r.Slot, r.Option)
			if len(ack) != 7 || ack[5] != r.Slot || ack[6] != r.Option {
				t.Fatal("native 6-byte reply geometry")
			}
		}
	}
	for _, v := range fixture.States {
		var options [10]byte
		for n := 1; n <= int(v.Level); n++ {
			options[n] = v.Option
		}
		body, err := OathSystemInfoWithTruth(115, 2, v.Level, options)
		want, _ := hex.DecodeString(v.Hex)
		if err != nil || !bytes.Equal(body, want) {
			t.Fatalf("native state level %d option %d: %x != %x %v", v.Level, v.Option, body, want, err)
		}
	}
	if _, err := DecodeOathTruthUpgrade(make([]byte, 32)); err == nil {
		t.Fatal("item-window prefix must not be accepted")
	}
	if _, err := DecodeOathTruthSelect(make([]byte, 16)); err == nil {
		t.Fatal("wrong cipher alignment accepted")
	}
}
