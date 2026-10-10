package main

import (
	"bytes"
	"dfolan/internal/game/wire"
	"dfolan/internal/social"
	"encoding/binary"
	"fmt"
)

// 1452543D0 reads the common API string after the first login string and two
// u32s, then stores it at 14DC67820. Change only an empty URL. All later flags
// and opaque bytes are copied unchanged; existing nonempty services are kept.
func socialLoginResponse(keys, raw []byte) ([]byte, error) {
	if err := wire.ValidateServer(raw); err != nil {
		return nil, err
	}
	if raw[0] != 1 || binary.LittleEndian.Uint16(raw[1:]) != 1 || raw[11] != wire.Checksum(raw[16:]) {
		return nil, fmt.Errorf("expected community login frame")
	}
	p, err := wire.DecryptPayload(keys, 1, raw[16:])
	if err != nil {
		return nil, err
	}
	if len(p) < 13 || p[0] != 1 {
		return nil, fmt.Errorf("short community login payload")
	}
	offset := 9
	skipString := func() error {
		if len(p)-offset < 4 {
			return fmt.Errorf("short login string length")
		}
		n := uint64(binary.LittleEndian.Uint32(p[offset:]))
		offset += 4
		if n >= 128 || n > uint64(len(p)-offset) {
			return fmt.Errorf("invalid native login string length")
		}
		offset += int(n)
		return nil
	}
	if err := skipString(); err != nil {
		return nil, err
	}
	if len(p)-offset < 12 {
		return nil, fmt.Errorf("short common API login fields")
	}
	offset += 8
	urlOffset := offset
	if err := skipString(); err != nil {
		return nil, err
	}
	if offset != urlOffset+4 {
		return bytes.Clone(raw), nil
	}
	body := append([]byte{}, p[:urlOffset]...)
	body = binary.LittleEndian.AppendUint32(body, uint32(len(social.LocalURL)))
	body = append(body, []byte(social.LocalURL)...)
	body = append(body, p[offset:]...)
	cipher, err := wire.EncryptPayload(keys, 1, body)
	if err != nil {
		return nil, err
	}
	frame, err := wire.ServerFrame(1, 1, cipher)
	if err != nil {
		return nil, err
	}
	// Keep the original envelope's reserved fields too.
	copy(frame[7:11], raw[7:11])
	copy(frame[12:16], raw[12:16])
	return frame, nil
}
