// Ispins (伊斯大陆, content id 101) wire contract.
//
// Every byte below is transcribed from the official capture
// analysis/tasks/next78-ispins-official-capture-p0.md and the underlying
// session_s4 streams (official 115 client, 2026-10-02). Fields whose
// semantics are not closed are replayed verbatim from the official frames
// and marked as such; do not reinterpret them without new evidence.
package legion

import (
	"encoding/binary"
	"fmt"
)

// IspinsContentID is the u32 carried at body offset 13 of the CMD2043/2045/
// 2046 requests (0x65 = 101). The apocalypse family carries 107 on the same
// offsets, so this value is the family discriminator.
const IspinsContentID uint32 = 101

// Ispins family ids that do not collide with the apocalypse family.
const (
	// CmdIspinsOperationSelect is CMD2047, absent from the apocalypse flow.
	// Variant A (b13=01) picks an operation, variant B (b13=02) confirms it.
	CmdIspinsOperationSelect uint16 = 2047
)

// NOTI ids shared with the apocalypse family by number only; the bodies are
// Ispins-specific (next78 §0: the 2895/2896/2657/2568 packets never appear).
const (
	NotiIspinsBasicClearReward      uint16 = 2252 // native fixed 7772B, next79 §29
	NotiIspinsAdditionalClearReward uint16 = 2253 // native fixed 2405B, next79 §29
	NotiIspinsEntryCharacterInfo    uint16 = 2254 // 272B, next78 §2.2
	NotiIspinsInfo                  uint16 = 2255 // 88B state machine, next78 §2.1
	NotiIspinsOperation             uint16 = 2256 // native3B: selected i16 index + u8 result
)

// Historical capture IDs for compatibility tests; runtime obtains IDs from
// the prepared LegionContents. Stage order is the
// client's selection; the run recorded there used the default order, whose
// dungeon ids map to these PVF dungeon_info_data indices.
// 100002985 ispins_ashcore, 100002986 ispins_itrenog,
// 100002987 ispins_nemaug, 100002988 ispins_nagor.
var IspinsStageDungeons = [4]uint32{100002987, 100002988, 100002985, 100002986}

// IspinsStageDungeonIndex is the dungeon_info_data position of each stage's
// dungeon in the recorded default order (nemaug, nagor, ashcore, itrenog).
var IspinsStageDungeonIndex = [4]byte{2, 3, 0, 1}

// Historical N31 head scalars. The matching old N2252 tail was elapsed
// milliseconds, not an opaque stage token. Runtime N2252 uses the owned clock;
// N31's nonzero branch remains the accepted capture projection.
var IspinsStageTokens = [4][2]byte{{0x93, 0x8c}, {0xa7, 0x69}, {0x75, 0x77}, {0xac, 0x87}}

// Per-stage 5B nonces observed on N31; semantics unclosed (next78 §5.1),
// replayed verbatim.
var ispinsStageNonces = [4][5]byte{
	{0x9f, 0xcb, 0x42, 0x06, 0x45},
	{0x47, 0xd5, 0x4a, 0xa6, 0x35},
	{0x91, 0xb9, 0x89, 0xd4, 0x35},
	{0x8d, 0x1d, 0xf7, 0xdc, 0x33},
}

// IspinsRequests reports whether the id belongs to the Ispins command family
// beyond the shared 2043/2045/2046 envelope.
func IspinsRequests(id uint16) bool { return id == CmdIspinsOperationSelect }

// DecodeIspinsStart reads the CMD2043 request sent on an Ispins channel.
// Current writer1424fe550 emits17 logical bytes; capture includes cipher padding.
func DecodeIspinsStart(p []byte) (StartRequest, error) {
	if len(p) < EnvelopeSize+4 {
		return StartRequest{}, fmt.Errorf("ispins start payload %d bytes, want at least %d", len(p), EnvelopeSize+4)
	}
	if content := binary.LittleEndian.Uint32(p[EnvelopeSize:]); content != IspinsContentID {
		return StartRequest{}, fmt.Errorf("ispins start content %d, want %d", content, IspinsContentID)
	}
	return StartRequest{Argument: IspinsContentID, BodyLength: len(p)}, nil
}

// IspinsOperationRequest is the decoded CMD2047 body. Variant A carries a
// u32 token at 0 and 0xffff at 17; variant B carries a counter at 8 and an
// auxiliary byte at 17 (next78 §2.5).
type IspinsOperationRequest struct {
	Variant        byte
	Token          uint32
	Counter        uint32
	Auxiliary      byte
	OperationIndex uint16
	BodyLength     int
}

