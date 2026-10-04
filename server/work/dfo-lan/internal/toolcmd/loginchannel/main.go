// loginchannel changes only the channel type in a synthetic local LOGIN fixture.
package loginchannel

import (
	"bytes"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
)

func project(raw []byte, channel byte) ([]byte, byte, error) {
	if err := wire.ValidateServer(raw); err != nil {
		return nil, 0, err
	}
	if raw[0] != 1 || binary.LittleEndian.Uint16(raw[1:3]) != 1 || raw[11] != wire.Checksum(raw[16:]) {
		return nil, 0, fmt.Errorf("not a valid LOGIN frame")
	}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	plain, err := wire.DecryptPayload(keys, 1, raw[16:])
	if err != nil {
		return nil, 0, err
	}
	if len(plain) < 5 || plain[0] != 1 {
		return nil, 0, fmt.Errorf("not LOGIN success")
	}
	// Native 1452546f5 reads payload[3]; 1452549b7 sets channel manager+11b0.
	old := plain[3]
	plain[3] = channel
	encrypted, err := wire.EncryptPayload(keys, 1, plain)
	if err != nil {
		return nil, 0, err
	}
	out := bytes.Clone(raw)
	copy(out[16:], encrypted)
	out[11] = wire.Checksum(encrypted)
	return out, old, nil
}

func Run() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: loginchannel INPUT OUTPUT CHANNEL_TYPE")
		os.Exit(2)
	}
	v, err := strconv.ParseUint(os.Args[3], 10, 8)
	if err != nil {
		panic(err)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	out, old, err := project(raw, byte(v))
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(os.Args[2], out, 0600); err != nil {
		panic(err)
	}
	fmt.Printf("LOGIN channel_type %d -> %d; bytes=%d\n", old, v, len(out))
}
