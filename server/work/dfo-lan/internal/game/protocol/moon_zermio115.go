package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

type MoonZermioReport115 struct {
	Opaque    [13]byte // deliberately not interpreted as coordinates/identity
	Health    uint64
	MeterBits uint32
}

// Native141326470 appends25B AFTER command-builder's outer13B header.
// Writes at local body+13/+21 are u64 state and float state respectively.
func DecodeMoonZermioReport115(p []byte) (MoonZermioReport115, error) {
	var r MoonZermioReport115
	if len(p) < 25 || len(p) > 40 {
		return r, fmt.Errorf("Moon Zermio report requires25 logical bytes")
	}
	for _, b := range p[25:] {
		if b != 0 {
			return r, fmt.Errorf("nonzero Zermio padding")
		}
	}
	copy(r.Opaque[:], p[:13])
	r.Health = binary.LittleEndian.Uint64(p[13:])
	r.MeterBits = binary.LittleEndian.Uint32(p[21:])
	f := float64(math.Float32frombits(r.MeterBits))
	if r.Health <= 1 || r.Health > math.MaxInt64 || math.IsNaN(f) || math.IsInf(f, 0) {
		return r, fmt.Errorf("invalid live Zermio state")
	}
	return r, nil
}

func MoonZermioState115(p []byte, hp uint64, meter uint32) ([]byte, error) {
	f := float64(math.Float32frombits(meter))
	if len(p) != 126 || hp > math.MaxInt64 || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, fmt.Errorf("invalid Zermio state projection")
	}
	out := append([]byte(nil), p...)
	binary.LittleEndian.PutUint64(out[106:], hp)
	binary.LittleEndian.PutUint32(out[114:], meter)
	return out, nil
}
