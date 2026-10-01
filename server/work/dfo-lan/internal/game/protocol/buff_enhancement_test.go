package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestBuffEnhancementLiveRequests(t *testing.T) {
	for _, tc := range []struct {
		body string
		want BuffEnhancementRequest
	}{
		{"32010d0039000000", BuffEnhancementRequest{306, 13, 0, 57}},
		{"3201302effff0000", BuffEnhancementRequest{306, 48, 46, 65535}},
	} {
		body, _ := hex.DecodeString(tc.body)
		got, err := DecodeBuffEnhancement(body)
		if err != nil || got != tc.want {
			t.Fatalf("live request %s: %+v %v", tc.body, got, err)
		}
	}
	for _, body := range []string{"", "32010d0039", "32010d0039000001", "320130003900", "32010d02ffff", "32012f003900"} {
		p, _ := hex.DecodeString(body)
		if _, err := DecodeBuffEnhancement(p); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestBuffEnhancementNativeVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/native_buff_enhancement.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Vectors []struct {
			Name, Direction string
			PayloadHex      string `json:"payload_hex"`
			Request         BuffEnhancementRequest
			Skill           uint16
			Items           []BuffEnhancementItem
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Vectors) == 0 {
		t.Fatal("missing native vectors")
	}
	for _, v := range fixture.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			p, err := hex.DecodeString(v.PayloadHex)
			if err != nil {
				t.Fatal(err)
			}
			switch v.Direction {
			case "C2S":
				got, err := DecodeBuffEnhancement(p)
				if err != nil || got != v.Request {
					t.Fatalf("native request: got %+v, want %+v: %v", got, v.Request, err)
				}
			case "S2C":
				got, err := BuffEnhancementAllData(v.Skill, v.Items)
				if err != nil || !bytes.Equal(got, p) {
					t.Fatalf("native restore: got %x, want %x: %v", got, p, err)
				}
			default:
				t.Fatalf("unsupported vector direction %q", v.Direction)
			}
		})
	}
}
