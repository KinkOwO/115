package protocol

import (
	"encoding/binary"
	"fmt"
)

// Native 14254b9e0 / 141d516b0 write 13 opaque bytes, two i32
// maze coordinates and the 18-byte StartMap placement record. C2070 is
// the scripted intra-dungeon teleport, distinct from an adjacent C45 door.
func DecodeBakalRoomWarp115(p []byte) ([2]byte, [18]byte, error) {
	var grid [2]byte
	var record [18]byte
	if len(p) < 39 || len(p) > 48 {
		return grid, record, fmt.Errorf("invalid native Bakal room warp length")
	}
	for i := range grid {
		v := int32(binary.LittleEndian.Uint32(p[13+i*4 : 17+i*4]))
		if v < 0 || v > 255 {
			return grid, record, fmt.Errorf("invalid Bakal maze coordinate")
		}
		grid[i] = byte(v)
	}
	copy(record[:], p[21:39])
	if record[0] != 1 || record[1] != 0 || record[2] != 0 || record[3] != 0 {
		return grid, record, fmt.Errorf("invalid Bakal placement record")
	}
	for _, b := range p[39:] {
		if b != 0 {
			return grid, record, fmt.Errorf("Bakal room warp padding")
		}
	}
	return grid, record, nil
}

// NOTI570 registered at144cd51e1, reader144ce7960: u8 count then
// (symbol i32, value i32). Setter144cfb8e0 updates the persistent +d8 map
// read by ACT CHECK RAID SYMBOL. NOTI583 instead updates timed +f8 entries.
func RaidGlobalSymbol115(symbol uint32, value int32) ([]byte, error) {
	if symbol == 0 {
		return nil, fmt.Errorf("invalid raid symbol")
	}
	return add32(add32([]byte{1}, symbol), uint32(value)), nil
}

// START_RAID sender144CECAE2 writes no fields. Body cipher padding is
// retained by the dispatcher; accept only empty/zero aligned representations.
func DecodeRaidStartRequest(p []byte) error {
	if len(p) != 0 && len(p) != 8 && len(p) != 16 {
		return fmt.Errorf("invalid raid start body length")
	}
	for _, b := range p {
		if b != 0 {
			return fmt.Errorf("unexpected raid start fields")
		}
	}
	return nil
}

// Native 140d70db0 computes actor HP/maxHP *10000 and resolves the current
// battlefield slot with142546a00. 14254ba90 writes flag/slot/HP after13B.
func DecodeBakalHealthReport115(p []byte) (uint32, int32, error) {
	if len(p) < 22 || len(p) > 32 || p[13] > 1 {
		return 0, 0, fmt.Errorf("invalid native Bakal health report")
	}
	for _, b := range p[22:] {
		if b != 0 {
			return 0, 0, fmt.Errorf("Bakal report padding")
		}
	}
	return binary.LittleEndian.Uint32(p[14:18]), int32(binary.LittleEndian.Uint32(p[18:22])), nil
}

// Native142542b30 chooses a buff kind (25 means none), then14254c960
// writes that kind after13B. This command is buff use, not boss damage.
func DecodeBakalBuffUse115(p []byte) (int32, error) {
	if len(p) < 17 || len(p) > 24 {
		return 0, fmt.Errorf("invalid native Bakal buff use")
	}
	for _, b := range p[17:] {
		if b != 0 {
			return 0, fmt.Errorf("Bakal buff padding")
		}
	}
	return int32(binary.LittleEndian.Uint32(p[13:17])), nil
}

// 142548400 converts battlefield camps52/53/54 to town areas2/3/4,
// then writes C2074 as13 opaque bytes followed by the area u32.
func DecodeBakalCampReturn115(p []byte) (uint32, error) {
	area, err := DecodeBakalBuffUse115(p)
	if err != nil || area < 2 || area > 4 {
		return 0, fmt.Errorf("invalid native Bakal camp return")
	}
	return uint32(area), nil
}

// Native string conversion 147714120 maps these names to packet enum values.
// Template IDs from bakalmonster.cos are separate and must never be sent here.
func BakalMonsterType(kind string) (uint32, error) {
	for i, name := range []string{"bakal", "sparazzi", "skasa", "hisma", "basilisk", "blona", "gerda", "nympha", "swan", "steich", "eclair", "brute", "steel dragon", "zamir", "routund", "normal monsters"} {
		if kind == name {
			return uint32(i + 1), nil
		}
	}
	return 0, fmt.Errorf("unknown native Bakal monster type %q", kind)
}

