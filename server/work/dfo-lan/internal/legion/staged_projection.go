package legion

import (
	"encoding/binary"
	"fmt"
)

const (
	CmdCorridorOperation uint16 = 2080
	CmdDuskyOperation    uint16 = 2143
	NotiCorridorInfo     uint16 = 2314
	NotiDuskyInfo        uint16 = 2467
)

// Both current native writers carry 13 opaque bytes, action u32, index i16.
// The index is an operation identity, not a difficulty or stage ordinal.
type StagedOperationRequest struct {
	Action uint32
	Index  int16
}

func DecodeStagedOperation(p []byte) (StagedOperationRequest, error) {
	if len(p) < EnvelopeSize+6 {
		return StagedOperationRequest{}, fmt.Errorf("native staged operation request truncated")
	}
	return StagedOperationRequest{binary.LittleEndian.Uint32(p[13:]), int16(binary.LittleEndian.Uint16(p[17:]))}, nil
}

// Ten-byte records (14209fbe0/141c22e60) contain two distinct signed
// parameters. Tail is retained verbatim until its fields are established.
type StagedOperationRecord struct {
	Index, ParameterA, ParameterB int16
	Tail                          [4]byte
}

func DecodeStagedOperationRecord(p []byte) (StagedOperationRecord, error) {
	var r StagedOperationRecord
	if len(p) != 10 {
		return r, fmt.Errorf("native staged operation record must be 10 bytes")
	}
	r.Index = int16(binary.LittleEndian.Uint16(p))
	r.ParameterA = int16(binary.LittleEndian.Uint16(p[2:]))
	r.ParameterB = int16(binary.LittleEndian.Uint16(p[4:]))
	copy(r.Tail[:], p[6:])
	return r, nil
}
func (r StagedOperationRecord) Bytes() []byte {
	p := make([]byte, 10)
	binary.LittleEndian.PutUint16(p, uint16(r.Index))
	binary.LittleEndian.PutUint16(p[2:], uint16(r.ParameterA))
	binary.LittleEndian.PutUint16(p[4:], uint16(r.ParameterB))
	copy(p[6:], r.Tail[:])
	return p
}

// StagedProjection only patches an existing native projection. It does not
// fabricate missing header fields, weekly dates, timestamps or waiting states.
// This keeps the fully verified offsets usable without guessing a start packet.
type StagedProjection struct {
	id   uint16
	body []byte
}

func ReadStagedProjection(id uint16, p []byte) (*StagedProjection, error) {
	want := 0
	switch id {
	case NotiCorridorInfo:
		want = 107
	case NotiDuskyInfo:
		want = 133
	default:
		return nil, fmt.Errorf("not a bound staged projection")
	}
	if len(p) != want {
		return nil, fmt.Errorf("native staged projection length%d want%d", len(p), want)
	}
	return &StagedProjection{id, append([]byte(nil), p...)}, nil
}
func (p *StagedProjection) Bytes() []byte { return append([]byte(nil), p.body...) }
func (p *StagedProjection) Operation(slot int) (StagedOperationRecord, error) {
	start, count := 63, 4
	if p.id == NotiDuskyInfo {
		start, count = 59, 7
	}
	if slot < 0 || slot >= count {
		return StagedOperationRecord{}, fmt.Errorf("operation slot outside native geometry")
	}
	return DecodeStagedOperationRecord(p.body[start+slot*10 : start+(slot+1)*10])
}
func (p *StagedProjection) SetOperation(slot int, r StagedOperationRecord) error {
	start, count := 63, 4
	if p.id == NotiDuskyInfo {
		start, count = 59, 7
	}
	if slot < 0 || slot >= count {
		return fmt.Errorf("operation slot outside native geometry")
	}
	copy(p.body[start+slot*10:], r.Bytes())
	return nil
}

// Corridor's three selected bosses are u32 fields inside 8B rows. Dusky's
// corresponding source index is a single byte inside 12B rows (1471b0f60,
// 141c0ebd0). Writing a u32 there would overwrite three unrelated bytes.
func (p *StagedProjection) DungeonIndex(stage int) (uint32, error) {
	if stage < 0 || stage >= 3 {
		return 0, fmt.Errorf("stage outside native projection")
	}
	if p.id == NotiDuskyInfo {
		return uint32(p.body[23+stage*12]), nil
	}
	return binary.LittleEndian.Uint32(p.body[23+stage*8:]), nil
}

func (p *StagedProjection) SetDungeonIndex(stage int, index uint32) error {
	if stage < 0 || stage >= 3 {
		return fmt.Errorf("stage outside native projection")
	}
	if p.id == NotiDuskyInfo {
		if index > 255 {
			return fmt.Errorf("Dusky source index exceeds native byte")
		}
		p.body[23+stage*12] = byte(index)
		return nil
	}
	binary.LittleEndian.PutUint32(p.body[23+stage*8:], index)
	return nil
}
