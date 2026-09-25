package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

// MaxItemPeriod is the fallback written to the 181-byte row's offset-56 cell
// (u32 LE) when nothing better is known. It is NOT a timestamp: the client
// renders that cell as `Expires in : %d Day(s)` (dstr 1057,
// PopupWindow\CNRDItemInfoWindow.cpp) by dividing it by 86400, with no
// reference to the current time.
//
// 实机 2026-09-23 取证：宠物 100991331（脚本 `[usable period] 7`）的行填了 2147483647，
// 客户端面板显示「过期时间:24856天」—— 2147483647 / 86400 = 24855.6 四舍五入即 24856，
// 逐位吻合，证明该格是「剩余秒数」而非时间戳（若减当前时间则应为 4138 天）。
//
// 因此声明了期限的物品，这格必须填「真实剩余秒数」；本常量只适合做
// 「没有更好值」的兼容兜底（脚本不声明期限的物品不会渲染这行文案）。
const MaxItemPeriod = math.MaxInt32

// OrdinaryItem is the current 181-byte base row. Special equipment branches
// require extra source/type validation and are deliberately separate.
func OrdinaryItem(slot uint16, template, amount uint32, expireTime ...uint32) [CurrentItemRecordSize]byte {
	var p [CurrentItemRecordSize]byte
	binary.LittleEndian.PutUint16(p[:], slot)
	binary.LittleEndian.PutUint32(p[2:], template)
	binary.LittleEndian.PutUint32(p[6:], amount)
	var period uint32
	if len(expireTime) > 0 {
		period = expireTime[0]
	}
	if period = ItemPeriodForWire(template, period); period != 0 {
		binary.LittleEndian.PutUint32(p[56:], period)
	}
	return p
}

func itemRows(rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if len(rows) > 65535 {
		return nil, fmt.Errorf("too many inventory rows")
	}
	p := add16(nil, uint16(len(rows)))
	seen := map[uint16]bool{}
	for _, r := range rows {
		slot := binary.LittleEndian.Uint16(r[:])
		if seen[slot] {
			return nil, fmt.Errorf("duplicate inventory slot")
		}
		seen[slot] = true
		p = append(p, r[:]...)
	}
	return p, nil
}

// NOTI13 list0：0x1452d5cf4 读取扩展格数，右移三位恢复档位；随后读取物品数。
func InventoryRestore(rows [][CurrentItemRecordSize]byte, expansion ...byte) ([]byte, error) {
	var tier byte
	if len(expansion) > 1 || (len(expansion) == 1 && expansion[0] > 2) {
		return nil, fmt.Errorf("背包扩展档位无效")
	}
	if len(expansion) == 1 {
		tier = expansion[0]
	}
	p, e := itemRows(rows)
	if e != nil {
		return nil, e
	}
	return append([]byte{0, tier * 8, 0}, p...), nil
}

// InventoryExpansionNotice：NOTI66 的 u16 类型 12 + u16 扩展格数。
// 0x1452c7624 读取后右移三位，0x1458db133 的类型 2 刷新档位并显示成功提示。
func InventoryExpansionNotice(tier byte) ([]byte, error) {
	if tier == 0 || tier > 2 {
		return nil, fmt.Errorf("背包扩展档位无效")
	}
	return add16(add16(nil, 12), uint16(tier)*8), nil
}

// InventoryRestoreSpace snapshots a non-bag NOTI13 list. The 115 client
// reader sub_1452D5A80 only consumes the extra u16 expansion count for lists
// 0/1 (and a u8+u16 pair for 38); every other list starts with the plain
// u16 row count. List 35 is the account material storage: rows at fixed
// slots 363..379 are re-harvested out of the bag manager when the list0
// snapshot follows (sub_145ADC2A0).
func InventoryRestoreSpace(space byte, rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if space == 0 {
		return InventoryRestore(rows)
	}
	p, e := itemRows(rows)
	if e != nil {
		return nil, e
	}
	return append([]byte{space}, p...), nil
}

func InventoryUpdate(rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	p, e := itemRows(rows)
	if e != nil {
		return nil, e
	}
	return append([]byte{0}, p...), nil
}

