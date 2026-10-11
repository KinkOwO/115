package protocol

import (
	"encoding/binary"
	"fmt"
	"unicode"
	"unicode/utf8"
)

// CMD1430/1436: 146D76080 calls 149185900 (CP_UTF8).
// The logical name capacity stays 16 GBK bytes (8 Chinese / 16 ASCII).
// UTF-8 uses up to 24 bytes; the paired client mod raises only the two
// adventure-name reader bounds to 25, including the trailing NUL.
func adventureNameText(name string) ([]byte, error) {
	if _, err := adventureText(name, 16); err != nil {
		return nil, err
	}
	if len(name) > 24 {
		return nil, fmt.Errorf("冒险团名称超过 UTF-8 字节容量")
	}
	return []byte(name), nil
}

func DecodeAdventureName(p []byte) (string, error) {
	if len(p) < 4 {
		return "", fmt.Errorf("冒险团名称字段不完整")
	}
	n := binary.LittleEndian.Uint32(p)
	if n == 0 || n > 24 || uint64(n)+4 > uint64(len(p)) {
		return "", fmt.Errorf("冒险团名称长度无效")
	}
	raw := p[4 : 4+n]
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("冒险团名称 UTF-8 编码无效")
	}
	name := string(raw)
	if _, err := adventureNameText(name); err != nil {
		return "", fmt.Errorf("冒险团名称编码无效")
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", fmt.Errorf("冒险团名称不能含空白或控制字符")
		}
	}
	return name, padding(p[4+n:], 16)
}

// CMD1436 success without the rename-coupon dialog reads a name at
// 14528B3D4 and opens the creation confirmation using that exact value.
// CMD1430 success also reads the committed name at 14529A107.
func AdventureNameAccepted(name string) ([]byte, error) {
	b, err := adventureNameText(name)
	if err != nil {
		return nil, err
	}
	return append(append([]byte{1}, add32(nil, uint32(len(b)))...), b...), nil
}

// NOTI1363, reader 1452DF9D0. Flag 2 == 1 opens the native naming dialog.
func AdventureNameInfo(name string, choose bool) ([]byte, error) {
	b, err := adventureNameText(name)
	if err != nil {
		return nil, err
	}
	p := append(add32(nil, uint32(len(b))), b...)
	p = append(p, 0, 0, 0)
	if choose {
		p[len(p)-2] = 1
	}
	return p, nil
}

// CMD1726 sender 14340ED90 passes the full 20-byte structure to 146D75B10.
// Its opaque 13-byte prefix is part of the body, not the transport header.
// Kind is at +13, alignment at +14..15, symbol u32 at +16.
func DecodeAdventureSymbol(p []byte) (uint32, error) {
	if len(p) < 20 || p[13] != 0 {
		return 0, fmt.Errorf("不支持的冒险团徽章设置")
	}
	return binary.LittleEndian.Uint32(p[16:]), padding(p[20:], 16)
}

// CMD1726 reader 142F9F2D0 consumes kind u8 and symbol u32 after success.
func AdventureSymbolChanged(id uint32) []byte { return add32([]byte{1, 0}, id) }

// NOTI1760 reader 142F9F480: symbol and three housing slots, all u32.
func AdventureSymbolInfo(id uint32) []byte { return append(add32(nil, id), make([]byte, 12)...) }
