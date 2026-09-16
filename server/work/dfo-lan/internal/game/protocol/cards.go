package protocol

import "fmt"

type CardSelection struct{ Side, Index byte }

// Current146ab9103/8f47 write two u8 values. Transport pads to a block.
func DecodeCardSelection(p []byte) (CardSelection, error) {
	if len(p) != 2 && len(p) != 8 && len(p) != 16 {
		return CardSelection{}, fmt.Errorf("invalid card request size")
	}
	for _, b := range p[2:] {
		if b != 0 {
			return CardSelection{}, fmt.Errorf("nonzero card padding")
		}
	}
	if p[0] > 1 || p[1] > 3 {
		return CardSelection{}, fmt.Errorf("invalid card side/index")
	}
	return CardSelection{p[0], p[1]}, nil
}
func CardLayout() []byte {
	p := []byte{1}
	for i := 0; i < 8; i++ {
		n := uint16(65535)
		if i == 0 {
			n = 1
		}
		p = add16(p, n)
	}
	return p
}

// CMD71's native handler unconditionally reads all eight rows. A short
// generic refusal would overrun the packet; return a full owned snapshot.
func CardSelected(index int) ([]byte, error) {
	if index < -1 || index > 3 {
		return nil, fmt.Errorf("invalid selected card")
	}
	p := []byte{1}
	for i := 0; i < 8; i++ {
		a := byte(255)
		if i == index {
			a = 0
		}
		p = append(p, a, 255, 0, 0)
	}
	return p, nil
}

type SettlementExit struct{ State, Option byte }

func DecodeSettlementExit(p []byte) (SettlementExit, error) {
	if len(p) != 3 && len(p) != 8 && len(p) != 16 {
		return SettlementExit{}, fmt.Errorf("invalid exit body")
	}
	// Current146ab8b43/54/63 writes state, option, literal1.
	if p[2] != 1 {
		return SettlementExit{}, fmt.Errorf("unsupported exit source")
	}
	for _, b := range p[3:] {
		if b != 0 {
			return SettlementExit{}, fmt.Errorf("nonzero exit padding")
		}
	}
	if (p[0] != 1 && p[0] != 2) || p[1] > 3 {
		return SettlementExit{}, fmt.Errorf("unsupported exit action")
	}
	return SettlementExit{p[0], p[1]}, nil
}
func SettlementExitSuccess(r SettlementExit) []byte { return []byte{1, r.State, r.Option} }
func SettlementExitRefused(option byte) []byte      { return append(Refusal(4), option) }
