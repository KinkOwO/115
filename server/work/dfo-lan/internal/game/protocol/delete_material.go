package protocol

import (
	"encoding/binary"
	"fmt"
)

type MaterialDelete struct {
	Slot            uint16
	Template, Count uint32
}

// Current CMD18 is length-prefixed PB_ENUM_CMDPACKET_DELETE_ITEM.
// Only the observed ordinary-bag skill-cost branch (reason2, material3037)
// is admitted, not general deletion, equipment, or other protobuf actions.
//
// The slot may name either storage the client keeps that material in:
//
//	121..176  the ordinary bag's material cells
//	363..379  the account-shared material store. The client's list0 reader
//	          harvests these cells into a separate panel (sub_145ADC2A0), so a
//	          skill whose cost is paid from the shared store sends its storage
//	          slot here - 367 for 无色小晶块. A frame like that used to be
//	          rejected outright, which left the cost undeducted and the client
//	          holding a pending reservation.
//
// Which template belongs to which storage cell is checked by the caller:
// that mapping lives in the inventory package, and protocol must stay below it.
func DecodeMaterialDelete(p []byte) ([]MaterialDelete, error) {
	return decodeMaterialDelete(p, false)
}

// DecodeCubeContractDelete 解析晶体契约的原因 5 消耗；与技能无色消耗分开校验。
// 实机向量：槽 368、金色小晶块 3262、数量 1。客户端契约材料表含六种晶块。
func DecodeCubeContractDelete(p []byte) ([]MaterialDelete, error) {
	return decodeMaterialDelete(p, true)
}

func decodeMaterialDelete(p []byte, contract bool) ([]MaterialDelete, error) {
	fail := func() ([]MaterialDelete, error) { return nil, fmt.Errorf("invalid skill material deletion") }
	if len(p) < 4 {
		return fail()
	}
	n := int(binary.LittleEndian.Uint32(p))
	if n < 1 || n > 4096 || n > len(p)-4 {
		return fail()
	}
	for _, b := range p[4+n:] {
		if b != 0 {
			return fail()
		}
	}
	data := p[4 : 4+n]
	var rows []MaterialDelete
	seen := map[uint16]bool{}
	inventory, mode := false, false
	for len(data) > 0 {
		tag, k := binary.Uvarint(data)
		if k <= 0 {
			return fail()
		}
		data = data[k:]
		v, k := binary.Uvarint(data)
		if k <= 0 {
			return fail()
		}
		data = data[k:]
		switch tag {
		case 16:
			if inventory || v != 0 {
				return fail()
			}
			inventory = true
		case 32:
			if mode || v != 0 {
				return fail()
			}
			mode = true
		case 26:
			if v > uint64(len(data)) || len(rows) >= 56 {
				return fail()
			}
			row := data[:int(v)]
			data = data[int(v):]
			fields := map[uint64]uint64{}
			for len(row) > 0 {
				t, a := binary.Uvarint(row)
				if a <= 0 {
					return fail()
				}
				row = row[a:]
				x, b := binary.Uvarint(row)
				if b <= 0 {
					return fail()
				}
				row = row[b:]
				if _, ok := fields[t]; ok {
					return fail()
				}
				fields[t] = x
			}
			reason, template := uint64(2), fields[24]
			validMaterial := template == 3037
			if contract {
				reason = 5
				validMaterial = (template >= 3033 && template <= 3037) || template == 3262
			}
			if len(fields) != 4 || fields[8] != reason || !validMaterial || fields[32] == 0 || fields[32] > 1000 {
				return fail()
			}
			slot := fields[16]
			inBag := slot >= 121 && slot <= 176
			inStorage := slot >= 363 && slot <= 379
			if !inBag && !inStorage {
				return fail()
			}
			r := MaterialDelete{uint16(slot), uint32(template), uint32(fields[32])}
			if seen[r.Slot] {
				return fail()
			}
			seen[r.Slot] = true
			rows = append(rows, r)
		default:
			return fail()
		}
	}
	if !inventory || !mode || len(rows) == 0 {
		return fail()
	}
	return rows, nil
}

// Native parsers140196000/140196270: RETURN{space1,rows2,error3},
// row{slot1,amount2,reason3}. Handler145272030 decrements the pending cube
// reservation in145ad2360 as well as the displayed stack.
func MaterialDeleteReply(rows []MaterialDelete, success bool) []byte {
	return materialDeleteReply(rows, success, 2)
}

// CubeContractDeleteReply 保留原生契约消耗原因，不能当作技能无色消耗回显。
func CubeContractDeleteReply(rows []MaterialDelete, success bool) []byte {
	return materialDeleteReply(rows, success, 5)
}

func materialDeleteReply(rows []MaterialDelete, success bool, reason byte) []byte {
	body := []byte{8, 0, 24, 0}
	for _, r := range rows {
		row := binary.AppendUvarint([]byte{8}, uint64(r.Slot))
		row = binary.AppendUvarint(append(row, 16), uint64(r.Count))
		row = append(row, 24, reason)
		body = binary.AppendUvarint(append(body, 18), uint64(len(row)))
		body = append(body, row...)
	}
	header := []byte{1}
	if !success {
		// 通用失败分发先消费 u16 错误码，145272030 随后才读 protobuf 长度。
		// 缺少这两字节会把正文误当长度，实机随即报告 CMD217 的 CMD18 溢出。
		header = Refusal(0)
	}
	p := add32(header, uint32(len(body)))
	return append(p, body...)
}
