package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestAttendanceNativeFirstDayVector(t *testing.T) {
	request, _ := hex.DecodeString("4b01000000000000")
	day, err := DecodeAttendanceDailyClaim115(request)
	if err != nil || day != 0 {
		t.Fatal("day zero rejected", day, err)
	}
	for _, state := range []byte{1, 2} {
		// Exact consumed prefix of the before/after NOTI1379 capture.
		want := make([]byte, 33)
		want[1] = 1
		want[5] = state
		var days [28]byte
		days[0] = state
		got, err := AttendanceDailyStatus115(0, 1, days)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal(got, err)
		}
	}
	reply, _ := hex.DecodeString("014b01000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
	if !bytes.Equal(EventReply115(AttendanceDailyEvent115, [10]uint32{}), reply) {
		t.Fatal("captured success prefix changed")
	}
}

func TestAttendanceRefusalUsesCurrentCommandHeader(t *testing.T) {
	want, _ := hex.DecodeString("0017004b010000")
	if got := AttendanceDailyRefusal115(23); !bytes.Equal(got, want) {
		t.Fatal("native dispatcher consumes error before event", got)
	}
}

func TestAttendanceOpeningCapturedRow(t *testing.T) {
	want, err := hex.DecodeString("4b0101020428000000372d446179204a6f75726e657920666f7220536b79206f6620612054686f7573616e6420536561730000000000000000802b716a8eb8c46a3a0000004c6976652f4576656e742f4b6f722f5c323032365c303332365f417474656e64616e63654461696c794576656e742f63616c2e7875692f302f300000000001")
	if err != nil {
		t.Fatal(err)
	}
	got, err := AppendAttendanceOpening115([]byte{0, 0, 0}, "7-Day Journey for Sky of a Thousand Seas", "Live/Event/Kor/\\2026\\0326_AttendanceDailyEvent/cal.xui", 1785801600, 1791277198)
	if err != nil || !bytes.Equal(got[2:len(got)-1], want) {
		t.Fatalf("captured opening row changed: %x %v", got, err)
	}
}

func TestAttendanceNativeRejectsUnclosedShapes(t *testing.T) {
	for _, raw := range []string{"4b010000", "4b0100001c000000", "4b0100000000000001000000", "9602000000000000"} {
		p, _ := hex.DecodeString(raw)
		if _, err := DecodeAttendanceDailyClaim115(p); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	var days [28]byte
	days[0] = 3
	if _, err := AttendanceDailyStatus115(0, 1, days); err == nil {
		t.Fatal("unknown state accepted")
	}
}
