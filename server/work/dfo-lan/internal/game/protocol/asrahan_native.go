package protocol

import (
	"encoding/binary"
	"fmt"
)

const (
	CmdAsrahanFinishLoading115  uint16 = 2187
	CmdAsrahanUseBeast115       uint16 = 2188
	CmdAsrahanSignal2189        uint16 = 2189
	CmdAsrahanReport2190        uint16 = 2190
	CmdAsrahanRestartRevival115 uint16 = 2195
	NotiAsrahanBeasts115        uint16 = 2516
	NotiAsrahanUseBeast115      uint16 = 2517
	NotiAsrahanThumbUp115       uint16 = 2518
	NotiAsrahanSunMoon115       uint16 = 2519
	NotiAsrahanMu115            uint16 = 2520
	NotiAsrahanPattern115       uint16 = 2521
	NotiAsrahanDebuffs115       uint16 = 2522
	NotiAsrahanOverflow115      uint16 = 2523
)

// The main-raid notifications use functor registration 14599d650 in
// 144cd49d0. Ordinary NOTI2187 is USE_ATTRIBUTE, not loading completion.
func asrahanNativeSize115(id uint16) int {
	switch id {
	case NotiAsrahanBeasts115:
		return 60 // 144cdeb80: twelve 5-byte rows
	case NotiAsrahanUseBeast115:
		return 5 // 144cdf6f0 -> 141ce2860
	case NotiAsrahanThumbUp115:
		return 96 // 144cdfc00: twelve opaque 8-byte rows
	case NotiAsrahanSunMoon115:
		return 19 // 144cdf510 -> 141ce23c0
	case NotiAsrahanMu115:
		return 16 // 144cdefc0 -> 141ce1e40
	case NotiAsrahanPattern115:
		return 20 // 144cdee00 -> 141ce1a60
	case NotiAsrahanDebuffs115:
		return 11 // 144cde240 -> 141ce11b0
	case NotiAsrahanOverflow115:
		return 12 // 144cdf280 -> 141ce21c0
	}
	return 0
}

// Projection accepts complete existing native bodies and owns a copy.
// Unknown phase/timer words are preserved; this is not an initial-raid builder.
type AsrahanNativeProjection115 struct {
	id   uint16
	body []byte
}

func ReadAsrahanNativeProjection115(id uint16, body []byte) (*AsrahanNativeProjection115, error) {
	size := asrahanNativeSize115(id)
	if size == 0 || len(body) != size {
		return nil, fmt.Errorf("unbound/truncated Asrahan native notification %d: %d bytes", id, len(body))
	}
	if id == NotiAsrahanDebuffs115 {
		if body[0] > 10 {
			return nil, fmt.Errorf("Asrahan native debuff count exceeds ten slots")
		}
		// 141ce11b0 indexes a five-entry array with EVERY tail byte, not
		// only count bytes. FF is absent; another index is an out-of-bounds
		// client write even when the count is zero.
		for _, index := range body[1:] {
			if index != 0xff && index > 4 {
				return nil, fmt.Errorf("Asrahan debuff index exceeds native array")
			}
		}
	}
	if id == NotiAsrahanBeasts115 {
		for i := 4; i < len(body); i += 5 {
			if body[i] > 5 {
				return nil, fmt.Errorf("invalid native Asrahan beast index")
			}
		}
	}
	return &AsrahanNativeProjection115{id, append([]byte(nil), body...)}, nil
}
func (p *AsrahanNativeProjection115) Bytes() []byte { return append([]byte(nil), p.body...) }
func (p *AsrahanNativeProjection115) Words() ([]uint32, error) {
	if p.id != NotiAsrahanMu115 && p.id != NotiAsrahanPattern115 && p.id != NotiAsrahanOverflow115 {
		return nil, fmt.Errorf("Asrahan projection is not a native word vector")
	}
	var out []uint32
	for i := 0; i < len(p.body); i += 4 {
		out = append(out, binary.LittleEndian.Uint32(p.body[i:]))
	}
	return out, nil
}
func (p *AsrahanNativeProjection115) SetWord(slot int, value uint32) error {
	words, err := p.Words()
	if err != nil {
		return err
	}
	if slot < 0 || slot >= len(words) {
		return fmt.Errorf("Asrahan word slot outside native reader")
	}
	binary.LittleEndian.PutUint32(p.body[slot*4:], value)
	return nil
}

type AsrahanBeastMember115 struct {
	Actor uint32
	Beast byte
}

func AsrahanBeastInfo115(members [12]AsrahanBeastMember115) ([]byte, error) {
	body := make([]byte, 60)
	for i, member := range members {
		binary.LittleEndian.PutUint32(body[i*5:], member.Actor)
		body[i*5+4] = member.Beast
	}
	p, err := ReadAsrahanNativeProjection115(NotiAsrahanBeasts115, body)
	if err != nil {
		return nil, err
	}
	return p.Bytes(), nil
}

// A debuff notification has a count and all ten source slots. The unused
// slots must be FF rather than zero, which otherwise adds beast-0 stacks.
func AsrahanDebuffInfo115(indices []byte) ([]byte, error) {
	if len(indices) > 10 {
		return nil, fmt.Errorf("too many native Asrahan debuffs")
	}
	body := make([]byte, 11)
	body[0] = byte(len(indices))
	for i := 1; i < len(body); i++ {
		body[i] = 0xff
	}
	copy(body[1:], indices)
	p, err := ReadAsrahanNativeProjection115(NotiAsrahanDebuffs115, body)
	if err != nil {
		return nil, err
	}
	return p.Bytes(), nil
}

// C2188 writer141cf1850 appends a 13-byte envelope and a single byte.
// The envelope remains opaque, as in the current shared legion writers.
func DecodeAsrahanUseBeast115(body []byte) (byte, error) {
	if err := raidBodySize(body, 14); err != nil {
		return 0, err
	}
	return body[13], nil
}

// C2190 at14081e951 appends 13 opaque bytes and one signed 32-bit word.
// Its native producer converts an HP-ratio delta using double1492f8c70
// (10000.0). Report ownership is still a separate contract: this is not a HP/clear
// verdict and must not authorize rewards or arbitrary symbol assignments.
func DecodeAsrahanReport2190(body []byte) (int32, error) {
	if err := raidBodySize(body, 17); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(body[13:])), nil
}

// C2187 writers146d19550/146d1ac90, C2189 writer14081d780,
// and C2195 writer141cf1c40
// finalize without appending a body. They do not use the 13-byte envelope.
// The decoded current CMD registrations contain no ACK consumer for these IDs;
// recognizing a loading signal is not authorization to fabricate an ACK.
func DecodeAsrahanEmptySignal115(id uint16, body []byte) error {
	if id != CmdAsrahanFinishLoading115 && id != CmdAsrahanSignal2189 && id != CmdAsrahanRestartRevival115 {
		return fmt.Errorf("unbound Asrahan empty command")
	}
	if len(body) != 0 && len(body) != 8 {
		return fmt.Errorf("Asrahan signal contains unexpected fields")
	}
	for _, b := range body {
		if b != 0 {
			return fmt.Errorf("Asrahan signal contains nonzero padding")
		}
	}
	return nil
}
