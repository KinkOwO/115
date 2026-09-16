package wire

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"golang.org/x/crypto/cast5"
	"os"
	"testing"
)

func TestNativeCipherVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/native_cipher_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct{ Algorithm, Key, Plain, Cipher string }
	if err = json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}
	aesRaw, err := os.ReadFile("testdata/native_aes_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var aesVectors []struct{ Algorithm, Key, Plain, Cipher string }
	if err = json.Unmarshal(aesRaw, &aesVectors); err != nil {
		t.Fatal(err)
	}
	vectors = append(vectors, aesVectors...)
	multiRaw, err := os.ReadFile("testdata/native_multi2_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var multiVectors []struct{ Algorithm, Key, Plain, Cipher string }
	if err = json.Unmarshal(multiRaw, &multiVectors); err != nil {
		t.Fatal(err)
	}
	vectors = append(vectors, multiVectors...)
	xorRaw, err := os.ReadFile("testdata/native_xor32_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var xorVectors []struct{ Algorithm, Key, Plain, Cipher string }
	if err = json.Unmarshal(xorRaw, &xorVectors); err != nil {
		t.Fatal(err)
	}
	vectors = append(vectors, xorVectors...)
	for _, v := range vectors {
		t.Run(v.Algorithm+"/"+v.Key, func(t *testing.T) {
			key, _ := hex.DecodeString(v.Key)
			plain, _ := hex.DecodeString(v.Plain)
			expected, _ := hex.DecodeString(v.Cipher)
			var block cipher.Block
			var err error
			switch v.Algorithm {
			case "noekeon":
				block, err = newNoekeon(key)
			case "skipjack":
				block, err = newSkipjack(key)
			case "cast5":
				block, err = cast5.NewCipher(key)
			case "dfo_rc6":
				block, err = newDFORC6(key)
			case "aes128":
				block, err = aes.NewCipher(key)
			case "xtea_le":
				block, err = newXTEALE(key)
			case "dfo_multi2":
				block, err = newDFOMulti2(key)
			case "dfo_xor32":
				block, err = newDFOXOR32(key)
			default:
				t.Fatal("unknown native vector")
			}
			if err != nil {
				t.Fatal(err)
			}
			out := make([]byte, len(plain))
			for off := 0; off < len(plain); off += block.BlockSize() {
				block.Encrypt(out[off:], plain[off:])
			}
			if !bytes.Equal(out, expected) {
				t.Fatalf("Go=%x native=%x", out, expected)
			}
			for off := 0; off < len(plain); off += block.BlockSize() {
				block.Decrypt(out[off:], expected[off:])
			}
			if !bytes.Equal(out, plain) {
				t.Fatalf("decrypted %x expected %x", out, plain)
			}
		})
	}
}

func TestCapturedCharacterListRequest(t *testing.T) {
	// Original client capture from roles_persist_01. Its native plaintext
	// breakpoint reported FFFF02 before the block padding.
	raw, _ := hex.DecodeString("01080015000000845d828b0600d2993ece48587f67")
	key := make([]byte, SessionKeyBytes)
	for i := range key {
		key[i] = byte(i%127 + 1)
	}
	plain, e := DecryptPayload(key, 8, raw[13:])
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(plain[:3], []byte{255, 255, 2}) {
		t.Fatalf("wrong native request %x", plain)
	}
	if Checksum(append(append([]byte{}, raw[11:13]...), plain...)) != raw[7] {
		t.Fatal("native request CRC mismatch")
	}
}

func TestCapturedMercenaryRequest(t *testing.T) {
	// roles_persist_select_05: wrapper direction verified using both native
	// routines. Only this direction yields both a valid shape and native CRC.
	raw, _ := hex.DecodeString("01b10115000000319cadde0900d80f6d97c25b9550")
	keys := make([]byte, SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	plain, e := DecryptPayload(keys, 433, raw[13:])
	if e != nil || !bytes.Equal(plain, []byte{6, 1, 2, 3, 4, 5, 6, 0}) {
		t.Fatalf("native mercenary payload %x: %v", plain, e)
	}
	if Checksum(append(append([]byte{}, raw[11:13]...), plain...)) != raw[7] {
		t.Fatal("native mercenary checksum mismatch")
	}
}

func TestAreaUsersFourBytePadding(t *testing.T) {
	keys := make([]byte, SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	plain, _ := hex.DecodeString("2600000000000000010003003102ea00000101")
	actual, e := EncryptPayload(keys, 24, plain)
	if e != nil {
		t.Fatal(e)
	}
	// Slot 10 key starts at byte 254: 01 02 03 04 05 06 07 08.
	// Nineteen bytes must pad to 20, not to an eight-byte key boundary.
	if hex.EncodeToString(actual) != "2306070805060708040604083404ed0805070608" {
		t.Fatalf("wrong native XOR/padding %x", actual)
	}
	if _, e = DecryptPayload(keys, 24, actual[:19]); e == nil {
		t.Fatal("unaligned ciphertext accepted")
	}
}
