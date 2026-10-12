package legion

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDuskySelectionNativeWriterAndFamilyIsolation(t *testing.T) {
	p := make([]byte, 24)
	for i := 0; i < EnvelopeSize; i++ {
		p[i] = byte(i + 91)
	}
	binary.LittleEndian.PutUint32(p[13:], 2)
	binary.LittleEndian.PutUint16(p[17:], 17)
	r, err := DecodeDuskySelection(p[:19])
	if err != nil || r.Action != 2 || r.Index != 17 {
		t.Fatalf("native writer: %+v %v", r, err)
	}
	if _, err := DecodeDuskySelection(p[:18]); err == nil {
		t.Fatal("truncation accepted")
	}
	binary.LittleEndian.PutUint16(p[17:], 0xffff)
	if _, err := DecodeDuskySelection(p); err == nil {
		t.Fatal("negative selection accepted")
	}
	for _, action := range []uint32{1, 4} {
		binary.LittleEndian.PutUint32(p[13:], action)
		if _, err := DecodeDuskySelection(p); err != nil {
			t.Fatal(err)
		}
	}
	if !Requests(CmdDuskyOperationSelect) || !DuskyRequest(CmdDuskyOperationSelect, nil) || DuskyRequest(NotiDuskyInfo, p) {
		t.Fatal("command/notification routing conflated")
	}
	for _, content := range []uint32{101, 102, 103, 104, 105, 106, 107, 359} {
		binary.LittleEndian.PutUint32(p[13:], content)
		for _, id := range []uint16{CmdStart, CmdFail, CmdEnterDungeon, CmdRewardEnd} {
			if DuskyRequest(id, p) != (content == 103) {
				t.Fatalf("content%d opcode%d misrouted", content, id)
			}
		}
	}
}

func TestDuskyRecordMatchesNativeReaderSpans(t *testing.T) {
	s := DuskyInfoState{Prefix: 0xffff, Selected: 3, State: 6, Outcome: 4, Phase: 2, Following: 1, Extra: -1, Reselections: 5}
	for i := range s.Stages {
		s.Stages[i] = DuskyStageRecord{byte(i), int32(i + 10), -1}
	}
	s.History[2] = DuskyOperationRecord{13, 30, 400, 3, 1, 0}
	s.Offers[3] = DuskyOperationRecord{17, 0, 0, 4, 0, 0}
	p := DuskyInfo(s)
	if len(p) != 133 || p[2] != 3 || binary.LittleEndian.Uint32(p[11:]) != 2 || binary.LittleEndian.Uint32(p[19:]) != math.MaxUint32 {
		t.Fatal("header span mismatch")
	}
	if p[47] != 2 || binary.LittleEndian.Uint32(p[51:]) != 12 || binary.LittleEndian.Uint32(p[55:]) != math.MaxUint32 {
		t.Fatal("stage2 record mismatch")
	}
	if binary.LittleEndian.Uint16(p[79:]) != 13 || binary.LittleEndian.Uint16(p[83:]) != 400 || p[87] != 1 {
		t.Fatal("history2 record mismatch")
	}
	if binary.LittleEndian.Uint16(p[119:]) != 17 || binary.LittleEndian.Uint16(p[125:]) != 4 || binary.LittleEndian.Uint16(p[131:]) != 5 {
		t.Fatal("offer3/tail mismatch")
	}
	ack := DuskySelectionAck(4, -1, true, 0x12345678, 1, 7)
	if len(ack) != 20 || ack[0] != 1 || binary.LittleEndian.Uint32(ack[1:]) != 4 || binary.LittleEndian.Uint16(ack[5:]) != 0xffff || ack[7] != 1 || binary.LittleEndian.Uint32(ack[12:]) != 0x12345678 || ack[16] != 1 || binary.LittleEndian.Uint16(ack[18:]) != 7 {
		t.Fatalf("native ACK mismatch: %x", ack)
	}
}

func TestDuskyOperandsNeverTruncateSourceMagnitudes(t *testing.T) {
	for _, v := range []float64{1.5, math.NaN(), math.Inf(1), -32769, 32768} {
		if _, err := DuskyOperationOperand(v); err == nil {
			t.Fatalf("accepted %v", v)
		}
	}
	for _, v := range []float64{-32768, 0, 1, 32767} {
		got, err := DuskyOperationOperand(v)
		if err != nil || float64(got) != v {
			t.Fatalf("lost integral operand %v", v)
		}
	}
}
