package legion

import (
	"encoding/binary"
	"testing"
)

// startPayload builds a CMD2043 body the way the client does: the 13-byte
// envelope followed by the u32 argument.
func startPayload(argument uint32) []byte {
	p := make([]byte, EnvelopeSize+4)
	binary.LittleEndian.PutUint32(p[EnvelopeSize:], argument)
	return p
}

func TestDecodeStartReadsArgumentAtEnvelope(t *testing.T) {
	got, err := DecodeStart(startPayload(0x000004D2))
	if err != nil {
		t.Fatalf("DecodeStart: %v", err)
	}
	if got.Argument != 0x000004D2 {
		t.Fatalf("argument %#x, want 0x4d2", got.Argument)
	}
	if got.BodyLength != EnvelopeSize+4 {
		t.Fatalf("body length %d, want %d", got.BodyLength, EnvelopeSize+4)
	}
}

// The envelope's opcode field must never be mistaken for the argument: a
// decoder that read from offset 0 would return this instead.
func TestDecodeStartIgnoresEnvelope(t *testing.T) {
	p := startPayload(7)
	binary.LittleEndian.PutUint16(p[1:], CmdStart) // envelope: 01 | opcode | zeros
	p[0] = 1
	got, err := DecodeStart(p)
	if err != nil {
		t.Fatalf("DecodeStart: %v", err)
	}
	if got.Argument != 7 {
		t.Fatalf("argument %#x, want 7 (envelope leaked into the field)", got.Argument)
	}
}

// X1 (next64 §6.2): the caller's appended length may or may not already include
// the envelope. Until a live session settles it, a longer body has to decode
// and still report its real length, so the log can tell the two apart.
func TestDecodeStartAcceptsUnsettledLongerBody(t *testing.T) {
	p := make([]byte, EnvelopeSize+17) // envelope + a full 17-byte stack frame
	binary.LittleEndian.PutUint32(p[EnvelopeSize:], 99)
	got, err := DecodeStart(p)
	if err != nil {
		t.Fatalf("DecodeStart: %v", err)
	}
	if got.Argument != 99 {
		t.Fatalf("argument %#x, want 99", got.Argument)
	}
	if got.BodyLength != len(p) {
		t.Fatalf("body length %d, want %d", got.BodyLength, len(p))
	}
}

func TestDecodeStartRejectsShortBody(t *testing.T) {
	if _, err := DecodeStart(make([]byte, EnvelopeSize+3)); err == nil {
		t.Fatal("a body too short to hold the argument must be refused")
	}
	if _, err := DecodeStart(nil); err == nil {
		t.Fatal("an empty body must be refused")
	}
}

// The client reader writes through a null pointer when the body is shorter than
// it expects, so this size is a hard contract.
func TestStartAckMeetsClientReadSize(t *testing.T) {
	if got := len(StartAck()); got != StartAckSize {
		t.Fatalf("StartAck %d bytes, want %d", got, StartAckSize)
	}
	if StartAckSize < 4 {
		t.Fatalf("CMD2043 handler reads 4 bytes; StartAckSize %d would crash the client", StartAckSize)
	}
}

func TestSessionBeginIsIdempotent(t *testing.T) {
	s := NewSession(42)
	s.Begin(42, 1)
	s.Begin(42, 2) // the client re-sends when it re-enters the channel
	if !s.Entered {
		t.Fatal("session should be marked entered")
	}
	if s.Argument != 2 {
		t.Fatalf("argument %d, want the latest (2)", s.Argument)
	}
	if s.PartySize != 1 {
		t.Fatalf("party size %d, want 1 (solo-party decision D2)", s.PartySize)
	}
}

func TestSessionResetClearsEntry(t *testing.T) {
	s := NewSession(42)
	s.Begin(42, 5)
	s.Operation = 9
	s.RoleValue = 3
	s.Phase = 2
	s.Reset()
	if s.Entered || s.Operation != 0 || s.RoleValue != 0 || s.Phase != 0 {
		t.Fatalf("reset left state behind: %+v", s)
	}
}

