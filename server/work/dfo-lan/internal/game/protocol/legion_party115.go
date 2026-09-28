package protocol

import "fmt"

const ApocalypsePartyMode115 byte = 38
const ApocalypsePartyRoute115 byte = 87

// Current-client registration 149B98AB0: online119/route87/content107/mode38.
// 142ABFBA0 dispatches to 1492E2640+264 = 14250F440, NOT the conquest
// two-byte member-counter reader. That reader always consumes u32 rowCount
// and u8 variableWidth, then (8+2*variableWidth) bytes per supplied member.
// No weekly counter mutation is claimed here: zero rows retains existing
// member counters, while the base packet publishes every real party seat.
func LegionPartyRoster115(id uint16, context [2]byte, seats [4]uint16, leader uint16, options PartyCreateOptions115) ([]byte, error) {
	if options.Mode != ApocalypsePartyMode115 || options.Extra != 0 {
		return nil, fmt.Errorf("unsupported legion party options")
	}
	base, err := PartyRosterSeats115(id, context, seats, leader, options.Capacity)
	if err != nil {
		return nil, err
	}
	base = applySpecialPartyOptions115(base, options, ApocalypsePartyRoute115)
	return append(base, 0, 0, 0, 0, 0), nil
}
