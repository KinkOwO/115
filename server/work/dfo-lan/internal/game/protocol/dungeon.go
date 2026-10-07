package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD15: normal town gate sends u32(0); the tutorial sender at146cce650
// uses a source dungeon ID instead. Slot1 pads the four-byte body to8.
func DecodeDungeonGate(p []byte) (uint32, error) {
	if len(p) != 8 {
		return 0, fmt.Errorf("dungeon gate requires four bytes padded to8")
	}
	if e := padding(p[4:], 8); e != nil {
		return 0, e
	}
	return binary.LittleEndian.Uint32(p), nil
}

// EnterDungeonSelection is NOTI27, handler145303260. This is the solo,
// non-relay path with no optional event/party collections. Scalar meanings
// beyond verified mode fields remain unnamed, rather than guessed IDs.
// The full 36-byte reader sequence is checked with the original native code.
func EnterDungeonSelection() []byte {
	p := []byte{0, 0, 0}      // first/relay/relay-extra, 1453032f6/307/7c4
	p = add32(add32(p, 0), 0) // 145303871/8ec
	p = append(p, 0)          // u16 collection, 145303ac3
	p = add32(p, 0)           // 145303b8f
	p = append(p, 0, 0, 0)    // u16 collections, 145303bc6/c38/d58
	p = add16(add16(p, 0), 0) // 145303f7b/fa2
	p = add32(p, 0)           // 145303fc7
	p = append(p, 0)          // mode flag, 145303ff4
	p = add32(p, 0)           // mode0, 145304033 (native clamps to0..1)
	p = add16(p, 0)           // 1453040b4
	return append(p, 0, 0)    // u32/u16 collections, 145304140/1b9
}

type DungeonSelection struct {
	ID         uint32
	Difficulty byte
	Extra      uint16
	Mode, Flag byte
	Party      uint16
	Reserved   uint32
	Tail       byte
	Quest      uint32
	Options    [2]byte
	Event      uint32
}

// DungeonDirectMove is CMD 2062 (ENUM_CMDPACKET_DUNGEON_DIRECT_MOVE): the
// "next story dungeon" gate the client offers next to "return to town" after a
// clear. Two 48-byte bodies were captured on 2026-09-21 (role test-jh, session
// roles_..._20260921_225338_199523_next37) when the player walked into that
// gate. Byte offsets, from the two captures:
//
//	+0  u32   207 / 334          (not named yet)
//	+4  u32   0
//	+8  u32   0xffffffff
//	+12 u8    0xff
//	+13 u32   dungeon id         100004984 / 100004947
//	+17 u8    difficulty         2, the same code the CMD 16 record carries at +4
//	+25 u8    1                  (not named yet, 1 in both captures)
//	+29 u32x4 gate rectangle     103,205,20,10 / 160,200,20,10
//	+45 3 zero bytes
//
// The dungeon id is written unaligned at +13, which the CMD 16 selection record
// also writes as a bare u32. The rectangle shares its 20x10 size in both
// captures and differs only in position, so it is the gate the client walked
// into; the remaining fields stay unnamed and the whole body is retained.
type DungeonDirectMove struct {
	ID         uint32
	Difficulty byte
	Gate       [4]uint32
	Record     [48]byte
}

func DecodeDungeonDirectMove(p []byte) (r DungeonDirectMove, e error) {
	if len(p) != 48 {
		return r, fmt.Errorf("direct move must contain 48 bytes")
	}
	copy(r.Record[:], p)
	r.ID = binary.LittleEndian.Uint32(p[13:])
	r.Difficulty = p[17]
	for i := range r.Gate {
		r.Gate[i] = binary.LittleEndian.Uint32(p[29+4*i:])
	}
	return r, nil
}

// 146d4a148..4d2: u32,u8,u16,u8,u8,u16,u32,u8,u32,u8,u8,u32.
func DecodeDungeonSelection(p []byte) (r DungeonSelection, e error) {
	if len(p) != 32 {
		return r, fmt.Errorf("select dungeon must contain26 bytes padded to32")
	}
	if e = padding(p[26:], 16); e != nil {
		return r, e
	}
	r.ID = binary.LittleEndian.Uint32(p)
	r.Difficulty = p[4]
	r.Extra = binary.LittleEndian.Uint16(p[5:])
	r.Mode = p[7]
	r.Flag = p[8]
	r.Party = binary.LittleEndian.Uint16(p[9:])
	r.Reserved = binary.LittleEndian.Uint32(p[11:])
	r.Tail = p[15]
	r.Quest = binary.LittleEndian.Uint32(p[16:])
	copy(r.Options[:], p[20:22])
	r.Event = binary.LittleEndian.Uint32(p[22:])
	return r, nil
}

