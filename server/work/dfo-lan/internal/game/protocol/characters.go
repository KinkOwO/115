// Package protocol contains build-specific packet layouts, independent of
// persistence and game rules. Layout evidence is recorded under docs/protocol.
package protocol

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type CreateRequest struct {
	Profession byte
	Name       string
	// GrowthType is the advancement slot the naming window had selected,
	// carried in option byte 8. It indexes the character script's
	// [growtype N] sections as slot N-1.
	GrowthType byte
	Options    []byte
}

func parseName(p []byte) (string, int, error) {
	if len(p) < 4 {
		return "", 0, fmt.Errorf("missing name length")
	}
	n := int(binary.LittleEndian.Uint32(p))
	if n < 1 || n > 31 || n > len(p)-4 {
		return "", 0, fmt.Errorf("invalid name length")
	}
	name := string(p[4 : 4+n])
	if !utf8.ValidString(name) || strings.TrimSpace(name) != name {
		return "", 0, fmt.Errorf("invalid name encoding or surrounding spaces")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", 0, fmt.Errorf("control character in name")
		}
	}
	return name, 4 + n, nil
}
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

// Native sender 0x1402396f0 writes one length-prefixed string (cipher slot 12).
func DecodeNameRequest(p []byte) (string, error) {
	n, k, e := parseName(p)
	if e != nil {
		return "", e
	}
	return n, padding(p[k:], 16)
}

// Native legacy sender 0x1402394b4 writes ten option bytes. The current
// naming window sender 0x141733fe7 writes twelve, including a selectable
// value at option 8 and two further bytes at 0x1417340ca/0x1417340d9.
func DecodeCreateRequest(p []byte) (CreateRequest, error) {
	var r CreateRequest
	if len(p) < 1 {
		return r, fmt.Errorf("missing profession")
	}
	r.Profession = p[0]
	n, k, e := parseName(p[1:])
	if e != nil {
		return r, e
	}
	r.Name = n
	k++
	if len(p) < k+10 {
		return r, fmt.Errorf("short creation options")
	}
	optionCount := 10
	if len(p)-k >= 12 {
		optionCount = 12
	}
	r.Options = append([]byte(nil), p[k:k+optionCount]...)
	suffix := [5]byte{0, 0, 0, 255, 0}
	for i, b := range suffix {
		if r.Options[i+3] != b {
			return r, fmt.Errorf("unsupported creation option layout")
		}
	}
	if optionCount >= 12 {
		r.GrowthType = r.Options[8]
	}
	return r, padding(p[k+optionCount:], 8)
}
func add16(p []byte, v uint16) []byte   { return binary.LittleEndian.AppendUint16(p, v) }
func add32(p []byte, v uint32) []byte   { return binary.LittleEndian.AppendUint32(p, v) }
func addName(p []byte, s string) []byte { return append(add32(p, uint32(len(s))), []byte(s)...) }

// 未保存显示偏好的旧角色沿用原始向量0x03；bit0是独立状态，必须保留。
const nativeGrowthStateFlags byte = 1<<0 | 1<<1

// GrowthEffectOption 来自141154C50及CMD2377实机请求：账号标量选项1。
const GrowthEffectOption uint16 = 1

// GrowthEffectFlags 对应14563EC60/14563E280及NOTI343的显示位。
// 真实觉醒阶段仍由Advancement携带，145F06E10按角色资格限制显示阶段。
func GrowthEffectFlags(value uint16) (byte, error) {
	switch value {
	case 0:
		return 1, nil
	case 1:
		return nativeGrowthStateFlags, nil
	case 2:
		return 1 | 1<<4, nil
	case 3:
		return 1 | 1<<5, nil
	default:
		return 0, fmt.Errorf("转职觉醒特效选项无效：%d", value)
	}
}

func (r CharacterRow) growthStateFlags() byte {
	if r.GrowthEffectFlags != 0 {
		return r.GrowthEffectFlags
	}
	return nativeGrowthStateFlags
}

// CharacterGrowthEffect 的注册点1452FA194指定NOTI343；1452C9940
// 依次读取u16角色编号和u8标志，只刷新特效，不重建背包或技能。
func CharacterGrowthEffect(actor uint16, flags byte) []byte {
	return append(add16(nil, actor), flags)
}

// The native create callback feeds this value into the roster-position lookup
// (0x1401f8a30 -> 0x14021adf0), not into a persistent character-ID lookup.
func CreateSuccess(slot uint16, name string) []byte { return addName(add16([]byte{1}, slot), name) }
func Refusal(code uint16) []byte                    { return add16([]byte{0}, code) }

type Equipment struct {
	Slot byte
	Item uint32
	// Model 是该槽的模型索引，在外观块里以「Len + 载荷」的形式携带。
	// 宠物幻化栏（穿戴槽 32）用它指向要画的生物实例 key；其余槽位留 0，
	// 客户端对 Len=0 的处理是「保留该槽原有的模型索引」。
	Model uint32
}

// 145639840 calls 1459a0220 with list46 for EVERY row, including weapons.
// That helper consumes a length-prefixed blob before the 26 fixed tail bytes.
func EquipmentAppearance(rows []Equipment) ([]byte, error) {
	if len(rows) > 48 {
		return nil, fmt.Errorf("too many appearance rows")
	}
	p := []byte{byte(len(rows))}
	seen := map[byte]bool{}
	for _, row := range rows {
		if row.Slot >= 48 || row.Item == 0 || seen[row.Slot] {
			return nil, fmt.Errorf("invalid appearance row")
		}
		seen[row.Slot] = true
		p = add32(append(p, row.Slot), row.Item)
		// Len 与载荷必须成对出现：Len=0 表示「这个槽的模型索引不动」，
		// Len=4 表示后面跟一个索引（1459a0220 读 Len 再读 Len 字节，客户端把
		// 它存到 [actor + slot*4 + 0x405]）。
		if row.Model == 0 {
			p = add32(p, 0)
		} else {
			p = add32(p, equippedAppearanceModelSize)
			p = add32(p, row.Model)
		}
		p = append(p, make([]byte, 26)...)
	}
	return p, nil
}

