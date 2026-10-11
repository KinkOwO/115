// Package legion holds the in-memory state and wire contract for the legion /
// apocalypse subsystem (军团频道 / 末世录).
//
// The imported per-connection Session is only a UI/request tracker. The
// production shared party/run owner now lives in internal/partyrun; do not
// restore the upstream solo-only assumption. Rewards use durable personal
// plans/receipts, whereas an active room is still in-memory. Historical
// next64/next65 notes are not current-version function-address evidence.
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
	CmdVenusRelic           uint16 = 2291 // ENUM_CMDPACKET_GET_VENUS_RELIC
	CmdVenusEndAtPhase4     uint16 = 2293 // ENUM_CMDPACKET_VENUS_END_AT_PHASE4
)

// N2895/N2896 and the N2252 personal result/exit path are wired. Other declared
// IDs are not automatically implemented; native full-route acceptance remains.
const (
	NotiClearRewardBasic      uint16 = 2252 // ENUM_NOTIPACKET_LEGION_BASIC_CLEAR_REWARD
	NotiClearRewardAdditional uint16 = 2253 // ENUM_NOTIPACKET_LEGION_ADDITIONAL_CLEAR_REWARD
	NotiEntryCharacterInfo    uint16 = 2254 // ENUM_NOTIPACKET_LEGION_ENTRY_CHARAC_INFO
	NotiPrepareEnterDungeon   uint16 = 2568 // ENUM_NOTIPACKET_PREPARE_LEGION_ENTER_DUNGEON
	NotiPhaseClearTick        uint16 = 2657 // ENUM_NOTIPACKET_LEGION_PHASE_CLEAR_TICK
	NotiLegionInfo            uint16 = 2895 // ENUM_NOTIPACKET_LEGION_INFO
	NotiLegionOperation       uint16 = 2896 // ENUM_NOTIPACKET_LEGION_OPERATION
	NotiDungeonTimeoutTime    uint16 = 1474 // ENUM_NOTIPACKET_DUNGEON_TIMEOUT_TIME
)

// EnvelopeSize is an opaque13-byte prefix INSIDE this family's plaintext.
// The observed start body has12 FF bytes plus00, not a13-byte network header.
// Current1424FEBB0/1424FEA50 write fields at+13/+17 before copying the body.
// Transport padding is not a field; the prefix semantics remain unnamed.
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

// CMD2043: generic success byte, then1424FE0C0 reads a4B result code.
// Inner code0 is accepted; it does NOT replace the outer success byte.
const StartAckSize = 1 + 4

// StartAck builds the CMD2043 response body.
func StartAck() []byte { return successfulReply(StartAckSize) }

// The generic CMD dispatcher consumes the success byte BEFORE invoking the
// per-command reader. Sizes below include this byte; NOTI payloads do not.
func successfulReply(size int) []byte {
	p := make([]byte, size)
	p[0] = 1
	return p
}

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

// CMD2354: generic success byte, then1424FDAE0 reads u16 content +14B.
const OperationAckSize = 1 + 2 + 14

// OperationAck echoes the action the player requested.
//
// The current handler1424FDAE0 reads the content u16, then the14-byte
// structure, and hands it to14069AA80, which branches on the first dword:
// 1 refreshes the operation screen (using a dword at +6), 2 confirms and enters
// — on that branch the client sends CMD2045 itself. Both ends use 1 and 2 for
// the same two actions, so echoing the request back is the smallest assumption
// that lets either branch run.
//
// The structure+6 deadline is still0 in this narrow prefix fix; its clock
// domain needs separate validation. Never replace a failed reply with an
// all-zero action, which would hide a missing UI transition.
func OperationAck(action uint32) []byte {
	body := successfulReply(OperationAckSize)
	binary.LittleEndian.PutUint16(body[1:], OperationChannelCode)
	binary.LittleEndian.PutUint32(body[3:], action)
	return body
}

