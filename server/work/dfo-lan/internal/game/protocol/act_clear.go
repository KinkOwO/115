package protocol

import (
	"bytes"
	"fmt"
)

// CMD1422 CLEAR_QUEST_TICKET: the native sender writes one zero dword;
// the captured C2S plaintext is two zero dwords after transport alignment.
func DecodeClearQuestTicket(p []byte) error {
	if len(p) != 8 || !bytes.Equal(p, make([]byte, 8)) {
		return fmt.Errorf("unsupported clear quest ticket request")
	}
	return nil
}

// The client CMD1422 reader receives its success byte from the command
// response wrapper and does not consume further payload fields.
func ClearQuestTicketAccepted() []byte { return []byte{1} }