type CharacterRow struct {
	Odyssey           bool
	Slot              uint16
	FixedSlot         byte // zero: normal list; otherwise one-based fixed grid cell
	Name              string
	Profession        byte
	Advancement       byte
	Level             byte
	Equipment         []Equipment
	FatigueRemaining  uint16
	FatigueBonus      uint16
	Fame              uint32
	CreatureItemID    uint32
	CreatureName      string
	GrowthEffectFlags byte // 0沿用旧默认；显式关闭为1，保留独立bit0。
	AuraVisible       bool // 选角光环显示；独立于觉醒特效和装备外观绑定。
	// 当前115客户端的28字节内容解锁区；只写已核对的资格位，未知位保持0。
	ContentClearFlags [28]byte
}

// Native list parser 0x145637a20, row parser 0x14563e280. Unknown scalar
// fields are zero in this experimental baseline, recorded as such in evidence.
func CharacterList(capacity uint16, roles []CharacterRow) ([]byte, error) {
	p := []byte{2, 0, 0}
	p = add16(p, capacity)
	p = add16(p, 0)
	p = add16(p, 0)
	p = add32(p, 0)
	p = add16(p, uint16(len(roles)))
	p, err := appendCharacterListRows(p, capacity, roles)
	if err != nil {
		return nil, err
	}
	p = append(p, 1, 0)
	p = add32(p, 0)
	p = add32(p, 0)
	return p, nil
}

// 14563850E的NOTI2模式13：服务器u8、数量u16，然后复用14563E280。
// 与选角列表不同，不带容量头和选角尾；1396的数量应先发送，
// 1401FA800核对各服务器实际条数后，才解除CMD1462的等待状态。
func AllServerCharacterList(server byte, capacity uint16, roles []CharacterRow) ([]byte, error) {
	if server == 0 {
		return nil, fmt.Errorf("账号角色资料缺少服务器编号")
	}
	return appendCharacterListRows(add16([]byte{13, server}, uint16(len(roles))), capacity, roles)
}

func appendCharacterListRows(p []byte, capacity uint16, roles []CharacterRow) ([]byte, error) {
	if capacity == 0 || len(roles) > int(capacity) {
		return nil, fmt.Errorf("invalid character capacity")
	}
	fixed := map[byte]bool{}
	for index, r := range roles {
		// 0x1401f64d8 builds the native lookup from insertion positions. The
		// row key and create receipt must use that same zero-based position.
		if r.Slot != uint16(index) || r.Level == 0 {
			return nil, fmt.Errorf("invalid character row")
		}
		if r.FixedSlot != 0 {
			if uint16(r.FixedSlot) > capacity || fixed[r.FixedSlot] {
				return nil, fmt.Errorf("invalid fixed character slot")
			}
			fixed[r.FixedSlot] = true
		}
		if _, _, e := parseName(addName(nil, r.Name)); e != nil {
			return nil, e
		}
		p = addName(add16(p, r.Slot), r.Name)
		p = append(p, 0, r.Profession, r.Advancement, r.Level, 0, 0)
		appearance, e := EquipmentAppearance(r.Equipment)
		if e != nil {
			return nil, e
		}
		p = append(p, appearance...)
		p = add32(p, 0)
		p = append(p, 0, 0, 0, 0)
		p = append(p, 0) // cosmetic helper 0x145639b70: zero count
		p = add32(p, 0)
		p = add32(p, 0)
		p = append(p, 0)
		p = add32(p, 0)
		p = append(p, 0)
		p = append(p, 0) // premium PC room helper 0x14563be50
		p = add32(p, 0)
		p = append(p, r.growthStateFlags())
		p = append(p, make([]byte, 8)...)
		// 14563E95E整块读取到临时角色资料+0x638，随后复制进角色列表。
		// 14022091E检查+0x63F（第7项）决定流放者山脉前置是否完成。
		p = append(p, r.ContentClearFlags[:]...)
		p = add32(p, 0)
		// 14563E985读取u16，等于1时写入角色资料+0x65E，并经145BF26A0
		// 设置显示对象+0x249C；145BDCFD0/145BBB430以此决定是否加载光环。
		// 原先固定0会让选角隐藏光环，即使装备槽9/11已正确同步。
		var auraVisible uint16
		if r.AuraVisible {
			auraVisible = 1
		}
		p = add16(p, auraVisible)
		// Native 14563e9ee stores this third byte at row-info+672.
		var mode byte
		if r.Odyssey {
			mode = 5
		}
		// 14563e9b0 stores the first byte at row-info+0x660;
		// 1401f6340 uses it to restore the fixed grid cell.
		p = append(p, r.FixedSlot, 0, mode)
		// 14563ea07 读取的首个 u32 写入 row-info+0x6b4，与进城名望使用同一字段。
		p = add32(p, r.Fame)
		for i := 0; i < 3; i++ {
			p = add32(p, 0)
		}
		// 14563ea6c/76 -> info+6d8/+6dc -> 140209be2 ->
		// 14020a04d renders these exact two values as "%d+%d".
		if r.FatigueRemaining > 32767 || r.FatigueBonus > 32767 {
			return nil, fmt.Errorf("roster fatigue exceeds native signed range")
		}
		p = add16(p, r.FatigueRemaining)
		p = add16(p, r.FatigueBonus)
		p = add32(p, 0)
	}
	return p, nil
}
