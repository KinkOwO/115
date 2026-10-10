package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Native automatic CMD12 after raid preparation creates an ordinary four-seat
// party (mode0), not another raid. The title identifies the assigned real party.
// Reuse the current N9 grammar, including both native mode stores and empty
// optional extension; never populate the empty seats with fictional actors.
func RaidSoloSubParty115(actor uint16, channel [2]byte, assignment byte, request []byte) ([]byte, error) {
	if assignment < 1 || assignment > 3 || channel[0] == 0 || channel[1] == 0 || len(request) < 36 {
		return nil, fmt.Errorf("invalid owned raid subparty")
	}
	n := int(binary.LittleEndian.Uint32(request[2:6]))
	if n > 63 || n > len(request)-36 {
		return nil, fmt.Errorf("invalid raid subparty title length")
	}
	at := 6 + n
	title := []byte(fmt.Sprintf("Party : %d", assignment))
	if request[0] != 0 || request[1] != 0 || !bytes.Equal(request[6:at], title) {
		return nil, fmt.Errorf("raid subparty is not the assigned automatic create")
	}
	// Live generic writer: capacity4/infoFFFFFFFF/byte5=5, ordinary mode0,
	// eight unconstrained slot filters7, field20FFFFFFFF, no selection/variant.
	want := []byte{4, 255, 255, 255, 255, 5, 0, 0, 0, 0, 0, 0, 7, 7, 7, 7, 7, 7, 7, 7, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(request[at:at+30], want) {
		return nil, fmt.Errorf("unsupported automatic raid subparty options")
	}
	if err := padding(request[at+30:], 16); err != nil {
		return nil, err
	}
	p, err := PartyRosterSeats115(uint16(assignment), channel, [4]uint16{actor}, actor, 4)
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(p[21:25], ^uint32(0))
	p[25] = 5
	copy(p[46:54], want[12:20])
	// The shared minimal packet has an empty title at14. Insert the exact
	// native title before the remaining full-info fields, preserving their order.
	binary.LittleEndian.PutUint32(p[14:18], uint32(n))
	result := append([]byte{}, p[:18]...)
	result = append(result, title...)
	return append(result, p[18:]...), nil
}
