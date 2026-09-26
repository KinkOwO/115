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

// SettlementExitSeamless is option 5: the EPLP seamless rechallenge the
// right-edge of the clear panel sends (CMD 72 ENUM_CMDPACKET_EPLP_COMMAND,
// body 01 05 01). The sender is the client's own dungeon module, so the
// gateway only has to stop rejecting it.
const SettlementExitSeamless byte = 5

// KeepsDungeonSelection reports whether this settlement exit leaves the client
// in the dungeon-selection flow. It carries the meaning the third byte of the
// old three-byte acknowledgement accidentally had - that byte was Option, so
// "payload[2] == 1" meant "option == 1". The flag is now derived from the
// decoded request, which is what makes the acknowledgement free to shrink to
// its native width.
func (r SettlementExit) KeepsDungeonSelection() bool { return r.Option == 1 }

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
	// Option 5 is the EPLP seamless rechallenge (right-edge walk-in). The
	// native sender writes state, option, literal1 just like options 0..3, so
	// only the range guard had to widen. Its body shape is pinned below by
	// the same length and padding rules.
	if (p[0] != 1 && p[0] != 2) || (p[1] > 3 && p[1] != SettlementExitSeamless) {
		return SettlementExit{}, fmt.Errorf("unsupported exit action")
	}
	return SettlementExit{p[0], p[1]}, nil
}

// SettlementExitSuccess is the acknowledgement body the native reader
// consumes: exactly two u8s, (state, option). Native handler 0x145244570 reads
// them at 0x1452445ad and 0x1452445b9, and testdata/native_card_exit_*.json pin
// 0100 / 0102 / ... The extra leading 1 this used to carry was not part of the
// body - the incoming request has a literal 1 at p[2] (see DecodeSettlementExit)
// but the outgoing shape does not. Reading that byte back inside the gateway is
// what once turned the acknowledgement's width into an out-of-range panic.
func SettlementExitSuccess(r SettlementExit) []byte { return []byte{r.State, r.Option} }
func SettlementExitRefused(option byte) []byte      { return append(Refusal(4), option) }
