package protocol

import (
	"bytes"
	"testing"
)

func TestImageCommunicationAckKeepsNPCInSecondDword(t *testing.T) {
	if got := ImageCommunicationAck(2001); !bytes.Equal(got, []byte{1, 0, 0, 0, 0, 0xd1, 0x07, 0, 0}) {
		t.Fatalf("CMD467 ACK body = %x", got)
	}
}
