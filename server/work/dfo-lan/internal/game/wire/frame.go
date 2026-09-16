// Package wire contains the framing verified against DFO 2.38.2.34.
// Payload codecs and game rules deliberately live outside this package.
package wire

import (
	"encoding/binary"
	"fmt"
	"io"
)

const ClientHeaderSize = 13
const ServerHeaderSize = 16
const MaxPacketSize = 1 << 20

type Frame struct {
	Type byte
	ID   uint16
	Raw  []byte
}

func ReadClient(r io.Reader) (Frame, error) {
	header := make([]byte, ClientHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return Frame{}, err
	}
	size := binary.LittleEndian.Uint32(header[3:7])
	if size < ClientHeaderSize || size > MaxPacketSize {
		return Frame{}, fmt.Errorf("invalid client frame size %d", size)
	}
	if header[0] > 1 {
		return Frame{}, fmt.Errorf("invalid frame type %d", header[0])
	}
	raw := make([]byte, int(size))
	copy(raw, header)
	if _, err := io.ReadFull(r, raw[ClientHeaderSize:]); err != nil {
		return Frame{}, err
	}
	return Frame{Type: raw[0], ID: binary.LittleEndian.Uint16(raw[1:3]), Raw: raw}, nil
}

func ValidateServer(raw []byte) error {
	if len(raw) < ServerHeaderSize || len(raw) > MaxPacketSize {
		return fmt.Errorf("invalid server frame length %d", len(raw))
	}
	if raw[0] > 1 || int(binary.LittleEndian.Uint32(raw[3:7])) != len(raw) {
		return fmt.Errorf("invalid server header")
	}
	return nil
}