type BakalPartyBuff struct {
	Kind, ExpiresAt uint32
}

type BakalPartyInfo struct {
	Party, Location, State uint32
	Buffs                  [20]BakalPartyBuff
}

// NOTI2285 reader 144CDE4D0 reads a u8 count and fixed 172-byte records.
// Consumer 1425434E0 reads three u32s followed by twenty (kind, expiry) pairs.
// Unused buffs must use kind25, not zero (zero is a real buff).
func BakalPartyInfoPayload(parties []BakalPartyInfo) ([]byte, error) {
	if len(parties) == 0 || len(parties) > 3 {
		return nil, fmt.Errorf("invalid native Bakal party count")
	}
	b := make([]byte, 1+172*len(parties))
	b[0] = byte(len(parties))
	seen := map[uint32]bool{}
	for i, p := range parties {
		if p.Party < 1 || p.Party > 3 || seen[p.Party] || p.Location < 1 || p.Location > 54 && p.Location != 56 {
			return nil, fmt.Errorf("invalid or duplicate native Bakal party")
		}
		seen[p.Party] = true
		row := b[1+172*i:]
		binary.LittleEndian.PutUint32(row, p.Party)
		binary.LittleEndian.PutUint32(row[4:], p.Location)
		binary.LittleEndian.PutUint32(row[8:], p.State)
		for j, buff := range p.Buffs {
			if buff.Kind > 25 {
				return nil, fmt.Errorf("invalid native Bakal party buff")
			}
			binary.LittleEndian.PutUint32(row[12+8*j:], buff.Kind)
			binary.LittleEndian.PutUint32(row[16+8*j:], buff.ExpiresAt)
		}
	}
	return b, nil
}

type BakalMonsterInfo struct {
	Kind, Location, Action, Health uint32
	Parties                        [3]int8
}

// NOTI2286 reader 144CDE400 passes fixed 19-byte records to 142543160.
// Party bytes are signed; -1 denotes an absent party association.
func BakalMonsterInfoPayload(monsters []BakalMonsterInfo) ([]byte, error) {
	if len(monsters) > 54 {
		return nil, fmt.Errorf("too many native Bakal monster locations")
	}
	b := make([]byte, 1+19*len(monsters))
	b[0] = byte(len(monsters))
	seen := map[uint32]bool{}
	for i, m := range monsters {
		if m.Kind > 16 || m.Location < 1 || m.Location > 51 || seen[m.Location] || m.Action < 1 || m.Action > 2 || m.Kind == 0 && (m.Action != 1 || m.Health != 0) {
			return nil, fmt.Errorf("invalid native Bakal monster record")
		}
		seen[m.Location] = true
		row := b[1+19*i:]
		for j, v := range []uint32{m.Kind, m.Location, m.Action, m.Health} {
			binary.LittleEndian.PutUint32(row[j*4:], v)
		}
		for j, party := range m.Parties {
			if party < -1 || party > 2 {
				return nil, fmt.Errorf("invalid native Bakal monster party")
			}
			row[16+j] = byte(party)
		}
	}
	return b, nil
}

// NOTI2288 is 13 bytes, not a vector of buff records. Five u8 inventory
// counts precede the used buff kind and target member position (142542F70).
func BakalBuffInfoPayload(counts [5]byte, usedBuff, memberPosition uint32) ([]byte, error) {
	if usedBuff > 25 || (memberPosition != ^uint32(0) && memberPosition > 11) {
		return nil, fmt.Errorf("invalid native Bakal buff use information")
	}
	b := make([]byte, 13)
	copy(b, counts[:])
	binary.LittleEndian.PutUint32(b[5:], usedBuff)
	binary.LittleEndian.PutUint32(b[9:], memberPosition)
	return b, nil
}

// RaidResult115 matches NOTI588 reader 144ce4e00: phase/result bytes,
// elapsed u32, member-clear u16, two native movie/result flags.
// Result zero follows the success branch; Bakal uses the single-phase title.
func BakalClearResult115(elapsed uint32) []byte {
	p := make([]byte, 10)
	binary.LittleEndian.PutUint32(p[2:], elapsed)
	p[9] = 1
	return p
}
