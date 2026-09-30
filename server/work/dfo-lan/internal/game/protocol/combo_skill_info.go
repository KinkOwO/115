package protocol

import (
	"encoding/binary"
	"fmt"
)

// ComboSkillInfo is CMD 500 ENUM_CMDPACKET_COMBO_SKILL_INFO, the body the
// client pushes every time the skill window closes and after every combo-cell
// edit.
//
// Layout, recovered from the retained sessions of 2026-09-26..27 (all nine
// distinct bodies decode cleanly):
//
//	u8      0                      // header, observed 0 in every sample
//	u8      cellCount              // 6 for the dark knight (comboset0..5)
//	repeat cellCount {
//	    u16le comboSkill           // 118..123 for the dark knight
//	    u8    chainCount           // 0..5
//	    u16le chain[chainCount]    // the skills ordered into this cell
//	}
//	...                            // zero padding to a 4-byte multiple
//
// The trailing bytes are padding, not payload: a 20-byte body is the six cells
// with no chains, a 22-byte body pads to 24, a 30-byte body pads to 32. Decode
// keeps Raw up to the last cell byte, exactly as DecodeSkillCommands does for
// CMD331, so an entry replay echoes the arrangement without the pad.
//
// The bodies observed on the wire:
//
//	00 06 76 00 00 77 00 00 78 00 00 79 00 00 7a 00 00 7b 00 00
//	00 06 76 00 01 6c 00 77 00 00 78 00 00 79 00 00 7a 00 00 7b 00 00 00 00
//	00 06 76 00 05 2e 00 6c 00 05 00 08 00 75 00 77 00 00 78 ... 7b 00 00 00 00
//	00 06 76 00 05 18 00 49 00 4a 00 51 00 fc 00 77 00 00 78 ... 7b 00 00 00 00
//
// which is cell 118 carrying a chain of 0 / 1 / 5 / 5 skills and cells
// 119..123 carrying none.
type ComboSkillInfo struct {
	Cells []ComboCell
	// Raw is the body up to the end of the last cell, without the trailing
	// zero alignment. It is what gets persisted and echoed back on entry.
	Raw []byte
}

// ComboCell is one combo-extension cell: the combo skill that owns the cell
// and the skills the player ordered into it.
type ComboCell struct {
	Skill uint16
	Chain []uint16
}

// maxComboCells bounds cellCount. The dark knight publishes six; the byte
// field makes anything above 255 unrepresentable anyway.
const maxComboCells = 64

// maxComboChain bounds one cell's chain. The count is a byte, so 255 is the
// hard ceiling; the observed maximum is 5, matching CMD331's 1..5 tokens.
const maxComboChain = 255

// DecodeComboSkillInfo reads CMD 500. The decrypted frame is block padded with
// zeros, so the count and entries are the authoritative length.
func DecodeComboSkillInfo(p []byte) (ComboSkillInfo, error) {
	var out ComboSkillInfo
	if len(p) < 2 {
		return out, fmt.Errorf("short combo skill info")
	}
	if p[0] != 0 {
		return out, fmt.Errorf("invalid combo skill info header")
	}
	count := int(p[1])
	if count == 0 || count > maxComboCells {
		return out, fmt.Errorf("invalid combo cell count")
	}
	pos := 2
	cells := make([]ComboCell, 0, count)
	seen := map[uint16]bool{}
	for i := 0; i < count; i++ {
		if len(p)-pos < 3 {
			return ComboSkillInfo{}, fmt.Errorf("short combo cell entry")
		}
		skill := binary.LittleEndian.Uint16(p[pos:])
		chainCount := int(p[pos+2])
		pos += 3
		if skill == 0 || seen[skill] {
			return ComboSkillInfo{}, fmt.Errorf("invalid combo cell skill")
		}
		if len(p)-pos < 2*chainCount {
			return ComboSkillInfo{}, fmt.Errorf("short combo cell chain")
		}
		chain := make([]uint16, chainCount)
		for j := range chain {
			v := binary.LittleEndian.Uint16(p[pos+2*j:])
			if v == 0 {
				return ComboSkillInfo{}, fmt.Errorf("invalid combo chain skill")
			}
			chain[j] = v
		}
		pos += 2 * chainCount
		seen[skill] = true
		cells = append(cells, ComboCell{Skill: skill, Chain: chain})
	}
	for _, b := range p[pos:] {
		if b != 0 {
			return ComboSkillInfo{}, fmt.Errorf("nonzero combo skill info padding")
		}
	}
	out.Cells = cells
	out.Raw = append([]byte(nil), p[:pos]...)
	return out, nil
}

