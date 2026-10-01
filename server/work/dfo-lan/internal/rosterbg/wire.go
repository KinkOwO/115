package rosterbg

import (
	"encoding/binary"
	"fmt"
)

// SelectRequest 是 CMD1725 选角背景选择请求（曾经的 protocol.SelectRosterBackgroundRequest）。
type SelectRequest struct {
	Background Background
	Page       uint16
}

// DecodeSelect 对应原生 140232880 的 CMD1725：u8 类别、u16 编号、u16 页号。
//
// 归属：选角背景报文由 rosterbg 拥有，protocol 不再反向依赖本领域。
func DecodeSelect(p []byte) (SelectRequest, error) {
	var r SelectRequest
	if len(p) < 5 {
		return r, fmt.Errorf("选角背景请求长度不足")
	}
	// CMD1725 使用不足 3、后 16 字节补零；2026-09-30 实测存在 5 字节字段后带 11 字节尾巴。
	if err := padding(p[5:], 16); err != nil {
		return r, err
	}
	r.Background = Background{Category: p[0], ID: binary.LittleEndian.Uint16(p[1:])}
	r.Page = binary.LittleEndian.Uint16(p[3:])
	if r.Page >= Pages || !r.Background.Valid() {
		return r, fmt.Errorf("选角背景类别、编号或页号无效")
	}
	return r, nil
}

// Restore 对应 NOTI1759 的原生 reader 1452E2C30。
// 在选择交换流程中，它会由原生日志把拥有情况重发给每个 u8/u16/u32；
// 其余背景由 1401FDF60 按 PVF 初始化背景池，只会把已解锁的外观背景。
func Restore(state State) ([]byte, error) {
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
