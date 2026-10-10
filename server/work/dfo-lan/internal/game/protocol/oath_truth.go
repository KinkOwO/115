package protocol

import (
	"encoding/binary"
	"fmt"
)

// PE 1421a15e0/1421a1630 write exactly 5/2 bytes after the common
// transport header, which is outside the decrypted body. Existing codecs
// pad CMD2420 to 16 bytes and CMD2421 to 8 bytes; there is no 13-byte
// item-window prefix in these two requests.
type OathTruthUpgradeRequest struct {
	Level byte
	// Payment is the one-based source group, not the window's zero-based index.
	Payment uint32
}

func DecodeOathTruthUpgrade(body []byte) (OathTruthUpgradeRequest, error) {
	if len(body) != 16 {
		return OathTruthUpgradeRequest{}, fmt.Errorf("truth upgrade body must be 16 bytes")
	}
	return OathTruthUpgradeRequest{body[0], binary.LittleEndian.Uint32(body[1:5])}, nil
}

type OathTruthSelectRequest struct{ Slot, Option byte }

func DecodeOathTruthSelect(body []byte) (OathTruthSelectRequest, error) {
	if len(body) != 8 {
		return OathTruthSelectRequest{}, fmt.Errorf("truth selection body must be 8 bytes")
	}
	return OathTruthSelectRequest{body[0], body[1]}, nil
}

// The kind-1 dispatcher consumes status before the native 9/6-byte reader.
// The callback consumes reader+4 as a one-based stage for its animation.
func OathTruthUpgradeReply(level byte, payment uint32) []byte {
	b := make([]byte, 10)
	b[0] = 1
	b[5] = level
	binary.LittleEndian.PutUint32(b[6:], payment)
	return b
}
func OathTruthSelectReply(slot, option byte) []byte {
	b := make([]byte, 7)
	b[0] = 1
	b[5] = slot
	b[6] = option
	return b
}
func OathTruthFailure() []byte { return []byte{0, 0, 0} }

// NOTI2839 reader -> 14057d140 -> 145d392a0 copies these 11 bytes
// to actor+189fc: level and options for source slots 0..9. Slot 0 is
// the empty source slot; the first active stage is options[1].
func OathSystemInfoWithTruth(characterLevel, oathOption int, truthLevel byte, options [10]byte) ([]byte, error) {
	if truthLevel > 9 {
		return nil, fmt.Errorf("truth level out of native range")
	}
	b, err := OathSystemInfo(characterLevel, oathOption)
	if err != nil {
		return nil, err
	}
	if characterLevel >= 115 {
		b[4] = truthLevel
		copy(b[5:15], options[:])
	}
	return b, nil
}
