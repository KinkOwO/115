package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestAdventureIdentityNativeLayouts(t *testing.T) {
	// Synthetic UTF-8 layout; the actual captured request is in adventure_name_live_test.go.
	name := append([]byte{6, 0, 0, 0}, []byte("冒险")...)
	decoded, err := DecodeAdventureName(name)
	if err != nil || decoded != "冒险" {
		t.Fatal(decoded, err)
	}
	info, err := AdventureNameInfo(decoded, true)
	if err != nil || !bytes.Equal(info, append(append([]byte(nil), name...), 0, 1, 0)) {
		t.Fatal(info, err)
	}
	for _, bad := range [][]byte{{}, {17, 0, 0, 0}, {1, 0, 0, 0, 0x81}, {1, 0, 0, 0, 0}, {1, 0, 0, 0, ' '}} {
		if _, err := DecodeAdventureName(bad); err == nil {
			t.Fatal("invalid name accepted", bad)
		}
	}
	// Sender copies its opaque prefix and unspecified alignment bytes too.
	body := bytes.Repeat([]byte{0xee}, 20)
	body[13], body[14], body[15] = 0, 0xab, 0xcd
	binary.LittleEndian.PutUint32(body[16:], 0x12345678)
	symbol, err := DecodeAdventureSymbol(body)
	if err != nil || symbol != 0x12345678 {
		t.Fatal(symbol, err)
	}
	if !bytes.Equal(AdventureSymbolChanged(symbol), []byte{1, 0, 0x78, 0x56, 0x34, 0x12}) {
		t.Fatal("ACK layout")
	}
	if b := AdventureSymbolInfo(symbol); len(b) != 16 || binary.LittleEndian.Uint32(b) != symbol || !bytes.Equal(b[4:], make([]byte, 12)) {
		t.Fatal("restore layout")
	}
	housing := append([]byte(nil), body...)
	housing[13] = 1
	for _, bad := range [][]byte{body[:19], housing, append(append([]byte(nil), body...), 1), append(append([]byte(nil), body...), make([]byte, 17)...)} {
		if _, err := DecodeAdventureSymbol(bad); err == nil {
			t.Fatal("invalid symbol request accepted", bad)
		}
	}
}
