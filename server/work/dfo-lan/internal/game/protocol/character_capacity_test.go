package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestCharacterSlotTicketNativeReaders(t *testing.T) {
	// Golden input consumed by1452c6e00: u16 branch8, then a single u8.
	body, err := CharacterCapacityUpdated(25)
	if err != nil || !bytes.Equal(body, []byte{0x08, 0x00, 0x19}) {
		t.Fatalf("NOTI66/8: %x %v", body, err)
	}
	for _, capacity := range []uint16{0, 256} {
		if _, err = CharacterCapacityUpdated(capacity); err == nil {
			t.Fatal("accepted a value the native byte reader cannot represent")
		}
	}
	r := UseStackableRequest{List: 0, Instance: 0x11223344, Template: 2660239}
	body = CharacterSlotTicketRefused(r)
	// Standard class1 dispatcher consumes flag+error16 before14529fd70 reads
	// list8, instance32, template32. Missing error16 shifts all three fields.
	if len(body) != 12 || body[0] != 0 || binary.LittleEndian.Uint16(body[1:]) != 17 || body[3] != r.List || binary.LittleEndian.Uint32(body[4:]) != r.Instance || binary.LittleEndian.Uint32(body[8:]) != r.Template {
		t.Fatalf("refusal reader alignment: %x", body)
	}
}
