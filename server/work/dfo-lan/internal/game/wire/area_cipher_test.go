package wire

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeAreaCipher(t *testing.T) {
	raw, err := os.ReadFile("testdata/native_area_cipher.json")
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
			for _, n := range keyLengths[:9] {
				start += n
			}
			copy(keys[start:], key)
			got, err := EncryptPayload(keys, 23, plain)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("Go=%x native=%x error=%v", got, want, err)
			}
			recovered, err := DecryptPayload(keys, 23, want)
			if err != nil || !bytes.Equal(recovered, plain) {
				t.Fatalf("native ciphertext decode=%x want=%x error=%v", recovered, plain, err)
			}
			block, _ := newAreaCipher(key)
			inPlace := append([]byte(nil), plain...)
			for i := 0; i < len(inPlace); i += 16 {
				block.Encrypt(inPlace[i:], inPlace[i:])
			}
			if !bytes.Equal(inPlace, want) {
				t.Fatal("in-place encryption differs")
			}
		})
	}
	if _, err := newAreaCipher(make([]byte, 15)); err == nil {
		t.Fatal("invalid key accepted")
	}
}