// DecodeIspinsOperationSelect reads the CMD2047 request (32B official).
func DecodeIspinsOperationSelect(p []byte) (IspinsOperationRequest, error) {
	if len(p) < EnvelopeSize+6 {
		return IspinsOperationRequest{}, fmt.Errorf("ispins operation payload %d bytes, want at least %d", len(p), EnvelopeSize+6)
	}
	out := IspinsOperationRequest{BodyLength: len(p)}
	switch p[EnvelopeSize] {
	case 1:
		out.Variant = 1
		out.Token = binary.LittleEndian.Uint32(p)
		if p[17] != 0xff || p[18] != 0xff {
			return IspinsOperationRequest{}, fmt.Errorf("ispins operation A missing ffff marker")
		}
	case 2:
		out.Variant = 2
		out.Counter = binary.LittleEndian.Uint32(p[8:])
		out.Auxiliary = p[17]
		out.OperationIndex = binary.LittleEndian.Uint16(p[17:])
	case 4:
		// Local native sender 1425311f0 emits action u32 at13, arg u16
		// at17; live change-operation request uses action4 and argffff.
		out.Variant = 4
		if binary.LittleEndian.Uint32(p[EnvelopeSize:]) != 4 || p[17] != 0xff || p[18] != 0xff {
			return IspinsOperationRequest{}, fmt.Errorf("ispins operation reset missing action4/ffff marker")
		}
	default:
		return IspinsOperationRequest{}, fmt.Errorf("ispins operation variant %d unknown", p[EnvelopeSize])
	}
	return out, nil
}

// DecodeIspinsEnter reads the CMD2045 request (24B official): content u32 at
// 13, stage u32 at 17 (0-based). The stage selects which of the four stage
// dungeons to load.
func DecodeIspinsEnter(p []byte) (EnterDungeonRequest, error) {
	if len(p) < EnvelopeSize+8 {
		return EnterDungeonRequest{}, fmt.Errorf("ispins enter payload %d bytes, want at least %d", len(p), EnvelopeSize+8)
	}
	req := EnterDungeonRequest{
		Channel:    binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Stage:      binary.LittleEndian.Uint32(p[EnvelopeSize+4:]),
		BodyLength: len(p),
	}
	if req.Channel != IspinsContentID {
		return EnterDungeonRequest{}, fmt.Errorf("ispins enter content %d, want %d", req.Channel, IspinsContentID)
	}
	return req, nil
}

// DecodeIspinsRewardEnd reads content u32@13, phase ordinal u32@17 and
// source dungeon index u8@21. Native writer1424fe4a0 and caller1425287d5
// read the latter from the owned N2255 dungeon record, not the next phase.
func DecodeIspinsRewardEnd(p []byte) (EnterDungeonRequest, uint32, error) {
	if len(p) < EnvelopeSize+9 {
		return EnterDungeonRequest{}, 0, fmt.Errorf("ispins reward-end payload %d bytes, want at least %d", len(p), EnvelopeSize+9)
	}
	req := EnterDungeonRequest{
		Channel:    binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Stage:      binary.LittleEndian.Uint32(p[EnvelopeSize+4:]),
		BodyLength: len(p),
	}
	if req.Channel != IspinsContentID {
		return EnterDungeonRequest{}, 0, fmt.Errorf("ispins reward-end content %d, want %d", req.Channel, IspinsContentID)
	}
	return req, uint32(p[EnvelopeSize+8]), nil
}

// IspinsStartAck builds the 16B ACK2043 observed at f377:
// 01 00000000 <5B token> 00000000000000.
func IspinsStartAck(nonce [5]byte) []byte {
	p := make([]byte, 16)
	p[0] = 1
	copy(p[5:], nonce[:])
	return p
}

// IspinsOperationAckA builds the 32B variant-A ACK2047 observed at f431:
// 01 01 000000 00 ffff 00000000 <LE unix ts> 00 f0 0000 <token> 00*7.
func IspinsOperationAckA(unixSeconds uint32, nonce [5]byte) []byte {
	p := make([]byte, 32)
	p[0], p[1] = 1, 1
	p[5], p[6] = 0xff, 0xff
	binary.LittleEndian.PutUint32(p[12:], unixSeconds)
	p[17] = 0xf0
	copy(p[20:], nonce[:])
	return p
}

// IspinsOperationAckB builds the 32B variant-B ACK2047 observed at f433:
// 01 02 000000 00 <aux> ... 00 f0 0000 <token> 00*7.
func IspinsOperationAckB(aux byte, nonce [5]byte) []byte {
	p := make([]byte, 32)
	p[0], p[1] = 1, 2
	p[5] = aux
	p[17] = 0xf0
	copy(p[20:], nonce[:])
	return p
}

