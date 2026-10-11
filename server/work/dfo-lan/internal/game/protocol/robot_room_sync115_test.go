package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestRobotRoomSync115NativeVectors(t *testing.T) {
	data, e := os.ReadFile("testdata/native_robot_room_sync115.json")
	if e != nil {
		t.Fatal(e)
	}
	var proof struct {
		SHA     string `json:"client_sha256"`
		Vectors []struct {
			Command  uint16
			Payload  string `json:"payload_hex"`
			Consumed int
		}
	}
	if e = json.Unmarshal(data, &proof); e != nil || proof.SHA != "515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb" || len(proof.Vectors) != 3 {
		t.Fatal("unbound proof", e)
	}
	for _, v := range proof.Vectors {
		want, _ := hex.DecodeString(v.Payload)
		owner := bytes.Clone(want)
		owner[3] = 0
		got, e := RobotRoomSync115(owner, 1)
		if e != nil || len(got) != v.Consumed || !bytes.Equal(got, want) {
			t.Fatal("native room sync mismatch", v.Command, e)
		}
		for n := 0; n < len(owner); n++ {
			if _, e = RobotRoomSync115(owner[:n], 1); e == nil {
				t.Fatal("truncation accepted", n)
			}
		}
		for _, slot := range []byte{0, 4, 255} {
			if _, e = RobotRoomSync115(owner, slot); e == nil {
				t.Fatal("foreign slot accepted")
			}
		}
		owner[3] = 2
		if _, e = RobotRoomSync115(owner, 1); e == nil {
			t.Fatal("another member's sync accepted")
		}
	}
	if _, e = RobotRoomSync115([]byte{1, 5, 0}, 1); e == nil {
		t.Fatal("combat packet replayed")
	}
	p := make([]byte, 13)
	p[0] = 1
	binary.LittleEndian.PutUint16(p[1:], 192)
	binary.LittleEndian.PutUint32(p[5:], 4)
	if _, e = RobotRoomSync115(p, 1); e == nil {
		t.Fatal("unknown loading stage accepted")
	}
}
func TestRobotFollowActualType5OwnerMovement(t *testing.T) {
	p, _ := hex.DecodeString("0104000105000200000000000000305555410080014400004643")
	got, e := RobotFollowMove115(p, 2, 65534, [2]uint16{0, 0})
	if e != nil {
		t.Fatal("live movement rejected", e)
	}
	want := bytes.Clone(p)
	binary.LittleEndian.PutUint16(want[6:], 65534)
	if !bytes.Equal(got, want) {
		t.Fatal("live type5 movement changed", hex.EncodeToString(got))
	}
	p[4] = 1
	if _, e = RobotFollowMove115(p, 2, 65534, [2]uint16{0, 0}); e == nil {
		t.Fatal("same numeric ID with nonplayer type accepted")
	}
}
