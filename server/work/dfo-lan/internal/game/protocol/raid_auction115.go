package protocol

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

type RaidAuctionBid115 struct {
	Pass   bool
	Amount uint32
}

// Writer1425c6830: u8(inverted UI flag), u32 gold, plus cipher padding.
// Both proven native bid buttons pass false and emit1. The flag0 action's
// caller is not yet closed, so it cannot authorize a wallet transition.
func DecodeRaidAuctionBid115(p []byte) (RaidAuctionBid115, error) {
	if err := raidBodySize(p, 5); err != nil {
		return RaidAuctionBid115{}, err
	}
	if p[0] != 1 {
		return RaidAuctionBid115{}, fmt.Errorf("invalid native auction bid action")
	}
	r := RaidAuctionBid115{Amount: binary.LittleEndian.Uint32(p[1:])}
	return r, nil
}

// N2060 reader1425c3190 uses signed count then a13B stackable row. The
// equipment branch consumes181 more bytes and is deliberately a separate ABI.
func RaidAuctionItems115(templates []uint32) ([]byte, error) {
	if len(templates) > 255 {
		return nil, fmt.Errorf("auction item index exceeds byte")
	}
	p := add32(nil, uint32(len(templates)))
	for _, id := range templates {
		if id == 0 {
			return nil, fmt.Errorf("empty auction template")
		}
		p = add32(add32(p, id), 1)
		p = append(p, 0, 0, 0, 0, 0)
	}
	return p, nil
}

// N2059 reader1425c3b60:3u32. Consumer1425c5b80 advances the already loaded
// list and checks its item's template against operand1; operand2 is unused.
func RaidAuctionStart115(template, minimum uint32) []byte {
	return add32(add32(add32(nil, template), 0), minimum)
}

// N2061 reader1425c3970: signed count then (actor u16, joined u8).
func RaidAuctionJoin115(actor uint16, joined bool) []byte {
	p := add16(add32(nil, 1), actor)
	b := byte(0)
	if joined {
		b = 1
	}
	return append(p, b)
}

// ACK1902 success consumer1425c2dc0 consumes actor/name, item index, three
// scalar operands, result0=bid/1=winner/2=unsold, and remaining milliseconds.
// History1425c3c90 and resume1425c5490 preserve the first two operands, but
// bid panel1425ce5b0 does not consume either of them. They are zero-filled
// unused operands, not inferred financial amounts. The third drives the owned
// lot and next minimum in1425c6b80/1425ce5b0; the ledger owns all gold changes.
func RaidAuctionUpdate115(actor uint16, name string, index byte, price uint32, result byte, millis uint32) ([]byte, error) {
	if actor == 0 || actor == 65535 || name == "" || len(name) >= 32 || !utf8.ValidString(name) || result > 2 {
		return nil, fmt.Errorf("invalid native auction owner/update")
	}
	p := add16([]byte{1}, actor)
	p = addName(p, name)
	p = append(p, index)
	p = add32(add32(add32(p, 0), 0), price)
	p = append(p, result)
	return add32(p, millis), nil
}
