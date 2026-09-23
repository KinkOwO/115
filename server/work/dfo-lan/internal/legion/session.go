// Package legion holds the in-memory state and wire contract for the legion /
// apocalypse subsystem (军团频道 / 末世录).
//
// Two standing decisions are recorded in
// analysis/tasks/next64-legion-apocalypse-plan.md — read §6.1 (the decisions)
// and §6.2 (the compromises they cost) before extending this package:
//
//	D2 solo party: a legion is a multi-member party in the retail client, but
//	   this server models exactly one member (the player). There is no party
//	   table and no member synchronisation, so member lists carry our single
//	   entry and the remaining slots stay zero (§6.2 T1/T4).
//	D3 no persistence: the session lives in process memory. A server restart
//	   drops it and the player falls out of the operation (§6.2 D3a).
//
// Everything here is derived from static analysis of client/DFO.exe.i64; the
// per-packet evidence lives in analysis/tasks/next65-legion-packet-table.md.
package legion

import (
	"encoding/binary"
	"fmt"
)

// C2S opcodes of the legion / apocalypse family (ENUM_CMDPACKET_*).
//
// Note that the client's cmd and noti families share one 16-bit id space with
// different meanings, so an id alone never identifies a packet: 0x8CD is
// CMD_SAVE_TRAINING_ROOM_PRESET and NOTI_LEGION_ADDITIONAL_CLEAR_REWARD at the
// same time. Only the cmd side is client-to-server.
const (
	CmdStart           uint16 = 2043 // ENUM_CMDPACKET_LEGION_START
	CmdFail            uint16 = 2044 // ENUM_CMDPACKET_LEGION_FAIL
	CmdEnterDungeon    uint16 = 2045 // ENUM_CMDPACKET_LEGION_ENTER_DUNGEON
	CmdRewardEnd       uint16 = 2046 // ENUM_CMDPACKET_LEGION_REWARD_END
	CmdOperationSelect uint16 = 2354 // ENUM_CMDPACKET_LEGION_OPERATION_SELECT
	CmdRoleSelect      uint16 = 2355 // ENUM_CMDPACKET_APOCALYPSE_ROLE_SELECT

	CmdVenusOperationSelect uint16 = 2290 // ENUM_CMDPACKET_VENUS_OPERATION_SELECT
	CmdVenusEndAtPhase4     uint16 = 2293 // ENUM_CMDPACKET_VENUS_END_AT_PHASE4
)

// EnvelopeSize is the fixed 13-byte prefix carried by every packet of this
// family. Measured in the client: sub_146D746E0 writes
// "01 | opcode(u16 LE) | 0x0000000000000000 | 0x0000" before the caller
// appends its own fields. The five protocol decoders already verified against
// the live client in this repo read their first field at p[13:] (awakening.go,
// advancement.go, dungeon.go, unified_option.go, world.go), so we follow the
// same convention.
//
// Residual uncertainty (tracked as X1 in next64 §6.2): whether the length the
// caller passes to the sender already includes this envelope. Decoders here
// therefore require only "long enough to hold the field" and report the
// observed length instead of hard-rejecting on an exact size — the first live
// session settles it from the logged plain_hex.
const EnvelopeSize = 13

// Requests reports whether the opcode belongs to this family. The caller still
// has to guard on world state; this only answers "is it ours".
func Requests(id uint16) bool {
	switch id {
	case CmdStart, CmdFail, CmdEnterDungeon, CmdRewardEnd, CmdOperationSelect, CmdRoleSelect:
		return true
	default:
		return false
	}
}

// StartRequest is the decoded CMD2043 LEGION_START body.
type StartRequest struct {
	// Argument is the single u32 the client sends at offset 13. Its meaning is
	// not established yet — the client passes it straight through and no
	// consuming branch has been pinned down, so it is recorded rather than
	// interpreted. Do not invent semantics for it.
	Argument uint32
	// BodyLength is the raw payload length as received. It is carried purely so
	// the first live run can settle X1 (see EnvelopeSize).
	BodyLength int
}

// DecodeStart reads the CMD2043 request. Body layout: u32 @13.
func DecodeStart(p []byte) (StartRequest, error) {
	if len(p) < EnvelopeSize+4 {
		return StartRequest{}, fmt.Errorf("legion start payload %d bytes, want at least %d", len(p), EnvelopeSize+4)
	}
	return StartRequest{
		Argument:   binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		BodyLength: len(p),
	}, nil
}

