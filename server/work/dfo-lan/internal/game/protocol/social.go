package protocol

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

// CMD1431, sender 14390DF0A..14390E06F. The strings use 146D76080;
// the final level/byte/byte/u32 fields use the existing native primitive writers.
// The local request captured at 2026-10-07T00:31:28.7370306Z independently
// confirms the order and widths. Context and Extra remain opaque.
type SocialAddRequest struct {
	Adventure, Memo, Context, Character string
	Group, Server, Profession, Growth   byte
	Level                               uint16
	Extra                               uint32
}

func DecodeSocialAddRequest(p []byte) (r SocialAddRequest, err error) {
	off := 0
	readString := func() (string, error) {
		if len(p)-off < 4 {
			return "", fmt.Errorf("social string length missing")
		}
		n := uint64(binary.LittleEndian.Uint32(p[off:]))
		off += 4
		if n > uint64(len(p)-off) || n > 1024 {
			return "", fmt.Errorf("social string length invalid")
		}
		b := p[off : off+int(n)]
		off += int(n)
		if !utf8.Valid(b) {
			return "", fmt.Errorf("social string is not UTF-8")
		}
		for _, c := range b {
			if c == 0 {
				return "", fmt.Errorf("social string contains NUL")
			}
		}
		return string(b), nil
	}
	if r.Adventure, err = readString(); err != nil {
		return
	}
	if r.Adventure == "" {
		return r, fmt.Errorf("empty target adventure")
	}
	if r.Memo, err = readString(); err != nil {
		return
	}
	if off >= len(p) {
		return r, fmt.Errorf("social group missing")
	}
	r.Group = p[off]
	off++
	if r.Context, err = readString(); err != nil {
		return
	}
	if off >= len(p) {
		return r, fmt.Errorf("social server missing")
	}
	r.Server = p[off]
	off++
	if r.Character, err = readString(); err != nil {
		return
	}
	if len(p)-off < 8 {
		return r, fmt.Errorf("social character fields missing")
	}
	r.Level = binary.LittleEndian.Uint16(p[off:])
	r.Profession, r.Growth = p[off+2], p[off+3]
	r.Extra = binary.LittleEndian.Uint32(p[off+4:])
	off += 8
	err = padding(p[off:], 16)
	return
}

// 145263A8F..145263B7B maps error 217 to inappropriate adventure name,
// 21 to absent adventure, 3 to retry. A self target is an invalid adventure
// target. No success packet or invitation is generated for that request.
func SocialAddRejected(code uint16) []byte { return []byte{0, byte(code), byte(code >> 8)} }
