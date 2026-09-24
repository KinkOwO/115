package protocol

import "testing"

func TestCapturedClearQuestTicket(t *testing.T) {
	if err := DecodeClearQuestTicket(make([]byte, 8)); err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{nil, make([]byte, 4), {1, 0, 0, 0, 0, 0, 0, 0}} {
		if DecodeClearQuestTicket(body) == nil {
			t.Fatalf("accepted unsupported clear body %x", body)
		}
	}
	if b := ClearQuestTicketAccepted(); len(b) != 1 || b[0] != 1 {
		t.Fatalf("unexpected success flag %x", b)
	}
}