type DungeonInfoState struct {
	ID               uint32
	Difficulty, Maze byte
	Boss             [2]byte
	Hell             *[2]byte
}

func DungeonInfo(s DungeonInfoState) []byte {
	p := add32(nil, s.ID)
	p = append(p, s.Difficulty)
	p = add16(p, 0)
	// NOTI28 writes boss XY to dungeon+1934/+193c, consumed by the
	// native room predicate145b34090 and path gate14614de00. The current
	// room belongs only in NOTI29. The following XY is the random-hell
	// location (145b27520);255/255 is the native absent sentinel1452a94b2.
	hell := [2]byte{255, 255}
	if s.Hell != nil {
		hell = *s.Hell
	}
	p = append(p, s.Maze, s.Boss[0], s.Boss[1], hell[0], hell[1], 0, 0)
	p = add16(add16(p, 0), 0)
	p = append(p, 0)
	p = add32(p, 0xffffffff)
	p = append(p, 0, 0, 0, 0, 0, 0, 0, 0)
	p = add16(add16(add16(p, 0), 0), 0)
	return add32(p, 0)
}

type DungeonMonster struct {
	Entity      uint16
	SourceIndex uint32
	Level       byte
	Template    uint32
	Rank        byte     // Original map parser1471e92ac..9368: normal0, champion1, super2, boss3.
	Team        uint32   // Current actor affiliation: source team0 friendly, default100 enemy.
	NonCombat   bool     // Source team0/dummy; still spawned for native cinematic scripts.
	SourceTail  [2]int32 // Map row fields6/7, retained separately from spawn count.
	APC         bool     // Fixed map AIC; uses ranks5..8 and its own source row index.
	// CreateTrigger 是该行在 map 里的 [monster create trigger] 原值。
	// 它是否会写进进图包由 StartMapState.EncodeCreateTrigger 决定；
	// 该字段的语义尚未确认，所以默认不编码。
	CreateTrigger byte
	// NOTI29 +0 order and +15 hidden are consumed by native145b26960:
	// hidden rows with selector -1 queue for the pillar's matching wave.
	SpawnOrder uint16
	Hidden     bool
}
type StartMapState struct {
	ReuseRoom   bool
	LayerChange bool
	// ExitLayer clears the native layer ordinal before selecting the base cache
	// (1452b787f -> 1452b7876). Ordinary flag 0 leaves that ordinal unchanged.
	ExitLayer     bool
	Transition    *[18]byte
	Position      [2]byte
	Seed, Map     uint32
	Monsters      []DungeonMonster
	HellPartyMode byte // header +7; approved A=1/B=2 compatibility mapping
	// EncodeCreateTrigger 置位时，怪物记录里 Rank 之后那一格（原本恒为 0）
	// 改写实例的 CreateTrigger。默认 false 时输出与原先逐字节一致。
	EncodeCreateTrigger bool
}

