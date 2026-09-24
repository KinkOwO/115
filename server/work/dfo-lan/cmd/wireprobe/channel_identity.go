package main

import (
	"bytes"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"fmt"
)

// Opt-in for clients with the documented NOTI2435 handler. Both identity
// and actor contexts are configured together, before any actor packet is built.
func channelIdentity(c channelrefresh.Config, id uint32) ([2]byte, []byte, error) {
	if c.ServerID == 0 || c.ServerID > 255 || id == 0 || id > 255 {
		return [2]byte{}, nil, fmt.Errorf("channel identity must fit nonzero u8 server/channel fields")
	}
	for _, ch := range c.Channels {
		if ch.ID != id {
			continue
		}
		if ch.Type > 255 {
			return [2]byte{}, nil, fmt.Errorf("channel type does not fit login u8")
		}
		p := make([]byte, 16)
		binary.LittleEndian.PutUint32(p, c.ServerID)
		binary.LittleEndian.PutUint32(p[4:], id)
		binary.LittleEndian.PutUint32(p[12:], ch.Type)
		return [2]byte{byte(c.ServerID), byte(id)}, p, nil
	}
	return [2]byte{}, nil, fmt.Errorf("channel absent from directory")
}

func channelLoginResponse(keys, raw []byte, channelType byte) ([]byte, error) {
	if err := wire.ValidateServer(raw); err != nil {
		return nil, err
	}
	if raw[0] != 1 || binary.LittleEndian.Uint16(raw[1:]) != 1 || raw[11] != wire.Checksum(raw[16:]) {
		return nil, fmt.Errorf("expected login response frame")
	}
	p, err := wire.DecryptPayload(keys, 1, raw[wire.ServerHeaderSize:])
	if err != nil {
		return nil, err
	}
	if len(p) < 5 || p[0] != 1 {
		return nil, fmt.Errorf("expected successful login payload")
	}
	// Local verified layout (cmd/loginchannel): type is payload[3], not [2].
	p[3] = channelType
	encrypted, err := wire.EncryptPayload(keys, 1, p)
	if err != nil {
		return nil, err
	}
	out := bytes.Clone(raw)
	copy(out[16:], encrypted)
	out[11] = wire.Checksum(encrypted)
	return out, nil
}