func TestRequestsCoversFamilyOnly(t *testing.T) {
	for _, id := range []uint16{CmdStart, CmdFail, CmdEnterDungeon, CmdRewardEnd, CmdOperationSelect, CmdRoleSelect} {
		if !Requests(id) {
			t.Fatalf("opcode %d should belong to the legion family", id)
		}
	}
	// Same id space, different family: 0x8CD is a cmd for the training-room
	// preset and a noti for the legion reward. Neither is ours.
	for _, id := range []uint16{CmdVenusOperationSelect, CmdVenusEndAtPhase4, 2062, 37} {
		if Requests(id) {
			t.Fatalf("opcode %d must not be claimed by the legion cmd family", id)
		}
	}
}

// operationPayload mirrors the client's own sender: int32 @13 (the action),
// char @17 (auxiliary), int32 @18 (the 107 channel code).
func operationPayload(action uint32, auxiliary byte, channel uint32) []byte {
	p := make([]byte, EnvelopeSize+9)
	binary.LittleEndian.PutUint32(p[EnvelopeSize:], action)
	p[EnvelopeSize+4] = auxiliary
	binary.LittleEndian.PutUint32(p[EnvelopeSize+5:], channel)
	return p
}

func TestDecodeOperationSelectFields(t *testing.T) {
	got, err := DecodeOperationSelect(operationPayload(2, 0xFF, OperationChannelCode))
	if err != nil {
		t.Fatalf("DecodeOperationSelect: %v", err)
	}
	if got.Action != 2 {
		t.Fatalf("action %d, want 2", got.Action)
	}
	if got.Auxiliary != 0xFF {
		t.Fatalf("auxiliary %#x, want 0xff", got.Auxiliary)
	}
	if got.Channel != OperationChannelCode {
		t.Fatalf("channel %d, want %d", got.Channel, OperationChannelCode)
	}
	if got.BodyLength != EnvelopeSize+9 {
		t.Fatalf("body length %d, want %d", got.BodyLength, EnvelopeSize+9)
	}
}

// The two client callers send 1 (sub_14069B580) and 2 (sub_14069B560); those
// are the only actions the UI offers, so both must round-trip.
func TestDecodeOperationSelectBothActions(t *testing.T) {
	for _, action := range []uint32{1, 2} {
		got, err := DecodeOperationSelect(operationPayload(action, 0, OperationChannelCode))
		if err != nil {
			t.Fatalf("action %d: %v", action, err)
		}
		if got.Action != action {
			t.Fatalf("action %d decoded as %d", action, got.Action)
		}
	}
}

func TestDecodeOperationSelectRejectsShortBody(t *testing.T) {
	if _, err := DecodeOperationSelect(make([]byte, EnvelopeSize+8)); err == nil {
		t.Fatal("a body too short to hold all three fields must be refused")
	}
}

// The handler reads a u16 then a 14-byte structure; anything shorter crashes
// the client.
func TestOperationAckLayout(t *testing.T) {
	body := OperationAck(2)
	if len(body) != OperationAckSize {
		t.Fatalf("OperationAck %d bytes, want %d", len(body), OperationAckSize)
	}
	if OperationAckSize < 2+14 {
		t.Fatalf("CMD2354 handler reads %d bytes; OperationAckSize %d would crash the client", 2+14, OperationAckSize)
	}
	if got := binary.LittleEndian.Uint16(body); got != OperationChannelCode {
		t.Fatalf("channel code %d, want %d", got, OperationChannelCode)
	}
	if got := binary.LittleEndian.Uint32(body[2:]); got != 2 {
		t.Fatalf("echoed action %d, want 2", got)
	}
}

func TestSessionSelectOperationIsIdempotent(t *testing.T) {
	s := NewSession(42)
	s.Begin(42, 1)
	s.SelectOperation(1, 0xFF)
	s.SelectOperation(2, 0x00) // the player re-opens the screen and confirms
	if s.Action != 2 || s.Auxiliary != 0x00 {
		t.Fatalf("action/auxiliary %d/%#x, want the latest 2/0x00", s.Action, s.Auxiliary)
	}
	if !s.Entered {
		t.Fatal("selecting an operation must not clear the entry flag")
	}
}