// EnterDungeonRequest is the decoded CMD2045 LEGION_ENTER_DUNGEON body.
//
// Layout (next65 §1, evidence sub_1424FE290): int32 @13, int32 @17. The
// client's own caller pins the fields: the operation-screen confirm branch
// invokes sub_1424FE290(107, *(a1 + 115)) — the same 107 channel code as the
// rest of the family, and the current stage held at manager+115. Native
// 140698C60 uses that same value to index the6x12 stage array; it is NOT the
// CTP difficulty key. Difficulty is the choice byte+1 (140699750).
type EnterDungeonRequest struct {
	Channel    uint32
	Stage      uint32
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
		Stage:      binary.LittleEndian.Uint32(p[EnvelopeSize+4:]),
		BodyLength: len(p),
	}, nil
}

// CMD2045: one generic success byte plus the13B command-specific block.
const EnterDungeonAckSize = 1 + 13

// EnterDungeonAck reports success.
//
// The handler reads 13 bytes and inspects only the first dword as a result
// code: 0 leaves the flow alone, 252 and 380 map to two localised failure
// strings. Sending zeroes therefore means "accepted" — and the remaining three
// fields (whose pre-read initialisers are 108, -1 and 0) are unused on that
// path, so they are left zero rather than pretended to mean something.
func EnterDungeonAck() []byte { return successfulReply(EnterDungeonAckSize) }

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

// CMD2355 has no command-specific body, but still needs the generic success
// byte. A nil payload is skipped entirely by the host's preparePackets.
func RoleSelectAck() []byte { return successfulReply(1) }

// FailRequest is the decoded CMD2044 LEGION_FAIL body.
//
// Layout (next65 §1, evidence sub_1424FE340): int32 @13, 17 bytes total.
// The client sends it from the run-abandon path. @13 is passed straight through
// by the caller and no consuming branch has been pinned down, so it is recorded
// rather than interpreted — do not read a reason code out of it.
type FailRequest struct {
	Argument   uint32
	BodyLength int
}

// DecodeFail reads the CMD2044 request.
func DecodeFail(p []byte) (FailRequest, error) {
	if len(p) < EnvelopeSize+4 {
		return FailRequest{}, fmt.Errorf("legion fail payload %d bytes, want at least %d", len(p), EnvelopeSize+4)
	}
	return FailRequest{
		Argument:   binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		BodyLength: len(p),
	}, nil
}

// FailAckSize is the body size the client reads for CMD2044: its handler
// sub_1424FD290 calls sub_146EA0BE0(v5, 8). The eight bytes are discarded
// afterwards, so only the length is contract.
const FailAckSize = 1 + 8

// FailAck builds the CMD2044 response body.
func FailAck() []byte { return successfulReply(FailAckSize) }

// RewardEndRequest is the decoded CMD2046 LEGION_REWARD_END body.
//
// Historical layout: int32 @13 = a1, int32 @17 = a3,
// char @21 = a2, 22 bytes total. The register names in the client are the only
// names we have, so the fields keep their positions instead of being renamed
// into gameplay meanings.
type RewardEndRequest struct {
	First      uint32
	Second     uint32
	Flag       byte
	BodyLength int
}

// DecodeRewardEnd reads the CMD2046 request.
func DecodeRewardEnd(p []byte) (RewardEndRequest, error) {
	const want = EnvelopeSize + 9 // 4 + 4 + 1
	if len(p) < want {
		return RewardEndRequest{}, fmt.Errorf("legion reward-end payload %d bytes, want at least %d", len(p), want)
	}
	return RewardEndRequest{
		First:      binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Second:     binary.LittleEndian.Uint32(p[EnvelopeSize+4:]),
		Flag:       p[EnvelopeSize+8],
		BodyLength: len(p),
	}, nil
}

// RewardEndAckSize is the body size the client reads for CMD2046: its handler
// sub_1424FD3A0 calls sub_146EA0BE0(v50, 13). Again only the length is
// contract. G0380 re-located the current handler via the2046 registration at
// 1424FEEDE:1424FDB60 still reads13B, then invokes client reward/UI cleanup.
// The older1424FD3A0 address is not this handler in2.38.3.25. Length evidence
// alone does not authorize a successful reply before durable own rewards.
const RewardEndAckSize = 1 + 13

// RewardEndAck builds the CMD2046 response body.
func RewardEndAck() []byte { return successfulReply(RewardEndAckSize) }