// IspinsOperationResetAck is success + the local handler's 19B struct:
// action u32=4, argument u16=ffff, selection flag0, existing timestamp slot.
// 142530a1e..a58 resets window644; the flag0 path reopens selection.
func IspinsOperationResetAck(timestamp uint32) []byte {
	p := IspinsOperationAckA(timestamp, [5]byte{})
	p[1] = 4
	return p[:20]
}

// IspinsEnterAck builds the 24B ACK2045: 01 00000000 65 000000 <stage>
// 00000000 <token> 000000 (f439-456 burst tail, next78 §2.5).
func IspinsEnterAck(stage byte, nonce [5]byte) []byte {
	p := make([]byte, 24)
	p[0] = 1
	p[5] = 0x65
	p[9] = stage
	copy(p[14:], nonce[:])
	return p
}

// IspinsRewardEndAck builds the 32B ACK2046: 01 00000000 65 000000 <stage>
// 0000 <source boss index> <token> ... It echoes the owned C2046 boss index.
// The captured order made that index (stage+2)%4; it is not a phase formula.
func IspinsRewardEndAck(stage, sourceIndex byte, nonce [5]byte) []byte {
	p := make([]byte, 32)
	p[0] = 1
	p[5] = 0x65
	p[9] = stage
	p[13] = sourceIndex
	copy(p[14:], nonce[:])
	return p
}

// IspinsReviveAck replays the 16B ACK2059 observed on the official run
// (constant 94f5c28b3c across all eight frames, next78 §2.5).
func IspinsReviveAck() []byte {
	p := make([]byte, 16)
	p[0] = 1
	p[6] = 0xed
	copy(p[8:], []byte{0x94, 0xf5, 0xc2, 0x8b, 0x3c})
	return p
}

// N2255 state machine templates. The 88-byte payloads are transcribed
// verbatim from the official frames (see ispins_vectors.generated.go); the
// only locally rewritten bytes are the 5B nonce at 79..83. Field map (next78
// §2.1): @2 place (ff hall / 02 dungeon), @3 state (01 init, 06 waiting, 02
// chosen, 03 final, 05 leave), @7 outcome (03 stage cleared), @11 stages
// completed, stage records at 19+8k (dungeon index, status 0 current /
// 1 cleared / 2 locked).

// IspinsInfoPayload returns the N2255 body for the named state. The nonce
// replaces bytes 79..83 of the official template; pass the zero value to
// replay the captured bytes.
func IspinsInfoPayload(state string, nonce [5]byte) ([]byte, error) {
	text, ok := ispinsInfoTemplates[state]
	if !ok {
		return nil, fmt.Errorf("ispins info state %q unknown", state)
	}
	p, err := decodeHex(text)
	if err != nil || len(p) != 88 {
		return nil, fmt.Errorf("ispins info template %q invalid (%d bytes)", state, len(p))
	}
	if nonce != ([5]byte{}) {
		copy(p[79:], nonce[:])
	}
	return p, nil
}

// IspinsEntryCharacterInfo builds the 272B N2254. The login frame replays
// f257 (entries 101/102/105 flag 7f — semantics unclosed, next78 §2.2); the
// in-run frame replays the f374 base with the stage flags set as the run
// progresses. The nonce replaces the 5 bytes at 256..260.
func IspinsEntryCharacterInfo(login bool, stageFlags [4]bool, nonce [5]byte) ([]byte, error) {
	text := ispinsLoginEntryCharacterInfoTemplate
	if !login {
		text = ispinsRunEntryCharacterInfoTemplate
	}
	p, err := decodeHex(text)
	if err != nil || len(p) != 272 {
		return nil, fmt.Errorf("ispins entry character info template invalid (%d bytes)", len(p))
	}
	if !login {
		for i, set := range stageFlags {
			if set {
				p[2+2*i] = 1
			}
		}
		// [ISPINS-ARENA-BOSS] 阶段结算 N2254 标记位对齐官服 s4（next79 §24/§28）：
		// §24 曾按 idx21/53/85=0x7f、idx117=0x39 写入，§28 对九测整链逐字节
		// 对照官服四阶段结算帧（490/585/668/777）发现真实形态四阶段完全一致：
		// idx21/69=0x7f、idx93=0x39、idx94=0x32、idx117=00（53/85 保持 00）。
		// §19 已实证客户端主动读这些旗标做 UI 门禁，错形阻断结算面板（1654
		// 不发→op=2060→682 闪退）。基线（无阶段结算，帧 374）与待机/登录帧
		// 保持原模板行为，只在有阶段已结算时覆盖标记位。
		anyCleared := false
		for _, set := range stageFlags {
			if set {
				anyCleared = true
				break
			}
		}
		if anyCleared {
			p[21], p[69] = 0x7f, 0x7f
			p[93], p[94] = 0x39, 0x32
		}
	}
	if nonce != ([5]byte{}) {
		copy(p[256:], nonce[:])
	}
	return p, nil
}

