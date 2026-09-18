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
}

func DungeonInfo(s DungeonInfoState) []byte {
	p := add32(nil, s.ID)
	p = append(p, s.Difficulty)
	p = add16(p, 0)
	// NOTI28 writes boss XY to dungeon+1934/+193c, consumed by the
	// native room predicate145b34090 and path gate14614de00. The current
	// room belongs only in NOTI29. The following XY is the random-hell
	// location (145b27520);255/255 is the native absent sentinel1452a94b2.
	p = append(p, s.Maze, s.Boss[0], s.Boss[1], 255, 255, 0, 0)
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
}
type StartMapState struct {
	ReuseRoom   bool
	LayerChange bool
	Transition  *[18]byte
	Position    [2]byte
	Seed, Map   uint32
	Monsters    []DungeonMonster
}

func StartMap(s StartMapState) ([]byte, error) {
	if s.Map == 0 || len(s.Monsters) > 255 {
		return nil, fmt.Errorf("invalid start map")
	}
	if s.ReuseRoom && (s.LayerChange || len(s.Monsters) != 0) {
		return nil, fmt.Errorf("cached room cannot initialize layers or monsters")
	}
	p := append([]byte{}, s.Position[0], s.Position[1], 0)
	if s.LayerChange {
		if s.Transition == nil {
			return nil, fmt.Errorf("layer transition record required")
		}
		p[2] = 1
	}
	p = add32(p, s.Seed)
	p = append(p, 0, 0)
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
		validRank := !m.APC && m.Rank <= 3 || m.APC && m.Rank >= 5 && m.Rank <= 8 && m.SourceIndex < 64
		if m.Entity == 0 || m.Entity == 65535 || seen[m.Entity] || m.Template == 0 || !validRank || m.Team > 0x7fffffff {
			return nil, fmt.Errorf("invalid monster identity")
		}
		seen[m.Entity] = true
		// 1452b7c3c -> 145b0d910: the SECOND u16 is MonsterUniqueId.
		// MonsterLevel is the first byte after the template u32. Confirmed
		// against the same constructor and named log at 1452b6492.
		p = add32(add16(p, 0), m.SourceIndex)
		p = add32(add16(p, m.Entity), m.Template)
		p = append(p, m.Level, m.Rank, 0, 0, 255)
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

type MonsterDeathReport struct {
	Entity uint32
	Killer uint16
}

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
	Position    [2]byte
	LayerChange bool
	Record      [18]byte
	Dungeon     uint32
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