func InventorySpaceUpdate(space byte, rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if space != 0 && space != 2 && space != 3 && space != 12 && space != 45 {
		return nil, fmt.Errorf("unsupported equipment update space")
	}
	p, e := itemRows(rows)
	if e != nil {
		return nil, e
	}
	return append([]byte{space}, p...), nil
}

// Current NOTI13 list3 omits list0's lock count and adds a u32 period after
// every181-byte row (1452d6a0b). Period0 represents ordinary permanent gear.
func WornRestore(rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if _, e := itemRows(rows); e != nil {
		return nil, e
	}
	p := add16([]byte{3}, uint16(len(rows)))
	for _, r := range rows {
		p = append(p, r[:]...)
		p = add32(p, 0)
	}
	return p, nil
}

type PickupRequest struct {
	Object                       uint32
	ActorX, ActorY, DropX, DropY uint16
	Context, Flag                byte // Opaque client observations; never item IDs or amounts.
}

func DecodePickup(p []byte) (PickupRequest, error) {
	var r PickupRequest
	if len(p) != 21 && len(p) != 24 && len(p) != 32 {
		return r, fmt.Errorf("pickup requires21 bytes with cipher block padding")
	}
	for _, b := range p[21:] {
		if b != 0 {
			return r, fmt.Errorf("pickup padding")
		}
	}
	if p[4] != 0 || p[20] > 1 {
		return r, fmt.Errorf("unsupported pickup option")
	}
	r = PickupRequest{Object: binary.LittleEndian.Uint32(p), ActorX: binary.LittleEndian.Uint16(p[6:]), ActorY: binary.LittleEndian.Uint16(p[8:]), DropX: binary.LittleEndian.Uint16(p[12:]), DropY: binary.LittleEndian.Uint16(p[14:]), Context: p[5], Flag: p[20]}
	if r.Object == 0 {
		return r, fmt.Errorf("zero scene object")
	}
	return r, nil
}

// NOTI39 gold consumes eight (u8 flag,u32 amount) rows and has no ordinary
// tail. Neutral flags avoid applying an unverified currency delta; NOTI14
// restores the authoritative gold balance before this scene removal.
func PickupConfirmed(object uint32, actor, slot uint16, gold bool) ([]byte, error) {
	if object == 0 || actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid pickup identity")
	}
	p := add16(add32(nil, object), actor)
	if gold {
		return append(p, make([]byte, 40)...), nil
	}
	p = append(p, make([]byte, 8)...)
	p = add16(add16(p, actor), slot)
	return append(p, 0), nil
}

// Current solo party0 gold row includes a flag, amount and an extra-count
// byte. This invokes both the local currency delta and overhead number;
// the remaining seven party rows are absent (flag0, amount0).
func GoldPickupConfirmed(object uint32, actor uint16, amount uint32) ([]byte, error) {
	if object == 0 || actor == 0 || actor == 65535 || amount == 0 {
		return nil, fmt.Errorf("invalid gold pickup")
	}
	p := add16(add32(nil, object), actor)
	p = add32(append(p, 1), amount)
	return append(p, make([]byte, 36)...), nil
}

// Failure consumes the scene object after the dispatcher's u16 error code.
// A generic Refusal alone truncates145244e5d and leaves pickup in flight.
func PickupRefused(object uint32) ([]byte, error) {
	if object == 0 {
		return nil, fmt.Errorf("zero pickup refusal object")
	}
	return add32(Refusal(4), object), nil
}

func MonsterDeathDrops(entity uint16, drops []SceneDrop) ([]byte, error) {
	if entity == 0 || entity == 65535 || len(drops) > 65535 {
		return nil, fmt.Errorf("invalid death/drop count")
	}
	p := add16(add16(nil, entity), uint16(len(drops)))
	seen := map[uint32]bool{}
	for _, d := range drops {
		if seen[d.Object] {
			return nil, fmt.Errorf("duplicate scene drop")
		}
		seen[d.Object] = true
		v, e := OrdinarySceneDropRecord(d)
		if e != nil {
			return nil, e
		}
		p = append(p, v...)
	}
	return append(p, 0, 0, 255, 0), nil
}
