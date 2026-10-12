package main

import (
	"dfolan/internal/legion"
	"encoding/binary"
	"testing"
)

func TestDuskyCannotCreateApocalypseSession(t *testing.T) {
	for _, channel := range []uint32{0, 85, 91, 119} {
		s := &legionSession{channelType: channel}
		p := make([]byte, 24)
		binary.LittleEndian.PutUint32(p[13:], legion.DuskyContentID)
		for _, id := range []uint16{legion.CmdStart, legion.CmdEnterDungeon, legion.CmdRewardEnd, legion.CmdFail} {
			r, err := s.handle(townSession(7), p, id)
			if err == nil || len(r.Packets) != 0 || s.apocalypse != nil || s.session != nil || s.nativeRun != nil {
				t.Fatalf("Dusky fell through channel%d opcode%d: %+v %v", channel, id, r, err)
			}
		}
		binary.LittleEndian.PutUint32(p[13:], 1)
		binary.LittleEndian.PutUint16(p[17:], 0xffff)
		if r, err := s.handle(townSession(7), p, legion.CmdDuskyOperationSelect); err == nil || len(r.Packets) != 0 {
			t.Fatal("unbound selection fabricated a reply")
		}
	}
}
