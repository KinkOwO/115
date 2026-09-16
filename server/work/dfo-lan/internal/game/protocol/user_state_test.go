package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestUserStateNativeRoomGuard(t *testing.T) {
	b, err := os.ReadFile("testdata/native_user_state.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Payload  string `json:"payload_hex"`
		Consumed int    `json:"consumed"`
		State    byte   `json:"state"`
		Stored   byte   `json:"stored_state"`
		Next     string `json:"room_guard_next"`
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatal("missing native state coverage")
	}
	for _, row := range rows {
		want, err := hex.DecodeString(row.Payload)
		if err != nil {
			t.Fatal(err)
		}
		got, err := UserState(503, row.State)
		if err != nil || !bytes.Equal(got, want) || len(got) != row.Consumed || row.Stored != row.State {
			t.Fatalf("native NOTI3 mismatch: %+v %v", row, err)
		}
		expected := "0x144d35912"
		if row.State == UserStateDungeon {
			expected = "0x144d34bf3"
		}
		if row.Next != expected {
			t.Fatal("native room guard did not switch with actor state")
		}
	}
	for _, actor := range []uint16{0, 65535} {
		if _, err = UserState(actor, 0); err == nil {
			t.Fatal("accepted unowned actor")
		}
	}
	if _, err = UserState(503, 2); err == nil {
		t.Fatal("accepted unsupported state")
	}
}