// EncodeComboSkillInfo writes the body back out. Entry replays the persisted
// Raw, so this is only the fallback for a state whose Raw was not retained.
func EncodeComboSkillInfo(info ComboSkillInfo) ([]byte, error) {
	if len(info.Cells) == 0 || len(info.Cells) > maxComboCells {
		return nil, fmt.Errorf("invalid combo cell count")
	}
	buf := make([]byte, 0, 2+3*len(info.Cells))
	buf = append(buf, 0, byte(len(info.Cells)))
	var pair [2]byte
	for _, cell := range info.Cells {
		if cell.Skill == 0 || len(cell.Chain) > maxComboChain {
			return nil, fmt.Errorf("invalid combo cell")
		}
		binary.LittleEndian.PutUint16(pair[:], cell.Skill)
		buf = append(buf, pair[0], pair[1], byte(len(cell.Chain)))
		for _, v := range cell.Chain {
			if v == 0 {
				return nil, fmt.Errorf("invalid combo chain skill")
			}
			binary.LittleEndian.PutUint16(pair[:], v)
			buf = append(buf, pair[0], pair[1])
		}
	}
	return buf, nil
}

// comboSkillInfoNotifyPrefix is the three-byte header NOTI433 carries in front
// of the cell list. The S2C arm is *not* the C2S body: the two enums share no
// layout. Recovered on 2026-09-27 by reading the live dispatch registry and
// disassembling the real handler, NOTIPACKET_COMBO_SKILL_INFO -> 0x1452A85B0
// (see docs/protocol/dark-knight-comboset-quickbar-20260930.md section 2):
//
//	0x1452a860f  read u8  -> T1     ; only gates one extra refresh call
//	0x1452a8623  read u8  -> A1     ; number of blocks; 0 jumps straight to the tail
//	0x1452a866b  read u8  -> PAGE   ; stored into A->[0x10D20], then 0x145D4C250(A)
//	0x1452a86b5  read u8  -> N_OUTER
//	0x1452a8715  read u16 -> KEY    ; looked up via A->vtbl[0x2048]
//	0x1452a8755  read u8  -> N_INNER
//	0x1452a8775  read u16 -> SUBKEY ; N_INNER of them
//
// N_OUTER + the {u16, u8, u8*u16} records after it are byte-for-byte the
// C2S cell list, so a notify body is this prefix followed by the C2S body with
// its own leading '0' dropped.
//
// Why each byte is what it is:
//
//	T1   = 0  the flag only adds a call we do not need.
//	A1   = 1  the arrangement is a single block.
//	PAGE = 0  it lands in A->[0x10D20]; the handler saves the old value on entry
//	          and restores it on the way out, so writing the current value is a
//	          no-op. Only two instructions in the whole image ever write that
//	          field (0x145CAD9E6 and 0x145CCFD26) and both store 0, so 0 *is*
//	          the live value. It also has to stay in range: 0x145D4C250 copies
//	          0x200 * 0x18 bytes to A + PAGE*0x3000 + 0xAD20, and feeding it the
//	          C2S body's third byte (0x76 = 118, the first combo skill) is what
//	          read 1.8 MB past the object and produced every 0xC0000005 of
//	          2026-09-27.
var comboSkillInfoNotifyPrefix = []byte{0x00, 0x01, 0x00}

// EncodeComboSkillInfoNotify renders the S2C NOTI433 body for an arrangement.
//
// The caller must not fall back to echoing the C2S body: the client reads the
// C2S bytes as {T1, A1, PAGE, ...}, so a verbatim echo makes PAGE = 0x76 and
// dereferences a few hundred kilobytes past the end of the object.
func EncodeComboSkillInfoNotify(info ComboSkillInfo) ([]byte, error) {
	body, err := EncodeComboSkillInfo(info)
	if err != nil {
		return nil, err
	}
	if len(body) < 2 || body[0] != 0 {
		return nil, fmt.Errorf("invalid combo skill info")
	}
	out := make([]byte, 0, len(body)+len(comboSkillInfoNotifyPrefix)-1)
	out = append(out, comboSkillInfoNotifyPrefix...)
	out = append(out, body[1:]...)
	return out, nil
}