func StartMap(s StartMapState) ([]byte, error) {
	if s.Map == 0 || len(s.Monsters) > 255 {
		return nil, fmt.Errorf("invalid start map")
	}
	if s.ReuseRoom && len(s.Monsters) != 0 {
		return nil, fmt.Errorf("cached room cannot initialize monsters")
	}
	if s.LayerChange && s.ExitLayer {
		return nil, fmt.Errorf("conflicting layer transition modes")
	}
	p := append([]byte{}, s.Position[0], s.Position[1], 0)
	if s.LayerChange || s.ExitLayer {
		if s.Transition == nil {
			return nil, fmt.Errorf("layer transition record required")
		}
		p[2] = 1
		if s.ExitLayer {
			p[2] = 2
		}
	}
	p = add32(p, s.Seed)
	p = append(p, s.HellPartyMode, 0)
	p = add32(p, 0xffffffff)
	// Native transition record defaults at1452b7494..4a7, consumed18 bytes.
	p = append(p, 0, 0, 0, 0, 255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0)
	if s.Transition != nil {
		copy(p[13:31], s.Transition[:])
	}
	// Native1452b78f0 skips map/spawn rows for mode0. The constructor at
	// 145b235b0 then retains the cached map and its one-shot ACT triggers.
	if s.ReuseRoom {
		return append(p, 0, 0, 255), nil // mode0, no groups, no other-party actor
	}
	p = append(p, 1)
	p = add32(p, s.Map)
	p = append(p, byte(len(s.Monsters)))
	seen := map[uint16]bool{}
	for _, m := range s.Monsters {
		// Fixed APCs address the current map's [ai character] table with
		// SourceIndex 0..63. Native145b20dc0 has a separate, explicit 10000
		// branch for an APC that has no row in the current map: it creates the
		// AIC by Template, applies the packet Team and enables following. This
		// is the retail path for carrying a story companion into the next room;
		// reusing its old map-table index there dereferences the wrong table.
		validAPCSource := m.SourceIndex < 64 || m.SourceIndex == 10000
		validRank := !m.APC && m.Rank <= 3 || m.APC && m.Rank >= 5 && m.Rank <= 8 && validAPCSource
		if m.Entity == 0 || m.Entity == 65535 || seen[m.Entity] || m.Template == 0 || !validRank || m.Team > 0x7fffffff {
			return nil, fmt.Errorf("invalid monster identity")
		}
		if m.Hidden && (m.SpawnOrder < 1 || m.SpawnOrder > 9 || m.NonCombat || m.APC && m.SourceIndex != 10000) || !m.Hidden && m.SpawnOrder != 0 {
			return nil, fmt.Errorf("invalid hidden wave identity")
		}
		seen[m.Entity] = true
		// 1452b7c3c -> 145b0d910: the SECOND u16 is MonsterUniqueId.
		// MonsterLevel is the first byte after the template u32. Confirmed
		// against the same constructor and named log at 1452b6492.
		p = add32(add16(p, m.SpawnOrder), m.SourceIndex)
		p = add32(add16(p, m.Entity), m.Template)
		createTrigger := byte(0)
		if s.EncodeCreateTrigger {
			createTrigger = m.CreateTrigger
		}
		hidden := byte(0)
		if m.Hidden {
			hidden = 1
		}
		p = append(p, m.Level, m.Rank, createTrigger, hidden, 255)
		// 145b0d910 record+40 ->145b219d0 ->145b144f0 ->vtable+a40
		// ->145dd6080 ->145d87580 writes actor+f10 (team), not HP.
		p = add32(p, m.Team)
		p = append(p, 0)
	}
	return append(p, 0, 0, 0, 255), nil // no extra APC/group rows; no other-party actor
}
func DungeonLoaded() []byte { return []byte{0, 0, 0, 0, 0} }

// NOTI132, native1452aee40, normal solo return-to-town branch.
func DungeonSelectionReturn() []byte { return []byte{0} }

// NOTI 2193 (0x0891, ENUM_NOTIPACKET_ELVENMERE_INFO), native 142223F70 -> 14222B360.
// Payload is exactly 203 bytes:
//
//	Byte 0: status/version (0 or 1)
//	Byte 1: max cleared floor (1..100, 0 if none)
//	Byte 2: current floor (1..100)
//	Bytes 3..102 (100 bytes): weekly clear/reward status for floors 1..100
//	Bytes 103..202 (100 bytes): season clear/reward status for floors 1..100
func ElvenmereInfo(currentFloor, maxCleared byte, weeklyRewards, seasonRewards [100]byte) []byte {
	p := make([]byte, 203)
	p[0] = 0 // mode/status
	p[1] = maxCleared
	p[2] = currentFloor
	copy(p[3:103], weeklyRewards[:])
	copy(p[103:203], seasonRewards[:])
	return p
}

type MonsterDeathReport struct {
	Entity uint32
	Killer uint16
}

// EplpRechallenge is the NOTI 261 body (ENUM_NOTIPACKET_EPLP_RECHALLENGE -
// the opcode name comes from analysis/dumps/opcodes.tsv). The client reads a
// single byte into the settlement panel's retry state: 9 lights the
// "continue challenge" entry and the right-edge arrow, 1 leaves it greyed.
//
// It is sent after NOTI 35 because the panel is built from that reward; a
// patch pushed before it has nothing to attach to. Only 9/1 are ever sent -
// do not pass arbitrary values, the client switches on them.
//
// Origin: the outside repair document describes the same single byte and the
// same two values. Our own evidence for the opcode name is the dump above;
// the two values still need live confirmation on this client build.
const (
	// EplpRechallengeReady enables the retry entry (another run is allowed).
	EplpRechallengeReady byte = 9
	// EplpRechallengeBlocked leaves it greyed out (fatigue or entry refused).
	EplpRechallengeBlocked byte = 1
)