// twoFieldPayload mirrors the CMD2045/CMD2355 sender shape: int32 @13, int32 @17.
func twoFieldPayload(first, second uint32) []byte {
	p := make([]byte, EnvelopeSize+8)
	binary.LittleEndian.PutUint32(p[EnvelopeSize:], first)
	binary.LittleEndian.PutUint32(p[EnvelopeSize+4:], second)
	return p
}

func TestDecodeEnterDungeonFields(t *testing.T) {
	got, err := DecodeEnterDungeon(twoFieldPayload(OperationChannelCode, 4242))
	if err != nil {
		t.Fatalf("DecodeEnterDungeon: %v", err)
	}
	if got.Channel != OperationChannelCode {
		t.Fatalf("channel %d, want %d", got.Channel, OperationChannelCode)
	}
	if got.Operation != 4242 {
		t.Fatalf("operation %d, want 4242", got.Operation)
	}
	if got.BodyLength != EnvelopeSize+8 {
		t.Fatalf("body length %d, want %d", got.BodyLength, EnvelopeSize+8)
	}
}

func TestDecodeEnterDungeonRejectsShortBody(t *testing.T) {
	if _, err := DecodeEnterDungeon(make([]byte, EnvelopeSize+7)); err == nil {
		t.Fatal("a body too short to hold both fields must be refused")
	}
}

// The handler reads 13 bytes and treats a zero first dword as success; 252 and
// 380 are its two failure codes, so zeroes must never collide with them.
func TestEnterDungeonAckIsSuccess(t *testing.T) {
	body := EnterDungeonAck()
	if len(body) != EnterDungeonAckSize {
		t.Fatalf("EnterDungeonAck %d bytes, want %d", len(body), EnterDungeonAckSize)
	}
	if EnterDungeonAckSize < 13 {
		t.Fatalf("CMD2045 handler reads 13 bytes; EnterDungeonAckSize %d would crash the client", EnterDungeonAckSize)
	}
	if got := binary.LittleEndian.Uint32(body); got != 0 {
		t.Fatalf("result code %d, want 0 (success)", got)
	}
	for _, failure := range []uint32{252, 380} {
		if binary.LittleEndian.Uint32(body) == failure {
			t.Fatalf("success body collides with failure code %d", failure)
		}
	}
}

func TestDecodeRoleSelectFields(t *testing.T) {
	got, err := DecodeRoleSelect(twoFieldPayload(2, OperationChannelCode))
	if err != nil {
		t.Fatalf("DecodeRoleSelect: %v", err)
	}
	if got.Role != 2 {
		t.Fatalf("role %d, want 2", got.Role)
	}
	if got.Channel != OperationChannelCode {
		t.Fatalf("channel %d, want %d", got.Channel, OperationChannelCode)
	}
}

func TestDecodeRoleSelectRejectsShortBody(t *testing.T) {
	if _, err := DecodeRoleSelect(make([]byte, EnvelopeSize+7)); err == nil {
		t.Fatal("a body too short to hold both fields must be refused")
	}
}

// The handler is a bare pass-through that never touches the cursor, so an empty
// body is correct rather than a shortcut.
func TestRoleSelectAckIsEmpty(t *testing.T) {
	if got := RoleSelectAck(); len(got) != 0 {
		t.Fatalf("RoleSelectAck %d bytes, want 0", len(got))
	}
}

func TestSessionEnterAndRoleAreIdempotent(t *testing.T) {
	s := NewSession(42)
	s.Begin(42, 1)
	s.EnterDungeon(77)
	s.EnterDungeon(0) // later rooms may re-send without an operation id
	if s.Operation != 77 {
		t.Fatalf("operation %d, want 77 (a zero must not clobber it)", s.Operation)
	}
	if !s.Entered {
		t.Fatal("entering must keep the entry flag")
	}
	s.SelectRole(3)
	s.SelectRole(3)
	if s.RoleValue != 3 {
		t.Fatalf("role %d, want 3", s.RoleValue)
	}
}

