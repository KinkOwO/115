package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestAsrahanNativeDebuffAllSlotsBound(t *testing.T) {
	body, err := AsrahanDebuffInfo115([]byte{1, 1, 4})
	if err != nil {
		t.Fatal(err)
	}
	if body[0] != 3 || !bytes.Equal(body[1:4], []byte{1, 1, 4}) || !bytes.Equal(body[4:], bytes.Repeat([]byte{0xff}, 7)) {
		t.Fatal("unused slots create phantom debuffs")
	}
	body[10] = 5
	if _, err := ReadAsrahanNativeProjection115(NotiAsrahanDebuffs115, body); err == nil {
		t.Fatal("inactive tail can write past native stack array")
	}
	if _, err := AsrahanDebuffInfo115(bytes.Repeat([]byte{0}, 11)); err == nil {
		t.Fatal("eleventh source slot accepted")
	}
}
func TestAsrahanProjectionRetainsNativeWordsAndDirection(t *testing.T) {
	for _, id := range []uint16{NotiAsrahanMu115, NotiAsrahanPattern115, NotiAsrahanOverflow115} {
		body := bytes.Repeat([]byte{0xa5}, asrahanNativeSize115(id))
		p, err := ReadAsrahanNativeProjection115(id, body)
		if err != nil {
			t.Fatal(err)
		}
		body[0] = 0
		if p.Bytes()[0] != 0xa5 {
			t.Fatal("projection retains mutable caller buffer")
		}
		before := p.Bytes()
		if err := p.SetWord(1, 0x12345678); err != nil {
			t.Fatal(err)
		}
		after := p.Bytes()
		if binary.LittleEndian.Uint32(after[4:]) != 0x12345678 || !bytes.Equal(before[:4], after[:4]) || !bytes.Equal(before[8:], after[8:]) {
			t.Fatal("unrelated native fields replaced")
		}
		if _, err := ReadAsrahanNativeProjection115(id, after[:len(after)-1]); err == nil {
			t.Fatal("truncated native notification accepted")
		}
	}
	if _, err := ReadAsrahanNativeProjection115(CmdAsrahanFinishLoading115, make([]byte, 8)); err == nil {
		t.Fatal("bound NOTI2187 attribute packet as CMD2187 loading")
	}
}
func TestAsrahanEmptySignalAndBeastEnvelope(t *testing.T) {
	for _, id := range []uint16{CmdAsrahanFinishLoading115, CmdAsrahanSignal2189, CmdAsrahanRestartRevival115} {
		if err := DecodeAsrahanEmptySignal115(id, nil); err != nil {
			t.Fatal(err)
		}
		if err := DecodeAsrahanEmptySignal115(id, make([]byte, 8)); err != nil {
			t.Fatal(err)
		}
		if err := DecodeAsrahanEmptySignal115(id, make([]byte, 13)); err == nil {
			t.Fatal("invented legion envelope accepted for empty native signal")
		}
	}
	body := make([]byte, 16)
	body[13] = 4
	choice, err := DecodeAsrahanUseBeast115(body)
	if err != nil || choice != 4 {
		t.Fatal("lost native C2188 byte after envelope")
	}
	body[15] = 1
	if _, err := DecodeAsrahanUseBeast115(body); err == nil {
		t.Fatal("nonzero transport tail accepted")
	}
}

func TestAsrahanReportWordKeepsSignedValueAndTransportBoundary(t *testing.T) {
	for _, size := range []int{17, 24} {
		body := bytes.Repeat([]byte{0}, size)
		for i := 0; i < 13; i++ {
			body[i] = byte(i + 1)
		}
		binary.LittleEndian.PutUint32(body[13:], 0xfffffffe)
		got, err := DecodeAsrahanReport2190(body)
		if err != nil || got != -2 {
			t.Fatal("changed signed native report or included opaque envelope", got, err)
		}
	}
	for _, size := range []int{0, 13, 16, 18, 25} {
		if _, err := DecodeAsrahanReport2190(make([]byte, size)); err == nil {
			t.Fatalf("accepted unrelated/truncated report geometry: %d", size)
		}
	}
	body := make([]byte, 24)
	body[23] = 1
	if _, err := DecodeAsrahanReport2190(body); err == nil {
		t.Fatal("nonzero transport suffix accepted")
	}
}
