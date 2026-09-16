package wire

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"golang.org/x/crypto/cast5"
	"golang.org/x/crypto/twofish"
	"golang.org/x/crypto/xtea"
)

var keyLengths = [14]int{16, 16, 60, 32, 16, 10, 16, 56, 16, 16, 8, 16, 16, 40}

const SessionKeyBytes = 334

func packetCipher(keys []byte, id uint16) (cipher.Block, int, error) {
	if len(keys) != SessionKeyBytes {
		return nil, 0, fmt.Errorf("session key bytes: got %d, need %d", len(keys), SessionKeyBytes)
	}
	index := int(id) % 14
	start := 0
	for _, n := range keyLengths[:index] {
		start += n
	}
	key := keys[start : start+keyLengths[index]]
	var block cipher.Block
	var err error
	alignment := 0
	switch index {
	case 0:
		block, err = xtea.NewCipher(key)
		alignment = 16
	case 1:
		block, err = cast5.NewCipher(key)
		alignment = 8
	case 2:
		block, err = newDFORC6(key)
		alignment = 16
	case 3:
		block, err = twofish.NewCipher(key)
		alignment = 16
	case 4:
		block, err = aes.NewCipher(key)
		alignment = 16
	case 5:
		block, err = newSkipjack(key)
		alignment = 8
	case 6:
		block, err = newKasumi(key)
		alignment = 8
	case 7:
		block, err = newDFOBlowfish(key)
		alignment = 8
	case 8:
		block, err = newXTEALE(key)
		alignment = 8
	case 9:
		block, err = newAreaCipher(key)
		alignment = 16
	case 10:
		block, err = newDFOXOR32(key)
		alignment = 4
	case 11:
		block, err = newCombatCipher(key)
		alignment = 8
	case 12:
		block, err = newNoekeon(key)
		alignment = 16
	case 13:
		block, err = newDFOMulti2(key)
		alignment = 8
	default:
		return nil, 0, fmt.Errorf("cipher slot %d is not implemented", index)
	}
	return block, alignment, err
}
func EncryptPayload(keys []byte, id uint16, payload []byte) ([]byte, error) {
	b, align, err := packetCipher(keys, id)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, (len(payload)+align-1)/align*align)
	copy(plain, payload)
	out := make([]byte, len(plain))
	for i := 0; i < len(plain); i += b.BlockSize() {
		b.Encrypt(out[i:], plain[i:])
	}
	return out, nil
}
func DecryptPayload(keys []byte, id uint16, payload []byte) ([]byte, error) {
	b, align, err := packetCipher(keys, id)
	if err != nil {
		return nil, err
	}
	if len(payload)%align != 0 {
		return nil, fmt.Errorf("invalid encrypted payload alignment")
	}
	out := make([]byte, len(payload))
	for i := 0; i < len(payload); i += b.BlockSize() {
		b.Decrypt(out[i:], payload[i:])
	}
	return out, nil
}
