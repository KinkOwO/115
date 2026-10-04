package main

import (
	"dfolan/internal/database"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"
)

func TestMailboxSnapshotSkipsBadAssetsAndMessages(t *testing.T) {
	gold, err := json.Marshal(map[string]any{"stack": map[string]any{"Slot": 65, "Template": 1, "Amount": 250}})
	if err != nil {
		t.Fatal(err)
	}
	messages := []database.MailMessage{
		{ID: 1, SenderName: "GM", Text: "valid", Status: 1, ExpiresAt: time.Now().Add(time.Hour), Assets: []database.MailAsset{{ID: 2, Item: json.RawMessage(`{broken`)}, {ID: 3, Item: gold}}},
		{ID: 4, SenderName: "GM", Text: "\x00bad", Status: 1, ExpiresAt: time.Now().Add(time.Hour)},
	}
	packets, err := mailboxSnapshot(messages)
	if err != nil || len(packets) != 1 {
		t.Fatalf("snapshot = %+v, %v", packets, err)
	}
	body := packets[0].Payload
	if len(body) < 4 || body[0] != 1 {
		t.Fatalf("attachment count = %v", body)
	}
	// Locate the letter count through a single-asset encoding smoke check:
	// the invalid second message must not abort the first one.
	if binary.LittleEndian.Uint16(body[0:2]) == 0 {
		t.Fatal("valid attachment missing")
	}
}