// DimCloisterRewardEndAck 构造**次元回廊**的 CMD2046 应答（照官服 s30 的 32 字节形状）。
//
// ★★ 2026-10-11 第二十四轮：业主实机「清完第 1 界，回城后不能选下一界」，
// 服务端日志显示客户端后来**再没发过第二次进图请求**（`enter request` 只出现一次）。
//
// 官服 c2s/s2c 逐帧对照给出原因 —— 官服的 CMD2046 应答是 **32 字节**，而且带界号：
//
//	#829（第 1 界清完）: 01 00000000 66 000000 | 00 000000 | 03 0b 04 d7 c6 39 | 00…
//	#903（第 2 界清完）: 01 00000000 66 000000 | 01 000000 | 02 65 56 4d 49 43 | 00…
//	#986（第 3 界清完）: 01 00000000 66 000000 | 02 000000 | 01 dd 05 01 56 3f | 00…
//	                     ↑ @0=1（成功） @5=0x66       ↑ @9=**界号**  ↑ @13.. 未知尾
//
// 而本仓此前回的是通用 `RewardEndAck()` = `[0]=1` + 13 个零 = **14 字节**，
// 连界号都没有 —— 客户端拿到这种应答无法把"本界结算完成"记进去，
// 于是界数不推进、回城后选不了下一界。
//
// ❗注意：这只改**次元回廊**这一条路。通用 `RewardEndAck()` 仍被维纳斯/苏醒之森等
// 共用（它们已实机验证可用），不动它。
//
// @13 起那 4 字节（`03/0b/04/d7`、`02/65/56/4d`、`01/dd/05/01`）语义未解，
// 语义未明就照官服逐界取值（同 `IspinsRewardEndAck` 的 nonce 做法），第 3 界之后复用最后一次。
func DimCloisterRewardEndAck(stage int) []byte {
	if stage < 0 {
		stage = 0
	}
	if stage >= len(dimCloisterRewardEndAckTails) {
		stage = len(dimCloisterRewardEndAckTails) - 1
	}
	p := make([]byte, 32)
	p[0] = 1
	p[5] = 0x66
	p[9] = byte(stage)
	copy(p[13:], dimCloisterRewardEndAckTails[stage][:])
	return p
}

// dimCloisterRewardEndAckTails 是官服 #829/#903/#986 在正文 @13..16 的实测值。
var dimCloisterRewardEndAckTails = [3][4]byte{
	{0x03, 0x0b, 0x04, 0xd7},
	{0x02, 0x65, 0x56, 0x4d},
	{0x01, 0xdd, 0x05, 0x01},
}

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

	// Failed records a CMD2044 LEGION_FAIL from the client (P5). It is a
	// record of what the client announced, not a server-side verdict: the
	// server has no way to judge a run while the phase drive is unimplemented
	// (X7), so it must not claim to have failed the player.
	Failed bool

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

// EnterDungeon records the accepted entry STAGE without changing difficulty.
// Production admission/validation is owned by partyrun, not this tracking row.
func (s *Session) EnterDungeon(stage uint32) {
	s.Phase = byte(stage)
	s.Entered = true
}

// SelectRole records a CMD2355 role change. The client only sends the packet
// when the value actually changed, so re-recording the same value is harmless.
func (s *Session) SelectRole(role uint32) { s.RoleValue = role }

// Fail records a CMD2044 LEGION_FAIL. The client sends this from its abandon
// path, so the session is marked failed and left otherwise intact: the run
// teardown is CMD2046's job, and keeping the two apart means a live log shows
// which of the two the client actually chose.
func (s *Session) Fail(argument uint32) {
	s.Argument = argument
	s.Failed = true
}

// EndReward records a CMD2046 LEGION_REWARD_END, the client's "the reward
// screen is done" signal. That is the end of the run, so the session returns to
// its pre-entry state; nothing is persisted either way (D3).
func (s *Session) EndReward() { s.Reset() }

// Reset drops the session back to its pre-entry state. It exists for the
// retreat path (P6) and for tests; with D3 there is nothing to persist.
func (s *Session) Reset() {
	s.Entered = false
	s.Operation = 0
	s.RoleValue = 0
	s.Phase = 0
	s.Failed = false
	s.Action = 0
	s.Auxiliary = 0
}
