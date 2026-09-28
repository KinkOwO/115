package protocol

import (
	"encoding/binary"
	"fmt"
)

// AccountVaultRestore 对应 NOTI13/0x1452D5C70：容器 12、u16 容量、
// u32 存储金币、u16 物品数，随后为普通 181 字节记录。零容量表示尚未开通。
func AccountVaultRestore(slots uint16, gold uint32, rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if slots > 320 || slots%8 != 0 || (slots == 0 && (gold != 0 || len(rows) != 0)) {
		return nil, fmt.Errorf("账号金库容量或未开通状态无效")
	}
	for _, row := range rows {
		if binary.LittleEndian.Uint16(row[:]) >= slots {
			return nil, fmt.Errorf("账号金库物品超出容量")
		}
	}
	p, err := itemRows(rows)
	if err != nil {
		return nil, err
	}
	return append(add32(add16([]byte{12}, slots), gold), p...), nil
}

// PersonalVaultUpgradeNotice 使用 NOTI66 的 u16 类型 1、u16 目标容量。
// 0x1452C6E7B 仅在容量增大时调用 0x1469E0140，重建已打开金库的格子。
// 必须先于 NOTI13 容量快照发送，否则 0x1452C6E9C 会跳过即时刷新。
func PersonalVaultUpgradeNotice(slots uint16, space ...byte) ([]byte, error) {
	if slots < 8 || slots > 264 || (slots-8)%16 != 0 {
		return nil, fmt.Errorf("个人金库扩容通知的容量档位无效")
	}
	kind := uint16(1)
	if len(space) == 1 && space[0] == 45 {
		// NOTI66 类型 22 经 0x1452C6F28 更新第二金库并刷新对应页签。
		kind = 22
	} else if len(space) > 1 || (len(space) == 1 && space[0] != 2) {
		return nil, fmt.Errorf("个人金库扩容通知的容器无效")
	}
	return add16(add16(nil, kind), slots), nil
}

// NOTI13, native1452d5a80: inventory kind2, u16 slot capacity, u16 item
// count, 181 bytes per row, then one zero byte read by the current client.
func EmptyPersonalVault(slots uint16) ([]byte, error) {
	return PersonalVault(slots, nil)
}

// Kind 2 reads capacity, count and exactly 181 bytes per row, followed by a
// single byte at 0x14563965e. Live reader traces on 2026-09-28 saw that byte
// succeed for six rows via cipher padding and fail for seven block-aligned
// rows without padding (then CMD217). Emit it explicitly for every kind-2
// snapshot. There is no avatar/period tail per row.
func PersonalVault(slots uint16, rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	return PersonalVaultSpace(2, slots, rows)
}

// NOTI13 的容器 45 与容器 2 都读取 u16 容量、u16 数量及普通物品记录。
// 0x1452D5C5E 将容器 45 的容量保存到第二金库，不能映射到第一金库。
func PersonalVaultSpace(space byte, slots uint16, rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if space != 2 && space != 45 {
		return nil, fmt.Errorf("个人金库快照容器无效")
	}
	if slots == 0 {
		return nil, fmt.Errorf("vault capacity must select a valid client grade")
	}
	for _, row := range rows {
		if binary.LittleEndian.Uint16(row[:]) >= slots {
			return nil, fmt.Errorf("vault slot outside capacity")
		}
	}
	p, e := itemRows(rows)
	if e != nil {
		return nil, e
	}
	body := append(add16([]byte{space}, slots), p...)
	if space == 2 {
		return append(body, 0), nil
	}
	return body, nil
}

// PersonalVaultRestore retains the upstream API with slot validation.
func PersonalVaultRestore(slots uint16, rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	return PersonalVault(slots, rows)
}