// failBody mirrors the client's CMD2044 sender: envelope + int32 @13.
func failBody(argument uint32) []byte {
	p := make([]byte, EnvelopeSize+4)
	binary.LittleEndian.PutUint32(p[EnvelopeSize:], argument)
	return p
}

func TestDecodeFailFields(t *testing.T) {
	got, err := DecodeFail(failBody(0x0000000C))
	if err != nil {
		t.Fatalf("DecodeFail: %v", err)
	}
	if got.Argument != 0x0C {
		t.Fatalf("argument %#x, want 0xc", got.Argument)
	}
	if got.BodyLength != EnvelopeSize+4 {
		t.Fatalf("body length %d, want %d", got.BodyLength, EnvelopeSize+4)
	}
}

func TestDecodeFailRejectsShortBody(t *testing.T) {
	if _, err := DecodeFail(make([]byte, EnvelopeSize+3)); err == nil {
		t.Fatal("a body shorter than the field was accepted")
	}
}

// The eight bytes are read off the cursor and discarded, so only the length can
// be wrong — and a short one makes the client write through a null pointer.
func TestFailAckMeetsClientReadSize(t *testing.T) {
	if got := len(FailAck()); got != FailAckSize {
		t.Fatalf("FailAck %d bytes, want %d", got, FailAckSize)
	}
	if FailAckSize < 8 {
		t.Fatalf("FailAckSize %d, below the client's 8-byte read (sub_1424FD290)", FailAckSize)
	}
}

// rewardEndBody mirrors the client's CMD2046 sender: int32 @13, int32 @17,
// char @21.
func rewardEndBody(first, second uint32, flag byte) []byte {
	p := make([]byte, EnvelopeSize+9)
	binary.LittleEndian.PutUint32(p[EnvelopeSize:], first)
	binary.LittleEndian.PutUint32(p[EnvelopeSize+4:], second)
	p[EnvelopeSize+8] = flag
	return p
}

func TestDecodeRewardEndFields(t *testing.T) {
	got, err := DecodeRewardEnd(rewardEndBody(0x11, 0x22, 0x05))
	if err != nil {
		t.Fatalf("DecodeRewardEnd: %v", err)
	}
	if got.First != 0x11 || got.Second != 0x22 || got.Flag != 0x05 {
		t.Fatalf("fields %#x/%#x/%#x, want 0x11/0x22/0x5", got.First, got.Second, got.Flag)
	}
	if got.BodyLength != EnvelopeSize+9 {
		t.Fatalf("body length %d, want %d", got.BodyLength, EnvelopeSize+9)
	}
}

func TestDecodeRewardEndRejectsShortBody(t *testing.T) {
	if _, err := DecodeRewardEnd(make([]byte, EnvelopeSize+8)); err == nil {
		t.Fatal("a body shorter than the fields was accepted")
	}
}

func TestRewardEndAckMeetsClientReadSize(t *testing.T) {
	if got := len(RewardEndAck()); got != RewardEndAckSize {
		t.Fatalf("RewardEndAck %d bytes, want %d", got, RewardEndAckSize)
	}
	if RewardEndAckSize < 13 {
		t.Fatalf("RewardEndAckSize %d, below the client's 13-byte read (sub_1424FD3A0)", RewardEndAckSize)
	}
}

// CMD2044 only records what the client announced; the run is still open so the
// log keeps both signals apart. CMD2046 is the teardown.
func TestSessionFailKeepsRunThenRewardEndClosesIt(t *testing.T) {
	s := NewSession(42)
	s.Begin(42, 1)
	s.EnterDungeon(5)
	s.Fail(9)
	if !s.Failed {
		t.Fatal("fail not recorded")
	}
	if s.Operation != 5 || !s.Entered {
		t.Fatalf("fail tore the run down: operation=%d entered=%v", s.Operation, s.Entered)
	}
	s.EndReward()
	if s.Entered || s.Failed || s.Operation != 0 {
		t.Fatalf("reward end left state behind: entered=%v failed=%v operation=%d", s.Entered, s.Failed, s.Operation)
	}
}
