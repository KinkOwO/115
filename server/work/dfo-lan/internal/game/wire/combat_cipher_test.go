package wire

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeCombatCipher(t *testing.T) {
	raw, err := os.ReadFile("testdata/native_combat_cipher.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ Key, Plain, Cipher string }
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		t.Run(row.Key, func(t *testing.T) {
			key, _ := hex.DecodeString(row.Key)
			plain, _ := hex.DecodeString(row.Plain)
			want, _ := hex.DecodeString(row.Cipher)
			keys := make([]byte, SessionKeyBytes)
			start := 0
			for _, n := range keyLengths[:11] {
				start += n
			}
			copy(keys[start:], key)
			got, err := EncryptPayload(keys, 39, plain)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("Go=%x native=%x error=%v", got, want, err)
			}
			recovered, err := DecryptPayload(keys, 39, want)
			if err != nil || !bytes.Equal(recovered, plain) {
				t.Fatalf("native ciphertext decode=%x want=%x error=%v", recovered, plain, err)
			}
			block, _ := newCombatCipher(key)
			inPlace := append([]byte(nil), plain...)
			for i := 0; i < len(inPlace); i += 8 {
				block.Encrypt(inPlace[i:], inPlace[i:])
			}
			if !bytes.Equal(inPlace, want) {
				t.Fatal("in-place encryption differs")
			}
		})
	}
	if _, err := newCombatCipher(make([]byte, 15)); err == nil {
		t.Fatal("invalid key accepted")
	}
}

func TestCapturedCombatChecksumAndDirection(t *testing.T) {
	data, e := os.ReadFile("testdata/captured_combat.json")
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Frame string `json:"cipher_frame"`
		Plain string `json:"plain_hex"`
	}
	if e = json.Unmarshal(data, &r); e != nil {
		t.Fatal(e)
	}
	raw, e := hex.DecodeString(r.Frame)
	if e != nil {
		t.Fatal(e)
	}
	want, e := hex.DecodeString(r.Plain)
	if e != nil {
		t.Fatal(e)
	}
	keys := make([]byte, SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	got, e := DecryptPayload(keys, 39, raw[13:])
	if e != nil || !bytes.Equal(got, want) {
		t.Fatalf("native live combat decode: %x %v", got, e)
	}
	if Checksum(append(append([]byte{}, raw[11:13]...), got...)) != raw[7] {
		t.Fatal("combat native checksum mismatch")
	}
}
