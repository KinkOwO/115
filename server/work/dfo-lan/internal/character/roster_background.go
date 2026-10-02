// RosterBackgroundState manages the account-level character select backgrounds.
package character

import (
	"encoding/binary"
	"fmt"
	"math"
)

const RosterBackgroundPages = 5

type RosterBackground struct {
	Category uint8
	ID       uint16
}

// RosterBackgroundUnlock 的时间为原生NOTI1759使用的Unix秒；0表示永久。
type RosterBackgroundUnlock struct {
	RosterBackground
	ExpiresAt uint32
}

type RosterBackgroundState struct {
	Selected [RosterBackgroundPages]RosterBackground
	Owned    []RosterBackgroundUnlock
}

// Valid 对照当前原版 etc/selectcharacterver2/selectcharacterver2.etc。
// 资源 SHA256：2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167。
// 仅证明资源存在；特殊背景仍须账号拥有，不能由编号范围自动授予。
func (b RosterBackground) Valid() bool {
	if catalog := currentTicketCatalog.Load(); catalog != nil {
		return catalog.ValidRosterBackground(b)
	}
	return legacyBackgroundValid(b)
}

func legacyBackgroundValid(b RosterBackground) bool {
	return b.Category == 0 && b.ID <= 5 || b.Category == 1 && (b.ID <= 51 || b.ID >= 500 && b.ID <= 504)
}

func (s RosterBackgroundState) CanSelect(b RosterBackground) bool {
	if !b.Valid() {
		return false
	}
	if b.Category == 0 {
		return true
	}
	for _, owned := range s.Owned {
		if owned.RosterBackground == b {
			return true
		}
	}
	return false
}

func (s RosterBackgroundState) Validate() error {
	if len(s.Owned) > 255 {
		return fmt.Errorf("选角背景拥有数量超出协议范围")
	}
	seen := make(map[RosterBackground]bool, len(s.Owned))
	for _, b := range s.Owned {
		if b.Category != 1 || !b.Valid() || seen[b.RosterBackground] || b.ExpiresAt > math.MaxInt32 {
			return fmt.Errorf("选角背景拥有记录无效或重复")
		}
		seen[b.RosterBackground] = true
	}
	for _, b := range s.Selected {
		if !s.CanSelect(b) {
			return fmt.Errorf("选角背景不存在或尚未解锁")
		}
	}
	return nil
}

// RosterBackgroundSelectRequest 是 CMD1725 选角背景选择请求（曾经的 protocol.SelectRosterBackgroundRequest）。
type RosterBackgroundSelectRequest struct {
	Background RosterBackground
	Page       uint16
}

// DecodeRosterBackgroundSelect 对应原生 140232880 的 CMD1725：u8 类别、u16 编号、u16 页号。
//
// 归属：选角背景报文由 character 领域拥有，protocol 不再反向依赖本领域。
func DecodeRosterBackgroundSelect(p []byte) (RosterBackgroundSelectRequest, error) {
	var r RosterBackgroundSelectRequest
	if len(p) < 5 {
		return r, fmt.Errorf("选角背景请求长度不足")
	}
	// CMD1725 使用不足 3、后 16 字节补零；2026-09-30 实测存在 5 字节字段后带 11 字节尾巴。
	if err := padding(p[5:], 16); err != nil {
		return r, err
	}
	r.Background = RosterBackground{Category: p[0], ID: binary.LittleEndian.Uint16(p[1:])}
	r.Page = binary.LittleEndian.Uint16(p[3:])
	if r.Page >= RosterBackgroundPages || !r.Background.Valid() {
		return r, fmt.Errorf("选角背景类别、编号或页号无效")
	}
	return r, nil
}

// RestoreRosterBackground 对应 NOTI1759 的原生 reader 1452E2C30。
// 在选择交换流程中，它会由原生日志把拥有情况重发给每个 u8/u16/u32；
// 其余背景由 1401FDF60 按 PVF 初始化背景池，只会把已解锁的外观背景。
func RestoreRosterBackground(state RosterBackgroundState) ([]byte, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	p := make([]byte, 17+7*len(state.Owned))
	for page, b := range state.Selected {
		p[page*3] = b.Category
		binary.LittleEndian.PutUint16(p[page*3+1:], b.ID)
	}
	// 原生日志的职业槽位尚未闭合，保留默认 0；编号槽位为拥有数量。
	p[16] = byte(len(state.Owned))
	for i, b := range state.Owned {
		at := 17 + 7*i
		p[at] = b.Category
		binary.LittleEndian.PutUint16(p[at+1:], b.ID)
		binary.LittleEndian.PutUint32(p[at+3:], b.ExpiresAt)
	}
	return p, nil
}

// padding 与原 protocol.padding 语义一致：尾部必须短于对齐值且全为零。
func padding(p []byte, alignment int) error {
	if len(p) >= alignment {
		return fmt.Errorf("excess packet tail")
	}
	for _, b := range p {
		if b != 0 {
			return fmt.Errorf("nonzero packet padding")
		}
	}
	return nil
}
