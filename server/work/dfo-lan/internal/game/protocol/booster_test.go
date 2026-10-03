package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestDecodeBoosterUseRequestCaptured(t *testing.T) {
	// 4200 01000000 0000 d8c20506 00 00 00 00
	p := []byte{
		0x42, 0x00, // slot 66
		0x01, 0x00, 0x00, 0x00, // amount 1
		0x00, 0x00, // category 0
		0xd8, 0xc2, 0x05, 0x06, // template 101040856
		0x00,       // avatar count 0
		0x00,       // trailing zero
		0x00, 0x00, // padding
	}
	req, err := DecodeBoosterUseRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	if req.Slot != 66 {
		t.Fatalf("expected slot 66, got %d", req.Slot)
	}
	if req.Amount != 1 {
		t.Fatalf("expected amount 1, got %d", req.Amount)
	}
	if req.Category != 0 {
		t.Fatalf("expected category 0, got %d", req.Category)
	}
	if len(req.Selections) != 1 || req.Selections[0] != 101040856 {
		t.Fatalf("expected 1 selection (101040856), got %+v", req.Selections)
	}
	if len(req.AvatarOptions) != 0 {
		t.Fatalf("expected 0 avatar options, got %+v", req.AvatarOptions)
	}
}

func TestDecodeBoosterUseRequestAvatarOptionSelection(t *testing.T) {
	// Client sub_14573DC60 format:
	// slot: 10, amount: 1, category: 5 (Gunner F)
	// selection: 505580001 (Copper Brown Skin)
	// avatar_count: 1
	// avatar entry: 505580001, option: 0 (Physical Def 850)
	// trailing zero: 0
	// padding to 32 bytes
	p := make([]byte, 32)
	binary.LittleEndian.PutUint16(p[0:2], 10)
	binary.LittleEndian.PutUint32(p[2:6], 1)
	binary.LittleEndian.PutUint16(p[6:8], 5)
	binary.LittleEndian.PutUint32(p[8:12], 505580001)
	p[12] = 1 // avatar count
	binary.LittleEndian.PutUint32(p[13:17], 505580001)
	p[17] = 0 // option
	p[18] = 0 // trailing zero

	req, err := DecodeBoosterUseRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	if req.Slot != 10 {
		t.Fatalf("expected slot 10, got %d", req.Slot)
	}
	if req.Amount != 1 {
		t.Fatalf("expected amount 1, got %d", req.Amount)
	}
	if req.Category != 5 {
		t.Fatalf("expected category 5, got %d", req.Category)
	}
	if len(req.Selections) != 1 || req.Selections[0] != 505580001 {
		t.Fatalf("expected selections [505580001], got %+v", req.Selections)
	}
	if len(req.AvatarOptions) != 1 {
		t.Fatalf("expected 1 avatar option, got %+v", req.AvatarOptions)
	}
	if req.AvatarOptions[0].Template != 505580001 || req.AvatarOptions[0].Option != 0 {
		t.Fatalf("unexpected avatar option: %+v", req.AvatarOptions[0])
	}
}

func TestDecodeBoosterUseRequestSimple(t *testing.T) {
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p[0:2], 65)
	binary.LittleEndian.PutUint32(p[2:6], 2)
	binary.LittleEndian.PutUint16(p[6:8], 0)

	req, err := DecodeBoosterUseRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	if req.Slot != 65 || req.Amount != 2 || req.Category != 0 {
		t.Fatalf("unexpected request: %+v", req)
	}
	if len(req.Selections) != 0 || len(req.AvatarOptions) != 0 {
		t.Fatalf("expected no selections, got selections=%+v, options=%+v", req.Selections, req.AvatarOptions)
	}
}

// The client sent eight templates and no ability options for both professions.
// The archer fourth template starts with 04, which previously looked like an
// option count and truncated the grant list to the first three templates.
func TestDecodeBoosterUseRequestNativeAvatarPackages(t *testing.T) {
	data, err := os.ReadFile("testdata/native_booster_avatar_package_20261001.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name       string   `json:"name"`
		Slot       uint16   `json:"slot"`
		Category   uint16   `json:"category"`
		PlainHex   string   `json:"plain_hex"`
		Selections []uint32 `json:"selections"`
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			raw, err := hex.DecodeString(v.PlainHex)
			if err != nil {
				t.Fatal(err)
			}
			req, err := DecodeBoosterUseRequest(raw)
			if err != nil {
				t.Fatal(err)
			}
			if req.Slot != v.Slot || req.Amount != 1 || req.Category != v.Category || !reflect.DeepEqual(req.Selections, v.Selections) || len(req.AvatarOptions) != 0 {
				t.Fatalf("native request decoded as %+v, want slot=%d category=%d selections=%v and no options", req, v.Slot, v.Category, v.Selections)
			}
		})
	}
}
