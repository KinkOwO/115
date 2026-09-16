package wire

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"testing"
)

func TestNativeClientFraming(t *testing.T) {
	raw, _ := hex.DecodeString("0112061d000000a4cd490b0000146454ff9e3d495038ea07a1ee5cc1bd")
	input := io.MultiReader(bytes.NewReader(raw[:2]), bytes.NewReader(raw[2:12]), bytes.NewReader(raw[12:]))
	f, err := ReadClient(input)
	if err != nil || f.ID != 1554 || !bytes.Equal(f.Raw, raw) {
		t.Fatalf("capture did not frame: %+v %v", f, err)
	}
	for _, size := range []uint32{0, 12, MaxPacketSize + 1, 0xffffffff} {
		b := append([]byte(nil), raw[:13]...)
		binary.LittleEndian.PutUint32(b[3:7], size)
		if _, err := ReadClient(bytes.NewReader(b)); err == nil {
			t.Fatalf("accepted size %d", size)
		}
	}
	if _, err := ReadClient(bytes.NewReader(raw[:20])); err == nil {
		t.Fatal("accepted truncated body")
	}
}

func TestNativeChannelCRC(t *testing.T) {
	// Exact CHANNELINFO body used in channel_02 and accepted by native recvPackets.
	pb := append([]byte{0x10, 1, 0x1a, 0x80, 8}, make([]byte, 1024)...)
	for i := 0; i < 1024; i++ {
		pb[5+i] = byte(i%127 + 1)
	}
	pb = append(pb, 0x22, 9)
	pb = append(pb, []byte("LAN Local")...)
	body := make([]byte, 4+len(pb))
	binary.LittleEndian.PutUint32(body, uint32(len(pb)))
	copy(body[4:], pb)
	for i, v := range body {
		x := v ^ 0xb5
		body[i] = x>>6 | x<<2
	}
	if got := Checksum(body); got != 0x50 {
		t.Fatalf("native checksum=50, Go=%02x", got)
	}
}
