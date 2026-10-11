package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestAdventureNameConfirmationUsesValidatedLiveText(t *testing.T) {
	// 2026-10-07 07:27:11+08 live CMD1436, before creation mistakenly
	// submitted the localized placeholder because the ACK omitted its name.
	request, _ := hex.DecodeString("09000000e8b0b7e69ca8e8afad000000")
	name, err := DecodeAdventureName(request)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := AdventureNameAccepted(name)
	want, _ := hex.DecodeString("0109000000e8b0b7e69ca8e8afad")
	if err != nil || !bytes.Equal(accepted, want) {
		t.Fatalf("confirmation reader must receive validated text: %x %v", accepted, err)
	}
	// Native confirmation copies the returned name into its creation request.
	created, err := DecodeAdventureName(accepted[1:])
	if err != nil || created != "谷木语" {
		t.Fatalf("confirmation changed validated name: %q %v", created, err)
	}
}
