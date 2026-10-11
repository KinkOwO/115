package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// The current DFO.exe's 146D780E9 calls 149185A00, which passes CP65001
// to MultiByteToWideChar. This literal vector pins that reader contract;
// it is an offline golden, not a captured client-generated S2C packet.
func TestMailboxCurrentNativeUTF8Wire(t *testing.T) {
	got, e := MailboxList(nil, []MailTextView{{ID: 209, Sender: "奇迹加速", Text: "奖励已送达，请查收。", SentAt: 1791328792, Status: 2}}, 1)
	if e != nil {
		t.Fatal(e)
	}
	want, e := hex.DecodeString("000001000100d100000000000000000000000c000000e5a587e8bfb9e58aa0e9809f1e000000e5a596e58ab1e5b7b2e98081e8bebeefbc8ce8afb7e69fa5e694b6e380821882c56a02000000000000")
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("wire=%x want=%x", got, want)
	}
	// Valid UTF-8 outside GBK must also survive unchanged, rather than making
	// the whole stored letter disappear during an unnecessary GBK conversion.
	text := "邮件𠮷"
	got, e = MailboxList(nil, []MailTextView{{ID: 1, Sender: "GM", Text: text, SentAt: 1791328792, Status: 1}}, 1)
	if e != nil || !bytes.Contains(got, []byte(text)) {
		t.Fatalf("UTF8 text lost: %x %v", got, e)
	}
}

func TestMailboxAttachmentAndLetterUseSameSentAt(t *testing.T) {
	const sentAt = uint32(1791328792)
	got, e := MailboxList([]MailAttachmentView{{ID: 208, MessageID: 209, Sender: "奇迹加速", Gold: 100, SentAt: sentAt}}, []MailTextView{{ID: 209, Sender: "奇迹加速", Text: "奖励已送达，请查收。", SentAt: sentAt, Status: 1}}, 1)
	if e != nil {
		t.Fatal(e)
	}
	off := 2 + 8
	nameLen := int(binary.LittleEndian.Uint32(got[off:]))
	off += 4 + nameLen
	// Gold/template/reserved/count/durability/quality/reserved fields, then the
	// native 181-byte item record. This layout is from NOTI97's native reader.
	off += 4 + 4 + 1 + 4 + 2 + 1 + 4 + 2 + 2 + CurrentItemRecordSize
	if n := binary.LittleEndian.Uint32(got[off:]); n != sentAt {
		t.Fatalf("attachment timestamp=%d", n)
	}
	off += 4 + 8 + 1 + 2 + 2 + 8 + 4
	nameLen = int(binary.LittleEndian.Uint32(got[off:]))
	off += 4 + nameLen
	textLen := int(binary.LittleEndian.Uint32(got[off:]))
	off += 4 + textLen
	if n := binary.LittleEndian.Uint32(got[off:]); n != sentAt {
		t.Fatalf("letter timestamp=%d", n)
	}
	for _, text := range []string{"a\x00b", string([]byte{255})} {
		if _, e := MailboxList(nil, []MailTextView{{ID: 1, Sender: "GM", Text: text, Status: 1}}, 1); e == nil {
			t.Fatal("invalid text accepted")
		}
	}
}
