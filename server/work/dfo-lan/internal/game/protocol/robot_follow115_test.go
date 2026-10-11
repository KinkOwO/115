package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestRobotFollowNativeVectors(t *testing.T) {
	data, e := os.ReadFile("testdata/native_robot_follow_vitals115.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Kind     string `json:"kind"`
		Slot     byte   `json:"slot"`
		Payload  string `json:"payload_hex"`
		Consumed int    `json:"consumed"`
		SHA      string `json:"client_sha256"`
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		want, e := hex.DecodeString(r.Payload)
		if e != nil {
			t.Fatal(e)
		}
		if r.SHA != "515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb" || len(want) != r.Consumed {
			t.Fatal("unbound native vector")
		}
		var got []byte
		if r.Kind == "town_vitals" {
			actors := []uint16{65534}
			if len(want) > 7 {
				actors = []uint16{65534, 65533, 65532}
			}
			got, e = PartyTownVitals115(actors)
		} else {
			got, e = StartMap(StartMapState{ReuseRoom: true, Position: [2]byte{2, 3}, Map: 76121, PartyActorSlot: &r.Slot})
		}
		if e != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s/%d native mismatch %x %v", r.Kind, r.Slot, got, e)
		}
	}
	for _, slot := range []byte{0, 4, 255} {
		if _, e = StartMap(StartMapState{ReuseRoom: true, Map: 1, PartyActorSlot: &slot}); e == nil {
			t.Fatal("invalid actor seat")
		}
	}
}
func TestRobotFollowMoveBoundsAndAuthority(t *testing.T) {
	p := []byte{0, 4, 0, 1, 5, 0, 3, 0, 0, 0, 2, 0, 3, 0, 0, 0, 0, 0}
	// Twelve explicit scalar values, including zero. This is a native all-zero flag record.
	p = append(p, make([]byte, 48)...)
	got, e := RobotFollowMove115(p, 3, 65534, [2]uint16{2, 3})
	if e != nil || binary.LittleEndian.Uint16(got[6:]) != 65534 || !bytes.Equal(got[8:], p[8:]) {
		t.Fatal("owner movement not projected", e)
	}
	for i := 0; i < len(p); i++ {
		if _, e = RobotFollowMove115(p[:i], 3, 65534, [2]uint16{2, 3}); e == nil {
			t.Fatalf("accepted truncation %d", i)
		}
	}
	if _, e = RobotFollowMove115(p, 4, 65534, [2]uint16{2, 3}); e == nil {
		t.Fatal("foreign owner")
	}
	if _, e = RobotFollowMove115(p, 3, 65534, [2]uint16{3, 3}); e == nil {
		t.Fatal("stale room")
	}
	if _, e = RobotFollowMove115(append(p, 0), 3, 65534, [2]uint16{2, 3}); e == nil {
		t.Fatal("trailing data")
	}
	other := bytes.Clone(p[4:])
	binary.LittleEndian.PutUint16(other, 2)
	mixed := append([]byte{0, 4, 0, 2}, other...)
	mixed = append(mixed, p[4:]...)
	if projected, err := RobotFollowMove115(mixed, 3, 65534, [2]uint16{2, 3}); err != nil || !bytes.Equal(projected, got) {
		t.Fatal("another object with the same numeric ID captured the owner", err)
	}
	mixed[4] = 5
	if _, err := RobotFollowMove115(mixed, 3, 65534, [2]uint16{2, 3}); err == nil {
		t.Fatal("duplicate player row accepted")
	}
	maximum := []byte{0, 4, 0, 32}
	for i := 0; i < 31; i++ {
		maximum = append(maximum, other...)
	}
	maximum = append(maximum, p[4:]...)
	if _, err := RobotFollowMove115(maximum, 3, 65534, [2]uint16{2, 3}); err != nil {
		t.Fatal("maximum bounded owner movement rejected", err)
	}
	// Native code1 zeros all axes without scalar bytes; code2 retains all axes.
	for _, flags := range []uint32{0x55555500, 0xaaaaaa00, 0xffffff00} {
		compact := append([]byte(nil), p[:18]...)
		binary.LittleEndian.PutUint32(compact[14:], flags)
		if _, e = RobotFollowMove115(compact, 3, 65534, [2]uint16{2, 3}); e != nil {
			t.Fatal("native compact flags", e)
		}
	}
}

func TestRobotFollowMovementNativeVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/native_robot_movement115.json")
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		SHA     string `json:"client_sha256"`
		Vectors []struct {
			Payload  string `json:"payload_hex"`
			Consumed int    `json:"consumed"`
		} `json:"vectors"`
	}
	if err = json.Unmarshal(data, &proof); err != nil || proof.SHA != "515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb" || len(proof.Vectors) != 5 {
		t.Fatal("missing pinned native movement proof", err)
	}
	for _, row := range proof.Vectors {
		want, err := hex.DecodeString(row.Payload)
		if err != nil || len(want) != row.Consumed {
			t.Fatal("invalid native movement vector", err)
		}
		owner := bytes.Clone(want)
		binary.LittleEndian.PutUint16(owner[6:], 3)
		got, err := RobotFollowMove115(owner, 3, 65534, [2]uint16{2, 3})
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("native movement scalar mismatch", err)
		}
	}
}