// StartAckSize is the minimum body the client reads for CMD2043: its handler
// sub_1424FD900 calls sub_146EA0BE0(&v27, 4). A shorter body makes the reader
// write through a null pointer and the client dies, so this size is a hard
// contract rather than a suggestion. The four bytes themselves are consumed
// and discarded by the handler — the client rebuilds its own screen state — so
// the contents do not matter.
const StartAckSize = 4

// StartAck builds the CMD2043 response body.
func StartAck() []byte { return make([]byte, StartAckSize) }

// OperationChannelCode is the u16 both ends of CMD2354/CMD2896/CMD2895 carry as
// the first field. The client's handler refuses the packet unless it reads
// exactly 107, and the client's own sender passes 107 as well, so it behaves
// like a channel or mode tag rather than data.
const OperationChannelCode = 107

// OperationSelectRequest is the decoded CMD2354 LEGION_OPERATION_SELECT body.
//
// Layout (next65 §1, evidence sub_1424FE3F0): int32 @13, char @17, int32 @18.
// The client's own callers pin the first field: sub_14069B580 sends 1 and
// sub_14069B560 sends 2, so @13 is the action picked on the operation screen.
// @17 is passed straight through by the UI and is recorded rather than
// interpreted. @18 carries the 107 channel code.
type OperationSelectRequest struct {
	Action     uint32
	Auxiliary  byte
	Channel    uint32
	BodyLength int
}

// DecodeOperationSelect reads the CMD2354 request.
func DecodeOperationSelect(p []byte) (OperationSelectRequest, error) {
	const want = EnvelopeSize + 9 // 4 + 1 + 4
	if len(p) < want {
		return OperationSelectRequest{}, fmt.Errorf("legion operation payload %d bytes, want at least %d", len(p), want)
	}
	return OperationSelectRequest{
		Action:     binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Auxiliary:  p[EnvelopeSize+4],
		Channel:    binary.LittleEndian.Uint32(p[EnvelopeSize+5:]),
		BodyLength: len(p),
	}, nil
}

// OperationAckSize is the minimum body the client reads for CMD2354: its
// handler sub_1424FD320 reads a u16 and then a 14-byte structure, so anything
// shorter makes the reader write through a null pointer.
const OperationAckSize = 2 + 14

// OperationAck echoes the action the player requested.
//
// The client handler (sub_1424FD320) reads the channel u16, then the 14-byte
// structure, and hands it to sub_14069A360, which branches on the first dword:
// 1 refreshes the operation screen (using a dword at +6), 2 confirms and enters
// — on that branch the client sends CMD2045 itself. Both ends use 1 and 2 for
// the same two actions, so echoing the request back is the smallest assumption
// that lets either branch run.
//
// ASSUMPTION (tracked as X4 in next64 §6.2): the reply is expected to echo the
// request's action, and the dword at +6 is left zero. Only the 107 channel code
// and the length are contract. If the live client rejects the echo, the
// fallback is to leave the whole structure zero — its handler then returns
// early and does nothing, which is safe but inert.
func OperationAck(action uint32) []byte {
	body := make([]byte, OperationAckSize)
	binary.LittleEndian.PutUint16(body, OperationChannelCode)
	binary.LittleEndian.PutUint32(body[2:], action)
	return body
}

// EnterDungeonRequest is the decoded CMD2045 LEGION_ENTER_DUNGEON body.
//
// Layout (next65 §1, evidence sub_1424FE290): int32 @13, int32 @17. The
// client's own caller pins the fields: the operation-screen confirm branch
// invokes sub_1424FE290(107, *(a1 + 115)) — the same 107 channel code as the
// rest of the family, and the operation id held by the screen object.
type EnterDungeonRequest struct {
	Channel    uint32
	Operation  uint32
	BodyLength int
}

// DecodeEnterDungeon reads the CMD2045 request.
func DecodeEnterDungeon(p []byte) (EnterDungeonRequest, error) {
	const want = EnvelopeSize + 8 // 4 + 4
	if len(p) < want {
		return EnterDungeonRequest{}, fmt.Errorf("legion enter payload %d bytes, want at least %d", len(p), want)
	}
	return EnterDungeonRequest{
		Channel:    binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Operation:  binary.LittleEndian.Uint32(p[EnvelopeSize+4:]),
		BodyLength: len(p),
	}, nil
}

// EnterDungeonAckSize is the body size the client reads for CMD2045: its
// handler sub_1424FD160 calls sub_146EA0BE0(v11, 13).
const EnterDungeonAckSize = 13

