package protocol

import (
	"encoding/binary"
	"fmt"
)

// OrdinaryPartyInvite115 is CMD10 action0 (145f159d0), distinct from action15.
// The UI metrics remain opaque: they are neither authority nor gameplay rules.
type OrdinaryPartyInvite115 struct {
	Actor   uint16
	Metrics [3]uint32
	Option  byte
}

func DecodeOrdinaryPartyInvite115(p []byte) (OrdinaryPartyInvite115, error) {
	var r OrdinaryPartyInvite115
	if len(p) != 16 || p[2] != 0 {
		return r, fmt.Errorf("invalid ordinary invitation")
	}
	r.Actor = binary.LittleEndian.Uint16(p)
	if r.Actor == 0 || r.Actor == 65535 || p[15] > 1 {
		return r, fmt.Errorf("invalid ordinary invitation identity or option")
	}
	for i := range r.Metrics {
		r.Metrics[i] = binary.LittleEndian.Uint32(p[3+4*i:])
	}
	r.Option = p[15]
	return r, nil
}

// OrdinaryPartyRoster115 uses the common full-info N9 header. The native
// 146d77f50 reader consumes a u32 length at offset65 even for an empty blob;
// member count therefore starts at69. Content-specific extensions are omitted.
// The local human captain is always seat0; robots occupy consecutive seats.
func OrdinaryPartyRoster115(id uint16, context [2]byte, actors []uint16) ([]byte, error) {
	if id == 0 || id == 65535 || len(actors) == 0 || len(actors) > 4 {
		return nil, fmt.Errorf("invalid ordinary party")
	}
	var seats [4]uint16
	for slot, actor := range actors {
		if actor == 0 || actor == 65535 {
			return nil, fmt.Errorf("invalid ordinary member")
		}
		seats[slot] = actor
	}
	return PartyRosterSeats115(id, context, seats, actors[0], 4)
}

// OrdinaryPartyGone115 removes the roster and cached actor bindings. N10 alone
// only updates connection state/chat. Native 1452f2620 action3 calls 145f10390
// for all eight seats, refreshes the owner HUD and deletes the roster by id.
func OrdinaryPartyGone115(id uint16, context [2]byte) ([]byte, error) {
	return ConquestPartyGone115(id, context, 0)
}

// DecodeOrdinaryPartyLeave115 accepts the native zero u32 and the observed
// eight-byte decrypted body (zero transport padding), never arbitrary tails.
func DecodeOrdinaryPartyLeave115(p []byte) error {
	if len(p) != 4 && len(p) != 8 {
		return fmt.Errorf("invalid leave size")
	}
	for _, v := range p {
		if v != 0 {
			return fmt.Errorf("invalid leave body")
		}
	}
	return nil
}

// 146a3fc8e sends CMD14 with a single seat byte via 146d75cc0. The live
// decrypted request is sixteen bytes with a zero tail; seat0 is the owner.
func DecodeOrdinaryPartyKick115(p []byte) (byte, error) {
	if len(p) != 1 && len(p) != 16 {
		return 0, fmt.Errorf("invalid kick size")
	}
	if p[0] < 1 || p[0] > 3 {
		return 0, fmt.Errorf("invalid robot seat")
	}
	for _, v := range p[1:] {
		if v != 0 {
			return 0, fmt.Errorf("invalid kick padding")
		}
	}
	return p[0], nil
}