// IspinsStandbyEntryCharacterInfo builds the 272B N2254 the official server
// sends once the standby (channel-switch) entry completes. The five head
// u16 slots at offsets 0/2/4/6/8 are the weekly Ispins stage marks: the
// official s4 capture (2026-10-02, the only standby session that then
// allowed team creation and a full battle) carries them ALL ZERO at the
// standby announce (frame 257, same shape as the login frame with 21/117
// = 0x7f). The switch capture (2026-10-03, week already exhausted) sent
// them all SET (frames 331/997), and replaying that shape makes the client
// panel show "weekly entry 2/3, weekly reward 0/1" and locally block team
// creation (next79 §19). So the standby body must use the all-zero flag
// form — byte-identical to the login body apart from the nonce.
func IspinsStandbyEntryCharacterInfo(nonce [5]byte) ([]byte, error) {
	p, err := IspinsEntryCharacterInfo(false, [4]bool{}, nonce)
	if err != nil {
		return nil, err
	}
	// run 基底自带 offset 0 的「已入场」标记，五个 u16 全部清零才等于
	// 官服 s4 帧 257 的待机形态。
	copy(p[:10], make([]byte, 10))
	p[21], p[117] = 0x7f, 0x7f
	return p, nil
}

// Captured exhausted standby (next79§19, frames331/997): five u16 head marks set.
func IspinsSpentStandbyEntryCharacterInfo() ([]byte, error) {
	p, e := IspinsStandbyEntryCharacterInfo([5]byte{})
	if e != nil {
		return nil, e
	}
	for off := 0; off < 10; off += 2 {
		binary.LittleEndian.PutUint16(p[off:], 1)
	}
	return p, nil
}

// Current NOTI2256 registration1425314b1 -> reader142530eb0 copies three
// bytes: signed operation index and result. IspinsManager::setOperResult
// 14252c460 compares that index to the selected operation, then stores
// result != 1 as its failure flag. Transport padding is not gameplay data.
func IspinsOperationNotice(operation uint16, success bool) ([]byte, error) {
	if operation == 0 || operation > 0x7fff {
		return nil, fmt.Errorf("invalid Ispins operation result index %d", operation)
	}
	p := make([]byte, 3)
	binary.LittleEndian.PutUint16(p, operation)
	if success {
		p[2] = 1
	}
	return p, nil
}

// IspinsDungeonClearEnabled builds the 16B N31 with the stage token that the
// N2252 tail must echo (next78 §1.4).
func IspinsDungeonClearEnabled(stage int) []byte {
	if stage < 0 || stage > 3 {
		return nil
	}
	p := make([]byte, 16)
	copy(p[0:2], IspinsStageTokens[stage][:])
	copy(p[4:], ispinsStageNonces[stage][:])
	return p
}

// IspinsStoryPause builds the 16B N170 observed at f787/f793; the second
// byte echoes the CMD191 pause byte (00 pause / 01 resume).
func IspinsStoryPause(resumed bool, nonce [5]byte) []byte {
	p := make([]byte, 16)
	p[0] = 0xed
	if resumed {
		p[2] = 1
	}
	p[3] = 1
	copy(p[4:], nonce[:])
	return p
}

// IspinsReplayFrame is one verbatim frame from the official capture's enter
// or confirm burst (ispins_replay_frames.generated.go).
type IspinsReplayFrame struct {
	Name string
	ID   uint16
	Body []byte
}

// IspinsReplayFrames returns the named official replay frames in order. A
// missing name is an error: a replay sequence must never silently drop a
// frame.
func IspinsReplayFrames(names ...string) ([]IspinsReplayFrame, error) {
	out := make([]IspinsReplayFrame, 0, len(names))
	for _, name := range names {
		var found bool
		for _, f := range ispinsReplayFrames {
			if f.Name != name {
				continue
			}
			body, err := decodeHex(f.Body)
			if err != nil || len(body) == 0 {
				return nil, fmt.Errorf("ispins replay frame %q invalid", name)
			}
			out = append(out, IspinsReplayFrame{Name: f.Name, ID: f.ID, Body: body})
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("ispins replay frame %q missing from capture table", name)
		}
	}
	return out, nil
}

func decodeHex(text string) ([]byte, error) {
	clean := make([]byte, 0, len(text)/2)
	for i := 0; i+1 < len(text); i += 2 {
		hi, ok1 := hexNibble(text[i])
		lo, ok2 := hexNibble(text[i+1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("invalid hex byte %q", text[i:i+2])
		}
		clean = append(clean, hi<<4|lo)
	}
	return clean, nil
}

func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