// EnterDungeonAck reports success.
//
// The handler reads 13 bytes and inspects only the first dword as a result
// code: 0 leaves the flow alone, 252 and 380 map to two localised failure
// strings. Sending zeroes therefore means "accepted" — and the remaining three
// fields (whose pre-read initialisers are 108, -1 and 0) are unused on that
// path, so they are left zero rather than pretended to mean something.
func EnterDungeonAck() []byte { return make([]byte, EnterDungeonAckSize) }

// RoleSelectRequest is the decoded CMD2355 APOCALYPSE_ROLE_SELECT body.
//
// Layout (next65 §1, evidence sub_14069E3B0): int32 @13, int32 @17 = 107.
// The client's callers pin @13: sub_14069B4E0 sends the role value only when it
// differs from the one already stored for this slot, so the packet is an
// edge-triggered "the player changed their role" notification rather than a
// poll. @17 carries the usual 107 channel code.
type RoleSelectRequest struct {
	Role       uint32
	Channel    uint32
	BodyLength int
}

// DecodeRoleSelect reads the CMD2355 request.
func DecodeRoleSelect(p []byte) (RoleSelectRequest, error) {
	const want = EnvelopeSize + 8 // 4 + 4
	if len(p) < want {
		return RoleSelectRequest{}, fmt.Errorf("legion role payload %d bytes, want at least %d", len(p), want)
	}
	return RoleSelectRequest{
		Role:       binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Channel:    binary.LittleEndian.Uint32(p[EnvelopeSize+4:]),
		BodyLength: len(p),
	}, nil
}

// RoleSelectAck is the CMD2355 response body. The client's handler
// sub_14069E320 is a bare pass-through: it clears the pending-response entry
// and forwards to the screen object without reading the payload, so the body is
// empty and only the opcode matters.
func RoleSelectAck() []byte { return nil }

// Session is the per-character legion progress. It is deliberately not
// persisted (D3): the operation is session state, not save data, and keeping
// it out of the database avoids touching archive compatibility at all.
type Session struct {
	CharacterID int64

	// Entered records that CMD2043 was answered for this character.
	Entered bool
	// Argument is the CMD2043 argument as received, kept for the same reason
	// the request carries it.
	Argument uint32

	// Operation is the operation chosen through CMD2354 (P2), RoleValue the
	// role chosen through CMD2355 (P3), Phase the current phase driven by
	// NOTI2657 (P4). They stay zero until their phase lands.
	Operation uint32
	RoleValue uint32
	Phase     byte

	// Action is the last CMD2354 action the player requested (1 or 2).
	// Auxiliary is the accompanying byte the UI passed through.
	Action    uint32
	Auxiliary byte

	// PartySize is always 1: the solo-party decision (D2). It is named rather
	// than inlined so the compromise is visible at every use site.
	PartySize int
}

// NewSession starts a session for a character.
func NewSession(characterID int64) *Session {
	return &Session{CharacterID: characterID, PartySize: 1}
}

// Begin records a CMD2043 request. Re-entering is idempotent: the client may
// re-send the packet when it returns to the channel, and answering again only
// re-opens the same screen, so the stored argument is refreshed and nothing
// else changes.
func (s *Session) Begin(characterID int64, argument uint32) {
	s.CharacterID = characterID
	s.Argument = argument
	s.Entered = true
}

// SelectOperation records a CMD2354 request. The channel code is not stored —
// it is a fixed tag, and a request carrying anything else is refused before
// reaching here. Re-requesting is idempotent, which matters because the client
// sends the action again every time the player re-opens the screen.
func (s *Session) SelectOperation(action uint32, auxiliary byte) {
	s.Action = action
	s.Auxiliary = auxiliary
}

// EnterDungeon records a CMD2045 request: the client sends it when the player
// confirms on the operation screen, and again for later rooms, so this only
// refreshes the stored values.
func (s *Session) EnterDungeon(operation uint32) {
	if operation != 0 {
		s.Operation = operation
	}
	s.Entered = true
}

// SelectRole records a CMD2355 role change. The client only sends the packet
// when the value actually changed, so re-recording the same value is harmless.
func (s *Session) SelectRole(role uint32) { s.RoleValue = role }

// Reset drops the session back to its pre-entry state. It exists for the
// retreat path (P6) and for tests; with D3 there is nothing to persist.
func (s *Session) Reset() {
	s.Entered = false
	s.Operation = 0
	s.RoleValue = 0
	s.Phase = 0
	s.Action = 0
	s.Auxiliary = 0
}