func EplpRechallenge(state byte) []byte { return []byte{state} }

func DecodeMonsterDeath(p []byte) (MonsterDeathReport, error) {
	// Native145dc9bf4..145dca2af. The following combat/check fields are opaque;
	// do not interpret them as HP, damage, item IDs or authoritative rewards.
	if len(p) < 64 || len(p) > 4096 || len(p)%8 != 0 {
		return MonsterDeathReport{}, fmt.Errorf("invalid monster death envelope")
	}
	if 31+int(p[30])*14+7 > len(p) {
		return MonsterDeathReport{}, fmt.Errorf("truncated death participant list")
	}
	return MonsterDeathReport{binary.LittleEndian.Uint32(p), binary.LittleEndian.Uint16(p[4:])}, nil
}

// The no-item NOTI38 branch is independently consumed by1452bb930.
// Drops are a separate generated outcome; this response does not grant any.
func MonsterDeathConfirmed(entity uint16) []byte {
	return append(add16(add16(nil, entity), 0), 0, 0, 255, 0)
}

// ispinsDeathTokens 是官服 s4 四个阶段的 N38 尾 5B token（帧 468/565/647/749，
// next79 §26）。语义未解（推测 per-stage nonce），逐帧回放。
var ispinsDeathTokens = [4][5]byte{
	{0x58, 0x49, 0xff, 0x0b, 0x3e}, // stage0
	{0xb4, 0x4e, 0xb2, 0x02, 0x44}, // stage1
	{0x12, 0x14, 0x19, 0x62, 0x42}, // stage2
	{0xb1, 0x5b, 0xb2, 0xb1, 0x39}, // stage3
}

// IspinsMonsterDeathConfirmed builds the 16B NOTI38 the official server sends
// for an Ispins boss death (next79 §24, s4 frame 468). Body layout:
// `<u32 entity> <4B zero> <5B token> <3B zero>`. The private server's
// generic MonsterDeathConfirmed (200B drops form) is rejected in legion
// context — the client expects the 16B echo form and crashes (op=682)
// otherwise. The 5B token is the per-stage nonce observed on the official
// N38; stage0 value replayed verbatim.
func IspinsMonsterDeathConfirmed(entity uint32, stage int) []byte {
	if stage < 0 || stage > 3 {
		stage = 0
	}
	p := make([]byte, 16)
	binary.LittleEndian.PutUint32(p, entity)
	copy(p[8:13], ispinsDeathTokens[stage][:])
	return p
}

func DecodeMoveDungeonRoom(p []byte) ([2]byte, error) {
	// Native144d35022..144d35507 writes155 bytes: target XY, actor XY,
	// transition flag, eight participant check/position rows, transition18,
	// state byte and final u32. Client check fields remain opaque.
	if len(p) != 160 {
		return [2]byte{}, fmt.Errorf("room move must contain155 bytes padded to160")
	}
	if e := padding(p[155:], 16); e != nil {
		return [2]byte{}, e
	}
	if p[10] > 1 {
		return [2]byte{}, fmt.Errorf("invalid room transition flag")
	}
	return [2]byte{p[0], p[1]}, nil
}

type DungeonRoomTransition struct {
	// Server-only authorization for a source Bakal arena-to-entry teleport.
	RaidReturn  bool
	Position    [2]byte
	LayerChange bool
	Record      [18]byte
	Dungeon     uint32
	// SceneExit 只由服务端设置：标记这次 layer 切换是「场景房点门」的出口，
	// 方向是回该位置的 base。客户端的 CMD45 永远是 SceneExit=false（前进）。
	SceneExit bool
}

func DecodeDungeonRoomTransition(p []byte) (r DungeonRoomTransition, err error) {
	r.Position, err = DecodeMoveDungeonRoom(p)
	if err != nil {
		return r, err
	}
	if len(p) >= 11 {
		r.LayerChange = p[10] == 1
	}
	if len(p) >= 150 {
		copy(r.Record[:], p[132:150])
	}
	if len(p) >= 155 {
		r.Dungeon = binary.LittleEndian.Uint32(p[151:155])
	}
	return r, nil
}
